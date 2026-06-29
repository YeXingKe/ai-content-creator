package parallel // 并行执行包，Agent5 配图阶段按 imageSource 分组并发处理

import (
	"context"          // 传递取消信号与超时，供各配图策略调用使用
	"encoding/json"    // 将单张配图完成事件序列化为 JSON 推送到 SSE
	"log"              // 打印并行执行过程与单张配图结果日志
	"sync"             // WaitGroup 等待 goroutine；Mutex 保护共享结果切片

	agentContext "github.com/ai-content-creator/backend/internal/agent/context" // 从 ctx 获取 SSE 流式推送回调
	"github.com/ai-content-creator/backend/internal/common"                   // SSE 消息类型常量
	"github.com/ai-content-creator/backend/internal/model"                    // ImageRequirement、ImageResult 等模型
	"github.com/ai-content-creator/backend/internal/service"                  // ImageServiceStrategy 配图策略
)

// ParallelImageGenerator 并行图片生成器（Agent5）
// 将 Agent4 输出的配图需求按 imageSource 分组，不同类型并行、同类型串行生成图片
type ParallelImageGenerator struct {
	imageStrategy *service.ImageServiceStrategy // 配图策略：按方式路由到 Pexels/NanoBanana/Mermaid 等并上传 COS
}

// NewParallelImageGenerator 创建并行图片生成器
func NewParallelImageGenerator(imageStrategy *service.ImageServiceStrategy) *ParallelImageGenerator {
	return &ParallelImageGenerator{
		imageStrategy: imageStrategy, // 注入配图策略，由 app 初始化时组装各 ImageService
	}
}

// Execute 执行并行图片生成任务（编排器在 Agent4 配图分析完成后调用）
func (g *ParallelImageGenerator) Execute(ctx context.Context, state *model.ArticleState) error {
	log.Printf("ParallelImageGenerator 开始执行: 配图需求数量=%d", len(state.ImageRequirements))

	// 无配图需求时直接跳过，避免空 goroutine 与无效 API 调用
	if len(state.ImageRequirements) == 0 {
		log.Printf("没有配图需求，跳过图片生成")
		state.Images = []model.ImageResult{} // 写入空切片，供 ContentMerger 安全处理
		return nil
	}

	// 从 context 获取 SSE 流式推送处理器（每完成一张图推送 IMAGE_COMPLETE 事件）
	streamHandler := agentContext.GetStreamHandler(ctx)

	// 按 imageSource（PEXELS、MERMAID 等）将需求分组，便于不同类型并行
	groupedBySource := g.groupByImageSource(state.ImageRequirements)

	log.Printf("配图需求按类型分组: %v", g.getGroupSummary(groupedBySource))

	// 各 imageSource 组并行执行，汇总所有 ImageResult
	allImages := g.executeParallel(ctx, groupedBySource, streamHandler)

	// 按 position 升序排序，保证与正文占位符顺序一致
	g.sortByPosition(allImages)

	state.Images = allImages // 将配图结果写回共享状态，供 ContentMerger 替换占位符
	log.Printf("ParallelImageGenerator 执行完成: 成功生成 %d 张图片", len(allImages))
	return nil // 成功返回 nil，编排器继续 ContentMerger 图文合成
}

// groupByImageSource 按 imageSource 字段将配图需求分组
func (g *ParallelImageGenerator) groupByImageSource(requirements []model.ImageRequirement) map[string][]model.ImageRequirement {
	grouped := make(map[string][]model.ImageRequirement) // key=imageSource，value=该类型的需求列表
	for _, req := range requirements {
		source := req.ImageSource // 配图方式，如 PEXELS、NANO_BANANA
		grouped[source] = append(grouped[source], req)
	}
	return grouped
}

// getGroupSummary 获取分组摘要（imageSource -> 数量），用于日志输出
func (g *ParallelImageGenerator) getGroupSummary(grouped map[string][]model.ImageRequirement) map[string]int {
	summary := make(map[string]int)
	for source, reqs := range grouped {
		summary[source] = len(reqs) // 每种方式的需求条数
	}
	return summary
}

// executeParallel 并行执行图片生成：不同 imageSource 并行，同一 imageSource 内部串行
func (g *ParallelImageGenerator) executeParallel(ctx context.Context, groupedBySource map[string][]model.ImageRequirement, streamHandler agentContext.StreamHandler) []model.ImageResult {
	var wg sync.WaitGroup // 等待所有 imageSource 组的 goroutine 完成
	var mu sync.Mutex     // 保护 allImages 切片，避免并发 append 竞态
	allImages := []model.ImageResult{}

	// 为每种 imageSource 启动一个 goroutine
	for imageSource, requirements := range groupedBySource {
		wg.Add(1) // 每启动一个 goroutine 计数 +1
		// 显式传参避免闭包捕获循环变量（Go 1.22+ 虽已修复，保持写法清晰）
		go func(source string, reqs []model.ImageRequirement) {
			defer wg.Done() // goroutine 结束时计数 -1

			log.Printf("开始处理 %s 类型的图片，数量: %d", source, len(reqs))

			// 同一类型内部串行：避免同 API 并发限流或重复连接开销
			for _, req := range reqs {
				imageResult := g.generateSingleImage(ctx, req, streamHandler)
				if imageResult != nil { // 失败时 generateSingleImage 返回 nil，跳过
					mu.Lock()
					allImages = append(allImages, *imageResult) // 加锁后追加到共享结果
					mu.Unlock()
				}
			}

			log.Printf("完成处理 %s 类型的图片", source)
		}(imageSource, requirements)
	}

	wg.Wait() // 阻塞直到所有 imageSource 组处理完毕

	return allImages
}

// generateSingleImage 根据单条配图需求调用策略生成图片并推送 SSE 完成事件
func (g *ParallelImageGenerator) generateSingleImage(ctx context.Context, req model.ImageRequirement, streamHandler agentContext.StreamHandler) *model.ImageResult {
	log.Printf("开始生成图片: position=%d, imageSource=%s, keywords=%s",
		req.Position, req.ImageSource, req.Keywords)

	// 转换为策略层统一的 ImageRequest 结构
	imageRequest := &model.ImageRequest{
		Keywords: req.Keywords, // 图库/图标搜索关键词
		Prompt:   req.Prompt,   // AI 生图或 Mermaid/SVG 描述
		Position: req.Position, // 在正文中的插入位置序号
		Type:     req.Type,     // 配图类型描述（如 cover、illustration）
	}

	// 按 imageSource 路由到对应 ImageService，获取图片并上传 COS，返回统一 URL
	result, err := g.imageStrategy.GetImageAndUpload(req.ImageSource, imageRequest)
	if err != nil {
		log.Printf("图片生成失败: imageSource=%s, position=%d, error=%v",
			req.ImageSource, req.Position, err)
		return nil // 单张失败不中断整体流程，该占位符后续可能无图
	}

	cosURL := result.URL   // 已上传到 COS 的访问地址
	method := result.Method // 实际使用的配图方式（可能与请求一致或为降级方案）

	// 组装配图结果，供 ContentMerger 按 PlaceholderID 替换占位符
	imageResult := &model.ImageResult{
		Position:      req.Position,
		URL:           cosURL,           // Markdown 图片 src
		Method:        method,           // 实际配图方式
		Keywords:      req.Keywords,     // 保留原始关键词
		SectionTitle:  req.SectionTitle, // 所属章节标题
		Description:   req.Type,         // 配图说明
		PlaceholderID: req.PlaceholderID, // 对应 {{IMAGE_PLACEHOLDER_N}} 的 N
	}

	// 每完成一张图，通过 SSE 通知前端更新进度/预览
	if streamHandler != nil {
		messageData := map[string]interface{}{
			"type":  common.SSEMsgImageComplete, // 事件类型：IMAGE_COMPLETE
			"image": imageResult,                // 单张配图详情
		}
		messageJSON, _ := json.Marshal(messageData)
		streamHandler(string(messageJSON)) // 推送到 SSE 连接
	}

	log.Printf("图片生成成功: position=%d, method=%s, cosUrl=%s",
		req.Position, method, cosURL)

	return imageResult
}

// sortByPosition 按 position 字段升序排序，使 Images 顺序与正文占位符一致
func (g *ParallelImageGenerator) sortByPosition(images []model.ImageResult) {
	// 冒泡排序：配图数量通常较少（个位数），简单实现足够
	n := len(images)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if images[j].Position > images[j+1].Position {
				images[j], images[j+1] = images[j+1], images[j] // 交换相邻元素
			}
		}
	}
}

package agents // 智能体包，每个文件实现一个独立的 Agent

import (
	"context"          // 上下文（与其他 Agent 接口保持一致，本 Agent 当前未使用）
	"encoding/json"    // 将日志输入/输出摘要序列化为 JSON
	"fmt"              // 格式化 Markdown 图片语法与日志消息
	"log"              // 打印执行过程日志
	"strings"          // 字符串查找与占位符替换
	"time"             // 记录 Agent 执行开始/结束时间与耗时

	"github.com/ai-content-creator/backend/internal/model"   // ArticleState、ImageResult、AgentLog 等模型
	"github.com/ai-content-creator/backend/internal/service" // AgentLogService 异步保存执行日志
)

// ContentMergerAgent 图文合成 Agent
// 将配图插入到正文的相应位置（通过占位符 {{IMAGE_PLACEHOLDER_N}} 替换为 Markdown 图片）
type ContentMergerAgent struct {
	agentLogService *service.AgentLogService // 智能体执行日志服务，用于记录本次合成耗时与输入输出摘要
}

// NewContentMergerAgent 创建图文合成 Agent
func NewContentMergerAgent(agentLogService *service.AgentLogService) *ContentMergerAgent {
	return &ContentMergerAgent{
		agentLogService: agentLogService, // 注入日志服务，由编排器在 app 初始化时传入
	}
}

// Execute 执行图文合成任务（编排器在 Agent5 配图完成后调用）
func (a *ContentMergerAgent) Execute(ctx context.Context, state *model.ArticleState) error {
	// 打印开始日志：带占位符的正文长度、待插入图片数量
	log.Printf("ContentMergerAgent 开始执行: 正文长度=%d, 图片数量=%d",
		len(state.ContentWithPlaceholders), len(state.Images))

	// 创建 Agent 执行日志，状态初始为 RUNNING
	startTime := time.Now() // 记录开始时间，用于计算耗时
	agentLog := &model.AgentLog{
		TaskID:    state.TaskID,       // 关联文章生成任务 ID
		AgentName: "content_merger",   // 智能体名称，写入 agent_log 表
		StartTime: startTime,          // 开始时间
		Status:    "RUNNING",          // 运行中
	}

	// 使用 defer 确保无论成功或 panic，日志都会在函数退出前异步保存
	defer func() {
		endTime := time.Now()                              // 结束时间
		agentLog.EndTime = &endTime                        // 写入日志结束时间（指针类型，表字段可空）
		duration := int(time.Since(startTime).Milliseconds()) // 耗时（毫秒）
		agentLog.DurationMs = &duration                    // 写入耗时
		a.agentLogService.SaveLogAsync(agentLog)           // 异步落库，不阻塞主流程
	}()

	// 记录输入摘要到日志（不存全文，避免日志过大）
	inputDataJSON, _ := json.Marshal(map[string]interface{}{
		"contentLength": len(state.ContentWithPlaceholders), // 带占位符正文长度
		"imagesCount":   len(state.Images),                  // 配图数量
	})
	inputDataStr := string(inputDataJSON) // []byte 转 string 供 AgentLog.InputData 存储
	agentLog.InputData = &inputDataStr  // 保存输入摘要

	// 核心逻辑：将占位符替换为 Markdown 图片，得到完整图文正文
	fullContent := a.mergeImagesIntoContent(state.ContentWithPlaceholders, state.Images)
	state.FullContent = fullContent // 写回共享状态，供落库与 SSE MERGE_COMPLETE 推送

	agentLog.Status = "SUCCESS" // 标记执行成功
	// 记录输出摘要到日志
	outputDataJSON, _ := json.Marshal(map[string]interface{}{
		"fullContentLength": len(fullContent),                              // 合成后全文长度
		"message":           fmt.Sprintf("完整内容长度: %d 字符", len(fullContent)), // 人类可读摘要
	})
	outputDataStr := string(outputDataJSON)
	agentLog.OutputData = &outputDataStr // 保存输出摘要

	log.Printf("ContentMergerAgent：图文合成完成, fullContentLength=%d", len(fullContent))
	return nil // 成功返回 nil，编排器继续后续流程
}

// mergeImagesIntoContent 将配图插入正文（使用占位符替换，不调 LLM）
func (a *ContentMergerAgent) mergeImagesIntoContent(content string, images []model.ImageResult) string {
	if len(images) == 0 {
		return content // 无配图时直接返回原正文（占位符可能仍存在）
	}

	fullContent := content // 在副本上逐张替换，避免修改入参语义

	// 遍历 Agent5 生成的每张配图结果
	for _, image := range images {
		placeholder := image.PlaceholderID // 占位符 ID，如 {{IMAGE_PLACEHOLDER_1}}
		log.Printf("处理图片: position=%d, placeholderId=%s, url=%s",
			image.Position, placeholder, image.URL)

		if placeholder != "" { // 占位符非空才尝试替换
			description := image.Description // 图片 alt 描述，用于 Markdown ![描述](url)
			if description == "" {
				description = "配图" // 描述为空时使用默认文案
			}
			imageMarkdown := fmt.Sprintf("![%s](%s)", description, image.URL) // 标准 Markdown 图片语法

			if strings.Contains(fullContent, placeholder) {
				// 只替换第一次出现，防止同一占位符被重复替换
				fullContent = strings.Replace(fullContent, placeholder, imageMarkdown, 1)
				log.Printf("成功替换占位符: %s -> %s", placeholder, truncate(imageMarkdown, 50))
			} else {
				log.Printf("正文中未找到占位符: %s", placeholder) // 找不到仅告警，不中断流程
			}
		} else {
			log.Printf("图片 position=%d 的 placeholderId 为空", image.Position) // 数据异常，跳过该图
		}
	}

	return fullContent // 返回替换后的完整 Markdown 正文
}

// truncate 截断字符串用于日志输出（避免 URL 过长刷屏）
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s // 未超长则原样返回
	}
	return s[:maxLen] + "..." // 截断并追加省略号
}

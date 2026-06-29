package agents // 智能体包，每个文件实现一个独立的 Agent

import (
	"context"          // 传递取消信号与超时，供 LLM 调用使用
	"encoding/json"    // 解析 LLM 返回的 JSON；日志输出摘要序列化
	"fmt"              // 格式化 Prompt 说明、错误与日志消息
	"log"              // 打印 Agent 执行过程与验证结果日志
	"strings"          // 替换 Prompt 占位符；拼接方式说明与使用指南
	"time"             // 记录 Agent 执行开始/结束时间与耗时

	"github.com/tmc/langchaingo/llms" // LangChainGo LLM 抽象，GenerateFromSinglePrompt 一次性生成
	"github.com/ai-content-creator/backend/internal/common" // Prompt 模板、配图方式常量与描述
	"github.com/ai-content-creator/backend/internal/model"    // ArticleState、ImageRequirement、AgentLog
	"github.com/ai-content-creator/backend/internal/service"  // AgentLogService 异步保存执行日志
)

// ImageAnalyzerAgent 配图需求分析 Agent（Agent4）
// 分析 Markdown 正文，生成配图需求列表，并在正文中插入 {{IMAGE_PLACEHOLDER_N}} 占位符
type ImageAnalyzerAgent struct {
	llm             llms.Model               // 大模型客户端，负责分析正文并输出结构化 JSON
	agentLogService *service.AgentLogService // 智能体执行日志服务
}

// NewImageAnalyzerAgent 创建配图需求分析 Agent
func NewImageAnalyzerAgent(llm llms.Model, agentLogService *service.AgentLogService) *ImageAnalyzerAgent {
	return &ImageAnalyzerAgent{
		llm:             llm,             // 注入 LLM 实例
		agentLogService: agentLogService, // 注入日志服务
	}
}

// Execute 执行配图需求分析任务（编排器在 Agent3 正文生成完成后调用）
func (a *ImageAnalyzerAgent) Execute(ctx context.Context, state *model.ArticleState) error {
	// 打印开始日志：主标题与用户启用的配图方式列表
	log.Printf("ImageAnalyzerAgent 开始执行: mainTitle=%s, enabledMethods=%v",
		state.Title.MainTitle, state.EnabledImageMethods)

	// 创建 Agent 执行日志，状态初始为 RUNNING
	startTime := time.Now() // 记录开始时间，用于计算耗时
	agentLog := &model.AgentLog{
		TaskID:    state.TaskID,                         // 关联文章生成任务 ID
		AgentName: "agent4_analyze_image_requirements", // 智能体名称，与旧版 Agent4 保持一致
		StartTime: startTime,                            // 开始时间
		Status:    "RUNNING",                              // 运行中
	}

	// 使用 defer 确保无论成功或失败，日志都会在函数退出前异步保存
	defer func() {
		endTime := time.Now()                               // 结束时间
		agentLog.EndTime = &endTime                         // 写入日志结束时间
		duration := int(time.Since(startTime).Milliseconds()) // 耗时（毫秒）
		agentLog.DurationMs = &duration                     // 写入耗时
		a.agentLogService.SaveLogAsync(agentLog)          // 异步落库，不阻塞主流程
	}()

	// 构建「可用配图方式」说明文本，供 LLM 了解可选 imageSource
	availableMethods := a.buildAvailableMethodsDescription(state.EnabledImageMethods)

	// 构建各配图方式的详细使用指南（keywords/prompt 如何填写）
	methodUsageGuide := a.buildMethodUsageGuide(state.EnabledImageMethods)

	// 组装完整 Prompt：主标题、正文、可用方式、使用指南
	prompt := strings.ReplaceAll(common.Agent4ImageRequirementsPrompt, "{mainTitle}", state.Title.MainTitle)
	prompt = strings.ReplaceAll(prompt, "{content}", state.Content)
	prompt = strings.ReplaceAll(prompt, "{availableMethods}", availableMethods)
	prompt = strings.ReplaceAll(prompt, "{methodUsageGuide}", methodUsageGuide)
	agentLog.Prompt = &prompt // 保存完整 Prompt 到日志

	// 一次性调用 LLM，期望返回 JSON（含带占位符正文与配图需求数组）
	content, err := llms.GenerateFromSinglePrompt(ctx, a.llm, prompt)
	if err != nil {
		agentLog.Status = "FAILED"      // 标记失败
		errMsg := err.Error()           // 提取错误信息
		agentLog.ErrorMessage = &errMsg // 写入日志
		return err                      // 返回错误，编排器将任务标记为 FAILED
	}

	// 解析 LLM 返回的 JSON 结构
	var result struct {
		ContentWithPlaceholders string                   `json:"contentWithPlaceholders"` // 插入占位符后的正文
		ImageRequirements       []model.ImageRequirement `json:"imageRequirements"`       // 配图需求列表
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("ImageAnalyzerAgent：配图需求解析失败, content=%s", content) // 打印原始内容便于排查
		agentLog.Status = "FAILED"                                          // 标记失败
		errMsg := "parse image requirements failed: " + err.Error()
		agentLog.ErrorMessage = &errMsg
		return fmt.Errorf("parse image requirements failed: %w", err) // 包装 JSON 解析错误
	}

	// 将带占位符的正文写回共享状态，供 Agent5 配图与 ContentMerger 使用
	state.ContentWithPlaceholders = result.ContentWithPlaceholders

	// 校验 imageSource 是否在用户允许列表中，非法项过滤或替换
	validatedRequirements := a.validateAndFilterImageRequirements(result.ImageRequirements, state.EnabledImageMethods)
	state.ImageRequirements = validatedRequirements // 写入校验后的配图需求

	agentLog.Status = "SUCCESS" // 标记成功
	// 记录输出摘要到日志（需求数量，不存全文）
	outputDataJSON, _ := json.Marshal(map[string]interface{}{
		"requirementsCount": len(validatedRequirements),                              // 有效配图需求数
		"message":           fmt.Sprintf("分析出 %d 个配图需求", len(validatedRequirements)), // 人类可读摘要
	})
	outputDataStr := string(outputDataJSON)
	agentLog.OutputData = &outputDataStr // 保存输出摘要

	log.Printf("ImageAnalyzerAgent：配图需求分析成功, count=%d, validated=%d, 已在正文中插入占位符",
		len(result.ImageRequirements), len(validatedRequirements)) // 原始数量 vs 校验后数量

	return nil // 成功返回 nil，编排器继续 Agent5 配图
}

// buildAvailableMethodsDescription 构建可用配图方式说明（供 Prompt 中的 {availableMethods} 占位符）
func (a *ImageAnalyzerAgent) buildAvailableMethodsDescription(enabledMethods []string) string {
	// 若用户未指定，视为支持所有非降级方式
	methods := enabledMethods
	if len(methods) == 0 {
		methods = common.GetAllMethods() // 获取全部注册的配图方式
		// 移除 PICSUM 等降级方案，不在 Prompt 中展示给 LLM
		filteredMethods := []string{}
		for _, m := range methods {
			if !common.IsFallback(m) { // 跳过降级/兜底方式
				filteredMethods = append(filteredMethods, m)
			}
		}
		methods = filteredMethods
	}

	// 逐条拼接「方式名: 描述」
	var descriptions []string
	for _, method := range methods {
		desc := common.GetDescription(method) // 从 common 包读取人类可读描述
		descriptions = append(descriptions, fmt.Sprintf("- %s: %s", method, desc))
	}

	return strings.Join(descriptions, "\n") // 多行文本，每行一种方式
}

// buildMethodUsageGuide 构建各配图方式的详细使用指南（供 Prompt 中的 {methodUsageGuide} 占位符）
func (a *ImageAnalyzerAgent) buildMethodUsageGuide(enabledMethods []string) string {
	// 若用户未指定，遍历全部方式生成指南
	methods := enabledMethods
	if len(methods) == 0 {
		methods = common.GetAllMethods()
	}

	guide := strings.Builder{} // 高效拼接多段指南文本

	for _, method := range methods {
		if common.IsFallback(method) {
			continue // 降级方案不写入指南
		}

		// 按配图方式输出 keywords / prompt 填写规则
		switch method {
		case common.ImageMethodPexels: // 图库搜索
			guide.WriteString("- PEXELS: 提供英文搜索关键词(keywords)，要准确、具体。prompt 留空。\n")
		case common.ImageMethodNanoBanana: // AI 生图
			guide.WriteString("- NANO_BANANA: 提供详细的英文生图提示词(prompt)，描述场景、风格、细节。keywords 留空。\n")
		case common.ImageMethodMermaid: // Mermaid 图表
			guide.WriteString("- MERMAID: 在 prompt 字段生成完整的 Mermaid 代码（如流程图、架构图）。keywords 留空。\n")
		case common.ImageMethodIconify: // 图标
			guide.WriteString("- ICONIFY: 提供英文图标关键词(keywords)，如：check、arrow、star、heart。prompt 留空。\n")
		case common.ImageMethodEmojiPack: // 表情包
			guide.WriteString("- EMOJI_PACK: 提供中文或英文关键词(keywords)描述表情内容。prompt 留空。系统会自动添加\"表情包\"搜索。\n")
		case common.ImageMethodSVGDiagram: // SVG 示意图
			guide.WriteString("- SVG_DIAGRAM: 在 prompt 字段描述示意图需求（中文），说明要表达的概念和关系。keywords 留空。\n")
			guide.WriteString("  示例：绘制思维导图样式的图，中心是\"自律\"，周围4个分支：习惯、环境、反馈、系统\n")
		}
	}

	return guide.String() // 返回完整使用指南字符串
}

// validateAndFilterImageRequirements 验证并过滤配图需求，确保 imageSource 在用户允许列表内
func (a *ImageAnalyzerAgent) validateAndFilterImageRequirements(requirements []model.ImageRequirement, enabledMethods []string) []model.ImageRequirement {
	// enabledMethods 为空表示不限制，直接返回 LLM 原始结果
	if len(enabledMethods) == 0 {
		return requirements
	}

	// 将允许的方式列表转为 map，便于 O(1) 查找
	allowedSet := make(map[string]bool)
	for _, method := range enabledMethods {
		allowedSet[method] = true
	}

	// 逐条校验配图需求
	var validated []model.ImageRequirement
	for _, req := range requirements {
		imageSource := req.ImageSource // LLM 指定的配图方式

		// 在允许列表内则直接保留
		if allowedSet[imageSource] {
			validated = append(validated, req)
			log.Printf("配图需求验证通过, position=%d, imageSource=%s", req.Position, imageSource)
		} else {
			// 不在允许列表内则记录并尝试替换
			log.Printf("配图需求不符合限制被过滤, position=%d, imageSource=%s, enabledMethods=%v",
				req.Position, imageSource, enabledMethods)

			// 替换为用户允许的第一种方式，避免整条需求丢失
			if len(enabledMethods) > 0 {
				fallbackSource := enabledMethods[0] // 优先使用列表首项
				req.ImageSource = fallbackSource
				validated = append(validated, req)
				log.Printf("配图需求已替换为允许的方式, position=%d, fallback=%s",
					req.Position, fallbackSource)
			}
		}
	}

	return validated // 返回校验/替换后的配图需求列表
}

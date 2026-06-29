package agents // 智能体包，每个文件实现一个独立的 Agent

import (
	"context"          // 传递取消信号、超时，以及流式 SSE 处理器
	"encoding/json"    // 将大纲序列化为 JSON 嵌入 Prompt；日志输出摘要
	"fmt"              // 格式化 SSE 消息与错误信息
	"log"              // 打印 Agent 执行过程日志
	"strings"          // 替换 Prompt 占位符；Builder 拼接流式正文
	"time"             // 记录 Agent 执行开始/结束时间与耗时

	"github.com/tmc/langchaingo/llms" // LangChainGo LLM 抽象，对接 DashScope（OpenAI 兼容）
	agentContext "github.com/ai-content-creator/backend/internal/agent/context" // 从 ctx 获取 SSE 流式推送回调
	"github.com/ai-content-creator/backend/internal/common"                   // Prompt 模板、文章风格、SSE 消息类型
	"github.com/ai-content-creator/backend/internal/model"                    // ArticleState、AgentLog 等模型
	"github.com/ai-content-creator/backend/internal/service"                  // AgentLogService 异步保存执行日志
)

// ContentGeneratorAgent 正文生成 Agent（Agent3）
// 根据用户确认的标题与大纲，调用 LLM 流式生成 Markdown 正文
type ContentGeneratorAgent struct {
	llm             llms.Model                  // 大模型客户端，负责 GenerateContent 流式调用
	agentLogService *service.AgentLogService    // 智能体执行日志服务
}

// NewContentGeneratorAgent 创建正文生成 Agent
func NewContentGeneratorAgent(llm llms.Model, agentLogService *service.AgentLogService) *ContentGeneratorAgent {
	return &ContentGeneratorAgent{
		llm:             llm,             // 注入 LLM 实例，与编排器/其他 Agent 共享
		agentLogService: agentLogService, // 注入日志服务
	}
}

// Execute 执行正文生成任务（编排器在用户确认大纲后调用，支持 SSE 流式推送）
func (a *ContentGeneratorAgent) Execute(ctx context.Context, state *model.ArticleState) error {
	// 打印开始日志，主标题来自用户 confirm-title 阶段选定的方案
	log.Printf("ContentGeneratorAgent 开始执行: mainTitle=%s", state.Title.MainTitle)

	// 创建 Agent 执行日志，状态初始为 RUNNING
	startTime := time.Now() // 记录开始时间，用于计算耗时
	agentLog := &model.AgentLog{
		TaskID:    state.TaskID,              // 关联文章生成任务 ID
		AgentName: "agent3_generate_content", // 智能体名称，与旧版单链路 Agent3 保持一致
		StartTime: startTime,                 // 开始时间
		Status:    "RUNNING",                   // 运行中
	}

	// 使用 defer 确保无论成功或失败，日志都会在函数退出前异步保存
	defer func() {
		endTime := time.Now()                               // 结束时间
		agentLog.EndTime = &endTime                         // 写入日志结束时间
		duration := int(time.Since(startTime).Milliseconds()) // 耗时（毫秒）
		agentLog.DurationMs = &duration                     // 写入耗时
		a.agentLogService.SaveLogAsync(agentLog)          // 异步落库，不阻塞主流程
	}()

	// 构建发给 LLM 的完整 Prompt
	outlineJSON, _ := json.Marshal(state.Outline.Sections) // 将大纲章节列表转为 JSON 字符串
	prompt := strings.ReplaceAll(common.Agent3ContentPrompt, "{mainTitle}", state.Title.MainTitle) // 替换主标题占位符
	prompt = strings.ReplaceAll(prompt, "{subTitle}", state.Title.SubTitle)                        // 替换副标题占位符
	prompt = strings.ReplaceAll(prompt, "{outline}", string(outlineJSON))                          // 替换大纲 JSON 占位符
	prompt += a.getStylePrompt(state.Style)                                                        // 追加文章风格附加说明（tech/emotional 等）
	agentLog.Prompt = &prompt // 保存完整 Prompt 到日志，便于审计与调试

	// 从 context 获取 SSE 流式推送处理器（编排器通过 WithStreamHandler 注入）
	streamHandler := agentContext.GetStreamHandler(ctx)

	// 流式调用 LLM：边生成边推送 SSE，同时拼接完整正文
	content, err := a.callLLMWithStreaming(ctx, prompt, streamHandler)
	if err != nil {
		agentLog.Status = "FAILED"       // 标记失败
		errMsg := err.Error()            // 提取错误信息
		agentLog.ErrorMessage = &errMsg  // 写入日志
		return err                       // 返回错误，编排器将任务标记为 FAILED
	}

	state.Content = content    // 将生成的 Markdown 正文写回共享状态，供后续配图 Agent 使用
	agentLog.Status = "SUCCESS"  // 标记成功
	// 记录输出摘要到日志（不存全文，避免日志过大）
	outputDataJSON, _ := json.Marshal(map[string]interface{}{
		"contentLength": len(content),                              // 正文字符数
		"message":       fmt.Sprintf("正文长度: %d 字符", len(content)), // 人类可读摘要
	})
	outputDataStr := string(outputDataJSON)
	agentLog.OutputData = &outputDataStr // 保存输出摘要

	log.Printf("ContentGeneratorAgent：正文生成成功, length=%d", len(content))
	return nil // 成功返回 nil，编排器继续 Agent4 配图分析
}

// callLLMWithStreaming 调用 LLM 并以流式方式接收、推送、拼接正文
func (a *ContentGeneratorAgent) callLLMWithStreaming(ctx context.Context, prompt string, streamHandler agentContext.StreamHandler) (string, error) {
	var contentBuilder strings.Builder // 高效拼接流式收到的多段文本

	// 调用 LLM 流式生成；WithStreamingFunc 每收到一块数据就执行回调
	_, err := a.llm.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt), // 构造 Human 角色消息，内容为完整 Prompt
	}, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		text := string(chunk)              // 将字节块转为字符串
		contentBuilder.WriteString(text)   // 追加到完整正文

		// 通过 SSE 实时推送给前端（格式：AGENT3_STREAMING:本次 chunk）
		if streamHandler != nil {
			message := fmt.Sprintf("%s%s", common.SSEMsgAgent3Streaming+":", text) // SSEMsgAgent3Streaming = "AGENT3_STREAMING"
			streamHandler(message) // 调用编排器注入的回调，最终经 SSEManager 推到浏览器
		}
		return nil // 回调无错误则继续接收下一块
	}))

	if err != nil {
		log.Printf("ContentGeneratorAgent：流式调用失败, error=%v", err)
		return "", fmt.Errorf("streaming LLM call failed: %w", err) // 包装错误返回给 Execute
	}

	return contentBuilder.String(), nil // 返回拼接后的完整 Markdown 正文
}

// getStylePrompt 根据文章风格返回附加 Prompt 片段（空风格返回空字符串）
func (a *ContentGeneratorAgent) getStylePrompt(style string) string {
	if style == "" {
		return "" // 未指定风格时不追加额外写作要求
	}

	switch style {
	case common.ArticleStyleTech: // 科技风格
		return common.StyleTechPrompt
	case common.ArticleStyleEmotional: // 情感风格
		return common.StyleEmotionalPrompt
	case common.ArticleStyleEducational: // 教育风格
		return common.StyleEducationalPrompt
	case common.ArticleStyleHumorous: // 轻松幽默风格
		return common.StyleHumorousPrompt
	default:
		return "" // 未知风格忽略，不影响主 Prompt
	}
}

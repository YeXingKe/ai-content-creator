package agents // 智能体包，每个文件实现一个独立的 Agent

import (
	"context"          // 传递取消信号、超时，以及流式 SSE 处理器
	"encoding/json"    // 解析 LLM 返回的大纲 JSON
	"fmt"              // 格式化 SSE 消息与错误信息
	"log"              // 打印 Agent 执行过程日志
	"strings"          // 替换 Prompt 占位符；Builder 拼接流式输出

	"github.com/tmc/langchaingo/llms" // LangChainGo LLM 抽象，对接 DashScope（OpenAI 兼容）
	agentContext "github.com/ai-content-creator/backend/internal/agent/context" // 从 ctx 获取 SSE 流式推送回调
	"github.com/ai-content-creator/backend/internal/common"                   // Prompt 模板、文章风格、SSE 消息类型
	"github.com/ai-content-creator/backend/internal/model"                    // ArticleState、OutlineResult 等模型
)

// OutlineGeneratorAgent 大纲生成 Agent（Agent2）
// 根据用户确认的标题（及可选描述）调用 LLM 流式生成文章大纲 JSON
type OutlineGeneratorAgent struct {
	llm llms.Model // 大模型客户端，负责 GenerateContent 流式调用
}

// NewOutlineGeneratorAgent 创建大纲生成 Agent
func NewOutlineGeneratorAgent(llm llms.Model) *OutlineGeneratorAgent {
	return &OutlineGeneratorAgent{
		llm: llm, // 注入 LLM 实例，与编排器/其他 Agent 共享
	}
}

// Execute 执行大纲生成任务（编排器在用户 confirm-title 后调用，支持 SSE 流式推送）
func (a *OutlineGeneratorAgent) Execute(ctx context.Context, state *model.ArticleState) error {
	// 打印开始日志，标题来自用户选定的标题方案
	log.Printf("OutlineGeneratorAgent 开始执行: mainTitle=%s, subTitle=%s",
		state.Title.MainTitle, state.Title.SubTitle)

	// 构建用户描述段落：有描述时插入模板，无描述时留空
	descriptionSection := ""
	if state.UserDescription != "" {
		descriptionSection = strings.ReplaceAll(
			common.Agent2DescriptionSection, // 描述段落模板，含 {userDescription} 占位符
			"{userDescription}",
			state.UserDescription, // 用户创建任务时填写的补充说明
		)
	}

	// 组装完整 Prompt：主标题、副标题、描述段落、风格附加说明
	prompt := strings.ReplaceAll(common.Agent2OutlinePrompt, "{mainTitle}", state.Title.MainTitle)
	prompt = strings.ReplaceAll(prompt, "{subTitle}", state.Title.SubTitle)
	prompt = strings.ReplaceAll(prompt, "{descriptionSection}", descriptionSection)
	prompt += a.getStylePrompt(state.Style) // 追加文章风格写作要求（tech/emotional 等）

	// 从 context 获取 SSE 流式推送处理器（编排器通过 WithStreamHandler 注入）
	streamHandler := agentContext.GetStreamHandler(ctx)

	// 流式调用 LLM：边生成边推送 SSE，同时拼接完整 JSON 字符串
	content, err := a.callLLMWithStreaming(ctx, prompt, streamHandler)
	if err != nil {
		return err // LLM 调用失败，编排器将任务标记为 FAILED
	}

	// 将 LLM 返回的 JSON 反序列化为 OutlineResult
	var outlineResult model.OutlineResult
	if err := json.Unmarshal([]byte(content), &outlineResult); err != nil {
		log.Printf("OutlineGeneratorAgent：大纲解析失败, content=%s", content) // 打印原始内容便于排查
		return fmt.Errorf("parse outline failed: %w", err)                  // 包装 JSON 解析错误
	}

	state.Outline = &outlineResult // 将大纲写回共享状态，等待用户 confirm-outline 后进入 Agent3
	log.Printf("OutlineGeneratorAgent：大纲生成成功, sections=%d", len(outlineResult.Sections))
	return nil // 成功返回 nil，任务进入 WAITING_CONFIRM_OUTLINE 阶段
}

// callLLMWithStreaming 调用 LLM 并以流式方式接收、推送、拼接大纲 JSON
func (a *OutlineGeneratorAgent) callLLMWithStreaming(ctx context.Context, prompt string, streamHandler agentContext.StreamHandler) (string, error) {
	var contentBuilder strings.Builder // 高效拼接流式收到的多段文本

	// 调用 LLM 流式生成；WithStreamingFunc 每收到一块数据就执行回调
	_, err := a.llm.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt), // 构造 Human 角色消息，内容为完整 Prompt
	}, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		text := string(chunk)            // 将字节块转为字符串
		contentBuilder.WriteString(text) // 追加到完整 JSON 字符串

		// 通过 SSE 实时推送给前端（格式：AGENT2_STREAMING:本次 chunk）
		if streamHandler != nil {
			message := fmt.Sprintf("%s%s", common.SSEMsgAgent2Streaming+":", text) // SSEMsgAgent2Streaming = "AGENT2_STREAMING"
			streamHandler(message) // 调用编排器注入的回调，最终经 SSEManager 推到浏览器
		}
		return nil // 回调无错误则继续接收下一块
	}))

	if err != nil {
		log.Printf("OutlineGeneratorAgent：流式调用失败, error=%v", err)
		return "", fmt.Errorf("streaming LLM call failed: %w", err) // 包装错误返回给 Execute
	}

	return contentBuilder.String(), nil // 返回拼接后的完整 JSON 字符串
}

// getStylePrompt 根据文章风格返回附加 Prompt 片段（空风格返回空字符串）
func (a *OutlineGeneratorAgent) getStylePrompt(style string) string {
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

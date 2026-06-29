package agents // 智能体包，每个文件实现一个独立的 Agent

import (
	"context"          // 传递取消信号与超时，供 LLM 调用使用
	"encoding/json"    // 解析 LLM 返回的标题方案 JSON 数组；日志输出摘要
	"fmt"              // 格式化错误与日志消息
	"log"              // 打印 Agent 执行过程日志
	"strings"          // 替换 Prompt 占位符
	"time"             // 记录 Agent 执行开始/结束时间与耗时

	"github.com/tmc/langchaingo/llms" // LangChainGo LLM 抽象，GenerateFromSinglePrompt 一次性生成
	"github.com/ai-content-creator/backend/internal/common" // Prompt 模板、文章风格常量
	"github.com/ai-content-creator/backend/internal/model"    // ArticleState、TitleOption、AgentLog
	"github.com/ai-content-creator/backend/internal/service"  // AgentLogService 异步保存执行日志
)

// TitleGeneratorAgent 标题生成 Agent（Agent1）
// 根据用户选题与文章风格，调用 LLM 生成 3-5 个爆款标题方案供用户选择
type TitleGeneratorAgent struct {
	llm             llms.Model               // 大模型客户端，负责一次性生成标题 JSON
	agentLogService *service.AgentLogService // 智能体执行日志服务
}

// NewTitleGeneratorAgent 创建标题生成 Agent
func NewTitleGeneratorAgent(llm llms.Model, agentLogService *service.AgentLogService) *TitleGeneratorAgent {
	return &TitleGeneratorAgent{
		llm:             llm,             // 注入 LLM 实例，与编排器/其他 Agent 共享
		agentLogService: agentLogService, // 注入日志服务
	}
}

// Execute 执行标题生成任务（编排器在 Phase1 开始时调用，为文章生成流程第一步）
func (a *TitleGeneratorAgent) Execute(ctx context.Context, state *model.ArticleState) error {
	// 打印开始日志：选题与文章风格来自用户创建任务时的输入
	log.Printf("TitleGeneratorAgent 开始执行: topic=%s, style=%s", state.Topic, state.Style)

	// 创建 Agent 执行日志，状态初始为 RUNNING
	startTime := time.Now() // 记录开始时间，用于计算耗时
	agentLog := &model.AgentLog{
		TaskID:    state.TaskID,           // 关联文章生成任务 ID
		AgentName: "agent1_generate_titles", // 智能体名称，与旧版单链路 Agent1 保持一致
		StartTime: startTime,              // 开始时间
		Status:    "RUNNING",                // 运行中
	}

	// 使用 defer 确保无论成功或失败，日志都会在函数退出前异步保存
	defer func() {
		endTime := time.Now()                               // 结束时间
		agentLog.EndTime = &endTime                         // 写入日志结束时间
		duration := int(time.Since(startTime).Milliseconds()) // 耗时（毫秒）
		agentLog.DurationMs = &duration                     // 写入耗时
		a.agentLogService.SaveLogAsync(agentLog)          // 异步落库，不阻塞主流程
	}()

	// 组装完整 Prompt：替换选题占位符，并追加风格附加说明
	prompt := strings.ReplaceAll(common.Agent1TitlePrompt, "{topic}", state.Topic)
	prompt += a.getStylePrompt(state.Style) // 追加 tech/emotional 等风格写作要求
	agentLog.Prompt = &prompt               // 保存完整 Prompt 到日志，便于审计与调试

	log.Printf("TitleGeneratorAgent：发送请求到 LLM, promptLength=%d", len(prompt))

	// 一次性调用 LLM，期望返回 TitleOption 数组的 JSON 字符串
	content, err := llms.GenerateFromSinglePrompt(ctx, a.llm, prompt)
	if err != nil {
		log.Printf("TitleGeneratorAgent：LLM 调用失败, error=%v", err)
		agentLog.Status = "FAILED"      // 标记失败
		errMsg := err.Error()           // 提取错误信息
		agentLog.ErrorMessage = &errMsg // 写入日志
		return fmt.Errorf("LLM call failed: %w", err) // 包装错误返回给编排器
	}

	log.Printf("TitleGeneratorAgent：收到响应, contentLength=%d", len(content))

	// 将 LLM 返回的 JSON 反序列化为标题方案列表
	var titleOptions []model.TitleOption
	if err := json.Unmarshal([]byte(content), &titleOptions); err != nil {
		log.Printf("TitleGeneratorAgent：标题方案解析失败, content=%s", content) // 打印原始内容便于排查
		agentLog.Status = "FAILED"                                            // 标记失败
		errMsg := "parse title options: " + err.Error()
		agentLog.ErrorMessage = &errMsg
		return fmt.Errorf("parse title options: %w", err) // 包装 JSON 解析错误
	}

	state.TitleOptions = titleOptions // 将标题方案写回共享状态，供前端展示与用户 confirm-title
	agentLog.Status = "SUCCESS"       // 标记成功
	// 记录输出摘要到日志（方案数量，不存全文）
	outputDataJSON, _ := json.Marshal(map[string]interface{}{
		"optionsCount": len(titleOptions),                              // 生成的标题方案数
		"message":      fmt.Sprintf("生成 %d 个标题方案", len(titleOptions)), // 人类可读摘要
	})
	outputDataStr := string(outputDataJSON)
	agentLog.OutputData = &outputDataStr // 保存输出摘要

	log.Printf("TitleGeneratorAgent：标题方案生成成功, optionsCount=%d", len(titleOptions))
	return nil // 成功返回 nil，任务进入 WAITING_CONFIRM_TITLE 阶段
}

// getStylePrompt 根据文章风格返回附加 Prompt 片段（空风格返回空字符串）
func (a *TitleGeneratorAgent) getStylePrompt(style string) string {
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

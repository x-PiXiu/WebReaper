package agent

import (
	"context"
	"fmt"

	"webreaper/internal/usecase/port"
)

// AgentOrchestrator 获客智能体编排器（整洁架构·Usecase层）。
//
// 设计动机：
//   - 用户通过聊天与AI交互，AI自动调用工具完成任务
//   - 用户不需要选择端点、模型、参数
//   - 根据用户权限决定是否需要确认
//
// 工作流程：
//   1. 获取用户权限
//   2. 构建系统提示词（动态配置 + 硬编码）
//   3. 调用 LLM 执行
//   4. 返回结果
type AgentOrchestrator struct {
	llm           port.AIGenerator
	promptBuilder *PromptBuilder
}

func NewAgentOrchestrator(
	llm port.AIGenerator,
	toolRegistry *port.ToolRegistry,  // 保留参数但不使用，保持接口兼容
	promptBuilder *PromptBuilder,
) *AgentOrchestrator {
	return &AgentOrchestrator{
		llm:           llm,
		promptBuilder: promptBuilder,
	}
}

// Execute 执行智能体任务。
//
// 参数：
//   - ctx：上下文
//   - tenantID：租户ID
//   - permissionLevel：用户权限级别（full_access/semi_auto/manual_confirm）
//   - task：用户任务描述
//
// 返回：
//   - 执行结果
//   - 错误信息
func (o *AgentOrchestrator) Execute(ctx context.Context, tenantID, permissionLevel, task string) (string, error) {
	// 1. 构建系统提示词（动态配置 + 硬编码）
	systemPrompt := ""
	if o.promptBuilder != nil {
		prompt, err := o.promptBuilder.Build(ctx, tenantID, permissionLevel)
		if err == nil {
			systemPrompt = prompt
		}
	}

	// 2. 调用 LLM 执行（流式对话）
	var result string
	_, err := o.llm.ChatStream(ctx, "", "", []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task},
	}, func(delta string) {
		result += delta
	})
	if err != nil {
		return "", fmt.Errorf("LLM 执行失败: %w", err)
	}

	// 3. 返回结果
	return result, nil
}

// needsConfirm 检查是否需要用户确认。
func (o *AgentOrchestrator) needsConfirm(permissionLevel string, toolName string) bool {
	// full_access：所有决策由LLM决定，无需确认
	if permissionLevel == "full_access" {
		return false
	}

	// semi_auto：关键决策需要确认
	if permissionLevel == "semi_auto" {
		// 关键决策：生成内容、选择素材、发布
		criticalTools := map[string]bool{
			"generate_image":         true,
			"generate_video":         true,
			"generate_audio":         true,
			"generate_digital_human": true,
			"publish_work":           true,
		}
		return criticalTools[toolName]
	}

	// manual_confirm：每一步都需要确认
	return true
}

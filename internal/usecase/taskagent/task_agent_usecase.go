// Package taskagent 实现"通用任务执行"用例。
//
// 职责：接收任意任务 → 委托 port.TaskAgent 自主执行（调工具/子能力直到完成）。
//
// 整洁架构定位：本用例只依赖 port.TaskAgent 接口，不知道具体是 Explorer 还是别的实现。
// "Agent 如何自主规划"是 adapter 层的事，本用例只做业务编排（执行 + 可选落库）。
package taskagent

import (
	"context"
	"fmt"

	"webreaper/internal/usecase/port"
)

// TaskAgentUseCase 通用任务执行用例。
type TaskAgentUseCase struct {
	agent  port.TaskAgent
	logger port.Logger
}

func NewTaskAgentUseCase(agent port.TaskAgent, logger port.Logger) *TaskAgentUseCase {
	if logger == nil {
		logger = port.NopLogger{}
	}
	return &TaskAgentUseCase{
		agent:  agent,
		logger: logger.With(port.String("component", "task_agent")),
	}
}

// TaskExecuteInput 用例输入。
type TaskExecuteInput struct {
	Task         string   // 任意任务描述
	Tools        []string // 允许的工具（空=全部）
	SystemPrompt string   // 系统提示词（空=默认）
}

// TaskExecuteOutput 用例输出。
type TaskExecuteOutput struct {
	Response string // Agent 最终回复
}

// Execute 执行任意任务，Agent 自主规划直到完成。
// onEvent 透传给 Agent（供 SSE 流式进度用）。
func (uc *TaskAgentUseCase) Execute(ctx context.Context, in TaskExecuteInput, onEvent func(port.TaskEvent)) (TaskExecuteOutput, error) {
	if in.Task == "" {
		return TaskExecuteOutput{}, fmt.Errorf("task is required")
	}
	if uc.agent == nil {
		return TaskExecuteOutput{}, fmt.Errorf("任务 Agent 未配置")
	}

	uc.logger.Info("执行通用任务",
		port.String("task", fmt.Sprintf("%.80s", in.Task)),
		port.Int("tools_count", len(in.Tools)))

	result, err := uc.agent.Execute(ctx, port.TaskInput{
		Task:         in.Task,
		Tools:        in.Tools,
		SystemPrompt: in.SystemPrompt,
	}, onEvent)
	if err != nil {
		return TaskExecuteOutput{}, fmt.Errorf("agent execute: %w", err)
	}

	uc.logger.Info("任务完成", port.Int("response_len", len(result.Response)))
	return TaskExecuteOutput{Response: result.Response}, nil
}

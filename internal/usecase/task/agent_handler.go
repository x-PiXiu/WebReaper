package task

import (
	"context"
	"encoding/json"
	"fmt"

	"webreaper/internal/domain/entity"
)

// AgentRunner 是 Agent 执行器接口（供 AgentHandler 依赖，避免 task 包依赖 adapter）。
// main 装配时把 adapter/agent.TrpcAgentRunner 注入实现此接口。
type AgentRunner interface {
	RunTask(ctx context.Context, task string, tools []string, systemPrompt string) (string, error)
}

// AgentHandler 把 Agent 执行包装为 TaskHandler（异步 Agent 任务）。
type AgentHandler struct {
	runner AgentRunner
}

func NewAgentHandler(runner AgentRunner) *AgentHandler {
	return &AgentHandler{runner: runner}
}

func (h *AgentHandler) TaskType() entity.TaskType { return entity.TaskTypeAgentRun }

// agentTaskInput Agent 任务的输入参数。
type agentTaskInput struct {
	Task         string   `json:"task"`
	Tools        []string `json:"tools"`
	SystemPrompt string   `json:"system_prompt"`
}

// Handle 执行 Agent 任务，返回 LLM 的回复文本。
func (h *AgentHandler) Handle(ctx context.Context, inputJSON string) (string, error) {
	var in agentTaskInput
	if err := json.Unmarshal([]byte(inputJSON), &in); err != nil {
		return "", fmt.Errorf("parse agent input: %w", err)
	}
	if in.Task == "" {
		return "", fmt.Errorf("task is required")
	}
	// 默认工具
	if len(in.Tools) == 0 {
		in.Tools = []string{"api_crawler", "static_crawler"}
	}
	result, err := h.runner.RunTask(ctx, in.Task, in.Tools, in.SystemPrompt)
	if err != nil {
		return "", err
	}
	output, _ := json.Marshal(map[string]string{"response": result})
	return string(output), nil
}

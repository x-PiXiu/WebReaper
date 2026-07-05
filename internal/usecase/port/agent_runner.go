package port

import "context"

// AgentRunInput 同步执行 Agent 的输入（用例层 DTO，不依赖 adapter 的 RunInput）。
type AgentRunInput struct {
	Task          string // 用户给 Agent 的任务
	SystemPrompt  string // 系统提示词（空则用默认）
	Tools         []string // 允许的工具（空则用默认全集）
	LLMConfigName string // 指定 LLM 配置（空则 default）
}

// AgentRunOutput 同步执行 Agent 的输出。
type AgentRunOutput struct {
	Response string // Agent 最终回复
}

// AgentSyncRunner 同步执行 Agent 任务的接口（用例层声明，适配器实现）。
//
// 设计动机（DIP）：
//   - HTTP handler 原先依赖 adapter/agent.TrpcAgentRunner 具体 struct，
//     被钉死在 trpc-agent-go 实现上。
//   - 把同步执行能力抽象为 port 接口，handler 依赖接口，main 注入实现，
//     Agent 执行器可替换（如未来换非 trpc-agent-go 的实现）。
//   - 与 task.AgentRunner（异步 RunTask）职责区分：那个给 worker 用，
//     这个给 HTTP 同步端点用。
type AgentSyncRunner interface {
	// RunSync 同步执行 Agent 任务，返回最终回复。
	RunSync(ctx context.Context, in AgentRunInput) (AgentRunOutput, error)
}

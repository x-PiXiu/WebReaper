package port

import "context"

// TaskAgent 是"通用任务执行 Agent"的抽象接口（边界）。
//
// 与 AgentSyncRunner 的区别：
//   - AgentSyncRunner 绑定 AgentConfig（数据库配置的固定提示词/工具），是配置驱动的。
//   - TaskAgent 接受任意任务描述 + 工具列表，是任务驱动的——LLM 自主规划如何完成。
//
// 设计动机（DIP）：
//   - "执行任意任务、自主调工具/子能力直到完成"是应用级能力，归用例层编排。
//   - 具体实现（Explorer ReAct 循环 / 别的 Agent 框架）是易变技术细节，通过本接口隔离。
//   - 换实现（如换 graphagent 多节点编排）时用例零改动。
//
// 自主性：
//   - 实现内部用 ReAct 循环——LLM 自己决定调哪个工具、调几次、何时算完成。
//   - 调用方只给任务描述，不规定执行步骤。
type TaskAgent interface {
	// Execute 执行任意任务，Agent 自主规划直到完成。
	//
	// onEvent 回调透传执行过程中的事件（工具调用、增量文本等），可为 nil。
	// 返回 TaskResult（最终回复 + token 消耗等）。
	Execute(ctx context.Context, in TaskInput, onEvent func(TaskEvent)) (TaskResult, error)
}

// TaskInput 通用任务输入（任意任务，不绑定 AgentConfig）。
type TaskInput struct {
	Task         string   // 任意任务描述（如"采集 Go 招聘信息并总结技能要求"）
	Tools        []string // 允许的工具名（空=全部已注册工具）
	SystemPrompt string   // 系统提示词（空=用默认通用提示词）
}

// TaskResult 任务执行结果。
type TaskResult struct {
	Response string // Agent 的最终回复
}

// TaskEvent 任务执行过程中的事件（与 ToolEvent 同构，独立定义避免耦合）。
type TaskEvent struct {
	Type       string // text-delta / tool-call / tool-result / finish / error
	Text       string // text-delta 时的增量文本
	ToolName   string // tool-call / tool-result 时的工具名
	ToolArgs   string // tool-call 时的参数（JSON）
	ToolResult string // tool-result 时的返回值（截断）
	Error      string // error 时的错误信息
}

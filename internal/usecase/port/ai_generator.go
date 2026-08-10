package port

import "context"

// ChatMessage 是一轮对话消息（通用聊天/Agent 用）。
type ChatMessage struct {
	Role    string // "user" / "assistant" / "system"
	Content string
}

// AIGenerator 是 AI 加工能力的抽象接口（边界）。
type AIGenerator interface {
	// ChatStream 流式对话（无工具），onDelta 回调每段增量。
	// conversationID 是会话隔离的关键：同一会话多轮共享上下文，不同会话完全隔离。
	// 传空时退化为无状态单轮（仅后台编排路径会这样用）。
	// llmConfigName 指定使用哪个 LLMConfig（留空用 "default"）。
	ChatStream(ctx context.Context, conversationID string, llmConfigName string, messages []ChatMessage, onDelta func(delta string)) (string, error)

	// RunWithTools 带工具流式执行（ReAct），通过 onEvent 回调推送所有事件类型。
	// 事件类型：text-delta / tool-call / tool-result / finish / error
	// conversationID 是会话隔离的关键：避免不同会话共享同一个工具对话历史。
	// llmConfigName 指定使用哪个 LLMConfig（留空用 "default"）。
	RunWithTools(ctx context.Context, conversationID string, llmConfigName string, task string, systemPrompt string, tools []string, onEvent func(event ToolEvent)) error
}

// ToolEvent 是工具执行过程中的事件（统一抽象，不依赖框架类型）。
type ToolEvent struct {
	Type     string `json:"type"`      // text-delta / tool-call / tool-result / finish / error
	Text     string `json:"text"`      // text-delta 时的增量文本
	ToolName string `json:"tool_name"` // tool-call / tool-result 时的工具名
	ToolArgs string `json:"tool_args"` // tool-call 时的参数（JSON）
	ToolResult string `json:"tool_result"` // tool-result 时的返回值（截断）
	Error    string `json:"error"`     // error 时的错误信息
}

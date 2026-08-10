package port

import "context"

// ChatMessage 是一轮对话消息（通用聊天/Agent 用）。
type ChatMessage struct {
	Role    string // "user" / "assistant" / "system"
	Content string
}

// ChatOptions 对话控制选项（渐进增强——支持方用，不支持方忽略并降级）。
//
// 设计动机（纵深防御 + 开闭原则）：
//   - think 过滤三层防线：请求参数层（厂商专属，此处声明）→ 提示词层（模型无关）→
//     后置 StripThinkTags（最后防线）。本选项是"请求参数层"的契约。
//   - 结构化输出：让 LLM 按 JSON Schema 输出（标题字段零解析成本），
//     引擎强制格式——从"赌格式"变"控格式"。
type ChatOptions struct {
	// ResponseFormat 输出格式：空=纯文本 / "json"=JSON 结构化输出。
	ResponseFormat string
	// JSONSchema 结构化输出的 JSON Schema（ResponseFormat=json 时生效）。
	// 由调用方声明（如 {"title","content"}），引擎原生强制（DeepSeek 自动降级 json_object）。
	JSONSchema map[string]any
	// SchemaExample 结构化输出的示例对象（框架据此自动推断 schema，与 JSONSchema 二选一）。
	SchemaExample any
	// SchemaDescription 结构化输出描述（提示引擎输出意图）。
	SchemaDescription string
	// DisableThinking 请求层关闭思考过程（厂商支持时：如 DeepSeek enable_thinking=false；
	// 不支持时降级提示词层禁令——由适配器决定）。
	DisableThinking bool
}

// ChatStreamInput 带选项的对话输入（统一封装，避免长参数列表）。
type ChatStreamInput struct {
	ConversationID string
	LLMConfigName  string
	Messages       []ChatMessage
	Options        ChatOptions
	OnDelta        func(delta string)
}

// OptionsAwareGenerator 支持对话控制选项的生成器（可选接口，零破坏增强）。
//
// 用法（与 AutoPublishChannel 同模式——项目已有先例）：
//
//	if gen, ok := ai.(port.OptionsAwareGenerator); ok {
//	    out, err = gen.ChatStreamWithOptions(ctx, in)
//	} else {
//	    out, err = ai.ChatStream(...) // 降级：不支持方走普通对话
//	}
type OptionsAwareGenerator interface {
	ChatStreamWithOptions(ctx context.Context, in ChatStreamInput) (string, error)
}

// ---- 关键词生成结构化输出契约（防随机化）----
// 用例层声明的输出格式，adapter 用 SchemaExample 推断 JSON schema，
// 引擎强制输出后由用例层解析校验（失败降级纯文本路径）。

// KeywordItem 单个候选关键词（term + 搜索意图）。
type KeywordItem struct {
	Term   string `json:"term"`
	Intent string `json:"intent"`
}

// KeywordList 关键词列表结构化输出（蒸馏/生成共用契约）。
type KeywordList struct {
	Keywords []KeywordItem `json:"keywords"`
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

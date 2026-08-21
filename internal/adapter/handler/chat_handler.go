package handler

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/port"
)

// ChatHandler 是 AI 流式聊天的 HTTP 适配器。
//
// 输出 AI SDK 兼容的 SSE 流（UI Message Stream 格式），
// 让前端 useChat hook 能直接消费。
type ChatHandler struct {
	ai        port.AIGenerator
	quotaGate port.QuotaStore // 配额检查门（可选；nil=不检查）
}

func NewChatHandler(ai port.AIGenerator) *ChatHandler {
	return &ChatHandler{ai: ai}
}

// SetQuotaGate 注入配额检查门（可选；未注入时不检查配额——向后兼容）。
// 注入后在 SSE 响应头设置前检查 chat 配额，超限返回 JSON 402（而非流式错误）。
func (h *ChatHandler) SetQuotaGate(g port.QuotaStore) {
	if g != nil {
		h.quotaGate = g
	}
}

// chatRequest 对应前端聊天请求。
type chatRequest struct {
	Messages       []port.ChatMessage `json:"messages"`
	SystemMessage  string             `json:"system_message"`
	Tools          []string           `json:"tools"`            // 工具名列表（空=使用全部已启用工具）
	UseTools       bool               `json:"use_tools"`        // 是否启用工具模式（true=走 RunWithTools）
	LLMConfigName  string             `json:"llm_config_name"`
	ConversationID string             `json:"conversation_id"`
}

// HandleStream POST /api/v1/chat —— SSE 流式响应
//
// 响应格式遵循 AI SDK 的 UIMessageStream 协议（简化版）：
//   data: {"type":"text-delta","textDelta":"..."}     ← 文本增量
//   data: {"type":"finish"}                            ← 结束
func (h *ChatHandler) HandleStream(c *gin.Context) {
	var req chatRequest
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"code": 40000, "msg": "read body failed"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(400, gin.H{"code": 40000, "msg": "invalid json"})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(400, gin.H{"code": 40000, "msg": "no messages"})
		return
	}

	// 配额检查（在 SSE 响应头设置前，超限可返回 JSON 402）
	tenantID := middleware.CurrentTenantID(c)
	if h.quotaGate != nil {
		if err := h.quotaGate.Check(c.Request.Context(), tenantID, "chat"); err != nil {
			fail(c, err)
			return
		}
	}

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	flusher, _ := c.Writer.(interface{ Flush() })

	// 计量挂钩：注入租户 + 场景到 ctx，trpc_agent 的 RecordUsage 据此落库 usages 表。
	ctx = port.WithUsageContext(ctx, tenantID, "chat")
	// 工具租户注入：商户主 Agent 的工具（query_brands/publish_work…）从 ctx 取租户，
	// 不信任 LLM 参数——多租户隔离的安全边界。
	ctx = port.WithToolTenant(ctx, tenantID)

	// 获取最后一条 user 消息（作为 Agent 任务输入）
	lastUser := ""
	for _, msg := range req.Messages {
		if msg.Role == "user" { lastUser = msg.Content }
	}

	// 分流：use_tools=true 或指定了工具名 → RunWithTools（ReAct + 工具调用事件）；否则 → ChatStream（纯对话）
	if (req.UseTools || len(req.Tools) > 0) && lastUser != "" {
		// 带工具模式：通过 SSE 推送所有事件类型
		runErr := h.ai.RunWithTools(ctx, req.ConversationID, req.LLMConfigName, lastUser, req.SystemMessage, req.Tools, func(evt port.ToolEvent) {
			// 把 ToolEvent 直接序列化为 SSE 推送
			writeSSE(c.Writer, evt)
			if flusher != nil { flusher.Flush() }
		})
		if runErr != nil {
			writeSSE(c.Writer, map[string]any{"type": "error", "error": runErr.Error()})
			if flusher != nil { flusher.Flush() }
		}
		return
	}

	// 纯对话模式（无工具）
	messages := req.Messages
	if req.SystemMessage != "" {
		systemMsg := port.ChatMessage{Role: "system", Content: req.SystemMessage}
		messages = append([]port.ChatMessage{systemMsg}, req.Messages...)
	}

	_, streamErr := h.ai.ChatStream(ctx, req.ConversationID, req.LLMConfigName, messages, func(delta string) {
		writeSSEDelta(c.Writer, delta)
		if flusher != nil { flusher.Flush() }
	})

	if streamErr != nil {
		writeSSE(c.Writer, map[string]any{"type": "error", "error": streamErr.Error()})
		if flusher != nil { flusher.Flush() }
		return
	}

	writeSSE(c.Writer, map[string]any{"type": "finish"})
	if flusher != nil { flusher.Flush() }
}

// writeSSEDelta 写一行文本增量（AI SDK text-delta 格式）。
func writeSSEDelta(w io.Writer, delta string) {
	writeSSE(w, map[string]any{"type": "text-delta", "textDelta": delta})
}

// writeSSE 写一行 SSE data: <json>\n\n
func writeSSE(w io.Writer, data any) {
	b, _ := json.Marshal(data)
	// SSE 格式：每行 "data: <content>\n"，消息间用额外 "\n" 分隔
	w.Write([]byte("data: "))
	w.Write(b)
	w.Write([]byte("\n\n"))
}

// 防止 strings 未使用警告（实际在后续可能用到，先保留）
var _ = strings.TrimSpace

package handler

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/port"
)

// ChatHandler 是 AI 流式聊天的 HTTP 适配器。
//
// 输出 AI SDK 兼容的 SSE 流（UI Message Stream 格式），
// 让前端 useChat hook 能直接消费。
type ChatHandler struct {
	ai port.AIGenerator
}

func NewChatHandler(ai port.AIGenerator) *ChatHandler {
	return &ChatHandler{ai: ai}
}

// chatRequest 对应前端聊天请求。
type chatRequest struct {
	Messages      []port.ChatMessage `json:"messages"`
	SystemMessage string             `json:"system_message"`
	Tools         []string           `json:"tools"`          // 可选：带工具时走 ReAct Agent
	LLMConfigName string             `json:"llm_config_name"` // 可选：指定 LLM 配置，留空用 default
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

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	flusher, _ := c.Writer.(interface{ Flush() })

	// 获取最后一条 user 消息（作为 Agent 任务输入）
	lastUser := ""
	for _, msg := range req.Messages {
		if msg.Role == "user" { lastUser = msg.Content }
	}

	// 分流：有 tools → RunWithTools（ReAct + 工具调用事件）；无 tools → ChatStream（纯对话）
	if len(req.Tools) > 0 && lastUser != "" {
		// 带工具模式：通过 SSE 推送所有事件类型
		runErr := h.ai.RunWithTools(ctx, req.LLMConfigName, lastUser, req.SystemMessage, req.Tools, func(evt port.ToolEvent) {
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

	_, streamErr := h.ai.ChatStream(ctx, req.LLMConfigName, messages, func(delta string) {
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

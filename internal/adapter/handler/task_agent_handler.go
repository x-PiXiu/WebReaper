package handler

import (
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/taskagent"
)

// TaskAgentHandler 是通用任务执行的 HTTP 适配器。
//
// 与 AgentHandler（/agents/run，绑定 AgentConfig）的区别：
// 本端点接受任意任务描述，Agent 自主规划完成——不依赖数据库里的 Agent 配置。
type TaskAgentHandler struct {
	uc *taskagent.TaskAgentUseCase
}

func NewTaskAgentHandler(uc *taskagent.TaskAgentUseCase) *TaskAgentHandler {
	return &TaskAgentHandler{uc: uc}
}

// TaskExecuteRequest POST /api/v1/agents/execute[/stream]
type TaskExecuteRequest struct {
	Task         string   `json:"task" binding:"required"` // 任意任务描述
	Tools        []string `json:"tools"`                   // 允许的工具（空=全部）
	SystemPrompt string   `json:"system_prompt"`           // 系统提示词（空=默认）
}

// HandleExecute POST /api/v1/agents/execute —— 通用任务执行（同步返回）
//
// Agent 可能多轮调工具，耗时较长，调用方需设较长超时。
// 需要实时看推理过程的用 /agents/execute/stream（SSE）。
func (h *TaskAgentHandler) HandleExecute(c *gin.Context) {
	var req TaskExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	out, err := h.uc.Execute(c.Request.Context(), taskagent.TaskExecuteInput{
		Task:         req.Task,
		Tools:        req.Tools,
		SystemPrompt: req.SystemPrompt,
	}, nil)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"response": out.Response})
}

// HandleExecuteStream POST /api/v1/agents/execute/stream —— SSE 流式上报任务执行进度
//
// 让用户实时看到 Agent 的推理过程（文本增量、工具调用、工具结果），
// 避免长任务（多轮调工具、图编排）时用户只能干等。
//
// SSE 事件格式（与 /chat 同构，前端可复用消费逻辑）：
//
//	data: {"type":"text-delta","text":"..."}       ← Agent 推理增量
//	data: {"type":"tool-call","tool_name":"..."}    ← 调工具
//	data: {"type":"tool-result","tool_name":"..."}  ← 工具返回
//	data: {"type":"finish"}                         ← 完成
//	data: {"type":"error","error":"..."}            ← 错误
//
// 设计（整洁架构）：SSE 传输机制（header/flusher）属 adapter 技术细节；
// 事件内容 port.TaskEvent 是 usecase 契约。handler 只做序列化+推送。
func (h *TaskAgentHandler) HandleExecuteStream(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"code": 40000, "msg": "read body failed"})
		return
	}
	var req TaskExecuteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(400, gin.H{"code": 40000, "msg": "invalid json"})
		return
	}
	if req.Task == "" {
		c.JSON(400, gin.H{"code": 40000, "msg": "task is required"})
		return
	}

	// 设置 SSE 响应头（复用 chat_handler 的成熟模式）
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // Nginx 不缓冲，保证实时

	flusher, _ := c.Writer.(interface{ Flush() })
	ctx := c.Request.Context()

	// onEvent 回调：把 TaskEvent 序列化为 SSE 推送
	onEvent := func(e port.TaskEvent) {
		// TaskEvent → SSE 事件（字段名对齐前端期望：text/textDelta 等）
		sseEvt := map[string]any{"type": e.Type}
		switch e.Type {
		case "text-delta":
			sseEvt["text"] = e.Text
		case "tool-call":
			sseEvt["tool_name"] = e.ToolName
			sseEvt["tool_args"] = e.ToolArgs
		case "tool-result":
			sseEvt["tool_name"] = e.ToolName
			sseEvt["tool_result"] = e.ToolResult
		case "error":
			sseEvt["error"] = e.Error
		}
		writeSSE(c.Writer, sseEvt)
		if flusher != nil {
			flusher.Flush()
		}
	}

	_, execErr := h.uc.Execute(ctx, taskagent.TaskExecuteInput{
		Task:         req.Task,
		Tools:        req.Tools,
		SystemPrompt: req.SystemPrompt,
	}, onEvent)

	if execErr != nil {
		writeSSE(c.Writer, map[string]any{"type": "error", "error": execErr.Error()})
		if flusher != nil {
			flusher.Flush()
		}
		return
	}

	writeSSE(c.Writer, map[string]any{"type": "finish"})
	if flusher != nil {
		flusher.Flush()
	}
}

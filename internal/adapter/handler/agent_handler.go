package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/port"
)

// AgentHandler 是 Agent 同步执行的 HTTP 适配器（薄 handler）。
//
// 设计要点（DIP）：依赖 port.AgentSyncRunner 接口而非具体 adapter struct，
// Agent 执行器可替换（如未来换非 trpc-agent-go 的实现）。
type AgentHandler struct {
	runner port.AgentSyncRunner
}

func NewAgentHandler(runner port.AgentSyncRunner) *AgentHandler {
	return &AgentHandler{runner: runner}
}

// AgentRunRequest POST /api/v1/agents/run
type AgentRunRequest struct {
	Task         string   `json:"task" binding:"required"`
	SystemPrompt string   `json:"system_prompt"`
	Tools        []string `json:"tools"`
}

// HandleRun POST /api/v1/agents/run —— 同步执行 Agent 任务
func (h *AgentHandler) HandleRun(c *gin.Context) {
	var req AgentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.runner.RunSync(c.Request.Context(), port.AgentRunInput{
		Task:         req.Task,
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"response": out.Response})
}

package handler

import (
	"github.com/gin-gonic/gin"

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

// TaskExecuteRequest POST /api/v1/agents/execute
type TaskExecuteRequest struct {
	Task         string   `json:"task" binding:"required"` // 任意任务描述
	Tools        []string `json:"tools"`                   // 允许的工具（空=全部）
	SystemPrompt string   `json:"system_prompt"`           // 系统提示词（空=默认）
}

// HandleExecute POST /api/v1/agents/execute —— 通用任务执行（Agent 自主规划）
//
// 首版同步返回。Agent 可能多轮调工具，耗时较长，调用方需设较长超时。
// 后续可改 SSE 流式上报进度。
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

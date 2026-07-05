package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/task"
)

// TaskHandler 是异步任务用例的 HTTP 适配器。
// 提供任务投递（异步）和状态查询两个端点。
type TaskHandler struct {
	enqueue *task.EnqueueUseCase
}

func NewTaskHandler(enqueue *task.EnqueueUseCase) *TaskHandler {
	return &TaskHandler{enqueue: enqueue}
}

// EnqueueRequest POST /api/v1/tasks
// type 是任务类型，input 是业务用例的输入参数。
type EnqueueRequest struct {
	Type  string `json:"type" binding:"required"`
	Input any    `json:"input"`
}

// TaskView 任务状态视图。
type TaskView struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Handle POST /api/v1/tasks —— 投递异步任务，立即返回 taskID
func (h *TaskHandler) HandleEnqueue(c *gin.Context) {
	var req EnqueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	out, err := h.enqueue.Execute(c.Request.Context(), task.EnqueueTaskInput{
		Type:  entity.TaskType(req.Type),
		Input: req.Input,
	})
	if err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"task_id": out.TaskID})
}

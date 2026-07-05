package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// EnqueueTaskInput 投递任务的输入。
// Input 会被 JSON 序列化存入 Task.Input，由对应 Handler 反序列化。
type EnqueueTaskInput struct {
	Type  entity.TaskType `json:"type"`  // 任务类型
	Input any             `json:"input"` // 业务用例的输入参数（会被 JSON 序列化）
}

// EnqueueTaskOutput 投递任务的输出。
type EnqueueTaskOutput struct {
	TaskID string `json:"task_id"` // 任务 ID，用于后续查询状态
}

// EnqueueUseCase 任务投递用例。
// 构造 Task → 序列化 Input → 入队 + 持久化 → 立即返回 taskID（不阻塞）。
type EnqueueUseCase struct {
	queue port.TaskQueue
	repo  port.TaskRepository
}

func NewEnqueueUseCase(queue port.TaskQueue, repo port.TaskRepository) *EnqueueUseCase {
	return &EnqueueUseCase{queue: queue, repo: repo}
}

// Execute 投递一个异步任务。
// 调用方（HTTP Handler）投递后立即返回 taskID，实际执行由后台 Worker 异步完成。
func (uc *EnqueueUseCase) Execute(ctx context.Context, in EnqueueTaskInput) (EnqueueTaskOutput, error) {
	if in.Type == "" {
		return EnqueueTaskOutput{}, fmt.Errorf("%w: task type is required", pkg.ErrInvalidArgument)
	}

	// 序列化业务输入
	inputJSON, err := json.Marshal(in.Input)
	if err != nil {
		return EnqueueTaskOutput{}, fmt.Errorf("marshal task input: %w", err)
	}

	now := time.Now()
	task := entity.Task{
		ID:        fmt.Sprintf("task-%d", now.UnixNano()),
		Type:      in.Type,
		Input:     string(inputJSON),
		Status:    valueobject.TaskStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 先持久化（记录任务存在），再入队（触发执行）
	if err := uc.repo.Save(ctx, task); err != nil {
		return EnqueueTaskOutput{}, fmt.Errorf("save task: %w", err)
	}
	if err := uc.queue.Enqueue(ctx, task); err != nil {
		return EnqueueTaskOutput{}, fmt.Errorf("enqueue task: %w", err)
	}

	return EnqueueTaskOutput{TaskID: task.ID}, nil
}

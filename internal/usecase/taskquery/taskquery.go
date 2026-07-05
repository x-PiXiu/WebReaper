// Package taskquery 实现"任务查询"用例。
//
// 职责：任务的查询（按 ID、列表）。
// 任务创建（入队）由 usecase/task.EnqueueUseCase 负责，这里只管读。
//
// 设计动机（整洁架构）：
//   - 原先 handler 直接调 taskRepo.FindByID / List，绕过 usecase。
//   - 现抽出查询用例，handler 只调 usecase，便于将来加缓存/权限等横切逻辑。
package taskquery

import (
	"context"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// TaskQueryUseCase 任务查询用例。
type TaskQueryUseCase struct {
	taskRepo port.TaskRepository
}

func NewTaskQueryUseCase(taskRepo port.TaskRepository) *TaskQueryUseCase {
	return &TaskQueryUseCase{taskRepo: taskRepo}
}

// GetByID 按 ID 查询任务。
func (uc *TaskQueryUseCase) GetByID(ctx context.Context, id string) (entity.Task, error) {
	return uc.taskRepo.FindByID(ctx, id)
}

// List 列出最近的任务。
func (uc *TaskQueryUseCase) List(ctx context.Context, limit int) ([]entity.Task, error) {
	if limit <= 0 {
		limit = 50
	}
	return uc.taskRepo.List(ctx, limit)
}

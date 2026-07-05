package port

import (
	"context"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
)

// TaskRepository 异步任务持久化接口。
type TaskRepository interface {
	Save(ctx context.Context, task entity.Task) error
	FindByID(ctx context.Context, id string) (entity.Task, error)
	List(ctx context.Context, limit int) ([]entity.Task, error)
	UpdateStatus(ctx context.Context, id string, status valueobject.TaskStatus, errMsg string) error
	UpdateOutput(ctx context.Context, id string, output string) error
	UpdateProgress(ctx context.Context, id string, progress string) error // 运行中进度更新
}

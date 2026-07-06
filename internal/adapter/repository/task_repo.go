package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// GormTaskRepository 是 port.TaskRepository 的 GORM 实现。
type GormTaskRepository struct {
	db *gorm.DB
}

// 编译期断言：实现 port.TaskRepository。
var _ port.TaskRepository = (*GormTaskRepository)(nil)

func NewGormTaskRepository(db *gorm.DB) *GormTaskRepository {
	return &GormTaskRepository{db: db}
}

func (r *GormTaskRepository) Save(ctx context.Context, task entity.Task) error {
	po := taskToPO(task)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormTaskRepository) FindByID(ctx context.Context, id string) (entity.Task, error) {
	var po TaskPO
	err := r.db.WithContext(ctx).First(&po, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Task{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.Task{}, err
	}
	return taskFromPO(po), nil
}

func (r *GormTaskRepository) List(ctx context.Context, limit int) ([]entity.Task, error) {
	if limit <= 0 { limit = 50 }
	var pos []TaskPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.Task, 0, len(pos))
	for _, p := range pos { result = append(result, taskFromPO(p)) }
	return result, nil
}

func (r *GormTaskRepository) UpdateStatus(ctx context.Context, id string, status valueobject.TaskStatus, errMsg string) error {
	result := r.db.WithContext(ctx).Model(&TaskPO{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status": string(status),
			"error":  errMsg,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return pkg.ErrNotFound
	}
	return nil
}

func (r *GormTaskRepository) UpdateOutput(ctx context.Context, id string, output string) error {
	result := r.db.WithContext(ctx).Model(&TaskPO{}).Where("id = ?", id).Update("output", output)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return pkg.ErrNotFound
	}
	return nil
}

func (r *GormTaskRepository) UpdateProgress(ctx context.Context, id string, progress string) error {
	// 用 Go 的 time.Now() 而非 gorm.Expr("CURRENT_TIMESTAMP(3)")：
	// 后者是 MySQL 方言（毫秒精度），SQLite 不支持（语法错）。
	// 直接赋值跨数据库兼容，且时间由应用控制更可测。
	result := r.db.WithContext(ctx).Model(&TaskPO{}).Where("id = ?", id).
		Updates(map[string]any{"progress": progress, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return pkg.ErrNotFound
	}
	return nil
}

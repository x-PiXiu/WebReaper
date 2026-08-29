package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// GenerationTaskPO 统一生成任务表映射。
type GenerationTaskPO struct {
	ID               string    `gorm:"primaryKey;size:64"`
	TenantID         string    `gorm:"size:64;index"`
	BrandID          string    `gorm:"size:64"`
	Type             string    `gorm:"size:32"`
	SubType          string    `gorm:"size:64"`
	Model            string    `gorm:"size:64"`
	Provider         string    `gorm:"size:32"`
	ProviderTaskID   string    `gorm:"size:128;index"`
	State            string    `gorm:"size:16;index"`
	ErrCode          string    `gorm:"size:64"`
	ErrMsg           string    `gorm:"size:512"`
	ParamsJSON       string    `gorm:"type:json"`
	Payload          string    `gorm:"size:512"`
	TimelineJSON     string     // B-Roll 台词时间轴（空=未定位）
	CreationsJSON    string    `gorm:"type:json"`
	Credits          int
	OffPeak          bool
	Watermark        bool
	CallbackReceived bool
	CallbackAt       *time.Time
	RetryCount       int
	ParamsHash       string    `gorm:"size:64;index"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	FinishedAt       *time.Time
}

func (GenerationTaskPO) TableName() string { return "generation_tasks" }

// GormGenerationTaskRepository 是 port.GenerationTaskRepository 的 GORM 实现。
type GormGenerationTaskRepository struct {
	db *gorm.DB
}

func NewGormGenerationTaskRepository(db *gorm.DB) *GormGenerationTaskRepository {
	return &GormGenerationTaskRepository{db: db}
}

func generationTaskToPO(t entity.GenerationTask) GenerationTaskPO {
	return GenerationTaskPO{
		ID: t.ID, TenantID: t.TenantID, BrandID: t.BrandID, Type: t.Type, SubType: t.SubType,
		Model: t.Model, Provider: t.Provider, ProviderTaskID: t.ProviderTaskID, State: t.State,
		ErrCode: t.ErrCode, ErrMsg: t.ErrMsg, ParamsJSON: t.ParamsJSON, Payload: t.Payload,
		TimelineJSON: t.TimelineJSON,
		CreationsJSON: t.CreationsJSON, Credits: t.Credits, OffPeak: t.OffPeak,
		Watermark: t.Watermark, CallbackReceived: t.CallbackReceived, CallbackAt: t.CallbackAt,
		RetryCount: t.RetryCount, ParamsHash: t.ParamsHash,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, FinishedAt: t.FinishedAt,
	}
}

func generationTaskFromPO(p GenerationTaskPO) entity.GenerationTask {
	return entity.GenerationTask{
		ID: p.ID, TenantID: p.TenantID, BrandID: p.BrandID, Type: p.Type, SubType: p.SubType,
		Model: p.Model, Provider: p.Provider, ProviderTaskID: p.ProviderTaskID, State: p.State,
		ErrCode: p.ErrCode, ErrMsg: p.ErrMsg, ParamsJSON: p.ParamsJSON, Payload: p.Payload,
		TimelineJSON: p.TimelineJSON,
		CreationsJSON: p.CreationsJSON, Credits: p.Credits, OffPeak: p.OffPeak,
		Watermark: p.Watermark, CallbackReceived: p.CallbackReceived, CallbackAt: p.CallbackAt,
		RetryCount: p.RetryCount, ParamsHash: p.ParamsHash,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, FinishedAt: p.FinishedAt,
	}
}

func (r *GormGenerationTaskRepository) Save(ctx context.Context, t entity.GenerationTask) error {
	po := generationTaskToPO(t)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormGenerationTaskRepository) FindByID(ctx context.Context, tenantID, id string) (entity.GenerationTask, error) {
	var po GenerationTaskPO
	q := r.db.WithContext(ctx).Where("id = ?", id)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.First(&po).Error; err != nil {
		return entity.GenerationTask{}, err
	}
	return generationTaskFromPO(po), nil
}

func (r *GormGenerationTaskRepository) FindByProviderTaskID(ctx context.Context, providerTaskID string) (entity.GenerationTask, error) {
	var po GenerationTaskPO
	if err := r.db.WithContext(ctx).Where("provider_task_id = ?", providerTaskID).First(&po).Error; err != nil {
		return entity.GenerationTask{}, err
	}
	return generationTaskFromPO(po), nil
}

func (r *GormGenerationTaskRepository) FindPendingByHash(ctx context.Context, tenantID, paramsHash string) ([]entity.GenerationTask, error) {
	var pos []GenerationTaskPO
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND params_hash = ? AND state NOT IN ?", tenantID, paramsHash,
			[]string{entity.TaskStateSuccess, entity.TaskStateFailed, entity.TaskStateCancelled}).
		Order("created_at DESC").Limit(5).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationTask, 0, len(pos))
	for _, p := range pos {
		out = append(out, generationTaskFromPO(p))
	}
	return out, nil
}

func (r *GormGenerationTaskRepository) List(ctx context.Context, tenantID string, limit int) ([]entity.GenerationTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var pos []GenerationTaskPO
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("created_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationTask, 0, len(pos))
	for _, p := range pos {
		out = append(out, generationTaskFromPO(p))
	}
	return out, nil
}

func (r *GormGenerationTaskRepository) ListActive(ctx context.Context, limit int) ([]entity.GenerationTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var pos []GenerationTaskPO
	if err := r.db.WithContext(ctx).
		Where("state NOT IN ?", []string{entity.TaskStateSuccess, entity.TaskStateFailed, entity.TaskStateCancelled}).
		Order("created_at ASC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationTask, 0, len(pos))
	for _, p := range pos {
		out = append(out, generationTaskFromPO(p))
	}
	return out, nil
}

// ListFailed 自动重试用：failed 任务按 updated_at 升序（最久未动者优先；
// 可重试分类/退避窗口由用例层判定——仓储只做数据访问）。
func (r *GormGenerationTaskRepository) ListFailed(ctx context.Context, limit int) ([]entity.GenerationTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var pos []GenerationTaskPO
	if err := r.db.WithContext(ctx).
		Where("state = ?", entity.TaskStateFailed).
		Order("updated_at ASC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationTask, 0, len(pos))
	for _, p := range pos {
		out = append(out, generationTaskFromPO(p))
	}
	return out, nil
}

// DeleteTerminalOlderThan 清理早于 before 的终态任务（P3 任务清理策略——避免 generation_tasks 无限增长）。
// 仅删终态（success/failed/cancelled），活跃任务不动。
func (r *GormGenerationTaskRepository) DeleteTerminalOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("state IN ? AND created_at < ?", []string{entity.TaskStateSuccess, entity.TaskStateFailed, entity.TaskStateCancelled}, before).
		Delete(&GenerationTaskPO{})
	return res.RowsAffected, res.Error
}

// Delete 删除单条任务（tenantID 非空时校验归属——租户只能删自己的）。
func (r *GormGenerationTaskRepository) Delete(ctx context.Context, tenantID, taskID string) error {
	q := r.db.WithContext(ctx).Where("id = ?", taskID)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	return q.Delete(&GenerationTaskPO{}).Error
}

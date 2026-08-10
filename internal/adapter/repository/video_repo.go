package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// ---- 视频生成任务仓储（多租户）----

// GormVideoTaskRepository 是 port.VideoTaskRepository 的 GORM 实现。
type GormVideoTaskRepository struct {
	db *gorm.DB
}

func NewGormVideoTaskRepository(db *gorm.DB) *GormVideoTaskRepository {
	return &GormVideoTaskRepository{db: db}
}

func (r *GormVideoTaskRepository) Save(ctx context.Context, t entity.VideoTask) error {
	return r.db.WithContext(ctx).Save(videoTaskToPO(t)).Error
}

func (r *GormVideoTaskRepository) FindByID(ctx context.Context, tenantID, id string) (entity.VideoTask, error) {
	var po VideoTaskPO
	q := r.db.WithContext(ctx)
	if tenantID != "" { // 空 tenantID = 内部驱动（pipeline 后台查自己的任务）
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.VideoTask{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.VideoTask{}, err
	}
	return videoTaskFromPO(po), nil
}

func (r *GormVideoTaskRepository) ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.VideoTask, error) {
	var pos []VideoTaskPO
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.VideoTask, 0, len(pos))
	for _, p := range pos {
		out = append(out, videoTaskFromPO(p))
	}
	return out, nil
}

func (r *GormVideoTaskRepository) UpdateStatus(ctx context.Context, tenantID, id string, status entity.VideoTaskStatus, errMsg string) error {
	return r.db.WithContext(ctx).Model(&VideoTaskPO{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]any{"status": string(status), "error": errMsg, "updated_at": time.Now()}).Error
}

func (r *GormVideoTaskRepository) UpdateResult(ctx context.Context, tenantID, id string, result map[string]any) error {
	cols := map[string]any{"updated_at": time.Now()}
	for k, v := range result {
		cols[k] = v
	}
	return r.db.WithContext(ctx).Model(&VideoTaskPO{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(cols).Error
}

func (r *GormVideoTaskRepository) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&VideoTaskPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// ---- 视频发布任务仓储（多租户）----

// GormVideoJobRepository 是 port.VideoJobRepository 的 GORM 实现。
type GormVideoJobRepository struct {
	db *gorm.DB
}

func NewGormVideoJobRepository(db *gorm.DB) *GormVideoJobRepository {
	return &GormVideoJobRepository{db: db}
}

func (r *GormVideoJobRepository) Save(ctx context.Context, j entity.VideoJob) error {
	return r.db.WithContext(ctx).Save(videoJobToPO(j)).Error
}

func (r *GormVideoJobRepository) FindByID(ctx context.Context, tenantID, id string) (entity.VideoJob, error) {
	var po VideoJobPO
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.VideoJob{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.VideoJob{}, err
	}
	return videoJobFromPO(po), nil
}

func (r *GormVideoJobRepository) ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.VideoJob, error) {
	var pos []VideoJobPO
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.VideoJob, 0, len(pos))
	for _, p := range pos {
		out = append(out, videoJobFromPO(p))
	}
	return out, nil
}

func (r *GormVideoJobRepository) UpdateStatus(ctx context.Context, tenantID, id, status, externalURL, errMsg string) error {
	return r.db.WithContext(ctx).Model(&VideoJobPO{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]any{
			"status": status, "external_url": externalURL, "error": errMsg,
		}).Error
}

// ---- 转换 ----

func videoTaskToPO(t entity.VideoTask) VideoTaskPO {
	return VideoTaskPO{
		ID: t.ID, TenantID: t.TenantID, BrandID: t.BrandID, Mode: t.Mode,
		Prompt: t.Prompt, MaterialURL: t.MaterialURL, Status: string(t.Status),
		VideoURL: t.VideoURL, VoiceText: t.VoiceText, VoiceURL: t.VoiceURL,
		FinalURL: t.FinalURL, DurationSec: t.DurationSec, Error: t.Error,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func videoTaskFromPO(p VideoTaskPO) entity.VideoTask {
	return entity.VideoTask{
		ID: p.ID, TenantID: p.TenantID, BrandID: p.BrandID, Mode: p.Mode,
		Prompt: p.Prompt, MaterialURL: p.MaterialURL, Status: entity.VideoTaskStatus(p.Status),
		VideoURL: p.VideoURL, VoiceText: p.VoiceText, VoiceURL: p.VoiceURL,
		FinalURL: p.FinalURL, DurationSec: p.DurationSec, Error: p.Error,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func videoJobToPO(j entity.VideoJob) VideoJobPO {
	return VideoJobPO{
		ID: j.ID, TenantID: j.TenantID, TaskID: j.TaskID, AccountID: j.AccountID,
		Platform: j.Platform, Status: j.Status, ExternalURL: j.ExternalURL,
		Error: j.Error, CreatedAt: j.CreatedAt,
	}
}

func videoJobFromPO(p VideoJobPO) entity.VideoJob {
	return entity.VideoJob{
		ID: p.ID, TenantID: p.TenantID, TaskID: p.TaskID, AccountID: p.AccountID,
		Platform: p.Platform, Status: p.Status, ExternalURL: p.ExternalURL,
		Error: p.Error, CreatedAt: p.CreatedAt,
	}
}

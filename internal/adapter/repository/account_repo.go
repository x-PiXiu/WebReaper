package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- 发布账号/发布任务仓储的 GORM 实现 ----
// 所有查询强制带 tenant_id 过滤（多租户隔离）。

// ============ AccountRepository ============

type GormAccountRepository struct{ db *gorm.DB }

var _ port.AccountRepository = (*GormAccountRepository)(nil)

func NewGormAccountRepository(db *gorm.DB) *GormAccountRepository {
	return &GormAccountRepository{db: db}
}

func (r *GormAccountRepository) Save(ctx context.Context, a entity.Account) error {
	po := accountToPO(a)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormAccountRepository) FindByID(ctx context.Context, tenantID, id string) (entity.Account, error) {
	var po AccountPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	err := q.Where("id = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Account{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.Account{}, err
	}
	return accountFromPO(po), nil
}

func (r *GormAccountRepository) ListByTenant(ctx context.Context, tenantID string) ([]entity.Account, error) {
	var pos []AccountPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Order("bound_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Account, 0, len(pos))
	for _, p := range pos {
		out = append(out, accountFromPO(p))
	}
	return out, nil
}

func (r *GormAccountRepository) ListByPlatform(ctx context.Context, tenantID, platform string) ([]entity.Account, error) {
	var pos []AccountPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if err := q.Where("platform = ?", platform).Order("bound_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Account, 0, len(pos))
	for _, p := range pos {
		out = append(out, accountFromPO(p))
	}
	return out, nil
}

func (r *GormAccountRepository) Delete(ctx context.Context, tenantID, id string) error {
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	return q.Where("id = ?", id).Delete(&AccountPO{}).Error
}

func (r *GormAccountRepository) ListAll(ctx context.Context) ([]entity.Account, error) {
	var pos []AccountPO
	if err := r.db.WithContext(ctx).Order("bound_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Account, 0, len(pos))
	for _, p := range pos {
		out = append(out, accountFromPO(p))
	}
	return out, nil
}

func (r *GormAccountRepository) UpdateHealth(ctx context.Context, id, health string) error {
	return r.db.WithContext(ctx).Model(&AccountPO{}).Where("id = ?", id).Update("health", health).Error
}

func (r *GormAccountRepository) UpdateLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&AccountPO{}).Where("id = ?", id).Update("last_used_at", lastUsedAt).Error
}

// ============ PublishJobRepository ============

type GormPublishJobRepository struct{ db *gorm.DB }

var _ port.PublishJobRepository = (*GormPublishJobRepository)(nil)

func NewGormPublishJobRepository(db *gorm.DB) *GormPublishJobRepository {
	return &GormPublishJobRepository{db: db}
}

func (r *GormPublishJobRepository) Save(ctx context.Context, j entity.PublishJob) error {
	po := publishJobToPO(j)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormPublishJobRepository) UpdateStatus(ctx context.Context, tenantID, id, status, externalURL, errorMsg string) error {
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	updates := map[string]any{"status": status, "external_url": externalURL, "error_msg": errorMsg}
	return q.Model(&PublishJobPO{}).Where("id = ?", id).Updates(updates).Error
}

func (r *GormPublishJobRepository) ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.PublishJob, error) {
	var pos []PublishJobPO
	q := applyTenantScope(r.db.WithContext(ctx), tenantID)
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.PublishJob, 0, len(pos))
	for _, p := range pos {
		out = append(out, publishJobFromPO(p))
	}
	return out, nil
}

func (r *GormPublishJobRepository) Count(ctx context.Context) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&PublishJobPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

// ListScheduledDue 列出已到期未执行的排期任务（全租户；调度任务扫描用）。
func (r *GormPublishJobRepository) ListScheduledDue(ctx context.Context, before time.Time) ([]entity.PublishJob, error) {
	var pos []PublishJobPO
	if err := r.db.WithContext(ctx).
		Where("status = ? AND scheduled_at > ? AND scheduled_at <= ?",
			entity.PublishStatusPending, time.Time{}, before).
		Order("scheduled_at ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.PublishJob, 0, len(pos))
	for _, p := range pos {
		out = append(out, publishJobFromPO(p))
	}
	return out, nil
}

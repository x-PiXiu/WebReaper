package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// WorkModerationPO 处置表映射（32号）。
type WorkModerationPO struct {
	ID        string `gorm:"primaryKey;size:64"`
	WorkKey   string `gorm:"column:work_key;size:64;uniqueIndex"`
	WorkKind  string `gorm:"size:16;not null;default:video"`
	TenantID  string `gorm:"size:64;not null;default:'';index"`
	Action    string `gorm:"size:16;not null;default:hidden"`
	Reason    string `gorm:"size:512;not null;default:''"`
	Source    string `gorm:"size:16;not null;default:admin"`
	Operator  string `gorm:"size:64;not null;default:''"`
	// 申诉流（32号 P2 终批）
	AppealStatus string     `gorm:"size:16;not null;default:none"`
	AppealText   string     `gorm:"size:512;not null;default:''"`
	AppealedAt   *time.Time `gorm:"column:appealed_at"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (WorkModerationPO) TableName() string { return "work_moderations" }

// GormWorkModerationRepository 是 port.WorkModerationRepository 的 GORM 实现。
type GormWorkModerationRepository struct {
	db *gorm.DB
}

func NewGormWorkModerationRepository(db *gorm.DB) *GormWorkModerationRepository {
	return &GormWorkModerationRepository{db: db}
}

func poToModeration(p WorkModerationPO) entity.WorkModeration {
	return entity.WorkModeration{
		ID: p.ID, WorkKey: p.WorkKey, WorkKind: p.WorkKind, TenantID: p.TenantID,
		Action: p.Action, Reason: p.Reason, Source: p.Source, Operator: p.Operator,
		AppealStatus: p.AppealStatus, AppealText: p.AppealText, AppealedAt: p.AppealedAt,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (r *GormWorkModerationRepository) FindByKey(ctx context.Context, workKey string) (entity.WorkModeration, error) {
	var po WorkModerationPO
	if err := r.db.WithContext(ctx).Where("work_key = ?", workKey).First(&po).Error; err != nil {
		return entity.WorkModeration{}, err
	}
	return poToModeration(po), nil
}

func (r *GormWorkModerationRepository) Upsert(ctx context.Context, m entity.WorkModeration) error {
	po := WorkModerationPO{
		ID: m.ID, WorkKey: m.WorkKey, WorkKind: m.WorkKind, TenantID: m.TenantID,
		Action: m.Action, Reason: m.Reason, Source: m.Source, Operator: m.Operator,
		AppealStatus: m.AppealStatus, AppealText: m.AppealText, AppealedAt: m.AppealedAt,
	}
	return r.db.WithContext(ctx).Save(&po).Error
}

// UpdateAppeal 申诉状态定向更新（不动处置主字段——申诉与处置生命周期分离）。
func (r *GormWorkModerationRepository) UpdateAppeal(ctx context.Context, workKey, status, text string, appealedAt *time.Time) error {
	return r.db.WithContext(ctx).Model(&WorkModerationPO{}).
		Where("work_key = ?", workKey).
		Updates(map[string]any{
			"appeal_status": status,
			"appeal_text":   text,
			"appealed_at":   appealedAt,
		}).Error
}

func (r *GormWorkModerationRepository) Delete(ctx context.Context, workKey string) error {
	return r.db.WithContext(ctx).Where("work_key = ?", workKey).Delete(&WorkModerationPO{}).Error
}

func (r *GormWorkModerationRepository) ListByTenant(ctx context.Context, tenantID string) ([]entity.WorkModeration, error) {
	var pos []WorkModerationPO
	// 含 flagged（机审待复核）——用户端产物隔离：违规内容管理员放行前显示"审核中"
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND action IN ?", tenantID,
			[]string{entity.WorkActionHidden, entity.WorkActionDeleted, entity.WorkActionFlagged}).
		Order("updated_at DESC").Limit(500).Find(&pos).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]entity.WorkModeration, 0, len(pos))
	for _, p := range pos {
		out = append(out, poToModeration(p))
	}
	return out, nil
}

func (r *GormWorkModerationRepository) ListRecent(ctx context.Context, limit int) ([]entity.WorkModeration, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var pos []WorkModerationPO
	if err := r.db.WithContext(ctx).Order("updated_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.WorkModeration, 0, len(pos))
	for _, p := range pos {
		out = append(out, poToModeration(p))
	}
	return out, nil
}

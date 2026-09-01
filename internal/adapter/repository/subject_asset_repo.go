package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"webreaper/internal/domain/entity"
)

// SubjectAssetPO 主体资产表映射。
type SubjectAssetPO struct {
	ID             string `gorm:"primaryKey;size:64"`
	TenantID       string `gorm:"size:64;index:idx_tenant_scope,priority:1"`
	Scope          string `gorm:"size:16;index:idx_tenant_scope,priority:2"`
	Kind           string `gorm:"size:16;index:idx_tenant_scope,priority:3"`
	Name           string `gorm:"size:128"`
	ServerID       string `gorm:"size:128;uniqueIndex:uk_server"`
	PortraitURL    string `gorm:"size:512"`
	AvatarVideoURL string `gorm:"size:1024"` // 089：Vidu签名URL可达~900字符（512会Data too long）
	VoiceID        string `gorm:"size:128"`
	Tags           string `gorm:"size:512"`
	SortOrder      int
	Status         string `gorm:"size:16;index:idx_tenant_scope,priority:4"`
	SourceTaskID   string `gorm:"size:64"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (SubjectAssetPO) TableName() string { return "subject_assets" }

func subjectAssetToPO(a entity.SubjectAsset) SubjectAssetPO {
	return SubjectAssetPO{
		ID: a.ID, TenantID: a.TenantID, Scope: a.Scope, Kind: a.Kind,
		Name: a.Name, ServerID: a.ServerID, PortraitURL: a.PortraitURL,
		AvatarVideoURL: a.AvatarVideoURL, VoiceID: a.VoiceID, Tags: a.Tags,
		SortOrder: a.SortOrder, Status: a.Status, SourceTaskID: a.SourceTaskID,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func subjectAssetFromPO(p SubjectAssetPO) entity.SubjectAsset {
	return entity.SubjectAsset{
		ID: p.ID, TenantID: p.TenantID, Scope: p.Scope, Kind: p.Kind,
		Name: p.Name, ServerID: p.ServerID, PortraitURL: p.PortraitURL,
		AvatarVideoURL: p.AvatarVideoURL, VoiceID: p.VoiceID, Tags: p.Tags,
		SortOrder: p.SortOrder, Status: p.Status, SourceTaskID: p.SourceTaskID,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// GormSubjectAssetRepository 是 port.SubjectAssetRepository 的 GORM 实现。
type GormSubjectAssetRepository struct {
	db *gorm.DB
}

func NewGormSubjectAssetRepository(db *gorm.DB) *GormSubjectAssetRepository {
	return &GormSubjectAssetRepository{db: db}
}

// Upsert 按 server_id 唯一键幂等写入（INSERT ... ON DUPLICATE KEY UPDATE）。
// 冲突更新列含 scope/tenant_id/sort_order：管理后台创建官方主体与终态物化钩子
// 竞争同 server_id（物化先写 personal 行），后写方（admin official）必须能改写归属。
func (r *GormSubjectAssetRepository) Upsert(ctx context.Context, asset entity.SubjectAsset) error {
	po := subjectAssetToPO(asset)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"scope", "tenant_id", "name", "portrait_url", "avatar_video_url", "voice_id", "tags", "sort_order", "status", "updated_at"}),
	}).Create(&po).Error
}

func (r *GormSubjectAssetRepository) FindByID(ctx context.Context, id string) (entity.SubjectAsset, error) {
	var po SubjectAssetPO
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		return entity.SubjectAsset{}, err
	}
	return subjectAssetFromPO(po), nil
}

func (r *GormSubjectAssetRepository) FindByServerID(ctx context.Context, serverID string) (entity.SubjectAsset, error) {
	var po SubjectAssetPO
	if err := r.db.WithContext(ctx).Where("server_id = ?", serverID).First(&po).Error; err != nil {
		return entity.SubjectAsset{}, err
	}
	return subjectAssetFromPO(po), nil
}

func (r *GormSubjectAssetRepository) ListByTenant(ctx context.Context, tenantID, scope, kind string, limit, offset int) ([]entity.SubjectAsset, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	q = q.Where("status = ?", entity.SubjectStatusActive)

	var total int64
	q.Model(&SubjectAssetPO{}).Count(&total)

	var pos []SubjectAssetPO
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]entity.SubjectAsset, 0, len(pos))
	for _, p := range pos {
		out = append(out, subjectAssetFromPO(p))
	}
	return out, total, nil
}

func (r *GormSubjectAssetRepository) UpdateAvatarVideoURL(ctx context.Context, serverID, avatarVideoURL string) error {
	return r.db.WithContext(ctx).
		Model(&SubjectAssetPO{}).
		Where("server_id = ?", serverID).
		Updates(map[string]any{"avatar_video_url": avatarVideoURL, "updated_at": time.Now()}).
		Error
}

func (r *GormSubjectAssetRepository) UpdateStatus(ctx context.Context, id, status string) error {
	return r.db.WithContext(ctx).
		Model(&SubjectAssetPO{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).
		Error
}

func (r *GormSubjectAssetRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&SubjectAssetPO{}).Error
}

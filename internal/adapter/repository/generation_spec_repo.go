package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// GenerationSpecPO 端点/模型规格表映射。
type GenerationSpecPO struct {
	SubType          string    `gorm:"primaryKey;size:64"`
	Model            string    `gorm:"primaryKey;size:64"`
	Provider         string    `gorm:"size:50;default:vidu"`
	Endpoint         string    `gorm:"size:128"`
	Enabled          bool      `gorm:"default:1"`
	IsDefault        bool      `gorm:"default:0"`
	SortOrder        int       `gorm:"default:0"`
	CapabilitiesJSON string    `gorm:"type:text"`
	CostCredits      int       `gorm:"default:0"` // 27 号：每次调用消耗积分（0=使用服务商返回值）
	UpdatedAt        time.Time
}

func (GenerationSpecPO) TableName() string { return "generation_specs" }

// GormGenerationSpecRepository 是 port.GenerationSpecRepository 的 GORM 实现。
type GormGenerationSpecRepository struct {
	db *gorm.DB
}

func NewGormGenerationSpecRepository(db *gorm.DB) *GormGenerationSpecRepository {
	return &GormGenerationSpecRepository{db: db}
}

func (r *GormGenerationSpecRepository) ListAll(ctx context.Context) ([]entity.GenerationSpec, error) {
	var pos []GenerationSpecPO
	if err := r.db.WithContext(ctx).Order("sub_type, model").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationSpec, 0, len(pos))
	for _, p := range pos {
		out = append(out, generationSpecFromPO(p))
	}
	return out, nil
}

func (r *GormGenerationSpecRepository) Find(ctx context.Context, subType, model string) (entity.GenerationSpec, error) {
	var po GenerationSpecPO
	if err := r.db.WithContext(ctx).Where("sub_type = ? AND model = ?", subType, model).First(&po).Error; err != nil {
		return entity.GenerationSpec{}, err
	}
	return generationSpecFromPO(po), nil
}

func (r *GormGenerationSpecRepository) Upsert(ctx context.Context, spec entity.GenerationSpec) error {
	po := GenerationSpecPO{
		SubType: spec.SubType, Model: spec.Model, Provider: spec.Provider,
		Endpoint: spec.Endpoint, Enabled: spec.Enabled, IsDefault: spec.IsDefault,
		SortOrder: spec.SortOrder, CapabilitiesJSON: spec.CapabilitiesJSON,
		CostCredits: spec.CostCredits, UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormGenerationSpecRepository) Delete(ctx context.Context, subType, model string) error {
	return r.db.WithContext(ctx).Where("sub_type = ? AND model = ?", subType, model).Delete(&GenerationSpecPO{}).Error
}

// FindDefaultModel 查找默认模型（按厂商+端点）。
func (r *GormGenerationSpecRepository) FindDefaultModel(ctx context.Context, provider, subType string) (entity.GenerationSpec, error) {
	var po GenerationSpecPO
	err := r.db.WithContext(ctx).
		Where("provider = ? AND sub_type = ? AND is_default = ? AND enabled = ?", provider, subType, true, true).
		First(&po).Error
	if err != nil {
		return entity.GenerationSpec{}, err
	}
	return generationSpecFromPO(po), nil
}

// ListByProvider 按厂商查询（管理后台用）。
func (r *GormGenerationSpecRepository) ListByProvider(ctx context.Context, provider string) ([]entity.GenerationSpec, error) {
	var pos []GenerationSpecPO
	if err := r.db.WithContext(ctx).Where("provider = ?", provider).Order("sub_type, sort_order, model").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.GenerationSpec, 0, len(pos))
	for _, p := range pos {
		out = append(out, generationSpecFromPO(p))
	}
	return out, nil
}

// SetDefault 设置默认模型（取消同端点其他模型的默认标记）。
func (r *GormGenerationSpecRepository) SetDefault(ctx context.Context, provider, subType, model string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 取消同端点其他模型的默认标记
		if err := tx.Model(&GenerationSpecPO{}).
			Where("provider = ? AND sub_type = ?", provider, subType).
			Update("is_default", false).Error; err != nil {
			return err
		}
		// 2. 设置目标模型为默认
		return tx.Model(&GenerationSpecPO{}).
			Where("provider = ? AND sub_type = ? AND model = ?", provider, subType, model).
			Update("is_default", true).Error
	})
}

func generationSpecFromPO(p GenerationSpecPO) entity.GenerationSpec {
	return entity.GenerationSpec{
		SubType: p.SubType, Model: p.Model, Provider: p.Provider,
		Endpoint: p.Endpoint, Enabled: p.Enabled, IsDefault: p.IsDefault,
		SortOrder: p.SortOrder, CapabilitiesJSON: p.CapabilitiesJSON,
		CostCredits: p.CostCredits, UpdatedAt: p.UpdatedAt,
	}
}

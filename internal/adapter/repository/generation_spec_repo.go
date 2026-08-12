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
	Endpoint         string    `gorm:"size:128"`
	Enabled          bool      `gorm:"default:1"`
	CapabilitiesJSON string    `gorm:"type:text"`
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
		SubType: spec.SubType, Model: spec.Model, Endpoint: spec.Endpoint,
		Enabled: spec.Enabled, CapabilitiesJSON: spec.CapabilitiesJSON,
		UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormGenerationSpecRepository) Delete(ctx context.Context, subType, model string) error {
	return r.db.WithContext(ctx).Where("sub_type = ? AND model = ?", subType, model).Delete(&GenerationSpecPO{}).Error
}

func generationSpecFromPO(p GenerationSpecPO) entity.GenerationSpec {
	return entity.GenerationSpec{
		SubType: p.SubType, Model: p.Model, Endpoint: p.Endpoint,
		Enabled: p.Enabled, CapabilitiesJSON: p.CapabilitiesJSON, UpdatedAt: p.UpdatedAt,
	}
}

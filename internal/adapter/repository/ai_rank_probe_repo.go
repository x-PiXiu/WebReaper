package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// AIRankProbePO AI 榜单探查结果表映射。
type AIRankProbePO struct {
	ID          string         `gorm:"primaryKey;size:64"`
	TenantID    string         `gorm:"size:64;index"`
	BrandID     string         `gorm:"size:64;index"`
	Results     datatypes.JSON `gorm:"type:json"`
	SampleCount int
	ProbedAt    time.Time `gorm:"index"`
	ExpireAt    time.Time
}

func (AIRankProbePO) TableName() string { return "geo_ai_rank_probes" }

// GormAIRankProbeRepository 是 port.AIRankProbeRepository 的 GORM 实现。
type GormAIRankProbeRepository struct {
	db *gorm.DB
}

func NewGormAIRankProbeRepository(db *gorm.DB) *GormAIRankProbeRepository {
	return &GormAIRankProbeRepository{db: db}
}

func (r *GormAIRankProbeRepository) Save(ctx context.Context, e entity.AIRankProbeResult) error {
	b, err := json.Marshal(e.Results)
	if err != nil {
		return err
	}
	po := AIRankProbePO{
		ID:          "airp-" + e.BrandID + "-" + e.ProbedAt.Format("20060102150405"),
		TenantID:    e.TenantID,
		BrandID:     e.BrandID,
		Results:     datatypes.JSON(b),
		SampleCount: e.SampleCount,
		ProbedAt:    e.ProbedAt,
		ExpireAt:    e.ExpireAt,
	}
	// 主键含时间戳——同品牌多次探查生成不同 ID（保留历史），查询取最新一条即可。
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormAIRankProbeRepository) Latest(ctx context.Context, tenantID, brandID string) (entity.AIRankProbeResult, error) {
	var po AIRankProbePO
	if err := r.db.WithContext(ctx).
		Where("brand_id = ?", brandID).
		Order("probed_at DESC").
		First(&po).Error; err != nil {
		return entity.AIRankProbeResult{}, err
	}
	var items []entity.AIRankProbeItem
	_ = json.Unmarshal(po.Results, &items)
	return entity.AIRankProbeResult{
		TenantID:    po.TenantID,
		BrandID:     po.BrandID,
		Results:     items,
		SampleCount: po.SampleCount,
		ProbedAt:    po.ProbedAt,
		ExpireAt:    po.ExpireAt,
	}, nil
}

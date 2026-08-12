package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// ProviderConfigPO 厂商配置表映射（041_provider_configs.sql）。
type ProviderConfigPO struct {
	Provider  string    `gorm:"primaryKey;size:32"`
	APIKey    string    `gorm:"size:512"`
	BaseURL   string    `gorm:"size:256"`
	Enabled   bool      `gorm:"default:1"`
	ExtraJSON string    `gorm:"type:text"`
	UpdatedAt time.Time
}

func (ProviderConfigPO) TableName() string { return "provider_configs" }

// GormProviderConfigRepository 是 port.ProviderConfigRepository 的 GORM 实现。
type GormProviderConfigRepository struct {
	db *gorm.DB
}

func NewGormProviderConfigRepository(db *gorm.DB) *GormProviderConfigRepository {
	return &GormProviderConfigRepository{db: db}
}

func (r *GormProviderConfigRepository) List(ctx context.Context) ([]entity.ProviderConfig, error) {
	var pos []ProviderConfigPO
	if err := r.db.WithContext(ctx).Order("provider").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.ProviderConfig, 0, len(pos))
	for _, p := range pos {
		out = append(out, providerConfigFromPO(p))
	}
	return out, nil
}

func (r *GormProviderConfigRepository) Get(ctx context.Context, provider string) (entity.ProviderConfig, error) {
	// 用 Find 而非 First：找不到返回零值 + nil（避免 GORM record-not-found 噪音日志——
	// 装配时厂商未配置是正常态，调用方按 APIKey == "" 判断）
	var po ProviderConfigPO
	if err := r.db.WithContext(ctx).Where("provider = ?", provider).Limit(1).Find(&po).Error; err != nil {
		return entity.ProviderConfig{}, err
	}
	return providerConfigFromPO(po), nil
}

func (r *GormProviderConfigRepository) Upsert(ctx context.Context, cfg entity.ProviderConfig) error {
	po := providerConfigToPO(cfg)
	// 非空字段更新（api_key 为空 = 不修改——前端掩码语义）
	updates := map[string]any{"base_url": po.BaseURL, "enabled": po.Enabled, "extra_json": po.ExtraJSON}
	if po.APIKey != "" {
		updates["api_key"] = po.APIKey
	}
	return r.db.WithContext(ctx).
		Where("provider = ?", po.Provider).
		Assign(updates).
		FirstOrCreate(&po).Error
}

func providerConfigFromPO(p ProviderConfigPO) entity.ProviderConfig {
	return entity.ProviderConfig{
		Provider:  p.Provider,
		APIKey:    p.APIKey,
		BaseURL:   p.BaseURL,
		Enabled:   p.Enabled,
		ExtraJSON: p.ExtraJSON,
		UpdatedAt: p.UpdatedAt,
	}
}

func providerConfigToPO(c entity.ProviderConfig) ProviderConfigPO {
	return ProviderConfigPO{
		Provider:  c.Provider,
		APIKey:    c.APIKey,
		BaseURL:   c.BaseURL,
		Enabled:   c.Enabled,
		ExtraJSON: c.ExtraJSON,
	}
}

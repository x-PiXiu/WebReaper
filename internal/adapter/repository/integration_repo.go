package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"webreaper/internal/domain/entity"
)

// ---- PO 定义 ----

type IntegrationVendorPO struct {
	ID        string    `gorm:"primaryKey;size:64"`
	Name      string    `gorm:"size:128"`
	BaseURL   string    `gorm:"column:base_url;size:512"`
	APIKey    string    `gorm:"column:api_key;size:512"`
	Protocol  string    `gorm:"size:32;default:'openai'"`
	Enabled   bool      `gorm:"default:1"`
	UpdatedAt time.Time
}

func (IntegrationVendorPO) TableName() string { return "integration_vendors" }

type IntegrationCapabilityPO struct {
	ID        string `gorm:"primaryKey;size:128"`
	CapID     string `gorm:"column:cap_id;size:64;index"`
	VendorID  string `gorm:"column:vendor_id;size:64;index"`
	Endpoint  string `gorm:"size:512"`
	Model     string `gorm:"size:128"`
	IsDefault bool   `gorm:"column:is_default;default:0"`
	Enabled   bool   `gorm:"default:1"`
	ExtraJSON string `gorm:"type:text"`
	UpdatedAt time.Time
}

func (IntegrationCapabilityPO) TableName() string { return "integration_capabilities" }

// ---- 仓储实现 ----

type GormIntegrationRepository struct {
	db *gorm.DB
}

func NewGormIntegrationRepository(db *gorm.DB) *GormIntegrationRepository {
	return &GormIntegrationRepository{db: db}
}

func (r *GormIntegrationRepository) SaveVendor(ctx context.Context, v entity.IntegrationVendor) error {
	po := IntegrationVendorPO{
		ID: v.ID, Name: v.Name, BaseURL: v.BaseURL, APIKey: v.APIKey,
		Protocol: v.Protocol, Enabled: v.Enabled, UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormIntegrationRepository) SaveCapability(ctx context.Context, c entity.IntegrationCapability) error {
	po := IntegrationCapabilityPO{
		ID: c.ID, CapID: c.CapID, VendorID: c.VendorID, Endpoint: c.Endpoint,
		Model: c.Model, IsDefault: c.IsDefault, Enabled: c.Enabled,
		ExtraJSON: c.ExtraJSON, UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormIntegrationRepository) ListVendors(ctx context.Context) ([]entity.IntegrationVendor, error) {
	var pos []IntegrationVendorPO
	if err := r.db.WithContext(ctx).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.IntegrationVendor, 0, len(pos))
	for _, p := range pos {
		out = append(out, vendorFromPO(p))
	}
	return out, nil
}

func (r *GormIntegrationRepository) ListCapabilities(ctx context.Context) ([]entity.IntegrationCapability, error) {
	var pos []IntegrationCapabilityPO
	if err := r.db.WithContext(ctx).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.IntegrationCapability, 0, len(pos))
	for _, p := range pos {
		out = append(out, capabilityFromPO(p))
	}
	return out, nil
}

// ResolveDefault 按能力 ID 取当前生效配置（IsDefault + Enabled + Vendor Enabled）。
func (r *GormIntegrationRepository) ResolveDefault(ctx context.Context, capID string) (entity.ResolvedCap, error) {
	var cap IntegrationCapabilityPO
	err := r.db.WithContext(ctx).
		Where("cap_id = ? AND is_default = 1 AND enabled = 1", capID).
		First(&cap).Error
	if err != nil {
		return entity.ResolvedCap{}, err
	}
	var vendor IntegrationVendorPO
	if err := r.db.WithContext(ctx).Where("id = ? AND enabled = 1", cap.VendorID).First(&vendor).Error; err != nil {
		return entity.ResolvedCap{}, err
	}
	endpoint := cap.Endpoint
	if endpoint == "" {
		endpoint = strings.TrimRight(vendor.BaseURL, "/")
	} else {
		// endpoint 是相对路径时拼接 vendor base_url（TrimRight 防双斜杠）
		if len(endpoint) > 0 && endpoint[0] == '/' && vendor.BaseURL != "" {
			endpoint = strings.TrimRight(vendor.BaseURL, "/") + endpoint
		}
	}
	return entity.ResolvedCap{
		VendorID:  vendor.ID,
		BaseURL:   vendor.BaseURL,
		APIKey:    vendor.APIKey,
		Protocol:  vendor.Protocol,
		Endpoint:  endpoint,
		Model:     cap.Model,
		ExtraJSON: cap.ExtraJSON,
	}, nil
}

// SetDefault 设置默认能力（同 CapID 下互斥——事务内先清除再设置）。
func (r *GormIntegrationRepository) SetDefault(ctx context.Context, capID, vendorID string) error {
	id := capID + "#" + vendorID
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 清除同 CapID 下所有默认
		if err := tx.Model(&IntegrationCapabilityPO{}).
			Where("cap_id = ? AND is_default = 1", capID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		// 设置目标
		return tx.Model(&IntegrationCapabilityPO{}).
			Where("id = ?", id).Update("is_default", true).Error
	})
}

// SeedIfEmpty 能力表空时写入种子数据（首次启动或旧表迁移后能力表仍空时）。
// 检查 capabilities 而非 vendors——旧表迁移可能已写入 vendor 行但无 capability 行。
// vendor 用 INSERT IGNORE（不覆盖迁移写入的 Key/启用状态）。
func (r *GormIntegrationRepository) SeedIfEmpty(ctx context.Context, vendors []entity.IntegrationVendor, caps []entity.IntegrationCapability) (int, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&IntegrationCapabilityPO{}).Count(&n).Error; err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, nil
	}
	// Vendor：INSERT IGNORE（保留迁移写入的 Key/状态）
	for _, v := range vendors {
		po := IntegrationVendorPO{
			ID: v.ID, Name: v.Name, BaseURL: v.BaseURL, APIKey: v.APIKey,
			Protocol: v.Protocol, Enabled: v.Enabled, UpdatedAt: time.Now(),
		}
		_ = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po).Error
	}
	for _, c := range caps {
		if err := r.SaveCapability(ctx, c); err != nil {
			return 0, err
		}
	}
	return len(vendors), nil
}

// DeleteCapability 删除能力条目。
func (r *GormIntegrationRepository) DeleteCapability(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&IntegrationCapabilityPO{}).Error
}

func vendorFromPO(p IntegrationVendorPO) entity.IntegrationVendor {
	return entity.IntegrationVendor{
		ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, APIKey: p.APIKey,
		Protocol: p.Protocol, Enabled: p.Enabled, UpdatedAt: p.UpdatedAt,
	}
}

func capabilityFromPO(p IntegrationCapabilityPO) entity.IntegrationCapability {
	return entity.IntegrationCapability{
		ID: p.ID, CapID: p.CapID, VendorID: p.VendorID, Endpoint: p.Endpoint,
		Model: p.Model, IsDefault: p.IsDefault, Enabled: p.Enabled,
		ExtraJSON: p.ExtraJSON, UpdatedAt: p.UpdatedAt,
	}
}

var _ = (*GormIntegrationRepository)(nil) // 编译期断言

package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// GormTenantSettingRepository port.TenantSettingRepository 的 GORM 实现。
type GormTenantSettingRepository struct {
	db *gorm.DB
}

func NewGormTenantSettingRepository(db *gorm.DB) *GormTenantSettingRepository {
	return &GormTenantSettingRepository{db: db}
}

func (r *GormTenantSettingRepository) Get(ctx context.Context, tenantID, key string) (entity.TenantSetting, error) {
	var po TenantSettingPO
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND setting_key = ?", tenantID, key).
		First(&po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.TenantSetting{}, pkg.ErrNotFound
		}
		return entity.TenantSetting{}, err
	}
	return entity.TenantSetting{
		TenantID: po.TenantID, Key: po.SettingKey,
		Value: po.Value, UpdatedAt: po.UpdatedAt,
	}, nil
}

func (r *GormTenantSettingRepository) Save(ctx context.Context, setting entity.TenantSetting) error {
	if setting.UpdatedAt.IsZero() {
		setting.UpdatedAt = time.Now()
	}
	po := TenantSettingPO{
		TenantID: setting.TenantID, SettingKey: setting.Key,
		Value: setting.Value, UpdatedAt: setting.UpdatedAt,
	}
	// upsert：主键冲突时更新 value/updated_at
	return r.db.WithContext(ctx).Save(&po).Error
}

// TenantSettingPO 租户设置持久化对象（tenant_settings 表，AutoMigrate 建表）。
type TenantSettingPO struct {
	TenantID  string    `gorm:"primaryKey;size:64"`
	SettingKey string   `gorm:"primaryKey;size:64"`
	Value     string    `gorm:"type:text"`
	UpdatedAt time.Time
}

func (TenantSettingPO) TableName() string { return "tenant_settings" }

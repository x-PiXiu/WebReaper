package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// GormSystemSettingRepository 是 port.SystemSettingRepository 的 GORM 实现。
type GormSystemSettingRepository struct{ db *gorm.DB }

// 编译期断言。
var _ port.SystemSettingRepository = (*GormSystemSettingRepository)(nil)

func NewGormSystemSettingRepository(db *gorm.DB) *GormSystemSettingRepository {
	return &GormSystemSettingRepository{db: db}
}

func (r *GormSystemSettingRepository) Get(ctx context.Context, key string) (entity.SystemSetting, error) {
	var po SystemSettingPO
	// 注意：用 PO 字段名（Key）做条件，让 GORM 自动映射到列 setting_key。
	// 不能手写 "key = ?"——key 是 MySQL 保留字，且实际列名是 setting_key，
	// 手写片段不会被 GORM 转译，会生成非法 SQL（Error 1064）。
	err := r.db.WithContext(ctx).Where(&SystemSettingPO{Key: key}).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.SystemSetting{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.SystemSetting{}, err
	}
	return entity.SystemSetting{Key: po.Key, Value: po.Value, UpdatedAt: po.UpdatedAt}, nil
}

func (r *GormSystemSettingRepository) Save(ctx context.Context, setting entity.SystemSetting) error {
	po := SystemSettingPO{Key: setting.Key, Value: setting.Value, UpdatedAt: setting.UpdatedAt}
	return r.db.WithContext(ctx).Save(&po).Error
}

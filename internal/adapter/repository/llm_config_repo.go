package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// GormLLMConfigRepository 是 port.LLMConfigRepository 的 GORM 实现。
type GormLLMConfigRepository struct{ db *gorm.DB }

// 编译期断言：实现 port.LLMConfigRepository。
var _ port.LLMConfigRepository = (*GormLLMConfigRepository)(nil)

func NewGormLLMConfigRepository(db *gorm.DB) *GormLLMConfigRepository {
	return &GormLLMConfigRepository{db: db}
}

func (r *GormLLMConfigRepository) Save(ctx context.Context, cfg entity.LLMConfig) error {
	po := llmConfigToPO(cfg)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormLLMConfigRepository) FindByName(ctx context.Context, name string) (entity.LLMConfig, error) {
	var po LLMConfigPO
	err := r.db.WithContext(ctx).First(&po, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.LLMConfig{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.LLMConfig{}, err
	}
	return llmConfigFromPO(po), nil
}

func (r *GormLLMConfigRepository) List(ctx context.Context) ([]entity.LLMConfig, error) {
	var pos []LLMConfigPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.LLMConfig, 0, len(pos))
	for _, p := range pos {
		result = append(result, llmConfigFromPO(p))
	}
	return result, nil
}

func (r *GormLLMConfigRepository) Delete(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Where("name = ?", name).Delete(&LLMConfigPO{}).Error
}

// FindByUsage 按用途标签查找配置（usage="" = 聊天模型；"vision" = 视觉模型）。
// 返回第一条匹配记录——多条同用途配置时取最新创建的。
func (r *GormLLMConfigRepository) FindByUsage(ctx context.Context, usage string) (entity.LLMConfig, error) {
	var po LLMConfigPO
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if usage == "" {
		q = q.Where("usage = '' OR usage IS NULL")
	} else {
		q = q.Where("usage = ?", usage)
	}
	err := q.First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.LLMConfig{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.LLMConfig{}, err
	}
	return llmConfigFromPO(po), nil
}

// SetDefault 设置默认模型（同 Usage 下互斥——先清除同 Usage 的其他默认，再设置目标）。
func (r *GormLLMConfigRepository) SetDefault(ctx context.Context, name string) error {
	// 先找到目标配置的 Usage
	target, err := r.FindByName(ctx, name)
	if err != nil {
		return err
	}
	// 清除同 Usage 下所有默认标记
	clearQ := r.db.WithContext(ctx).Model(&LLMConfigPO{}).Where("is_default = 1")
	if target.Usage == "" {
		clearQ = clearQ.Where("usage = '' OR usage IS NULL")
	} else {
		clearQ = clearQ.Where("usage = ?", target.Usage)
	}
	if err := clearQ.Update("is_default", false).Error; err != nil {
		return err
	}
	// 设置目标为默认
	return r.db.WithContext(ctx).Model(&LLMConfigPO{}).Where("name = ?", name).Update("is_default", true).Error
}

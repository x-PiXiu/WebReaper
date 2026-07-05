package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

type GormAgentConfigRepository struct{ db *gorm.DB }

// 编译期断言：实现 port.AgentConfigRepository。
var _ port.AgentConfigRepository = (*GormAgentConfigRepository)(nil)

func NewGormAgentConfigRepository(db *gorm.DB) *GormAgentConfigRepository {
	return &GormAgentConfigRepository{db: db}
}

func (r *GormAgentConfigRepository) Save(ctx context.Context, cfg entity.AgentConfig) error {
	po := agentConfigToPO(cfg)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormAgentConfigRepository) FindByName(ctx context.Context, name string) (entity.AgentConfig, error) {
	var po AgentConfigPO
	err := r.db.WithContext(ctx).First(&po, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.AgentConfig{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.AgentConfig{}, err
	}
	return agentConfigFromPO(po), nil
}

func (r *GormAgentConfigRepository) List(ctx context.Context) ([]entity.AgentConfig, error) {
	var pos []AgentConfigPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.AgentConfig, 0, len(pos))
	for _, p := range pos { result = append(result, agentConfigFromPO(p)) }
	return result, nil
}

func (r *GormAgentConfigRepository) Delete(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Where("name = ?", name).Delete(&AgentConfigPO{}).Error
}

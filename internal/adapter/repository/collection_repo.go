package repository

import (
	"context"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

type GormCollectionRepository struct{ db *gorm.DB }

// 编译期断言：实现 port.CollectionRepository。
var _ port.CollectionRepository = (*GormCollectionRepository)(nil)

func NewGormCollectionRepository(db *gorm.DB) *GormCollectionRepository {
	return &GormCollectionRepository{db: db}
}

func (r *GormCollectionRepository) Save(ctx context.Context, c entity.Collection) error {
	po := collectionToPO(c)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormCollectionRepository) FindByID(ctx context.Context, id string) (entity.Collection, error) {
	var po CollectionPO
	if err := r.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return entity.Collection{}, err
	}
	return collectionFromPO(po), nil
}

func (r *GormCollectionRepository) List(ctx context.Context, limit int) ([]entity.Collection, error) {
	if limit <= 0 { limit = 50 }
	var pos []CollectionPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.Collection, 0, len(pos))
	for _, p := range pos { result = append(result, collectionFromPO(p)) }
	return result, nil
}

func (r *GormCollectionRepository) UpdateStatus(ctx context.Context, id string, status entity.CollectionStatus) error {
	return r.db.WithContext(ctx).Model(&CollectionPO{}).Where("id = ?", id).Update("status", string(status)).Error
}

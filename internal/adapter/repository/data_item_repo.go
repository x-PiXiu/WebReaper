package repository

import (
	"context"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

type GormDataItemRepository struct{ db *gorm.DB }

// 编译期断言：实现 port.DataItemRepository。
var _ port.DataItemRepository = (*GormDataItemRepository)(nil)

func NewGormDataItemRepository(db *gorm.DB) *GormDataItemRepository {
	return &GormDataItemRepository{db: db}
}

func (r *GormDataItemRepository) Save(ctx context.Context, item entity.DataItem) error {
	po := dataItemToPO(item)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormDataItemRepository) SaveBatch(ctx context.Context, items []entity.DataItem) error {
	pos := make([]DataItemPO, 0, len(items))
	for _, item := range items {
		pos = append(pos, dataItemToPO(item))
	}
	return r.db.WithContext(ctx).CreateInBatches(pos, 100).Error
}

func (r *GormDataItemRepository) FindByID(ctx context.Context, id string) (entity.DataItem, error) {
	var po DataItemPO
	if err := r.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return entity.DataItem{}, err
	}
	return dataItemFromPO(po), nil
}

func (r *GormDataItemRepository) List(ctx context.Context, limit int) ([]entity.DataItem, error) {
	if limit <= 0 { limit = 50 }
	var pos []DataItemPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.DataItem, 0, len(pos))
	for _, p := range pos { result = append(result, dataItemFromPO(p)) }
	return result, nil
}

func (r *GormDataItemRepository) ListByCollection(ctx context.Context, collectionID string) ([]entity.DataItem, error) {
	var pos []DataItemPO
	if err := r.db.WithContext(ctx).Where("collection_id = ?", collectionID).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.DataItem, 0, len(pos))
	for _, p := range pos { result = append(result, dataItemFromPO(p)) }
	return result, nil
}

func (r *GormDataItemRepository) ListByStatus(ctx context.Context, status entity.ItemStatus) ([]entity.DataItem, error) {
	var pos []DataItemPO
	if err := r.db.WithContext(ctx).Where("status = ?", string(status)).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.DataItem, 0, len(pos))
	for _, p := range pos { result = append(result, dataItemFromPO(p)) }
	return result, nil
}

func (r *GormDataItemRepository) UpdateStatus(ctx context.Context, id string, status entity.ItemStatus) error {
	return r.db.WithContext(ctx).Model(&DataItemPO{}).Where("id = ?", id).Update("status", string(status)).Error
}

// Delete 删除数据项。
func (r *GormDataItemRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&DataItemPO{}).Error
}

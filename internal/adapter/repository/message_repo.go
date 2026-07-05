package repository

import (
	"context"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// GormMessageRepository 是 port.MessageRepository 的 GORM 实现。
type GormMessageRepository struct{ db *gorm.DB }

// 编译期断言：实现 port.MessageRepository。
var _ port.MessageRepository = (*GormMessageRepository)(nil)

func NewGormMessageRepository(db *gorm.DB) *GormMessageRepository {
	return &GormMessageRepository{db: db}
}

func (r *GormMessageRepository) Save(ctx context.Context, msg entity.Message) error {
	po := messageToPO(msg)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormMessageRepository) ListByConversation(ctx context.Context, convID string) ([]entity.Message, error) {
	var pos []MessagePO
	if err := r.db.WithContext(ctx).Where("conversation_id = ?", convID).Order("created_at ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.Message, 0, len(pos))
	for _, p := range pos {
		result = append(result, messageFromPO(p))
	}
	return result, nil
}

func (r *GormMessageRepository) DeleteByConversation(ctx context.Context, convID string) error {
	return r.db.WithContext(ctx).Where("conversation_id = ?", convID).Delete(&MessagePO{}).Error
}

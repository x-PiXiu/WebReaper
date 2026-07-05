package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// GormConversationRepository 是 port.ConversationRepository 的 GORM 实现。
type GormConversationRepository struct{ db *gorm.DB }

// 编译期断言：实现 port.ConversationRepository。
var _ port.ConversationRepository = (*GormConversationRepository)(nil)

func NewGormConversationRepository(db *gorm.DB) *GormConversationRepository {
	return &GormConversationRepository{db: db}
}

func (r *GormConversationRepository) Save(ctx context.Context, conv entity.Conversation) error {
	po := conversationToPO(conv)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormConversationRepository) FindByID(ctx context.Context, id string) (entity.Conversation, error) {
	var po ConversationPO
	err := r.db.WithContext(ctx).First(&po, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Conversation{}, pkg.ErrNotFound
	}
	if err != nil {
		return entity.Conversation{}, err
	}
	return conversationFromPO(po), nil
}

func (r *GormConversationRepository) ListByUser(ctx context.Context, userID string) ([]entity.Conversation, error) {
	var pos []ConversationPO
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.Conversation, 0, len(pos))
	for _, p := range pos {
		result = append(result, conversationFromPO(p))
	}
	return result, nil
}

func (r *GormConversationRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&ConversationPO{}).Error
}

func (r *GormConversationRepository) UpdateTitle(ctx context.Context, id, title string) error {
	return r.db.WithContext(ctx).Model(&ConversationPO{}).Where("id = ?", id).
		Updates(map[string]any{"title": title, "updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)")}).Error
}

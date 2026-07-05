// Package conversation 实现"聊天会话管理"用例。
//
// 职责：会话 CRUD + 消息保存/查询，按 UserID 隔离。
//
// 设计动机（整洁架构）：
//   - 把会话持久化编排从 handler 下沉到用例层，handler 只调 usecase。
//   - 删除会话时级联清理消息（事务性由 usecase 编排保证）。
package conversation

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ConversationUseCase 聊天会话管理用例。
type ConversationUseCase struct {
	convRepo port.ConversationRepository
	msgRepo  port.MessageRepository
}

func NewConversationUseCase(convRepo port.ConversationRepository, msgRepo port.MessageRepository) *ConversationUseCase {
	return &ConversationUseCase{convRepo: convRepo, msgRepo: msgRepo}
}

// CreateInput 创建会话的输入。
type CreateInput struct {
	ID        string // 前端生成的会话 ID
	Title     string
	AgentName string
	UserID    string
}

// Create 创建会话。
func (uc *ConversationUseCase) Create(ctx context.Context, in CreateInput) (entity.Conversation, error) {
	now := time.Now()
	conv := entity.Conversation{
		ID: in.ID, Title: in.Title, AgentName: in.AgentName, UserID: in.UserID,
		CreatedAt: now, UpdatedAt: now,
	}
	if !conv.IsValid() {
		return entity.Conversation{}, fmt.Errorf("会话无效：id 和 user_id 不能为空")
	}
	if err := uc.convRepo.Save(ctx, conv); err != nil {
		return entity.Conversation{}, fmt.Errorf("save conversation: %w", err)
	}
	return conv, nil
}

// List 列出指定用户的全部会话（按更新时间倒序）。
func (uc *ConversationUseCase) List(ctx context.Context, userID string) ([]entity.Conversation, error) {
	return uc.convRepo.ListByUser(ctx, userID)
}

// GetMessages 拉取指定会话的全部消息（按时间正序）。
func (uc *ConversationUseCase) GetMessages(ctx context.Context, convID string) ([]entity.Message, error) {
	return uc.msgRepo.ListByConversation(ctx, convID)
}

// SaveMessage 保存一条消息（流式结束后整体保存）。
func (uc *ConversationUseCase) SaveMessage(ctx context.Context, msg entity.Message) error {
	if !msg.IsValid() {
		return fmt.Errorf("消息无效：id/conversation_id/role 非法")
	}
	if err := uc.msgRepo.Save(ctx, msg); err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	// 收到新消息时更新会话的 UpdatedAt（用于会话列表排序）
	conv, err := uc.convRepo.FindByID(ctx, msg.ConversationID)
	if err == nil {
		conv.UpdatedAt = time.Now()
		_ = uc.convRepo.Save(ctx, conv)
	}
	return nil
}

// Rename 重命名会话。
func (uc *ConversationUseCase) Rename(ctx context.Context, id, title string) error {
	if title == "" {
		return fmt.Errorf("标题不能为空")
	}
	return uc.convRepo.UpdateTitle(ctx, id, title)
}

// Delete 删除会话（级联删除其下全部消息）。
func (uc *ConversationUseCase) Delete(ctx context.Context, id string) error {
	// 先删消息，再删会话（避免孤儿消息）
	if err := uc.msgRepo.DeleteByConversation(ctx, id); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if err := uc.convRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

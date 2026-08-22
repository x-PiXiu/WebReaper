package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// AgentConfigRepository Agent 配置持久化接口。
type AgentConfigRepository interface {
	Save(ctx context.Context, cfg entity.AgentConfig) error
	FindByName(ctx context.Context, name string) (entity.AgentConfig, error)
	List(ctx context.Context) ([]entity.AgentConfig, error)
	Delete(ctx context.Context, name string) error
}

// LLMConfigRepository LLM 配置持久化接口。
//
// LLM 配置独立于 Agent 配置持久化（聚合边界分离）。
// AgentConfig 通过 LLMConfigName 引用此处的记录。
type LLMConfigRepository interface {
	Save(ctx context.Context, cfg entity.LLMConfig) error
	FindByName(ctx context.Context, name string) (entity.LLMConfig, error)
	FindByUsage(ctx context.Context, usage string) (entity.LLMConfig, error)
	List(ctx context.Context) ([]entity.LLMConfig, error)
	Delete(ctx context.Context, name string) error
	// SetDefault 设置默认模型（同 Usage 下互斥——先清除再设置）。
	SetDefault(ctx context.Context, name string) error
}

// ConversationRepository 聊天会话持久化接口（按 UserID 隔离）。
type ConversationRepository interface {
	Save(ctx context.Context, conv entity.Conversation) error
	FindByID(ctx context.Context, id string) (entity.Conversation, error)
	ListByUser(ctx context.Context, userID string) ([]entity.Conversation, error)
	Delete(ctx context.Context, id string) error
	UpdateTitle(ctx context.Context, id, title string) error
}

// MessageRepository 聊天消息持久化接口。
type MessageRepository interface {
	Save(ctx context.Context, msg entity.Message) error
	ListByConversation(ctx context.Context, convID string) ([]entity.Message, error)
	DeleteByConversation(ctx context.Context, convID string) error
}

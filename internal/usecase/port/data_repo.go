package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// DataItemRepository 数据项持久化接口。
type DataItemRepository interface {
	Save(ctx context.Context, item entity.DataItem) error
	SaveBatch(ctx context.Context, items []entity.DataItem) error
	FindByID(ctx context.Context, id string) (entity.DataItem, error)
	List(ctx context.Context, limit int) ([]entity.DataItem, error)
	ListByCollection(ctx context.Context, collectionID string) ([]entity.DataItem, error)
	ListByStatus(ctx context.Context, status entity.ItemStatus) ([]entity.DataItem, error)
	UpdateStatus(ctx context.Context, id string, status entity.ItemStatus) error
	Delete(ctx context.Context, id string) error

	// ---- 统计聚合（仪表盘用）----
	// CountByStatus 按状态分组计数，返回 {status: count}。
	CountByStatus(ctx context.Context) (map[string]int, error)
	// DailyCounts 近 days 天每日新增量，按日期升序。
	DailyCounts(ctx context.Context, days int) ([]DailyCount, error)
	// GroupByMetaKey 按 metadata 的某个 key 分组计数（如 crawler_type）。
	GroupByMetaKey(ctx context.Context, key string) ([]GroupCount, error)
	// TopTags 标签频次 Top N。
	TopTags(ctx context.Context, limit int) ([]GroupCount, error)
}

// DailyCount 单日计数（趋势图用）。
type DailyCount struct {
	Date  string `json:"date"`  // YYYY-MM-DD
	Count int    `json:"count"`
}

// GroupCount 分组计数（饼图/条形图用）。
type GroupCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

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
	List(ctx context.Context) ([]entity.LLMConfig, error)
	Delete(ctx context.Context, name string) error
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

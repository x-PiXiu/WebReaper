package repository

import (
	"time"

	"gorm.io/datatypes"
)

// ---- UserPO（认证用，多租户）----

type UserPO struct {
	ID           string    `gorm:"primaryKey;size:64"`
	Username     string    `gorm:"size:64;uniqueIndex"`
	PasswordHash string    `gorm:"size:128"`
	Role         string    `gorm:"size:32;index"`       // admin / merchant
	TenantID     string    `gorm:"size:64;index"`       // 归属租户（admin 可空）
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserPO) TableName() string { return "webreaper_users" }

// ---- TaskPO（异步任务，保留）----

type TaskPO struct {
	ID        string `gorm:"primaryKey;size:64"`
	Type      string `gorm:"size:32"`
	Input     string `gorm:"type:text"`
	Output    string `gorm:"type:text"`
	Progress  string `gorm:"type:text"` // 运行中进度描述
	Status    string `gorm:"size:16;index"`
	Error     string `gorm:"type:text"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (TaskPO) TableName() string { return "tasks" }

// ---- AgentConfigPO（Agent 配置，存DB）----

type AgentConfigPO struct {
	Name          string `gorm:"primaryKey;size:64"`
	SystemPrompt  string `gorm:"type:text"`
	Tools         datatypes.JSON `gorm:"type:json"`
	Model         string `gorm:"size:64"`           // 历史字段（保留向后兼容，新代码用 LLMConfigName）
	LLMConfigName string `gorm:"column:llm_config_name;size:64"` // 引用的 LLM 配置名
	MaxIterations int    `gorm:"default:10"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (AgentConfigPO) TableName() string { return "agent_configs" }

// ---- LLMConfigPO（LLM 配置，存DB）----

type LLMConfigPO struct {
	Name      string `gorm:"primaryKey;size:64"`
	Provider  string `gorm:"size:32"`
	APIKey    string `gorm:"column:api_key;size:256"`
	BaseURL   string `gorm:"column:base_url;size:256"`
	Model     string `gorm:"size:64"`
	// CostPerMTok 每百万 tokens 参考成本（分；P1-1 按引擎差异化成本分析）。
	CostPerMTok int    `gorm:"column:cost_per_mtok;default:100"`
	// Usage 用途标签："" = 聊天/内容（默认）；"vision" = 视觉模型。
	Usage     string `gorm:"size:32;default:''"`
	// IsDefault 是否为该用途的默认模型（同 Usage 下互斥——SetDefault 时清除其他）。
	IsDefault bool   `gorm:"column:is_default;default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (LLMConfigPO) TableName() string { return "llm_configs" }

// ---- ConversationPO / MessagePO（聊天会话持久化）----

type ConversationPO struct {
	ID        string    `gorm:"primaryKey;size:64"`
	Title     string    `gorm:"size:128"`
	AgentName string    `gorm:"size:64"`
	UserID    string    `gorm:"size:64;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ConversationPO) TableName() string { return "conversations" }

type MessagePO struct {
	ID             string    `gorm:"primaryKey;size:64"`
	ConversationID string    `gorm:"size:64;index"`
	Role           string    `gorm:"size:16"`
	Content        string    `gorm:"type:longtext"`
	ToolCallsJSON  string    `gorm:"column:tool_calls;type:longtext"`
	CreatedAt      time.Time
}

func (MessagePO) TableName() string { return "messages" }

// ---- SystemSettingPO（系统级配置，key-value）----
// 注意：列名用 setting_key 而非 key，因为 KEY 是 MySQL 保留字。

type SystemSettingPO struct {
	Key       string    `gorm:"column:setting_key;primaryKey;size:64"`
	Value     string    `gorm:"type:longtext"`
	UpdatedAt time.Time
}

func (SystemSettingPO) TableName() string { return "system_settings" }

// allModels 返回所有需要迁移的 PO。
func allModels() []any {
	return []any{
		&UserPO{},
		&TaskPO{},
		&AgentConfigPO{},
		&LLMConfigPO{},
		&ConversationPO{},
		&MessagePO{},
		&SystemSettingPO{},
		// GEO 表（013_geo_core.sql）
		&BrandPO{},
		&KeywordPO{},
		&MonitoringResultPO{},
		&OptimizedContentPO{},
		// 发布账号/发布任务表（014_publish_accounts.sql）
		&AccountPO{},
		&PublishJobPO{},
		// LLM 用量计量（经济系统基础，AutoMigrate 自动建表）
		&UsagePO{},
		// 租户级设置（多租户个性化配置）
		&TenantSettingPO{},
		// 提示词模板仓库（内容生成/优化提示词可管理）
		&PromptTemplatePO{},
		// 经济系统（套餐 / 订阅 / 订单）
		&PlanPO{},
		&SubscriptionPO{},
		&OrderPO{},
		// 平台知识库（043_knowledge_materials.sql：按行业采集素材，带来源溯源）
		&KnowledgeMaterialPO{},
	}
}

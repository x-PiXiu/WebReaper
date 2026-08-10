package repository

import "time"

// ---- 多平台发布 PO（持久化对象）----
// 与领域实体分离（ADR-003），通过 mapper 双向转换。
// 所有表带 tenant_id，强制多租户隔离。

// AccountPO 平台账号（加密 cookie）。
type AccountPO struct {
	ID              string    `gorm:"primaryKey;size:64"`
	TenantID        string    `gorm:"size:64;index"`
	Platform        string    `gorm:"size:32"`
	DisplayName     string    `gorm:"size:128"`
	CookieEncrypted string    `gorm:"type:text"`
	Health          string    `gorm:"size:16"`
	LoginMethod     string    `gorm:"size:16"`
	ExpiresAt       time.Time `gorm:"index"`
	BoundAt         time.Time
	LastUsedAt      time.Time
}

func (AccountPO) TableName() string { return "geo_accounts" }

// PublishJobPO 发布任务记录。
type PublishJobPO struct {
	ID          string    `gorm:"primaryKey;size:64"`
	TenantID    string    `gorm:"size:64;index"`
	AccountID   string    `gorm:"size:64"`
	Platform    string    `gorm:"size:32"`
	ContentID   string    `gorm:"size:64"`
	BrandID     string    `gorm:"size:64"`
	Title       string    `gorm:"size:256"`
	Content     string    `gorm:"type:longtext"`
	Mode        string    `gorm:"size:16"`
	Status      string    `gorm:"size:16"`
	ExternalURL string    `gorm:"type:text"`
	ErrorMsg    string    `gorm:"type:text"`
	CreatedAt   time.Time
	PublishedAt time.Time
	PreMentionRate  float64 `gorm:"type:decimal(5,2)"`
	PostMentionRate float64 `gorm:"type:decimal(5,2)"`
}

func (PublishJobPO) TableName() string { return "geo_publish_jobs" }

package repository

import "time"

// ---- 多平台发布 PO（持久化对象）----
// 与领域实体分离（ADR-003），通过 mapper 双向转换。
// 所有表带 tenant_id，强制多租户隔离。

// AccountPO 平台账号（加密 cookie）。
// 可空时间列（expires_at/bound_at/last_used_at）用 *time.Time——防零日期写库（Error 1292）。
type AccountPO struct {
	ID              string     `gorm:"primaryKey;size:64"`
	TenantID        string     `gorm:"size:64;index"`
	Platform        string     `gorm:"size:32"`
	DisplayName     string     `gorm:"size:128"`
	CookieEncrypted string     `gorm:"type:text"`
	Health          string     `gorm:"size:16"`
	LoginMethod     string     `gorm:"size:16"`
	ExpiresAt       *time.Time `gorm:"index"`
	BoundAt         *time.Time
	LastUsedAt      *time.Time
	// 官方 OAuth 授权（抖音开放平台等）：token 密文 + open_id
	AuthType          string `gorm:"size:16;default:cookie"` // cookie（扫码浏览器）/ oauth（官方授权）
	AccessTokenEnc    string `gorm:"type:text"`
	RefreshTokenEnc   string `gorm:"type:text"`
	OpenID            string `gorm:"size:128"`
}

func (AccountPO) TableName() string { return "geo_accounts" }

// PublishJobPO 发布任务记录。
type PublishJobPO struct {
	ID              string `gorm:"primaryKey;size:64"`
	TenantID        string `gorm:"size:64;index"`
	AccountID       string `gorm:"size:64"`
	Platform        string `gorm:"size:32"`
	ContentID       string `gorm:"size:64"`
	BrandID         string `gorm:"size:64"`
	Title           string `gorm:"size:256"`
	Content         string `gorm:"type:longtext"`
	Mode            string `gorm:"size:16"`
	Status          string `gorm:"size:16"`
	ExternalURL     string `gorm:"type:text"`
	ErrorMsg        string `gorm:"type:text"`
	CreatedAt       time.Time
	PublishedAt     *time.Time // 实际发布时间（可空列）
	PreMentionRate  float64     `gorm:"type:decimal(5,2)"`
	PostMentionRate float64     `gorm:"type:decimal(5,2)"`
	ScheduledAt     *time.Time  `gorm:"index"` // 排期发布时间（nil=立即）
	StoreAddress    string      `gorm:"size:256"` // 门店地址（本地生活 P3：内容层本地曝光信号）
	ContentType     string      `gorm:"size:16"`  // 内容形态：image/video/article/audio
	MediaURLsJSON   string      `gorm:"type:text"` // 媒体文件 URL 列表（JSON 数组）
	CoverURL        string      `gorm:"type:text"` // 封面图 URL
}

func (PublishJobPO) TableName() string { return "geo_publish_jobs" }

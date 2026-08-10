package repository

import (
	"time"

	"gorm.io/datatypes"
)

// ---- GEO PO（持久化对象）----
// 与领域实体分离（ADR-003），通过 mapper 双向转换。
// 所有表带 tenant_id，强制多租户隔离。

// BrandPO 品牌。
type BrandPO struct {
	ID           string         `gorm:"primaryKey;size:64"`
	TenantID     string         `gorm:"size:64;index"`
	Name         string         `gorm:"size:128"`
	Positioning  string         `gorm:"type:text"`
	CoreSelling  datatypes.JSON `gorm:"type:json"`
	Competitors  datatypes.JSON `gorm:"type:json"`
	CreatedAt    time.Time
}

func (BrandPO) TableName() string { return "geo_brands" }

// KeywordPO 关键词。
type KeywordPO struct {
	ID        string    `gorm:"primaryKey;size:64"`
	TenantID  string    `gorm:"size:64;index"`
	BrandID   string    `gorm:"size:64;index"`
	Term      string    `gorm:"size:256"`
	Intent    string    `gorm:"size:32"`
	CreatedAt time.Time
}

func (KeywordPO) TableName() string { return "geo_keywords" }

// MonitoringResultPO 监测结果（核心数据资产）。
type MonitoringResultPO struct {
	ID           string         `gorm:"primaryKey;size:64"`
	TenantID     string         `gorm:"size:64;index"`
	BrandID      string         `gorm:"size:64;index"`
	KeywordID    string         `gorm:"size:64;index"`
	EngineName   string         `gorm:"size:64"`
	SampleCount  int
	MentionCount int
	MentionRate  float64 `gorm:"type:decimal(4,3)"`
	AvgPosition  int
	Sentiment    string         `gorm:"size:16"`
	Competitors  datatypes.JSON `gorm:"type:json"`
	Confidence   float64        `gorm:"type:decimal(4,3)"`
	ProbedAt     time.Time      `gorm:"index"`
	RawSample    string         `gorm:"type:text"`
}

func (MonitoringResultPO) TableName() string { return "geo_monitoring_results" }

// OptimizedContentPO 优化内容。
type OptimizedContentPO struct {
	ID            string    `gorm:"primaryKey;size:64"`
	TenantID      string    `gorm:"size:64;index"`
	BrandID       string    `gorm:"size:64;index"`
	KeywordID     string    `gorm:"size:64"`
	OriginalText  string    `gorm:"type:longtext"`
	OptimizedText string    `gorm:"type:longtext"`
	Version       int
	ScoreTotal    float64   `gorm:"type:decimal(5,2)"`
	Authority     float64   `gorm:"type:decimal(5,2)"`
	Specificity   float64   `gorm:"type:decimal(5,2)"`
	Structure     float64   `gorm:"type:decimal(5,2)"`
	Uniqueness    float64   `gorm:"type:decimal(5,2)"`
	Recency       float64   `gorm:"type:decimal(5,2)"`
	Status        string    `gorm:"size:32"`
	CreatedAt     time.Time
}

func (OptimizedContentPO) TableName() string { return "geo_optimized_contents" }

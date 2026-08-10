package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
)

// GormUsageRecorder port.UsageRecorder 的 GORM 实现（LLM 用量落库）。
// 计量失败不阻断主流程（记录器内部吞错）。
type GormUsageRecorder struct {
	db *gorm.DB
}

func NewGormUsageRecorder(db *gorm.DB) *GormUsageRecorder {
	return &GormUsageRecorder{db: db}
}

func (r *GormUsageRecorder) RecordUsage(ctx context.Context, rec entity.UsageRecord) error {
	if rec.ID == "" {
		rec.ID = fmt.Sprintf("usage-%d", time.Now().UnixNano())
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	po := UsagePO{
		ID: rec.ID, TenantID: rec.TenantID, UserID: rec.UserID, Scene: rec.Scene,
		LLMConfigName: rec.LLMConfigName, Model: rec.Model,
		PromptTokens: rec.PromptTokens, CompletionTokens: rec.CompletionTokens,
		TotalTokens: rec.TotalTokens, LLMCalls: rec.LLMCalls, CreatedAt: rec.CreatedAt,
	}
	// 计量是横切关注点：失败仅记日志级别处理（这里直接忽略错误返回给调用方，
	// 由调用方决定是否告警——主流程不阻塞）
	return r.db.WithContext(ctx).Create(&po).Error
}

// UsagePO LLM 用量持久化对象（usages 表，AutoMigrate 建表）。
type UsagePO struct {
	ID               string    `gorm:"primaryKey;size:64"`
	TenantID         string    `gorm:"size:64;index"` // 空 = 平台后台消耗
	UserID           string    `gorm:"size:64;index"`
	Scene            string    `gorm:"size:32;index"` // chat/monitor/content-gen/...
	LLMConfigName    string    `gorm:"size:64"`
	Model            string    `gorm:"size:64"`
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	LLMCalls         int
	CreatedAt        time.Time `gorm:"index"`
}

func (UsagePO) TableName() string { return "usages" }

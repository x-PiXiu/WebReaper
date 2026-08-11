package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
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

// CountSince 实现 port.UsageQueryer：统计租户某场景计费周期内的 LLM 调用次数。
// 配额扣减的数据源——计数派生型配额（用量即扣减，无独立计数器状态）。
func (r *GormUsageRecorder) CountSince(ctx context.Context, tenantID, scene string, since time.Time) (int, error) {
	if tenantID == "" || scene == "" {
		return 0, nil // 平台消耗（空租户）不计入配额
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&UsagePO{}).
		Where("tenant_id = ? AND scene = ? AND created_at >= ?", tenantID, scene, since).
		Count(&n).Error
	return int(n), err
}

// SumBySceneSince 实现 port.UsageStatsQueryer：按场景聚合用量（X-01 成本分析）。
// Scene 为空的历史数据归入 "" 组（老版本未打场景标），成本报表可见并可忽略。
func (r *GormUsageRecorder) SumBySceneSince(ctx context.Context, since time.Time) ([]port.SceneUsage, error) {
	var rows []struct {
		Scene       string
		Calls       int
		TotalTokens int64
	}
	err := r.db.WithContext(ctx).Model(&UsagePO{}).
		Select("scene AS scene, COUNT(*) AS calls, COALESCE(SUM(total_tokens), 0) AS total_tokens").
		Where("created_at >= ?", since).
		Group("scene").
		Order("total_tokens DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]port.SceneUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.SceneUsage{Scene: row.Scene, Calls: row.Calls, TotalTokens: row.TotalTokens})
	}
	return out, nil
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

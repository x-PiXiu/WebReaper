// Package scheduledtask 提供业务定时任务实现（注册进通用调度器）。
//
// 每个任务实现 port.ScheduledTask 接口——只写业务逻辑（Execute），
// 调度细节（周期/防重入/分布式锁/日志）由通用调度器统一承担。
package scheduledtask

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/port"
)

// ---- ① 账号健康度检查 ----

// AccountHealthTask 定期检查全部账号 cookie 过期状态（每 10 分钟）。
type AccountHealthTask struct {
	uc     *account.AccountUseCase
	logger port.Logger
}

func NewAccountHealthTask(uc *account.AccountUseCase, logger port.Logger) *AccountHealthTask {
	return &AccountHealthTask{uc: uc, logger: logger}
}

func (t *AccountHealthTask) Name() string            { return "account-health-check" }
func (t *AccountHealthTask) Interval() time.Duration { return 10 * time.Minute }
func (t *AccountHealthTask) Execute(ctx context.Context) error {
	t.uc.CheckAccountHealth(ctx)
	return nil
}

// ---- ② 每日自动监测（自动盯盘）----

// DailyMonitorTask 每日对全平台所有品牌执行一次监测（提及率趋势自动生长）。
//
// 设计要点（两级开关 + 套餐门禁）：
//   - 平台级总闸：system_settings.auto_monitor_enabled（管理后台「平台设置」页）
//   - 租户级开关：tenant_settings.auto_monitor_enabled（商户端可自行关闭，
//     避免消耗自己的 LLM 额度）——关闭的租户跳过
//   - 套餐门禁：订阅套餐须含 auto-monitor 能力位（free 无 / pro / team 有）——
//     免费用户不参与自动盯盘（避免静默消耗其配额）
//   - 开关未配置时回退构造时传入的默认值
//   - 通过品牌仓储 ListAll 枚举全平台品牌（admin 旁路视角）
//   - 单个品牌失败不中断其余；分布式部署由调度器锁保证单实例执行
type DailyMonitorTask struct {
	uc                *geo.MonitorUseCase
	brandRepo         port.BrandRepository
	settingRepo       port.SystemSettingRepository
	tenantSettingRepo port.TenantSettingRepository // 租户开关（nil=全部租户都监测）
	planRepo          port.PlanRepository         // 套餐能力位门禁（nil=不检查）
	subRepo           port.SubscriptionRepository
	notifier          *MonitorNotifier             // 变化通知（nil=不通知）
	defaultEnabled    bool                         // 平台开关未配置时的兜底（.env AUTO_MONITOR_ENABLED）
	logger            port.Logger
}

func NewDailyMonitorTask(uc *geo.MonitorUseCase, brandRepo port.BrandRepository, settingRepo port.SystemSettingRepository, tenantSettingRepo port.TenantSettingRepository, defaultEnabled bool, logger port.Logger) *DailyMonitorTask {
	return &DailyMonitorTask{
		uc: uc, brandRepo: brandRepo, settingRepo: settingRepo,
		tenantSettingRepo: tenantSettingRepo,
		defaultEnabled:    defaultEnabled, logger: logger,
	}
}

// SetPlanGate 注入套餐能力位门禁（可选；P2：auto-monitor 是付费能力——
// 免费用户不参与自动盯盘，避免静默消耗配额；未注入=全部租户可盯）。
func (t *DailyMonitorTask) SetPlanGate(planRepo port.PlanRepository, subRepo port.SubscriptionRepository) {
	if planRepo != nil && subRepo != nil {
		t.planRepo = planRepo
		t.subRepo = subRepo
	}
}

// SetNotifier 注入监测变化通知器（可选；nil=仅监测不通知）。
func (t *DailyMonitorTask) SetNotifier(n *MonitorNotifier) {
	if n != nil {
		t.notifier = n
	}
}

func (t *DailyMonitorTask) Name() string { return "daily-brand-monitor" }

// Interval 调度间隔：6 小时一轮（租户级频率 gate 控制实际盯盘节奏——
// daily 租户每 24h 一次、half_day 每 12h、weekly 每 7 天；调度器 6h 保证 half_day 可达）。
func (t *DailyMonitorTask) Interval() time.Duration { return 6 * time.Hour }

// enabled 读取平台级开关（system_settings 优先；未配置回退默认值）。
func (t *DailyMonitorTask) enabled(ctx context.Context) bool {
	if t.settingRepo != nil {
		if s, err := t.settingRepo.Get(ctx, entity.SettingKeyAutoMonitor); err == nil {
			return s.Value == "true"
		}
	}
	return t.defaultEnabled
}

// tenantEnabled 租户级判断：套餐能力位 + 租户开关。
//   - 套餐门禁：订阅套餐须含 auto-monitor 能力位（free 无 → 跳过，不消耗配额）
//   - 租户开关：tenant_settings.auto_monitor_enabled（未配置默认开启）
func (t *DailyMonitorTask) tenantEnabled(ctx context.Context, tenantID string) bool {
	if t.planRepo != nil && t.subRepo != nil {
		hasFeature := false
		if sub, err := t.subRepo.FindByTenant(ctx, tenantID); err == nil && sub.IsActive(time.Now()) {
			if plan, pErr := t.planRepo.FindByID(ctx, sub.PlanID); pErr == nil {
				for _, f := range plan.Features {
					if f == "auto-monitor" {
						hasFeature = true
						break
					}
				}
			}
		}
		if !hasFeature {
			return false // 套餐无 auto-monitor 能力位：跳过（免费用户不消耗配额）
		}
	}
	if t.tenantSettingRepo == nil {
		return true
	}
	s, err := t.tenantSettingRepo.Get(ctx, tenantID, entity.TenantSettingKeyAutoMonitor)
	if err != nil {
		return true
	}
	return s.Value != "false"
}

// tenantConfig 读租户盯盘配置（未配置/读取失败 → 默认：每日、5 采样、default 引擎）。
func (t *DailyMonitorTask) tenantConfig(ctx context.Context, tenantID string) entity.AutoMonitorConfig {
	cfg := entity.DefaultAutoMonitorConfig()
	if t.tenantSettingRepo == nil {
		return cfg
	}
	s, err := t.tenantSettingRepo.Get(ctx, tenantID, entity.TenantSettingKeyAutoMonitorConfig)
	if err != nil {
		return cfg
	}
	var c entity.AutoMonitorConfig
	if json.Unmarshal([]byte(s.Value), &c) != nil {
		return cfg
	}
	return c.Valid()
}

// frequencyDue 频率 gate：按上次执行时间判断本次是否该跑。
// daily=距上次≥24h、half_day=≥12h、weekly=≥7 天；无记录=立即执行首次。
func (t *DailyMonitorTask) frequencyDue(ctx context.Context, tenantID, frequency string) bool {
	if t.tenantSettingRepo == nil {
		return true
	}
	s, err := t.tenantSettingRepo.Get(ctx, tenantID, entity.TenantSettingKeyAutoMonitorLastRun)
	if err != nil {
		return true // 从未执行过：跑首次
	}
	last, pErr := time.Parse(time.RFC3339, s.Value)
	if pErr != nil {
		return true
	}
	var minGap time.Duration
	switch frequency {
	case "half_day":
		minGap = 12 * time.Hour
	case "weekly":
		minGap = 7 * 24 * time.Hour
	default: // daily
		minGap = 24 * time.Hour
	}
	return time.Since(last) >= minGap
}

// markRun 记录租户本次盯盘执行时间（RFC3339）。
func (t *DailyMonitorTask) markRun(ctx context.Context, tenantID string) {
	if t.tenantSettingRepo == nil {
		return
	}
	_ = t.tenantSettingRepo.Save(ctx, entity.TenantSetting{
		TenantID: tenantID,
		Key:      entity.TenantSettingKeyAutoMonitorLastRun,
		Value:    time.Now().Format(time.RFC3339),
	})
}

func (t *DailyMonitorTask) Execute(ctx context.Context) error {
	if !t.enabled(ctx) {
		return nil // 平台总闸关闭：空转（保留注册，开启后下个周期生效）
	}
	brands, err := t.brandRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("枚举品牌失败: %w", err)
	}
	if len(brands) == 0 {
		return nil // 无品牌可监测
	}

	success, failed, skipped := 0, 0, 0
	skippedTenants := map[string]bool{}
	for _, b := range brands {
		// 租户级开关：商户关闭自动盯盘的租户跳过（节省其 LLM 额度）
		if !t.tenantEnabled(ctx, b.TenantID) {
			if !skippedTenants[b.TenantID] {
				skippedTenants[b.TenantID] = true
				skipped++
			}
			continue
		}
		// 盯盘配置（频率 gate + 采样/引擎）：按租户个性化节奏执行
		cfg := t.tenantConfig(ctx, b.TenantID)
		if !t.frequencyDue(ctx, b.TenantID, cfg.Frequency) {
			if !skippedTenants[b.TenantID] {
				skippedTenants[b.TenantID] = true
				skipped++
			}
			continue
		}
		// 变化通知基线：上一批监测的平均提及率（Trend 取最近记录）
		beforeAvg := 0.0
		if t.notifier != nil {
			beforeAvg = t.notifier.BaselineAvg(ctx, b.TenantID, b.ID)
		}
		results, mErr := t.uc.Monitor(ctx, geo.MonitorInput{
			TenantID:   b.TenantID,
			BrandID:    b.ID,
			SampleSize: cfg.SampleSize, // 租户配置的每关键词采样数
			EngineName: cfg.EngineName, // 租户配置的引擎（空=default）
		})
		if mErr != nil {
			failed++
			t.logger.Error("每日监测失败", port.Err(mErr), port.String("brand", b.Name))
			continue
		}
		success++
		// 记录本次执行时间（频率 gate）
		t.markRun(ctx, b.TenantID)
		// 变化通知：按租户配置的阈值（提及率显著下降 / 竞品反超）→ 站内通知
		if t.notifier != nil {
			t.notifier.EvaluateAndNotify(ctx, b.TenantID, b.Name, beforeAvg, results, cfg.NotifyDropThreshold, cfg.NotifyOvertake)
		}
	}
	t.logger.Info("每日自动监测完成",
		port.Int("brands", len(brands)),
		port.Int("success", success),
		port.Int("failed", failed),
		port.Int("skipped_tenants", skipped),
	)
	return nil
}

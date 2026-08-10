// Package scheduledtask 提供业务定时任务实现（注册进通用调度器）。
//
// 每个任务实现 port.ScheduledTask 接口——只写业务逻辑（Execute），
// 调度细节（周期/防重入/分布式锁/日志）由通用调度器统一承担。
package scheduledtask

import (
	"context"
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
// 设计要点：
//   - 运行时开关（system_settings.auto_monitor_enabled，管理后台可切换）：
//     关闭时任务空转（保留注册，避免重启——开关即时生效）
//   - 开关未配置时回退构造时传入的默认值（来自 .env AUTO_MONITOR_ENABLED）
//   - 通过品牌仓储 ListAll 枚举全平台品牌（admin 旁路视角）
//   - 单个品牌失败不中断其余（降级记录日志，下一周期重试）
//   - 分布式部署时由调度器锁保证单实例执行（不会重复消耗 LLM 额度）
type DailyMonitorTask struct {
	uc             *geo.MonitorUseCase
	brandRepo      port.BrandRepository
	settingRepo    port.SystemSettingRepository
	defaultEnabled bool // 开关未配置时的兜底（.env AUTO_MONITOR_ENABLED）
	logger         port.Logger
}

func NewDailyMonitorTask(uc *geo.MonitorUseCase, brandRepo port.BrandRepository, settingRepo port.SystemSettingRepository, defaultEnabled bool, logger port.Logger) *DailyMonitorTask {
	return &DailyMonitorTask{
		uc: uc, brandRepo: brandRepo, settingRepo: settingRepo,
		defaultEnabled: defaultEnabled, logger: logger,
	}
}

func (t *DailyMonitorTask) Name() string { return "daily-brand-monitor" }

func (t *DailyMonitorTask) Interval() time.Duration { return 24 * time.Hour }

// enabled 读取运行时开关（system_settings 优先；未配置回退默认值）。
func (t *DailyMonitorTask) enabled(ctx context.Context) bool {
	if t.settingRepo != nil {
		if s, err := t.settingRepo.Get(ctx, entity.SettingKeyAutoMonitor); err == nil {
			return s.Value == "true"
		}
	}
	return t.defaultEnabled
}

func (t *DailyMonitorTask) Execute(ctx context.Context) error {
	if !t.enabled(ctx) {
		return nil // 开关关闭：空转（保留注册，管理后台开启后下个周期生效）
	}
	brands, err := t.brandRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("枚举品牌失败: %w", err)
	}
	if len(brands) == 0 {
		return nil // 无品牌可监测
	}

	success, failed := 0, 0
	for _, b := range brands {
		if _, mErr := t.uc.Monitor(ctx, geo.MonitorInput{
			TenantID: b.TenantID,
			BrandID:  b.ID,
		}); mErr != nil {
			failed++
			t.logger.Error("每日监测失败", port.Err(mErr), port.String("brand", b.Name))
			continue
		}
		success++
	}
	t.logger.Info("每日自动监测完成",
		port.Int("brands", len(brands)),
		port.Int("success", success),
		port.Int("failed", failed),
	)
	return nil
}

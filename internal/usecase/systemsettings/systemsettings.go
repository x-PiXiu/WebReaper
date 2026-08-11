// Package systemsettings 实现"平台系统设置"用例。
//
// 职责：平台级运行时配置（system_settings 通用键值）与租户级个性化配置
// （tenant_settings）的读写。整洁架构：依赖 port 接口，业务规则在用例层。
package systemsettings

import (
	"context"
	"encoding/json"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// SystemSettingsUseCase 平台系统设置用例（平台总闸 + 租户个性化开关）。
type SystemSettingsUseCase struct {
	settingRepo      port.SystemSettingRepository
	tenantSettingRepo port.TenantSettingRepository // 租户级设置（可为 nil：租户开关降级为始终开启）
}

func NewSystemSettingsUseCase(settingRepo port.SystemSettingRepository) *SystemSettingsUseCase {
	return &SystemSettingsUseCase{settingRepo: settingRepo}
}

// SetTenantSettingRepo 注入租户级设置仓储（可选）。
// 未注入时租户开关降级为始终开启（平台级总闸仍生效）。
func (uc *SystemSettingsUseCase) SetTenantSettingRepo(r port.TenantSettingRepository) {
	if r != nil {
		uc.tenantSettingRepo = r
	}
}

// ---- 平台级：自动盯盘总闸 ----

// GetAutoMonitor 读每日自动监测开关（未配置返回 false）。
func (uc *SystemSettingsUseCase) GetAutoMonitor(ctx context.Context) (bool, error) {
	s, err := uc.settingRepo.Get(ctx, entity.SettingKeyAutoMonitor)
	if err != nil {
		return false, nil // 未配置不当作错误
	}
	return s.Value == "true", nil
}

// SetAutoMonitor 写每日自动监测开关（true/false）。
// 立即生效：调度任务每个周期读一次该配置（无需重启）。
func (uc *SystemSettingsUseCase) SetAutoMonitor(ctx context.Context, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return uc.settingRepo.Save(ctx, entity.SystemSetting{
		Key:   entity.SettingKeyAutoMonitor,
		Value: value,
	})
}

// ---- 租户级：我的品牌自动盯盘 ----

// GetTenantAutoMonitor 读某租户的自动盯盘开关（未配置默认开启——跟随平台总闸）。
func (uc *SystemSettingsUseCase) GetTenantAutoMonitor(ctx context.Context, tenantID string) (bool, error) {
	if uc.tenantSettingRepo == nil || tenantID == "" {
		return true, nil
	}
	s, err := uc.tenantSettingRepo.Get(ctx, tenantID, entity.TenantSettingKeyAutoMonitor)
	if err != nil {
		return true, nil // 未配置默认开启（平台总闸仍生效）
	}
	return s.Value != "false", nil
}

// SetTenantAutoMonitor 写某租户的自动盯盘开关。
func (uc *SystemSettingsUseCase) SetTenantAutoMonitor(ctx context.Context, tenantID string, enabled bool) error {
	if uc.tenantSettingRepo == nil {
		return fmt.Errorf("租户设置仓储未配置")
	}
	value := "true"
	if !enabled {
		value = "false"
	}
	return uc.tenantSettingRepo.Save(ctx, entity.TenantSetting{
		TenantID: tenantID,
		Key:      entity.TenantSettingKeyAutoMonitor,
		Value:    value,
	})
}

// ---- 租户级：自动盯盘配置（频率/采样/引擎/通知阈值）----

// GetTenantAutoMonitorConfig 读某租户的盯盘配置（未配置返回默认——默认保守省额度）。
func (uc *SystemSettingsUseCase) GetTenantAutoMonitorConfig(ctx context.Context, tenantID string) (entity.AutoMonitorConfig, error) {
	def := entity.DefaultAutoMonitorConfig()
	if uc.tenantSettingRepo == nil || tenantID == "" {
		return def, nil
	}
	s, err := uc.tenantSettingRepo.Get(ctx, tenantID, entity.TenantSettingKeyAutoMonitorConfig)
	if err != nil {
		return def, nil // 未配置 = 默认
	}
	var cfg entity.AutoMonitorConfig
	if json.Unmarshal([]byte(s.Value), &cfg) != nil {
		return def, nil // 配置损坏 = 默认
	}
	return cfg.Valid(), nil
}

// SetTenantAutoMonitorConfig 写某租户的盯盘配置（校验后落库）。
func (uc *SystemSettingsUseCase) SetTenantAutoMonitorConfig(ctx context.Context, tenantID string, cfg entity.AutoMonitorConfig) error {
	if uc.tenantSettingRepo == nil {
		return fmt.Errorf("租户设置仓储未配置")
	}
	cfg = cfg.Valid()
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("配置序列化失败: %w", err)
	}
	return uc.tenantSettingRepo.Save(ctx, entity.TenantSetting{
		TenantID: tenantID,
		Key:      entity.TenantSettingKeyAutoMonitorConfig,
		Value:    string(b),
	})
}

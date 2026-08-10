// Package systemsettings 实现"平台系统设置"用例。
//
// 职责：平台级运行时配置（system_settings 通用键值）的读写。
// 整洁架构：依赖 port.SystemSettingRepository，业务规则（键白名单/校验）在用例层。
package systemsettings

import (
	"context"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// SystemSettingsUseCase 平台系统设置用例（管理后台运行时开关）。
type SystemSettingsUseCase struct {
	settingRepo port.SystemSettingRepository
}

func NewSystemSettingsUseCase(settingRepo port.SystemSettingRepository) *SystemSettingsUseCase {
	return &SystemSettingsUseCase{settingRepo: settingRepo}
}

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

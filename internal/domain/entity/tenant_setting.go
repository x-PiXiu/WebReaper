package entity

import "time"

// TenantSetting 租户级设置（system_settings 是平台级；此处按租户隔离）。
//
// 设计动机：
//   - 平台级设置（system_settings）管"平台全局行为"（如自动盯盘总闸）
//   - 租户级设置（tenant_settings）管"商户个性化行为"（如我的品牌要不要自动监测）
//   - 多租户隔离铁律延伸：租户的配置只属于该租户
type TenantSetting struct {
	TenantID  string // 归属租户（复合主键一部分）
	Key       string // 配置键，如 "auto_monitor_enabled"
	Value     string // 配置值（"true"/"false" 或 JSON）
	UpdatedAt time.Time
}

// 租户级配置键常量
const (
	// TenantSettingKeyAutoMonitor 租户自动盯盘开关（"true"/"false"）。
	// 平台总闸开启时，仅对开启本开关的租户品牌执行每日自动监测（节省 LLM 额度）。
	TenantSettingKeyAutoMonitor = "auto_monitor_enabled"
)

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
	// TenantSettingKeyAutoMonitorConfig 自动盯盘配置（JSON：频率/采样/引擎/通知阈值）。
	TenantSettingKeyAutoMonitorConfig = "auto_monitor_config"
	// TenantSettingKeyAutoMonitorLastRun 上次自动盯盘执行时间（RFC3339；频率 gate 用）。
	TenantSettingKeyAutoMonitorLastRun = "auto_monitor_last_run"
)

// AutoMonitorConfig 自动盯盘配置（商户可自控，JSON 存 tenant_settings.auto_monitor_config）。
//
// 站在用户角度：开启自动盯盘后，系统按此配置每天/每 12 小时自动监测，
// 变化超阈值自动通知。默认值语义："保守省额度、清晰可预期"。
type AutoMonitorConfig struct {
	// Frequency 盯盘频率：daily（每天）/ half_day（每 12 小时）/ weekly（每周）。
	Frequency string `json:"frequency"`
	// SampleSize 每关键词采样次数（3/5/10——越多越准但越烧 token）。
	SampleSize int `json:"sample_size"`
	// EngineName 盯盘引擎（空=default 模拟引擎；多引擎为付费能力，暂不支持盯盘）。
	EngineName string `json:"engine_name"`
	// NotifyDropThreshold 提及率下降通知阈值（百分点；默认 20 = 下降 20pp 通知）。
	NotifyDropThreshold int `json:"notify_drop_threshold"`
	// NotifyOvertake 竞品反超通知开关（默认开）。
	NotifyOvertake bool `json:"notify_overtake"`
}

// DefaultAutoMonitorConfig 默认盯盘配置（每日 1 次、5 采样、default 引擎、降 20pp/反超通知）。
func DefaultAutoMonitorConfig() AutoMonitorConfig {
	return AutoMonitorConfig{
		Frequency:           "daily",
		SampleSize:          5,
		EngineName:          "",
		NotifyDropThreshold: 20,
		NotifyOvertake:      true,
	}
}

// Valid 配置合法性（非法字段回退默认，不阻断盯盘）。
func (c AutoMonitorConfig) Valid() AutoMonitorConfig {
	def := DefaultAutoMonitorConfig()
	if c.Frequency != "daily" && c.Frequency != "half_day" && c.Frequency != "weekly" {
		c.Frequency = def.Frequency
	}
	if c.SampleSize < 3 || c.SampleSize > 10 {
		c.SampleSize = def.SampleSize
	}
	if c.NotifyDropThreshold < 5 || c.NotifyDropThreshold > 80 {
		c.NotifyDropThreshold = def.NotifyDropThreshold
	}
	return c
}

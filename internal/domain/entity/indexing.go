package entity

import (
	"regexp"
	"time"
)

// ---- 收录管理领域实体（搜索引擎收录通知的运行时配置与审计）----

// SettingKeyIndexingConfig 收录配置在 system_settings 表的键
// （值 = IndexingConfig 的 JSON 序列化；运行时可调，覆盖 .env 启动兜底）。
const SettingKeyIndexingConfig = "indexing_config"

// IndexingConfig 是"收录通知"的运行时配置快照。
//
// 设计动机（运行时配置 vs 启动 env）：
//   收录渠道的凭据（IndexNow key / 百度 token）是运营高频维护项——
//   换 token 不应要求重启服务。配置存 system_settings（管理后台可读写），
//   读取时 DB 配置优先、.env 兜底（未配置 DB 也能用 env 启动）。
type IndexingConfig struct {
	IndexNowKey string `json:"index_now_key"` // IndexNow 密钥（8-128 个 a-zA-Z0-9-）
	BaiduSite   string `json:"baidu_site"`    // 百度已验证域名
	BaiduToken  string `json:"baidu_token"`   // 百度准入 token
	UpdatedAt   time.Time `json:"updated_at"`
}

// IsConfigured 是否有任一渠道启用。
func (c IndexingConfig) IsConfigured() bool {
	return c.IndexNowKey != "" || (c.BaiduSite != "" && c.BaiduToken != "")
}

// indexNowKeyPattern IndexNow key 格式（IndexNow 协议要求：8-128 个 a-zA-Z0-9-）。
var indexNowKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9-]{8,128}$`)

// ValidIndexNowKey 校验 IndexNow key 格式（领域规则，usecase 与 adapter 共用）。
func ValidIndexNowKey(key string) bool {
	return indexNowKeyPattern.MatchString(key)
}

// Validate 校验配置（非法返回错误描述；空配置合法——表示未启用）。
func (c IndexingConfig) Validate() error {
	if c.IndexNowKey != "" && !ValidIndexNowKey(c.IndexNowKey) {
		return &IndexingConfigError{msg: "IndexNow key 格式无效（需 8-128 个字母/数字/连字符）"}
	}
	if (c.BaiduSite == "") != (c.BaiduToken == "") {
		return &IndexingConfigError{msg: "百度 site 与 token 必须同时配置或同时为空"}
	}
	return nil
}

// IndexingConfigError 配置错误（显式类型便于上层识别）。
type IndexingConfigError struct{ msg string }

func (e *IndexingConfigError) Error() string { return e.msg }

// IndexingChannel 收录渠道标识。
type IndexingChannel string

// 渠道常量。
const (
	IndexingChannelIndexNow IndexingChannel = "indexnow" // Bing/Yandex/Naver
	IndexingChannelBaidu    IndexingChannel = "baidu"    // 百度主动推送
)

// IndexingSubmitLog 是一次收录提交的审计记录（排查"为什么没被收录"）。
type IndexingSubmitLog struct {
	ID          string
	Channel     IndexingChannel // indexnow / baidu
	URL         string          // 提交的页面 URL
	Status      string          // success / failed
	ErrorMsg    string          // 失败原因（成功为空）
	SubmittedAt time.Time
}

// ---- 平台设置键（system_settings 通用键值）----

// SettingKeyAutoMonitor 每日自动监测开关（"true"/"false"）。
// 管理后台运行时开关——开启后调度器每日对全平台品牌自动监测（趋势自动生长）。
const SettingKeyAutoMonitor = "auto_monitor_enabled"

// SettingKeyPaymentConfig 支付网关配置（JSON：pid/key/notify_url/return_url）。
// admin 后台运行时配置——支持多通道动态切换（当前 zpay）。
// 值为 JSON 字符串，由 BillingUseCase 解析为具体网关配置。
const SettingKeyPaymentConfig = "payment_config"
const SettingKeyBrowserHeaded = "browser_headed"

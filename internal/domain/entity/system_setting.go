package entity

import "time"

// SystemSetting 系统级配置项（key-value 形式，全局单例语义）。
//
// 设计动机（整洁架构）：
//   - 爬虫速率、robots 开关等运行时可调的配置，存数据库而非 .env，
//     让 UI 能动态修改、无需重启。
//   - 用通用 key-value 表承载，避免每加一个配置就建一张表。
//   - CrawlPolicy 等结构化配置序列化为 JSON 存 Value 字段。
type SystemSetting struct {
	Key       string    // 配置键，如 "crawl_policy"
	Value     string    // 配置值（结构化配置存 JSON）
	UpdatedAt time.Time
}

// 配置键常量
const (
	SettingKeyCrawlPolicy = "crawl_policy" // 爬虫限流策略（JSON 序列化的 CrawlPolicy）
)

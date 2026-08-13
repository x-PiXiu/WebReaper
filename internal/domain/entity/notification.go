package entity

import "time"

// Notification 站内通知（主动唤醒：SaaS 留存核心）。
//
// 设计动机（被动感知 → 主动唤醒）：
//   - 用户在平台内产生关键变化时主动通知（提及率显著下降/竞品反超/
//     自动复测完成/排期发布完成）
//   - 不依赖邮件/SMS（后续可扩展通知渠道，实体不变）
type Notification struct {
	ID        string
	TenantID  string // 多租户隔离
	Type      string // mention_drop / competitor_overtake / recheck_done / scheduled_publish / system
	Title     string
	Content   string
	Link      string // 跳转链接（如 /m/keywords）
	Read      bool
	CreatedAt time.Time
}

// NotificationType 通知类型常量。
const (
	NotificationTypeMentionDrop        = "mention_drop"        // 提及率显著下降
	NotificationTypeCompetitorOvertake = "competitor_overtake" // 竞品反超
	NotificationTypeRecheckDone        = "recheck_done"        // 自动复测完成
	NotificationTypeScheduledPublish   = "scheduled_publish"   // 排期发布完成
	NotificationTypeContentIndexed     = "content_indexed"     // 内容已被搜索引擎收录（付费说服力事件）
	NotificationTypeSystem             = "system"              // 系统通知
)

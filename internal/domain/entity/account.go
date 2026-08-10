package entity

import "time"

// 多平台发布账号域：围绕「平台账号（扫码绑定）→ cookie 复用 → 半自动发布」组织。
//
// 设计动机（整洁架构 + 策略/适配器模式）：
//   - 社媒平台（知乎/小红书/抖音）没有内容发布 API，只能通过浏览器登录态发布。
//   - 登录态靠 cookie 维持，cookie 只能通过真人扫码获取（无法自动登录）。
//   - cookie 是高敏感数据，必须加密存储（AES-GCM），绝不落明文。
//   - 用例层只依赖 port.QRLoginSession / port.PublishChannel 接口，
//     浏览器自动化（chromedp）和平台差异全部关在适配器层。
//
// 安全红线（实体层业务规则，不可漏）：
//   - CookieEncrypted 只存密文，明文 cookie 绝不落库。
//   - 多租户隔离：所有查询强制带 TenantID。

// 账号健康状态常量。
const (
	AccountHealthActive  = "active"  // cookie 有效，可发布
	AccountHealthExpired = "expired" // cookie 过期，需重新扫码
	AccountHealthBanned  = "banned"  // 账号被封禁，不可用
)

// 发布模式常量。
const (
	PublishModeSemiAuto = "semi-auto" // 半自动：系统生成内容+预填链接，用户手动确认发布
	PublishModeAuto     = "auto"      // 全自动：RPA 模拟浏览器点击发布（脆弱，后续扩展）
)

// 发布任务状态常量。
const (
	PublishStatusPending   = "pending"   // 待发布（已生成预填链接，等用户确认）
	PublishStatusRunning   = "running"   // 全自动发布中（chromedp 正在执行）
	PublishStatusPublished = "published" // 已发布
	PublishStatusFailed    = "failed"    // 发布失败
)

// Account 是商户绑定的社媒平台账号。
//
// 一个商户（Tenant）可绑定多个平台的多个账号（账号池）。
// cookie 过期后需重新扫码绑定。
type Account struct {
	ID              string    // 账号唯一 ID
	TenantID        string    // 租户隔离
	Platform        string    // 平台标识：zhihu / xiaohongshu / douyin / gongzhonghao
	DisplayName     string    // 账号显示名（如 "@某装修公司官方"）
	CookieEncrypted string    // AES-GCM 加密后的 cookie 密文（绝不存明文）
	Health          string    // 健康状态：active / expired / banned
	LoginMethod     string    // 登录方式：zhihu/wechat/qq/weibo（区分第三方登录）
	ExpiresAt       time.Time // 认证 cookie 的过期时间（到期需重新扫码）
	BoundAt         time.Time // 绑定时间（首次扫码成功）
	LastUsedAt      time.Time // 最后一次用于发布的时间
}

// IsValid 领域规则：账号必须有 ID、TenantID、Platform。
func (a Account) IsValid() bool {
	return a.ID != "" && a.TenantID != "" && a.Platform != ""
}

// IsHealthy 账号是否可用于发布（健康状态为 active）。
func (a Account) IsHealthy() bool {
	return a.Health == AccountHealthActive
}

// PublishJob 是一次发布任务的记录。
//
// 半自动模式下，Status 为 pending 时 ExternalURL 是预填好的发布页链接，
// 用户点击跳转到平台确认发布后，前端可标记为 published。
type PublishJob struct {
	ID          string    // 任务 ID
	TenantID    string    // 租户隔离
	AccountID   string    // 使用的账号 ID
	Platform    string    // 目标平台
	ContentID   string    // 关联的 OptimizedContent ID（内容工作台生成的内容）
	BrandID     string    // 品牌ID（发布效果追踪用——触发监测需要）
	Title       string    // 发布标题
	Content     string    // 发布正文
	Mode        string    // 发布模式：semi-auto / auto
	Status      string    // 任务状态：pending / running / published / failed
	ExternalURL string    // 外部链接（半自动=预填发布页URL；全自动=发布后的文章URL）
	ErrorMsg    string    // 失败原因
	CreatedAt   time.Time // 创建时间
	PublishedAt time.Time // 发布成功时间（status=published 时有值）
	PreMentionRate  float64 // 发布前品牌提及率（发布效果追踪用）
	PostMentionRate float64 // 发布后品牌提及率（发布效果追踪用）
	// ScheduledAt 排期发布时间（零值 = 立即发布）。
	// 将来时间 = 定时发送：任务落库 pending，调度任务到期后自动执行发布。
	ScheduledAt time.Time
}

// IsValid 领域规则。
func (j PublishJob) IsValid() bool {
	return j.ID != "" && j.TenantID != "" && j.Platform != ""
}

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

// 账号绑定方式常量（获客智能体：官方授权优先，浏览器扫码兜底）。
const (
	AccountAuthCookie = "cookie" // 浏览器扫码绑定（RPA 通道，cookie 维持登录态）
	AccountAuthOAuth  = "oauth"  // 平台官方 OAuth 授权（API 通道，token 维持登录态）
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
	CookieEncrypted string    // AES-GCM 加密后的 cookie 密文（绝不存明文；AuthType=cookie 时有值）
	Health          string    // 健康状态：active / expired / banned
	LoginMethod     string    // 登录方式：zhihu/wechat/qq/weibo（区分第三方登录）
	ExpiresAt       time.Time // 认证凭据过期时间（cookie 或 access_token 到期；到期需重新绑定/刷新）
	BoundAt         time.Time // 绑定时间（首次扫码/授权成功）
	LastUsedAt      time.Time // 最后一次用于发布的时间

	// ---- 官方 OAuth 授权（抖音等开放平台 API 通道）----
	// AuthType 绑定方式：cookie（扫码浏览器）/ oauth（官方授权）；空=cookie（向后兼容）。
	// 发布时按此路由：oauth → API 通道，cookie → RPA 通道。
	AuthType string
	// AccessTokenEnc / RefreshTokenEnc OAuth token 密文（AES-GCM，绝不存明文；AuthType=oauth 时有值）。
	AccessTokenEnc  string
	RefreshTokenEnc string
	// OpenID 平台用户唯一标识（OAuth 授权返回；同一应用内 open_id 稳定，用于识别重复绑定）。
	OpenID string
	// UnionID 开放平台维度用户标识（同一开发者主体下跨应用稳定）。
	// 跨端账号打通的钥匙：网站应用/小程序/移动应用是三个不同的 client_key，
	// 同一抖音用户在各应用下 open_id 不同，但 UnionID 相同——未来接入小程序/App 时
	// 靠它把三端的"同一个人"合并成一个账号。
	UnionID string
	// RefreshExpiresAt refresh_token 过期时间（抖音：授权起 30 天，续期一次 +30 天、
	// 最多 5 次=最长 195 天；到期前健康检查自动续期，续尽则需用户重新授权）。
	// ExpiresAt 存的是 access_token 过期时间（抖音 15 天，到期前 48h 自动刷新）。
	RefreshExpiresAt time.Time

	// Role 账号角色：merchant（商户自有，发布用）/ platform（平台工作账号，只读搜索用）。
	// 空值 = merchant（向后兼容）。搜索等只读操作优先用 platform 账号——
	// 风控风险集中到平台可控的账号，不消耗商户账号的信任额度。
	Role string
}

// AccountRole 常量。
const (
	AccountRoleMerchant = "merchant"
	AccountRolePlatform = "platform"
)

// IsPlatform 是否平台工作账号。
func (a Account) IsPlatform() bool { return a.Role == AccountRolePlatform }

// IsOAuth 是否官方 OAuth 授权账号（发布走 API 通道而非浏览器 RPA）。
func (a Account) IsOAuth() bool { return a.AuthType == AccountAuthOAuth }

// OAuthToken 平台 OAuth 授权换取的凭据（抖音等开放平台，授权码模式）。
type OAuthToken struct {
	AccessToken      string // 接口调用凭据（加密落库）
	RefreshToken     string // 刷新凭据（加密落库；access_token 过期前用它续期）
	ExpiresIn        int    // access_token 有效期（秒）
	RefreshExpiresIn int    // refresh_token 有效期（秒）
	OpenID           string // 用户在应用内的唯一标识
	UnionID          string // 用户在开放平台维度的唯一标识（可能为空）
	Scope            string // 实际授予的权限作用域
}

// OAuthUserInfo OAuth 授权用户的公开信息（账号显示名用）。
type OAuthUserInfo struct {
	Nickname string // 昵称（账号显示名首选）
	Avatar   string // 头像 URL
	UnionID  string // 开放平台维度用户标识（跨应用稳定——三端账号打通用）
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
// 内容形态常量（Platform × ContentType 双维度——同一平台支持多种形态）
const (
	ContentTypeImage   = "image"   // 图文笔记（小红书主流）
	ContentTypeVideo   = "video"   // 视频笔记
	ContentTypeArticle = "article" // 长文章
	ContentTypeAudio   = "audio"   // 音频
)

// ChannelConstraints 平台内容形态约束（发布前校验的单一事实源——规则归平台适配器声明，
// 前端据此动态生成检查清单，不再硬编码）。零值 = 无约束。按内容形态分别声明。
type ChannelConstraints struct {
	TitleMaxRunes int `json:"title_max_runes,omitempty"` // 标题最大字数（0=不限）
	MinImages     int `json:"min_images,omitempty"`      // 最少配图数（0=不要求）
	MinVideos     int `json:"min_videos,omitempty"`      // 最少视频数（0=不要求）
}

// ChannelInfo 发布通道能力清单（GET /geo/publish/channels 下发——前端能力驱动的数据源：
// 选内容形态 → 过滤可用平台 → 按约束生成检查清单。新平台注册即自动出现，前端零改动）。
type ChannelInfo struct {
	Platform     string                        `json:"platform"`
	Name         string                        `json:"name"`           // 商户友好名（知乎/小红书）
	ContentTypes []string                      `json:"content_types"`  // 支持的内容形态（article/image/video/audio）
	Constraints  map[string]ChannelConstraints `json:"constraints,omitempty"` // key=内容形态
	SemiAuto     bool                          `json:"semi_auto"`      // 半自动可用
	Auto         bool                          `json:"auto"`           // 全自动可用
}

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
	ScheduledAt time.Time
	// StoreAddress 门店地址（本地生活 P3：内容层本地曝光信号）。
	StoreAddress string

	// ContentType 内容形态（Platform × ContentType 双维度）。
	// 同一平台支持多种形态：小红书 image/video/article/audio；知乎默认 article。
	// 空 = 向后兼容（知乎走 article；小红书走 image 默认）。
	ContentType string
	// MediaURLs 媒体文件 URL 列表（图文=图片[]、视频=[mp4]、音频=[mp3]）。
	// RPA 发布时下载到本地临时文件再上传到平台。空=纯文本发布（知乎）。
	// 持久化为 JSON 文本（mapper 层处理序列化）。
	MediaURLs []string
	// CoverURL 封面图 URL（视频/长文/音频需要；图文取首图）。
	CoverURL string
	// Transport 实际执行通道（link/rpa/api——发布域三轴重构后按次记录，
	// 降级链"启动前短路切换"的实际落点；空=历史数据）。
	Transport string
}

// IsValid 领域规则。
func (j PublishJob) IsValid() bool {
	return j.ID != "" && j.TenantID != "" && j.Platform != ""
}

// VideoMetric 视频互动数据快照（数据回读——每日任务/手动刷新写入时间序列）。
// 详情 Drawer 画趋势、作品数据页汇总最新值。
type VideoMetric struct {
	ID          string    // vm-{nano}
	TenantID    string    // 租户隔离
	JobID       string    // 发布任务 ID
	Platform    string    // douyin / kuaishou / ...
	VideoID     string    // 平台内视频 ID（aweme_id）
	Views       int64     // 播放
	Likes       int64     // 点赞
	Comments    int64     // 评论
	Shares      int64     // 分享
	CollectedAt time.Time // 采集时间
}

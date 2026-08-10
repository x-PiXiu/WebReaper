package port

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
)

// ---- 多平台发布账号域接口（用例层声明，适配器实现）----
//
// 设计要点（整洁架构 / 依赖倒置）：
//   - 用例层只依赖这些接口，绝不 import chromedp / 平台 SDK。
//   - 浏览器自动化、cookie 加密、平台差异全部关在适配器层。
//   - 换浏览器引擎（chromedp→playwright）或换平台 = 重写适配器，业务零改动。

// AccountRepository 平台账号仓储（多租户）。
type AccountRepository interface {
	Save(ctx context.Context, a entity.Account) error
	FindByID(ctx context.Context, tenantID, id string) (entity.Account, error)
	ListByTenant(ctx context.Context, tenantID string) ([]entity.Account, error)
	ListByPlatform(ctx context.Context, tenantID, platform string) ([]entity.Account, error)
	ListAll(ctx context.Context) ([]entity.Account, error)             // 不带租户过滤（定时任务用）
	UpdateHealth(ctx context.Context, id, health string) error         // 更新健康状态（定时检查用）
	UpdateLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error // 更新最后使用时间（账号池调度用）
	Delete(ctx context.Context, tenantID, id string) error
}

// AccountPool 账号池调度接口。
// 用于全自动发布时自动选择最优账号（最久未使用优先，避免单号高频被封）。
type AccountPool interface {
	// Acquire 借出一个健康账号（按调度策略选择，如最久未使用优先）。
	Acquire(ctx context.Context, tenantID, platform string) (entity.Account, error)
	// Release 归还账号（更新 LastUsedAt）。
	Release(ctx context.Context, account entity.Account) error
}

// PublishJobRepository 发布任务记录仓储（多租户）。
type PublishJobRepository interface {
	Save(ctx context.Context, j entity.PublishJob) error
	UpdateStatus(ctx context.Context, tenantID, id, status, externalURL, errorMsg string) error
	ListByTenant(ctx context.Context, tenantID string, limit int) ([]entity.PublishJob, error)
	// Count 统计发布任务总数（平台总览用，admin 看全局）。
	Count(ctx context.Context) (int, error)
	// ListScheduledDue 列出已到期未执行的排期任务（调度任务用，全租户）。
	ListScheduledDue(ctx context.Context, before time.Time) ([]entity.PublishJob, error)
}

// QRLoginResult 是一次扫码登录轮询的结果。
type QRLoginResult struct {
	Status      string    // preparing / waiting / scanned / success / expired / error
	QRImage     string    // 二维码截图 base64（preparing 阶段后台截好图后通过此字段返回）
	Cookie      string    // 仅 status=success 时有值（原始 cookie 字符串，由用例层加密入库）
	ExpiresAt   time.Time // 仅 status=success 时有值（认证 cookie 的过期时间）
	AccountName string    // 仅 status=success 时有值（登录后的账号显示名）
	Error       string    // 仅 status=error 时有值（失败原因）
}

// QRLoginSession 扫码登录会话接口（浏览器自动化适配器实现）。
//
// 生命周期（异步设计——StartLogin 快速返回，不阻塞 HTTP 请求）：
//  1. StartLogin：开浏览器 → 导航到登录页 → 立即返回 sessionID（不等二维码）
//  2. 后台 goroutine：轮询直到二维码出现 → 截图 → 继续检测登录成功
//  3. PollStatus：前端每 2s 轮询，返回当前状态 + 二维码图片（如果已截到）
//     - preparing：浏览器已开，二维码还没截到（前端继续轮询）
//     - waiting：二维码已截到（通过 QRImage 返回），等待用户扫码
//     - scanned：已扫码，等待手机确认
//     - success：登录成功（通过 Cookie 返回）
//  4. Cleanup：关闭浏览器释放资源
type QRLoginSession interface {
	// StartLogin 启动浏览器并打开平台登录页，立即返回会话 ID（不等二维码出现）。
	// method 指定登录方式（如 "zhihu"/"wechat"/"qq"/"weibo"），空=平台默认扫码。
	StartLogin(ctx context.Context, platform, method string) (sessionID string, err error)
	// PollStatus 轮询登录状态和二维码图片。
	PollStatus(ctx context.Context, sessionID string) (QRLoginResult, error)
	// Cleanup 关闭浏览器会话，释放资源。
	Cleanup(ctx context.Context, sessionID string) error

}

// MonitorTrigger 监测触发接口（发布效果追踪用）。
// PublishUseCase 通过此接口在发布前取基线、发布后触发监测，对比前后提及率。
// 避免 usecase 之间直接依赖（PublishUseCase 不 import geo 包）。
type MonitorTrigger interface {
	// TriggerMonitor 触发一次品牌监测，返回监测后的平均提及率（0~1）。
	TriggerMonitor(ctx context.Context, tenantID, brandID string) (float64, error)
	// BaselineRate 取品牌最近一次监测的平均提及率（发布前基线；无监测记录返回 0）。
	BaselineRate(ctx context.Context, tenantID, brandID string) (float64, error)
}

// CookieVault cookie 加密存储接口（敏感数据隔离）。
//
// 实现用 AES-GCM 加密 + 密钥从配置读取。
// 后续可升级为 KMS 管理主密钥 + 租户级派生密钥，接口不变。
type CookieVault interface {
	Encrypt(cookie string) (string, error)
	Decrypt(encCookie string) (string, error)
}

// PublishChannel 发布通道接口（策略模式）。
//
// 每个社媒平台实现此接口。用例层通过 PublishChannelRegistry 按平台名获取通道。
// 半自动模式返回预填好的发布页 URL；全自动模式用浏览器模拟点击（后续扩展）。
type PublishChannel interface {
	// PublishSemiAuto 半自动发布：生成预填内容的发布页 URL，用户手动确认。
	PublishSemiAuto(ctx context.Context, job entity.PublishJob, account entity.Account) (externalURL string, err error)
	// Platform 返回平台标识（zhihu / xiaohongshu / ...）。
	Platform() string
	// SupportedMediaType 返回支持的内容类型（text / image / video）。
	SupportedMediaType() []string
}

// AutoPublishChannel 全自动发布通道（可选接口）。
//
// 不修改现有 PublishChannel 接口——半自动通道不实现此接口，零破坏。
// 用例层用类型断言检查通道是否支持全自动：
//   if autoCh, ok := ch.(AutoPublishChannel); ok { autoCh.PublishAuto(...) }
//
// 全自动模式用 chromedp 注入 cookie → 导航到发布页 → 自动填充标题/正文 → 点击发送。
type AutoPublishChannel interface {
	// PublishAuto 全自动发布：注入 cookie + 浏览器自动填充内容并点击发送。
	// cookie 是解密后的原始 cookie 字符串（name=value; name2=value2; ...）。
	// 返回发布后的文章 URL。
	PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (articleURL string, err error)
}

// PublishChannelRegistry 发布通道注册表（工厂模式）。
// 新增平台 = 注册新适配器，开闭原则。
type PublishChannelRegistry interface {
	Get(platform string) (PublishChannel, error)
	List() []PublishChannel
}

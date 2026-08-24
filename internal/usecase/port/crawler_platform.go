package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- 爬虫平台接口（灵感广场数据采集）----
// 设计参考：MediaCrawler 的 AbstractCrawler + AbstractApiClient 分离模式
// 整洁架构定位：usecase/port 层，定义接口由 adapter 层实现

// CrawlerPlatform 社交媒体平台爬虫接口（编排层）。
//
// 设计（策略模式）：每个平台实现此接口，用例层通过平台名动态选择。
// 新增平台 = 新增 adapter 实现 + 注册一行，用例零改动。
//
// 内部分工（参考 MediaCrawler core.py + client.py 分离模式）：
//   - Crawler 实现本接口（编排：搜索循环、分页、去重、排序）
//   - Client 是 Crawler 的内部组件（执行：httpx 请求、签名、代理轮换）
type CrawlerPlatform interface {
	// Platform 返回平台标识（"douyin"/"kuaishou"/"bilibili"）。
	Platform() string

	// Search 关键词搜索热门视频。
	// 内部流程：构造参数 → 签名 → httpx 调 API → 解析 JSON → 去重排序
	Search(ctx context.Context, opts entity.SearchOptions) ([]entity.CrawledVideo, error)

	// GetDetail 获取单个视频详情（含完整互动数据）。
	GetDetail(ctx context.Context, videoID string) (*entity.CrawledVideo, error)

	// RefreshMetrics 批量刷新视频的实时指标（播放量/点赞/评论等）。
	RefreshMetrics(ctx context.Context, videoIDs []string) ([]entity.MetricsUpdate, error)

	// IsAlive 检测平台连接是否正常（Cookie 有效性）。
	IsAlive(ctx context.Context) bool

	// GetCapabilities 返回平台支持的能力（搜索/详情/评论/刷新）。
	GetCapabilities() entity.PlatformCapabilities
}

// CrawlerClient 平台 API 客户端接口（HTTP 请求层）。
//
// 设计（参考 MediaCrawler AbstractApiClient）：
//   - 每个平台实现此接口，负责 HTTP 请求细节
//   - Crawler 通过此接口调用平台 API，不直接处理 HTTP
//   - 职责：签名生成、代理轮换、Cookie 管理
type CrawlerClient interface {
	// Search 发起搜索 API 请求（单次，不含分页循环）。
	Search(ctx context.Context, keyword string, offset int, limit int) (*entity.SearchResponse, error)

	// GetDetail 发起视频详情 API 请求。
	GetDetail(ctx context.Context, videoID string) (*entity.CrawledVideo, error)

	// GetComments 发起评论 API 请求（支持分页）。
	GetComments(ctx context.Context, videoID string, cursor string, limit int) (*entity.CommentResponse, error)

	// UpdateCookies 从浏览器上下文提取并更新 Cookie。
	UpdateCookies(ctx context.Context, cookies string, userAgent string) error

	// IsAlive 检测 Cookie 是否有效。
	IsAlive(ctx context.Context) bool
}

// CrawlerAccountRepository 平台方账号仓储。
type CrawlerAccountRepository interface {
	// Save 保存账号（新增或更新）。
	Save(ctx context.Context, account entity.CrawlerAccount) error
	// FindByID 根据 ID 查询账号。
	FindByID(ctx context.Context, id int64) (entity.CrawlerAccount, error)
	// ListByPlatform 按平台查询账号列表。
	ListByPlatform(ctx context.Context, platform string) ([]entity.CrawlerAccount, error)
	// ListAll 查询所有账号。
	ListAll(ctx context.Context) ([]entity.CrawlerAccount, error)
	// Delete 删除账号。
	Delete(ctx context.Context, id int64) error
	// UpdateStatus 更新账号状态。
	UpdateStatus(ctx context.Context, id int64, status string) error
	// UpdateHealth 更新健康检查结果。
	UpdateHealth(ctx context.Context, id int64, result string) error
	// IncrementUsage 增加使用次数。
	IncrementUsage(ctx context.Context, id int64) error
	// ResetDailyUsage 重置每日使用次数（定时任务调用）。
	ResetDailyUsage(ctx context.Context) error
	// SelectAvailable 选择一个可用账号（负载均衡：使用次数最少的）。
	SelectAvailable(ctx context.Context, platform string) (*entity.CrawlerAccount, error)
}

// CrawlerConfigRepository 爬虫配置仓储。
type CrawlerConfigRepository interface {
	// Save 保存配置（新增或更新）。
	Save(ctx context.Context, config entity.CrawlerConfig) error
	// FindByPlatform 根据平台查询配置。
	FindByPlatform(ctx context.Context, platform string) (entity.CrawlerConfig, error)
	// ListAll 查询所有配置。
	ListAll(ctx context.Context) ([]entity.CrawlerConfig, error)
	// Delete 删除配置。
	Delete(ctx context.Context, platform string) error
	// UpdateLastCrawled 更新最后爬取时间。
	UpdateLastCrawled(ctx context.Context, platform string) error
	// UpdateLastError 更新最后错误信息。
	UpdateLastError(ctx context.Context, platform string, errMsg string) error
}

// CrawlerTaskLogRepository 采集任务日志仓储。
type CrawlerTaskLogRepository interface {
	// Save 保存任务日志。
	Save(ctx context.Context, log entity.CrawlerTaskLog) error
	// FindByID 根据 ID 查询任务日志。
	FindByID(ctx context.Context, id int64) (entity.CrawlerTaskLog, error)
	// ListByPlatform 按平台查询任务日志。
	ListByPlatform(ctx context.Context, platform string, limit int) ([]entity.CrawlerTaskLog, error)
	// ListAll 查询所有任务日志。
	ListAll(ctx context.Context, limit int) ([]entity.CrawlerTaskLog, error)
	// UpdateStatus 更新任务状态。
	UpdateStatus(ctx context.Context, id int64, status string, errMsg string) error
	// UpdateResult 更新任务结果（videos_found/videos_new/videos_updated）。
	UpdateResult(ctx context.Context, id int64, found, new, updated int) error
}

// InspirationVideoRepository 灵感视频仓储。
type InspirationVideoRepository interface {
	// SaveBatch 批量保存视频（去重：按 platform + platform_video_id）。
	SaveBatch(ctx context.Context, videos []entity.InspirationVideo) (newCount int, err error)
	// List 查询灵感视频列表（支持分页、排序、筛选）。
	List(ctx context.Context, brandID, platform, keyword, sortBy string, page, pageSize int) ([]entity.InspirationVideo, int, error)
	// FindByID 根据 ID 查询视频。
	FindByID(ctx context.Context, id string) (entity.InspirationVideo, error)
	// UpdateMetrics 更新视频的互动指标。
	UpdateMetrics(ctx context.Context, videoID string, metrics entity.MetricsUpdate) error
	// Delete 删除视频。
	Delete(ctx context.Context, id string) error
	// CountByPlatform 按平台统计视频数量。
	CountByPlatform(ctx context.Context) ([]PlatformCount, error)
	// CountByBrand 按品牌统计视频数量。
	CountByBrand(ctx context.Context) ([]BrandCount, error)
}

// PlatformCount 平台统计结果。
type PlatformCount struct {
	Platform string `json:"platform"`
	Count    int    `json:"count"`
}

// BrandCount 品牌统计结果。
type BrandCount struct {
	BrandID   string  `json:"brand_id"`
	BrandName string  `json:"brand_name"`
	Count     int     `json:"count"`
	AvgScore  float64 `json:"avg_viral_score"`
}

// BrandInspirationRepository 品牌-视频关联仓储。
type BrandInspirationRepository interface {
	// Link 建立品牌-视频关联（幂等）。
	Link(ctx context.Context, brandID, videoID, keyword string) error
	// Unlink 解除关联。
	Unlink(ctx context.Context, brandID, videoID string) error
	// ListByBrand 查询品牌关联的视频 ID 列表。
	ListByBrand(ctx context.Context, brandID string) ([]string, error)
	// ListByVideo 查询视频关联的品牌 ID 列表。
	ListByVideo(ctx context.Context, videoID string) ([]string, error)
}

// ---- 爬虫管理器接口 ----

// CrawlerManager 爬虫管理器（管理后台动态控制）。
type CrawlerManager interface {
	// GetPlatform 获取平台爬虫实例。
	GetPlatform(ctx context.Context, platform string) (CrawlerPlatform, error)
	// ListPlatforms 列出所有已注册的平台。
	ListPlatforms(ctx context.Context) []PlatformInfo
	// GetConfig 获取平台爬虫配置。
	GetConfig(ctx context.Context, platform string) (*entity.CrawlerConfig, error)
	// UpdateConfig 更新平台爬虫配置（热生效）。
	UpdateConfig(ctx context.Context, platform string, config entity.CrawlerConfig) error
	// TriggerCrawl 手动触发采集。
	TriggerCrawl(ctx context.Context, brandID string, force bool) (string, error)
}

// PlatformInfo 平台信息。
type PlatformInfo struct {
	Platform     string                   `json:"platform"`
	Enabled      bool                     `json:"enabled"`
	Capabilities entity.PlatformCapabilities `json:"capabilities"`
	AccountCount int                      `json:"account_count"`
	HealthyCount int                      `json:"healthy_count"`
}

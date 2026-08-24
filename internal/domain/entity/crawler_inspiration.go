package entity

import "time"

// ---- 爬虫平台相关实体（灵感广场数据采集）----
// 设计参考：MediaCrawler 的 SocialVideo / CrawledVideo 模式
// 整洁架构定位：domain/entity 层，零外部依赖

// CrawledVideo 爬取到的视频数据（平台无关，适配器层转换后统一格式）。
type CrawledVideo struct {
	Platform     string    `json:"platform"`      // douyin/kuaishou/bilibili
	VideoID      string    `json:"video_id"`       // 平台视频 ID
	Title        string    `json:"title"`          // 标题
	Description  string    `json:"description"`    // 描述/文案
	CoverURL     string    `json:"cover_url"`      // 封面 URL
	VideoURL     string    `json:"video_url"`      // 视频链接
	Author       string    `json:"author"`         // 作者名称
	AuthorAvatar string    `json:"author_avatar"`  // 作者头像
	Duration     int       `json:"duration"`       // 时长（秒）
	PublishTime  time.Time `json:"publish_time"`   // 发布时间
	PlayCount    int64     `json:"play_count"`     // 播放量
	DiggCount    int64     `json:"digg_count"`     // 点赞数
	CommentCount int64     `json:"comment_count"`  // 评论数
	ShareCount   int64     `json:"share_count"`    // 分享数
	CollectCount int64     `json:"collect_count"`  // 收藏数
	Topics       []string  `json:"topics"`         // 话题标签
	MusicName    string    `json:"music_name"`     // 背景音乐
	MusicAuthor  string    `json:"music_author"`   // 音乐作者
}

// MetricsUpdate 指标更新（用于实时刷新旧数据的互动指标）。
type MetricsUpdate struct {
	VideoID      string `json:"video_id"`
	PlayCount    int64  `json:"play_count"`
	DiggCount    int64  `json:"digg_count"`
	CommentCount int64  `json:"comment_count"`
	ShareCount   int64  `json:"share_count"`
	CollectCount int64  `json:"collect_count"`
}

// SearchOptions 搜索选项。
type SearchOptions struct {
	Keyword     string `json:"keyword"`      // 搜索关键词
	Limit       int    `json:"limit"`        // 结果数量限制
	SortBy      string `json:"sort_by"`      // 排序：popular/latest
	PublishTime string `json:"publish_time"` // 时间过滤：day/week/month/all
	Offset      int    `json:"offset"`       // 分页偏移
}

// PlatformCapabilities 平台能力声明。
type PlatformCapabilities struct {
	SupportSearch   bool `json:"support_search"`   // 支持关键词搜索
	SupportDetail   bool `json:"support_detail"`   // 支持视频详情
	SupportComments bool `json:"support_comments"` // 支持评论获取
	SupportRefresh  bool `json:"support_refresh"`  // 支持指标刷新
	SupportCreator  bool `json:"support_creator"`  // 支持创作者主页
	MaxSearchLimit  int  `json:"max_search_limit"` // 单次搜索最大结果数
	RateLimitPerMin int  `json:"rate_limit_per_min"` // 每分钟请求限制
}

// SearchResponse 搜索响应（Client → Crawler 传递）。
type SearchResponse struct {
	Videos   []CrawledVideo `json:"videos"`
	HasMore  bool           `json:"has_more"`
	Cursor   string         `json:"cursor"`   // 分页游标
	Total    int            `json:"total"`    // 总数（部分平台不提供）
}

// CommentResponse 评论响应。
type CommentResponse struct {
	Comments []Comment `json:"comments"`
	HasMore  bool      `json:"has_more"`
	Cursor   string    `json:"cursor"`
}

// Comment 评论数据。
type Comment struct {
	CommentID string    `json:"comment_id"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	LikeCount int64     `json:"like_count"`
	ReplyCount int64    `json:"reply_count"`
	CreatedAt time.Time `json:"created_at"`
}

// ---- 爬虫配置相关实体 ----

// CrawlerAccount 平台方账号（管理员维护，用于数据爬取）。
type CrawlerAccount struct {
	ID                 int64      `json:"id"`
	Platform           string     `json:"platform"`             // douyin/kuaishou/bilibili
	AccountName        string     `json:"account_name"`         // 账号名称（管理员备注）
	CookieEncrypted    string     `json:"cookie_encrypted"`     // AES-256-GCM 加密的 Cookie
	UserAgent          string     `json:"user_agent"`
	ProxyAddress       string     `json:"proxy_address"`        // 专属代理地址
	Status             string     `json:"status"`               // active/expired/banned
	LastUsedAt         *time.Time `json:"last_used_at"`
	LastHealthCheckAt  *time.Time `json:"last_health_check_at"`
	HealthCheckResult  string     `json:"health_check_result"`  // healthy/unhealthy/unknown
	DailyUsageCount    int        `json:"daily_usage_count"`
	DailyUsageLimit    int        `json:"daily_usage_limit"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// CrawlerAccount 状态常量。
const (
	CrawlerAccountActive  = "active"
	CrawlerAccountExpired = "expired"
	CrawlerAccountBanned  = "banned"
)

// 健康检查结果常量。
const (
	HealthUnknown   = "unknown"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

// CrawlerConfig 爬虫配置（DB 驱动，管理后台可改）。
type CrawlerConfig struct {
	ID                  int64     `json:"id"`
	Platform            string    `json:"platform"`
	TenantID            string    `json:"tenant_id"`
	Enabled             bool      `json:"enabled"`
	SearchKeywords      []string  `json:"search_keywords"`
	ExtraKeywords       []string  `json:"extra_keywords"`
	CrawlIntervalMinutes int      `json:"crawl_interval_minutes"`
	MaxResults          int       `json:"max_results"`
	SortBy              string    `json:"sort_by"`
	PublishTime         string    `json:"publish_time"`
	EnableComments      bool      `json:"enable_comments"`
	EnableRefresh       bool      `json:"enable_refresh"`
	RefreshIntervalHours int      `json:"refresh_interval_hours"`
	RateLimitPerMin     int       `json:"rate_limit_per_min"`
	ProxyEnabled        bool      `json:"proxy_enabled"`
	MaxRetryCount       int       `json:"max_retry_count"`
	LastCrawledAt       *time.Time `json:"last_crawled_at"`
	LastError           string    `json:"last_error"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// CrawlerTaskLog 采集任务日志。
type CrawlerTaskLog struct {
	ID           int64      `json:"id"`
	TaskID       string     `json:"task_id"`
	Platform     string     `json:"platform"`
	BrandID      string     `json:"brand_id"`
	TriggerType  string     `json:"trigger_type"`  // scheduled/manual/first_time
	Status       string     `json:"status"`         // running/success/failed
	KeywordsUsed []string   `json:"keywords_used"`
	VideosFound  int        `json:"videos_found"`
	VideosNew    int        `json:"videos_new"`
	VideosUpdated int       `json:"videos_updated"`
	ErrorMessage string     `json:"error_message"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	DurationMs   int        `json:"duration_ms"`
}

// 任务状态常量。
const (
	TaskLogRunning = "running"
	TaskLogSuccess = "success"
	TaskLogFailed  = "failed"
)

// 触发类型常量。
const (
	TriggerScheduled = "scheduled"
	TriggerManual    = "manual"
	TriggerFirstTime = "first_time"
)

// ---- 灵感数据实体 ----

// InspirationVideo 灵感视频（持久化到 DB，用户端只读）。
type InspirationVideo struct {
	ID              string    `json:"id"`
	Platform        string    `json:"platform"`
	PlatformVideoID string    `json:"platform_video_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	CoverURL        string    `json:"cover_url"`
	VideoURL        string    `json:"video_url"`
	Author          string    `json:"author"`
	AuthorAvatar    string    `json:"author_avatar"`
	Duration        int       `json:"duration"`
	PublishTime     time.Time `json:"publish_time"`
	PlayCount       int64     `json:"play_count"`
	DiggCount       int64     `json:"digg_count"`
	CommentCount    int64     `json:"comment_count"`
	ShareCount      int64     `json:"share_count"`
	CollectCount    int64     `json:"collect_count"`
	Topics          []string  `json:"topics"`
	MusicName       string    `json:"music_name"`
	MusicAuthor     string    `json:"music_author"`
	Sentiment       string    `json:"sentiment"`
	ViralScore      float64   `json:"viral_score"`
	IsPinned        bool      `json:"is_pinned"`
	IsRecommended   bool      `json:"is_recommended"`
	AdminNote       string    `json:"admin_note"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastRefreshedAt *time.Time `json:"last_refreshed_at"`
}

// BrandInspiration 品牌-视频关联。
type BrandInspiration struct {
	ID             int64     `json:"id"`
	BrandID        string    `json:"brand_id"`
	VideoID        string    `json:"video_id"`
	SearchKeyword  string    `json:"search_keyword"`
	RelevanceScore float64   `json:"relevance_score"`
	CreatedAt      time.Time `json:"created_at"`
}

// CalculateViralScore 计算爆款指数（0-100）。
//
// 算法：综合互动率 = (点赞×40% + 评论×25% + 分享×20% + 收藏×15%) / 播放量 × 100
// 归一化：假设 5% 互动率 = 100 分
func CalculateViralScore(play, digg, comment, share, collect int64) float64 {
	if play == 0 {
		return 0
	}
	engageRate := float64(digg)*0.4 + float64(comment)*0.25 + float64(share)*0.2 + float64(collect)*0.15
	rate := engageRate / float64(play) * 100
	score := rate / 5.0 * 100
	if score > 100 {
		score = 100
	}
	// 保留一位小数
	return float64(int(score*10)) / 10
}

// CrawledVideoToInspiration 将爬取的视频转换为灵感视频实体。
func CrawledVideoToInspiration(v CrawledVideo) InspirationVideo {
	return InspirationVideo{
		Platform:        v.Platform,
		PlatformVideoID: v.VideoID,
		Title:           v.Title,
		Description:     v.Description,
		CoverURL:        v.CoverURL,
		VideoURL:        v.VideoURL,
		Author:          v.Author,
		AuthorAvatar:    v.AuthorAvatar,
		Duration:        v.Duration,
		PublishTime:     v.PublishTime,
		PlayCount:       v.PlayCount,
		DiggCount:       v.DiggCount,
		CommentCount:    v.CommentCount,
		ShareCount:      v.ShareCount,
		CollectCount:    v.CollectCount,
		Topics:          v.Topics,
		MusicName:       v.MusicName,
		MusicAuthor:     v.MusicAuthor,
		Sentiment:       "neutral",
		ViralScore:      CalculateViralScore(v.PlayCount, v.DiggCount, v.CommentCount, v.ShareCount, v.CollectCount),
	}
}

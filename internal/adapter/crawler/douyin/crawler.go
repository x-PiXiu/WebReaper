package crawler

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// DouyinCrawler 抖音平台爬虫（实现 port.CrawlerPlatform）。
//
// 设计（参考 MediaCrawler core.py + client.py 分离模式）：
//   - Crawler 负责编排（搜索循环、分页、去重、排序）
//   - 内部复用 douyinweb.Searcher（已有的 XHR 方式）
//   - 后续可升级为 httpx API 直接调用
type DouyinCrawler struct {
	searcher douyinwebSearcher
	config   CrawlerConfig
}

// douyinwebSearcher 是 douyinweb.Searcher 的接口投影（解耦具体实现）。
type douyinwebSearcher interface {
	SearchHotVideos(ctx context.Context, tenantID, plat, keyword string, limit int) ([]port.SocialVideo, error)
	GetVideoDetail(ctx context.Context, tenantID, plat, videoID string) (*port.SocialVideo, error)
	IsAlive(ctx context.Context, tenantID, plat string) bool
}

// CrawlerConfig 爬虫配置。
type CrawlerConfig struct {
	MaxResults    int
	SortBy        string
	PublishTime   string
	LimitPerQuery int
}

// NewDouyinCrawler 创建抖音爬虫。
func NewDouyinCrawler(searcher douyinwebSearcher, config *CrawlerConfig) *DouyinCrawler {
	cfg := CrawlerConfig{
		MaxResults:    20,
		SortBy:        "popular",
		PublishTime:   "week",
		LimitPerQuery: 10,
	}
	if config != nil {
		cfg = *config
	}
	return &DouyinCrawler{
		searcher: searcher,
		config:   cfg,
	}
}

func (c *DouyinCrawler) Platform() string { return "douyin" }

// Search 关键词搜索热门视频。
func (c *DouyinCrawler) Search(ctx context.Context, opts entity.SearchOptions) ([]entity.CrawledVideo, error) {
	if opts.Keyword == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	limit := opts.Limit
	if limit <= 0 || limit > c.config.MaxResults {
		limit = c.config.MaxResults
	}

	videos, err := c.searcher.SearchHotVideos(ctx, "", "douyin", opts.Keyword, limit)
	if err != nil {
		return nil, fmt.Errorf("抖音搜索失败: %w", err)
	}

	result := make([]entity.CrawledVideo, 0, len(videos))
	for _, v := range videos {
		result = append(result, socialVideoToCrawled(v))
	}
	return result, nil
}

// GetDetail 获取单个视频详情。
func (c *DouyinCrawler) GetDetail(ctx context.Context, videoID string) (*entity.CrawledVideo, error) {
	if videoID == "" {
		return nil, fmt.Errorf("视频 ID 不能为空")
	}
	v, err := c.searcher.GetVideoDetail(ctx, "", "douyin", videoID)
	if err != nil {
		return nil, fmt.Errorf("获取抖音视频详情失败: %w", err)
	}
	result := socialVideoToCrawled(*v)
	return &result, nil
}

// RefreshMetrics 批量刷新视频的实时指标。
func (c *DouyinCrawler) RefreshMetrics(ctx context.Context, videoIDs []string) ([]entity.MetricsUpdate, error) {
	updates := make([]entity.MetricsUpdate, 0, len(videoIDs))
	for _, id := range videoIDs {
		v, err := c.searcher.GetVideoDetail(ctx, "", "douyin", id)
		if err != nil {
			continue
		}
		updates = append(updates, entity.MetricsUpdate{
			VideoID:      id,
			PlayCount:    int64(v.PlayCount),
			DiggCount:    int64(v.DiggCount),
			CommentCount: int64(v.CommentCount),
			ShareCount:   int64(v.ShareCount),
		})
	}
	return updates, nil
}

// IsAlive 检测平台连接是否正常。
func (c *DouyinCrawler) IsAlive(ctx context.Context) bool {
	return c.searcher.IsAlive(ctx, "", "douyin")
}

// GetCapabilities 返回平台支持的能力。
func (c *DouyinCrawler) GetCapabilities() entity.PlatformCapabilities {
	return entity.PlatformCapabilities{
		SupportSearch:   true,
		SupportDetail:   true,
		SupportComments: true,
		SupportRefresh:  true,
		SupportCreator:  false,
		MaxSearchLimit:  c.config.MaxResults,
		RateLimitPerMin: 10,
	}
}

// socialVideoToCrawled 将 port.SocialVideo 转换为 entity.CrawledVideo。
//
// 字段映射（参考 MediaCrawler store/douyin/__init__.py update_douyin_aweme）：
//   - 搜索 API 返回：标题/点赞/评论/分享/收藏/封面/视频URL/时长
//   - 搜索 API 不返回：播放量（需详情 API 补充）
func socialVideoToCrawled(v port.SocialVideo) entity.CrawledVideo {
	c := entity.CrawledVideo{
		Platform:     v.Platform,
		VideoID:      v.VideoID,
		Title:        v.Desc,
		Description:  v.Desc,
		CoverURL:     v.CoverURL,
		VideoURL:     v.VideoURL,
		Author:       v.Author,
		AuthorAvatar: v.AuthorAvatar,
		Duration:     v.Duration,
		PlayCount:    int64(v.PlayCount),
		DiggCount:    int64(v.DiggCount),
		CommentCount: int64(v.CommentCount),
		ShareCount:   int64(v.ShareCount),
		CollectCount: int64(v.CollectCount),
	}
	if v.CreateTime > 0 {
		c.PublishTime = time.Unix(v.CreateTime, 0)
	}
	return c
}

package entity

import "context"

// HotVideo 热门同款视频（前端卡片 + DB 持久化 + 用例间传递）。
type HotVideo struct {
	TenantID    string `json:"tenant_id,omitempty"`
	BrandID     string `json:"brand_id,omitempty"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Platform    string `json:"platform"`
	HotPoint    string `json:"hot_point"`
	Topic       string `json:"topic"`
	CoverURL    string `json:"cover_url,omitempty"`
	Author      string `json:"author,omitempty"`
	PlayCount   int64  `json:"play_count,omitempty"`
	DiggCount   int64  `json:"digg_count,omitempty"`
	CommentCount int64 `json:"comment_count,omitempty"`
	PublishTime string `json:"publish_time,omitempty"` // RFC3339
	Source      string `json:"source,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// HotVideoListOptions 热门视频列表查询选项（搜索/排序/分页）。
type HotVideoListOptions struct {
	Platform string
	Keyword  string
	SortBy   string // publish_time / digg_count / play_count / comment_count / created_at
	Limit    int
	Offset   int
}

// HotVideoRepository 热门视频持久化仓储。
type HotVideoRepository interface {
	SaveBatch(ctx context.Context, videos []HotVideo) (int, error)
	List(ctx context.Context, brandID string, opts HotVideoListOptions) ([]HotVideo, int, error)
}

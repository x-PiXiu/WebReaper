package port

import "context"

// DouyinVideo 抖音站内视频（web 接口结构化结果）。
type DouyinVideo struct {
	AwemeID       string // 视频 ID（详情查询/数据回读的主键）
	Desc          string // 视频文案（标题）
	Author        string // 作者昵称
	URL           string // https://www.douyin.com/video/{aweme_id}
	PlayCount     int    // 播放
	DiggCount     int    // 点赞
	CommentCount  int    // 评论
	ShareCount    int    // 分享
	CreateTime    int64  // 发布时间（unix 秒）
}

// DouyinSearcher 抖音站内搜索/详情（MediaCrawler 协议知识的 Go 复刻）。
//
// 机制：chromedp 携登录 cookie 打开 douyin.com → 页面内同源 fetch web 接口
// （免 a_bogus 签名的端点；cookie/referer 由浏览器环境天然携带）。
// 实现自包含账号选择（租户下任一健康抖音 cookie 账号——搜索只读，任意登录态可用），
// 调用方不感知 cookie 细节。
//
// 落点：热门同款 tab 数据源（真实爆款+数据）、数据回读（GetVideoDetail 按视频 ID 直查）。
type DouyinSearcher interface {
	// SearchHotVideos 站内搜索"一周内 + 最多点赞"的热门视频（最近很火语义）。
	SearchHotVideos(ctx context.Context, tenantID, keyword string, limit int) ([]DouyinVideo, error)
	// GetVideoDetail 单视频详情（含最新 play/like/comment/share——数据回读用）。
	GetVideoDetail(ctx context.Context, tenantID, awemeID string) (*DouyinVideo, error)
}

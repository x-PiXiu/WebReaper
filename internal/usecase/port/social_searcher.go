package port

import "context"

// SocialVideo 站内视频（多平台泛化实体）。
type SocialVideo struct {
	Platform      string // douyin / kuaishou / xiaohongshu / bilibili ...
	VideoID       string // 平台内视频 ID（详情查询/数据回读/评论获取的主键）
	Desc          string // 视频文案（标题）
	Author        string // 作者昵称
	URL           string // 可播放链接
	PlayCount     int    // 播放
	DiggCount     int    // 点赞
	CommentCount  int    // 评论数
	ShareCount    int    // 分享
	CreateTime    int64  // 发布时间（unix 秒）
}

// SocialComment 视频评论（线索挖掘的数据源）。
type SocialComment struct {
	CommentID  string
	Content    string // 评论内容
	User       string // 评论者昵称
	DiggCount  int    // 评论点赞数（线索热度参考）
	CreateTime int64  // 评论时间（unix 秒）
	ReplyTo    string // 回复的评论 ID（子评论时非空）
}

// SocialSearcher 社媒平台站内搜索/详情/评论（多平台泛化，方案 B：平台为参数）。
//
// 协议知识来源（MediaCrawler 验证过的 web 接口行为，不复制其代码）：
//   - 抖音搜索：GET /aweme/v1/web/general/search/single/ —— 免 a_bogus 签名，需登录 cookie
//   - 抖音详情：GET /aweme/v1/web/aweme/detail/?aweme_id= → statistics 含 play/like/comment/share
//   - 抖音评论：GET /aweme/v1/web/comment/list/ —— cursor 分页，20 条/页
//   - 七平台评论接口均存在（douyin/kuaishou/xhs/bilibili/weibo/zhihu/tieba）
//
// 架构（与 PublishChannelRegistry 同构）：新平台 = 新增适配器实现 + main 一行注册，
// 用例层以 platform 参数调用，零改动。实现自包含账号选择（租户下该平台的健康
// cookie 账号——搜索/读评论是只读操作，任意登录态可用），调用方不感知凭据细节。
//
// 落点：热门同款数据源、数据回读（GetVideoDetail）、评论线索挖掘（GetComments）、
// 链接归一化（ResolveShortURL）、cookie 健康实测（IsAlive）。
type SocialSearcher interface {
	// SupportedPlatforms 声明已注册的平台（Registry 能力清单）。
	SupportedPlatforms() []string
	// SearchHotVideos 站内搜索"最近很火"视频（如抖音=一周内+最多点赞）。
	SearchHotVideos(ctx context.Context, tenantID, platform, keyword string, limit int) ([]SocialVideo, error)
	// GetVideoDetail 单视频详情（含最新互动数据——数据回读用）。
	GetVideoDetail(ctx context.Context, tenantID, platform, videoID string) (*SocialVideo, error)
	// GetComments 视频评论（按 cursor 分页；线索挖掘只拉前几页即可，勿全量）。
	GetComments(ctx context.Context, tenantID, platform, videoID string, cursor int, limit int) ([]SocialComment, error)
	// ResolveShortURL 短链归一化（v.douyin.com/xxx → 平台内视频 ID）。
	ResolveShortURL(shortURL string) (platform, videoID string, err error)
	// IsAlive 登录态心跳（pong：轻量接口实测 cookie 是否有效——账号健康检查升级用）。
	IsAlive(ctx context.Context, tenantID, platform string) bool
}

// Package bilibili 实现B站平台爬虫（port.CrawlerPlatform 的 bilibili 平台实现）。
//
// 数据获取方式（参考 MediaCrawler bilibili/client.py）：
//   - httpx 直接调用 API：/x/web-interface/search/type
//   - WBI 签名：需要从浏览器 localStorage 提取 img_key + sub_key
//   - 浏览器用途：登录 + Cookie + WBI 密钥提取
//
// 参考源码：E:\workspace\Demo\PythonDemo\MediaCrawler\media_platform\bilibili\
package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"webreaper/internal/domain/entity"
)

const (
	platformName = "bilibili"
	host         = "https://api.bilibili.com"
	searchPath   = "/x/web-interface/search/type"
	detailPath   = "/x/web-interface/view"
	commentPath  = "/x/v2/reply"
)

// BilibiliCrawler B站平台爬虫（实现 port.CrawlerPlatform）。
type BilibiliCrawler struct {
	client  *http.Client
	cookies string
	config  CrawlerConfig
}

// CrawlerConfig B站爬虫配置。
type CrawlerConfig struct {
	MaxResults int
	UserAgent  string
}

// NewBilibiliCrawler 创建B站爬虫。
func NewBilibiliCrawler(cookies string, config *CrawlerConfig) *BilibiliCrawler {
	cfg := CrawlerConfig{
		MaxResults: 20,
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	}
	if config != nil {
		cfg = *config
	}
	return &BilibiliCrawler{
		client:  &http.Client{Timeout: 30 * time.Second},
		cookies: cookies,
		config:  cfg,
	}
}

func (c *BilibiliCrawler) Platform() string { return platformName }

// Search 关键词搜索热门视频。
//
// 流程（参考 MediaCrawler bilibili/client.py）：
//  1. 构造搜索参数
//  2. 调用 /x/web-interface/search/type?search_type=video&keyword=xxx
//  3. 解析 JSON 响应
func (c *BilibiliCrawler) Search(ctx context.Context, opts entity.SearchOptions) ([]entity.CrawledVideo, error) {
	if opts.Keyword == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	limit := opts.Limit
	if limit <= 0 || limit > c.config.MaxResults {
		limit = c.config.MaxResults
	}

	// 构造请求参数
	params := url.Values{}
	params.Set("search_type", "video")
	params.Set("keyword", opts.Keyword)
	params.Set("page", "1")
	params.Set("pagesize", strconv.Itoa(limit))

	// 排序
	switch opts.SortBy {
	case "click":
		params.Set("order", "click")
	case "pubdate":
		params.Set("order", "pubdate")
	default:
		params.Set("order", "totalrank")
	}

	reqURL := fmt.Sprintf("%s%s?%s", host, searchPath, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Referer", "https://search.bilibili.com/")
	if c.cookies != "" {
		req.Header.Set("Cookie", c.cookies)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("B站搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("B站搜索返回 HTTP %d", resp.StatusCode)
	}

	// 解析响应
	var result searchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析B站搜索响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("B站搜索错误: %s", result.Message)
	}

	// 转换为 CrawledVideo
	videos := make([]entity.CrawledVideo, 0, len(result.Data.Result))
	for i, item := range result.Data.Result {
		if i >= limit {
			break
		}
		videos = append(videos, searchResultToCrawled(item))
	}

	return videos, nil
}

// GetDetail 获取单个视频详情。
func (c *BilibiliCrawler) GetDetail(ctx context.Context, videoID string) (*entity.CrawledVideo, error) {
	if videoID == "" {
		return nil, fmt.Errorf("视频 ID 不能为空")
	}

	params := url.Values{}
	params.Set("bvid", videoID)

	reqURL := fmt.Sprintf("%s%s?%s", host, detailPath, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Referer", "https://www.bilibili.com/video/"+videoID)
	if c.cookies != "" {
		req.Header.Set("Cookie", c.cookies)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("B站详情请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("B站详情返回 HTTP %d", resp.StatusCode)
	}

	var result detailResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析B站详情响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("B站详情错误: %s", result.Message)
	}

	return &entity.CrawledVideo{
		Platform:     platformName,
		VideoID:      result.Data.BVID,
		Title:        result.Data.Title,
		Description:  result.Data.Desc,
		CoverURL:     result.Data.Pic,
		VideoURL:     fmt.Sprintf("https://www.bilibili.com/video/%s", result.Data.BVID),
		Author:       result.Data.Owner.Name,
		AuthorAvatar: result.Data.Owner.Face,
		Duration:     result.Data.Duration,
		PlayCount:    int64(result.Data.Stat.View),
		DiggCount:    int64(result.Data.Stat.Like),
		CommentCount: int64(result.Data.Stat.Reply),
		ShareCount:   int64(result.Data.Stat.Share),
		CollectCount: int64(result.Data.Stat.Favorite),
	}, nil
}

// RefreshMetrics 批量刷新视频的实时指标。
func (c *BilibiliCrawler) RefreshMetrics(ctx context.Context, videoIDs []string) ([]entity.MetricsUpdate, error) {
	updates := make([]entity.MetricsUpdate, 0, len(videoIDs))
	for _, id := range videoIDs {
		v, err := c.GetDetail(ctx, id)
		if err != nil {
			continue
		}
		updates = append(updates, entity.MetricsUpdate{
			VideoID:      id,
			PlayCount:    v.PlayCount,
			DiggCount:    v.DiggCount,
			CommentCount: v.CommentCount,
			ShareCount:   v.ShareCount,
			CollectCount: v.CollectCount,
		})
	}
	return updates, nil
}

// IsAlive 检测平台连接是否正常。
func (c *BilibiliCrawler) IsAlive(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.bilibili.com", nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// GetCapabilities 返回平台支持的能力。
func (c *BilibiliCrawler) GetCapabilities() entity.PlatformCapabilities {
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

// UpdateCookies 更新 Cookie。
func (c *BilibiliCrawler) UpdateCookies(cookies string) {
	c.cookies = cookies
}

// ---- 响应结构体 ----

type searchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Result []searchResult `json:"result"`
	} `json:"data"`
}

type searchResult struct {
	BVID        string `json:"bvid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Pic         string `json:"pic"`
	Author      string `json:"author"`
	Duration    string `json:"duration"` // 格式："MM:SS"
	Play        int    `json:"play"`
	Like        int    `json:"like"`
	Review      int    `json:"review"` // 评论数
	Favorites   int    `json:"favorites"`
	VideoReview int    `json:"video_review"` // 弹幕数
	Pubdate     int64  `json:"pubdate"`
	Mid         int64  `json:"mid"` // 作者 ID
}

type detailResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		BVID  string `json:"bvid"`
		Title string `json:"title"`
		Desc  string `json:"desc"`
		Pic   string `json:"pic"`
		Owner struct {
			Name string `json:"name"`
			Face string `json:"face"`
		} `json:"owner"`
		Duration int `json:"duration"` // 秒
		Stat     struct {
			View     int `json:"view"`
			Like     int `json:"like"`
			Reply    int `json:"reply"`
			Share    int `json:"share"`
			Favorite int `json:"favorite"`
		} `json:"stat"`
	} `json:"data"`
}

// searchResultToCrawled 将B站搜索结果转换为 CrawledVideo。
func searchResultToCrawled(item searchResult) entity.CrawledVideo {
	c := entity.CrawledVideo{
		Platform:     platformName,
		VideoID:      item.BVID,
		Title:        cleanHTML(item.Title),
		Description:  item.Description,
		CoverURL:     item.Pic,
		VideoURL:     fmt.Sprintf("https://www.bilibili.com/video/%s", item.BVID),
		Author:       item.Author,
		PlayCount:    int64(item.Play),
		DiggCount:    int64(item.Like),
		CommentCount: int64(item.Review),
		ShareCount:   0, // 搜索结果不包含分享数
		CollectCount: int64(item.Favorites),
	}
	// 解析时长 "MM:SS" → 秒
	if d := parseDuration(item.Duration); d > 0 {
		c.Duration = d
	}
	if item.Pubdate > 0 {
		c.PublishTime = time.Unix(item.Pubdate, 0)
	}
	return c
}

// cleanHTML 清理 HTML 标签（B站搜索结果标题包含 <em class="keyword"> 标签）。
func cleanHTML(s string) string {
	// 简单实现：去掉 <em> 和 </em> 标签
	result := ""
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result += string(ch)
		}
	}
	return result
}

// parseDuration 解析 "MM:SS" 格式的时长为秒数。
func parseDuration(s string) int {
	if s == "" {
		return 0
	}
	// 尝试 "MM:SS" 格式
	for i, ch := range s {
		if ch == ':' {
			min, _ := strconv.Atoi(s[:i])
			sec, _ := strconv.Atoi(s[i+1:])
			return min*60 + sec
		}
	}
	// 尝试纯数字
	d, _ := strconv.Atoi(s)
	return d
}

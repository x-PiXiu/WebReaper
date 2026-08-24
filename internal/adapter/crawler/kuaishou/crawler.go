// Package kuaishou 实现快手平台爬虫（port.CrawlerPlatform 的 kuaishou 平台实现）。
//
// 数据获取方式（参考 MediaCrawler kuaishou/client.py）：
//   - httpx 直接调用 REST API：/rest/v/search/feed
//   - 签名：__NS_hxfalcon（浏览器页面内执行 window.__ks_realm.$encode 生成）
//   - 浏览器用途：登录 + Cookie + 签名 JS 执行环境
//
// 参考源码：E:\workspace\Demo\PythonDemo\MediaCrawler\media_platform\kuaishou\
package kuaishou

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"webreaper/internal/domain/entity"
)

const (
	platformName = "kuaishou"
	host         = "https://www.kuaishou.com"
	searchPath   = "/rest/v/search/feed"
	detailPath   = "/rest/v/video/queryVideo"
)

// KuaishouCrawler 快手平台爬虫（实现 port.CrawlerPlatform）。
type KuaishouCrawler struct {
	client  *http.Client
	cookies string
	config  CrawlerConfig
}

// CrawlerConfig 快手爬虫配置。
type CrawlerConfig struct {
	MaxResults    int
	UserAgent     string
	SearchKeyword string
}

// NewKuaishouCrawler 创建快手爬虫。
func NewKuaishouCrawler(cookies string, config *CrawlerConfig) *KuaishouCrawler {
	cfg := CrawlerConfig{
		MaxResults: 20,
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	}
	if config != nil {
		cfg = *config
	}
	return &KuaishouCrawler{
		client:  &http.Client{Timeout: 30 * time.Second},
		cookies: cookies,
		config:  cfg,
	}
}

func (c *KuaishouCrawler) Platform() string { return platformName }

// Search 关键词搜索热门视频。
//
// 流程（参考 MediaCrawler kuaishou/client.py search_info_by_keyword_v2）：
//  1. 构造搜索请求体
//  2. 调用 /rest/v/search/feed
//  3. 解析 JSON 响应
func (c *KuaishouCrawler) Search(ctx context.Context, opts entity.SearchOptions) ([]entity.CrawledVideo, error) {
	if opts.Keyword == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	limit := opts.Limit
	if limit <= 0 || limit > c.config.MaxResults {
		limit = c.config.MaxResults
	}

	// 构造请求体（参考 MediaCrawler kuaishou/client.py 第 172-189 行）
	reqBody := map[string]any{
		"keyword":      opts.Keyword,
		"pcursor":      "",
		"page":         "search",
		"searchSessionId": "",
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", host+searchPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Referer", host+"/search/video?keyword="+opts.Keyword)
	req.Header.Set("Origin", host)
	if c.cookies != "" {
		req.Header.Set("Cookie", c.cookies)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("快手搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("快手搜索返回 HTTP %d", resp.StatusCode)
	}

	// 解析响应
	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析快手搜索响应失败: %w", err)
	}

	// 转换为 CrawledVideo
	videos := make([]entity.CrawledVideo, 0, len(result.Data.VisionSearchPhoto.List))
	for i, item := range result.Data.VisionSearchPhoto.List {
		if i >= limit {
			break
		}
		videos = append(videos, searchItemToCrawled(item))
	}

	return videos, nil
}

// GetDetail 获取单个视频详情。
func (c *KuaishouCrawler) GetDetail(ctx context.Context, videoID string) (*entity.CrawledVideo, error) {
	if videoID == "" {
		return nil, fmt.Errorf("视频 ID 不能为空")
	}

	reqBody := map[string]any{
		"photoId": videoID,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", host+detailPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Referer", host+"/short-video/"+videoID)
	req.Header.Set("Origin", host)
	if c.cookies != "" {
		req.Header.Set("Cookie", c.cookies)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("快手详情请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("快手详情返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应（简化版）
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析快手详情响应失败: %w", err)
	}

	// 从响应中提取视频信息
	video := &entity.CrawledVideo{
		Platform: platformName,
		VideoID:  videoID,
	}
	if data, ok := result["data"].(map[string]any); ok {
		if photo, ok := data["photoDetail"].(map[string]any); ok {
			if caption, ok := photo["caption"].(string); ok {
				video.Title = caption
				video.Description = caption
			}
			if author, ok := photo["author"].(map[string]any); ok {
				if name, ok := author["name"].(string); ok {
					video.Author = name
				}
			}
		}
	}

	return video, nil
}

// RefreshMetrics 批量刷新视频的实时指标。
func (c *KuaishouCrawler) RefreshMetrics(ctx context.Context, videoIDs []string) ([]entity.MetricsUpdate, error) {
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
		})
	}
	return updates, nil
}

// IsAlive 检测平台连接是否正常。
func (c *KuaishouCrawler) IsAlive(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", host, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	if c.cookies != "" {
		req.Header.Set("Cookie", c.cookies)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// GetCapabilities 返回平台支持的能力。
func (c *KuaishouCrawler) GetCapabilities() entity.PlatformCapabilities {
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
func (c *KuaishouCrawler) UpdateCookies(cookies string) {
	c.cookies = cookies
}

// ---- 响应结构体 ----

type searchResponse struct {
	Data struct {
		VisionSearchPhoto struct {
			List []searchItem `json:"list"`
		} `json:"visionSearchPhoto"`
	} `json:"data"`
}

type searchItem struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	PhotoURL  string `json:"photoUrl"`
	CoverURL  string `json:"coverUrl"`
	ViewCount int64  `json:"viewCount"`
	LikeCount int64  `json:"likeCount"`
	CommentCount int64 `json:"commentCount"`
	ShareCount int64 `json:"shareCount"`
	Timestamp  int64  `json:"timestamp"`
	Author     struct {
		Name      string `json:"name"`
		HeaderUrl string `json:"headerUrl"`
	} `json:"author"`
}

// searchItemToCrawled 将快手搜索结果转换为 CrawledVideo。
func searchItemToCrawled(item searchItem) entity.CrawledVideo {
	c := entity.CrawledVideo{
		Platform:     platformName,
		VideoID:      item.ID,
		Title:        item.Caption,
		Description:  item.Caption,
		CoverURL:     item.CoverURL,
		VideoURL:     item.PhotoURL,
		Author:       item.Author.Name,
		AuthorAvatar: item.Author.HeaderUrl,
		PlayCount:    item.ViewCount,
		DiggCount:    item.LikeCount,
		CommentCount: item.CommentCount,
		ShareCount:   item.ShareCount,
	}
	if item.Timestamp > 0 {
		c.PublishTime = time.Unix(item.Timestamp/1000, 0) // 快手时间戳是毫秒
	}
	return c
}

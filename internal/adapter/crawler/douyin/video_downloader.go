package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// VideoDownloader 抖音视频下载器。
//
// 功能：
//   - 从抖音视频链接解析视频 ID
//   - 调用 API 获取视频详情（含播放地址）
//   - 下载视频文件到本地
//
// 支持的链接格式：
//   - https://www.douyin.com/video/7525538910311632128
//   - https://v.douyin.com/iF12345ABC/
//   - 7525538910311632128（纯数字 ID）
type VideoDownloader struct {
	client     *http.Client
	cookies    string
	userAgent  string
}

// NewVideoDownloader 创建视频下载器。
func NewVideoDownloader(cookies string) *VideoDownloader {
	return &VideoDownloader{
		client:    &http.Client{Timeout: 60 * time.Second},
		cookies:   cookies,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	}
}

// VideoInfo 视频信息。
type VideoInfo struct {
	VideoID      string `json:"video_id"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	PlayCount    int64  `json:"play_count"`
	DiggCount    int64  `json:"digg_count"`
	CommentCount int64  `json:"comment_count"`
	Duration     int    `json:"duration"`
	CoverURL     string `json:"cover_url"`
	VideoURL     string `json:"video_url"`     // 原始视频页面 URL
	PlayURL      string `json:"play_url"`      // 视频播放地址（可下载）
}

// ParseVideoID 从视频链接解析视频 ID。
//
// 支持格式：
//   - https://www.douyin.com/video/7525538910311632128
//   - https://v.douyin.com/iF12345ABC/
//   - 7525538910311632128
func ParseVideoID(url string) (string, error) {
	url = strings.TrimSpace(url)

	// 纯数字 ID
	if matched, _ := regexp.MatchString(`^\d+$`, url); matched {
		return url, nil
	}

	// 标准链接：/video/数字
	re := regexp.MustCompile(`/video/(\d+)`)
	if matches := re.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1], nil
	}

	// 短链接：v.douyin.com/xxx（需要 HTTP 跟随重定向）
	if strings.Contains(url, "v.douyin.com") {
		return resolveShortURL(url)
	}

	return "", fmt.Errorf("无法解析视频 ID: %s", url)
}

// resolveShortURL 解析短链接获取视频 ID。
func resolveShortURL(shortURL string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不自动跟随重定向
		},
	}

	resp, err := client.Get(shortURL)
	if err != nil {
		return "", fmt.Errorf("解析短链接失败: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("短链接无重定向")
	}

	// 从重定向 URL 中提取视频 ID
	re := regexp.MustCompile(`/video/(\d+)`)
	if matches := re.FindStringSubmatch(location); len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("重定向 URL 中未找到视频 ID: %s", location)
}

// GetVideoInfo 获取视频信息（包括播放地址）。
func (d *VideoDownloader) GetVideoInfo(ctx context.Context, videoID string) (*VideoInfo, error) {
	// 调用抖音 API 获取视频详情
	apiURL := fmt.Sprintf("https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=%s&aid=6383&device_platform=webapp", videoID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", d.userAgent)
	req.Header.Set("Referer", "https://www.douyin.com/video/"+videoID)
	if d.cookies != "" {
		req.Header.Set("Cookie", d.cookies)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 解析响应
	var result struct {
		AwemeDetail struct {
			AwemeID  string `json:"aweme_id"`
			Desc     string `json:"desc"`
			Author   struct {
				Nickname string `json:"nickname"`
			} `json:"author"`
			Video struct {
				PlayAddr struct {
					URLList []string `json:"url_list"`
				} `json:"play_addr"`
				Cover struct {
					URLList []string `json:"url_list"`
				} `json:"cover"`
				Duration int `json:"duration"`
			} `json:"video"`
			Statistics struct {
				PlayCount    int64 `json:"play_count"`
				DiggCount    int64 `json:"digg_count"`
				CommentCount int64 `json:"comment_count"`
				ShareCount   int64 `json:"share_count"`
			} `json:"statistics"`
		} `json:"aweme_detail"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	detail := result.AwemeDetail
	if detail.AwemeID == "" {
		return nil, fmt.Errorf("未找到视频信息")
	}

	// 提取播放地址
	playURL := ""
	if len(detail.Video.PlayAddr.URLList) > 0 {
		playURL = detail.Video.PlayAddr.URLList[0]
	}

	// 提取封面地址
	coverURL := ""
	if len(detail.Video.Cover.URLList) > 0 {
		coverURL = detail.Video.Cover.URLList[0]
	}

	return &VideoInfo{
		VideoID:      detail.AwemeID,
		Title:        detail.Desc,
		Author:       detail.Author.Nickname,
		PlayCount:    detail.Statistics.PlayCount,
		DiggCount:    detail.Statistics.DiggCount,
		CommentCount: detail.Statistics.CommentCount,
		Duration:     detail.Video.Duration / 1000, // 毫秒转秒
		CoverURL:     coverURL,
		VideoURL:     fmt.Sprintf("https://www.douyin.com/video/%s", detail.AwemeID),
		PlayURL:      playURL,
	}, nil
}

// DownloadVideo 下载视频文件。
func (d *VideoDownloader) DownloadVideo(ctx context.Context, videoID string) ([]byte, string, error) {
	// 1. 获取视频信息
	info, err := d.GetVideoInfo(ctx, videoID)
	if err != nil {
		return nil, "", err
	}

	if info.PlayURL == "" {
		return nil, "", fmt.Errorf("视频播放地址为空")
	}

	// 2. 下载视频
	req, err := http.NewRequestWithContext(ctx, "GET", info.PlayURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("User-Agent", d.userAgent)
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %w", err)
	}

	return data, info.Title, nil
}

// DownloadFromURL 从 URL 直接下载视频（一步完成）。
func (d *VideoDownloader) DownloadFromURL(ctx context.Context, videoURL string) ([]byte, string, error) {
	videoID, err := ParseVideoID(videoURL)
	if err != nil {
		return nil, "", err
	}
	return d.DownloadVideo(ctx, videoID)
}

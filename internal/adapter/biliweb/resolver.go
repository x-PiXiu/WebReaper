// Package biliweb B站视频链接解析（文案提取链路的 resolver 扩展）。
//
// 链路（全部使用公开 API，无需登录 cookie——与抖音 resolver 的账号基建不同）：
//	① BV 号 + 分P 号解析（/video/BVxxx?p=N）
//	② view API 拿分P 的 cid 与标题
//	③ playurl API（qn=16 流畅 360P + fnval=0 → durl mp4 直链——360P 对 ASR 足够，
//	  体积小下载快；高清晰度需要 wbi 签名/登录，故不取）
//	④ 下载时带 Referer 防盗链头
package biliweb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const platform = "bilibili"

// bvidRe BV 号（BV 开头 10 位字母数字）。
var bvidRe = regexp.MustCompile(`/video/(BV[0-9A-Za-z]+)`)

// pageRe 分P 参数（?p=2）。
var pageRe = regexp.MustCompile(`[?&]p=(\d+)`)

// Resolver B站链接解析器。
type Resolver struct {
	client *http.Client
}

var _ interface {
	SupportedPlatforms() []string
	Resolve(ctx context.Context, tenantID, rawURL string) (string, string, string, string, error)
} = (*Resolver)(nil)

// NewResolver 创建 B站解析器。
func NewResolver() *Resolver {
	return &Resolver{client: &http.Client{Timeout: 15 * time.Second}}
}

// SupportedPlatforms 支持的平台。
func (r *Resolver) SupportedPlatforms() []string { return []string{platform} }

// IsBilibiliLink 是否B站视频链接（www.bilibili.com/video/BV… 或 b23.tv 短链）。
func IsBilibiliLink(rawURL string) bool {
	return strings.Contains(rawURL, "bilibili.com/video/BV") ||
		strings.Contains(rawURL, "b23.tv")
}

type viewResp struct {
	Code int `json:"code"`
	Data struct {
		Title string `json:"title"`
		Pages []struct {
			CID  int64  `json:"cid"`
			Part string `json:"part"`
			Page int    `json:"page"`
		} `json:"pages"`
	} `json:"data"`
}

type playResp struct {
	Code int `json:"code"`
	Data struct {
		Durl []struct {
			URL  string `json:"url"`
			Size int64  `json:"size"`
		} `json:"durl"`
	} `json:"data"`
}

// Resolve B站链接 → 播放直链（mp4 durl）+ 标题 + 平台。
func (r *Resolver) Resolve(ctx context.Context, tenantID, rawURL string) (string, string, string, string, error) {
	// b23.tv 短链 → 跟随重定向拿最终地址（不读体）
	if strings.Contains(rawURL, "b23.tv") {
		final, err := r.followRedirect(ctx, rawURL)
		if err != nil {
			return "", "", "", "", fmt.Errorf("B站短链解析失败: %w", err)
		}
		rawURL = final
	}
	if !IsBilibiliLink(rawURL) {
		return "", "", "", "", fmt.Errorf("非B站视频链接")
	}
	m := bvidRe.FindStringSubmatch(rawURL)
	if m == nil {
		return "", "", "", "", fmt.Errorf("链接里找不到 BV 号（可能已删除或非视频内容）")
	}
	bvid := m[1]

	// 分P（默认 1）
	page := 1
	if pm := pageRe.FindStringSubmatch(rawURL); pm != nil {
		if p, err := strconv.Atoi(pm[1]); err == nil && p > 0 {
			page = p
		}
	}

	// ① view API：cid + 标题
	vr := &viewResp{}
	if _, err := r.getJSON(ctx, "https://api.bilibili.com/x/web-interface/view?bvid="+bvid, vr); err != nil {
		return "", "", "", "", fmt.Errorf("B站视频信息获取失败: %w", err)
	}
	if vr.Code != 0 || len(vr.Data.Pages) == 0 {
		return "", "", "", "", fmt.Errorf("B站视频不存在或无分P信息（code=%d）", vr.Code)
	}
	if page > len(vr.Data.Pages) {
		page = 1 // 超范围回落第 1P
	}
	pg := vr.Data.Pages[page-1]
	title := vr.Data.Title
	if len(vr.Data.Pages) > 1 {
		title = fmt.Sprintf("%s（%s）", title, pg.Part)
	}

	// ② playurl API：360P mp4 直链（公开、无需登录）
	var pr playResp
	playURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?bvid=%s&cid=%d&qn=16&fnval=0&platform=html5", bvid, pg.CID)
	if _, err := r.getJSON(ctx, playURL, &pr); err != nil {
		return "", "", "", "", fmt.Errorf("B站播放地址获取失败: %w", err)
	}
	if pr.Code != 0 || len(pr.Data.Durl) == 0 || pr.Data.Durl[0].URL == "" {
		return "", "", "", "", fmt.Errorf("B站无公开播放地址（可能为会员专属或已下架，code=%d）", pr.Code)
	}
	return pr.Data.Durl[0].URL, title, platform, "", nil
}

// getJSON 带 Referer/UA 请求 B站公开 API 并解析。
func (r *Resolver) getJSON(ctx context.Context, apiURL string, out any) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return out, json.NewDecoder(resp.Body).Decode(out)
}

// followRedirect 短链 302 → 最终地址。
func (r *Resolver) followRedirect(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) Chrome/126")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Request.URL.String(), nil
}

// DownloadHeaders 下载直链所需的防盗链头（safeDownload 之外的扩展点——
// B站 CDN 校验 Referer，缺失返回 403）。
func DownloadHeaders(rawURL string) map[string]string {
	if strings.Contains(rawURL, "bilivideo.com") || strings.Contains(rawURL, "hdslb.com") {
		return map[string]string{"Referer": "https://www.bilibili.com"}
	}
	return nil
}

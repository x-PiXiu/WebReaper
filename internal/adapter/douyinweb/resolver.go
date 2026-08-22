// douyinweb 分享链解析（port.VideoLinkResolver 实现——08 计划 D4 提取管线入口）。
//
// 流程：分享短链（v.douyin.com/xxx）HTTP 跟随重定向 → /video/{aweme_id}
//       → 复用搜索器的视频页 RPA 上下文调详情接口 → 播放直链（play_addr）。
// 抖音 play_addr 域名是防盗链的内部域名——replaceDomain 换公开 CDN 域后可下载。
package douyinweb

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"webreaper/internal/usecase/port"
)

// LinkResolver port.VideoLinkResolver 的 douyin 实现（复用 Searcher 的账号/RPA）。
type LinkResolver struct {
	searcher *Searcher
	client   *http.Client
}

// NewLinkResolver 创建分享链解析器。
func NewLinkResolver(s *Searcher) *LinkResolver {
	return &LinkResolver{searcher: s, client: &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向过多")
			}
			return nil
		},
	}}
}

func (r *LinkResolver) SupportedPlatforms() []string { return []string{platform} }

var awemeIDRe = regexp.MustCompile(`/video/(\d+)`)

// IsDouyinLink 是否抖音链接（分享短链/网页链）。
func IsDouyinLink(rawURL string) bool {
	return strings.Contains(rawURL, "v.douyin.com") ||
		strings.Contains(rawURL, "www.douyin.com/video/") ||
		strings.Contains(rawURL, "iesdouyin.com")
}

// Resolve 分享链/网页链 → 播放直链。
func (r *LinkResolver) Resolve(ctx context.Context, tenantID, rawURL string) (string, string, string, error) {
	if !IsDouyinLink(rawURL) {
		return "", "", "", fmt.Errorf("暂不支持该链接（当前支持抖音分享链；其他平台可下载后直接上传视频）")
	}
	// ① 短链 → 最终网页地址 → aweme_id
	videoID := ""
	if m := awemeIDRe.FindStringSubmatch(rawURL); m != nil {
		videoID = m[1]
	} else {
		finalURL, err := r.followRedirect(ctx, rawURL)
		if err != nil {
			return "", "", "", fmt.Errorf("分享链解析失败: %w", err)
		}
		m := awemeIDRe.FindStringSubmatch(finalURL)
		if m == nil {
			return "", "", "", fmt.Errorf("链接里找不到视频（可能已删除或非视频内容）")
		}
		videoID = m[1]
	}
	// ② 视频页上下文调详情接口拿播放直链（复用搜索器的账号/RPA 基建）
	info, err := r.searcher.getAwemeDetail(ctx, tenantID, videoID)
	if err != nil {
		return "", "", "", err
	}
	if len(info.Video.PlayAddr.URLList) == 0 {
		return "", "", "", fmt.Errorf("详情无播放地址（可能视频已删除）")
	}
	playURL := replacePlayDomain(info.Video.PlayAddr.URLList[0])
	return playURL, info.Desc, platform, nil
}

// followRedirect 分享短链 302 → 最终网页地址（不下载内容）。
func (r *LinkResolver) followRedirect(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := r.client.Do(req)
	if err != nil {
		// 部分 CDN 不支持 HEAD——降级 GET 不读体
		req.Method = http.MethodGet
		resp, err = r.client.Do(req)
		if err != nil {
			return "", err
		}
	}
	defer resp.Body.Close()
	return resp.Request.URL.String(), nil
}

// replacePlayDomain 抖音 play_addr 的防盗链域名 → 公开 CDN 域（可直接 GET 下载）。
// 协议知识：aweme.snssdk.com/aweme/v1/play/ 系列替换为 www.douyin.com/aweme/v1/play/
func replacePlayDomain(u string) string {
	return strings.Replace(u, "aweme.snssdk.com", "www.douyin.com", 1)
}

var _ port.VideoLinkResolver = (*LinkResolver)(nil)

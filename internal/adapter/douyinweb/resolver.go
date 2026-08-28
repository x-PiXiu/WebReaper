// douyinweb 分享链解析（port.VideoLinkResolver 实现——08 计划 D4 提取管线入口）。
//
// 流程：分享短链（v.douyin.com/xxx）HTTP 跟随重定向 → /video/{aweme_id}
//       → 复用搜索器的视频页 RPA 上下文调详情接口 → 播放直链候选列表（play_addr.url_list）。
// url_list 全量返回由下载层依次尝试——CDN 节点可能个别 403（2026-08 实测 v26-web 节点）。
package douyinweb

import (
	"context"
	"fmt"
	"log"
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

// Resolve 分享链/网页链 → 候选播放直链列表。
//
// 双通道（快慢路径）：
//  ① Python sidecar（快路径）：纯 HTTP 走 iesdouyin 分享页 SSR——免浏览器
//     免账号，1~2s；依赖 Python OpenSSL TLS 指纹过 WAF（Go 指纹只拿壳页）
//  ② chromedp 详情接口（降级）：① 失败（python 缺失/风控/壳页）时兜底，
//     复用搜索器的账号/RPA 基建（含空响应换会话重试）
func (r *LinkResolver) Resolve(ctx context.Context, tenantID, rawURL string) ([]string, string, string, string, error) {
	if !IsDouyinLink(rawURL) {
		return nil, "", "", "", fmt.Errorf("暂不支持该链接——当前支持抖音 / B站分享链，及 yt-dlp 覆盖的 YouTube/微博/西瓜等大部分平台（长尾平台失败多为网络不可达或需登录）；也可下载视频后直接上传")
	}
	// ① Python sidecar 快路径
	if res, err := resolveViaPython(ctx, rawURL); err == nil {
		log.Printf("[douyinweb] Python sidecar 快路径成功：title=%.40s 候选=%d", res.Title, len(res.URLs))
		return res.URLs, res.Title, platform, "", nil
	} else {
		log.Printf("[douyinweb] Python sidecar 失败（%v），降级 chromedp 通道", err)
	}
	// ② chromedp：短链 → 最终网页地址 → aweme_id
	videoID := ""
	if m := awemeIDRe.FindStringSubmatch(rawURL); m != nil {
		videoID = m[1]
	} else {
		finalURL, err := r.followRedirect(ctx, rawURL)
		if err != nil {
			return nil, "", "", "", fmt.Errorf("分享链解析失败: %w", err)
		}
		m := awemeIDRe.FindStringSubmatch(finalURL)
		if m == nil {
			return nil, "", "", "", fmt.Errorf("链接里找不到视频（可能已删除或非视频内容）")
		}
		videoID = m[1]
	}
	// 视频页上下文调详情接口拿播放直链（复用搜索器的账号/RPA 基建）
	info, err := r.searcher.getAwemeDetail(ctx, tenantID, videoID)
	if err != nil {
		return nil, "", "", "", err
	}
	if len(info.Video.PlayAddr.URLList) == 0 {
		return nil, "", "", "", fmt.Errorf("详情无播放地址（可能视频已删除）")
	}
	// url_list 全量返回（2026-08 实测通常 3 个：douyinvod 节点 ×2 +
	// www.douyin.com/aweme/v1/play 官方域）——CDN 节点可能个别 403，
	// 由下载层依次尝试。旧协议的 aweme.snssdk.com 防盗链域名已被抖音下线，
	// url_list 本身即为公开可下发的 CDN 直链，无需再替换域名。
	return info.Video.PlayAddr.URLList, info.Desc, platform, "", nil
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

var _ port.VideoLinkResolver = (*LinkResolver)(nil)

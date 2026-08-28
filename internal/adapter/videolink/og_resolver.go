// og_resolver.go 通用 og:video 解析器（多平台分享页 fallback）。
//
// 大量视频平台的分享/网页页在 HTML head 里带 og:video / og:video:url /
// og:video:secure_url meta 标签（微博/西瓜/梨视频/皮皮虾等）——无需平台专属
// API，跟随重定向拉 HTML 解析即可拿到直链。挂在组合链末位兜底：
// 平台专属 resolver（抖音账号基建/B站公开 API）优先，未命中的链接尝试 og。
package videolink

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// OGResolver 通用 og:video meta 解析器。
type OGResolver struct {
	client *http.Client
}

var _ interface {
	SupportedPlatforms() []string
	Resolve(ctx context.Context, tenantID, rawURL string) ([]string, string, string, string, error)
} = (*OGResolver)(nil)

// NewOGResolver 创建（超时 15s——拉分享页 HTML）。
func NewOGResolver() *OGResolver {
	return &OGResolver{client: &http.Client{Timeout: 15 * time.Second}}
}

// SupportedPlatforms 通用兜底（不列具体平台——组合链最后挂载）。
func (r *OGResolver) SupportedPlatforms() []string { return []string{"og-generic"} }

var (
	ogVideoRe   = regexp.MustCompile(`<meta[^>]+property=["']og:video(?::secure_url|:url)?["'][^>]+content=["']([^"']+)["']`)
	ogVideoRe2  = regexp.MustCompile(`<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:video(?::secure_url|:url)?["']`)
	ogTitleRe   = regexp.MustCompile(`<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	ogTitleRe2  = regexp.MustCompile(`<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:title["']`)
)

// Resolve 分享页 HTML → og:video 直链（单候选）。
func (r *OGResolver) Resolve(ctx context.Context, tenantID, rawURL string) ([]string, string, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("分享页打开失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", "", fmt.Errorf("分享页 HTTP %d", resp.StatusCode)
	}
	// 读 head 部分（og meta 在 head；限制 256KB 防超大页面）。
	// 必须读满而非单次 Read——TCP 分包下单次 Read 返回不全，og meta 靠后的
	// 大页面会被截断漏解析（曾表现为"明明有 og:video 却报未声明"）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil && len(body) == 0 {
		return nil, "", "", "", fmt.Errorf("分享页读取失败: %w", err)
	}
	html := string(body)

	videoURL := firstMatch(html, ogVideoRe, ogVideoRe2)
	if videoURL == "" {
		return nil, "", "", "", fmt.Errorf("分享页未声明 og:video（可能需要平台专属解析或 JS 渲染）")
	}
	// HTML 实体还原（&amp; → &）
	videoURL = strings.ReplaceAll(videoURL, "&amp;", "&")
	title := firstMatch(html, ogTitleRe, ogTitleRe2)
	return []string{videoURL}, title, "og-generic", "", nil
}

func firstMatch(s string, patterns ...*regexp.Regexp) string {
	for _, p := range patterns {
		if m := p.FindStringSubmatch(s); len(m) > 1 && m[1] != "" {
			return m[1]
		}
	}
	return ""
}

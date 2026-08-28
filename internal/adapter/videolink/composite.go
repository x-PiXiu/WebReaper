// Package videolink 多平台视频链接解析组合器（文案提取链路入口）。
//
// 按链接特征分发到平台 resolver：B站（b23.tv/bilibili.com/BV 号）→ biliweb；
// 其余（抖音系）→ douyinweb（不支持的链接由其报错并引导直接上传）。
// 新增平台 = 新 resolver + 此处挂载，用例层零改动。
package videolink

import (
	"context"
	"fmt"
	"strings"

	"webreaper/internal/adapter/biliweb"
	"webreaper/internal/usecase/port"
)

// CompositeResolver 多平台组合解析器。
type CompositeResolver struct {
	douyin port.VideoLinkResolver
	bili   port.VideoLinkResolver
	ytdlp  *YtDlpResolver // yt-dlp 通用兜底（YouTube/微博/西瓜等 1800+ 站点）
	og     *OGResolver    // 通用 og:video 兜底（微博/西瓜等——分享页 meta 解析）
}

var _ port.VideoLinkResolver = (*CompositeResolver)(nil)

// NewComposite 创建组合解析器（douyin/bili/ytdlp 任一参数可为 nil——nil 平台不可用；
// og 兜底内置——平台专属未命中时尝试分享页 og:video）。
func NewComposite(douyin, bili port.VideoLinkResolver, ytdlp *YtDlpResolver) *CompositeResolver {
	return &CompositeResolver{douyin: douyin, bili: bili, ytdlp: ytdlp, og: NewOGResolver()}
}

// SupportedPlatforms 支持的平台合集。
func (c *CompositeResolver) SupportedPlatforms() []string {
	out := []string{}
	if c.douyin != nil {
		out = append(out, c.douyin.SupportedPlatforms()...)
	}
	if c.bili != nil {
		out = append(out, c.bili.SupportedPlatforms()...)
	}
	return out
}

// isKuaishouLink 快手链接识别（v.kuaishou.com 短链 / kuaishou.com 长链）。
// 快手分享页为 JS 渲染，og:video 不可用——当前不支持自动提取。
func isKuaishouLink(rawURL string) bool {
	return strings.Contains(rawURL, "v.kuaishou.com") ||
		strings.Contains(rawURL, "kuaishou.com/short-video") ||
		strings.Contains(rawURL, "kuaishou.com/f/")
}

// Resolve 按链接特征分发（透传候选直链列表）。
//
// 分发顺序：B站 → 快手（明确拒绝）→ 抖音（sidecar+chromedp）→ yt-dlp 通用
// → og:video 兜底。平台专属优先（协议自研、快），yt-dlp 管长尾，og 收尾。
func (c *CompositeResolver) Resolve(ctx context.Context, tenantID, rawURL string) ([]string, string, string, string, error) {
	if biliweb.IsBilibiliLink(rawURL) {
		if c.bili != nil {
			return c.bili.Resolve(ctx, tenantID, rawURL)
		}
		return nil, "", "", "", nil
	}
	// 快手链接：JS 渲染页面，og:video 不可用——提前返回明确提示
	if isKuaishouLink(rawURL) {
		return nil, "", "", "", fmt.Errorf("暂不支持快手分享链自动提取——请下载视频后直接上传，我们正在接入快手解析")
	}
	if c.douyin != nil {
		u, t, p, lp, e := c.douyin.Resolve(ctx, tenantID, rawURL)
		if e == nil {
			return u, t, p, lp, nil
		}
		// 抖音专属失败 → yt-dlp 通用 → og 依次兜底；全失败返回抖音原始错误（提示最具体）
		if c.ytdlp != nil {
			if u2, t2, p2, lp2, e2 := c.ytdlp.Resolve(ctx, tenantID, rawURL); e2 == nil {
				return u2, t2, p2, lp2, nil
			}
		}
		if c.og != nil {
			if u3, t3, p3, lp3, e3 := c.og.Resolve(ctx, tenantID, rawURL); e3 == nil {
				return u3, t3, p3, lp3, nil
			}
		}
		return nil, "", "", "", e
	}
	if c.ytdlp != nil {
		if u, t, p, lp, e := c.ytdlp.Resolve(ctx, tenantID, rawURL); e == nil {
			return u, t, p, lp, nil
		}
	}
	if c.og != nil {
		return c.og.Resolve(ctx, tenantID, rawURL)
	}
	return nil, "", "", "", nil
}

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
	og     *OGResolver // 通用 og:video 兜底（微博/西瓜等——分享页 meta 解析）
}

var _ port.VideoLinkResolver = (*CompositeResolver)(nil)

// NewComposite 创建组合解析器（任一参数可为 nil——nil 平台不可用；
// og 兜底内置——平台专属未命中时尝试分享页 og:video）。
func NewComposite(douyin, bili port.VideoLinkResolver) *CompositeResolver {
	return &CompositeResolver{douyin: douyin, bili: bili, og: NewOGResolver()}
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

// Resolve 按链接特征分发。
func (c *CompositeResolver) Resolve(ctx context.Context, tenantID, rawURL string) (string, string, string, string, error) {
	if biliweb.IsBilibiliLink(rawURL) {
		if c.bili != nil {
			return c.bili.Resolve(ctx, tenantID, rawURL)
		}
		return "", "", "", "", nil
	}
	// 快手链接：JS 渲染页面，og:video 不可用——提前返回明确提示
	if isKuaishouLink(rawURL) {
		return "", "", "", "", fmt.Errorf("暂不支持快手分享链自动提取——请下载视频后直接上传，我们正在接入快手解析")
	}
	if c.douyin != nil {
		if u, t, p, lp, e := c.douyin.Resolve(ctx, tenantID, rawURL); e == nil {
			return u, t, p, lp, nil
		} else if c.og != nil {
			// 抖音专属失败（如账号未绑）→ og 兜底再试
			if u, t, p, lp, e2 := c.og.Resolve(ctx, tenantID, rawURL); e2 == nil {
				return u, t, p, lp, nil
			}
			return "", "", "", "", e // 返回原始错误（提示更具体）
		} else {
			return "", "", "", "", e
		}
	}
	if c.og != nil {
		return c.og.Resolve(ctx, tenantID, rawURL)
	}
	return "", "", "", "", nil
}

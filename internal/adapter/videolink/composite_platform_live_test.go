package videolink

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"webreaper/internal/adapter/biliweb"
	"webreaper/internal/adapter/douyinweb"
)

// TestCompositeAllPlatformsLive 现有拉取下载逻辑全平台实测（composite 入口 → 解析 → 首候选下载探测）。
//
// 目的：盘点各通道当前真实有效性（无效通道考虑下线，防维护负担）。
// CI 无网/依赖缺失自动跳过。
func TestCompositeAllPlatformsLive(t *testing.T) {
	// 与 main.go 装配同构：抖音（sidecar 快路径；searcher 无 repo 时 chromedp 段不可用，
	// sidecar 失败即整体失败——如实反映"无账号场景"的抖音可用性）
	resolver := NewComposite(
		douyinweb.NewLinkResolver(douyinweb.NewSearcher(nil, nil)),
		biliweb.NewResolver(),
		NewYtDlpResolver(""),
	)

	cases := []struct {
		name string
		url  string
	}{
		{"抖音分享口令", "9.28 A@G.vs WMj:/ 09/28 :5pm 【清稚竹马】我还想说，我想你了！# ai漫剧 # 原创动画 # 漫剧 # 校园  https://v.douyin.com/m5RF3M1l8Bw/ 复制此链接，打开Dou音搜索，直接观看视频！"},
		{"B站视频页", "https://www.bilibili.com/video/BV1GJ411x7h7"},
		{"yt-dlp generic 直链", "https://www.w3schools.com/html/mov_bbb.mp4"},
		{"og:video 兜底（B站页 og meta）", "https://www.bilibili.com/video/BV1GJ411x7h7?t=og-probe"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			urls, title, plat, _, err := resolver.Resolve(ctx, "t", tc.url)
			if err != nil {
				t.Errorf("❌ %s 解析失败: %v", tc.name, err)
				return
			}
			fmt.Printf("✅ %-24s 解析成功 platform=%-14s title=%.30s 候选=%d\n", tc.name, plat, title, len(urls))

			// 首候选下载探测（模拟 safeDownload：UA + 按域 Referer）
			if len(urls) == 0 {
				t.Errorf("❌ %s 无候选地址", tc.name)
				return
			}
			dl, _ := http.NewRequest(http.MethodGet, urls[0], nil)
			dl.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36")
			if h := refererForHost(urls[0]); h != "" {
				dl.Header.Set("Referer", h)
			}
			resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(dl)
			if err != nil {
				t.Errorf("❌ %s 下载探测失败: %v", tc.name, err)
				return
			}
			resp.Body.Close()
			fmt.Printf("   └ 下载探测: HTTP %s type=%s len=%s\n", resp.Status, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"))
			if resp.StatusCode != http.StatusOK {
				t.Errorf("❌ %s 下载非 200: %s", tc.name, resp.Status)
			}
		})
	}
}

// refererForHost 复刻 videotranscript.downloadOne 的按域 Referer 规则。
func refererForHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Host
	switch {
	case contains(host, "bilivideo.com"), contains(host, "hdslb.com"):
		return "https://www.bilibili.com"
	case contains(host, "douyinvod.com"), contains(host, "douyin.com"),
		contains(host, "snssdk.com"), contains(host, "iesdouyin.com"):
		return "https://www.douyin.com/"
	}
	return ""
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

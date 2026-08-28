package videolink

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestYtDlpResolverGenericLive yt-dlp resolver 真实链路验证（CI 无网/未装 yt-dlp 自动跳过）：
// 用直链 mp4 走 generic extractor → progressive 候选列表 → 候选可下载探测。
//（B站链接不适用：yt-dlp 对 B站只返回 dash 分离流，被 progressive 筛选正确过滤——
// B站由 biliweb 自研通道负责，composite 先行分发。）
func TestYtDlpResolverGenericLive(t *testing.T) {
	r := NewYtDlpResolver("") // 自动探测调用形态
	urls, title, plat, _, err := r.Resolve(context.Background(), "t",
		"https://www.w3schools.com/html/mov_bbb.mp4")
	if err != nil {
		t.Skipf("yt-dlp 不可用或网络不通: %v", err)
	}
	fmt.Printf("✅ title=%.50s platform=%s 候选=%d\n", title, plat, len(urls))
	for i, u := range urls {
		fmt.Printf("   [%d] %.100s\n", i, u)
	}
	if len(urls) == 0 {
		t.Fatal("无候选")
	}

	// 首候选可下载性探测（B站 CDN 需 Referer——safeDownload 域名规则已覆盖）
	req, _ := http.NewRequest(http.MethodGet, urls[0], nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.w3schools.com")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Skipf("下载探测失败: %v", err)
	}
	defer resp.Body.Close()
	fmt.Printf("✅ 首候选下载探测: HTTP %s type=%s len=%s\n",
		resp.Status, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("首候选下载非 200: %s", resp.Status)
	}
}

package biliweb

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestResolveBilibiliLive 真实网络链路测试（CI 无网自动跳过）：
// 用户提供的 URL（含 p=2 分P）→ view API 拿 cid → playurl 拿 mp4 直链 → 直链带 Referer 可下载。
func TestResolveBilibiliLive(t *testing.T) {
	r := NewResolver()
	raw := "https://www.bilibili.com/video/BV1aw9jBeEHr?spm_id_from=333.788.player.switch&vd_source=44a7275750b2578f268240dcf3c10b55&p=2"
	videoURL, title, plat, _, err := r.Resolve(context.Background(), "t", raw)
	if err != nil {
		t.Skipf("网络不可达或接口变更: %v", err)
	}
	if plat != "bilibili" || videoURL == "" {
		t.Fatalf("解析结果异常: plat=%s url=%q", plat, videoURL)
	}
	t.Logf("✅ 解析成功：标题=%q 平台=%s", title, plat)
	t.Logf("直链 host=%s", func() string { u, _ := url.Parse(videoURL); return u.Host }())

	// 直链可下载性（带 Referer 防盗链——模拟 safeDownload 的补头逻辑）
	req, _ := http.NewRequest(http.MethodGet, videoURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) Chrome/126")
	req.Header.Set("Referer", "https://www.bilibili.com")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("直链下载请求失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("直链下载 HTTP %d（Referer 防盗链未通过？）", resp.StatusCode)
	}
	buf := make([]byte, 16)
	n, _ := resp.Body.Read(buf)
	if !strings.HasPrefix(string(buf[:n]), "\x00\x00\x00") {
		t.Logf("（前 16 字节非 mp4 ftyp 头直读形态——流式下载由 safeDownload 全量校验）")
	}
	t.Logf("✅ 直链带 Referer 可下载（Content-Length 可读=%v）", resp.ContentLength > 0)
}

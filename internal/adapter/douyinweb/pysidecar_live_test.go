package douyinweb

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestPySidecarLive Python sidecar 真实链路验证（需 python3+requests；CI 无网跳过）。
func TestPySidecarLive(t *testing.T) {
	res, err := resolveViaPython(context.Background(),
		"9.28 A@G.vs WMj:/ 09/28 :5pm 【清稚竹马】我还想说，我想你了！# ai漫剧 # 原创动画 # 漫剧 # 校园  https://v.douyin.com/m5RF3M1l8Bw/ 复制此链接，打开Dou音搜索，直接观看视频！")
	if err != nil {
		t.Skipf("sidecar 不可用（可能缺 python/requests 或风控）: %v", err)
	}
	fmt.Printf("✅ title=%s author=%s duration=%ds 候选=%d\n", res.Title, res.Author, res.Duration, len(res.URLs))
	for i, u := range res.URLs {
		fmt.Printf("   [%d] %s\n", i, u[:min(100, len(u))])
	}
	if len(res.URLs) == 0 {
		t.Fatal("无候选地址")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestPySidecarDownloadGo Go 下载 sidecar 返回的直链（验证 CDN 层不按 TLS 分流）。
func TestPySidecarDownloadGo(t *testing.T) {
	res, err := resolveViaPython(context.Background(), "https://v.douyin.com/m5RF3M1l8Bw/")
	if err != nil {
		t.Skipf("sidecar 不可用: %v", err)
	}
	for i, u := range res.URLs {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1")
		req.Header.Set("Referer", "https://www.douyin.com/")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			t.Errorf("候选[%d] 下载失败: %v", i, err)
			continue
		}
		head := make([]byte, 16)
		n, _ := resp.Body.Read(head)
		resp.Body.Close()
		fmt.Printf("候选[%d] HTTP %s type=%s len=%s 头=%x\n", i, resp.Status,
			resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"), head[:n])
	}
}

package videolink

import (
	"context"
	"testing"
)

// TestOGResolverBilibiliLive 用B站视频页验证 og 解析器（B站页有 og:video——
// 真实网络测试；CI 无网自动跳过）。
func TestOGResolverBilibiliLive(t *testing.T) {
	r := NewOGResolver()
	// 用一个公开短视频页（B站视频页带 og:video meta）
	urls, title, plat, _, err := r.Resolve(context.Background(), "t", "https://www.bilibili.com/video/BV1GJ411x7h7")
	if err != nil {
		t.Skipf("网络不可达或页面无 og:video: %v", err)
	}
	u := ""
	if len(urls) > 0 {
		u = urls[0]
	}
	t.Logf("✅ og 解析成功：platform=%s title=%.40s url=%.80s", plat, title, u)
	if u == "" {
		t.Fatal("og:video 为空")
	}
}

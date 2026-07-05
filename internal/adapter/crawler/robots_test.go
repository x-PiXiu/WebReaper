package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRobotsChecker_NoRuleDomainDoesNotPanic 验证修复：
// robots.txt 不存在（404）的域名，多次访问不应 panic（原 bug 是负向缓存 nil 后解引用）。
func TestRobotsChecker_NoRuleDomainDoesNotPanic(t *testing.T) {
	// 模拟一个不提供 robots.txt 的站点（返回 404）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	checker := NewRobotsChecker()
	ctx := context.Background()

	// 第一次访问：拉取 robots.txt 失败（404），应返回允许且不 panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("第一次访问 panic（不应发生）: %v", r)
		}
	}()
	allowed := checker.IsAllowed(ctx, ts.URL+"/some/path")
	if !allowed {
		t.Errorf("无 robots.txt 时应默认允许，得到 false")
	}

	// 第二次访问同一域名：命中负向缓存，原 bug 在此 panic（取出 nil 解引用 fetched）
	// 修复后应缓存空规则对象，正常返回 true
	for i := 0; i < 3; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("第 %d 次访问 panic（负向缓存 nil 解引用，bug 未修复）: %v", i+2, r)
				}
			}()
			if a := checker.IsAllowed(ctx, ts.URL+"/some/path"); !a {
				t.Errorf("缓存命中时应返回允许，得到 false")
			}
		}()
	}
}

// TestRobotsChecker_DisallowRuleRespected 验证正常规则解析：
// 有 Disallow /private 的 robots.txt，访问 /private/x 应被禁止。
func TestRobotsChecker_DisallowRuleRespected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("User-agent: *\nDisallow: /private\n"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	checker := NewRobotsChecker()
	ctx := context.Background()

	if checker.IsAllowed(ctx, ts.URL+"/private/secret") {
		t.Errorf("/private/secret 应被 robots.txt 禁止")
	}
	if !checker.IsAllowed(ctx, ts.URL+"/public") {
		t.Errorf("/public 应被允许")
	}
}

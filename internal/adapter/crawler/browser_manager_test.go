package crawler

import (
	"fmt"
	"testing"
	"time"
)

func TestParseCookieString(t *testing.T) {
	tests := []struct {
		name      string
		cookieStr string
		domain    string
		wantCount int
		wantFirst string
	}{
		{
			name:      "正常Cookie",
			cookieStr: "session_id=abc123; user_token=xyz789",
			domain:    ".douyin.com",
			wantCount: 2,
			wantFirst: "session_id",
		},
		{
			name:      "单个Cookie",
			cookieStr: "token=abc",
			domain:    ".example.com",
			wantCount: 1,
			wantFirst: "token",
		},
		{
			name:      "空字符串",
			cookieStr: "",
			domain:    ".example.com",
			wantCount: 0,
		},
		{
			name:      "带空格的Cookie",
			cookieStr: "  a=1 ; b=2  ",
			domain:    ".example.com",
			wantCount: 2,
			wantFirst: "a",
		},
		{
			name:      "Cookie值含等号",
			cookieStr: "data=key=value",
			domain:    ".example.com",
			wantCount: 1,
			wantFirst: "data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cookies := parseCookieString(tt.cookieStr, tt.domain)
			if len(cookies) != tt.wantCount {
				t.Errorf("parseCookieString() returned %d cookies, want %d", len(cookies), tt.wantCount)
				return
			}
			if tt.wantCount > 0 && cookies[0].Name != tt.wantFirst {
				t.Errorf("first cookie name = %v, want %v", cookies[0].Name, tt.wantFirst)
			}
			if tt.wantCount > 0 && cookies[0].Domain != tt.domain {
				t.Errorf("cookie domain = %v, want %v", cookies[0].Domain, tt.domain)
			}
		})
	}
}

func TestNewCDPBrowserManager_Defaults(t *testing.T) {
	m := NewCDPBrowserManager(nil)
	if m.debugPort != 9222 {
		t.Errorf("debugPort = %v, want 9222", m.debugPort)
	}
	if m.mode != "auto" {
		t.Errorf("mode = %v, want auto", m.mode)
	}
}

func TestNewCDPBrowserManager_CustomConfig(t *testing.T) {
	cfg := &BrowserConfig{DebugPort: 9333, Mode: "cdp"}
	m := NewCDPBrowserManager(cfg)
	if m.debugPort != 9333 {
		t.Errorf("debugPort = %v, want 9333", m.debugPort)
	}
	if m.mode != "cdp" {
		t.Errorf("mode = %v, want cdp", m.mode)
	}
}

func TestCDPBrowserManager_IsConnected(t *testing.T) {
	m := NewCDPBrowserManager(nil)
	if m.IsConnected("douyin") {
		t.Error("should not be connected initially")
	}
}

func TestProxyRefreshMixin_NilPool(t *testing.T) {
	m := NewProxyRefreshMixin(nil)
	if err := m.RefreshIfNeeded(); err != nil {
		t.Errorf("RefreshIfNeeded() with nil pool should not error, got %v", err)
	}
	if m.GetProxy() != "" {
		t.Errorf("GetProxy() = %v, want empty", m.GetProxy())
	}
}

func TestProxyRefreshMixin_SetProxy(t *testing.T) {
	m := NewProxyRefreshMixin(nil)
	m.SetProxy("http://proxy:8080", 1*time.Hour)

	if m.GetProxy() != "http://proxy:8080" {
		t.Errorf("GetProxy() = %v, want http://proxy:8080", m.GetProxy())
	}
}

// mockProxyProvider 是 ProxyProvider 的 mock 实现。
type mockProxyProvider struct {
	proxy    string
	duration time.Duration
	err      error
}

func (m *mockProxyProvider) GetProxy() (string, time.Duration, error) {
	return m.proxy, m.duration, m.err
}

func TestProxyRefreshMixin_WithProvider(t *testing.T) {
	pool := &mockProxyProvider{
		proxy:    "http://new-proxy:9090",
		duration: 30 * time.Minute,
	}
	m := NewProxyRefreshMixin(pool)

	// 第一次刷新
	if err := m.RefreshIfNeeded(); err != nil {
		t.Fatalf("RefreshIfNeeded() error = %v", err)
	}
	if m.GetProxy() != "http://new-proxy:9090" {
		t.Errorf("GetProxy() = %v, want http://new-proxy:9090", m.GetProxy())
	}

	// 未过期，不刷新
	pool.proxy = "http://another-proxy:1234"
	if err := m.RefreshIfNeeded(); err != nil {
		t.Fatalf("RefreshIfNeeded() error = %v", err)
	}
	if m.GetProxy() != "http://new-proxy:9090" {
		t.Errorf("GetProxy() = %v, want http://new-proxy:9090 (should not refresh)", m.GetProxy())
	}
}

func TestProxyRefreshMixin_ProviderError(t *testing.T) {
	pool := &mockProxyProvider{
		err: fmt.Errorf("代理池不可用"),
	}
	m := NewProxyRefreshMixin(pool)

	err := m.RefreshIfNeeded()
	if err == nil {
		t.Error("RefreshIfNeeded() should return error when provider fails")
	}
}

func TestFindChromePath(t *testing.T) {
	// 这个测试在非 Windows 环境下可能返回空
	path := findChromePath()
	t.Logf("findChromePath() = %v", path)
	// 不断言具体值，因为路径因环境而异
}

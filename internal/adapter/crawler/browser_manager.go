// Package crawler 提供爬虫平台的浏览器管理。
//
// CDPBrowserManager 参考 MediaCrawler 的 tools/cdp_browser.py 设计：
//   - 三级降级：用户 Chrome → 用户 Edge → 内置 Chromium
//   - 浏览器进程生命周期管理
//   - Cookie 注入（从加密存储解密后注入浏览器）
package crawler

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// CDPBrowserManager 浏览器进程生命周期管理器。
type CDPBrowserManager struct {
	debugPort int
	mode      string

	mu       sync.Mutex
	contexts map[string]context.Context
	running  bool
}

// BrowserConfig 浏览器管理器配置。
type BrowserConfig struct {
	DebugPort int
	Mode      string // "auto" / "cdp" / "builtin"
}

// NewCDPBrowserManager 创建浏览器管理器。
func NewCDPBrowserManager(cfg *BrowserConfig) *CDPBrowserManager {
	port := 9222
	mode := "auto"
	if cfg != nil {
		if cfg.DebugPort > 0 {
			port = cfg.DebugPort
		}
		if cfg.Mode != "" {
			mode = cfg.Mode
		}
	}
	return &CDPBrowserManager{
		debugPort: port,
		mode:      mode,
		contexts:  make(map[string]context.Context),
	}
}

// Connect 三级降级连接浏览器。
func (m *CDPBrowserManager) Connect(ctx context.Context, platform string) (context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if bCtx, ok := m.contexts[platform]; ok {
		return bCtx, nil
	}

	switch m.mode {
	case "cdp":
		return m.connectCDP(ctx, platform)
	case "builtin":
		return m.launchBuiltin(ctx, platform)
	default:
		if bCtx, err := m.connectCDP(ctx, platform); err == nil {
			log.Printf("[browser] 已连接用户 Chrome (platform=%s)", platform)
			return bCtx, nil
		}
		if bCtx, err := m.connectEdge(ctx, platform); err == nil {
			log.Printf("[browser] 已连接用户 Edge (platform=%s)", platform)
			return bCtx, nil
		}
		log.Printf("[browser] 降级到内置 Chromium (platform=%s)", platform)
		return m.launchBuiltin(ctx, platform)
	}
}

// connectCDP 连接已运行的 Chrome。
func (m *CDPBrowserManager) connectCDP(ctx context.Context, platform string) (context.Context, error) {
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", m.debugPort)
	allocatorCtx, _ := chromedp.NewRemoteAllocator(ctx, wsURL)

	browserCtx, _ := chromedp.NewContext(allocatorCtx)

	if err := chromedp.Run(browserCtx, chromedp.Navigate("about:blank")); err != nil {
		return nil, fmt.Errorf("Chrome 连接验证失败: %w", err)
	}

	m.contexts[platform] = browserCtx
	m.running = true
	return browserCtx, nil
}

// connectEdge 连接已运行的 Edge。
func (m *CDPBrowserManager) connectEdge(ctx context.Context, platform string) (context.Context, error) {
	for _, port := range []int{9222, 9223, 9224} {
		wsURL := fmt.Sprintf("ws://127.0.0.1:%d", port)
		allocatorCtx, _ := chromedp.NewRemoteAllocator(ctx, wsURL)
		browserCtx, _ := chromedp.NewContext(allocatorCtx)
		if err := chromedp.Run(browserCtx, chromedp.Navigate("about:blank")); err != nil {
			continue
		}
		m.contexts[platform] = browserCtx
		m.running = true
		return browserCtx, nil
	}
	return nil, fmt.Errorf("未找到运行中的 Edge 实例")
}

// launchBuiltin 启动内置 Chromium。
func (m *CDPBrowserManager) launchBuiltin(ctx context.Context, platform string) (context.Context, error) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("headless", "new"),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("mute-audio", true),
	}

	if runtime.GOOS == "windows" {
		if p := findChromePath(); p != "" {
			opts = append(opts, chromedp.ExecPath(p))
		}
	}

	allocatorCtx, _ := chromedp.NewExecAllocator(ctx, opts...)

	browserCtx, _ := chromedp.NewContext(allocatorCtx)

	// 注入反检测
	chromedp.Run(browserCtx, chromedp.Evaluate(`
		Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
		window.chrome = {runtime: {}};
	`, nil))

	m.contexts[platform] = browserCtx
	m.running = true
	return browserCtx, nil
}

// InjectCookies 向浏览器注入 Cookie。
func (m *CDPBrowserManager) InjectCookies(ctx context.Context, platform, cookieStr, domain string) error {
	m.mu.Lock()
	browserCtx, ok := m.contexts[platform]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("平台 %s 未连接浏览器", platform)
	}

	cookies := parseCookieString(cookieStr, domain)
	return chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return network.SetCookies(cookies).Do(ctx)
	}))
}

// Navigate 导航到指定 URL。
func (m *CDPBrowserManager) Navigate(ctx context.Context, platform, url string) error {
	m.mu.Lock()
	browserCtx, ok := m.contexts[platform]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("平台 %s 未连接浏览器", platform)
	}

	return chromedp.Run(browserCtx, chromedp.Navigate(url))
}

// Close 关闭指定平台的浏览器连接。
func (m *CDPBrowserManager) Close(platform string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx, ok := m.contexts[platform]; ok {
		chromedp.Cancel(ctx)
		delete(m.contexts, platform)
	}
}

// CloseAll 关闭所有浏览器连接。
func (m *CDPBrowserManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ctx := range m.contexts {
		chromedp.Cancel(ctx)
	}
	m.contexts = make(map[string]context.Context)
	m.running = false
}

// IsConnected 检查指定平台是否已连接。
func (m *CDPBrowserManager) IsConnected(platform string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.contexts[platform]
	return ok
}

// parseCookieString 将 Cookie 字符串解析为 chromedp Cookie 数组。
func parseCookieString(cookieStr, domain string) []*network.CookieParam {
	var cookies []*network.CookieParam
	for _, part := range strings.Split(cookieStr, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		cookies = append(cookies, &network.CookieParam{
			Name:   strings.TrimSpace(kv[0]),
			Value:  strings.TrimSpace(kv[1]),
			Domain: domain,
			Path:   "/",
		})
	}
	return cookies
}

// findChromePath 在 Windows 上查找 Chrome 安装路径。
func findChromePath() string {
	paths := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		os.Getenv("LOCALAPPDATA") + `\Google\Chrome\Application\chrome.exe`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if out, err := exec.Command("where", "chrome.exe").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 {
			return strings.TrimSpace(lines[0])
		}
	}
	return ""
}

// WaitForNavigation 等待页面导航完成。
func WaitForNavigation(ctx context.Context, timeout time.Duration) error {
	select {
	case <-time.After(timeout):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

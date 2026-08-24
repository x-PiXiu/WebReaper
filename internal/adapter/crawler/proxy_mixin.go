package crawler

import (
	"sync"
	"time"
)

// ProxyRefreshMixin 代理自动轮换组件（参考 MediaCrawler 的 ProxyRefreshMixin）。
//
// 设计（Go 组合模式替代 Python Mixin）：
//   - 每个 Client 组合此结构体
//   - 在每次请求前调用 RefreshIfNeeded() 检查代理是否过期
//   - 过期则从代理池获取新代理
type ProxyRefreshMixin struct {
	pool      ProxyProvider
	proxy     string
	expireAt  time.Time
	mu        sync.Mutex
}

// ProxyProvider 代理提供者接口。
type ProxyProvider interface {
	// GetProxy 获取一个可用的代理地址。
	GetProxy() (string, time.Duration, error)
}

// NewProxyRefreshMixin 创建代理轮换组件。
func NewProxyRefreshMixin(pool ProxyProvider) *ProxyRefreshMixin {
	return &ProxyRefreshMixin{
		pool: pool,
	}
}

// RefreshIfNeeded 检查代理是否过期，过期则刷新。
func (m *ProxyRefreshMixin) RefreshIfNeeded() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pool == nil {
		return nil // 未配置代理池，跳过
	}

	// 未过期，跳过
	if time.Now().Before(m.expireAt) {
		return nil
	}

	// 获取新代理
	proxy, duration, err := m.pool.GetProxy()
	if err != nil {
		return err
	}

	m.proxy = proxy
	m.expireAt = time.Now().Add(duration)
	return nil
}

// GetProxy 获取当前代理地址。
func (m *ProxyRefreshMixin) GetProxy() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.proxy
}

// SetProxy 手动设置代理地址（用于管理后台配置）。
func (m *ProxyRefreshMixin) SetProxy(proxy string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.proxy = proxy
	m.expireAt = time.Now().Add(duration)
}

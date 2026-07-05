package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 全局合规开关（受 CrawlPolicy 控制，由 main 在配置变更时同步）。
//   - respectRobotsEnabled：是否遵守 robots.txt（默认 true，合规）
//   - globalUserAgent：全局 User-Agent（默认可识别的 WebReaper/1.0）
var (
	respectRobotsEnabled = true
	robotsMu             sync.RWMutex
	globalUserAgent      = "WebReaper/1.0"
)

// SetRespectRobots 更新全局 robots 遵守开关。
func SetRespectRobots(enabled bool) {
	robotsMu.Lock()
	defer robotsMu.Unlock()
	respectRobotsEnabled = enabled
}

// SetGlobalUserAgent 更新全局 User-Agent。
func SetGlobalUserAgent(ua string) {
	if ua == "" {
		return
	}
	robotsMu.Lock()
	defer robotsMu.Unlock()
	globalUserAgent = ua
}

// UserAgent 返回当前全局 User-Agent（供各爬虫设置请求头）。
func UserAgent() string {
	robotsMu.RLock()
	defer robotsMu.RUnlock()
	return globalUserAgent
}

// IsRespectRobots 返回当前是否遵守 robots.txt。
func IsRespectRobots() bool {
	robotsMu.RLock()
	defer robotsMu.RUnlock()
	return respectRobotsEnabled
}

// RobotsChecker 检查 robots.txt 规则，决定爬虫是否被允许访问某 URL。
//
// 合规基础设施：所有爬虫在访问目标前都应检查 robots.txt。
// 规则：读 https://domain/robots.txt，解析 Disallow 路径。
// 缓存：同域名的 robots.txt 缓存 1 小时，避免重复请求。
//
// 实现要点（健壮性）：
//   - 缓存值统一为 *robotsRule，绝不存 nil。robots.txt 不存在/拉取失败时
//     缓存一个空规则对象（emptyRule）作为「已查询无规则」的哨兵，
//     避免负向缓存导致后续 nil 解引用 panic。
//   - getRule 用双检锁（double-checked locking）避免同域名并发重复拉取。
type RobotsChecker struct {
	mu     sync.RWMutex
	cache  map[string]*robotsRule // key = domain；值恒非 nil（无规则时存 emptyRule）
	locks  map[string]*sync.Mutex // 每域名一把锁，避免并发拉取同一 robots.txt
	client *http.Client
}

// robotsRule 是解析后的 robots.txt 规则。
type robotsRule struct {
	disallow []string // Disallow 路径前缀
	allow    []string // Allow 路径前缀（优先级高于 Disallow）
	fetched  time.Time
}

// NewRobotsChecker 创建 robots.txt 检查器。
func NewRobotsChecker() *RobotsChecker {
	return &RobotsChecker{
		cache:  make(map[string]*robotsRule),
		locks:  make(map[string]*sync.Mutex),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// IsAllowed 检查指定 URL 是否被允许爬取。
// 返回 true 表示允许，false 表示 robots.txt 禁止。
func (c *RobotsChecker) IsAllowed(ctx context.Context, targetURL string) bool {
	u, err := url.Parse(targetURL)
	if err != nil {
		return true // URL 解析失败，默认允许（不阻塞业务）
	}
	domain := u.Host
	path := u.Path
	if path == "" {
		path = "/"
	}

	rule := c.getRule(ctx, domain, u.Scheme)
	if rule == nil {
		return true // 无规则（robots.txt 不存在或获取失败），默认允许
	}

	// 检查 Allow（优先级更高）
	for _, a := range rule.allow {
		if strings.HasPrefix(path, a) {
			return true
		}
	}
	// 检查 Disallow
	for _, d := range rule.disallow {
		if d == "" {
			continue // "Disallow:" 空值表示允许全部
		}
		if strings.HasPrefix(path, d) {
			return false
		}
	}
	return true
}

// getRule 获取域名对应的 robots 规则（带缓存）。
//
// 返回值恒非 nil：robots.txt 不存在或拉取失败时返回空规则对象
// （disallow/allow 均为空，IsAllowed 会判定为允许）。
// 用双检锁避免同域名并发重复拉取。
func (c *RobotsChecker) getRule(ctx context.Context, domain, scheme string) *robotsRule {
	// 1. 快路径：读缓存（1 小时有效）
	c.mu.RLock()
	if rule, ok := c.cache[domain]; ok && rule != nil && time.Since(rule.fetched) < time.Hour {
		c.mu.RUnlock()
		return rule
	}
	c.mu.RUnlock()

	// 2. 取（或创建）该域名的拉取锁，串行化同域名的并发拉取
	c.mu.Lock()
	domainLock, ok := c.locks[domain]
	if !ok {
		domainLock = &sync.Mutex{}
		c.locks[domain] = domainLock
	}
	c.mu.Unlock()

	domainLock.Lock()
	defer domainLock.Unlock()

	// 3. 双检：拿到锁后再查一次缓存（可能已被其他协程填充）
	c.mu.RLock()
	if rule, ok := c.cache[domain]; ok && rule != nil && time.Since(rule.fetched) < time.Hour {
		c.mu.RUnlock()
		return rule
	}
	c.mu.RUnlock()

	// 4. 拉取 robots.txt（fetchRobots 恒返回非 nil）
	rule := c.fetchRobots(ctx, scheme, domain)
	c.mu.Lock()
	c.cache[domain] = rule
	c.mu.Unlock()
	return rule
}

// fetchRobots 拉取并解析 robots.txt。
// 恒返回非 nil：失败时返回空规则对象（视为无规则，默认允许），
// 避免 nil 入缓存导致后续解引用 panic。
func (c *RobotsChecker) fetchRobots(ctx context.Context, scheme, domain string) *robotsRule {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", scheme, domain)
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return &robotsRule{fetched: time.Now()} // 空规则
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return &robotsRule{fetched: time.Now()} // 拉取失败 → 空规则（默认允许）
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return &robotsRule{fetched: time.Now()}
	}

	// 解析（简单实现：匹配 User-agent: * 的规则）
	body := make([]byte, 8192)
	n, _ := resp.Body.Read(body)
	parsed := parseRobotsTxt(string(body[:n]))
	if parsed == nil {
		return &robotsRule{fetched: time.Now()}
	}
	return parsed
}

// parseRobotsTxt 解析 robots.txt 文本，提取 User-agent: * 的规则。
func parseRobotsTxt(content string) *robotsRule {
	rule := &robotsRule{fetched: time.Now()}
	lines := strings.Split(content, "\n")
	appliesToAll := true // 是否在 User-agent: * 段内

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 key: value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		key = strings.ToLower(key)

		if key == "user-agent" {
			appliesToAll = (value == "*")
			continue
		}

		if !appliesToAll {
			continue
		}

		switch key {
		case "disallow":
			rule.disallow = append(rule.disallow, value)
		case "allow":
			rule.allow = append(rule.allow, value)
		}
	}
	return rule
}

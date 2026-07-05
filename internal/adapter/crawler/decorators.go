package crawler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ===== Focused Crawler（聚焦爬虫 - 主题过滤装饰器）=====
//
// 包装任意基础爬虫，采集后用关键词过滤——只保留与主题相关的内容。
// 装饰器模式：不改变基础爬虫的实现，给它加一层"过滤"能力。

// FocusedCrawler 包装一个基础爬虫，增加主题关键词过滤。
type FocusedCrawler struct {
	inner    port.CrawlerTool // 被装饰的基础爬虫（复用 port 接口，单一接口定义点）
	keywords []string         // 主题关键词（内容必须包含至少一个才算相关）
}

// NewFocusedCrawler 创建聚焦爬虫装饰器。
// inner 是被装饰的基础爬虫（如 static_crawler），keywords 是主题关键词。
func NewFocusedCrawler(inner port.CrawlerTool, keywords []string) *FocusedCrawler {
	return &FocusedCrawler{inner: inner, keywords: keywords}
}

func (c *FocusedCrawler) Name() string { return "focused_" + c.inner.Name() }

func (c *FocusedCrawler) Description() string {
	return fmt.Sprintf("聚焦爬虫（包装 %s），只采集包含关键词 [%s] 的内容。", c.inner.Name(), strings.Join(c.keywords, ", "))
}

func (c *FocusedCrawler) ToolDeclaration() port.ToolDecl {
	d := c.inner.ToolDeclaration()
	d.Name = c.Name()
	return d
}

func (c *FocusedCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	result, err := c.inner.Execute(ctx, argsJSON)
	if err != nil {
		return entity.DataItem{}, err
	}
	// 过滤：内容必须包含至少一个关键词
	content := strings.ToLower(result.Content)
	matched := false
	for _, kw := range c.keywords {
		if strings.Contains(content, strings.ToLower(kw)) {
			matched = true
			break
		}
	}
	if !matched {
		return entity.DataItem{}, fmt.Errorf("content does not match focused keywords: %v", c.keywords)
	}
	return result, nil
}

// ===== Incremental Crawler（增量爬虫 - 只采新的）=====
//
// 包装基础爬虫，记录已采内容的指纹（SHA-256），跳过重复内容。
// 用内存 map 存储（后续可换 Redis）。

// IncrementalCrawler 包装基础爬虫，跳过已采集过的内容。
type IncrementalCrawler struct {
	inner port.CrawlerTool
	mu    sync.Mutex
	seen  map[string]bool // 已采集内容的指纹
}

// NewIncrementalCrawler 创建增量爬虫装饰器。
func NewIncrementalCrawler(inner port.CrawlerTool) *IncrementalCrawler {
	return &IncrementalCrawler{inner: inner, seen: make(map[string]bool)}
}

func (c *IncrementalCrawler) Name() string { return "incremental_" + c.inner.Name() }

func (c *IncrementalCrawler) Description() string {
	return fmt.Sprintf("增量爬虫（包装 %s），跳过已采集过的重复内容。", c.inner.Name())
}

func (c *IncrementalCrawler) ToolDeclaration() port.ToolDecl {
	d := c.inner.ToolDeclaration()
	d.Name = c.Name()
	return d
}

func (c *IncrementalCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	result, err := c.inner.Execute(ctx, argsJSON)
	if err != nil {
		return entity.DataItem{}, err
	}
	// 计算内容指纹
	fp := fingerprint(result.Content)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen[fp] {
		return entity.DataItem{}, fmt.Errorf("duplicate content (already crawled): %s", result.SourceURL)
	}
	c.seen[fp] = true
	return result, nil
}

// fingerprint 计算内容的 SHA-256 指纹。
func fingerprint(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}

// 编译期断言：三个装饰器都实现 port.CrawlerTool（装饰器模式要求装饰器和被装饰者同接口）。
var (
	_ port.CrawlerTool = (*FocusedCrawler)(nil)
	_ port.CrawlerTool = (*IncrementalCrawler)(nil)
	_ port.CrawlerTool = (*DeepCrawler)(nil)
	_ port.CrawlerTool = (*RateLimitCrawler)(nil)
)

// ===== RateLimit Crawler（限流装饰器）=====
//
// 包装任意爬虫，按 CrawlPolicy 在请求前等待，避免压垮目标站。
// 装饰器模式：给基础爬虫加一层"节流"能力，不改变其实现。
//
// 设计要点：
//   - 按 domain 记录上次请求时间，同域名两次请求间隔 ≥ RequestInterval。
//   - 线程安全（sync.Mutex 保护 lastSeen map）。
//   - policy 可由 config 注入，未来可扩展为 UI 动态调配。

// RateLimitCrawler 按策略限流的爬虫装饰器。
type RateLimitCrawler struct {
	inner     port.CrawlerTool
	policy    entity.CrawlPolicy
	mu        sync.Mutex
	lastSeen  map[string]time.Time // key = domain，记录上次请求时间
}

// NewRateLimitCrawler 创建限流装饰器。
// inner 是被装饰的基础爬虫，policy 定义限流策略。
func NewRateLimitCrawler(inner port.CrawlerTool, policy entity.CrawlPolicy) *RateLimitCrawler {
	if !policy.IsValid() {
		policy = entity.DefaultCrawlPolicy()
	}
	return &RateLimitCrawler{inner: inner, policy: policy, lastSeen: make(map[string]time.Time)}
}

func (c *RateLimitCrawler) Name() string { return c.inner.Name() }
func (c *RateLimitCrawler) Description() string { return c.inner.Description() }
func (c *RateLimitCrawler) ToolDeclaration() port.ToolDecl { return c.inner.ToolDeclaration() }

// Execute 在执行前按域名等待请求间隔。
func (c *RateLimitCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	// 从 argsJSON 提取 domain（尽力而为，失败则全局等待）
	domain := extractDomain(argsJSON)
	c.waitIfNeeded(domain)

	return c.inner.Execute(ctx, argsJSON)
}

// waitIfNeeded 若该域名距上次请求不足 RequestInterval，则等待补齐。
func (c *RateLimitCrawler) waitIfNeeded(domain string) {
	interval := time.Duration(c.policy.RequestIntervalMs) * time.Millisecond
	if interval <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := domain
	if key == "" {
		key = "_global"
	}
	last, exists := c.lastSeen[key]
	now := time.Now()
	if exists {
		elapsed := now.Sub(last)
		if elapsed < interval {
			time.Sleep(interval - elapsed)
		}
	}
	c.lastSeen[key] = time.Now()
}

// extractDomain 从 argsJSON 中尽力提取域名（用于按域限流）。
func extractDomain(argsJSON string) string {
	// argsJSON 形如 {"url":"https://example.com/..."}
	marker := `"url":"`
	idx := strings.Index(argsJSON, marker)
	if idx < 0 {
		return ""
	}
	rest := argsJSON[idx+len(marker):]
	endIdx := strings.IndexByte(rest, '"')
	if endIdx < 0 {
		return ""
	}
	rawURL := rest[:endIdx]
	// 提取 host：去掉 scheme
	if i := strings.Index(rawURL, "://"); i >= 0 {
		rawURL = rawURL[i+3:]
	}
	if i := strings.IndexByte(rawURL, '/'); i >= 0 {
		rawURL = rawURL[:i]
	}
	return rawURL
}

// ===== Deep Crawler（深度爬虫 - 列表→详情）=====
//
// 包装基础爬虫，先抓列表页提取链接，再逐个抓详情页。
// 两层采集：第一层拿链接列表，第二层拿详情内容。

// DeepCrawler 包装基础爬虫，增加深度采集（列表→详情）。
type DeepCrawler struct {
	inner port.CrawlerTool
}

// NewDeepCrawler 创建深度爬虫装饰器。
func NewDeepCrawler(inner port.CrawlerTool) *DeepCrawler {
	return &DeepCrawler{inner: inner}
}

func (c *DeepCrawler) Name() string { return "deep_" + c.inner.Name() }

func (c *DeepCrawler) Description() string {
	return fmt.Sprintf("深度爬虫（包装 %s），先抓列表页提取链接，再逐个抓取详情页内容。", c.inner.Name())
}

func (c *DeepCrawler) ToolDeclaration() port.ToolDecl {
	d := c.inner.ToolDeclaration()
	d.Name = c.Name()
	d.Properties["link_selector"] = port.PropSpec{Type: "string", Description: "列表页中详情链接的CSS选择器"}
	d.Properties["max_depth"] = port.PropSpec{Type: "integer", Description: "最多抓多少个详情页（默认5）"}
	return d
}

// deepCrawlerArgs 深度爬虫参数。
type deepCrawlerArgs struct {
	URL            string `json:"url"`               // 列表页 URL
	LinkSelector   string `json:"link_selector"`     // 列表页中详情链接的 CSS 选择器
	MaxDepth       int    `json:"max_depth"`          // 最多抓多少个详情页（默认 5）
}

func (c *DeepCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args deepCrawlerArgs
	if err := jsonUnmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.URL == "" {
		return entity.DataItem{}, fmt.Errorf("url is required")
	}
	if args.MaxDepth <= 0 {
		args.MaxDepth = 5
	}

	// 第一层：抓列表页，提取详情链接
	// 用 static_crawler 的逻辑抓列表页 HTML
	listItem, err := c.inner.Execute(ctx, argsJSON)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("crawl list page: %w", err)
	}

	// 从列表页 HTML 中提取详情链接
	links := extractLinks(listItem.RawContent, args.LinkSelector, args.URL)
	if len(links) == 0 {
		// 没有提取到链接，直接返回列表页内容
		return listItem, nil
	}

	// 第二层：逐个抓详情页
	var contents []string
	limit := len(links)
	if limit > args.MaxDepth {
		limit = args.MaxDepth
	}
	for i := 0; i < limit; i++ {
		detailArgs, _ := jsonMarshal(map[string]string{"url": links[i]})
		detailItem, err := c.inner.Execute(ctx, string(detailArgs))
		if err != nil {
			continue // 单个详情页失败不影响整体
		}
		contents = append(contents, fmt.Sprintf("## %s\nURL: %s\n\n%s", detailItem.Title, links[i], detailItem.Content))
	}

	// 合并所有详情页内容
	combined := strings.Join(contents, "\n\n---\n\n")
	return entity.DataItem{
		ID:         fmt.Sprintf("deep-%d", time.Now().UnixNano()),
		Title:      listItem.Title,
		Content:    combined,
		SourceURL:  args.URL,
		RawContent: combined,
		Status:     entity.ItemStatusPendingReview,
		Metadata:   map[string]string{"crawler_type": "deep", "pages_crawled": fmt.Sprintf("%d", len(contents))},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// extractLinks 从 HTML 中提取链接。
func extractLinks(html, selector, baseURL string) []string {
	// 简单实现：提取所有 href="..." 的链接
	// 生产环境用 goquery，这里用字符串匹配
	var links []string
	marker := `href="`
	for {
		idx := strings.Index(html, marker)
		if idx < 0 {
			break
		}
		html = html[idx+len(marker):]
		endIdx := strings.Index(html, `"`)
		if endIdx < 0 {
			break
		}
		href := html[:endIdx]
		// 转绝对 URL（简化版：只保留 http 开头的）
		if strings.HasPrefix(href, "http") {
			links = append(links, href)
		} else if strings.HasPrefix(href, "/") && baseURL != "" {
			links = append(links, baseURL + href)
		}
	}
	return links
}

// 辅助函数
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

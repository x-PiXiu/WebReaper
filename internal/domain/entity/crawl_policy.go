package entity

// CrawlPolicy 爬虫限流策略（值对象）。
//
// 设计动机（整洁架构 + 装饰器模式）：
//   - 爬虫速率限制是横切关注点，不应散落在每个爬虫里。
//   - 把策略抽象为值对象，由 RateLimitCrawler 装饰器统一执行。
//   - 策略可由 config 启动时注入，未来可扩展为 UI 动态调配。
//
// 限流维度：
//   - RequestInterval：同域名两次请求的最小间隔（礼貌爬取，避免压垮目标站）
//   - RequestTimeout：单次请求的超时时间
//   - MaxRetries：失败重试次数（本轮预留，默认 0 不重试）
//   - RespectRobots：是否遵守 robots.txt（默认 true，关闭则无视 robots 强制爬取）
type CrawlPolicy struct {
	RequestIntervalMs int  `json:"request_interval_ms"` // 请求间隔（毫秒），默认 1000
	RequestTimeoutMs  int  `json:"request_timeout_ms"`  // 单请求超时（毫秒），默认 30000
	MaxRetries        int  `json:"max_retries"`         // 最大重试次数，默认 0
	RespectRobots     bool `json:"respect_robots"`      // 是否遵守 robots.txt，默认 true
}

// DefaultCrawlPolicy 默认爬虫策略（礼貌但不激进）。
func DefaultCrawlPolicy() CrawlPolicy {
	return CrawlPolicy{
		RequestIntervalMs: 1000,
		RequestTimeoutMs:  30000,
		MaxRetries:        0,
		RespectRobots:     true,
	}
}

// IsValid 策略合法性校验。
func (p CrawlPolicy) IsValid() bool {
	return p.RequestIntervalMs >= 0 && p.RequestTimeoutMs > 0
}

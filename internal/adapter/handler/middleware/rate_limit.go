package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter IP 维度令牌桶限流器。
//
// 设计（跨切面关注点——gin 中间件，不依赖业务逻辑）：
//   - 每个 IP 一个令牌桶，按时间补充令牌
//   - 超限时返回 429（前端可提示"请求过于频繁"）
//   - 轻量内存实现（map + mutex）；阶段 2 多实例可换 Redis
type RateLimiter struct {
	buckets map[string]*tokenBucket
	mu      sync.Mutex
	rate    int // 每秒补充令牌数
	burst   int // 桶容量（最大突发）
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimiter 创建限流器。
// rate: 每秒令牌数（如 20 = 每秒最多 20 请求）
// burst: 突发上限（如 40 = 瞬间最多 40 请求）
func NewRateLimiter(rate, burst int) *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*tokenBucket), rate: rate, burst: burst}
}

// Middleware gin 中间件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	// 后台清理过期桶（防内存泄漏——长时间不活跃的 IP 清掉）
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-30 * time.Minute)
			for ip, b := range rl.buckets {
				if b.last.Before(cutoff) {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		rl.mu.Lock()
		b, ok := rl.buckets[ip]
		if !ok {
			b = &tokenBucket{tokens: float64(rl.burst), last: time.Now()}
			rl.buckets[ip] = b
		}
		// 按时间补充令牌
		elapsed := time.Since(b.last).Seconds()
		b.tokens += elapsed * float64(rl.rate)
		if b.tokens > float64(rl.burst) {
			b.tokens = float64(rl.burst)
		}
		b.last = time.Now()
		if b.tokens < 1 {
			rl.mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": 42900, "msg": "请求过于频繁，请稍后重试",
			})
			c.Abort()
			return
		}
		b.tokens--
		rl.mu.Unlock()
		c.Next()
	}
}

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/port"
)

// HandleDebugMetrics GET /api/v1/debug/metrics —— 运营指标聚合输出（R3）。
// 输出 JSON：全部 metric:* 计数器 + 缓存命中率（从 RedisCache.Stats() 取进程内计数）。
// 仅 admin 可访问（路由层 RequireRole 守卫）。
func (r *Router) HandleDebugMetrics(c *gin.Context) {
	if r.metrics == nil {
		c.JSON(http.StatusOK, gin.H{"metrics": gin.H{}, "note": "未配置 Redis——指标不可用"})
		return
	}
	all, err := r.metrics.All(c.Request.Context(), "")
	if err != nil {
		fail(c, err)
		return
	}

	// 计算派生指标
	calls := all[port.MetricLLMCalls]
	errs := all[port.MetricLLMErrors]
	hits := all[port.MetricCacheHits]
	misses := all[port.MetricCacheMisses]

	llmSuccessRate := 1.0
	if calls > 0 {
		llmSuccessRate = float64(calls-errs) / float64(calls)
	}
	cacheHitRate := 0.0
	if hits+misses > 0 {
		cacheHitRate = float64(hits) / float64(hits+misses)
	}

	c.JSON(http.StatusOK, gin.H{
		"counters": all,
		"derived": gin.H{
			"llm_success_rate":  llmSuccessRate,
			"cache_hit_rate":    cacheHitRate,
		},
	})
}

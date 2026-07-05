package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/crawlconfig"
)

// CrawlConfigHandler 采集配置的 HTTP 适配器（薄 handler）。
type CrawlConfigHandler struct {
	uc *crawlconfig.CrawlConfigUseCase
}

func NewCrawlConfigHandler(uc *crawlconfig.CrawlConfigUseCase) *CrawlConfigHandler {
	return &CrawlConfigHandler{uc: uc}
}

// HandlePolicy GET /api/v1/crawl-policy —— 公开的采集政策声明（无需认证）
// 让外部可查询本系统的合规承诺与当前采集行为参数。
func (h *CrawlConfigHandler) HandlePolicy(c *gin.Context) {
	p, _ := h.uc.GetPolicy(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"system": "WebReaper",
		"policy": "本系统遵守 robots.txt、礼貌爬取、对个人数据脱敏。详见 Docs/crawl-policy.md",
		"current_settings": gin.H{
			"request_interval_ms": p.RequestIntervalMs,
			"request_timeout_ms":  p.RequestTimeoutMs,
			"respect_robots":      p.RespectRobots,
		},
		"pii_redaction": gin.H{
			"email":     "保留首字符和域名，其余打码",
			"phone":     "保留前3后4，中间打码",
			"id_card":   "保留前6后4，中间打码",
			"bank_card": "保留前4后4，中间打码",
		},
		"opt_out": "在 robots.txt 中禁止 WebReaper UA 或相关路径即可",
	})
}

// HandleGet GET /api/v1/crawl-config —— 读取当前采集配置（需认证）
func (h *CrawlConfigHandler) HandleGet(c *gin.Context) {
	p, err := h.uc.GetPolicy(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"request_interval_ms": p.RequestIntervalMs,
		"request_timeout_ms":  p.RequestTimeoutMs,
		"max_retries":         p.MaxRetries,
		"respect_robots":      p.RespectRobots,
	})
}

// HandleUpdate PUT /api/v1/crawl-config —— 更新采集配置
func (h *CrawlConfigHandler) HandleUpdate(c *gin.Context) {
	var req struct {
		RequestIntervalMs int  `json:"request_interval_ms"`
		RequestTimeoutMs  int  `json:"request_timeout_ms"`
		MaxRetries        int  `json:"max_retries"`
		RespectRobots     *bool `json:"respect_robots"` // 指针区分"未传"和"传 false"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	// 读取当前值作为基底（未传的字段保留原值）
	cur, _ := h.uc.GetPolicy(c.Request.Context())
	policy := entity.CrawlPolicy{
		RequestIntervalMs: req.RequestIntervalMs,
		RequestTimeoutMs:  req.RequestTimeoutMs,
		MaxRetries:        req.MaxRetries,
		RespectRobots:     cur.RespectRobots,
	}
	if req.RequestIntervalMs == 0 {
		policy.RequestIntervalMs = cur.RequestIntervalMs
	}
	if req.RequestTimeoutMs == 0 {
		policy.RequestTimeoutMs = cur.RequestTimeoutMs
	}
	if req.RespectRobots != nil {
		policy.RespectRobots = *req.RespectRobots
	}
	if err := h.uc.UpdatePolicy(c.Request.Context(), policy); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"request_interval_ms": policy.RequestIntervalMs,
		"request_timeout_ms":  policy.RequestTimeoutMs,
		"max_retries":         policy.MaxRetries,
		"respect_robots":      policy.RespectRobots,
	})
}

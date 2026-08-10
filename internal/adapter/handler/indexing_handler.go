package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
)

// ---- 收录管理端点（管理后台，admin 角色）----

// HandleGetIndexingConfig GET /api/v1/admin/indexing/config —— 读收录配置。
// 注意：返回包含凭据（IndexNow key / 百度 token）——仅 admin 可访问（路由已限角色）。
func (r *Router) HandleGetIndexingConfig(c *gin.Context) {
	if r.indexingUC == nil {
		fail(c, errNotConfigured("收录管理"))
		return
	}
	cfg, err := r.indexingUC.GetConfig(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"index_now_key": cfg.IndexNowKey,
		"baidu_site":    cfg.BaiduSite,
		"baidu_token":   cfg.BaiduToken,
		"updated_at":    cfg.UpdatedAt,
	})
}

// indexingConfigRequest 收录配置更新请求体。
type indexingConfigRequest struct {
	IndexNowKey string `json:"index_now_key"`
	BaiduSite   string `json:"baidu_site"`
	BaiduToken  string `json:"baidu_token"`
}

// HandleUpdateIndexingConfig PUT /api/v1/admin/indexing/config —— 更新收录配置。
// 修改后 30s 内生效（submitter TTL 缓存自动重建，无需重启）。
func (r *Router) HandleUpdateIndexingConfig(c *gin.Context) {
	if r.indexingUC == nil {
		fail(c, errNotConfigured("收录管理"))
		return
	}
	var req indexingConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg := entity.IndexingConfig{
		IndexNowKey: req.IndexNowKey,
		BaiduSite:   req.BaiduSite,
		BaiduToken:  req.BaiduToken,
		UpdatedAt:   time.Now(),
	}
	if err := r.indexingUC.UpdateConfig(c.Request.Context(), cfg); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"ok": true, "note": "收录配置已保存，30 秒内生效"})
}

// HandleListIndexingLogs GET /api/v1/admin/indexing/logs —— 收录提交日志（审计排查）。
func (r *Router) HandleListIndexingLogs(c *gin.Context) {
	if r.indexingUC == nil {
		fail(c, errNotConfigured("收录管理"))
		return
	}
	logs, err := r.indexingUC.ListLogs(c.Request.Context(), 50)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		views = append(views, gin.H{
			"id": l.ID, "channel": l.Channel, "url": l.URL,
			"status": l.Status, "error_msg": l.ErrorMsg,
			"submitted_at": l.SubmittedAt,
		})
	}
	success(c, views)
}

// HandleReSubmitAll POST /api/v1/admin/indexing/re-submit —— 手动补提交全部已发布内容。
// 使用场景：渠道故障后补推 / 内容大规模更新后重推。
func (r *Router) HandleReSubmitAll(c *gin.Context) {
	if r.indexingUC == nil {
		fail(c, errNotConfigured("收录管理"))
		return
	}
	submitted, failed, err := r.indexingUC.ReSubmitAll(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"submitted": submitted, "failed": failed})
}

// HandleGenerateIndexingKey POST /api/v1/admin/indexing/generate-key —— 自动生成 IndexNow 密钥。
// IndexNow 协议：密钥由网站所有者生成（所有权证明）——系统代为生成 GUID 并自动托管 key 文件。
func (r *Router) HandleGenerateIndexingKey(c *gin.Context) {
	if r.indexingUC == nil {
		fail(c, errNotConfigured("收录管理"))
		return
	}
	key, err := r.indexingUC.GenerateKey(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"index_now_key": key})
}

// HandleVerifyIndexingKey GET /api/v1/admin/indexing/verify-key —— 验证密钥文件可公开访问。
// 以搜索引擎视角探测 {baseURL}/{key}.txt（200 且内容一致 = 验证通过）。
func (r *Router) HandleVerifyIndexingKey(c *gin.Context) {
	if r.indexingUC == nil {
		fail(c, errNotConfigured("收录管理"))
		return
	}
	result, err := r.indexingUC.VerifyKey(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, result)
}

// errNotConfigured 未装配时的统一错误。
func errNotConfigured(what string) error {
	return &configMissingError{what: what}
}

type configMissingError struct{ what string }

func (e *configMissingError) Error() string { return e.what + "未启用（需配置数据库）" }

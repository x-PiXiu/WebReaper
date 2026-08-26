package handler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// ---- 平台知识库管理端点（管理后台，admin 角色）----
// 管理后台动态修改：向量嵌入模型 / 向量库 / 行业采集配置（30s 内生效，免重启）。

// HandleGetKnowledgeEmbeddingConfig GET /api/v1/admin/knowledge/embedding-config —— 读向量配置。
// 注意：返回包含 API Key——仅 admin 可访问（路由已限角色）。
func (r *Router) HandleGetKnowledgeEmbeddingConfig(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	cfg, err := r.knowledgeUC.GetEmbeddingConfig(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"model":             cfg.Model,
		"base_url":          cfg.BaseURL,
		"api_key":           cfg.APIKey,
		"dimensions":        cfg.Dimensions,
		"vector_db":         cfg.EffectiveVectorDB(),
		"milvus_host":       cfg.MilvusHost,
		"milvus_port":       cfg.MilvusPort,
		"milvus_collection": cfg.MilvusCollection,
		"updated_at":        cfg.UpdatedAt,
	})
}

// knowledgeEmbeddingConfigRequest 向量配置更新请求体。
type knowledgeEmbeddingConfigRequest struct {
	Model            string `json:"model"`
	BaseURL          string `json:"base_url"`
	APIKey           string `json:"api_key"`
	Dimensions       int    `json:"dimensions"` // 0=模型默认（智谱 embedding-3 默认 2048，可设 256-2048）
	VectorDB         string `json:"vector_db"`  // mysql（默认）/ milvus
	MilvusHost       string `json:"milvus_host"`
	MilvusPort       string `json:"milvus_port"`
	MilvusCollection string `json:"milvus_collection"`
}

// HandleUpdateKnowledgeEmbeddingConfig PUT /api/v1/admin/knowledge/embedding-config —— 更新向量配置。
// 修改后 30s 内生效（CachedEmbedder / VectorStoreProvider TTL 自动重建，无需重启）。
func (r *Router) HandleUpdateKnowledgeEmbeddingConfig(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	var req knowledgeEmbeddingConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg := entity.EmbeddingRuntimeConfig{
		Model: req.Model, BaseURL: req.BaseURL, APIKey: req.APIKey,
		VectorDB: req.VectorDB,
		MilvusHost: req.MilvusHost, MilvusPort: req.MilvusPort, MilvusCollection: req.MilvusCollection,
		UpdatedAt: time.Now(),
	}
	if err := r.knowledgeUC.SaveEmbeddingConfig(c.Request.Context(), cfg); err != nil {
		fail(c, err)
		return
	}
	note := "向量配置已保存，30 秒内生效"
	if cfg.EffectiveVectorDB() == entity.VectorDBMilvus {
		note += "；⚠️ milvus 驱动未接入时检索将报错（保持 mysql 可运行）"
	}
	success(c, gin.H{"ok": true, "note": note})
}

// HandleGetKnowledgeCrawlConfig GET /api/v1/admin/knowledge/crawl-config —— 读行业采集配置。
func (r *Router) HandleGetKnowledgeCrawlConfig(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	cfgs, err := r.knowledgeUC.GetIndustryConfigs(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, cfgs)
}

// HandleUpdateKnowledgeCrawlConfig PUT /api/v1/admin/knowledge/crawl-config —— 更新行业采集配置。
// 修改后下一轮采集任务（每 6h）按新配置执行。
func (r *Router) HandleUpdateKnowledgeCrawlConfig(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	var cfgs []entity.IndustryCrawlConfig
	if err := c.ShouldBindJSON(&cfgs); err != nil {
		fail(c, err)
		return
	}
	if err := r.knowledgeUC.SaveIndustryConfigs(c.Request.Context(), cfgs); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"ok": true, "note": "行业采集配置已保存，下一轮采集任务生效"})
}

// HandleReindexKnowledgeMaterials POST /api/v1/admin/knowledge/reindex —— 重建素材向量。
// 用途：管理后台换 embedding 模型后，存量向量为旧模型产物——重建恢复检索正确性。
// 参数：?industry=餐饮（空=全部）&only_missing=true（仅补无向量素材，增量模式）。
func (r *Router) HandleReindexKnowledgeMaterials(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	industry := c.Query("industry")
	onlyMissing := c.Query("only_missing") == "true"
	processed, updated, failed, err := r.knowledgeUC.ReindexMaterials(c.Request.Context(), industry, onlyMissing)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"processed": processed, "updated": updated, "failed": failed,
		"note": "向量重建完成（素材更新时自动同步向量库）",
	})
}

// HandleCrawlKnowledgeNow POST /api/v1/admin/knowledge/crawl —— 手动触发一轮行业采集。
// 用途：配置行业后立即采集（不等 6h 定时任务）；排查采集链路问题。
func (r *Router) HandleCrawlKnowledgeNow(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	if err := r.knowledgeUC.CrawlIndustries(c.Request.Context()); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"ok": true, "note": "采集完成（详情见服务端日志）"})
}

// HandleSearchKnowledgeMaterials GET /api/v1/admin/knowledge/search —— 检索验证/调试。
// 参数：query 必填（检索词）；industry 可选（行业过滤）；num 可选（默认 3）。
// 返回带来源的素材引用——管理后台验证"生成时注入什么"。
func (r *Router) HandleSearchKnowledgeMaterials(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	query := c.Query("query")
	if query == "" {
		fail(c, pkg.ErrInvalidArgument)
		return
	}
	industry := c.Query("industry")
	num := 3
	if n, err := strconv.Atoi(c.Query("num")); err == nil && n > 0 && n <= 10 {
		num = n
	}
	// 30 秒硬超时兜底：Milvus/embedding 不可达时 SDK 内部可能不响应 context cancel
	// ——goroutine 隔离 + select 确保响应一定发出（管理后台不挂死）
	type searchResult struct {
		refs []entity.MaterialRef
		err  error
	}
	done := make(chan searchResult, 1)
	timer := time.NewTimer(30 * time.Second)
	go func() {
		searchCtx, searchCancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer searchCancel()
		refs, err := r.knowledgeUC.SearchMaterials(searchCtx, industry, query, num)
		done <- searchResult{refs, err}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			fail(c, res.err)
			return
		}
		success(c, res.refs)
	case <-timer.C:
		fail(c, fmt.Errorf("知识库检索超时（30 秒）——Milvus 向量库或 embedding API 不可达。请检查管理后台「知识库配置」中的向量库地址与 API 配额"))
		return
	}
}

// HandleGetKnowledgeCrawlInterval GET /api/v1/admin/knowledge/crawl-interval —— 读采集间隔（分钟）。
func (r *Router) HandleGetKnowledgeCrawlInterval(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	minutes, err := r.knowledgeUC.GetCrawlIntervalMinutes(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"interval_minutes": minutes})
}

// crawlIntervalRequest 采集间隔更新请求体。
type crawlIntervalRequest struct {
	IntervalMinutes int `json:"interval_minutes"`
}

// HandleUpdateKnowledgeCrawlInterval PUT /api/v1/admin/knowledge/crawl-interval —— 更新采集间隔。
// 修改后下个周期生效（scheduler 每周期刷新 Interval，免重启）；范围 30-1440 分钟。
func (r *Router) HandleUpdateKnowledgeCrawlInterval(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	var req crawlIntervalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := r.knowledgeUC.SaveCrawlIntervalMinutes(c.Request.Context(), req.IntervalMinutes); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"ok": true, "note": "采集间隔已保存，下个周期生效（免重启）"})
}

// HandleGetKnowledgeStats GET /api/v1/admin/knowledge/stats —— 知识库素材统计。
func (r *Router) HandleGetKnowledgeStats(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	total, err := r.knowledgeUC.Count(c.Request.Context(), "")
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"total_materials": total})
}

// HandleListKnowledgeMaterials GET /api/v1/admin/knowledge/materials —— 素材列表（分页，按行业可选）。
func (r *Router) HandleListKnowledgeMaterials(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	industry := c.Query("industry")
	limit, offset := 20, 0
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 && n <= 100 {
		limit = n
	}
	if n, err := strconv.Atoi(c.Query("offset")); err == nil && n >= 0 {
		offset = n
	}
	list, err := r.knowledgeUC.ListByIndustry(c.Request.Context(), industry, limit, offset)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(list))
	for _, m := range list {
		views = append(views, gin.H{
			"id": m.ID, "industry": m.Industry, "title": m.Title,
			"source_url": m.SourceURL, "summary": m.Summary,
			"crawl_keyword": m.CrawlKeyword, "status": m.Status,
			"has_vector": len(m.Embedding) > 0, "created_at": m.CreatedAt,
		})
	}
	success(c, views)
}

// HandleDeleteKnowledgeMaterial DELETE /api/v1/admin/knowledge/materials/:id —— 删除素材（含向量）。
func (r *Router) HandleDeleteKnowledgeMaterial(c *gin.Context) {
	if r.knowledgeUC == nil {
		fail(c, errNotConfigured("知识库管理"))
		return
	}
	id := c.Param("id")
	if id == "" {
		fail(c, pkg.ErrInvalidArgument)
		return
	}
	if err := r.knowledgeUC.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"ok": true})
}

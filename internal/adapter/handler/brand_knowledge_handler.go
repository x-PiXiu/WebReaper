package handler

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/knowledge"
)

// BrandKnowledgeHandler 商户端品牌知识库 HTTP 适配器（获客智能体转型新增）。
//
// 多租户：JWT 取 tenant_id + 路由参数取 brand_id；品牌必须属于该租户（FindByID 内部校验）。
// 上入流程：文本 → embedder 向量化 → kb_materials（brand_id/tenant_id 标记）→ 向量库同步。
type BrandKnowledgeHandler struct {
	uc *knowledge.KnowledgeUseCase
}

func NewBrandKnowledgeHandler(uc *knowledge.KnowledgeUseCase) *BrandKnowledgeHandler {
	return &BrandKnowledgeHandler{uc: uc}
}

// HandleUploadMaterial POST /geo/brands/:id/knowledge/materials
// 商户上传品牌知识库素材（纯文本粘贴或 .txt/.md 文件内容）。
// 自动：清洗 → 内容指纹去重 → 向量化 → 入库（brand_id/tenant_id 标记品牌私有）。
func (h *BrandKnowledgeHandler) HandleUploadMaterial(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")

	var req struct {
		Title   string `json:"title" binding:"required"`   // 素材标题（商户起名或从文件名取）
		Content string `json:"content" binding:"required"` // 正文（纯文本；文件场景由前端读取后传入）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	// 清洗（复用领域规则：最小长度/截断/摘要）
	content := strings.TrimSpace(req.Content)
	runes := []rune(content)
	if len(runes) < 50 {
		fail(c, fmt.Errorf("%w: 内容太短（至少 50 字）——请粘贴有实质内容的品牌资料", pkg.ErrInvalidArgument))
		return
	}
	if len(runes) > entity.MaterialContentMaxRunes {
		content = string(runes[:entity.MaterialContentMaxRunes])
	}
	summary := content
	if len(runes) > entity.MaterialSummaryMaxRunes {
		summary = string(runes[:entity.MaterialSummaryMaxRunes])
	}
	title := strings.TrimSpace(req.Title)
	if len([]rune(title)) > 100 {
		title = string([]rune(title)[:100])
	}

	mat, err := h.uc.UploadBrandMaterial(c.Request.Context(), tenantID, brandID, title, content, summary)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"id":        mat.ID,
		"title":     mat.Title,
		"has_vector": len(mat.Embedding) > 0,
		"message":   "AI 已学习你的资料——写文章和做视频时会自动引用",
	})
}

// HandleListMaterials GET /geo/brands/:id/knowledge/materials
// 列出品牌私有素材（created_at 降序；tenantID 隔离）。
func (h *BrandKnowledgeHandler) HandleListMaterials(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")

	materials, total, err := h.uc.ListBrandMaterials(c.Request.Context(), tenantID, brandID)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(materials))
	for _, m := range materials {
		out = append(out, gin.H{
			"id":          m.ID,
			"title":       m.Title,
			"summary":     m.Summary,
			"has_vector":  len(m.Embedding) > 0,
			"crawl_keyword": m.CrawlKeyword,
			"created_at":  m.CreatedAt,
		})
	}
	success(c, gin.H{"materials": out, "total": total})
}

// HandleDeleteMaterial DELETE /geo/brands/:id/knowledge/materials/:mid
// 删除品牌私有素材（tenantID 隔离——只能删自己品牌的）。
func (h *BrandKnowledgeHandler) HandleDeleteMaterial(c *gin.Context) {
	tenantID := middleware.CurrentTenantID(c)
	brandID := c.Param("id")
	materialID := c.Param("mid")

	if err := h.uc.DeleteBrandMaterial(c.Request.Context(), tenantID, brandID, materialID); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": true})
}

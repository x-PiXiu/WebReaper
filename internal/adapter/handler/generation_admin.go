package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// GenerationAdminHandler 管理后台：端点×模型规格全局掌控（generation_specs 表驱动）。
type GenerationAdminHandler struct {
	registry port.EndpointRegistry
	specRepo port.GenerationSpecRepository
}

// NewGenerationAdminHandler 创建规格管理 handler。
func NewGenerationAdminHandler(registry port.EndpointRegistry, specRepo port.GenerationSpecRepository) *GenerationAdminHandler {
	return &GenerationAdminHandler{registry: registry, specRepo: specRepo}
}

// HandleListSpecs GET /admin/generation/specs —— 全量矩阵（端点×模型×能力，含代码默认回退）。
func (h *GenerationAdminHandler) HandleListSpecs(c *gin.Context) {
	if h.registry == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	type view struct {
		entity.GenerationSpec
		HasOverride bool `json:"has_override"` // true=DB 覆盖行（可删除恢复出厂）
	}
	dbList, _ := h.specRepo.ListAll(c.Request.Context())
	dbSet := map[string]bool{}
	for _, s := range dbList {
		dbSet[s.SubType+"|"+s.Model] = true
	}
	specs := h.registry.AllSpecs(c.Request.Context())
	out := make([]view, 0, len(specs))
	for _, s := range specs {
		out = append(out, view{GenerationSpec: s, HasOverride: dbSet[s.SubType+"|"+s.Model]})
	}
	success(c, gin.H{"specs": out})
}

// HandleSaveSpec PUT /admin/generation/specs/:subType/:model —— 保存（新增模型/修改能力/停用）。
func (h *GenerationAdminHandler) HandleSaveSpec(c *gin.Context) {
	if h.specRepo == nil {
		fail(c, fmt.Errorf("规格仓储未注入"))
		return
	}
	subType := c.Param("subType")
	model := c.Param("model")
	var req struct {
		Endpoint string `json:"endpoint"`
		Enabled  *bool  `json:"enabled"`
		Capability *entity.ModelCapability `json:"capability"` // 能力向量（结构化编辑）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	// 能力 JSON：结构化对象优先；缺省读现有（修改开关不丢能力）
	capsJSON := ""
	if req.Capability != nil {
		req.Capability.Model = model
		b, _ := json.Marshal(req.Capability)
		capsJSON = string(b)
	} else {
		if existing, err := h.specRepo.Find(c.Request.Context(), subType, model); err == nil {
			capsJSON = existing.CapabilitiesJSON
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	endpoint := req.Endpoint
	if endpoint == "" {
		endpoint = existingEndpoint(h.registry, subType)
	}
	if err := h.specRepo.Upsert(c.Request.Context(), entity.GenerationSpec{
		SubType: subType, Model: model, Endpoint: endpoint,
		Enabled: enabled, CapabilitiesJSON: capsJSON,
	}); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"saved": subType + "/" + model})
}

// HandleDeleteSpec DELETE /admin/generation/specs/:subType/:model —— 删除行（恢复出厂默认）。
func (h *GenerationAdminHandler) HandleDeleteSpec(c *gin.Context) {
	if h.specRepo == nil {
		fail(c, fmt.Errorf("规格仓储未注入"))
		return
	}
	if err := h.specRepo.Delete(c.Request.Context(), c.Param("subType"), c.Param("model")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": c.Param("subType") + "/" + c.Param("model")})
}

// existingEndpoint 取端点默认路径（新模型未填 endpoint 时用端点默认）。
func existingEndpoint(registry port.EndpointRegistry, subType string) string {
	if registry == nil {
		return ""
	}
	if a, err := registry.Get(context.Background(), subType); err == nil {
		return a.Endpoint()
	}
	return ""
}

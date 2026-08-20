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

// ---- 模式开关（傻瓜化：商户端生成模式按 sub_type 批量启停）----

// modeTier 推荐模式分层（admin 开关的建议档位；tier 仅作展示分组，实际状态以 DB 为准）。
var modeTier = map[string]string{
	// 默认集：两类商户（实体店/线上运营）都用得上的核心创作能力
	"text2image": "default", "text2video": "default", "img2video": "default",
	"tts": "default", "digital_human": "default", "reference2video": "default",
	// 进阶折叠：参考生视频的配套（音色互通 voice_id / 主体库 server_id 复用）
	"voice_clone": "advanced", "subject": "advanced",
	// 默认关闭：专业创作者功能（admin 可开）
	"start_end2video": "closed", "multiframe": "closed", "text2audio": "closed",
	"sound_effect": "closed", "template": "closed",
}

// HandleListModes GET /admin/generation/modes —— 模式开关状态（按 sub_type 聚合）。
// enabled=该模式全部模型启用；partial=部分启用（开关显示为开，提示有 N 个模型单独停用）。
func (h *GenerationAdminHandler) HandleListModes(c *gin.Context) {
	if h.registry == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	specs := h.registry.AllSpecs(c.Request.Context())
	type modeView struct {
		SubType    string `json:"sub_type"`
		Tier       string `json:"tier"`        // default/advanced/closed（推荐档位）
		Enabled    bool   `json:"enabled"`     // 全部模型启用
		Partial    bool   `json:"partial"`     // 部分模型启用
		ModelCount int    `json:"model_count"` // 该模式模型数
	}
	agg := map[string]*modeView{}
	for _, s := range specs {
		mv := agg[s.SubType]
		if mv == nil {
			mv = &modeView{SubType: s.SubType, Tier: modeTier[s.SubType], Enabled: true, ModelCount: 0}
			if mv.Tier == "" {
				mv.Tier = "closed" // 未知模式默认归关闭档（如后续新端点）
			}
			agg[s.SubType] = mv
		}
		mv.ModelCount++
		if !s.Enabled {
			mv.Enabled = false
			mv.Partial = true
		}
	}
	out := make([]modeView, 0, len(agg))
	for _, mv := range agg {
		out = append(out, *mv)
	}
	success(c, gin.H{"modes": out})
}

// HandleSetMode PUT /admin/generation/modes/:subType —— 模式开关（批量启停该模式全部模型）。
// 保留各模型现有能力 JSON（只改 Enabled）；客户端 listGenerationTypes 自动收敛。
func (h *GenerationAdminHandler) HandleSetMode(c *gin.Context) {
	if h.specRepo == nil || h.registry == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	subType := c.Param("subType")
	var req struct {
		Enabled bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	specs := h.registry.AllSpecs(c.Request.Context())
	saved := 0
	for _, s := range specs {
		if s.SubType != subType {
			continue
		}
		if err := h.specRepo.Upsert(c.Request.Context(), entity.GenerationSpec{
			SubType: s.SubType, Model: s.Model, Endpoint: s.Endpoint,
			Enabled: req.Enabled, CapabilitiesJSON: s.CapabilitiesJSON,
		}); err != nil {
			fail(c, fmt.Errorf("保存 %s/%s 失败: %w", s.SubType, s.Model, err))
			return
		}
		saved++
	}
	if saved == 0 {
		fail(c, fmt.Errorf("未知模式: %s", subType))
		return
	}
	success(c, gin.H{"saved": saved})
}

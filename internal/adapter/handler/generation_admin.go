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
		Provider   string `json:"provider"`
		Endpoint   string `json:"endpoint"`
		Enabled    *bool  `json:"enabled"`
		IsDefault  *bool  `json:"is_default"`
		SortOrder  *int   `json:"sort_order"`
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
	isDefault := false
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}
	provider := req.Provider
	if provider == "" {
		provider = "vidu" // 默认厂商
	}
	endpoint := req.Endpoint
	if endpoint == "" {
		endpoint = existingEndpoint(h.registry, subType)
	}
	if err := h.specRepo.Upsert(c.Request.Context(), entity.GenerationSpec{
		SubType: subType, Model: model, Provider: provider, Endpoint: endpoint,
		Enabled: enabled, IsDefault: isDefault, SortOrder: sortOrder, CapabilitiesJSON: capsJSON,
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

// HandleSetDefault PUT /admin/generation/specs/:subType/:model/default —— 设置默认模型。
func (h *GenerationAdminHandler) HandleSetDefault(c *gin.Context) {
	if h.specRepo == nil {
		fail(c, fmt.Errorf("规格仓储未注入"))
		return
	}
	subType := c.Param("subType")
	model := c.Param("model")
	provider := c.DefaultQuery("provider", "vidu")

	if err := h.specRepo.SetDefault(c.Request.Context(), provider, subType, model); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"default": subType + "/" + model})
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
// 08 计划 D1 收敛：傻瓜式定位只保留口播主链五端点（reference2video/subject/
// lip_sync/tts/voice_clone）为 default 档；其余专业创作模式默认关闭（admin 可开）。
var modeTier = map[string]string{
	// 默认集：口播视频主链路（向导/资产库/创作页）
	"reference2video": "default", "subject": "default", "lip_sync": "default",
	"tts": "default", "voice_clone": "default",
	// 默认关闭：专业创作者功能（admin 可开）
	"text2video": "closed", "img2video": "closed", "start_end2video": "closed",
	"multiframe": "closed", "digital_human": "closed", "text2image": "closed",
	"text2audio": "closed", "sound_effect": "closed", "template": "closed",
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

// HandleApplyRecommendedModes POST /admin/generation/modes/apply-recommended ——
// 一键应用推荐档位（08 计划 D1 收敛）：default 档全开、closed 档全关。
// 存量部署的收敛入口（新部署 seed 已收敛）；运营 reopen 的模式会被此操作覆盖——
// 按钮文案需明示"将覆盖手动调整"。
func (h *GenerationAdminHandler) HandleApplyRecommendedModes(c *gin.Context) {
	if h.specRepo == nil || h.registry == nil {
		fail(c, fmt.Errorf("生成服务未配置"))
		return
	}
	specs := h.registry.AllSpecs(c.Request.Context())
	enabled, disabled := 0, 0
	for _, s := range specs {
		want := modeTier[s.SubType] != "closed" // default/advanced 档开
		if s.Enabled == want {
			continue // 已符合
		}
		if err := h.specRepo.Upsert(c.Request.Context(), entity.GenerationSpec{
			SubType: s.SubType, Model: s.Model, Endpoint: s.Endpoint,
			Enabled: want, CapabilitiesJSON: s.CapabilitiesJSON,
		}); err != nil {
			fail(c, fmt.Errorf("保存 %s/%s 失败: %w", s.SubType, s.Model, err))
			return
		}
		if want {
			enabled++
		} else {
			disabled++
		}
	}
	success(c, gin.H{"enabled": enabled, "disabled": disabled})
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
		Enabled *bool `json:"enabled"` // 使用指针类型，避免 false 被当作零值
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if req.Enabled == nil {
		fail(c, fmt.Errorf("enabled 参数必填"))
		return
	}
	enabled := *req.Enabled
	specs := h.registry.AllSpecs(c.Request.Context())
	saved := 0
	for _, s := range specs {
		if s.SubType != subType {
			continue
		}
		if err := h.specRepo.Upsert(c.Request.Context(), entity.GenerationSpec{
			SubType: s.SubType, Model: s.Model, Endpoint: s.Endpoint,
			Enabled: enabled, CapabilitiesJSON: s.CapabilitiesJSON,
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

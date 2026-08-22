package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/llmconfig"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/providerconfig"
)

// IntegrationHandler 第三方集成中心（08 计划 D7——能力路由模型）。
//
// 设计：不建新表，聚合层直读 provider_configs + llm_configs + generation_specs
// 三张现有表，拼成统一"集成条目"视图。新增第三方 = handler 加一个元数据条目，
// 页面框架零改动（插件式注册模式）。

// getProvider 从厂商配置列表中按 provider 名查找。
func (h *IntegrationHandler) getProvider(ctx context.Context, name string) (entity.ProviderConfig, bool) {
	providers, _ := h.providerUC.List(ctx)
	for _, p := range providers {
		if p.Provider == name {
			return p, true
		}
	}
	return entity.ProviderConfig{}, false
}

// upsertProvider 保存厂商配置。
func (h *IntegrationHandler) upsertProvider(ctx context.Context, cfg entity.ProviderConfig) error {
	return h.providerUC.Upsert(ctx, cfg)
}

type IntegrationHandler struct {
	providerUC *providerconfig.UseCase
	specRepo   port.GenerationSpecRepository
	llmCfgUC   *llmconfig.LLMConfigUseCase
	registry   port.EndpointRegistry
	provider   port.GenerationProvider
	intRepo    integrationRepo // 新表（能力路由）
}

// integrationRepo 接口投影（解耦具体 GORM 实现）。
type integrationRepo interface {
	ListVendors(ctx context.Context) ([]entity.IntegrationVendor, error)
	ListCapabilities(ctx context.Context) ([]entity.IntegrationCapability, error)
	SaveVendor(ctx context.Context, v entity.IntegrationVendor) error
	SaveCapability(ctx context.Context, c entity.IntegrationCapability) error
	SetDefault(ctx context.Context, capID, vendorID string) error
	DeleteCapability(ctx context.Context, id string) error
}

// NewIntegrationHandler 创建集成中心 handler。
func NewIntegrationHandler(
	providerUC *providerconfig.UseCase,
	specRepo port.GenerationSpecRepository,
	llmCfgUC *llmconfig.LLMConfigUseCase,
	registry port.EndpointRegistry,
	provider port.GenerationProvider,
	intRepo integrationRepo,
) *IntegrationHandler {
	return &IntegrationHandler{
		providerUC: providerUC, specRepo: specRepo, llmCfgUC: llmCfgUC,
		registry: registry, provider: provider, intRepo: intRepo,
	}
}

// ---- 集成元数据（插件式注册——新增第三方在此加一条）----

// integrationMeta 集成条目元数据（静态描述 + 能力标签 + 详情区块列表）。
type integrationMeta struct {
	ID           string   `json:"id"`            // vidu / llm / asr / tavily / zpay
	Name         string   `json:"name"`          // 显示名
	Icon         string   `json:"icon"`          // 图标标识（前端映射）
	Desc         string   `json:"desc"`          // 一句话描述
	Capabilities []string `json:"capabilities"`  // 能力标签：generation / asr / llm / search / payment
	Sections     []string `json:"sections"`      // 详情页注册的区块 ID（前端按此渲染）
	Vendor       string   `json:"vendor"`        // 厂商标识（分组用）
}

// integrationMetas 全部集成条目（新增第三方 = 此表加一行 + 注册区块）。
var integrationMetas = []integrationMeta{
	{
		ID: "vidu", Name: "Vidu", Icon: "video-camera", Vendor: "智谱清影",
		Desc: "视频 / 图片 / 音频 / 数字人生成",
		Capabilities: []string{"generation"},
		Sections:     []string{"overview", "api-key", "modes", "models", "preferred-model", "voices", "callback-health", "usage"},
	},
	{
		ID: "llm", Name: "LLM 对话", Icon: "robot", Vendor: "多厂商",
		Desc: "AI 文案生成 / 改写 / 品牌知识库问答",
		Capabilities: []string{"llm"},
		Sections:     []string{"llm-configs"},
	},
	{
		ID: "asr", Name: "语音识别", Icon: "audio", Vendor: "多厂商",
		Desc: "视频说话内容提取（支持硅基流动/OpenAI/智谱等）",
		Capabilities: []string{"asr"},
		Sections:     []string{"asr-config"},
	},
	{
		ID: "tavily", Name: "Tavily 搜索", Icon: "search", Vendor: "Tavily",
		Desc: "AI 专用高质量搜索 API（监测/GEO 用）",
		Capabilities: []string{"search"},
		Sections:     []string{"tavily-config"},
	},
	{
		ID: "zpay", Name: "ZPAY 支付", Icon: "wallet", Vendor: "ZPAY",
		Desc: "商户收款网关",
		Capabilities: []string{"payment"},
		Sections:     []string{"zpay-config"},
	},
}

// HandleList GET /admin/integrations —— 集成中心列表。
// 支持 ?view=vendor（按厂商分组，默认）| ?view=capability（按能力分组）。
func (h *IntegrationHandler) HandleList(c *gin.Context) {
	ctx := c.Request.Context()
	view := c.DefaultQuery("view", "vendor")

	// 读各数据源拼健康状态
	providers, _ := h.providerUC.List(ctx)
	providerMap := map[string]entity.ProviderConfig{}
	for _, p := range providers {
		providerMap[p.Provider] = p
	}

	type entry struct {
		IntegrationMeta integrationMeta `json:"meta"`
		Configured      bool            `json:"configured"`
		Enabled         bool            `json:"enabled"`
		HasKey          bool            `json:"has_key"`
		HealthStatus    string          `json:"health_status"` // ok / degraded / down / unknown
		HealthDetail    string          `json:"health_detail"`
		UpdatedAt       string          `json:"updated_at"`
	}

	entries := make([]entry, 0, len(integrationMetas))
	for _, meta := range integrationMetas {
		e := entry{IntegrationMeta: meta, HealthStatus: "unknown"}
		switch meta.ID {
		case "vidu":
			if cfg, ok := providerMap["vidu"]; ok {
				e.Configured = cfg.APIKey != ""
				e.Enabled = cfg.Enabled
				e.HasKey = cfg.APIKey != ""
				e.UpdatedAt = cfg.UpdatedAt.Format(time.RFC3339)
				if !cfg.Enabled {
					e.HealthStatus = "down"; e.HealthDetail = "已停用"
				} else if cfg.APIKey == "" {
					e.HealthStatus = "degraded"; e.HealthDetail = "未配置 Key"
				} else {
					e.HealthStatus = "ok"
				}
			}
		case "asr":
			if cfg, ok := providerMap["asr"]; ok {
				e.Configured = cfg.APIKey != ""
				e.Enabled = cfg.Enabled
				e.HasKey = cfg.APIKey != ""
				e.UpdatedAt = cfg.UpdatedAt.Format(time.RFC3339)
				if cfg.APIKey == "" || !cfg.Enabled {
					e.HealthStatus = "degraded"; e.HealthDetail = "未配置或已停用"
				} else {
					e.HealthStatus = "ok"
				}
			}
		case "tavily":
			if cfg, ok := providerMap["tavily"]; ok {
				e.Configured = cfg.APIKey != ""
				e.Enabled = cfg.Enabled
				e.HasKey = cfg.APIKey != ""
				e.UpdatedAt = cfg.UpdatedAt.Format(time.RFC3339)
				if cfg.APIKey != "" && cfg.Enabled {
					e.HealthStatus = "ok"
				} else {
					e.HealthStatus = "degraded"
				}
			}
		case "zpay":
			if cfg, ok := providerMap["zpay"]; ok {
				e.Configured = cfg.APIKey != "" || cfg.ExtraJSON != ""
				e.Enabled = cfg.Enabled
				e.HasKey = cfg.APIKey != ""
				e.UpdatedAt = cfg.UpdatedAt.Format(time.RFC3339)
				if e.Configured && cfg.Enabled {
					e.HealthStatus = "ok"
				} else {
					e.HealthStatus = "down"; e.HealthDetail = "未配置（走 mock）"
				}
			}
		case "llm":
			e.Configured = true // LLM 配置始终可查
			e.Enabled = true
			e.HealthStatus = "ok"
		}
		entries = append(entries, e)
	}

	// 按视图分组
	type group struct {
		Label   string  `json:"label"`
		Entries []entry `json:"entries"`
	}
	var groups []group
	if view == "capability" {
		capMap := map[string]*group{}
		capOrder := []string{"generation", "llm", "asr", "search", "payment"}
		capLabels := map[string]string{"generation": "视频生成", "llm": "AI 对话", "asr": "语音识别", "search": "搜索", "payment": "支付"}
		for _, k := range capOrder {
			capMap[k] = &group{Label: capLabels[k]}
		}
		for _, e := range entries {
			for _, cap := range e.IntegrationMeta.Capabilities {
				if g, ok := capMap[cap]; ok {
					g.Entries = append(g.Entries, e)
				}
			}
		}
		for _, k := range capOrder {
			if g := capMap[k]; len(g.Entries) > 0 {
				groups = append(groups, *g)
			}
		}
	} else {
		// 按厂商分组（默认）
		vendorMap := map[string]*group{}
		vendorOrder := []string{}
		for _, e := range entries {
			vendor := e.IntegrationMeta.Vendor
			if vendor == "" { vendor = "其他" }
			if _, ok := vendorMap[vendor]; !ok {
				vendorMap[vendor] = &group{Label: vendor}
				vendorOrder = append(vendorOrder, vendor)
			}
			vendorMap[vendor].Entries = append(vendorMap[vendor].Entries, e)
		}
		for _, v := range vendorOrder {
			groups = append(groups, *vendorMap[v])
		}
	}
	success(c, gin.H{"integrations": entries, "groups": groups, "view": view})
}

// HandleDetail GET /admin/integrations/:id —— 厂商详情（聚合所有区块数据）。
func (h *IntegrationHandler) HandleDetail(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	var meta *integrationMeta
	for i := range integrationMetas {
		if integrationMetas[i].ID == id {
			meta = &integrationMetas[i]
			break
		}
	}
	if meta == nil {
		c.JSON(404, gin.H{"code": 40400, "msg": "未知集成: " + id})
		return
	}

	result := gin.H{"meta": meta, "sections": gin.H{}}

	switch id {
	case "vidu":
		h.buildViduDetail(ctx, result)
	case "llm":
		h.buildLLMDetail(ctx, result)
	case "asr":
		h.buildASRDetail(ctx, result)
	case "tavily":
		h.buildTavilyDetail(ctx, result)
	case "zpay":
		h.buildZPAYDetail(ctx, result)
	}

	success(c, result)
}

// buildViduDetail Vidu 详情：Key/启停 + 模式开关 + 模型矩阵 + 首选模型 + 音色 + 积分。
func (h *IntegrationHandler) buildViduDetail(ctx context.Context, result gin.H) {
	sections := gin.H{}

	// overview / api-key
	if cfg, ok := h.getProvider(ctx, "vidu"); ok {
		key := cfg.APIKey
		if key != "" {
			key = maskKey(key)
		}
		sections["overview"] = gin.H{"enabled": cfg.Enabled, "has_key": cfg.APIKey != "", "key_masked": key, "updated_at": cfg.UpdatedAt}
		sections["api-key"] = gin.H{"key_masked": key, "enabled": cfg.Enabled}
	}

	// modes（从 generation_specs 聚合）
	if h.registry != nil {
		specs := h.registry.AllSpecs(ctx)
		type modeView struct {
			SubType    string `json:"sub_type"`
			Tier       string `json:"tier"`
			Enabled    bool   `json:"enabled"`
			ModelCount int    `json:"model_count"`
		}
		agg := map[string]*modeView{}
		for _, s := range specs {
			if s.SubType == "subject" { continue } // 主体无模式概念
			mv := agg[s.SubType]
			if mv == nil {
				mv = &modeView{SubType: s.SubType}
				agg[s.SubType] = mv
			}
			mv.ModelCount++
			if !s.Enabled {
				mv.Enabled = false
			} else if mv.ModelCount == 1 {
				mv.Enabled = true // 全部模型启用
			}
		}
		modes := make([]modeView, 0, len(agg))
		for _, mv := range agg { modes = append(modes, *mv) }
		sections["modes"] = modes
	}

	// models（完整矩阵）
	if h.specRepo != nil {
		if specs, err := h.specRepo.ListAll(ctx); err == nil {
			sections["models"] = specs
		}
	}

	// voices（音色库只读）
	sections["voices"] = gin.H{"note": "GET /api/v1/generation/voices 查看完整音色列表"}

	// callback-health（回调配置）
	if h.registry != nil {
		sections["callback-health"] = gin.H{"note": "回调地址由用例层注入（PUBLIC_BASE_URL + /api/v1/generation/callback）"}
	}

	result["sections"] = sections
}

// buildLLMDetail LLM 详情：全部配置条目。
func (h *IntegrationHandler) buildLLMDetail(ctx context.Context, result gin.H) {
	if h.llmCfgUC == nil {
		result["sections"] = gin.H{"llm-configs": []any{}}
		return
	}
	configs, _ := h.llmCfgUC.List(ctx)
	type llmView struct {
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		Model     string `json:"model"`
		IsDefault bool   `json:"is_default"`
		HasKey    bool   `json:"has_key"`
	}
	out := make([]llmView, 0, len(configs))
	for _, c := range configs {
		out = append(out, llmView{Name: c.Name, BaseURL: c.BaseURL, Model: c.Model, IsDefault: c.IsDefault, HasKey: c.APIKey != ""})
	}
	result["sections"] = gin.H{"llm-configs": out}
}

// buildASRDetail ASR 详情：当前配置 + 可选服务商列表。
func (h *IntegrationHandler) buildASRDetail(ctx context.Context, result gin.H) {
	cfg, _ := h.getProvider(ctx, "asr")
	var ac struct {
		Endpoint      string `json:"endpoint"`
		Model         string `json:"model"`
		ResponseStyle string `json:"response_style"`
	}
	if cfg.ExtraJSON != "" {
		_ = json.Unmarshal([]byte(cfg.ExtraJSON), &ac)
	}
	if ac.Endpoint == "" {
		ac.Endpoint = cfg.BaseURL
	}
	result["sections"] = gin.H{
		"asr-config": gin.H{
			"has_key":       cfg.APIKey != "",
			"key_masked":    maskKey(cfg.APIKey),
			"enabled":       cfg.Enabled,
			"endpoint":      ac.Endpoint,
			"model":         ac.Model,
			"response_style": ac.ResponseStyle,
			"updated_at":    cfg.UpdatedAt,
			"providers": []gin.H{
				{"id": "siliconflow", "name": "硅基流动 SenseVoice（推荐，免费）", "endpoint": "https://api.siliconflow.cn/v1/audio/transcriptions", "model": "FunAudioLLM/SenseVoiceSmall", "free": true},
				{"id": "openai", "name": "OpenAI Whisper", "endpoint": "https://api.openai.com/v1/audio/transcriptions", "model": "whisper-1"},
				{"id": "zhipu", "name": "智谱 GLM-ASR", "endpoint": "https://open.bigmodel.cn/api/paas/v4/audio/transcriptions", "model": "glm-asr", "note": "返回 chat.completion 结构，需 response_style=chat"},
			},
		},
	}
}

// buildTavilyDetail Tavily 详情。
func (h *IntegrationHandler) buildTavilyDetail(ctx context.Context, result gin.H) {
	cfg, _ := h.getProvider(ctx, "tavily")
	result["sections"] = gin.H{"tavily-config": gin.H{
		"has_key": cfg.APIKey != "", "key_masked": maskKey(cfg.APIKey), "enabled": cfg.Enabled, "updated_at": cfg.UpdatedAt,
	}}
}

// buildZPAYDetail ZPAY 详情。
func (h *IntegrationHandler) buildZPAYDetail(ctx context.Context, result gin.H) {
	cfg, _ := h.getProvider(ctx, "zpay")
	result["sections"] = gin.H{"zpay-config": gin.H{
		"has_key": cfg.APIKey != "", "key_masked": maskKey(cfg.APIKey), "enabled": cfg.Enabled,
		"base_url": cfg.BaseURL, "extra_json": cfg.ExtraJSON, "updated_at": cfg.UpdatedAt,
	}}
}

// HandleHealth GET /admin/integrations/:id/health —— 单厂商健康检查。
func (h *IntegrationHandler) HandleHealth(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	status := "unknown"
	detail := ""

	switch id {
	case "vidu":
		if h.provider != nil {
			credits, err := h.provider.QueryCredits(ctx)
			if err != nil {
				status = "degraded"; detail = "积分查询失败: " + err.Error()
			} else {
				status = "ok"; detail = fmt.Sprintf("剩余积分 %d", credits)
				if credits == 0 { status = "degraded"; detail = "积分为 0——任务会被拒" }
			}
		}
	case "asr":
		cfg, _ := h.getProvider(ctx, "asr")
		if cfg.APIKey == "" || !cfg.Enabled {
			status = "degraded"; detail = "未配置或已停用"
		} else {
			status = "ok"; detail = "已配置（" + cfg.BaseURL + "）"
		}
	default:
		cfg, _ := h.getProvider(ctx, id)
		if cfg.APIKey != "" && cfg.Enabled {
			status = "ok"
		} else {
			status = "degraded"
		}
	}
	success(c, gin.H{"id": id, "status": status, "detail": detail, "checked_at": time.Now()})
}

// HandlePreferredModel PUT /admin/integrations/vidu/preferred-model —— 首选模型配置。
// D3 自动切换的可配化：图片主体首选 / 视频主体首选。
func (h *IntegrationHandler) HandlePreferredModel(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		ImageSubject string `json:"image_subject"` // 如 viduq3-turbo
		VideoSubject string `json:"video_subject"` // 如 viduq2-pro
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err); return
	}
	cfg, _ := h.getProvider(ctx, "vidu")
	extra := map[string]string{}
	if cfg.ExtraJSON != "" { _ = json.Unmarshal([]byte(cfg.ExtraJSON), &extra) }
	if req.ImageSubject != "" { extra["preferred_image_subject"] = req.ImageSubject }
	if req.VideoSubject != "" { extra["preferred_video_subject"] = req.VideoSubject }
	extraJSON, _ := json.Marshal(extra)
	cfg.ExtraJSON = string(extraJSON)
	if err := h.upsertProvider(ctx, cfg); err != nil {
		fail(c, err); return
	}
	success(c, gin.H{"saved": true})
}

// ---- 能力路由管理端点（新表 integration_vendors + integration_capabilities）----

// HandleVendors GET /admin/integrations/vendors —— 全部厂商列表（含能力条目）。
func (h *IntegrationHandler) HandleVendors(c *gin.Context) {
	if h.intRepo == nil {
		fail(c, fmt.Errorf("能力路由未配置")); return
	}
	ctx := c.Request.Context()
	vendors, _ := h.intRepo.ListVendors(ctx)
	caps, _ := h.intRepo.ListCapabilities(ctx)
	// 按 vendor 分组能力
	capMap := map[string][]entity.IntegrationCapability{}
	for _, c := range caps {
		capMap[c.VendorID] = append(capMap[c.VendorID], c)
	}
	type vendorView struct {
		entity.IntegrationVendor
		Capabilities []entity.IntegrationCapability `json:"capabilities"`
	}
	out := make([]vendorView, 0, len(vendors))
	for _, v := range vendors {
		out = append(out, vendorView{IntegrationVendor: v, Capabilities: capMap[v.ID]})
	}
	success(c, gin.H{"vendors": out})
}

// HandleSaveVendor PUT /admin/integrations/vendors/:id —— 保存厂商（启停/改 Key/改端点）。
func (h *IntegrationHandler) HandleSaveVendor(c *gin.Context) {
	if h.intRepo == nil {
		fail(c, fmt.Errorf("能力路由未配置")); return
	}
	id := c.Param("id")
	var req struct {
		Name     string `json:"name"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Protocol string `json:"protocol"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err); return
	}
	// 读现有（部分更新语义：空值保留）
	existing, _ := h.intRepo.ListVendors(c.Request.Context())
	var old entity.IntegrationVendor
	for _, v := range existing {
		if v.ID == id { old = v; break }
	}
	if old.ID == "" {
		old.ID = id
	}
	if req.Name != "" { old.Name = req.Name }
	if req.BaseURL != "" { old.BaseURL = req.BaseURL }
	if req.APIKey != "" { old.APIKey = req.APIKey }
	if req.Protocol != "" { old.Protocol = req.Protocol }
	if req.Enabled != nil { old.Enabled = *req.Enabled }
	if err := h.intRepo.SaveVendor(c.Request.Context(), old); err != nil {
		fail(c, err); return
	}
	success(c, gin.H{"saved": id})
}

// HandleCapabilities GET /admin/integrations/capabilities —— 全部能力路由列表。
func (h *IntegrationHandler) HandleCapabilities(c *gin.Context) {
	if h.intRepo == nil {
		fail(c, fmt.Errorf("能力路由未配置")); return
	}
	caps, _ := h.intRepo.ListCapabilities(c.Request.Context())
	success(c, gin.H{"capabilities": caps})
}

// HandleSetCapabilityDefault PUT /admin/integrations/capabilities/:id/default
// 设置某能力的默认厂商（同 capId 下互斥）。
// :id 仅用于路由匹配，实际 cap_id 从请求体取（前端传 cap_id 如 "asr"）。
func (h *IntegrationHandler) HandleSetCapabilityDefault(c *gin.Context) {
	if h.intRepo == nil {
		fail(c, fmt.Errorf("能力路由未配置")); return
	}
	var req struct {
		CapID    string `json:"cap_id" binding:"required"`
		VendorID string `json:"vendor_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err); return
	}
	if err := h.intRepo.SetDefault(c.Request.Context(), req.CapID, req.VendorID); err != nil {
		fail(c, err); return
	}
	success(c, gin.H{"cap_id": req.CapID, "default_vendor": req.VendorID})
}

// HandleSaveCapability PUT /admin/integrations/capabilities/save —— 保存能力条目（新增/编辑）。
// id 在请求体中（# 在 URL 中会被截断为片段标识符）。
func (h *IntegrationHandler) HandleSaveCapability(c *gin.Context) {
	if h.intRepo == nil {
		fail(c, fmt.Errorf("能力路由未配置")); return
	}
	var req struct {
		ID       string `json:"id"`       // 复合 ID 如 "asr#siliconflow"（新增时必填）
		CapID    string `json:"cap_id"`   // 能力类型（新增时必填）
		VendorID string `json:"vendor_id"` // 厂商（新增时必填）
		Enabled  *bool  `json:"enabled"`
		Model    string `json:"model"`
		Endpoint string `json:"endpoint"`
		ExtraJSON string `json:"extra_json"`
		IsDefault *bool  `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err); return
	}
	id := req.ID
	if id == "" && req.CapID != "" && req.VendorID != "" {
		id = req.CapID + "#" + req.VendorID
	}
	if id == "" {
		fail(c, fmt.Errorf("缺少 id 或 cap_id+vendor_id")); return
	}
	// 查现有（编辑场景）
	existing, _ := h.intRepo.ListCapabilities(c.Request.Context())
	var old entity.IntegrationCapability
	for _, cap := range existing {
		if cap.ID == id { old = cap; break }
	}
	if old.ID == "" {
		// 新增
		old = entity.IntegrationCapability{
			ID: id, CapID: req.CapID, VendorID: req.VendorID,
		}
	}
	if req.Model != "" { old.Model = req.Model }
	if req.Endpoint != "" { old.Endpoint = req.Endpoint }
	if req.ExtraJSON != "" { old.ExtraJSON = req.ExtraJSON }
	if req.Enabled != nil { old.Enabled = *req.Enabled }
	if req.IsDefault != nil { old.IsDefault = *req.IsDefault }
	if err := h.intRepo.SaveCapability(c.Request.Context(), old); err != nil {
		fail(c, err); return
	}
	success(c, gin.H{"saved": id})
}

// HandleDeleteCapability DELETE /admin/integrations/capabilities/delete —— 删除能力条目。
// id 在请求体中（# 在 URL 中会被截断）。
func (h *IntegrationHandler) HandleDeleteCapability(c *gin.Context) {
	if h.intRepo == nil {
		fail(c, fmt.Errorf("能力路由未配置")); return
	}
	var req struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err); return
	}
	if err := h.intRepo.DeleteCapability(c.Request.Context(), req.ID); err != nil {
		fail(c, err); return
	}
	success(c, gin.H{"deleted": req.ID})
}

func maskKey(key string) string {
	if len(key) <= 8 { return "***" }
	return key[:4] + "***" + key[len(key)-4:]
}

package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/providerconfig"
)

// ProviderConfigHandler 厂商配置管理（管理后台：按厂商设置 API Key / 启用）。
type ProviderConfigHandler struct {
	uc       *providerconfig.UseCase
	provider port.GenerationProvider // 已装配的生成厂商（用于热更新，可空）
}

// NewProviderConfigHandler 创建厂商配置 handler。
// provider 为当前装配的生成 provider（若实现了 port.ConfigurableProvider 则保存后热生效）。
func NewProviderConfigHandler(uc *providerconfig.UseCase, provider port.GenerationProvider) *ProviderConfigHandler {
	return &ProviderConfigHandler{uc: uc, provider: provider}
}

// HandleList GET /api/v1/admin/provider-configs —— 全部厂商配置（Key 掩码脱敏）。
func (h *ProviderConfigHandler) HandleList(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("厂商配置服务未配置"))
		return
	}
	list, err := h.uc.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, cfg := range list {
		out = append(out, providerConfigToView(cfg, true))
	}
	success(c, gin.H{"providers": out})
}

// HandleSave PUT /api/v1/admin/provider-configs/:provider —— 保存厂商配置。
// api_key 为空 = 不修改（前端掩码语义）；非空则保存并热更新已装配 provider。
func (h *ProviderConfigHandler) HandleSave(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("厂商配置服务未配置"))
		return
	}
	provider := c.Param("provider")
	var req struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg := entity.ProviderConfig{
		Provider: provider,
		APIKey:   req.APIKey,
		BaseURL:  req.BaseURL,
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	// 读现有配置：APIKey 为空 = 保留原 Key（掩码语义）
	if cfg.APIKey == "" {
		if existing, err := h.uc.List(c.Request.Context()); err == nil {
			for _, e := range existing {
				if e.Provider == provider {
					cfg.APIKey = e.APIKey
					break
				}
			}
		}
	}
	if err := h.uc.Upsert(c.Request.Context(), cfg); err != nil {
		fail(c, err)
		return
	}
	// 热更新已装配厂商（无需重启）
	if req.APIKey != "" {
		updateProviderAPIKey(h.provider, req.APIKey)
	}
	saved, _ := h.uc.List(c.Request.Context())
	out := make([]gin.H, 0, len(saved))
	for _, cfg := range saved {
		out = append(out, providerConfigToView(cfg, true))
	}
	success(c, gin.H{"providers": out})
}

// updateProviderAPIKey 若厂商实现了 ConfigurableProvider 则热更新 Key。
func updateProviderAPIKey(p port.GenerationProvider, key string) {
	if cp, ok := p.(port.ConfigurableProvider); ok {
		cp.UpdateAPIKey(key)
	}
}

func providerConfigToView(cfg entity.ProviderConfig, masked bool) gin.H {
	key := cfg.APIKey
	if masked && key != "" {
		key = providerconfig.MaskKey(key)
	}
	return gin.H{
		"provider":  cfg.Provider,
		"api_key":   key,
		"has_key":   cfg.APIKey != "",
		"base_url":  cfg.BaseURL,
		"enabled":   cfg.Enabled,
		"updated_at": cfg.UpdatedAt,
	}
}

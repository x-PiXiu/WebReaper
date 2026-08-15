package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/llmconfig"
)

// Agent 配置与 LLM 配置管理 handler（从原 data_handler 拆出，dataitem 部分已随域删除）。

func (r *Router) handleListAgentConfigs(c *gin.Context) {
	configs, err := r.agentCfgUC.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(configs))
	for _, cfg := range configs {
		views = append(views, agentConfigToView(cfg))
	}
	success(c, views)
}

func (r *Router) handleCreateAgentConfig(c *gin.Context) {
	var req struct {
		Name          string   `json:"name" binding:"required"`
		SystemPrompt  string   `json:"system_prompt" binding:"required"`
		Tools         []string `json:"tools"`
		LLMConfigName string   `json:"llm_config_name"`
		MaxIterations int      `json:"max_iterations"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg, err := r.agentCfgUC.Create(c.Request.Context(), agentconfig.CreateInput{
		Name: req.Name, SystemPrompt: req.SystemPrompt,
		LLMConfigName: req.LLMConfigName, MaxIterations: req.MaxIterations,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, agentConfigToView(cfg))
}

func (r *Router) handleDeleteAgentConfig(c *gin.Context) {
	name := c.Param("name")
	if err := r.agentCfgUC.Delete(c.Request.Context(), name); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": name})
}

// handleUpdateAgentConfig PUT /api/v1/agents/:name —— 部分更新 Agent 配置。
func (r *Router) handleUpdateAgentConfig(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		SystemPrompt  *string  `json:"system_prompt"`
		Tools         []string `json:"tools"`
		LLMConfigName *string  `json:"llm_config_name"`
		MaxIterations *int     `json:"max_iterations"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg, err := r.agentCfgUC.Update(c.Request.Context(), name, agentconfig.UpdateInput{
		SystemPrompt:  req.SystemPrompt,
		LLMConfigName: req.LLMConfigName,
		MaxIterations: req.MaxIterations,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, agentConfigToView(cfg))
}

func agentConfigToView(cfg entity.AgentConfig) gin.H {
	return gin.H{
		"name":            cfg.Name,
		"system_prompt":   cfg.SystemPrompt,
		"tools":           cfg.Tools,
		"llm_config_name": cfg.LLMConfigName,
		"max_iterations":  cfg.MaxIterations,
	}
}

// ---- LLM 配置（薄 handler：DTO 转换 + 调用 llmCfgUC）----

// llmConfigToView 完整厂商配置视图（含 api_key）——仅 admin 路由可达。
// 商户端引擎选择走 handleListEngineNames（两个用例、两个 Output Model）。
func llmConfigToView(cfg entity.LLMConfig) gin.H {
	return gin.H{
		"name":           cfg.Name,
		"provider":       cfg.Provider,
		"api_key":        cfg.APIKey,
		"base_url":       cfg.BaseURL,
		"model":          cfg.Model,
		"cost_per_mtok":  cfg.CostPerMTok,
	}
}

// handleListEngineNames GET /api/v1/geo/engines —— 引擎名单（商户端速查/矩阵执行选择用）。
// 仅暴露 name/provider/model（展示必需，均非敏感）；厂商密钥不进入商户可达视图。
func (r *Router) handleListEngineNames(c *gin.Context) {
	configs, err := r.llmCfgUC.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	type engineOptionView struct {
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	views := make([]engineOptionView, 0, len(configs))
	for _, cfg := range configs {
		views = append(views, engineOptionView{Name: cfg.Name, Provider: cfg.Provider, Model: cfg.Model})
	}
	success(c, views)
}

func (r *Router) handleListLLMConfigs(c *gin.Context) {
	configs, err := r.llmCfgUC.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(configs))
	for _, cfg := range configs {
		views = append(views, llmConfigToView(cfg))
	}
	success(c, views)
}

func (r *Router) handleCreateLLMConfig(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Provider    string `json:"provider"`
		APIKey      string `json:"api_key" binding:"required"`
		BaseURL     string `json:"base_url"`
		Model       string `json:"model"`
		CostPerMTok int    `json:"cost_per_mtok"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg, err := r.llmCfgUC.Create(c.Request.Context(), llmconfig.CreateInput{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		BaseURL: req.BaseURL, Model: req.Model, CostPerMTok: req.CostPerMTok,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, llmConfigToView(cfg))
}

func (r *Router) handleDeleteLLMConfig(c *gin.Context) {
	name := c.Param("name")
	if err := r.llmCfgUC.Delete(c.Request.Context(), name); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": name})
}

func (r *Router) handleUpdateLLMConfig(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Provider    *string `json:"provider"`
		APIKey      *string `json:"api_key"`
		BaseURL     *string `json:"base_url"`
		Model       *string `json:"model"`
		CostPerMTok *int    `json:"cost_per_mtok"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg, err := r.llmCfgUC.Update(c.Request.Context(), name, llmconfig.UpdateInput{
		Provider: req.Provider, APIKey: req.APIKey,
		BaseURL: req.BaseURL, Model: req.Model, CostPerMTok: req.CostPerMTok,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, llmConfigToView(cfg))
}

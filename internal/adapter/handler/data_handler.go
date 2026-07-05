package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/agentconfig"
	"webreaper/internal/usecase/dataitem"
	"webreaper/internal/usecase/llmconfig"
)

// ---- 任务查询（薄 handler：DTO 转换 + 调用 taskQueryUC）----

func (r *Router) handleGetTask(c *gin.Context) {
	id := c.Param("id")
	t, err := r.taskQueryUC.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, taskToView(t))
}

func (r *Router) handleListTasks(c *gin.Context) {
	tasks, err := r.taskQueryUC.List(c.Request.Context(), 50)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, taskToView(t))
	}
	success(c, views)
}

func taskToView(t entity.Task) gin.H {
	return gin.H{
		"id":         t.ID,
		"type":       string(t.Type),
		"status":     string(t.Status),
		"error":      t.Error,
		"output":     t.Output,
		"progress":   t.Progress,
		"created_at": t.CreatedAt,
		"updated_at": t.UpdatedAt,
	}
}

// ---- 数据项（薄 handler：DTO 转换 + 调用 dataItemUC）----

func dataItemToView(item entity.DataItem) gin.H {
	return gin.H{
		"id":            item.ID,
		"collection_id": item.CollectionID,
		"title":         item.Title,
		"content":       item.Content,
		"summary":       item.Summary,
		"tags":          item.Tags,
		"source_url":    item.SourceURL,
		"raw_content":   item.RawContent,
		"status":        string(item.Status),
		"metadata":      item.Metadata,
		"created_at":    item.CreatedAt,
	}
}

func (r *Router) handleListDataItems(c *gin.Context) {
	items, err := r.dataItemUC.ListDataItems(c.Request.Context(), 50)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for _, item := range items {
		views = append(views, dataItemToView(item))
	}
	success(c, views)
}

func (r *Router) handleApproveItem(c *gin.Context) {
	id := c.Param("id")
	out, err := r.dataItemUC.Approve(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"id": out.ItemID, "status": "approved", "message": out.Message})
}

func (r *Router) handleRejectItem(c *gin.Context) {
	id := c.Param("id")
	if err := r.dataItemUC.Reject(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"id": id, "status": "rejected"})
}

// handleCreateFromContent POST /api/v1/data-items/from-content
// 把 LLM 对话生成的结构化内容落库为 DataItem（打通"对话生成→自动落库"闭环）。
func (r *Router) handleCreateFromContent(c *gin.Context) {
	var req struct {
		Content      string `json:"content" binding:"required"`
		FieldMapping string `json:"field_mapping"`
		SourceURL    string `json:"source_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	item, err := r.dataItemUC.CreateFromContent(c.Request.Context(), dataitem.CreateFromContentInput{
		Content: req.Content, FieldMapping: req.FieldMapping, SourceURL: req.SourceURL,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, dataItemToView(item))
}

// ---- 采集集合 ----

func collectionToView(c entity.Collection) gin.H {
	return gin.H{
		"id":         c.ID,
		"name":       c.Name,
		"agent_name": c.AgentName,
		"task_id":    c.TaskID,
		"status":     string(c.Status),
		"item_count": c.ItemCount,
		"created_at": c.CreatedAt,
	}
}

func (r *Router) handleListCollections(c *gin.Context) {
	cols, err := r.dataItemUC.ListCollections(c.Request.Context(), 50)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(cols))
	for _, col := range cols {
		views = append(views, collectionToView(col))
	}
	success(c, views)
}

// ---- Agent 配置（薄 handler：DTO 转换 + 调用 agentCfgUC）----

func agentConfigToView(cfg entity.AgentConfig) gin.H {
	return gin.H{
		"name":            cfg.Name,
		"system_prompt":   cfg.SystemPrompt,
		"tools":           cfg.Tools,
		"llm_config_name": cfg.LLMConfigName,
		"max_iterations":  cfg.MaxIterations,
		"auto_save":       cfg.AutoSave,
		"field_mapping":   cfg.FieldMapping,
	}
}

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
		AutoSave      bool     `json:"auto_save"`
		FieldMapping  string   `json:"field_mapping"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg, err := r.agentCfgUC.Create(c.Request.Context(), agentconfig.CreateInput{
		Name: req.Name, SystemPrompt: req.SystemPrompt,
		LLMConfigName: req.LLMConfigName, MaxIterations: req.MaxIterations,
		AutoSave: req.AutoSave, FieldMapping: req.FieldMapping,
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

// ---- LLM 配置（薄 handler：DTO 转换 + 调用 llmCfgUC）----

func llmConfigToView(cfg entity.LLMConfig) gin.H {
	return gin.H{
		"name":     cfg.Name,
		"provider": cfg.Provider,
		"api_key":  cfg.APIKey,
		"base_url": cfg.BaseURL,
		"model":    cfg.Model,
	}
}

func (r *Router) handleListLLMConfigs(c *gin.Context) {
	cfgs, err := r.llmCfgUC.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(cfgs))
	for _, cfg := range cfgs {
		views = append(views, llmConfigToView(cfg))
	}
	success(c, views)
}

func (r *Router) handleCreateLLMConfig(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Provider string `json:"provider"`
		APIKey   string `json:"api_key" binding:"required"`
		BaseURL  string `json:"base_url"`
		Model    string `json:"model" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	cfg, err := r.llmCfgUC.Create(c.Request.Context(), llmconfig.CreateInput{
		Name: req.Name, Provider: req.Provider, APIKey: req.APIKey,
		BaseURL: req.BaseURL, Model: req.Model,
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

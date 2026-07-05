package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/publish"
)

// PublishHandler 外部推送系统的 HTTP 适配器（薄 handler）。
type PublishHandler struct {
	publishUC *publish.PublishUseCase
	sysCfgUC  *publish.SystemConfigUseCase
}

func NewPublishHandler(puc *publish.PublishUseCase, scuc *publish.SystemConfigUseCase) *PublishHandler {
	return &PublishHandler{publishUC: puc, sysCfgUC: scuc}
}

func externalSystemToView(s entity.ExternalSystem) gin.H {
	mode := s.Mode
	if mode == "" {
		mode = entity.PublishModeRaw
	}
	return gin.H{
		"name":          s.Name,
		"description":   s.Description,
		"endpoint":      s.Endpoint,
		"method":        s.Method,
		"headers":       s.Headers,
		"mode":          mode,
		"field_mapping": s.FieldMapping,
		"body_template": s.BodyTemplate,
		"content_type":  s.ContentType,
		"enabled":       s.Enabled,
		"updated_at":    s.UpdatedAt,
	}
}

// HandleListSystems GET /api/v1/external-systems
func (h *PublishHandler) HandleListSystems(c *gin.Context) {
	list, err := h.sysCfgUC.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(list))
	for _, s := range list {
		views = append(views, externalSystemToView(s))
	}
	success(c, views)
}

// HandleCreateSystem POST /api/v1/external-systems
func (h *PublishHandler) HandleCreateSystem(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		Endpoint     string `json:"endpoint" binding:"required"`
		Method       string `json:"method"`
		Headers      string `json:"headers"`
		Mode         string `json:"mode"`
		FieldMapping string `json:"field_mapping"`
		BodyTemplate string `json:"body_template"`
		ContentType  string `json:"content_type"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	sys, err := h.sysCfgUC.Create(c.Request.Context(), publish.CreateInput{
		Name: req.Name, Description: req.Description, Endpoint: req.Endpoint,
		Method: req.Method, Headers: req.Headers, Mode: req.Mode,
		FieldMapping: req.FieldMapping, BodyTemplate: req.BodyTemplate,
		ContentType: req.ContentType, Enabled: req.Enabled,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, externalSystemToView(sys))
}

// HandleDeleteSystem DELETE /api/v1/external-systems/:name
func (h *PublishHandler) HandleDeleteSystem(c *gin.Context) {
	name := c.Param("name")
	if err := h.sysCfgUC.Delete(c.Request.Context(), name); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": name})
}

// HandlePublish POST /api/v1/external-systems/publish —— 推送一条数据到外部系统
func (h *PublishHandler) HandlePublish(c *gin.Context) {
	var req struct {
		DataItemID string `json:"data_item_id" binding:"required"`
		SystemName string `json:"system_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.publishUC.Publish(c.Request.Context(), publish.PublishInput{
		DataItemID: req.DataItemID, SystemName: req.SystemName,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{
		"success":     out.Success,
		"external_id": out.ExternalID,
		"error":       out.ErrorMsg,
	})
}

// HandlePublishRecords GET /api/v1/data-items/:id/publish-records —— 查询推送记录
func (h *PublishHandler) HandlePublishRecords(c *gin.Context) {
	id := c.Param("id")
	// 用 publishUC 内部组件查询记录
	recs, err := h.sysCfgUC.ListRecords(c.Request.Context(), id, h.publishUC.RecRepo())
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(recs))
	for _, r := range recs {
		views = append(views, gin.H{
			"id": r.ID, "system_name": r.SystemName, "success": r.Success,
			"external_id": r.ExternalID, "error_msg": r.ErrorMsg, "result_at": r.ResultAt,
		})
	}
	success(c, views)
}

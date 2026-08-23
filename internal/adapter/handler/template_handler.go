package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/usecase/generation"
)

// TemplateHandler 模板管理 API。
type TemplateHandler struct {
	uc *generation.TemplateUseCase
}

func NewTemplateHandler(uc *generation.TemplateUseCase) *TemplateHandler {
	return &TemplateHandler{uc: uc}
}

// HandleList GET /api/v1/generation/templates —— 查询租户可用模板。
func (h *TemplateHandler) HandleList(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("模板服务未配置"))
		return
	}

	tenantID := middleware.CurrentTenantID(c)
	templates, err := h.uc.List(c.Request.Context(), tenantID)
	if err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"templates": templates})
}

// HandleGet GET /api/v1/generation/templates/:id —— 查询单个模板。
func (h *TemplateHandler) HandleGet(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("模板服务未配置"))
		return
	}

	template, err := h.uc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}

	success(c, template)
}

// HandleCreate POST /api/v1/admin/templates —— 创建模板（管理后台）。
func (h *TemplateHandler) HandleCreate(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("模板服务未配置"))
		return
	}

	var req generation.CreateTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	template, err := h.uc.Create(c.Request.Context(), req)
	if err != nil {
		fail(c, err)
		return
	}

	success(c, template)
}

// HandleUpdate PUT /api/v1/admin/templates/:id —— 更新模板（管理后台）。
func (h *TemplateHandler) HandleUpdate(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("模板服务未配置"))
		return
	}

	var req generation.UpdateTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}

	template, err := h.uc.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		fail(c, err)
		return
	}

	success(c, template)
}

// HandleDelete DELETE /api/v1/admin/templates/:id —— 删除模板（管理后台）。
func (h *TemplateHandler) HandleDelete(c *gin.Context) {
	if h.uc == nil {
		fail(c, fmt.Errorf("模板服务未配置"))
		return
	}

	if err := h.uc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, err)
		return
	}

	success(c, gin.H{"deleted": c.Param("id")})
}

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// ---- 经济系统 handler（admin + 商户端）----
//
// admin 端：套餐 CRUD / 全部订阅 / 全部订单（旁路视角，无租户过滤）
// 商户端：在售套餐 / 我的套餐 / 我的订单（多租户隔离）

// ---- admin 端点 ----

// HandleAdminListPlans GET /admin/billing/plans —— 全部套餐（含下架）。
func (r *Router) HandleAdminListPlans(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	plans, err := r.billingUC.ListPlans(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"plans": plans})
}

// HandleAdminSavePlan POST /admin/billing/plans —— 创建或更新套餐。
func (r *Router) HandleAdminSavePlan(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	var p entity.Plan
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数解析失败: " + err.Error()})
		return
	}
	saved, err := r.billingUC.SavePlan(c.Request.Context(), p)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, saved)
}

// HandleAdminDeletePlan DELETE /admin/billing/plans/:id —— 下架/删除套餐。
func (r *Router) HandleAdminDeletePlan(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	if err := r.billingUC.DeletePlan(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"id": c.Param("id")})
}

// HandleAdminListSubscriptions GET /admin/billing/subscriptions —— 全部订阅（全局视角）。
func (r *Router) HandleAdminListSubscriptions(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	subs, err := r.billingUC.ListSubscriptions(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"subscriptions": subs})
}

// HandleAdminListOrders GET /admin/billing/orders —— 全部订单流水（收入报表用）。
func (r *Router) HandleAdminListOrders(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	orders, err := r.billingUC.ListAllOrders(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"orders": orders})
}

// ---- 商户端端点 ----

// HandleListActivePlans GET /billing/plans —— 在售套餐列表（购买页用）。
func (r *Router) HandleListActivePlans(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	plans, err := r.billingUC.ListActivePlans(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"plans": plans})
}

// HandleGetMyPlan GET /billing/my-plan —— 我的订阅（无订阅降级提示购买）。
func (r *Router) HandleGetMyPlan(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	if tenantID == "" {
		fail(c, pkg.ErrInvalidArgument)
		return
	}
	sub, err := r.billingUC.GetSubscription(c.Request.Context(), tenantID)
	if err != nil {
		// 无订阅——返回降级标记（前端引导购买，不报错）
		success(c, gin.H{"subscription": nil, "hint": "未开通套餐，默认免费额度"})
		return
	}
	success(c, gin.H{"subscription": sub})
}

// HandleListMyOrders GET /billing/orders —— 我的订单流水。
func (r *Router) HandleListMyOrders(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	if tenantID == "" {
		fail(c, pkg.ErrInvalidArgument)
		return
	}
	orders, err := r.billingUC.ListOrdersByTenant(c.Request.Context(), tenantID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"orders": orders})
}

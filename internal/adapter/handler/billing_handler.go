package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"webreaper/internal/adapter/handler/middleware"
	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/billing"
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

// ---- 商户端下单 + 确认 ----

// HandleCreateOrder POST /billing/orders —— 商户下单购买套餐。
// 创建 pending 订单，返回支付页 URL（前端跳转拉起支付）。
func (r *Router) HandleCreateOrder(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	var req struct {
		PlanID string `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan_id 必填"})
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	if tenantID == "" {
		fail(c, pkg.ErrInvalidArgument)
		return
	}
	result, err := r.billingUC.CreateOrder(c.Request.Context(), billing.CreateOrderInput{
		TenantID: tenantID, PlanID: req.PlanID,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"order": result.Order, "payment_url": result.PaymentURL})
}

// HandleConfirmOrder POST /billing/orders/:id/confirm —— 确认支付完成（mock 自动确认 / 真实回调）。
func (r *Router) HandleConfirmOrder(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	sub, err := r.billingUC.ConfirmPayment(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"subscription": sub})
}

// ---- admin 手动开通 ----

// HandleAdminAssignPlan PUT /admin/billing/subscriptions/:tenant —— 手动给租户开通套餐（线下收款）。
func (r *Router) HandleAdminAssignPlan(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	var req struct {
		PlanID string `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan_id 必填"})
		return
	}
	sub, err := r.billingUC.AssignPlan(c.Request.Context(), c.Param("tenant"), req.PlanID)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"subscription": sub})
}

// ---- admin 收入报表 + 商户用量 ----

// HandleAdminRevenueReport GET /admin/billing/revenue —— 收入概览（仪表盘用）。
func (r *Router) HandleAdminRevenueReport(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	summary, err := r.billingUC.RevenueReport(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, summary)
}

// HandleGetMyUsage GET /billing/usage —— 我的用量（配额余量，商户端进度条用）。
func (r *Router) HandleGetMyUsage(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	if tenantID == "" {
		fail(c, pkg.ErrInvalidArgument)
		return
	}
	summary, err := r.billingUC.GetMyUsage(c.Request.Context(), tenantID, r.quotaGate)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, summary)
}

// ---- 支付网关异步回调（webhook）----

// HandlePaymentCallback POST/GET /billing/webhook/:gateway —— 支付网关异步回调。
// ZPAY 用 GET 参数回调；验签通过后标记 paid + 开通订阅。
// 必须返回纯文本 "success"（ZPAY 协议要求），否则平台会重试。
// 注意：此端点不需要 JWT 认证（支付平台回调无 token）。
func (r *Router) HandlePaymentCallback(c *gin.Context) {
	if r.billingUC == nil {
		c.String(http.StatusOK, "fail")
		return
	}
	// ZPAY 回调用 GET query 参数；其他通道可能是 POST body
	params := make(map[string]string)
	// 优先读 GET query（ZPAY 协议）
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	// 兼容 POST form（部分通道用 POST）
	if c.Request.Method == http.MethodPost {
		_ = c.Request.ParseForm()
		for k, v := range c.Request.PostForm {
			if len(v) > 0 && params[k] == "" {
				params[k] = v[0]
			}
		}
	}
	result, err := r.billingUC.HandleCallback(c.Request.Context(), params)
	if err != nil {
		c.String(http.StatusOK, "fail")
		return
	}
	c.String(http.StatusOK, result) // "success" 或 "fail"
}

// ---- admin 支付网关配置管理 ----

// HandleGetPaymentConfig GET /admin/billing/payment-config —— 读取当前支付配置。
// 返回脱敏后的配置（key 只显示前 4 位）。
func (r *Router) HandleGetPaymentConfig(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	cfg, err := r.billingUC.GetPaymentConfig(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"config": cfg})
}

// HandleSetPaymentConfig PUT /admin/billing/payment-config —— 保存支付网关配置。
// 配置存入 system_settings，运行时生效（重启不丢失）。
// body: {"gateway":"zpay","pid":"xxx","key":"xxx","notify_url":"xxx","return_url":"xxx"}
func (r *Router) HandleSetPaymentConfig(c *gin.Context) {
	if r.billingUC == nil {
		fail(c, errNotConfigured("计费"))
		return
	}
	var req struct {
		Gateway   string `json:"gateway"`
		PID       string `json:"pid"`
		Key       string `json:"key"`
		NotifyURL string `json:"notify_url"`
		ReturnURL string `json:"return_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数解析失败: " + err.Error()})
		return
	}
	cfg := map[string]string{
		"gateway": req.Gateway, "pid": req.PID, "key": req.Key,
		"notify_url": req.NotifyURL, "return_url": req.ReturnURL,
	}
	if err := r.billingUC.SetPaymentConfig(c.Request.Context(), cfg); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"saved": true})
}

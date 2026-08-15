package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/port"
)

// UserHandler 是用户管理（管理端）的 HTTP 适配器。
// 仅 admin 角色可访问（路由层用 middleware.RequireRole("admin") 守卫）。
type UserHandler struct {
	register *auth.RegisterUseCase
	userRepo port.UserRepository
	// F3-1 运营聚合注入（可选；Router 从既有用例传入闭包——handler 不新增仓储依赖）：
	// 品牌数（租户品牌资产）与最近活跃（最近一次监测时间——GEO 产品的真实使用信号）。
	brandCountByTenant func(ctx context.Context, tenantID string) int
	lastActiveByTenant func(ctx context.Context, tenantID string) (time.Time, bool)
}

func NewUserHandler(register *auth.RegisterUseCase, userRepo port.UserRepository) *UserHandler {
	return &UserHandler{register: register, userRepo: userRepo}
}

// SetUsageEnrichment 注入运营聚合闭包（可选；未注入则列表只含基础字段）。
func (h *UserHandler) SetUsageEnrichment(brandCount func(ctx context.Context, tenantID string) int, lastActive func(ctx context.Context, tenantID string) (time.Time, bool)) {
	h.brandCountByTenant = brandCount
	h.lastActiveByTenant = lastActive
}

// userView 用户列表项（不返回密码哈希）。F3-1：附品牌数与最近活跃——
// 管理员据此判断"谁在正常使用、谁有流失风险"。
type userView struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	TenantID   string `json:"tenant_id"`
	BrandCount int    `json:"brand_count"`
	LastActive string `json:"last_active"` // 最近一次监测时间（空=从未使用——沉睡商户信号）
}

// HandleListUsers GET /api/v1/admin/users —— 列出全部用户（仅 admin）。
func (h *UserHandler) HandleListUsers(c *gin.Context) {
	users, err := h.userRepo.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ctx := c.Request.Context()
	views := make([]userView, 0, len(users))
	for _, u := range users {
		v := userView{ID: u.ID, Username: u.Username, Role: u.Role, TenantID: u.TenantID}
		if h.brandCountByTenant != nil {
			v.BrandCount = h.brandCountByTenant(ctx, u.TenantID)
		}
		if h.lastActiveByTenant != nil {
			if t, ok := h.lastActiveByTenant(ctx, u.TenantID); ok {
				v.LastActive = t.Format("2006-01-02 15:04")
			}
		}
		views = append(views, v)
	}
	success(c, views)
}

// HandleCreateMerchant POST /api/v1/admin/users —— 管理员创建商户账号。
func (h *UserHandler) HandleCreateMerchant(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		TenantID string `json:"tenant_id"` // 可选：指定归属租户
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.register.Execute(c.Request.Context(), auth.RegisterInput{
		Username: req.Username, Password: req.Password,
		Role: "merchant", TenantID: req.TenantID,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"user_id": out.UserID, "role": out.Role, "tenant_id": out.TenantID})
}

// HandleDeleteUser DELETE /api/v1/admin/users/:id —— 删除用户。
func (h *UserHandler) HandleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"deleted": id})
}

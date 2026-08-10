package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/usecase/auth"
	"webreaper/internal/usecase/port"
)

// UserHandler 是用户管理（管理端）的 HTTP 适配器。
// 仅 admin 角色可访问（路由层用 middleware.RequireRole("admin") 守卫）。
type UserHandler struct {
	register *auth.RegisterUseCase
	userRepo port.UserRepository
}

func NewUserHandler(register *auth.RegisterUseCase, userRepo port.UserRepository) *UserHandler {
	return &UserHandler{register: register, userRepo: userRepo}
}

// userView 用户列表项（不返回密码哈希）。
type userView struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

// HandleListUsers GET /api/v1/admin/users —— 列出全部用户（仅 admin）。
func (h *UserHandler) HandleListUsers(c *gin.Context) {
	users, err := h.userRepo.List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]userView, 0, len(users))
	for _, u := range users {
		views = append(views, userView{ID: u.ID, Username: u.Username, Role: u.Role, TenantID: u.TenantID})
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

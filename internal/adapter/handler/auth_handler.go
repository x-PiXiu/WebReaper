package handler

import (
	"github.com/gin-gonic/gin"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/auth"
)

// AuthHandler 是认证用例的 HTTP 适配器（注册/登录）。
type AuthHandler struct {
	register *auth.RegisterUseCase
	login    *auth.LoginUseCase
}

func NewAuthHandler(register *auth.RegisterUseCase, login *auth.LoginUseCase) *AuthHandler {
	return &AuthHandler{register: register, login: login}
}

// RegisterRequest POST /api/v1/auth/register
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`      // 可选：admin / merchant，留空默认 merchant
	TenantID string `json:"tenant_id"` // 可选：指定租户（管理员创建商户时用）
}

// HandleRegister POST /api/v1/auth/register
func (h *AuthHandler) HandleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.register.Execute(c.Request.Context(), auth.RegisterInput{
		Username: req.Username, Password: req.Password,
		Role: req.Role, TenantID: req.TenantID,
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"user_id": out.UserID, "role": out.Role, "tenant_id": out.TenantID})
}

// LoginRequest POST /api/v1/auth/login
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// HandleLogin POST /api/v1/auth/login
func (h *AuthHandler) HandleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	out, err := h.login.Execute(c.Request.Context(), auth.LoginInput{
		Username: req.Username, Password: req.Password,
	})
	if err != nil {
		fail(c, err)
		return
	}
	// 登录返回 token + 用户身份（前端据此分流到商户端/管理端）
	success(c, gin.H{
		"token":     out.Token,
		"role":      out.Role,
		"tenant_id": out.TenantID,
		"username":  out.Username,
	})
}

// 用户管理（管理端）handler —— 列出/删除用户
// 这部分放在 user_handler.go，这里只保留认证入口。

// roleFromContext 从 gin.Context 取当前用户角色（供 handler 判断权限）。
func roleFromContext(c *gin.Context) string {
	if v, ok := c.Get("role"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// _ 确保 entity 包被引用（角色常量在 entity 定义）
var _ = entity.RoleAdmin

package handler

import (
	"github.com/gin-gonic/gin"

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
	})
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"user_id": out.UserID})
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
	success(c, gin.H{"token": out.Token})
}

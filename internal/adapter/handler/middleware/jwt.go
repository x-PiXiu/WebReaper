// Package middleware 提供 HTTP 中间件（接口适配器层）。
//
// 认证是传输层关注点，不进 domain/usecase。JWT 验证逻辑封装在此。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	authadapter "webreaper/internal/adapter/auth"
)

// JWTAuth 返回一个 JWT 认证中间件。
//
// 行为：
//   - tokenGenerator 为 nil 时（未启用认证），中间件直接放行（开发环境友好）
//   - 从 Authorization: Bearer <token> 提取 token
//   - 验证通过后，把 user_id/username 注入 gin.Context 供后续 handler 使用
//   - 验证失败返回 401
func JWTAuth(tokenParser *authadapter.JWTGenerator) gin.HandlerFunc {
	// 未启用认证（secret 为空），中间件放行
	if tokenParser == nil {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "missing authorization header"})
			return
		}

		// 期望格式 "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40101, "msg": "invalid authorization format"})
			return
		}

		claims, err := tokenParser.ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40102, "msg": "invalid or expired token"})
			return
		}

		// 注入用户信息供后续 handler 使用
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

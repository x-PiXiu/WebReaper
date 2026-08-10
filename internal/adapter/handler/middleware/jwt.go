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
//   - 验证通过后，把 user_id/username/role/tenant_id 注入 gin.Context 供后续 handler 使用
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

		// 租户隔离铁律：token 必须携带租户归属（租户隔离升级前签发的旧 token
		// 无 tenant_id，空租户在仓储层 = 不过滤 = 越权看全局）。
		// 空租户直接 401，强制客户端重新登录（登录时会兜底分配租户）。
		if claims.TenantID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40103, "msg": "登录态已过期（租户隔离升级），请重新登录"})
			return
		}

		// 注入用户身份信息供后续 handler 使用
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Next()
	}
}

// RequireRole 返回一个角色校验中间件（RBAC）。
// 仅允许指定角色通过，否则返回 403。
// 用法：api.Group("/admin").Use(middleware.RequireRole("admin"))
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		// 未启用认证时（role 为空），放行（开发环境友好）
		if roleStr == "" {
			c.Next()
			return
		}
		if !allowed[roleStr] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 40300, "msg": "权限不足"})
			return
		}
		c.Next()
	}
}

// CurrentTenantID 从 gin.Context 取当前请求的租户 ID。
// 已由 JWTAuth 中间件保证非空（租户隔离铁律：空租户 401）——
// 商户与 admin 在各自上下文都携带明确租户，仓储层据此隔离。
func CurrentTenantID(c *gin.Context) string {
	v, ok := c.Get("tenant_id")
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// CurrentRole 从 gin.Context 取当前请求的角色。
func CurrentRole(c *gin.Context) string {
	v, ok := c.Get("role")
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

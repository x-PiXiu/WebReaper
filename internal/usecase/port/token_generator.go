package port

// TokenClaims 是令牌内的声明（用例层 DTO，与 JWT 实现解耦）。
// 用例层把用户身份信息打包成此结构传给 TokenGenerator，不关心 JWT 细节。
type TokenClaims struct {
	UserID   string
	Username string
	Role     string // admin / merchant
	TenantID string // 归属租户（admin 可空）
}

// TokenGenerator 认证令牌生成接口（边界）。
//
// 依赖倒置：登录用例调此接口生成 token，不关心底层是 JWT 还是其他方案。
// JWT 的签发逻辑（密钥/过期时间）在适配器层实现。
type TokenGenerator interface {
	// Generate 为指定用户生成认证令牌。
	// claims 携带用户身份（id/username/role/tenant_id），全部写入令牌供中间件解析。
	Generate(claims TokenClaims) (string, error)
}

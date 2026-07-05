package port

// TokenGenerator 认证令牌生成接口（边界）。
//
// 依赖倒置：登录用例调此接口生成 token，不关心底层是 JWT 还是其他方案。
// JWT 的签发逻辑（密钥/过期时间）在适配器层实现。
type TokenGenerator interface {
	// Generate 为指定用户生成认证令牌。
	// userID 是用户唯一标识，username 用于令牌内的声明。
	Generate(userID string, username string) (string, error)
}

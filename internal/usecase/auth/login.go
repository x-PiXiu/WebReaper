package auth

import (
	"context"
	"errors"
	"fmt"

	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- 登录用例 ----

// LoginInput 登录输入。
type LoginInput struct {
	Username string
	Password string // 明文密码
}

// LoginOutput 登录输出。
type LoginOutput struct {
	Token    string // 认证令牌（JWT 等，由 TokenGenerator 生成）
	Role     string // 用户角色（前端据此渲染不同界面）
	TenantID string // 归属租户（前端/后续请求据此隔离数据）
	Username string
}

// LoginUseCase 用户登录用例。
type LoginUseCase struct {
	repo     port.UserRepository
	hasher   port.PasswordHasher
	tokenGen port.TokenGenerator
}

func NewLoginUseCase(repo port.UserRepository, hasher port.PasswordHasher, tokenGen port.TokenGenerator) *LoginUseCase {
	return &LoginUseCase{repo: repo, hasher: hasher, tokenGen: tokenGen}
}

// Execute 执行登录：查用户 → 验密 → 发 token。
// 用户不存在或密码错误都返回统一的"用户名或密码错误"（避免枚举用户）。
func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (LoginOutput, error) {
	// 1. 查用户
	user, err := uc.repo.FindByUsername(ctx, in.Username)
	if err != nil {
		// 用户不存在统一归为"无效凭据"，不暴露具体原因
		if errors.Is(err, pkg.ErrNotFound) {
			return LoginOutput{}, fmt.Errorf("%w: invalid username or password", pkg.ErrInvalidArgument)
		}
		return LoginOutput{}, fmt.Errorf("find user: %w", err)
	}

	// 2. 验证密码
	if err := uc.hasher.Compare(user.PasswordHash, in.Password); err != nil {
		return LoginOutput{}, fmt.Errorf("%w: invalid username or password", pkg.ErrInvalidArgument)
	}

	// 2.5 租户兜底（隔离铁律）：存量账号（租户隔离改造前注册）可能无 tenant_id。
	// 任何账号登录后必须归属一个租户——否则在用户界面会因"空租户=看全局"越权。
	if user.TenantID == "" {
		user.TenantID = pkg.TenantID()
		if err := uc.repo.Save(ctx, user); err != nil {
			return LoginOutput{}, fmt.Errorf("assign tenant: %w", err)
		}
	}

	// 3. 生成 token（把 role/tenant_id 一并写入令牌）
	token, err := uc.tokenGen.Generate(port.TokenClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		TenantID: user.TenantID,
	})
	if err != nil {
		return LoginOutput{}, fmt.Errorf("generate token: %w", err)
	}

	return LoginOutput{
		Token:    token,
		Role:     user.Role,
		TenantID: user.TenantID,
		Username: user.Username,
	}, nil
}

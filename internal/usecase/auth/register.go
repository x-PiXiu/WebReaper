// Package auth 实现"用户注册与登录"用例。
//
// 这是用例层：编排"校验 → 哈希密码 → 存库"（注册）和
// "查用户 → 验密 → 发 token"（登录）流程，
// 只依赖 domain 实体和 port 接口，不依赖 bcrypt/jwt/gorm。
package auth

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- 注册用例 ----

// RegisterInput 注册输入。
type RegisterInput struct {
	Username string
	Password string // 明文密码
}

// RegisterOutput 注册输出。
type RegisterOutput struct {
	UserID string
}

// RegisterUseCase 用户注册用例。
type RegisterUseCase struct {
	repo   port.UserRepository
	hasher port.PasswordHasher
}

func NewRegisterUseCase(repo port.UserRepository, hasher port.PasswordHasher) *RegisterUseCase {
	return &RegisterUseCase{repo: repo, hasher: hasher}
}

// Execute 执行注册：校验用户名/密码长度 → 检查重名 → 哈希 → 存库。
func (uc *RegisterUseCase) Execute(ctx context.Context, in RegisterInput) (RegisterOutput, error) {
	// 1. 领域规则校验
	if !entity.IsValidUsername(in.Username) {
		return RegisterOutput{}, fmt.Errorf("%w: username must be at least %d chars", pkg.ErrInvalidArgument, entity.MinUsernameLength)
	}
	if !entity.IsValidPassword(in.Password) {
		return RegisterOutput{}, fmt.Errorf("%w: password must be at least %d chars", pkg.ErrInvalidArgument, entity.MinPasswordLength)
	}

	// 2. 检查用户名是否已存在
	if existing, err := uc.repo.FindByUsername(ctx, in.Username); err == nil && existing.ID != "" {
		return RegisterOutput{}, fmt.Errorf("%w: username %q already exists", pkg.ErrAlreadyExists, in.Username)
	}

	// 3. 哈希密码（用例不接触 bcrypt，通过 port 接口）
	hash, err := uc.hasher.Hash(in.Password)
	if err != nil {
		return RegisterOutput{}, fmt.Errorf("hash password: %w", err)
	}

	// 4. 构造用户并存库
	now := time.Now()
	user := entity.User{
		ID:           fmt.Sprintf("user-%d", now.UnixNano()),
		Username:     in.Username,
		PasswordHash: hash,
		CreatedAt:    now,
	}
	if err := uc.repo.Save(ctx, user); err != nil {
		return RegisterOutput{}, fmt.Errorf("save user: %w", err)
	}

	return RegisterOutput{UserID: user.ID}, nil
}

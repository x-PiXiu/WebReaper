package auth

import (
	"context"
	"fmt"

	"webreaper/internal/usecase/port"
)

// ChangePasswordUseCase 修改当前登录用户密码（F1-5：默认弱口令 admin/admin123 的治理闭环）。
// 旧密码验证通过才允许更新——防止会话被借用后改密。
type ChangePasswordUseCase struct {
	userRepo port.UserRepository
	hasher   port.PasswordHasher
}

func NewChangePasswordUseCase(r port.UserRepository, h port.PasswordHasher) *ChangePasswordUseCase {
	return &ChangePasswordUseCase{userRepo: r, hasher: h}
}

// Execute 校验旧密码并更新为新密码。newPassword 长度 ≥8。
func (uc *ChangePasswordUseCase) Execute(ctx context.Context, userID, oldPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("新密码至少 8 位")
	}
	u, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	if err := uc.hasher.Compare(u.PasswordHash, oldPassword); err != nil {
		return fmt.Errorf("旧密码不正确")
	}
	hash, err := uc.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}
	u.PasswordHash = hash
	return uc.userRepo.Save(ctx, u)
}

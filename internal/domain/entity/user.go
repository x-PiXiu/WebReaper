package entity

import "time"

// 用户角色（多租户 SaaS 权限模型）。
const (
	// RoleAdmin 平台管理员：可管理所有租户、全局配置、收费、封号。
	RoleAdmin = "admin"
	// RoleMerchant 商户：只能管理自己租户内的品牌、关键词、监测、账号。
	RoleMerchant = "merchant"
)

// User 表示一个系统用户（用于认证）。
// 这是实体（Entity）：有唯一标识 ID，密码哈希而非明文（领域规则）。
//
// 多租户设计：
//   - 每个用户归属一个 TenantID（商户隔离的根）。
//   - Role 决定权限：admin 看全局，merchant 只看自己的 tenant_id。
//   - admin 的 TenantID 可为空（平台级账号，不归属任何商户）。
//
// 整洁架构要求：本文件不 import 任何框架（无 bcrypt/jwt/gorm）。
// 密码哈希/校验由 port.PasswordHasher 接口在用例层调用，实体只存哈希值。
type User struct {
	ID           string
	Username     string
	PasswordHash string // 已哈希的密码，绝不存明文
	Role         string // 角色：admin / merchant
	TenantID     string // 归属租户 ID（admin 可为空）
	CreatedAt    time.Time
}

// IsValid 领域规则：有效的用户必须有用户名和密码哈希。
func (u User) IsValid() bool {
	return u.Username != "" && u.PasswordHash != ""
}

// IsAdmin 是否为管理员。
func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsMerchant 是否为商户。
func (u User) IsMerchant() bool {
	return u.Role == RoleMerchant
}

// EffectiveTenantID 返回查询数据时应使用的 tenant_id。
// admin 不带 tenant_id 过滤（看全局），merchant 必须用自己的。
// 用 "admin" 作为 admin 的虚拟租户标识，仓储层据此跳过过滤。
func (u User) EffectiveTenantID() string {
	if u.IsAdmin() {
		return ""
	}
	return u.TenantID
}

// MinUsernameLength 用户名最小长度（领域规则）。
const MinUsernameLength = 3

// MinPasswordLength 密码最小长度（领域规则，针对原始密码而非哈希）。
const MinPasswordLength = 6

// IsValidUsername 检查用户名是否符合长度规则。
func IsValidUsername(username string) bool {
	return len(username) >= MinUsernameLength
}

// IsValidPassword 检查原始密码是否符合长度规则。
func IsValidPassword(password string) bool {
	return len(password) >= MinPasswordLength
}

// IsValidRole 检查角色是否合法。
func IsValidRole(role string) bool {
	return role == RoleAdmin || role == RoleMerchant
}

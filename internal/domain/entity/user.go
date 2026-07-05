package entity

import "time"

// User 表示一个系统用户（用于认证）。
// 这是实体（Entity）：有唯一标识 ID，密码哈希而非明文（领域规则）。
//
// 整洁架构要求：本文件不 import 任何框架（无 bcrypt/jwt/gorm）。
// 密码哈希/校验由 port.PasswordHasher 接口在用例层调用，实体只存哈希值。
type User struct {
	ID           string
	Username     string
	PasswordHash string // 已哈希的密码，绝不存明文
	CreatedAt    time.Time
}

// IsValid 领域规则：有效的用户必须有用户名和密码哈希。
func (u User) IsValid() bool {
	return u.Username != "" && u.PasswordHash != ""
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

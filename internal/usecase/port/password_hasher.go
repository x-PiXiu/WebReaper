package port

// PasswordHasher 密码哈希接口（边界）。
//
// 依赖倒置：用例层只依赖此接口，bcrypt 实现在适配器层。
// 这样换哈希算法（bcrypt→argon2）时用例层零修改。
type PasswordHasher interface {
	// Hash 把明文密码转为哈希。
	Hash(password string) (string, error)
	// Compare 校验明文密码是否匹配哈希。匹配返回 nil，不匹配返回错误。
	Compare(hash string, password string) error
}

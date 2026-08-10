package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"webreaper/internal/usecase/port"
)

// AESCookieVault 用 AES-GCM 加密/解密 cookie（敏感数据隔离）。
//
// 设计动机（整洁架构 / 依赖倒置）：
//   - 加密是基础设施细节，用 port.CookieVault 接口和用例层解耦。
//   - 后续可升级为 KMS 管理主密钥 + 租户级派生密钥，接口不变。
//
// 安全要点：
//   - AES-256-GCM（密钥经 SHA-256 派生为 32 字节）。
//   - 每次加密用随机 nonce，nonce 拼在密文前（解密时切分）。
//   - 输出 base64 便于存 TEXT 列。
type AESCookieVault struct {
	gcm cipher.AEAD
}

var _ port.CookieVault = (*AESCookieVault)(nil)

// NewAESCookieVault 用配置的密钥构造加密器。
// secret 长度不限（经 SHA-256 派生为 AES-256 密钥）；为空时返回错误。
func NewAESCookieVault(secret string) (*AESCookieVault, error) {
	if secret == "" {
		return nil, errors.New("cookie encryption secret is empty (set PUBLISH_COOKIE_SECRET)")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESCookieVault{gcm: gcm}, nil
}

// Encrypt 加密 cookie 明文，返回 base64 编码的「nonce + 密文」。
func (v *AESCookieVault) Encrypt(cookie string) (string, error) {
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// Seal 把 nonce 拼在密文开头
	encrypted := v.gcm.Seal(nonce, nonce, []byte(cookie), nil)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// Decrypt 解密 base64 编码的密文，返回 cookie 明文。
func (v *AESCookieVault) Decrypt(encCookie string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encCookie)
	if err != nil {
		return "", err
	}
	nonceSize := v.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("encrypted cookie too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := v.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

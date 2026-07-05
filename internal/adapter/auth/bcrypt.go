// Package auth 提供 port.PasswordHasher / port.TokenGenerator 的实现。
//
// 整洁架构定位：本层把密码哈希（bcrypt）和令牌生成（JWT）的细节封装起来，
// 用例层只依赖 port 接口，对本文件的具体实现一无所知。
//
// 依赖方向：auth → bcrypt/jwt + port（向内）。
// bcrypt/jwt 只出现在本目录，domain/usecase 对其一无所知。
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"webreaper/internal/usecase/port"
)

// BcryptHasher 是 port.PasswordHasher 的 bcrypt 实现。
type BcryptHasher struct{}

func NewBcryptHasher() *BcryptHasher { return &BcryptHasher{} }

func (BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(b), nil
}

func (BcryptHasher) Compare(hash string, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("bcrypt compare: %w", err)
	}
	return nil
}

// JWTClaims 是 JWT 的自定义声明（payload）。
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTGenerator 是 port.TokenGenerator 的 JWT 实现。
type JWTGenerator struct {
	secret     []byte
	expiration time.Duration
}

// NewJWTGenerator 创建 JWT 令牌生成器。
// secret 是签名密钥，expiration 是过期时长（秒）。
func NewJWTGenerator(secret string, expirationSec int) *JWTGenerator {
	if expirationSec <= 0 {
		expirationSec = 3600 // 默认 1 小时
	}
	return &JWTGenerator{
		secret:     []byte(secret),
		expiration: time.Duration(expirationSec) * time.Second,
	}
}

func (g *JWTGenerator) Generate(userID string, username string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(g.expiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(g.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

// ParseToken 解析并验证 JWT 令牌。供中间件使用。
// 密钥为空时返回错误（表示未启用认证）。
func (g *JWTGenerator) ParseToken(tokenString string) (*JWTClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return g.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}
	claims, ok := tok.Claims.(*JWTClaims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// 编译期断言：确保实现 port 接口。
var _ port.PasswordHasher = (*BcryptHasher)(nil)
var _ port.TokenGenerator = (*JWTGenerator)(nil)

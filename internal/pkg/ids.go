package pkg

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ShortID 生成 n 字节的十六进制随机 ID（n=6 → 12 字符，如 32afa9cca14b）。
// 用于文件名、用户 ID、租户 ID 等需要简短唯一标识的场景。
func ShortID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%0*x", n*2, time.Now().UnixNano())[:n*2]
	}
	return hex.EncodeToString(b)
}

// UserID 生成用户 ID（u_ + 12 hex = 14 字符，如 u_32afa9cca14b）
func UserID() string { return "u_" + ShortID(6) }

// TenantID 生成租户 ID（t_ + 12 hex = 14 字符，如 t_a1b2c3d4e5f6）
func TenantID() string { return "t_" + ShortID(6) }

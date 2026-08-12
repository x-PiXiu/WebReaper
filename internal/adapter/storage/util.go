package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// shortID 生成 12 位十六进制随机 ID（如 32afa9cca14b）。
// 6 字节随机 = 48 位熵，碰撞概率极低（281 万亿空间）。
// 用于素材文件名 / 产物文件名，替代长纳秒时间戳。
func shortID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// fallback：时间戳 hex（极小概率 rand 失败）
		return fmt.Sprintf("%012x", time.Now().UnixNano())[:12]
	}
	return hex.EncodeToString(b)
}

package storage

import (
	"time"

	"webreaper/internal/pkg"
)

// shortID 生成 12 位十六进制随机 ID（委托 pkg.ShortID）。
// 用于素材文件名 / 产物文件名。
func shortID() string { return pkg.ShortID(6) }

// datePath 返回日期目录（如 2026-08-13），用于存储路径分组。
func datePath() string { return time.Now().Format("2006-01-02") }

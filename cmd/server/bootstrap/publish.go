// publish.go 发布域初始化辅助（从 main.go 迁移——27号优化 main.go 瘦身）。
//
// 提供发布域初始化的辅助函数，main.go 仍保留主装配逻辑。
package bootstrap

import (
	"context"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/adapter/crypto"
	"webreaper/internal/config"
	"webreaper/internal/usecase/port"
)

// PublishDeps 发布域依赖集合（main.go 装配后传入）。
type PublishDeps struct {
	DB           *gorm.DB
	Config       *config.Config
	MonitorUC    port.MonitorTrigger
	PublicBaseURL string
}

// InitCookieVault 初始化 Cookie 加密保险库。
func InitCookieVault(secret string, logger port.Logger) port.CookieVault {
	if secret == "" {
		logger.Warn("PUBLISH_COOKIE_SECRET 未配置，扫码登录不可用（cookie 无法加密存储）")
		return nil
	}
	v, err := crypto.NewAESCookieVault(secret)
	if err != nil {
		logger.Error("cookie 加密保险库初始化失败，扫码登录将不可用", port.Err(err))
		return nil
	}
	return v
}

// ReapStaleJobs 启动时清扫僵尸发布任务（running 超过 30 分钟）。
func ReapStaleJobs(jobRepo port.PublishJobRepository, logger port.Logger) {
	if n, rErr := jobRepo.ReapStaleJobs(context.Background(), 30*time.Minute); rErr == nil && n > 0 {
		logger.Info("启动清扫僵尸发布任务", port.Int("count", int(n)))
	}
}

// InitAccountReposDB 初始化发布账号域仓储。
func InitAccountReposDB(db *gorm.DB) *AccountRepos {
	return InitAccountRepos(db)
}

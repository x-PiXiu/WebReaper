package repository

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// GormAccountPool 账号池调度实现（最久未使用优先策略）。
// 单账号高频发布必封，轮换使用避免风控。
type GormAccountPool struct {
	repo port.AccountRepository
}

var _ port.AccountPool = (*GormAccountPool)(nil)

func NewGormAccountPool(repo port.AccountRepository) *GormAccountPool {
	return &GormAccountPool{repo: repo}
}

// Acquire 借出最久未使用的健康账号。
// 策略：按 platform 过滤 active 账号 → 按 LastUsedAt 升序（最久未使用优先）→ 取第一个。
func (p *GormAccountPool) Acquire(ctx context.Context, tenantID, platform string) (entity.Account, error) {
	accounts, err := p.repo.ListByPlatform(ctx, tenantID, platform)
	if err != nil {
		return entity.Account{}, fmt.Errorf("查询账号失败: %w", err)
	}

	// 过滤健康账号
	var healthy []entity.Account
	for _, a := range accounts {
		if a.Health == entity.AccountHealthActive {
			healthy = append(healthy, a)
		}
	}
	if len(healthy) == 0 {
		return entity.Account{}, fmt.Errorf("没有可用的健康 %s 账号", platform)
	}

	// 最久未使用优先（LastUsedAt 最早的最先选）
	selected := healthy[0]
	for _, a := range healthy[1:] {
		if a.LastUsedAt.Before(selected.LastUsedAt) {
			selected = a
		}
	}
	return selected, nil
}

// Release 归还账号，更新 LastUsedAt。
func (p *GormAccountPool) Release(ctx context.Context, account entity.Account) error {
	return p.repo.UpdateLastUsed(ctx, account.ID, time.Now())
}

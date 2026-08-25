package account

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// BrandAccountSelector 品牌账号选择器
// 优先从品牌绑定的账号中选择，无绑定时回退到全局账号池
type BrandAccountSelector struct {
	bindingRepo port.AccountBrandBindingRepository
	accountRepo port.AccountRepository
	pool        port.AccountPool
}

// NewBrandAccountSelector 创建品牌账号选择器
func NewBrandAccountSelector(bindingRepo port.AccountBrandBindingRepository, accountRepo port.AccountRepository, pool port.AccountPool) *BrandAccountSelector {
	return &BrandAccountSelector{
		bindingRepo: bindingRepo,
		accountRepo: accountRepo,
		pool:        pool,
	}
}

// SelectAccount 为品牌选择可用账号
func (s *BrandAccountSelector) SelectAccount(ctx context.Context, tenantID, brandID, platform string) (entity.Account, error) {
	// 1. 获取品牌绑定的账号
	bindings, err := s.bindingRepo.FindByBrand(ctx, tenantID, brandID)
	if err != nil {
		return entity.Account{}, err
	}

	// 2. 过滤出指定平台的账号
	var accountIDs []string
	for _, b := range bindings {
		if b.Platform == platform {
			accountIDs = append(accountIDs, b.AccountID)
		}
	}

	// 3. 如果有绑定账号，从绑定账号中选择
	if len(accountIDs) > 0 {
		// 获取账号详情（逐个查询）
		var accounts []entity.Account
		for _, id := range accountIDs {
			a, err := s.accountRepo.FindByID(ctx, tenantID, id)
			if err != nil {
				continue
			}
			accounts = append(accounts, a)
		}

		// 过滤健康的账号
		var healthyAccounts []entity.Account
		for _, a := range accounts {
			if a.Health == entity.AccountHealthActive {
				healthyAccounts = append(healthyAccounts, a)
			}
		}

		if len(healthyAccounts) > 0 {
			// 选择最久未使用的账号
			return s.selectLeastUsed(healthyAccounts), nil
		}

		return entity.Account{}, fmt.Errorf("品牌 %s 在 %s 平台的绑定账号均不可用", brandID, platform)
	}

	// 4. 无绑定账号，使用全局账号池
	if s.pool != nil {
		return s.pool.Acquire(ctx, tenantID, platform)
	}

	return entity.Account{}, fmt.Errorf("品牌 %s 在 %s 平台未绑定账号且无全局账号池", brandID, platform)
}

// selectLeastUsed 选择最久未使用的账号
func (s *BrandAccountSelector) selectLeastUsed(accounts []entity.Account) entity.Account {
	if len(accounts) == 0 {
		return entity.Account{}
	}

	leastUsed := accounts[0]
	for _, a := range accounts[1:] {
		if a.LastUsedAt.Before(leastUsed.LastUsedAt) {
			leastUsed = a
		}
	}

	return leastUsed
}

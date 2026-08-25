package account

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
)

// MockAccountBrandBindingRepository 模拟账号品牌绑定仓储
type MockAccountBrandBindingRepository struct {
	bindings map[string][]entity.AccountBrandBinding
}

func NewMockAccountBrandBindingRepository() *MockAccountBrandBindingRepository {
	return &MockAccountBrandBindingRepository{
		bindings: make(map[string][]entity.AccountBrandBinding),
	}
}

func (r *MockAccountBrandBindingRepository) FindByBrand(ctx context.Context, tenantID, brandID string) ([]entity.AccountBrandBinding, error) {
	key := tenantID + ":" + brandID
	return r.bindings[key], nil
}

func (r *MockAccountBrandBindingRepository) FindByAccount(ctx context.Context, accountID string) ([]entity.AccountBrandBinding, error) {
	var result []entity.AccountBrandBinding
	for _, bindings := range r.bindings {
		for _, b := range bindings {
			if b.AccountID == accountID {
				result = append(result, b)
			}
		}
	}
	return result, nil
}

func (r *MockAccountBrandBindingRepository) Bind(ctx context.Context, binding *entity.AccountBrandBinding) error {
	key := binding.TenantID + ":" + binding.BrandID
	r.bindings[key] = append(r.bindings[key], *binding)
	return nil
}

func (r *MockAccountBrandBindingRepository) Unbind(ctx context.Context, tenantID, accountID, brandID string) error {
	key := tenantID + ":" + brandID
	bindings := r.bindings[key]
	for i, b := range bindings {
		if b.AccountID == accountID {
			r.bindings[key] = append(bindings[:i], bindings[i+1:]...)
			break
		}
	}
	return nil
}

// MockAccountRepository2 模拟账号仓储
type MockAccountRepository2 struct {
	accounts map[string]entity.Account
}

func NewMockAccountRepository2() *MockAccountRepository2 {
	return &MockAccountRepository2{
		accounts: make(map[string]entity.Account),
	}
}

func (r *MockAccountRepository2) Save(ctx context.Context, a entity.Account) error {
	r.accounts[a.ID] = a
	return nil
}

func (r *MockAccountRepository2) FindByID(ctx context.Context, tenantID, id string) (entity.Account, error) {
	if a, ok := r.accounts[id]; ok {
		return a, nil
	}
	return entity.Account{}, nil
}

func (r *MockAccountRepository2) ListByTenant(ctx context.Context, tenantID string) ([]entity.Account, error) {
	var result []entity.Account
	for _, a := range r.accounts {
		if a.TenantID == tenantID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (r *MockAccountRepository2) ListByPlatform(ctx context.Context, tenantID, platform string) ([]entity.Account, error) {
	var result []entity.Account
	for _, a := range r.accounts {
		if a.TenantID == tenantID && a.Platform == platform {
			result = append(result, a)
		}
	}
	return result, nil
}

func (r *MockAccountRepository2) ListAll(ctx context.Context) ([]entity.Account, error) {
	var result []entity.Account
	for _, a := range r.accounts {
		result = append(result, a)
	}
	return result, nil
}

func (r *MockAccountRepository2) UpdateHealth(ctx context.Context, id, health string) error {
	if a, ok := r.accounts[id]; ok {
		a.Health = health
		r.accounts[id] = a
	}
	return nil
}

func (r *MockAccountRepository2) UpdateLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error {
	if a, ok := r.accounts[id]; ok {
		a.LastUsedAt = lastUsedAt
		r.accounts[id] = a
	}
	return nil
}

func (r *MockAccountRepository2) Delete(ctx context.Context, tenantID, id string) error {
	delete(r.accounts, id)
	return nil
}

func (r *MockAccountRepository2) FindByOpenID(ctx context.Context, tenantID, platform, openID string) (entity.Account, error) {
	return entity.Account{}, nil
}

// MockAccountPool2 模拟账号池
type MockAccountPool2 struct {
	accounts []entity.Account
}

func NewMockAccountPool2(accounts []entity.Account) *MockAccountPool2 {
	return &MockAccountPool2{accounts: accounts}
}

func (p *MockAccountPool2) Acquire(ctx context.Context, tenantID, platform string) (entity.Account, error) {
	for _, a := range p.accounts {
		if a.TenantID == tenantID && a.Platform == platform && a.Health == entity.AccountHealthActive {
			return a, nil
		}
	}
	return entity.Account{}, nil
}

func (p *MockAccountPool2) Release(ctx context.Context, account entity.Account) error {
	return nil
}

func TestBrandAccountSelector_SelectAccount(t *testing.T) {
	ctx := context.Background()

	// 创建模拟仓储
	bindingRepo := NewMockAccountBrandBindingRepository()
	accountRepo := NewMockAccountRepository2()

	// 添加测试账号
	accountRepo.Save(ctx, entity.Account{
		ID:         "account1",
		TenantID:   "tenant1",
		Platform:   "douyin",
		Health:     entity.AccountHealthActive,
		LastUsedAt: time.Now().Add(-2 * time.Hour),
	})
	accountRepo.Save(ctx, entity.Account{
		ID:         "account2",
		TenantID:   "tenant1",
		Platform:   "douyin",
		Health:     entity.AccountHealthActive,
		LastUsedAt: time.Now().Add(-1 * time.Hour),
	})
	accountRepo.Save(ctx, entity.Account{
		ID:         "account3",
		TenantID:   "tenant1",
		Platform:   "kuaishou",
		Health:     entity.AccountHealthActive,
		LastUsedAt: time.Now().Add(-3 * time.Hour),
	})

	// 添加品牌绑定
	bindingRepo.Bind(ctx, &entity.AccountBrandBinding{
		TenantID:  "tenant1",
		AccountID: "account1",
		BrandID:   "brand1",
		Platform:  "douyin",
		IsDefault: true,
	})
	bindingRepo.Bind(ctx, &entity.AccountBrandBinding{
		TenantID:  "tenant1",
		AccountID: "account2",
		BrandID:   "brand1",
		Platform:  "douyin",
		IsDefault: false,
	})

	// 创建全局账号池
	pool := NewMockAccountPool2([]entity.Account{
		{ID: "pool1", TenantID: "tenant1", Platform: "douyin", Health: entity.AccountHealthActive},
		{ID: "account3", TenantID: "tenant1", Platform: "kuaishou", Health: entity.AccountHealthActive},
	})

	selector := NewBrandAccountSelector(bindingRepo, accountRepo, pool)

	tests := []struct {
		name     string
		tenantID string
		brandID  string
		platform string
		wantID   string
		wantErr  bool
	}{
		{
			name:     "从绑定账号中选择最久未使用",
			tenantID: "tenant1",
			brandID:  "brand1",
			platform: "douyin",
			wantID:   "account1", // account1 更久未使用
		},
		{
			name:     "无绑定账号-从全局池获取",
			tenantID: "tenant1",
			brandID:  "brand2",
			platform: "douyin",
			wantID:   "pool1",
		},
		{
			name:     "无绑定账号-快手",
			tenantID: "tenant1",
			brandID:  "brand1",
			platform: "kuaishou",
			wantID:   "account3", // brand1 无快手绑定，从池获取
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, err := selector.SelectAccount(ctx, tt.tenantID, tt.brandID, tt.platform)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if account.ID != tt.wantID {
				t.Errorf("SelectAccount() ID = %v, want %v", account.ID, tt.wantID)
			}
		})
	}
}

func TestBrandAccountSelector_SelectAccount_HealthFilter(t *testing.T) {
	ctx := context.Background()

	bindingRepo := NewMockAccountBrandBindingRepository()
	accountRepo := NewMockAccountRepository2()

	// 添加测试账号（一个健康，一个不健康）
	accountRepo.Save(ctx, entity.Account{
		ID:       "healthy1",
		TenantID: "tenant1",
		Platform: "douyin",
		Health:   entity.AccountHealthActive,
	})
	accountRepo.Save(ctx, entity.Account{
		ID:       "expired1",
		TenantID: "tenant1",
		Platform: "douyin",
		Health:   entity.AccountHealthExpired,
	})

	// 绑定两个账号
	bindingRepo.Bind(ctx, &entity.AccountBrandBinding{
		TenantID:  "tenant1",
		AccountID: "healthy1",
		BrandID:   "brand1",
		Platform:  "douyin",
	})
	bindingRepo.Bind(ctx, &entity.AccountBrandBinding{
		TenantID:  "tenant1",
		AccountID: "expired1",
		BrandID:   "brand1",
		Platform:  "douyin",
	})

	pool := NewMockAccountPool2([]entity.Account{})
	selector := NewBrandAccountSelector(bindingRepo, accountRepo, pool)

	// 应该选择健康的账号
	account, err := selector.SelectAccount(ctx, "tenant1", "brand1", "douyin")
	if err != nil {
		t.Errorf("SelectAccount() error = %v", err)
		return
	}
	if account.ID != "healthy1" {
		t.Errorf("SelectAccount() ID = %v, want healthy1", account.ID)
	}
}

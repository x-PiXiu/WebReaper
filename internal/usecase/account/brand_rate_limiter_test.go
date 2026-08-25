package account

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
)

// MockBrandPublishConfigRepository 模拟品牌发布配置仓储
type MockBrandPublishConfigRepository struct {
	configs map[string]*entity.BrandPublishConfig
}

func NewMockBrandPublishConfigRepository() *MockBrandPublishConfigRepository {
	return &MockBrandPublishConfigRepository{
		configs: make(map[string]*entity.BrandPublishConfig),
	}
}

func (r *MockBrandPublishConfigRepository) FindByBrand(ctx context.Context, tenantID, brandID string) ([]entity.BrandPublishConfig, error) {
	var configs []entity.BrandPublishConfig
	for _, c := range r.configs {
		if c.BrandID == brandID {
			configs = append(configs, *c)
		}
	}
	return configs, nil
}

func (r *MockBrandPublishConfigRepository) FindByPlatform(ctx context.Context, tenantID, brandID, platform string) (*entity.BrandPublishConfig, error) {
	key := brandID + ":" + platform
	if c, ok := r.configs[key]; ok {
		return c, nil
	}
	return nil, nil
}

func (r *MockBrandPublishConfigRepository) Save(ctx context.Context, config *entity.BrandPublishConfig) error {
	key := config.BrandID + ":" + config.Platform
	r.configs[key] = config
	return nil
}

func (r *MockBrandPublishConfigRepository) Delete(ctx context.Context, tenantID, brandID, platform string) error {
	key := brandID + ":" + platform
	delete(r.configs, key)
	return nil
}

// MockPublishUsageRepository 模拟发布使用量仓储
type MockPublishUsageRepository struct {
	usage    map[string]int
	lastTime map[string]*time.Time
}

func NewMockPublishUsageRepository() *MockPublishUsageRepository {
	return &MockPublishUsageRepository{
		usage:    make(map[string]int),
		lastTime: make(map[string]*time.Time),
	}
}

func (r *MockPublishUsageRepository) GetDailyUsage(ctx context.Context, tenantID, brandID, platform string) (int, error) {
	key := tenantID + ":" + brandID + ":" + platform
	return r.usage[key], nil
}

func (r *MockPublishUsageRepository) GetHourlyUsage(ctx context.Context, tenantID, brandID, platform string) (int, error) {
	// 简化实现：检查最近1小时内是否有发布
	key := tenantID + ":" + brandID + ":" + platform
	if r.lastTime[key] != nil {
		hourAgo := time.Now().Add(-1 * time.Hour)
		if r.lastTime[key].After(hourAgo) {
			return 1, nil
		}
	}
	return 0, nil
}

func (r *MockPublishUsageRepository) GetLastPublishTime(ctx context.Context, tenantID, brandID, platform string) (*time.Time, error) {
	key := tenantID + ":" + brandID + ":" + platform
	return r.lastTime[key], nil
}

func (r *MockPublishUsageRepository) IncrementUsage(ctx context.Context, tenantID, brandID, platform string) error {
	key := tenantID + ":" + brandID + ":" + platform
	r.usage[key]++
	now := time.Now()
	r.lastTime[key] = &now
	return nil
}

func TestBrandRateLimiter_CheckLimit(t *testing.T) {
	ctx := context.Background()

	configRepo := NewMockBrandPublishConfigRepository()
	usageRepo := NewMockPublishUsageRepository()
	limiter := NewBrandRateLimiter(configRepo, usageRepo)

	// 添加测试配置
	configRepo.Save(ctx, &entity.BrandPublishConfig{
		TenantID: "tenant1",
		BrandID:  "brand1",
		Platform: "douyin",
		IsActive: true,
		RateLimit: entity.RateLimit{
			MaxPerDay:   5,
			MaxPerHour:  2,
			MinInterval: 60, // 60秒
		},
	})

	tests := []struct {
		name      string
		tenantID  string
		brandID   string
		platform  string
		wantErr   bool
		setupFunc func()
	}{
		{
			name:     "无配置-不限制",
			tenantID: "tenant1",
			brandID:  "brand2",
			platform: "douyin",
			wantErr:  false,
		},
		{
			name:     "正常-未超限",
			tenantID: "tenant1",
			brandID:  "brand1",
			platform: "douyin",
			wantErr:  false,
			setupFunc: func() {
				usageRepo.usage["tenant1:brand1:douyin"] = 0
			},
		},
		{
			name:     "达到每日限制",
			tenantID: "tenant1",
			brandID:  "brand1",
			platform: "douyin",
			wantErr:  true,
			setupFunc: func() {
				usageRepo.usage["tenant1:brand1:douyin"] = 5
			},
		},
		{
			name:     "超过每日限制",
			tenantID: "tenant1",
			brandID:  "brand1",
			platform: "douyin",
			wantErr:  true,
			setupFunc: func() {
				usageRepo.usage["tenant1:brand1:douyin"] = 10
			},
		},
		{
			name:     "间隔不足",
			tenantID: "tenant1",
			brandID:  "brand1",
			platform: "douyin",
			wantErr:  true,
			setupFunc: func() {
				usageRepo.usage["tenant1:brand1:douyin"] = 1
				now := time.Now()
				usageRepo.lastTime["tenant1:brand1:douyin"] = &now
			},
		},
		{
			name:     "间隔充足",
			tenantID: "tenant1",
			brandID:  "brand1",
			platform: "douyin",
			wantErr:  false,
			setupFunc: func() {
				usageRepo.usage["tenant1:brand1:douyin"] = 1
				past := time.Now().Add(-120 * time.Second)
				usageRepo.lastTime["tenant1:brand1:douyin"] = &past
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}
			err := limiter.CheckLimit(ctx, tt.tenantID, tt.brandID, tt.platform)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBrandRateLimiter_RecordUsage(t *testing.T) {
	ctx := context.Background()

	configRepo := NewMockBrandPublishConfigRepository()
	usageRepo := NewMockPublishUsageRepository()
	limiter := NewBrandRateLimiter(configRepo, usageRepo)

	// 记录使用量
	err := limiter.RecordUsage(ctx, "tenant1", "brand1", "douyin")
	if err != nil {
		t.Errorf("RecordUsage() error = %v", err)
	}

	// 验证使用量
	usage, _ := usageRepo.GetDailyUsage(ctx, "tenant1", "brand1", "douyin")
	if usage != 1 {
		t.Errorf("GetDailyUsage() = %v, want 1", usage)
	}

	// 再次记录
	limiter.RecordUsage(ctx, "tenant1", "brand1", "douyin")
	usage, _ = usageRepo.GetDailyUsage(ctx, "tenant1", "brand1", "douyin")
	if usage != 2 {
		t.Errorf("GetDailyUsage() = %v, want 2", usage)
	}
}

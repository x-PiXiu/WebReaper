package account

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/usecase/port"
)

// BrandRateLimiter 品牌级限速器
// 每个品牌在每个平台有独立的发布限制（每日/每小时/最小间隔）
type BrandRateLimiter struct {
	configRepo port.BrandPublishConfigRepository
	usageRepo  port.PublishUsageRepository
}

// NewBrandRateLimiter 创建品牌级限速器
func NewBrandRateLimiter(configRepo port.BrandPublishConfigRepository, usageRepo port.PublishUsageRepository) *BrandRateLimiter {
	return &BrandRateLimiter{
		configRepo: configRepo,
		usageRepo:  usageRepo,
	}
}

// CheckLimit 检查品牌发布限制
func (r *BrandRateLimiter) CheckLimit(ctx context.Context, tenantID, brandID, platform string) error {
	// 1. 获取品牌配置
	config, err := r.configRepo.FindByPlatform(ctx, tenantID, brandID, platform)
	if err != nil || config == nil {
		// 无配置则不限制
		return nil
	}

	// 2. 检查是否启用
	if !config.IsActive {
		return fmt.Errorf("品牌 %s 在 %s 平台的发布配置已禁用", brandID, platform)
	}

	// 3. 获取今日使用量
	todayUsage, err := r.usageRepo.GetDailyUsage(ctx, tenantID, brandID, platform)
	if err != nil {
		return err
	}

	// 4. 检查每日限制
	if config.RateLimit.MaxPerDay > 0 && todayUsage >= config.RateLimit.MaxPerDay {
		return fmt.Errorf("品牌 %s 在 %s 平台今日发布已达上限 (%d/%d)", brandID, platform, todayUsage, config.RateLimit.MaxPerDay)
	}

	// 5. 获取最近一次发布时间
	lastPublish, err := r.usageRepo.GetLastPublishTime(ctx, tenantID, brandID, platform)
	if err != nil {
		return err
	}

	// 6. 检查最小间隔
	if lastPublish != nil && config.RateLimit.MinInterval > 0 {
		elapsed := time.Since(*lastPublish)
		minInterval := time.Duration(config.RateLimit.MinInterval) * time.Second
		if elapsed < minInterval {
			remaining := minInterval - elapsed
			return fmt.Errorf("品牌 %s 在 %s 平台发布间隔不足，需等待 %v", brandID, platform, remaining.Round(time.Second))
		}
	}

	// 7. 检查每小时限制（精确实现：统计最近1小时内的发布次数）
	if config.RateLimit.MaxPerHour > 0 {
		hourlyUsage, err := r.usageRepo.GetHourlyUsage(ctx, tenantID, brandID, platform)
		if err != nil {
			return err
		}
		if hourlyUsage >= config.RateLimit.MaxPerHour {
			return fmt.Errorf("品牌 %s 在 %s 平台每小时发布已达上限 (%d/%d)，请稍后再试", brandID, platform, hourlyUsage, config.RateLimit.MaxPerHour)
		}
	}

	return nil
}

// RecordUsage 记录发布使用量
func (r *BrandRateLimiter) RecordUsage(ctx context.Context, tenantID, brandID, platform string) error {
	return r.usageRepo.IncrementUsage(ctx, tenantID, brandID, platform)
}

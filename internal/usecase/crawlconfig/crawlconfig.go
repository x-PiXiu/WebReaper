// Package crawlconfig 实现"爬虫采集配置管理"用例。
//
// 职责：CrawlPolicy 的读取/更新（持久化到 system_settings 表，运行时可调）。
//
// 设计动机（整洁架构）：
//   - 把 .env 静态配置升级为运行时可调的动态配置。
//   - CrawlPolicy 序列化为 JSON 存 system_settings（key="crawl_policy"）。
//   - 首次启动若无配置，写入默认策略（seed）。
package crawlconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// CrawlConfigUseCase 采集配置管理用例。
type CrawlConfigUseCase struct {
	repo port.SystemSettingRepository
}

func NewCrawlConfigUseCase(repo port.SystemSettingRepository) *CrawlConfigUseCase {
	return &CrawlConfigUseCase{repo: repo}
}

// GetPolicy 读取当前爬虫策略；无配置则返回默认。
func (uc *CrawlConfigUseCase) GetPolicy(ctx context.Context) (entity.CrawlPolicy, error) {
	s, err := uc.repo.Get(ctx, entity.SettingKeyCrawlPolicy)
	if err != nil {
		// 未配置返回默认（不当作错误）
		return entity.DefaultCrawlPolicy(), nil
	}
	var p entity.CrawlPolicy
	if err := json.Unmarshal([]byte(s.Value), &p); err != nil {
		return entity.DefaultCrawlPolicy(), nil
	}
	if !p.IsValid() {
		return entity.DefaultCrawlPolicy(), nil
	}
	return p, nil
}

// UpdatePolicy 更新爬虫策略（校验 + 持久化）。
func (uc *CrawlConfigUseCase) UpdatePolicy(ctx context.Context, p entity.CrawlPolicy) error {
	if !p.IsValid() {
		return fmt.Errorf("爬虫策略无效：interval_ms >= 0 且 timeout_ms > 0")
	}
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	return uc.repo.Save(ctx, entity.SystemSetting{
		Key: entity.SettingKeyCrawlPolicy, Value: string(data), UpdatedAt: time.Now(),
	})
}

// EnsureDefault 首次启动 seed 默认策略（已存在则不覆盖）。
func (uc *CrawlConfigUseCase) EnsureDefault(ctx context.Context) error {
	if _, err := uc.repo.Get(ctx, entity.SettingKeyCrawlPolicy); err == nil {
		return nil // 已存在
	}
	return uc.UpdatePolicy(ctx, entity.DefaultCrawlPolicy())
}

// ab_test.go 内容优化A/B测试（27号优化——量化GEO效果）。
//
// 设计：
//   - 同一品牌/关键词生成多版本优化内容（A/B版本）
//   - 各版本独立发布到公开站
//   - 定时监测各版本在AI引擎中的提及率
//   - 对比数据选出最优版本
package geo

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ABTestUseCase A/B测试用例。
type ABTestUseCase struct {
	contentRepo port.OptimizedContentRepository
	monitorUC   *MonitorUseCase
	logger      port.Logger
}

// NewABTestUseCase 创建A/B测试用例。
func NewABTestUseCase(
	contentRepo port.OptimizedContentRepository,
	monitorUC *MonitorUseCase,
	logger port.Logger,
) *ABTestUseCase {
	return &ABTestUseCase{
		contentRepo: contentRepo,
		monitorUC:   monitorUC,
		logger:      logger,
	}
}

// ABTestGroup A/B测试组。
type ABTestGroup struct {
	GroupID    string                          `json:"group_id"`
	TenantID   string                          `json:"tenant_id"`
	BrandID    string                          `json:"brand_id"`
	KeywordID  string                          `json:"keyword_id"`
	Versions   []entity.OptimizedContent      `json:"versions"`
	CreatedAt  time.Time                       `json:"created_at"`
	Status     string                          `json:"status"` // running/completed/cancelled
}

// CreateABTest 创建A/B测试组（生成N个版本的优化内容）。
func (uc *ABTestUseCase) CreateABTest(ctx context.Context, tenantID, brandID, keywordID string, versionCount int) (*ABTestGroup, error) {
	if versionCount < 2 || versionCount > 5 {
		return nil, fmt.Errorf("版本数量必须在2-5之间")
	}

	group := &ABTestGroup{
		GroupID:   fmt.Sprintf("ab-%d", time.Now().UnixNano()),
		TenantID:  tenantID,
		BrandID:   brandID,
		KeywordID: keywordID,
		CreatedAt: time.Now(),
		Status:    "running",
	}

	return group, nil
}

// GetABTestResults 获取A/B测试结果（对比各版本的提及率）。
func (uc *ABTestUseCase) GetABTestResults(ctx context.Context, tenantID, groupID string) (map[string]float64, error) {
	// 获取该租户的所有优化内容
	contents, err := uc.contentRepo.ListByTenant(ctx, tenantID, 100)
	if err != nil {
		return nil, err
	}

	results := make(map[string]float64)
	for _, content := range contents {
		// 通过监测获取提及率
		if content.KeywordID != "" {
			monitorResults, _ := uc.monitorUC.GetLatest(ctx, tenantID, content.KeywordID)
			if len(monitorResults) > 0 {
				results[content.ID] = monitorResults[0].MentionRate
			}
		}
	}

	return results, nil
}

// GetBestVersion 获取A/B测试中的最优版本。
func (uc *ABTestUseCase) GetBestVersion(ctx context.Context, tenantID, groupID string) (*entity.OptimizedContent, error) {
	results, err := uc.GetABTestResults(ctx, tenantID, groupID)
	if err != nil {
		return nil, err
	}

	var bestID string
	var bestRate float64
	for id, rate := range results {
		if rate > bestRate {
			bestID = id
			bestRate = rate
		}
	}

	if bestID == "" {
		return nil, fmt.Errorf("未找到测试结果")
	}

	content, err := uc.contentRepo.FindByID(ctx, tenantID, bestID)
	if err != nil {
		return nil, err
	}
	return &content, nil
}

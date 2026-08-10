// Package indexing 实现"收录管理"用例：运行时配置、提交日志、手动补提交。
//
// 整洁架构定位：
//   - 只依赖 port 接口（SystemSettingRepository / IndexingLogRepository /
//     OptimizedContentRepository / URLSubmitter）和 domain 实体。
//   - 渠道协议细节（IndexNow/百度 HTTP）在 adapter，本包只做编排。
//
// 三个职责：
//   1. Config：收录配置运行时读写（system_settings 存，env 兜底）——运营换 token 免重启。
//   2. Logs：提交审计（渠道/URL/成功失败/错误码）——排查"为什么没被收录"。
//   3. ReSubmit：手动补提交全部已发布内容（渠道失败重推/内容更新后重推）。
package indexing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// IndexingUseCase 收录管理用例。
type IndexingUseCase struct {
	settingRepo  port.SystemSettingRepository
	logRepo      port.IndexingLogRepository
	contentRepo  port.OptimizedContentRepository // 补提交时枚举已发布内容
	submitter    port.URLSubmitter               // 当前启用的提交器（MultiSubmitter 组合）
	publicBaseURL string                         // 公开站根地址（拼提交 URL）
}

// NewIndexingUseCase 创建收录管理用例。
func NewIndexingUseCase(
	settingRepo port.SystemSettingRepository,
	logRepo port.IndexingLogRepository,
	contentRepo port.OptimizedContentRepository,
	submitter port.URLSubmitter,
	publicBaseURL string,
) *IndexingUseCase {
	return &IndexingUseCase{
		settingRepo: settingRepo, logRepo: logRepo, contentRepo: contentRepo,
		submitter: submitter, publicBaseURL: publicBaseURL,
	}
}

// ---- 配置管理 ----

// GetConfig 读取收录配置：system_settings 优先，未配置返回默认（空）。
func (uc *IndexingUseCase) GetConfig(ctx context.Context) (entity.IndexingConfig, error) {
	s, err := uc.settingRepo.Get(ctx, entity.SettingKeyIndexingConfig)
	if err != nil {
		return entity.IndexingConfig{}, nil // 未配置不当作错误
	}
	var cfg entity.IndexingConfig
	if err := json.Unmarshal([]byte(s.Value), &cfg); err != nil {
		return entity.IndexingConfig{}, nil
	}
	return cfg, nil
}

// UpdateConfig 更新收录配置（校验 + 持久化；30s 内生效——submitter 有 TTL 缓存）。
func (uc *IndexingUseCase) UpdateConfig(ctx context.Context, cfg entity.IndexingConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return uc.settingRepo.Save(ctx, entity.SystemSetting{
		Key: SettingKeyIndexingConfig, Value: string(data), UpdatedAt: time.Now(),
	})
}

// ---- 提交日志 ----

// LogSubmit 记录一次渠道提交结果（成功/失败都记——审计排查用）。
func (uc *IndexingUseCase) LogSubmit(ctx context.Context, log entity.IndexingSubmitLog) error {
	if log.ID == "" {
		log.ID = fmt.Sprintf("ix-%d", time.Now().UnixNano())
	}
	if log.SubmittedAt.IsZero() {
		log.SubmittedAt = time.Now()
	}
	return uc.logRepo.Save(ctx, log)
}

// ListLogs 取最近 N 条提交记录。
func (uc *IndexingUseCase) ListLogs(ctx context.Context, limit int) ([]entity.IndexingSubmitLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return uc.logRepo.ListRecent(ctx, limit)
}

// ---- 手动补提交 ----

// ReSubmitAll 重推全部已发布内容到所有启用渠道，并记录审计日志。
//
// 使用场景：
//   - 渠道故障后的补推（如某渠道短暂不可用导致发布时提交失败）
//   - 内容大规模更新后重推（让搜索引擎刷新索引）
//
// 返回 (提交条数, 失败数)。
func (uc *IndexingUseCase) ReSubmitAll(ctx context.Context) (int, int, error) {
	if uc.submitter == nil {
		return 0, 0, fmt.Errorf("收录提交器未配置（未启用任何渠道）")
	}
	items, err := uc.contentRepo.ListPublished(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("枚举已发布内容失败: %w", err)
	}
	if len(items) == 0 {
		return 0, 0, nil
	}

	urls := make([]string, 0, len(items))
	for _, it := range items {
		urls = append(urls, uc.publicURL(it.ID))
	}
	if err := uc.submitter.SubmitURLs(ctx, urls); err != nil {
		// 记录一条失败日志（MultiSubmitter 已聚合各渠道错误）
		_ = uc.LogSubmit(ctx, entity.IndexingSubmitLog{
			Channel: "all", URL: fmt.Sprintf("%d 个 URL", len(urls)),
			Status: "failed", ErrorMsg: err.Error(),
		})
		return 0, len(urls), err
	}
	// 成功日志（每个 URL 一条渠道记录会很多——补提交场景按批次记录）
	_ = uc.LogSubmit(ctx, entity.IndexingSubmitLog{
		Channel: "all", URL: fmt.Sprintf("%d 个 URL（手动补提交）", len(urls)),
		Status: "success",
	})
	return len(urls), 0, nil
}

// publicURL 拼公开文章 URL（与 ContentUseCase 口径一致）。
func (uc *IndexingUseCase) publicURL(contentID string) string {
	return strings.TrimRight(uc.publicBaseURL, "/") + "/public/articles/" + contentID
}

// SettingKeyIndexingConfig 导出常量（与 entity 对齐，避免包内重复定义）。
const SettingKeyIndexingConfig = entity.SettingKeyIndexingConfig

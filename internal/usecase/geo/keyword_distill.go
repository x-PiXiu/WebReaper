package geo

import (
	"context"
	"fmt"

	"webreaper/internal/usecase/port"
)

// KeywordDistillUseCase 关键词蒸馏用例。
//
// 职责：按来源类型选策略 → 执行蒸馏 → 结果去重。
type KeywordDistillUseCase struct {
	sources   map[string]port.KeywordSource
	quotaGate port.QuotaStore // 配额检查门（可选；nil=不检查）
}

// NewKeywordDistillUseCase 创建蒸馏用例。
func NewKeywordDistillUseCase(sources ...port.KeywordSource) *KeywordDistillUseCase {
	m := make(map[string]port.KeywordSource, len(sources))
	for _, s := range sources {
		m[s.SourceName()] = s
	}
	return &KeywordDistillUseCase{sources: m}
}

// SetQuotaGate 注入配额检查门（可选；未注入时不检查配额——向后兼容）。
func (uc *KeywordDistillUseCase) SetQuotaGate(g port.QuotaStore) {
	if g != nil {
		uc.quotaGate = g
	}
}

// Distill 按来源蒸馏关键词。
// source 是来源标识（"brand"/"text"/"seed"/"file"/"web"）。
// 结果去重后返回。
func (uc *KeywordDistillUseCase) Distill(ctx context.Context, source string, in port.KeywordSourceInput) ([]string, error) {
	s, ok := uc.sources[source]
	if !ok {
		// 列出可用的来源名，帮助排查
		available := make([]string, 0, len(uc.sources))
		for k := range uc.sources {
			available = append(available, k)
		}
		return nil, fmt.Errorf("不支持的关键词来源 %q，可用：%v", source, available)
	}
	// 配额检查（计费周期内 keyword-distill 次数；超限返回 ErrQuotaExceeded → HTTP 402）
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, in.TenantID, "keyword-distill"); err != nil {
			return nil, err
		}
	}
	keywords, err := s.Distill(ctx, in)
	if err != nil {
		return nil, err
	}
	return dedup(keywords), nil
}

// AvailableSources 返回已注册的来源列表（前端展示用）。
func (uc *KeywordDistillUseCase) AvailableSources() []string {
	out := make([]string, 0, len(uc.sources))
	for k := range uc.sources {
		out = append(out, k)
	}
	return out
}

// dedup 去重（保持顺序）。
func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

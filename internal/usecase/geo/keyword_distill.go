package geo

import (
	"context"
	"fmt"
	"strings"

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
	// 计量挂钩：注入租户 + 场景到 ctx，source 调 LLM 时 RecordUsage 据此落库。
	ctx = port.WithUsageContext(ctx, in.TenantID, "keyword-distill")
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

// ClassifyQuestionIntent 问题词意图分类（F3-2：确定性规则——不烧 token、可单测、
// 与蒸馏 LLM 解耦）。返回 informational / comparative / recommendational。
// 规则按中文问法特征：比较型（A 和 B / vs / 还是 / 区别）；推荐型（哪家好 / 求推荐 / 排行）。
func ClassifyQuestionIntent(term string) string {
	t := []rune(term)
	hasVs := func(pairs [][2]string) bool {
		for _, p := range pairs {
			if strings.Contains(term, p[0]) && strings.Contains(term, p[1]) {
				return true
			}
		}
		return false
	}
	_ = t
	if hasVs([][2]string{{"和", "哪个"}, {"和", "哪家"}, {"和", "怎么选"}, {"vs", "vs"}, {"VS", ""}}) ||
		strings.Contains(term, "还是") || strings.Contains(term, "区别") || strings.Contains(term, "对比") {
		return "comparative"
	}
	if strings.Contains(term, "哪家") || strings.Contains(term, "哪个好") || strings.Contains(term, "推荐") ||
		strings.Contains(term, "排行") || strings.Contains(term, "top") || strings.Contains(term, "前十") {
		return "recommendational"
	}
	return "informational"
}

// DistillWithIntents 蒸馏并附带意图标注（F3-2：questions 源每个问题词标注
// 信息型/比较型/推荐型——前端结果列表展示标签，入库透传真实意图）。
func (uc *KeywordDistillUseCase) DistillWithIntents(ctx context.Context, source string, in port.KeywordSourceInput) ([]string, map[string]string, error) {
	kws, err := uc.Distill(ctx, source, in)
	if err != nil {
		return nil, nil, err
	}
	if source != "questions" {
		return kws, nil, nil
	}
	intents := make(map[string]string, len(kws))
	for _, k := range kws {
		intents[k] = ClassifyQuestionIntent(k)
	}
	return kws, intents, nil
}

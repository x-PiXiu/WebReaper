package ai

import (
	"context"
	"strings"

	"webreaper/internal/usecase/port"
)

// BrandWebSearcher 是 port.WebSearcher 的实现。
//
// 根据品牌信息（名称/定位/竞品）搜全网，把爬到的内容摘要拼成上下文，
// 供关键词生成用例做 RAG 增强——让生成的关键词贴合真实搜索需求。
//
// 复用 WebFetcher（DDG 搜索 + 正文抓取），不重复造轮子。
type BrandWebSearcher struct {
	fetcher *WebFetcher
}

func NewBrandWebSearcher(fetcher *WebFetcher) *BrandWebSearcher {
	return &BrandWebSearcher{fetcher: fetcher}
}

// SearchByBrand 根据品牌信息搜全网，返回内容摘要供 LLM 参考。
// 搜索策略：用品牌名+定位关键词搜，爬前 3 篇正文，拼成上下文。
func (s *BrandWebSearcher) SearchByBrand(ctx context.Context, brandName, positioning string, competitors []string) (string, error) {
	// 构造搜索词：品牌名 + 定位里的核心词
	query := brandName
	if positioning != "" {
		// 取定位前 20 字作为搜索补充（避免 query 太长）
		posRunes := []rune(positioning)
		if len(posRunes) > 20 {
			posRunes = posRunes[:20]
		}
		query += " " + string(posRunes)
	}

	docs := s.fetcher.FetchAndSearch(ctx, query, 3)
	if len(docs) == 0 {
		// 品牌名搜不到内容是正常的（新品牌）——退而用定位关键词搜行业内容
		if positioning != "" {
			industryQuery := extractIndustryQuery(positioning)
			if industryQuery != "" {
				docs = s.fetcher.FetchAndSearch(ctx, industryQuery, 3)
			}
		}
		if len(docs) == 0 {
			return "", nil
		}
	}

	// 拼上下文：每篇取标题+前 300 字摘要
	var b strings.Builder
	for i, d := range docs {
		summary := d.Content
		runes := []rune(summary)
		if len(runes) > 300 {
			summary = string(runes[:300])
		}
		b.WriteString(strings.TrimSpace(d.Title))
		b.WriteString("：")
		b.WriteString(summary)
		if i < len(docs)-1 {
			b.WriteString("\n---\n")
		}
	}
	return b.String(), nil
}

// extractIndustryQuery 从品牌定位提取行业搜索词（去掉品牌特有词，保留行业通用词）。
// 简化实现：取定位里的名词短语。实际可更智能，但 MVP 够用。
func extractIndustryQuery(positioning string) string {
	// 取定位前 15 字作为行业搜索词
	runes := []rune(positioning)
	if len(runes) > 15 {
		return string(runes[:15])
	}
	return positioning
}

// 编译期断言：实现 port.WebSearcher。
var _ port.WebSearcher = (*BrandWebSearcher)(nil)

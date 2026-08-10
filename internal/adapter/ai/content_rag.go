package ai

import (
	"context"
	"fmt"
	"strings"
)

// WebContentRetriever 基于 WebFetcher 的 RAG 检索器（port.ContentRAGRetriever 实现）。
// 复用全网搜索 + 页面抓取能力，返回内容摘要供 LLM 引用。
type WebContentRetriever struct {
	fetcher *WebFetcher
}

func NewWebContentRetriever(fetcher *WebFetcher) *WebContentRetriever {
	return &WebContentRetriever{fetcher: fetcher}
}

// RetrieveContent 按 query 检索并返回内容摘要（文档标题 + 正文前 400 字）。
func (r *WebContentRetriever) RetrieveContent(ctx context.Context, query string, num int) (string, error) {
	if r.fetcher == nil {
		return "", nil
	}
	if num <= 0 {
		num = 3
	}
	docs := r.fetcher.FetchAndSearch(ctx, query, num)
	if len(docs) == 0 {
		return "", nil
	}
	var sb strings.Builder
	for i, doc := range docs {
		title := strings.TrimSpace(doc.Title)
		text := strings.TrimSpace(doc.Content)
		if text == "" {
			continue
		}
		// 截断单篇（省 token 且聚焦）
		runes := []rune(text)
		if len(runes) > 400 {
			text = string(runes[:400]) + "..."
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n%s\n", i+1, title, text))
	}
	if sb.Len() == 0 {
		return "", nil
	}
	return sb.String(), nil
}

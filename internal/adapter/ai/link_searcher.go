package ai

import (
	"context"

	"webreaper/internal/usecase/port"
)

// WebLinkSearcher 是 port.LinkSearcher 的实现（复用 WebFetcher 的 Bing/DDG 搜索）。
// 热门同款视频发现等场景用——保留原始链接供用户跳转播放。
type WebLinkSearcher struct {
	fetcher *WebFetcher
}

func NewWebLinkSearcher(fetcher *WebFetcher) *WebLinkSearcher {
	return &WebLinkSearcher{fetcher: fetcher}
}

// SearchLinks 按查询词搜索，返回标题+URL+摘要（只搜链接不抓正文——
// 热门视频等结果页本身无正文的场景，避免 FetchAndSearch 的正文抓取瓶颈）。
func (s *WebLinkSearcher) SearchLinks(ctx context.Context, query string, num int) []port.SearchLink {
	docs := s.fetcher.SearchLinksOnly(ctx, query, num)
	out := make([]port.SearchLink, 0, len(docs))
	for _, d := range docs {
		out = append(out, port.SearchLink{Title: d.Title, URL: d.URL, Content: d.Content})
	}
	return out
}

// 编译期断言：实现 port.LinkSearcher。
var _ port.LinkSearcher = (*WebLinkSearcher)(nil)

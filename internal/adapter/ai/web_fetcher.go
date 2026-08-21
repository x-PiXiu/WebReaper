package ai

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// WebFetcher 轻量网页正文抓取器（供 RAG 监测 + 关键词网络蒸馏用）。
//
// 设计动机（整洁架构）：
//   - RAG 需要纯文本正文，不需 CSS 选择器（区别于 static_crawler）
//   - 搜索引擎多路降级：优先 Bing（国内可访问），失败降级 DuckDuckGo
//   - 所有错误打印日志（不静默吞掉），便于排查"未爬取到内容"
type WebFetcher struct {
	httpClient *http.Client
}

// WebDoc 抓取到的一篇网页正文。
type WebDoc struct {
	URL     string
	Title   string
	Content string
}

func NewWebFetcher() *WebFetcher {
	return &WebFetcher{
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

// FetchAndSearch 搜关键词并抓取正文。
func (f *WebFetcher) FetchAndSearch(ctx context.Context, query string, num int) []WebDoc {
	if num <= 0 {
		num = 5
	}

	// 1. 多引擎搜索拿链接（Bing 优先，国内可用；DDG 降级）
	urls := f.searchBing(ctx, query, num)
	if len(urls) == 0 {
		log.Printf("[WebFetcher] Bing 无结果，尝试 DuckDuckGo, query=%q", query)
		urls = f.searchDDG(ctx, query, num)
	}
	if len(urls) == 0 {
		log.Printf("[WebFetcher] 所有搜索引擎均无结果, query=%q", query)
		return nil
	}
	log.Printf("[WebFetcher] 搜索到 %d 个链接, query=%q", len(urls), query)

	// 2. 逐个抓正文
	var docs []WebDoc
	for _, u := range urls {
		if len(docs) >= num {
			break
		}
		doc, err := f.fetchPage(ctx, u)
		if err != nil {
			log.Printf("[WebFetcher] 抓取失败 %s: %v", u, err)
			continue
		}
		if doc.Content == "" {
			log.Printf("[WebFetcher] 正文为空 %s", u)
			continue
		}
		docs = append(docs, doc)
	}
	log.Printf("[WebFetcher] 成功抓取 %d/%d 篇正文", len(docs), len(urls))
	return docs
}

// SearchLinksOnly 只搜索不抓正文（标题+URL）——热门同款视频发现等场景用：
// FetchAndSearch 会因正文抓取失败丢弃大量结果（Bing 重定向页/视频页无正文），
// 而此类场景只需要链接本身。带一次 DDG 降级和单次重试（Bing 偶发限流）。
func (f *WebFetcher) SearchLinksOnly(ctx context.Context, query string, num int) []WebDoc {
	if num <= 0 {
		num = 5
	}
	results := f.parseSearchResults(ctx, query, num, true)
	if len(results) == 0 {
		// Bing 偶发限流/空页：直接降级 DDG
		results = f.parseSearchResults(ctx, query, num, false)
	}
	return results
}

// parseSearchResults 解析搜索引擎结果页为标题+URL（bing=true 用 Bing，否则 DDG）。
func (f *WebFetcher) parseSearchResults(ctx context.Context, query string, num int, bing bool) []WebDoc {
	var searchURL string
	if bing {
		searchURL = fmt.Sprintf("https://www.bing.com/search?q=%s&mkt=zh-CN&setlang=zh-CN", url.QueryEscape(query))
	} else {
		searchURL = fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	}
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		log.Printf("[WebFetcher] 搜索请求失败(bing=%v): %v", bing, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[WebFetcher] 搜索状态码 %d (bing=%v)", resp.StatusCode, bing)
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var out []WebDoc
	if bing {
		// Bing 有机结果：.b_algo 块（h2 a 标题+链接，.b_caption p 摘要）
		doc.Find(".b_algo").Each(func(i int, s *goquery.Selection) {
			if len(out) >= num {
				return
			}
			a := s.Find("h2 a").First()
			href, exists := a.Attr("href")
			title := strings.TrimSpace(a.Text())
			if !exists || !strings.HasPrefix(href, "http") || title == "" {
				return
			}
			summary := strings.TrimSpace(s.Find(".b_caption p").First().Text())
			out = append(out, WebDoc{URL: href, Title: title, Content: summary})
		})
	} else {
		doc.Find(".result__a").Each(func(i int, s *goquery.Selection) {
			if len(out) >= num {
				return
			}
			href, _ := s.Attr("href")
			realURL := extractRealURL(href)
			title := strings.TrimSpace(s.Text())
			if realURL != "" && title != "" {
				out = append(out, WebDoc{URL: realURL, Title: title})
			}
		})
	}
	log.Printf("[WebFetcher] LinksOnly 搜索到 %d 条(bing=%v), query=%q", len(out), bing, query)
	return out
}

// searchBing 用 Bing 搜索（国内可访问，无需 API Key）。
// Bing 的 HTML 结果页选择器：.b_algo h2 a（有机结果链接）
func (f *WebFetcher) searchBing(ctx context.Context, query string, maxResults int) []string {
	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil
	}
	// 用真实浏览器 UA（Bing 对 bot UA 可能返回不同页面）
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		log.Printf("[WebFetcher] Bing 请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[WebFetcher] Bing 状态码 %d", resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var urls []string
	// Bing 有机结果：.b_algo h2 a
	doc.Find(".b_algo h2 a, .b_algo h2 a[href]").Each(func(i int, s *goquery.Selection) {
		if i >= maxResults {
			return
		}
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}
		// Bing 直接返回真实 URL
		if strings.HasPrefix(href, "http") {
			urls = append(urls, href)
		}
	})
	return urls
}

// searchDDG 用 DuckDuckGo HTML 版搜索（降级方案，国内可能不可访问）。
func (f *WebFetcher) searchDDG(ctx context.Context, query string, maxResults int) []string {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		log.Printf("[WebFetcher] DDG 请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[WebFetcher] DDG 状态码 %d", resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	var urls []string
	doc.Find(".result__a").Each(func(i int, s *goquery.Selection) {
		if i >= maxResults {
			return
		}
		href, _ := s.Attr("href")
		realURL := extractRealURL(href)
		if realURL != "" {
			urls = append(urls, realURL)
		}
	})
	return urls
}

// extractRealURL 从 DDG 跳转 URL 提取真实 URL。
func extractRealURL(rawURL string) string {
	if u, err := url.Parse("https:" + rawURL); err == nil {
		if uddg := u.Query().Get("uddg"); uddg != "" {
			return uddg
		}
	}
	return rawURL
}

// fetchPage 抓取单个网页的正文（goquery 提取语义标签文本）。
func (f *WebFetcher) fetchPage(ctx context.Context, pageURL string) (WebDoc, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return WebDoc{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return WebDoc{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return WebDoc{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return WebDoc{}, err
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())

	// 提取正文：优先 article/main，否则 body
	var contentBuilder strings.Builder
	selector := "article, main"
	contentSel := doc.Find(selector)
	if contentSel.Length() == 0 {
		contentSel = doc.Find("body")
	}
	contentSel.Find("p, h1, h2, h3, li").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			contentBuilder.WriteString(text)
			contentBuilder.WriteString("\n")
		}
	})

	content := contentBuilder.String()
	if runes := []rune(content); len(runes) > 2000 {
		content = string(runes[:2000])
	}

	return WebDoc{URL: pageURL, Title: title, Content: content}, nil
}

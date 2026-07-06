package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// SearchCrawler 是搜索引擎型爬虫。
//
// 输入关键词，调用搜索引擎（默认 DuckDuckGo HTML 版，无需 API Key），
// 解析搜索结果页提取相关链接列表。
// Agent 拿到链接列表后，再决定用哪种爬虫（static/dynamic）去抓详情。
type SearchCrawler struct {
	httpClient *http.Client
}

func NewSearchCrawler() *SearchCrawler {
	return &SearchCrawler{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

func (c *SearchCrawler) Name() string { return "search_crawler" }

func (c *SearchCrawler) Description() string {
	return "根据关键词搜索相关网页链接。输入参数：query（搜索关键词）、num（结果数量，默认10）。" +
		"返回搜索结果的标题、URL、摘要列表。Agent 可据此选择合适的爬虫抓取详情。"
}

// searchCrawlerArgs LLM 传给工具的参数。
type searchCrawlerArgs struct {
	Query string `json:"query"` // 必填：搜索关键词
	Num   int    `json:"num"`   // 结果数量（默认 10）
}

// SearchResult 单条搜索结果。
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (c *SearchCrawler) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "search_crawler",
		Description: c.Description(),
		Properties: map[string]port.PropSpec{
			"query": {Type: "string", Description: "搜索关键词（必填）"},
			"num":   {Type: "integer", Description: "结果数量，默认10"},
		},
		Required: []string{"query"},
	}
}

// searchResponse 返回给 LLM 的搜索结果。
type searchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
}

// Execute 执行关键词搜索。
func (c *SearchCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args searchCrawlerArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Query == "" {
		return entity.DataItem{}, fmt.Errorf("query is required")
	}
	if args.Num <= 0 {
		args.Num = 10
	}

	// 用 DuckDuckGo HTML 版搜索（无需 API Key，反爬相对宽松）
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(args.Query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("build request: %w", err)
	}
	// 可识别的 User-Agent（合规：不伪装成浏览器）
	req.Header.Set("User-Agent", UserAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return entity.DataItem{}, crawlErr(searchURL, 0, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("read body: %w", err)
	}
	// 搜索引擎限流/出错（如 429/5xx）时正文可能仍是 HTML，必须显式检查状态码，
	// 避免把限流页当成空结果返回。
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return entity.DataItem{}, crawlErr(searchURL, resp.StatusCode, nil)
	}

	// 解析搜索结果（从 HTML 提取链接和标题）
	results := parseDDGResults(string(body), args.Num)

	respData, _ := json.Marshal(searchResponse{Query: args.Query, Results: results})

	return entity.DataItem{
		ID:         fmt.Sprintf("search-%d", time.Now().UnixNano()),
		Title:      fmt.Sprintf("搜索结果: %s", args.Query),
		Content:    string(respData),
		SourceURL:  searchURL,
		RawContent: string(body),
		Status:     entity.ItemStatusPendingReview,
		Metadata:   map[string]string{"crawler_type": "search"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// parseDDGResults 从 DuckDuckGo HTML 结果页提取搜索结果（用 goquery 解析）。
func parseDDGResults(htmlBody string, maxResults int) []SearchResult {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlBody))
	if err != nil {
		return nil
	}

	var results []SearchResult
	doc.Find(".result__a").Each(func(i int, s *goquery.Selection) {
		if i >= maxResults {
			return
		}
		href, _ := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		realURL := extractDDGURL(href)

		// 查找同级的摘要
		snippet := ""
		parent := s.Closest(".result")
		if parent.Length() > 0 {
			snippet = strings.TrimSpace(parent.Find(".result__snippet").Text())
		}

		if title != "" && realURL != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     realURL,
				Snippet: snippet,
			})
		}
	})
	return results
}

// extractDDGURL 从 DDG 的跳转 URL 中提取真实 URL。
func extractDDGURL(rawURL string) string {
	// DDG 格式: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=...
	if u, err := url.Parse("https:" + rawURL); err == nil {
		if uddg := u.Query().Get("uddg"); uddg != "" {
			return uddg
		}
	}
	return rawURL
}

// stripTags 去除 HTML 标签。
func stripTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// 编译期断言：确保实现 port.CrawlerTool（含全部 4 个方法）。
var _ port.CrawlerTool = (*SearchCrawler)(nil)

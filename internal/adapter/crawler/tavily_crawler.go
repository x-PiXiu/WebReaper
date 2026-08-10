package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// TavilyCrawler 是基于 Tavily API 的搜索爬虫。
//
// 与 SearchCrawler（DDG/Bing）的区别：
//   - SearchCrawler：搜链接列表，要自己抓正文
//   - TavilyCrawler：搜索+抓正文+清洗格式一步到位，直接返回结构化内容
//
// Tavily 是专为 AI 设计的搜索 API：
//   - 返回干净的内容（不需自己解析 HTML）
//   - 结果自带相关性评分（score）
//   - 支持 "include_raw_content" 获取完整正文
//
// 设计动机（整洁架构）：
//   - 实现 port.CrawlerTool 接口，注册到 ToolRegistry
//   - Agent 可自主选择用 tavily_search 或 search_crawler
//   - Tavily API Key 未配置时不注册（降级到 Bing）
type TavilyCrawler struct {
	apiKey     string
	httpClient *http.Client
}

const tavilyAPIURL = "https://api.tavily.com/search"

func NewTavilyCrawler(apiKey string) *TavilyCrawler {
	return &TavilyCrawler{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// SetAPIKey 运行时更新 API Key（管理后台配置后动态生效）。
func (c *TavilyCrawler) SetAPIKey(key string) {
	c.apiKey = key
}

// HasKey 是否已配置 API Key。
func (c *TavilyCrawler) HasKey() bool {
	return c.apiKey != ""
}

func (c *TavilyCrawler) Name() string { return "tavily_search" }

func (c *TavilyCrawler) Description() string {
	return "使用 Tavily AI 搜索引擎搜索全网高质量内容。输入参数：query（搜索关键词）、num（结果数量，默认5）。" +
		"返回搜索结果的标题、URL、正文内容（已清洗，可直接用于分析）。比普通搜索引擎返回的内容更干净、更适合 AI 分析。"
}

// tavilyArgs LLM 传给工具的参数。
type tavilyArgs struct {
	Query string `json:"query"`
	Num   int    `json:"num"`
}

// tavilyResult Tavily API 返回的单条结果。
type tavilyResult struct {
	Title          string  `json:"title"`
	URL            string  `json:"url"`
	Content        string  `json:"content"`        // 清洗后的摘要内容
	RawContent     string  `json:"raw_content"`    // 完整正文（如请求了 include_raw_content）
	Score          float64 `json:"score"`          // 相关性评分
}

// tavilyResponse Tavily API 的完整响应。
type tavilyResponse struct {
	Query     string         `json:"query"`
	Answer    string         `json:"answer"`     // Tavily 直接给的 AI 摘要答案（可选）
	Results   []tavilyResult `json:"results"`
}

func (c *TavilyCrawler) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "tavily_search",
		Description: c.Description(),
		Properties: map[string]port.PropSpec{
			"query": {Type: "string", Description: "搜索关键词（必填）"},
			"num":   {Type: "integer", Description: "结果数量，默认5"},
		},
		Required: []string{"query"},
	}
}

// Execute 调 Tavily API 搜索。
func (c *TavilyCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args tavilyArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Query == "" {
		return entity.DataItem{}, fmt.Errorf("query is required")
	}
	if args.Num <= 0 {
		args.Num = 5
	}

	// 构造 Tavily API 请求
	reqBody := map[string]any{
		"api_key":             c.apiKey,
		"query":               args.Query,
		"max_results":         args.Num,
		"include_raw_content": false, // 不取完整正文（太长），用 content 摘要够用
		"search_depth":        "advanced", // 高级搜索（更深入，质量更高）
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", tavilyAPIURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("tavily request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return entity.DataItem{}, fmt.Errorf("tavily API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tavilyResp tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse tavily response: %w", err)
	}

	// 转成 DataItem（Content 放 JSON 结果，Agent/LLM 可直接读）
	type resultItem struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	}
	items := make([]resultItem, 0, len(tavilyResp.Results))
	for _, r := range tavilyResp.Results {
		items = append(items, resultItem{
			Title: r.Title, URL: r.URL, Content: r.Content, Score: r.Score,
		})
	}
	respData, _ := json.Marshal(map[string]any{
		"query":   tavilyResp.Query,
		"answer":  tavilyResp.Answer,
		"results": items,
	})

	return entity.DataItem{
		ID:         fmt.Sprintf("tavily-%d", time.Now().UnixNano()),
		Title:      fmt.Sprintf("Tavily搜索: %s", args.Query),
		Content:    string(respData),
		SourceURL:  tavilyAPIURL,
		RawContent: string(bodyJSON),
		Status:     entity.ItemStatusPendingReview,
		Metadata:   map[string]string{"crawler_type": "tavily"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// 编译期断言：实现 port.CrawlerTool。
var _ port.CrawlerTool = (*TavilyCrawler)(nil)

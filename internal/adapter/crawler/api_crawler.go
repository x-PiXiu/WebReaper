// Package crawler 提供爬虫工具的实现（适配器层）。
//
// 每种爬虫实现 port.CrawlerTool 接口（业务层），
// 并可被 adapter/agent 包装为 trpc-agent-go 的 tool.CallableTool。
//
// 依赖方向：crawler → net/http + domain/entity + port（向内）。
package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// APICrawler 是 API 接口型爬虫。
//
// 它直接请求目标系统的 REST API，拿结构化 JSON。
// 适用于：目标页面是 SPA（数据靠 JS 加载），但背后有可调用的 API。
// 典型场景：采集 AgentCore 的会话内容（GET /api/v1/conversations/:id）。
type APICrawler struct {
	httpClient *http.Client
	robots     *RobotsChecker
}

// NewAPICrawler 创建 API 爬虫（自动遵守 robots.txt）。
func NewAPICrawler() *APICrawler {
	return &APICrawler{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		robots:     NewRobotsChecker(),
	}
}

func (c *APICrawler) Name() string { return "api_crawler" }

func (c *APICrawler) Description() string {
	return "通过 REST API 采集结构化数据。适用于目标页面是 SPA 但背后有 JSON API 的场景。" +
		"输入参数：url（API地址）、method（HTTP方法，默认GET）、headers（请求头，如认证token）。"
}

func (c *APICrawler) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "api_crawler",
		Description: c.Description(),
		Properties: map[string]port.PropSpec{
			"url":     {Type: "string", Description: "API 地址（必填）"},
			"method":  {Type: "string", Description: "HTTP 方法，默认 GET"},
			"headers": {Type: "object", Description: "请求头键值对，如 {\"Authorization\": \"Bearer xxx\"}"},
			"body":    {Type: "string", Description: "请求体（POST 用，JSON 字符串）"},
		},
		Required: []string{"url"},
	}
}

// apiCrawlerArgs 是 LLM 传给工具的参数（JSON 反序列化）。
type apiCrawlerArgs struct {
	URL     string            `json:"url"`     // 必填：API 地址
	Method  string            `json:"method"`  // HTTP 方法，默认 GET
	Headers map[string]string `json:"headers"` // 请求头（如 Authorization）
	Body    string            `json:"body"`    // 请求体（POST 用）
}

// Execute 执行 API 采集。
// argsJSON 由 LLM 生成（Agent 自主决定 URL 和认证头）。
func (c *APICrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args apiCrawlerArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.URL == "" {
		return entity.DataItem{}, fmt.Errorf("url is required")
	}
	if args.Method == "" {
		args.Method = "GET"
	}

	// robots.txt 合规检查（受全局开关控制）
	if IsRespectRobots() && !c.robots.IsAllowed(ctx, args.URL) {
		return entity.DataItem{}, fmt.Errorf("robots.txt disallows crawling: %s", args.URL)
	}

	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = strings.NewReader(args.Body)
	}

	req, err := http.NewRequestWithContext(ctx, args.Method, args.URL, bodyReader)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("build request: %w", err)
	}
	// 设置请求头
	for k, v := range args.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" && args.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	// 可识别的 User-Agent（合规：不伪装，让目标站知道是爬虫）
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return entity.DataItem{}, fmt.Errorf("api returned status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	bodyStr := string(body)
	return entity.DataItem{
		ID:          fmt.Sprintf("crawl-%d", time.Now().UnixNano()),
		Title:       fmt.Sprintf("API采集: %s", truncate(args.URL, 80)),
		Content:     bodyStr,
		SourceURL:   args.URL,
		Metadata:    map[string]string{"crawler_type": "api"},
		RawContent:  bodyStr,
		Status:      entity.ItemStatusPendingReview,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// 编译期断言：确保实现 port.CrawlerTool（含全部 4 个方法）。
var _ port.CrawlerTool = (*APICrawler)(nil)

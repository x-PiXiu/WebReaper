package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gocolly/colly/v2"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// StaticCrawler 是静态网页爬虫（作为 Agent 工具）。
//
// 用 colly 抓取服务端渲染的 HTML，按 CSS 选择器提取内容。
// 适用于：传统网站、博客、论坛（非 SPA）。
// 现有 adapter/spider/GenericWebSpider 是用例直接调的版本，
// 这个是 Agent 工具版本（接口不同，但底层都用 colly）。
type StaticCrawler struct {
	robots *RobotsChecker
}

func NewStaticCrawler() *StaticCrawler {
	return &StaticCrawler{robots: NewRobotsChecker()}
}

// 编译期断言：确保实现 port.CrawlerTool（含全部 4 个方法）。
var _ port.CrawlerTool = (*StaticCrawler)(nil)

func (c *StaticCrawler) Name() string { return "static_crawler" }

func (c *StaticCrawler) Description() string {
	return "抓取静态网页（服务端渲染的 HTML），用 CSS 选择器提取内容。" +
		"适用于传统网站。输入参数：url（页面地址）、selectors（CSS选择器映射）。"
}

// staticCrawlerArgs LLM 传给工具的参数。
type staticCrawlerArgs struct {
	URL       string            `json:"url"`                 // 必填：页面 URL
	Selectors map[string]string `json:"selectors"`           // CSS 选择器映射（title/content/etc → selector）
}

// Execute 执行静态网页采集。
func (c *StaticCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args staticCrawlerArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.URL == "" {
		return entity.DataItem{}, fmt.Errorf("url is required")
	}

	// robots.txt 合规检查（受全局开关控制）
	if IsRespectRobots() && !c.robots.IsAllowed(ctx, args.URL) {
		return entity.DataItem{}, fmt.Errorf("robots.txt disallows crawling: %s", args.URL)
	}

	// 用 colly 抓取页面文本（简化版：取整页文本，LLM 再从中提取）
	collyInst := colly.NewCollector(colly.MaxDepth(1))
	var content string
	// 捕获 HTTP 状态码——colly 把 4xx/5xx 作为 Visit 错误返回，但同时会在
	// OnError 里给出 r.StatusCode。若不捕获，错误信息里会丢失数字状态码
	// （只剩 "Not Found" 之类文本）。OnError 与 OnResponse 二选一被触发。
	var statusCode int
	collyInst.OnHTML("body", func(e *colly.HTMLElement) {
		content = e.Text
	})
	collyInst.OnResponse(func(r *colly.Response) {
		statusCode = r.StatusCode
	})
	collyInst.OnError(func(r *colly.Response, err error) {
		statusCode = r.StatusCode
	})

	if err := collyInst.Visit(args.URL); err != nil {
		// colly 把 HTTP 4xx/5xx 也作为 Visit 错误返回（同时触发 OnError 设置 statusCode）。
		// 用 OnError 捕获到的状态码（若已设置），保留数字状态码到错误信息里。
		return entity.DataItem{}, crawlErr(args.URL, statusCode, err)
	}
	collyInst.Wait()

	// Visit 未报错但仍可能记录了非 2xx 状态码（防御性检查）
	if statusCode != 0 && (statusCode < 200 || statusCode >= 300) {
		return entity.DataItem{}, crawlErr(args.URL, statusCode, nil)
	}

	// 合规检查：不采集需要登录/付费的内容（不绕过认证）
	if IsLoginRequiredContent(args.URL, content) {
		return entity.DataItem{}, fmt.Errorf("该页面需要登录才能查看，WebReaper 不采集登录态内容（合规）")
	}
	if IsPaywallContent(args.URL, content) {
		return entity.DataItem{}, fmt.Errorf("该页面为付费墙内容，WebReaper 不采集付费内容（合规）")
	}

	return entity.DataItem{
		ID:          fmt.Sprintf("crawl-%d", time.Now().UnixNano()),
		Title:       fmt.Sprintf("网页采集: %s", truncate(args.URL, 80)),
		Content:     content,
		SourceURL:   args.URL,
		Metadata:    map[string]string{"crawler_type": "static"},
		RawContent:  content,
		Status:      entity.ItemStatusPendingReview,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (c *StaticCrawler) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "static_crawler",
		Description: c.Description(),
		Properties: map[string]port.PropSpec{
			"url":       {Type: "string", Description: "页面地址（必填）"},
			"selectors": {Type: "object", Description: "CSS选择器映射，如 {\"title\":\"h1\",\"content\":\".article\"}"},
		},
		Required: []string{"url"},
	}
}

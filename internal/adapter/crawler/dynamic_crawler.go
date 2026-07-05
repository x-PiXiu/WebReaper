package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// DynamicCrawler 是动态网页爬虫（chromedp 无头浏览器）。
//
// 适用于 JS 渲染的 SPA 网站（React/Vue/Angular）。
// 启动真实 Chrome 浏览器加载页面，等待 JS 执行完成后提取 DOM 内容。
// 比 static_crawler 慢（要启动浏览器），但能抓到 JS 渲染后的内容。
type DynamicCrawler struct {
	robots *RobotsChecker // 复用 checker，避免每次 Execute 都新建导致 robots.txt 缓存失效
}

func NewDynamicCrawler() *DynamicCrawler { return &DynamicCrawler{robots: NewRobotsChecker()} }

func (c *DynamicCrawler) Name() string { return "dynamic_crawler" }

func (c *DynamicCrawler) Description() string {
	return "抓取动态网页（JS 渲染的 SPA），用无头浏览器加载页面后提取内容。" +
		"适用于 React/Vue 等单页应用。输入参数：url（页面地址）、wait_selector（等待该元素出现后再提取，可选）、selector（提取内容的CSS选择器，默认取 body 文本）。"
}

// dynamicCrawlerArgs LLM 传给工具的参数。
type dynamicCrawlerArgs struct {
	URL          string `json:"url"`                    // 必填：页面 URL
	WaitSelector string `json:"wait_selector"`          // 等待该 CSS 选择器出现后再提取（可选）
	Selector     string `json:"selector"`               // 提取内容的 CSS 选择器（默认 body）
}

// Execute 执行动态网页采集。
func (c *DynamicCrawler) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args dynamicCrawlerArgs
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

	// 创建无头浏览器上下文
	allocCtx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(allocCtx, 30*time.Second)
	defer timeoutCancel()

	var content string
	actions := []chromedp.Action{
		chromedp.Navigate(args.URL),
	}

	// 等待指定元素出现（可选）
	if args.WaitSelector != "" {
		actions = append(actions, chromedp.WaitVisible(args.WaitSelector, chromedp.ByQuery))
	} else {
		// 默认等待 body 加载
		actions = append(actions, chromedp.Sleep(2*time.Second))
	}

	// 提取内容
	selector := args.Selector
	if selector == "" {
		selector = "body"
	}
	actions = append(actions, chromedp.Text(selector, &content, chromedp.ByQuery))

	if err := chromedp.Run(timeoutCtx, actions...); err != nil {
		return entity.DataItem{}, fmt.Errorf("chromedp run: %w", err)
	}

	// 截断过长的标题页
	title := args.URL
	if nodes, err := cdpNodeText(timeoutCtx, "title"); err == nil && len(nodes) > 0 {
		title = nodes
	}

	return entity.DataItem{
		ID:        fmt.Sprintf("crawl-%d", time.Now().UnixNano()),
		Title:     truncate(title, 200),
		Content:   content,
		SourceURL: args.URL,
		RawContent: content,
		Status:    entity.ItemStatusPendingReview,
		Metadata:  map[string]string{"crawler_type": "dynamic"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// cdpNodeText 辅助提取单个节点的文本。
func cdpNodeText(ctx context.Context, sel string) (string, error) {
	var text string
	err := chromedp.Run(ctx, chromedp.Text(sel, &text, chromedp.ByQuery))
	return text, err
}

// 编译期断言：确保实现 port.CrawlerTool（含全部 4 个方法）。
var _ port.CrawlerTool = (*DynamicCrawler)(nil)

func (c *DynamicCrawler) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "dynamic_crawler",
		Description: c.Description(),
		Properties: map[string]port.PropSpec{
			"url":            {Type: "string", Description: "页面地址（必填）"},
			"wait_selector":  {Type: "string", Description: "等待该CSS选择器出现后再提取（可选）"},
			"selector":       {Type: "string", Description: "提取内容的CSS选择器，默认body"},
		},
		Required: []string{"url"},
	}
}

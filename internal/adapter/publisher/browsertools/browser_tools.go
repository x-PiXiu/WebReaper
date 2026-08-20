// Package browsertools 提供 Agent 驱动的浏览器工具集（截屏→LLM→决策循环的执行层）。
//
// 设计动机（获客智能体转型——分发系统混合 Agent 架构）：
//   正常发布流程走 chromedp 代码（快、零成本）；异常时（弹窗/滑块/改版）
//   Agent 接管——视觉 LLM 看截屏、决策下一步、调用本工具集执行。
//
// 架构：
//   ┌─ 混合 Agent 循环 ─────────────────────────────┐
//   │ 1. chromedp 正常操作 → VerifySuccess() 检查    │
//   │    ├─ ✅ 通过 → 继续下一步                      │
//   │    └─ ❌ 失败 → Agent 接管：                     │
//   │        Screenshot → LLM 分析 → 选择工具 → 执行   │
//   │        → 再 Screenshot 验证 → 恢复正常流程        │
//   └───────────────────────────────────────────────┘
//
// 工具集实现 trpc-agent-go 的 tool.CallableTool 接口，
// 可直接注册进 LLM Agent 的工具列表。
package browsertools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// ---- 浏览器工具集（7 个原语 + 1 个滑块处理）----

// BrowserSession 封装一次浏览器会话——所有工具共享同一个 chromedp context。
type BrowserSession struct {
	ctx     context.Context
	lastURL string
}

// NewBrowserSession 创建浏览器会话（传入已初始化的 chromedp context）。
func NewBrowserSession(ctx context.Context) *BrowserSession {
	return &BrowserSession{ctx: ctx}
}

// ---- 截屏工具 ----

// ScreenshotTool 截取当前页面全屏截图（返回 base64 编码——视觉 LLM 直接可用）。
type ScreenshotTool struct {
	session *BrowserSession
}

func NewScreenshotTool(s *BrowserSession) *ScreenshotTool { return &ScreenshotTool{s} }

func (t *ScreenshotTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_screenshot",
		Description: "截取当前浏览器页面的全屏截图。用于观察页面当前状态、识别页面元素、验证操作结果。无需参数。",
		InputSchema: &tool.Schema{Type: "object"},
	}
}

func (t *ScreenshotTool) Call(ctx context.Context, _ []byte) (any, error) {
	var buf []byte
	if err := chromedp.Run(t.session.ctx, chromedp.FullScreenshot(&buf, 80)); err != nil {
		return nil, fmt.Errorf("截屏失败: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf)
	return map[string]any{
		"success":   true,
		"screenshot": encoded, // base64 PNG——直接发给视觉 LLM
		"url":       t.session.lastURL,
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

// ---- 点击工具 ----

// ClickTool 点击页面元素（按选择器或坐标）。
type ClickTool struct {
	session *BrowserSession
}

func NewClickTool(s *BrowserSession) *ClickTool { return &ClickTool{s} }

func (t *ClickTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_click",
		Description: "点击页面上的元素。提供 CSS 选择器（如 '#btn-submit' 或 '[data-testid=\"close\"]'）或坐标（x, y）。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"selector": {Type: "string", Description: "CSS 选择器（优先使用）"},
				"x":        {Type: "number", Description: "X 坐标（选择器找不到时用）"},
				"y":        {Type: "number", Description: "Y 坐标"},
			},
		},
	}
}

func (t *ClickTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		Selector string  `json:"selector"`
		X        float64 `json:"x"`
		Y        float64 `json:"y"`
	}
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	if args.Selector != "" {
		if err := chromedp.Run(t.session.ctx, chromedp.Click(args.Selector)); err != nil {
			return nil, fmt.Errorf("点击 %s 失败: %w", args.Selector, err)
		}
		// 等待页面响应
		time.Sleep(500 * time.Millisecond)
		return map[string]any{"success": true, "clicked": args.Selector}, nil
	}

	// 按坐标点击（用 JS 模拟，因为 chromedp 没有坐标点击原语）
	if args.X > 0 && args.Y > 0 {
		js := fmt.Sprintf(`
			const el = document.elementFromPoint(%f, %f);
			if (el) { el.click(); } else { throw new Error('no element at (%f, %f)'); }
		`, args.X, args.Y, args.X, args.Y)
		if err := chromedp.Run(t.session.ctx, chromedp.Evaluate(js, nil)); err != nil {
			return nil, fmt.Errorf("坐标点击失败: %w", err)
		}
		return map[string]any{"success": true, "clicked_at": fmt.Sprintf("(%.0f, %.0f)", args.X, args.Y)}, nil
	}

	return nil, fmt.Errorf("需要提供 selector 或坐标")
}

// ---- 输入工具 ----

// TypeTool 在输入框中输入文本。
type TypeTool struct {
	session *BrowserSession
}

func NewTypeTool(s *BrowserSession) *TypeTool { return &TypeTool{s} }

func (t *TypeTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_type",
		Description: "在输入框中输入文本。先清空已有内容再输入。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"selector": {Type: "string", Description: "CSS 选择器（input 或 textarea）"},
				"text":     {Type: "string", Description: "要输入的文本内容"},
			},
			Required: []string{"selector", "text"},
		},
	}
}

func (t *TypeTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		Selector string `json:"selector"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	// 清空已有内容
	clearJS := fmt.Sprintf(`
		const el = document.querySelector(%q);
		if (el) {
			el.focus();
			el.value = '';
			el.dispatchEvent(new Event('input', { bubbles: true }));
		}
	`, args.Selector)
	if err := chromedp.Run(t.session.ctx, chromedp.Evaluate(clearJS, nil)); err != nil {
		return nil, fmt.Errorf("清空输入框失败: %w", err)
	}

	// 输入新内容
	if err := chromedp.Run(t.session.ctx, chromedp.SendKeys(args.Selector, args.Text)); err != nil {
		return nil, fmt.Errorf("输入失败: %w", err)
	}

	return map[string]any{"success": true, "typed": args.Text}, nil
}

// ---- 上传工具 ----

// UploadTool 上传文件（视频/图片）。
type UploadTool struct {
	session *BrowserSession
}

func NewUploadTool(s *BrowserSession) *UploadTool { return &UploadTool{s} }

func (t *UploadTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_upload",
		Description: "上传本地文件到页面的文件上传控件（视频 .mp4 / 图片 .jpg 等）。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"selector":  {Type: "string", Description: "file input 的 CSS 选择器（通常为 input[type=file]）"},
				"file_path": {Type: "string", Description: "本地文件的绝对路径"},
			},
			Required: []string{"selector", "file_path"},
		},
	}
}

func (t *UploadTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		Selector string `json:"selector"`
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	if err := chromedp.Run(t.session.ctx, chromedp.SetUploadFiles(args.Selector, []string{args.FilePath})); err != nil {
		return nil, fmt.Errorf("上传文件失败: %w", err)
	}

	// 等待上传开始（UI 变化）
	time.Sleep(2 * time.Second)

	return map[string]any{"success": true, "uploaded": args.FilePath}, nil
}

// ---- 导航工具 ----

// NavigateTool 导航到指定 URL。
type NavigateTool struct {
	session *BrowserSession
}

func NewNavigateTool(s *BrowserSession) *NavigateTool { return &NavigateTool{s} }

func (t *NavigateTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_navigate",
		Description: "导航到指定 URL。等待页面加载完成。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"url": {Type: "string", Description: "目标 URL（含 https://）"},
			},
			Required: []string{"url"},
		},
	}
}

func (t *NavigateTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	if err := chromedp.Run(t.session.ctx, chromedp.Navigate(args.URL)); err != nil {
		return nil, fmt.Errorf("导航失败: %w", err)
	}

	t.session.lastURL = args.URL
	time.Sleep(2 * time.Second) // 等页面加载

	return map[string]any{"success": true, "navigated_to": args.URL}, nil
}

// ---- 等待工具 ----

// WaitTool 等待指定秒数（或等待元素出现）。
type WaitTool struct {
	session *BrowserSession
}

func NewWaitTool(s *BrowserSession) *WaitTool { return &WaitTool{s} }

func (t *WaitTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_wait",
		Description: "等待指定秒数（让页面加载/动画完成），或等待某个元素出现。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"seconds":  {Type: "number", Description: "等待秒数（1-30）"},
				"selector": {Type: "string", Description: "等待此元素出现（可选，优先于秒数）"},
			},
		},
	}
}

func (t *WaitTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		Seconds  float64 `json:"seconds"`
		Selector string  `json:"selector"`
	}
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	if args.Selector != "" {
		timeout := 10 * time.Second
		if args.Seconds > 0 {
			timeout = time.Duration(args.Seconds * float64(time.Second))
		}
		waitCtx, cancel := context.WithTimeout(t.session.ctx, timeout)
		defer cancel()
		if err := chromedp.Run(waitCtx, chromedp.WaitVisible(args.Selector)); err != nil {
			return map[string]any{"success": false, "error": fmt.Sprintf("元素 %s 未在 %.0f 秒内出现", args.Selector, timeout.Seconds())}, nil
		}
		return map[string]any{"success": true, "waited_for": args.Selector}, nil
	}

	if args.Seconds > 0 && args.Seconds <= 30 {
		time.Sleep(time.Duration(args.Seconds * float64(time.Second)))
		return map[string]any{"success": true, "waited_seconds": args.Seconds}, nil
	}

	return nil, fmt.Errorf("需要提供 seconds 或 selector")
}

// ---- 滚动工具 ----

// ScrollTool 滚动页面。
type ScrollTool struct {
	session *BrowserSession
}

func NewScrollTool(s *BrowserSession) *ScrollTool { return &ScrollTool{s} }

func (t *ScrollTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_scroll",
		Description: "滚动页面。direction 可为 down/up，pixels 为滚动像素数（默认 500）。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"direction": {Type: "string", Description: "down 或 up", Enum: []any{"down", "up"}},
				"pixels":    {Type: "number", Description: "滚动像素数（默认 500）"},
			},
		},
	}
}

func (t *ScrollTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		Direction string  `json:"direction"`
		Pixels    float64 `json:"pixels"`
	}
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	if args.Pixels <= 0 {
		args.Pixels = 500
	}
	if args.Direction == "" {
		args.Direction = "down"
	}

	dir := ""
	if args.Direction == "up" {
		dir = "-"
	}

	js := fmt.Sprintf(`window.scrollBy({ top: %s%d, behavior: 'smooth' })`, dir, int(args.Pixels))
	if err := chromedp.Run(t.session.ctx, chromedp.Evaluate(js, nil)); err != nil {
		return nil, fmt.Errorf("滚动失败: %w", err)
	}

	time.Sleep(1 * time.Second) // 等滚动动画
	return map[string]any{"success": true, "scrolled": args.Direction, "pixels": args.Pixels}, nil
}

// ---- 滑块验证工具（Agent 的第一个应用场景）----

// SliderTool 自动处理滑块验证码。
// 识别滑块位置 → 计算贝塞尔轨迹 → 模拟人类拖拽（带加速度变化）。
type SliderTool struct {
	session *BrowserSession
}

func NewSliderTool(s *BrowserSession) *SliderTool { return &SliderTool{s} }

func (t *SliderTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_solve_slider",
		Description: "自动处理滑块验证码。识别滑块轨道和拖块位置，用模拟人类拖拽的轨迹完成验证。适用于抖音/快手等平台的滑块验证。无需参数（自动检测页面上的滑块）。",
		InputSchema: &tool.Schema{Type: "object"},
	}
}

func (t *SliderTool) Call(ctx context.Context, _ []byte) (any, error) {
	// 第一步：检测页面是否有滑块
	var sliderInfo map[string]any
	detectJS := `
		(() => {
			// 通用滑块检测（兼容多种平台的选择器）
			const selectors = [
				'.nc_container .btn_slide',           // 阿里系
				'.slider-btn',                         // 通用
				'.verify-move-block',                  // 通用验证码
				'[class*="slider"] [class*="btn"]',    // 模糊匹配
				'[class*="drag"]',                     // 拖拽
				'.tcaptcha-drag-thumb',                // 腾讯
				'[class*="captcha"] [class*="slider"]', // 验证码滑块
			];
			for (const sel of selectors) {
				const btn = document.querySelector(sel);
				if (btn && btn.offsetWidth > 0) {
					const track = btn.closest('[class*="track"], [class*="container"], [class*="slider"]')
						|| btn.parentElement;
					if (track && track.offsetWidth > btn.offsetWidth) {
						const btnRect = btn.getBoundingClientRect();
						const trackRect = track.getBoundingClientRect();
						return {
							found: true,
							btnX: btnRect.x + btnRect.width / 2,
							btnY: btnRect.y + btnRect.height / 2,
							trackWidth: trackRect.width,
							btnWidth: btnRect.width,
							distance: trackRect.width - btnRect.width,
							selector: sel,
						};
					}
				}
			}
			return { found: false };
		})()
	`
	if err := chromedp.Run(t.session.ctx, chromedp.Evaluate(detectJS, &sliderInfo)); err != nil {
		return nil, fmt.Errorf("滑块检测失败: %w", err)
	}

	if sliderInfo == nil || sliderInfo["found"] != true {
		return map[string]any{"success": false, "message": "页面上没有检测到滑块验证"}, nil
	}

	btnX, _ := sliderInfo["btnX"].(float64)
	btnY, _ := sliderInfo["btnY"].(float64)
	distance, _ := sliderInfo["distance"].(float64)

	if distance <= 0 {
		return map[string]any{"success": false, "message": "滑块距离无效"}, nil
	}

	// 第二步：模拟人类拖拽（分段+变速+随机偏移）
	// chromedp 没有 mouse drag 原语——用 CDP 的 Input.dispatchMouseEvent 模拟
	steps := generateSliderTrajectory(btnX, btnY, distance)

	for _, step := range steps {
		js := fmt.Sprintf(`
			const btn = document.querySelector(%q);
			if (btn) {
				const ev = new MouseEvent('mousemove', {
					clientX: %f, clientY: %f, bubbles: true
				});
				btn.dispatchEvent(ev);
			}
		`, sliderInfo["selector"], step.x, step.y)
		_ = chromedp.Run(t.session.ctx, chromedp.Evaluate(js, nil))

		// 变速延迟：开始快→中间慢→结束快（人的拖拽习惯）
		d := time.Duration(step.delayMs) * time.Millisecond
		time.Sleep(d)
	}

	// 最后模拟 mouseup（松手）
	releaseJS := fmt.Sprintf(`
		const btn = document.querySelector(%q);
		if (btn) {
			btn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
		}
	`, sliderInfo["selector"])
	_ = chromedp.Run(t.session.ctx, chromedp.Evaluate(releaseJS, nil))

	time.Sleep(1 * time.Second) // 等验证结果

	return map[string]any{
		"success":  true,
		"dragged":  distance,
		"steps":    len(steps),
		"message":  "滑块拖拽完成（等待平台验证结果）",
	}, nil
}

// sliderStep 滑块拖拽轨迹的一步。
type sliderStep struct {
	x, y    float64
	delayMs int
}

// generateSliderTrajectory 生成人类拖拽滑块的轨迹。
// 特征：开始快速→中间减速→接近目标时缓慢对准→到达后停顿。
func generateSliderTrajectory(startX, startY, distance float64) []sliderStep {
	steps := make([]sliderStep, 0)
	totalSteps := int(distance / 5) // 每 5px 一步
	if totalSteps < 10 {
		totalSteps = 10
	}
	if totalSteps > 50 {
		totalSteps = 50
	}

	for i := 0; i <= totalSteps; i++ {
		progress := float64(i) / float64(totalSteps)

		// 速度曲线：ease-out（开始快，结束慢——人的拖拽习惯）
		// 使用 easeOutCubic: 1 - (1-t)^3
		eased := 1 - (1-progress)*(1-progress)*(1-progress)
		x := startX + distance*eased

		// Y 轴微小随机偏移（人的手不可能完全水平）
		y := startY + (float64((i*37)%10) - 5) * 0.5

		// 延迟：开始短（快）→ 结尾长（慢+对准）
		var delayMs int
		if progress < 0.3 {
			delayMs = 5 + i%5     // 开始阶段：5-10ms（快速启动）
		} else if progress < 0.8 {
			delayMs = 15 + i%10    // 中间阶段：15-25ms（稳定移动）
		} else {
			delayMs = 30 + i%20    // 结尾阶段：30-50ms（缓慢对准）
		}

		steps = append(steps, sliderStep{x: x, y: y, delayMs: delayMs})
	}

	// 最后加几步微调（模拟人的"精确对准"）
	for i := 0; i < 3; i++ {
		steps = append(steps, sliderStep{
			x:       startX + distance + float64(i)*0.5,
			y:       startY + float64(i%3-1)*0.3,
			delayMs: 50,
		})
	}

	return steps
}

// ---- 获取页面文本工具（轻量验证用——不截图，只读 DOM）----

// GetTextTool 获取指定元素的文本内容。
type GetTextTool struct {
	session *BrowserSession
}

func NewGetTextTool(s *BrowserSession) *GetTextTool { return &GetTextTool{s} }

func (t *GetTextTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_get_text",
		Description: "获取指定元素的文本内容。用于验证页面状态（如检查是否显示'发布成功'）。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"selector": {Type: "string", Description: "CSS 选择器"},
			},
			Required: []string{"selector"},
		},
	}
}

func (t *GetTextTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		Selector string `json:"selector"`
	}
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}

	var text string
	if err := chromedp.Run(t.session.ctx, chromedp.Text(args.Selector, &text)); err != nil {
		return map[string]any{"success": false, "error": fmt.Sprintf("元素 %s 不存在或无文本", args.Selector)}, nil
	}

	return map[string]any{"success": true, "text": strings.TrimSpace(text)}, nil
}

// ---- 完成工具（标记任务结束）----

// DoneTool 标记 Agent 任务完成。
type DoneTool struct {
	session *BrowserSession
}

func NewDoneTool(s *BrowserSession) *DoneTool { return &DoneTool{s} }

func (t *DoneTool) Declaration() *tool.Declaration {
	return &tool.Declaration{
		Name:        "browser_done",
		Description: "标记浏览器操作任务已完成。当目标已达成（如视频已发布/弹窗已关闭/滑块已通过）时调用此工具。",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"result": {Type: "string", Description: "任务结果描述（如'视频发布成功'）"},
			},
			Required: []string{"result"},
		},
	}
}

func (t *DoneTool) Call(_ context.Context, jsonArgs []byte) (any, error) {
	var args struct {
		Result string `json:"result"`
	}
	_ = json.Unmarshal(jsonArgs, &args)
	return map[string]any{"done": true, "result": args.Result}, nil
}

// ---- 工具集注册 ----

// AllTools 返回完整的浏览器工具集（注册进 trpc-agent-go 的 LLM Agent）。
func AllTools(session *BrowserSession) []tool.CallableTool {
	return []tool.CallableTool{
		NewScreenshotTool(session),
		NewClickTool(session),
		NewTypeTool(session),
		NewUploadTool(session),
		NewNavigateTool(session),
		NewWaitTool(session),
		NewScrollTool(session),
		NewSliderTool(session),
		NewGetTextTool(session),
		NewDoneTool(session),
	}
}

// AllDeclarations 返回所有工具的声明（传给 LLM 的 function definitions）。
func AllDeclarations(session *BrowserSession) []*tool.Declaration {
	tools := AllTools(session)
	decls := make([]*tool.Declaration, 0, len(tools))
	for _, t := range tools {
		decls = append(decls, t.Declaration())
	}
	return decls
}

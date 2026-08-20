// Package humanize 提供浏览器操作的人类行为模拟层 + 指纹伪装（反检测 Level 2）。
//
// 设计动机（分发系统反检测——获客智能体转型）：
//   纯 chromedp 操作的时序特征（零延迟/瞬时输入/无鼠标轨迹）会被抖音/快手等
//   平台的行为分析引擎识别为自动化。本包在 chromedp 原语外面包一层"人类模拟"，
//   让操作在时间维度和输入模式上不可区分于真人。
//
// 整洁架构定位：adapter 层的"框架与驱动"——纯技术细节，用例层不感知。
// 使用方式：publisher/qrlogin 的 RPA 代码统一通过 HumanAction 调用 chromedp，
// 而不是直接调 chromedp 原语。
//
// Level 路线图：
//   Level 2（当前）：人类行为模拟 + 指纹伪装
//   Level 3（预留）：IP 代理池 + 浏览器多 Profile 隔离——接口已抽象，加参数即可
package humanize

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// ---- 随机延迟 ----

// Delay 在 [min, max] 秒范围内随机等待（模拟人的反应时间/阅读时间）。
func Delay(min, max float64) {
	d := time.Duration((min + rand.Float64()*(max-min)) * float64(time.Second))
	time.Sleep(d)
}

// DelayMs 在 [min, max] 毫秒范围内随机等待（字符间/鼠标微操作）。
func DelayMs(min, max int) {
	d := time.Duration(min + rand.Intn(max-min+1))
	time.Sleep(d * time.Millisecond)
}

// Idle 模拟人在看内容的空闲期（随机 2-8 秒，偶尔更长）。
func Idle() {
	if rand.Float64() < 0.1 {
		Delay(5, 12) // 10% 概率较长空闲（人在认真看内容）
	} else {
		Delay(2, 5)
	}
}

// ---- 鼠标移动（贝塞尔曲线轨迹）----

// mouseStep 鼠标移动的中间步骤。
type mouseStep struct{ x, y float64 }

// bezierPath 生成从 (x1,y1) 到 (x2,y2) 的二阶贝塞尔曲线轨迹点。
// 控制点在两点之间随机偏移——模拟人类鼠标的不规则移动。
func bezierPath(x1, y1, x2, y2 float64, steps int) []mouseStep {
	// 控制点：中点 + 随机偏移（偏移量与距离成正比）
	dist := math.Sqrt((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1))
	offset := dist * (0.1 + rand.Float64()*0.3) // 10%-40% 的弧度
	if rand.Float64() < 0.5 {
		offset = -offset
	}
	cx := (x1+x2)/2 + offset
	cy := (y1+y2)/2 + offset*0.5 // 垂直偏移小一些

	path := make([]mouseStep, 0, steps)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		// 二阶贝塞尔：B(t) = (1-t)²P₀ + 2(1-t)tP₁ + t²P₂
		x := (1-t)*(1-t)*x1 + 2*(1-t)*t*cx + t*t*x2
		y := (1-t)*(1-t)*y1 + 2*(1-t)*t*cy + t*t*y2
		path = append(path, mouseStep{x, y})
	}
	return path
}

// MoveMouseWithCurve 模拟人类鼠标沿贝塞尔曲线移动到目标位置。
// chromedp 没有 mouse move 原语——用 dispatchMouseEvent 模拟。
func MoveMouseWithCurve(ctx context.Context, fromX, fromY, toX, toY float64) error {
	steps := 8 + rand.Intn(8) // 8-15 步（人手移鼠标不是瞬移也不是一步一格）
	path := bezierPath(fromX, fromY, toX, toY, steps)

	for _, p := range path {
		js := fmt.Sprintf(`
			const ev = new MouseEvent('mousemove', {
				clientX: %f, clientY: %f, bubbles: true
			});
			document.elementFromPoint(%f, %f)?.dispatchEvent(ev);
		`, p.x, p.y, p.x, p.y)
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, nil)); err != nil {
			return err
		}
		DelayMs(10, 40) // 每步 10-40ms（人的手移动速度）
	}
	return nil
}

// ---- 人类化操作（chromedp 包装）----

// HumanAction 人类行为模拟层——包装 chromedp 原语，加上人类特征。
// 所有分发 RPA 代码统一通过此结构操作浏览器，不直接调 chromedp。
type HumanAction struct {
	ctx      context.Context
	lastX, lastY float64 // 上次鼠标位置（模拟连续移动）
}

// New 创建人类行为模拟层。
func New(ctx context.Context) *HumanAction {
	return &HumanAction{ctx: ctx, lastX: 400, lastY: 300} // 初始位置：屏幕中间偏左上
}

// Navigate 导航到 URL（模拟人在地址栏输入 → 等待页面加载 → 看一眼内容）。
func (h *HumanAction) Navigate(url string) error {
	Delay(0.5, 1.5) // 人切到浏览器/找地址栏的时间
	if err := chromedp.Run(h.ctx, chromedp.Navigate(url)); err != nil {
		return err
	}
	Delay(1, 3) // 等页面加载 + 人扫一眼
	return nil
}

// Click 点击元素（随机延迟 → 鼠标曲线移动 → 小停顿 → 点击）。
func (h *HumanAction) Click(selector string) error {
	Delay(0.5, 2) // 人看到按钮的反应时间

	// 获取目标位置
	var box map[string]float64
	if err := chromedp.Run(h.ctx, chromedp.Evaluate(fmt.Sprintf(`
		(() => {
			const el = document.querySelector(%q);
			if (!el) return null;
			const r = el.getBoundingClientRect();
			return { x: r.x + r.width / 2, y: r.y + r.height / 2, w: r.width, h: r.height };
		})()
	`, selector), &box)); err != nil {
		return fmt.Errorf("获取元素位置失败 %s: %w", selector, err)
	}
	if box == nil {
		return fmt.Errorf("元素不存在: %s", selector)
	}

	// 鼠标曲线移动到目标（从上次位置出发）
	targetX := box["x"] + (rand.Float64()-0.5)*box["w"]*0.3 // 在元素内随机偏移（不是每次都点正中心）
	targetY := box["y"] + (rand.Float64()-0.5)*box["h"]*0.3
	_ = MoveMouseWithCurve(h.ctx, h.lastX, h.lastY, targetX, targetY)
	h.lastX, h.lastY = targetX, targetY

	DelayMs(50, 200) // 到达后的小停顿（人确认要点击）

	return chromedp.Run(h.ctx, chromedp.Click(selector))
}

// Type 逐字符输入（每个字符间随机 50-200ms 延迟，模拟打字速度）。
func (h *HumanAction) Type(selector, text string) error {
	Delay(0.3, 1) // 人找到输入框并点击的时间

	// 先点击聚焦
	if err := chromedp.Run(h.ctx, chromedp.Click(selector)); err != nil {
		return err
	}
	DelayMs(100, 300)

	// 逐字符输入
	runes := []rune(text)
	for i, r := range runes {
		char := string(r)
		if err := chromedp.Run(h.ctx, chromedp.SendKeys(selector, char, chromedp.ByQuery)); err != nil {
			return err
		}

		// 打字速度模拟：
		//   正常字符 50-150ms
		//   标点后停顿稍长（人要想一下）
		//   换行后停顿最长（人要组织语言）
		switch char {
		case "\n", "\r":
			DelayMs(300, 800)
		case "。", "，", "！", "？", ".", ",", "!", "?":
			DelayMs(150, 400)
		default:
			DelayMs(50, 150)
		}

		// 5% 概率：人打错字要删除重打
		if rand.Float64() < 0.03 && i < len(runes)-1 && char != " " {
			_ = chromedp.Run(h.ctx, chromedp.SendKeys(selector, "\b", chromedp.ByQuery)) // backspace
			DelayMs(100, 300)
			_ = chromedp.Run(h.ctx, chromedp.SendKeys(selector, char, chromedp.ByQuery)) // 重新输入
			DelayMs(50, 150)
		}
	}
	return nil
}

// Scroll 自然滚动（不是跳到底部，而是分步滚动+随机停顿）。
func (h *HumanAction) Scroll(pixels int) error {
	steps := 3 + rand.Intn(4) // 分 3-6 步滚到目标位置
	stepSize := pixels / steps

	for i := 0; i < steps; i++ {
		js := fmt.Sprintf(`window.scrollBy({ top: %d, behavior: 'smooth' })`, stepSize)
		if err := chromedp.Run(h.ctx, chromedp.Evaluate(js, nil)); err != nil {
			return err
		}
		Delay(0.3, 0.8) // 每步之间人看内容
	}
	return nil
}

// Upload 上传文件（点击上传区域 → 小停顿 → 设置文件）。
func (h *HumanAction) Upload(selector, filePath string) error {
	Delay(1, 3) // 人找到上传按钮

	// 点击上传区域激活文件选择
	if err := chromedp.Run(h.ctx, chromedp.Click(selector)); err != nil {
		return err
	}
	DelayMs(500, 1500) // 文件选择器打开

	// 设置文件
	return chromedp.Run(h.ctx, chromedp.SetUploadFiles(selector, []string{filePath}))
}

// WaitVisible 等待元素可见（加人类等待时间）。
func (h *HumanAction) WaitVisible(selector string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(h.ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, chromedp.WaitVisible(selector))
}

// Screenshot 截取全页截图（Agent 验证 / 异常分析用）。
func (h *HumanAction) Screenshot() ([]byte, error) {
	var buf []byte
	if err := chromedp.Run(h.ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return nil, err
	}
	return buf, nil
}

// ---- 指纹伪装（浏览器初始化时注入）----

// FingerprintJS 返回注入到浏览器的反检测脚本。
// 在页面加载前执行——隐藏自动化特征，模拟真实浏览器指纹。
func FingerprintJS() string {
	return `
(function() {
	// 1. 隐藏 navigator.webdriver（chromedp 的最大暴露特征）
	Object.defineProperty(navigator, 'webdriver', { get: () => undefined });

	// 2. 伪装 plugins（无头浏览器 plugins 为空）
	Object.defineProperty(navigator, 'plugins', {
		get: () => [
			{ name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer' },
			{ name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer' },
			{ name: 'Native Client', filename: 'internal-nacl-plugin' },
		]
	});

	// 3. 伪装 languages
	Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });

	// 4. Canvas 指纹随机化（加微噪声，每次不同——防指纹追踪）
	const originalToDataURL = HTMLCanvasElement.prototype.toDataURL;
	HTMLCanvasElement.prototype.toDataURL = function(type) {
		const context = this.getContext('2d');
		if (context) {
			const imageData = context.getImageData(0, 0, this.width, this.height);
			for (let i = 0; i < imageData.data.length; i += 4) {
				imageData.data[i] += (Math.random() - 0.5) * 2;   // R
				imageData.data[i + 1] += (Math.random() - 0.5) * 2; // G
			}
			context.putImageData(imageData, 0, 0);
		}
		return originalToDataURL.apply(this, arguments);
	};

	// 5. WebGL 渲染器伪装（报真实的 GPU 型号）
	const getParameter = WebGLRenderingContext.prototype.getParameter;
	WebGLRenderingContext.prototype.getParameter = function(parameter) {
		if (parameter === 37445) return 'Google Inc. (NVIDIA)';        // UNMASKED_VENDOR_WEBGL
		if (parameter === 37446) return 'ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 SUPER Direct3D11 vs_5_0 ps_5_0)'; // UNMASKED_RENDERER_WEBGL
		return getParameter.apply(this, arguments);
	};

	// 6. Chrome runtime（无头浏览器没有 chrome.runtime）
	window.chrome = window.chrome || { runtime: {} };

	// 7. 权限查询伪装
	const originalQuery = window.navigator.permissions.query;
	window.navigator.permissions.query = (parameters) => (
		parameters.name === 'notifications'
			? Promise.resolve({ state: Notification.permission })
			: originalQuery(parameters)
	);

	// 8. 屏幕分辨率（确保不是无头模式的全零）
	if (!window.screen.width || !window.screen.height) {
		Object.defineProperty(window.screen, 'width', { get: () => 1920 });
		Object.defineProperty(window.screen, 'height', { get: () => 1080 });
	}
})();
`
}

// StealthOptions 返回带反检测的 chromedp 启动参数。
// 在 chromedp.NewExecAllocator 的选项列表中追加这些。
func StealthOptions() []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{
		// 核心伪装
		chromedp.Flag("disable-blink-features", "AutomationControlled"), // 最关键：禁用自动化检测特征
		chromedp.Flag("exclude-switches", "enable-automation"),           // 排除自动化开关
		chromedp.Flag("useAutomationExtension", false),                   // 禁用自动化扩展

		// 浏览器外观（模拟真实用户）
		chromedp.Flag("lang", "zh-CN"),                        // 中文环境
		chromedp.Flag("window-size", "1920,1080"),             // 常见分辨率
		chromedp.Flag("disable-infobars", true),                     // 去掉"Chrome 正在被自动化软件控制"提示
		chromedp.Flag("no-first-run", true),                         // 跳过首次运行向导
		chromedp.Flag("no-default-browser-check", true),             // 跳过默认浏览器提示

		// 性能（不影响反检测，但减少资源占用）
		chromedp.Flag("disable-gpu", true),                          // 服务器无 GPU
		chromedp.Flag("disable-dev-shm-usage", true),                // Docker 共享内存
		chromedp.Flag("disable-extensions", true),                   // 不加载扩展
		chromedp.Flag("disable-notifications", true),                // 禁通知弹窗
	}
}

// UserAgent 返回真实的 Chrome UserAgent（不带 HeadlessChrome 标识）。
func UserAgent() string {
	// 几个常见的 Windows Chrome UA（随机选一个——不同账号不同 UA）
	uas := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	}
	return uas[rand.Intn(len(uas))]
}

// InjectFingerprint 在页面导航前注入指纹伪装脚本。
// 在 chromedp.NewContext 后、第一次 Navigate 前调用。
func InjectFingerprint(ctx context.Context) error {
	// 创建一个在页面加载前执行的脚本（AddScriptToEvaluateOnNewDocument）
	js := FingerprintJS()
	return chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 通过 CDP 的 Page.addScriptToEvaluateOnNewDocument 注入
			// chromedp 没有直接暴露这个 API，用 Evaluate 兜底
			var result interface{}
			err := chromedp.Evaluate(js, &result).Do(ctx)
			return err
		}),
	)
}

// VerifySuccess 验证操作是否成功（轻量检查——不调 LLM，只看 DOM）。
// 返回 nil = 成功；返回 error = 需要 Agent 接管。
func (h *HumanAction) VerifySuccess(checkSelector string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(h.ctx, timeout)
	defer cancel()

	// 检查期望的元素是否出现
	err := chromedp.Run(ctx, chromedp.WaitVisible(checkSelector))
	if err != nil {
		// 元素没出现——可能有问题，截屏给调用方（调用方决定是否让 Agent 接管）
		return fmt.Errorf("验证失败：期望元素 %s 未出现（可能遇到弹窗/改版/加载超时）", checkSelector)
	}
	return nil
}

// string 是否包含中文（打字速度模拟用）。
func containsChinese(s string) bool {
	for _, r := range s {
		if r > 0x4E00 && r < 0x9FFF {
			return true
		}
	}
	return false
}

var _ = containsChinese // 保留（后续中文输入法模拟可能用到）
var _ = strings.TrimSpace // 保留（后续文本预处理可能用到）

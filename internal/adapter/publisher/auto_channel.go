package publisher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 全自动发布通道（chromedp RPA）----
//
// 整洁架构要点：
//   - 实现 port.AutoPublishChannel 可选接口（不修改现有 PublishChannel）
//   - 浏览器自动化细节全部关在此文件，用例层零感知
//   - 每个平台一个 struct，共用 cookie 注入和浏览器启动逻辑
//
// 【暂缓·P4 平台定位探路】"添加定位"（POI 挂载）未实现：
//   - 抖音/小红书发布时模拟"添加定位"（发布页点定位 → 搜索地址 → 选中）
//   - 状态：仅标记，不做。原因：视频+定位是平台风控重点区；抖音 POI 挂载官方
//     通道需"抖音来客/企业号 + 本地生活服务商资质"（资质问题非技术问题）。
//   - 完善路径：先人工验证（内测账号手操 10 次记录风控表现）→ 再在 PublishAuto
//     发布流程末尾追加定位步骤（地址来自 job.StoreAddress，字段已落库）；业务侧
//     零改动（PublishJob.StoreAddress 已就位，见 usecase/account/account.go）。
//   - 半自动兜底：前端"分发中心"提供定位操作指引（手动选定位，最稳）。

// allocOpts 反检测浏览器选项（与 qrlogin 模块一致）
func allocOpts() []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.WindowSize(1280, 800),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
		// 不设 chromedp.Headless = 显示模式（用户可见浏览器窗口）
	}
}

// parseCookies 将 cookie 字符串解析为 chromedp CookieParam 列表。
func parseCookies(cookieStr, domain string) []*network.CookieParam {
	var cookies []*network.CookieParam
	for _, part := range strings.Split(cookieStr, "; ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx < 0 {
			continue
		}
		cookies = append(cookies, &network.CookieParam{
			Name:   part[:idx],
			Value:  part[idx+1:],
			Domain: domain,
			Path:   "/",
		})
	}
	return cookies
}

// ZhihuAutoChannel 知乎全自动发布通道。
// 同时实现 PublishChannel（半自动）和 AutoPublishChannel（全自动）。
type ZhihuAutoChannel struct{}

var _ port.PublishChannel = (*ZhihuAutoChannel)(nil)
var _ port.AutoPublishChannel = (*ZhihuAutoChannel)(nil)

func NewZhihuAutoChannel() *ZhihuAutoChannel { return &ZhihuAutoChannel{} }

func (c *ZhihuAutoChannel) Platform() string           { return "zhihu" }
func (c *ZhihuAutoChannel) SupportedMediaType() []string { return []string{"text"} }

// PublishSemiAuto 半自动模式：返回知乎写文章页 URL
func (c *ZhihuAutoChannel) PublishSemiAuto(_ context.Context, job entity.PublishJob, _ entity.Account) (string, error) {
	u := "https://zhuanlan.zhihu.com/write"
	if job.Title != "" {
		u += "?title=" + job.Title
	}
	return u, nil
}

// PublishAuto 全自动发布到知乎专栏。
func (c *ZhihuAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookieStr string) (string, error) {
	log.Printf("[PublishAuto:zhihu] 开始全自动发布，标题=%q", job.Title)

	cookies := parseCookies(cookieStr, ".zhihu.com")

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts()...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 3*time.Minute)
	defer func() {
		sessionCancel()
		browserCancel()
		allocCancel()
	}()

	// ⚠️ 首次 chromedp.Run + cookie 注入 + 导航必须用同一个 context（sessionCtx）
	// 不能创建临时 context（cookieCtx）做首次 Run——cancel 后 Tab 会被关闭（和 qrlogin 同样的坑）
	var currentURL string
	err := chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(cookies),
		chromedp.Navigate("https://zhuanlan.zhihu.com/write"),
		chromedp.Sleep(3*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return "", fmt.Errorf("导航到写文章页失败: %w", err)
	}
	log.Printf("[PublishAuto:zhihu] cookie已注入，已导航到: %s", currentURL)

	// 如果被重定向到登录页，说明 cookie 失效
	if strings.Contains(currentURL, "signin") || strings.Contains(currentURL, "login") {
		return "", fmt.Errorf("cookie已过期，请重新绑定账号")
	}

	// 3. 填充标题
	err = chromedp.Run(sessionCtx,
		chromedp.WaitVisible(`textarea[placeholder*="标题"], input[placeholder*="标题"], .PostEditor-titleInput`, chromedp.ByQuery),
		chromedp.SendKeys(`textarea[placeholder*="标题"], input[placeholder*="标题"], .PostEditor-titleInput`, job.Title, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		return "", fmt.Errorf("填充标题失败: %w", err)
	}
	log.Printf("[PublishAuto:zhihu] 标题已填充")

	// 4. 填充正文（知乎用 contenteditable div / DraftJS 编辑器）
	// execCommand('insertText') 对长文本不可靠（有长度限制 + DraftJS 拦截）
	// 改用：先 focus + selectAll 清空，再用 execCommand 逐段插入（每段 500 字符）
	fillContentJS := fmt.Sprintf(`(() => {
		const editors = document.querySelectorAll('[contenteditable="true"], .public-DraftEditor-content, .ProseMirror');
		for (const editor of editors) {
			if (editor.offsetParent !== null) {
				editor.focus();
				// 清空编辑器
				document.execCommand('selectAll', false, null);
				document.execCommand('delete', false, null);

				// 分段插入正文（每段 500 字符，避免 insertText 长度限制）
				const content = %q;
				const chunkSize = 500;
				for (let i = 0; i < content.length; i += chunkSize) {
					const chunk = content.slice(i, i + chunkSize);
					document.execCommand('insertText', false, chunk);
				}
				return true;
			}
		}
		return false;
	})()`, job.Content)
	var filled bool
	err = chromedp.Run(sessionCtx,
		chromedp.Evaluate(fillContentJS, &filled),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil || !filled {
		return "", fmt.Errorf("填充正文失败: %w (filled=%v)", err, filled)
	}
	log.Printf("[PublishAuto:zhihu] 正文已填充")

	// 5. 点击发布按钮
	clickPublishJS := `(() => {
		const btns = document.querySelectorAll('button');
		for (const btn of btns) {
			if (btn.textContent.trim() === '发布' && btn.offsetParent !== null) {
				btn.click();
				return true;
			}
		}
		return false;
	})()`
	var clicked bool
	err = chromedp.Run(sessionCtx,
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(clickPublishJS, &clicked),
	)
	if err != nil || !clicked {
		return "", fmt.Errorf("点击发布按钮失败: %w (clicked=%v)", err, clicked)
	}
	log.Printf("[PublishAuto:zhihu] 发布按钮已点击")

	// 6. 等待跳转获取文章 URL
	var articleURL string
	err = chromedp.Run(sessionCtx,
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&articleURL),
	)
	if err != nil {
		return "", fmt.Errorf("获取文章URL失败: %w", err)
	}

	if strings.Contains(articleURL, "/p/") {
		log.Printf("[PublishAuto:zhihu] 发布成功！文章URL: %s", articleURL)
		return articleURL, nil
	}
	return "", fmt.Errorf("发布可能失败，当前URL: %s", articleURL)
}

// XiaohongshuAutoChannel 小红书全自动发布通道。
// 同时实现 PublishChannel（半自动）和 AutoPublishChannel（全自动）。
type XiaohongshuAutoChannel struct{}

var _ port.PublishChannel = (*XiaohongshuAutoChannel)(nil)
var _ port.AutoPublishChannel = (*XiaohongshuAutoChannel)(nil)

func NewXiaohongshuAutoChannel() *XiaohongshuAutoChannel { return &XiaohongshuAutoChannel{} }

func (c *XiaohongshuAutoChannel) Platform() string           { return "xiaohongshu" }
func (c *XiaohongshuAutoChannel) SupportedMediaType() []string { return []string{"text", "image"} }

// PublishSemiAuto 半自动模式：返回小红书发布页 URL
func (c *XiaohongshuAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://creator.xiaohongshu.com/publish/publish", nil
}

// PublishAuto 全自动发布到小红书。
func (c *XiaohongshuAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookieStr string) (string, error) {
	log.Printf("[PublishAuto:xiaohongshu] 开始全自动发布，标题=%q", job.Title)

	cookies := parseCookies(cookieStr, ".xiaohongshu.com")

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts()...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 3*time.Minute)
	defer func() {
		sessionCancel()
		browserCancel()
		allocCancel()
	}()

	// 首次 Run：cookie 注入 + 导航（同一 context，避免 Tab 被关闭）
	var currentURL string
	err := chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(cookies),
		chromedp.Navigate("https://creator.xiaohongshu.com/publish/publish"),
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return "", fmt.Errorf("导航到发布页失败: %w", err)
	}
	log.Printf("[PublishAuto:xiaohongshu] cookie已注入，已导航到: %s", currentURL)

	if strings.Contains(currentURL, "login") {
		return "", fmt.Errorf("cookie已过期，请重新绑定账号")
	}

	// 选择"上传图文"tab
	clickTabJS := `(() => {
		const els = document.querySelectorAll('div, span, li, button');
		for (const el of els) {
			if (el.textContent.includes('上传图文') && el.offsetParent !== null) {
				el.click();
				return true;
			}
		}
		return false;
	})()`
	var tabClicked bool
	_ = chromedp.Run(sessionCtx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(clickTabJS, &tabClicked),
		chromedp.Sleep(2*time.Second),
	)
	log.Printf("[PublishAuto:xiaohongshu] 选择上传图文: %v", tabClicked)

	// 填充标题
	err = chromedp.Run(sessionCtx,
		chromedp.WaitVisible(`input[placeholder*="标题"], #title, [class*="title"] input`, chromedp.ByQuery),
		chromedp.SendKeys(`input[placeholder*="标题"], #title, [class*="title"] input`, job.Title, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		return "", fmt.Errorf("填充标题失败: %w", err)
	}
	log.Printf("[PublishAuto:xiaohongshu] 标题已填充")

	// 填充正文
	fillContentJS := fmt.Sprintf(`(() => {
		const editors = document.querySelectorAll('[contenteditable="true"], textarea[placeholder*="正文"], [class*="content"] textarea, [class*="desc"] textarea');
		for (const editor of editors) {
			if (editor.offsetParent !== null) {
				editor.focus();
				editor.value = %q;
				editor.dispatchEvent(new Event('input', {bubbles: true}));
				return true;
			}
		}
		return false;
	})()`, job.Content)
	var filled bool
	err = chromedp.Run(sessionCtx,
		chromedp.Evaluate(fillContentJS, &filled),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil || !filled {
		log.Printf("[PublishAuto:xiaohongshu] 正文填充可能失败: %v (filled=%v)", err, filled)
	} else {
		log.Printf("[PublishAuto:xiaohongshu] 正文已填充")
	}

	// 点击发布按钮
	clickPublishJS := `(() => {
		const btns = document.querySelectorAll('button');
		for (const btn of btns) {
			const text = btn.textContent.trim();
			if ((text === '发布' || text === '发布笔记') && btn.offsetParent !== null && !btn.disabled) {
				btn.click();
				return true;
			}
		}
		return false;
	})()`
	var clicked bool
	err = chromedp.Run(sessionCtx,
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(clickPublishJS, &clicked),
	)
	if err != nil || !clicked {
		return "", fmt.Errorf("点击发布按钮失败: %w (clicked=%v)", err, clicked)
	}
	log.Printf("[PublishAuto:xiaohongshu] 发布按钮已点击")

	// 等待发布完成
	var articleURL string
	err = chromedp.Run(sessionCtx,
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&articleURL),
	)
	if err != nil {
		return "", fmt.Errorf("获取发布结果失败: %w", err)
	}

	log.Printf("[PublishAuto:xiaohongshu] 发布完成，URL: %s", articleURL)
	return articleURL, nil
}

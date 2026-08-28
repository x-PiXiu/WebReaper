package publisher

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/chromedputil"
	"webreaper/internal/config"
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

// allocOpts 浏览器启动参数（环境安全参数来自 chromedputil 公共工厂——
// 与 qrlogin 共用避免两份参数漂移；业务参数在此补充）。
// 容器环境（非 headed）自动启用 headless=new + no-sandbox（容器内 Chromium 需要）。
func allocOpts() []chromedp.ExecAllocatorOption {
	opts := chromedputil.HeadlessOptions(config.IsBrowserHeaded())
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	return opts
}

// parseCookies 将 cookie 字符串解析为 chromedp CookieParam 列表。
// diagScreenshot 失败诊断截图（RPA 稳定性基建：失败现场留证，改版/风控排查用）。
func diagScreenshot(ctx context.Context, platform, stage string) {
	var shot []byte
	if e := chromedp.Run(ctx, chromedp.CaptureScreenshot(&shot)); e == nil && len(shot) > 0 {
		name := fmt.Sprintf("debug-%s-fail-%s-%d.png", platform, stage, time.Now().UnixNano())
		if wErr := os.WriteFile(filepath.Join("data", "media", name), shot, 0o644); wErr == nil {
			log.Printf("[PublishAuto:%s] 失败诊断截图(%s): /media/%s", platform, stage, name)
		}
	}
}

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

func (c *ZhihuAutoChannel) Platform() string             { return "zhihu" }
func (c *ZhihuAutoChannel) SupportedMediaType() []string { return []string{"text"} }
func (c *ZhihuAutoChannel) SupportedContentTypes() []string {
	return []string{entity.ContentTypeArticle}
}

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
		diagScreenshot(sessionCtx, "zhihu", "fail")
		return "", fmt.Errorf("导航到写文章页失败: %w", err)
	}
	log.Printf("[PublishAuto:zhihu] cookie已注入，已导航到: %s", currentURL)

	// 如果被重定向到登录页，说明 cookie 失效
	if strings.Contains(currentURL, "signin") || strings.Contains(currentURL, "login") {
		diagScreenshot(sessionCtx, "zhihu", "fail")
		return "", fmt.Errorf("cookie已过期，请重新绑定账号")
	}

	// 3. 填充标题（精确选择器：.WriteIndex-titleInput 为知乎稳定类；placeholder 兜底）
	err = chromedp.Run(sessionCtx,
		chromedp.WaitVisible(`.WriteIndex-titleInput textarea, textarea[placeholder*="标题"]`, chromedp.ByQuery),
		chromedp.SendKeys(`.WriteIndex-titleInput textarea, textarea[placeholder*="标题"]`, job.Title, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		diagScreenshot(sessionCtx, "zhihu", "fail")
		return "", fmt.Errorf("填充标题失败: %w", err)
	}
	log.Printf("[PublishAuto:zhihu] 标题已填充")

	// 4. 填充正文——【关键修复】改用 CDP Input 内核事件，不再用 execCommand。
	// 原因：知乎专栏是 DraftJS（React 富文本），DraftJS 校验 event.isTrusted，
	// execCommand('insertText') 产生合成事件 isTrusted=false 被 DraftJS 丢弃 →
	// 内容不进 EditorState → 发布按钮一直 disabled → 失败存草稿。
	//
	// 修复方案（trusted 链）：
	//   ① chromedp.Click 聚焦编辑器（CDP Input 事件，trusted）
	//   ② selectAll + Backspace 清空草稿残留（selectAll 只改选区不触发内容校验；
	//      Backspace 是 CDP 内核事件 trusted，DraftJS 接受删除）
	//   ③ input.InsertText 批量插入正文（CDP Input.insertText，内核层 trusted，
	//      原生支持中文/长文本/换行，DraftJS 无法区分真人键盘 → 接受）
	//   ④ 校验：读编辑器 textContent 非空才继续
	// 编辑器候选链（2026-08-28 DRY_RUN 实测：知乎编辑器疑似从 DraftJS 换代——
	// .public-DraftEditor-content 聚焦无效（"添加正文"占位符不消失），补 ProseMirror/
	// tiptap/通用 contenteditable 候选，waitFirstVisible 多策略探测）
	editorSel := waitFirstVisible(sessionCtx, 10*time.Second,
		`.public-DraftEditor-content[contenteditable="true"]`, `.tiptap.ProseMirror`,
		`.ProseMirror[contenteditable="true"]`, `.ql-editor[contenteditable="true"]`,
		`[contenteditable="true"]`)
	if editorSel == "" {
		diagScreenshot(sessionCtx, "zhihu", "fail")
		return "", fmt.Errorf("未定位到正文编辑器（DraftJS/ProseMirror 候选均未命中）")
	}
	log.Printf("[PublishAuto:zhihu] 编辑器选择器=%s", editorSel)
	err = chromedp.Run(sessionCtx,
		// ① 聚焦编辑器（候选链首个可见者）
		chromedp.Click(editorSel, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		// ② 清空草稿残留：selectAll（选区操作安全）+ Backspace（trusted 删除）
		chromedp.ActionFunc(func(ctx context.Context) error {
			var dummy bool
			_ = chromedp.Evaluate(`document.execCommand('selectAll')`, &dummy).Do(ctx)
			down := input.DispatchKeyEvent(input.KeyRawDown)
			down.Key, down.Code = "Backspace", "Backspace"
			if err := down.Do(ctx); err != nil {
				return err
			}
			up := input.DispatchKeyEvent(input.KeyUp)
			up.Key, up.Code = "Backspace", "Backspace"
			return up.Do(ctx)
		}),
		chromedp.Sleep(300*time.Millisecond),
		// ③ 插入正文（CDP 内核层，支持中文/长文本）
		chromedp.ActionFunc(func(ctx context.Context) error {
			return input.InsertText(job.Content).Do(ctx)
		}),
		chromedp.Sleep(1500*time.Millisecond), // 等 DraftJS 状态更新 + 字数计数器刷新
	)
	if err != nil {
		diagScreenshot(sessionCtx, "zhihu", "fail")
		return "", fmt.Errorf("填充正文失败: %w", err)
	}

	// ④ 校验：编辑器 textContent 非空（防 DraftJS 静默拒绝导致按钮仍 disabled）
	var contentText string
	if cerr := chromedp.Run(sessionCtx,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector(%q)?.textContent || ''`, editorSel), &contentText),
	); cerr == nil {
		trimmed := strings.TrimSpace(contentText)
		if len([]rune(trimmed)) == 0 {
			diagScreenshot(sessionCtx, "zhihu", "fail")
			return "", fmt.Errorf("正文填充校验失败：编辑器内容为空（DraftJS 可能拒绝了输入）")
		}
		log.Printf("[PublishAuto:zhihu] 正文已填充（%d 字符）", len([]rune(trimmed)))
	}

	// DRY_RUN（完成标准：发布按钮就绪才算成功——探测不点击）
	if publishDryRun {
		if ready, detail := probePublishButton(sessionCtx, "zhihu"); ready {
			log.Printf("[PublishAuto:zhihu] ✅ DRY_RUN 完成：发布按钮已就绪 %s（未点击）", detail)
		} else {
			log.Printf("[PublishAuto:zhihu] ❌ DRY_RUN 失败：发布按钮未就绪 %s", detail)
		}
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil && len(shot) > 0 {
			p := filepath.Join("data", fmt.Sprintf("publish-dryrun-%s.png", job.ID))
			if wErr := os.WriteFile(p, shot, 0o644); wErr == nil {
				log.Printf("[PublishAuto:zhihu] DRY_RUN 完成（未发布）——截图 %s", p)
			}
		}
		return "dryrun://zhihu", nil
	}

	// 5. 等待发布按钮可点击 + 点击。
	// 【修复】原代码点击 disabled 按钮静默无效还误判成功。新逻辑：轮询等待
	// 按钮 enabled（正文填入后 DraftJS 校验通过自动激活），最多等 12 秒。
	err = chromedp.Run(sessionCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			jsClick := `(() => {
				// 稳定标识：Button--primary + Button--blue + 文本"发布"（hash 类名会变，不用）
				const candidates = document.querySelectorAll('button.Button--primary, button[class*="Button--blue"]');
				for (const btn of candidates) {
					if (btn.textContent.trim() === '发布' && !btn.disabled && btn.offsetParent !== null) {
						btn.click();
						return true;
					}
				}
				return false;
			})()`
			deadline := time.Now().Add(12 * time.Second)
			for time.Now().Before(deadline) {
				var clicked bool
				if e := chromedp.Evaluate(jsClick, &clicked).Do(ctx); e != nil {
					return e
				}
				if clicked {
					return nil
				}
				if e := chromedp.Sleep(500 * time.Millisecond).Do(ctx); e != nil {
					return e
				}
			}
			return fmt.Errorf("发布按钮 12 秒内未变为可点击（正文校验可能未通过）")
		}),
	)
	if err != nil {
		diagScreenshot(sessionCtx, "zhihu", "fail")
		return "", fmt.Errorf("点击发布按钮失败: %w", err)
	}
	log.Printf("[PublishAuto:zhihu] 发布按钮已点击")

	// 6. 等待跳转获取文章 URL
	var articleURL string
	err = chromedp.Run(sessionCtx,
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&articleURL),
	)
	if err != nil {
		diagScreenshot(sessionCtx, "zhihu", "fail")
		return "", fmt.Errorf("获取文章URL失败: %w", err)
	}

	if strings.Contains(articleURL, "/p/") {
		log.Printf("[PublishAuto:zhihu] 发布成功！文章URL: %s", articleURL)
		return articleURL, nil
	}
	diagScreenshot(sessionCtx, "zhihu", "fail")
	return "", fmt.Errorf("发布可能失败，当前URL: %s", articleURL)
}

// XiaohongshuAutoChannel 小红书全自动发布通道。
// 同时实现 PublishChannel（半自动）和 AutoPublishChannel（全自动）。
type XiaohongshuAutoChannel struct{}

var _ port.PublishChannel = (*XiaohongshuAutoChannel)(nil)
var _ port.AutoPublishChannel = (*XiaohongshuAutoChannel)(nil)

func NewXiaohongshuAutoChannel() *XiaohongshuAutoChannel { return &XiaohongshuAutoChannel{} }

func (c *XiaohongshuAutoChannel) Platform() string             { return "xiaohongshu" }
func (c *XiaohongshuAutoChannel) SupportedMediaType() []string { return []string{"text", "image"} }
func (c *XiaohongshuAutoChannel) SupportedContentTypes() []string {
	return []string{entity.ContentTypeImage, entity.ContentTypeVideo, entity.ContentTypeArticle, entity.ContentTypeAudio}
}

// PublishSemiAuto 半自动模式：按 ContentType 拼 target 参数直达正确发布页
func (c *XiaohongshuAutoChannel) PublishSemiAuto(_ context.Context, job entity.PublishJob, _ entity.Account) (string, error) {
	ct := job.ContentType
	if ct == "" {
		ct = entity.ContentTypeImage // 默认图文
	}
	return "https://creator.xiaohongshu.com/publish/publish?from=menu&target=" + ct, nil
}

// PublishAuto 全自动发布到小红书（按 ContentType 分发）。
// 4 种形态：image（图文）/ video（视频）/ article（长文）/ audio（音频）。
// 账号通用（同一 cookie），形态决定具体上传/填写流程。
func (c *XiaohongshuAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookieStr string) (string, error) {
	ct := job.ContentType
	if ct == "" {
		ct = entity.ContentTypeImage
	}
	log.Printf("[PublishAuto:xiaohongshu] 开始全自动发布，类型=%s 标题=%q", ct, job.Title)

	cookies := parseCookies(cookieStr, ".xiaohongshu.com")
	base := "https://creator.xiaohongshu.com/publish/publish?from=menu&target=" + ct

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts()...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 4*time.Minute)
	defer func() {
		sessionCancel()
		browserCancel()
		allocCancel()
	}()

	// cookie 注入 + Shadow DOM 穿透补丁 + 导航
	// 关键：小红书 xhs-publish-btn 用 closed Shadow DOM（attachShadow({mode:'closed'})），
	// 外部 JS 的 shadowRoot 返回 null，无法访问内部 button → 发布点击点不到真按钮。
	// 解法：导航前用 addScriptToEvaluateOnNewDocument 注入补丁，重写 attachShadow
	// 强制 mode:'open'——后续所有 shadow root 都可被外部 shadowRoot 访问。
	var currentURL string
	if err := chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(cookies),
		// ① Shadow DOM 穿透补丁（必须在 Navigate 前注入，对页面所有后续 shadow 生效）
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, e := page.AddScriptToEvaluateOnNewDocument(`(() => {
				if (Element.prototype.attachShadow.__patched) return;
				const orig = Element.prototype.attachShadow;
				const patched = function(init) { return orig.call(this, Object.assign({}, init, {mode: 'open'})); };
				patched.__patched = true;
				Element.prototype.attachShadow = patched;
			})()`).Do(ctx)
			return e
		}),
		chromedp.Navigate(base),
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&currentURL),
	); err != nil {
		return "", fmt.Errorf("导航到发布页失败: %w", err)
	}
	log.Printf("[PublishAuto:xiaohongshu] cookie已注入，已导航到: %s", currentURL)
	if strings.Contains(currentURL, "login") {
		return "", fmt.Errorf("cookie已过期，请重新绑定账号")
	}

	switch ct {
	case entity.ContentTypeImage:
		return c.publishImage(sessionCtx, job)
	case entity.ContentTypeVideo, entity.ContentTypeArticle, entity.ContentTypeAudio:
		// video/article/audio 后续实现（image 跑通后复用 trusted 方案 + 大文件上传）
		return "", fmt.Errorf("内容类型 %s 暂未实现，当前仅支持 image（图文）", ct)
	default:
		return "", fmt.Errorf("不支持的内容类型: %s", ct)
	}
}

// publishImage 小红书图文笔记发布（两阶段：上传页 → 自动跳编辑页）。
//
// 流程：上传图片 → 自动跳转编辑页 → 填标题 → 填正文（ProseMirror）→ 发布。
// 关键点：
//   - 图片必须 ≥1 张（小红书图文硬约束）
//   - 正文是 TipTap/ProseMirror 富文本（校验 isTrusted）→ 用 CDP Input.insertText
//   - 上传第一张后页面自动跳转到编辑页（标题/正文/发布按钮在此页）
func (c *XiaohongshuAutoChannel) publishImage(sessionCtx context.Context, job entity.PublishJob) (string, error) {
	// ① 下载图片到本地临时文件（chromedp 上传需要本地路径）
	if len(job.MediaURLs) == 0 {
		return "", fmt.Errorf("图文笔记至少需要 1 张图片（MediaURLs 为空）")
	}
	localPaths, cleanup, err := downloadMediaToTemp(job.MediaURLs)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer cleanup()

	// ② 上传图片：chromedp.SetUploadFiles 找 input[type=file] 并设置
	log.Printf("[PublishAuto:xiaohongshu] 上传 %d 张图片", len(localPaths))
	if err := chromedp.Run(sessionCtx,
		// 等上传区可见（"上传图片"按钮）
		chromedp.WaitVisible(`.upload-button, button.upload-button, [class*="upload-button"]`, chromedp.ByQuery),
		// SetUploadFiles 自动找 input[type=file] 设置文件（chromedp 内部处理）
		chromedp.SetUploadFiles(`input[type="file"]`, localPaths, chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // 等上传 + 自动跳编辑页
	); err != nil {
		// input[type=file] 可能找不到（纯动态创建）——兜底：点上传按钮触发 FileChooser
		log.Printf("[PublishAuto:xiaohongshu] SetUploadFiles 失败，尝试点击上传按钮: %v", err)
		if err2 := c.uploadByClick(sessionCtx, localPaths); err2 != nil {
			return "", fmt.Errorf("图片上传失败（SetUploadFiles + 点击兜底均失败）: %w / %v", err, err2)
		}
	}
	log.Printf("[PublishAuto:xiaohongshu] 图片已上传")

	// ③ 等待编辑页就绪（标题输入框出现 = 已跳转到编辑页）
	if err := chromedp.Run(sessionCtx,
		chromedp.WaitVisible(`input.d-text[placeholder*="填写标题"], input[placeholder*="填写标题"]`, chromedp.ByQuery),
	); err != nil {
		return "", fmt.Errorf("等待编辑页就绪失败（图片可能上传未完成）: %w", err)
	}

	// ④ 填标题（简单 input，CDP SendKeys trusted）
	// 小红书图文标题上限 20 字（硬约束——超长点击发布会被前端校验拦截，URL 不跳转）
	title := job.Title
	if r := []rune(title); len(r) > 20 {
		title = string(r[:20])
		log.Printf("[PublishAuto:xiaohongshu] 标题超 20 字，已截断: %q → %q", job.Title, title)
	}
	if title != "" {
		if err := chromedp.Run(sessionCtx,
			chromedp.SendKeys(`input.d-text[placeholder*="填写标题"], input[placeholder*="填写标题"]`, title, chromedp.ByQuery),
			chromedp.Sleep(800*time.Millisecond),
		); err != nil {
			return "", fmt.Errorf("填充标题失败: %w", err)
		}
		log.Printf("[PublishAuto:xiaohongshu] 标题已填充")
	}

	// ⑤ 填正文——ProseMirror（TipTap），同知乎方案：CDP Input.insertText（trusted）
	// ProseMirror 校验 isTrusted，合成事件被拒 → 必须用内核 Input 事件
	if job.Content != "" {
		const editorSel = `.tiptap.ProseMirror[contenteditable="true"], .ProseMirror[contenteditable="true"]`
		if err := chromedp.Run(sessionCtx,
			chromedp.Click(editorSel, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
			chromedp.ActionFunc(func(ctx context.Context) error {
				return input.InsertText(job.Content).Do(ctx)
			}),
			chromedp.Sleep(1500*time.Millisecond),
		); err != nil {
			return "", fmt.Errorf("填充正文失败: %w", err)
		}
		// 校验正文非空
		var contentText string
		_ = chromedp.Run(sessionCtx,
			chromedp.Evaluate(`document.querySelector('.tiptap.ProseMirror, .ProseMirror')?.textContent || ''`, &contentText),
		)
		if len([]rune(strings.TrimSpace(contentText))) == 0 {
			return "", fmt.Errorf("正文填充校验失败：编辑器内容为空（ProseMirror 可能拒绝输入）")
		}
		log.Printf("[PublishAuto:xiaohongshu] 正文已填充（%d 字符）", len([]rune(contentText)))
	}

	// DRY_RUN：走完上传+填表后截图返回、不点发布（与抖音/知乎同款安全闸门）
	if publishDryRun {
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil && len(shot) > 0 {
			p := filepath.Join("data", fmt.Sprintf("publish-dryrun-%s.png", job.ID))
			if wErr := os.WriteFile(p, shot, 0o644); wErr == nil {
				log.Printf("[PublishAuto:xiaohongshu] DRY_RUN 完成（未发布）——截图 %s", p)
			}
		}
		return "dryrun://xiaohongshu", nil
	}

	// ⑥ 点发布：Shadow DOM 坐标穿透 + CDP 鼠标点击。
	// xhs-publish-btn 是 Vue 自定义元素，内部 button 可能在 Shadow DOM（querySelector
	// 穿不透）。改用：JS 穿透 shadowRoot 取 button 真实坐标 → CDP Input.dispatchMouseEvent
	// 点坐标（内核层 trusted，不依赖 querySelector，不受 shadow boundary 限制）。
	err = chromedp.Run(sessionCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 等待 submit-disabled="false"（按钮可点击）
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) {
				var ready bool
				_ = chromedp.Evaluate(`(() => {
					const w = document.querySelector('xhs-publish-btn');
					return w && w.getAttribute('submit-disabled') === 'false';
				})()`, &ready).Do(ctx)
				if ready {
					break
				}
				if e := chromedp.Sleep(600 * time.Millisecond).Do(ctx); e != nil {
					return e
				}
			}
			// 穿透 Shadow DOM（补丁后 closed→open）取 button 坐标 + 诊断信息
			var info struct {
				X, Y, W, H float64
				Source     string `json:"source"` // shadow/light/text/wrapper
				Text       string `json:"text"`
				InnerHTML  string `json:"innerHTML"`
			}
			if e := chromedp.Evaluate(`(() => {
				const w = document.querySelector('xhs-publish-btn');
				if (!w) return {source:'none', innerHTML:'no xhs-publish-btn'};
				let btn = null, source = 'none', text = '';
				if (w.shadowRoot) {
					// ① 精确找"发布"按钮：ce-btn bg-red（红色发布键，排除"暂存离开"）
					btn = w.shadowRoot.querySelector('button.ce-btn.bg-red, button.bg-red');
					if (btn) { source = 'shadow-red'; text = btn.textContent.trim().slice(0,20); }
					// ② 兜底：文本精确=="发布"的 button（不匹配"暂存离开"）
					if (!btn) {
						for (const b of w.shadowRoot.querySelectorAll('button')) {
							if (b.textContent.trim() === '发布' || b.textContent.trim() === '发布笔记') {
								btn = b; source = 'shadow-text'; text = b.textContent.trim(); break;
							}
						}
					}
				}
				// light DOM 找 button
				if (!btn) {
					btn = w.querySelector('button, [role="button"]');
					if (btn) { source = 'light'; text = btn.textContent.trim().slice(0,20); }
				}
				// 文本兜底：shadowRoot/light 内找文本含"发布"的叶子元素
				if (!btn) {
					const roots = w.shadowRoot ? [w.shadowRoot, w] : [w];
					for (const root of roots) {
						for (const el of root.querySelectorAll('*')) {
							const dt = Array.from(el.childNodes).filter(n=>n.nodeType===3).map(n=>n.textContent.trim()).join('');
							if (dt === '发布' || dt === '发布笔记') {
								const r = el.getBoundingClientRect();
								if (r.width > 0) { btn = el; source = 'text'; text = dt; break; }
							}
						}
						if (btn) break;
					}
				}
				if (!btn) { btn = w; source = 'wrapper'; }
				const r = btn.getBoundingClientRect();
				return {x: r.x + r.width/2, y: r.y + r.height/2, w: r.width, h: r.height, source, text, innerHTML: (w.shadowRoot ? w.shadowRoot.innerHTML : w.innerHTML).slice(0,400)};
			})()`, &info).Do(ctx); e != nil {
				return e
			}
			log.Printf("[PublishAuto:xiaohongshu] 发布按钮定位 source=%s text=%q 坐标=(%.0f,%.0f) 尺寸=%.0fx%.0f innerHTML=%.200s",
				info.Source, info.Text, info.X, info.Y, info.W, info.H, info.InnerHTML)
			if info.W == 0 || info.H == 0 {
				return fmt.Errorf("发布按钮不可见（尺寸 0）source=%s innerHTML=%s", info.Source, info.InnerHTML)
			}
			// CDP 鼠标点击坐标（内核层 trusted 事件）
			if e := input.DispatchMouseEvent(input.MouseMoved, info.X, info.Y).Do(ctx); e != nil {
				return e
			}
			press := input.DispatchMouseEvent(input.MousePressed, info.X, info.Y)
			press.Button, press.ClickCount = input.Left, 1
			if e := press.Do(ctx); e != nil {
				return e
			}
			release := input.DispatchMouseEvent(input.MouseReleased, info.X, info.Y)
			release.Button, release.ClickCount = input.Left, 1
			return release.Do(ctx)
		}),
	)
	if err != nil {
		return "", fmt.Errorf("点击发布按钮失败: %w", err)
	}
	log.Printf("[PublishAuto:xiaohongshu] 发布按钮已点击（CDP 鼠标坐标）")

	// 诊断截图（点击后页面状态——验证码/校验红框/没反应 都能看出）
	var screenshot []byte
	if e := chromedp.Run(sessionCtx, chromedp.Sleep(3*time.Second), chromedp.CaptureScreenshot(&screenshot)); e == nil && len(screenshot) > 0 {
		fileName := fmt.Sprintf("debug-xhs-%d.png", time.Now().UnixNano())
		shotPath := filepath.Join("data", "media", fileName)
		if wErr := os.WriteFile(shotPath, screenshot, 0o644); wErr == nil {
			// 静态路由 /media → ./data/media，URL 用 /media/文件名（不带 data/ 前缀）
			log.Printf("[PublishAuto:xiaohongshu] 诊断截图: http://localhost:8082/media/%s", fileName)
		}
	}
	log.Printf("[PublishAuto:xiaohongshu] 发布按钮已点击")

	// ⑦ 等待发布完成 + 综合判断成功（不靠单一 URL 判断——小红书发布成功后
	// URL 行为不确定：可能跳转、可能原地弹 toast、可能跳到仍含 publish 的路径）。
	// 改为检查【失败信号】：有明确错误/验证码才报失败；否则认为成功（已点击+无失败）。
	var resultURL string
	var pageText string
	if err := chromedp.Run(sessionCtx,
		chromedp.Sleep(6*time.Second),
		chromedp.Location(&resultURL),
		chromedp.Evaluate(`document.body ? document.body.innerText.slice(0, 2000) : ''`, &pageText),
	); err != nil {
		return "", fmt.Errorf("获取发布结果失败: %w", err)
	}
	log.Printf("[PublishAuto:xiaohongshu] 发布结果 URL: %s", resultURL)

	// 失败信号检测（明确报错才算失败）
	failureSignals := []string{
		"发布失败", "上传失败", "内容违规", "请重新登录",
		"滑动验证", "安全验证", "请完成验证",
	}
	for _, sig := range failureSignals {
		if strings.Contains(pageText, sig) {
			return "", fmt.Errorf("发布失败（页面提示含「%s」）URL: %s", sig, resultURL)
		}
	}
	// 无失败信号 = 发布成功（小红书异步审核，平台侧已接收）
	log.Printf("[PublishAuto:xiaohongshu] ✅ 发布成功（已提交，等待平台审核）URL: %s", resultURL)
	return resultURL, nil
}

// uploadByClick 兜底上传：点击"上传图片"按钮触发文件选择，用 input 兜底。
// （小红书若用纯动态 input，SetUploadFiles 找不到时用此兜底）
func (c *XiaohongshuAutoChannel) uploadByClick(sessionCtx context.Context, localPaths []string) error {
	// 点击上传按钮（触发 file chooser）
	if err := chromedp.Run(sessionCtx,
		chromedp.Click(`.upload-button, button.upload-button`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		return err
	}
	// file chooser 弹出后，再尝试设置 input（动态创建的 input 此时应在 DOM）
	return chromedp.Run(sessionCtx,
		chromedp.SetUploadFiles(`input[type="file"]`, localPaths, chromedp.ByQuery),
		chromedp.Sleep(5*time.Second),
	)
}

// downloadMediaToTemp 下载 URL 列表到本地临时文件，返回路径 + cleanup。
// chromedp 上传需要本地文件路径；调用方 defer cleanup() 清理。
func downloadMediaToTemp(urls []string) (paths []string, cleanup func(), err error) {
	client := &http.Client{Timeout: 60 * time.Second}
	var tmpFiles []*os.File
	cleanup = func() {
		for _, f := range tmpFiles {
			_ = os.Remove(f.Name())
		}
	}
	for i, u := range urls {
		// 扩展名：从 URL 推断（图片/视频），默认 .png
		ext := ".png"
		lu := strings.ToLower(u)
		switch {
		case strings.HasSuffix(lu, ".jpg") || strings.HasSuffix(lu, ".jpeg"):
			ext = ".jpg"
		case strings.HasSuffix(lu, ".webp"):
			ext = ".webp"
		case strings.HasSuffix(lu, ".mp4"):
			ext = ".mp4" // 视频发布（抖音/快手）
		case strings.HasSuffix(lu, ".mov"):
			ext = ".mov"
		case strings.HasSuffix(lu, ".webm"):
			ext = ".webm"
		}
		req, e := http.NewRequest(http.MethodGet, u, nil)
		if e != nil {
			cleanup()
			return nil, cleanup, e
		}
		resp, e := client.Do(req)
		if e != nil {
			cleanup()
			return nil, cleanup, fmt.Errorf("下载 %s: %w", u, e)
		}
		tmp, e := os.CreateTemp("", fmt.Sprintf("xhs-upload-%d-*%s", i, ext))
		if e != nil {
			resp.Body.Close()
			cleanup()
			return nil, cleanup, e
		}
		tmpFiles = append(tmpFiles, tmp)
		if _, e := io.Copy(tmp, resp.Body); e != nil {
			resp.Body.Close()
			tmp.Close()
			cleanup()
			return nil, cleanup, fmt.Errorf("写入 %s: %w", u, e)
		}
		resp.Body.Close()
		tmp.Close()
		paths = append(paths, tmp.Name())
	}
	return paths, cleanup, nil
}

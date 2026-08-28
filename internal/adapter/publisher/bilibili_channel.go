package publisher

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/publisher/humanize"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- B站发布通道（半自动 + 全自动）----
//
// 架构（与抖音/快手同模式）：半自动 + 全自动合并在同一个 Channel。
// 能力声明：video（主力）。
//
// 反检测（Level 2）：全自动发布走 humanize.HumanAction（人类行为模拟 + 指纹伪装）。

// BilibiliAutoChannel B站发布通道（半自动 + 全自动）。
type BilibiliAutoChannel struct {
	accountRepo port.AccountRepository // 可选：cookie 回写
	vault       port.CookieVault
}

var _ port.PublishChannel = (*BilibiliAutoChannel)(nil)
var _ port.ChannelInfoProvider = (*BilibiliAutoChannel)(nil)
var _ port.AutoPublishChannel = (*BilibiliAutoChannel)(nil)

// SetAccountStore 注入账号存储（可选）。
func (c *BilibiliAutoChannel) SetAccountStore(ar port.AccountRepository, v port.CookieVault) {
	c.accountRepo, c.vault = ar, v
}

func NewBilibiliAutoChannel() *BilibiliAutoChannel { return &BilibiliAutoChannel{} }

func (c *BilibiliAutoChannel) Platform() string             { return "bilibili" }
func (c *BilibiliAutoChannel) SupportedMediaType() []string { return []string{"video"} }
func (c *BilibiliAutoChannel) SupportedContentTypes() []string {
	return []string{entity.ContentTypeVideo}
}

func (c *BilibiliAutoChannel) DisplayName() string { return "B站" }
func (c *BilibiliAutoChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		// 标签必填 ≥1（上限10）+ 分区必选——投稿页硬约束，服务端下发给前端动态表单
		entity.ContentTypeVideo: {TitleMaxRunes: 80, MinVideos: 1, RequireTags: true, RequireCategory: true, MaxTags: 10},
	}
}

// PublishSemiAuto 半自动：生成B站发布页 URL（用户手动上传+发布）。
func (c *BilibiliAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://member.bilibili.com/platform/upload/video/frame", nil
}

// PublishAuto 全自动：chromedp + HumanAction 反检测层，RPA 发布视频。
func (c *BilibiliAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (string, error) {
	videoURL, newCookie, err := publishBilibiliVideo(ctx, job, cookie)
	if err != nil {
		return "", err
	}
	// cookie 滚动回写
	c.writebackCookie(context.Background(), job, newCookie)
	return videoURL, nil
}

// writebackCookie 把浏览器最新 cookie 加密回写账号。
func (c *BilibiliAutoChannel) writebackCookie(ctx context.Context, job entity.PublishJob, newCookie string) {
	if c.accountRepo == nil || c.vault == nil || newCookie == "" || job.AccountID == "" {
		return
	}
	acc, err := c.accountRepo.FindByID(ctx, job.TenantID, job.AccountID)
	if err != nil || acc.ID == "" {
		return
	}
	enc, err := c.vault.Encrypt(newCookie)
	if err != nil {
		return
	}
	acc.CookieEncrypted = enc
	if saveErr := c.accountRepo.Save(ctx, acc); saveErr == nil {
		log.Printf("[PublishAuto:bilibili] cookie 已滚动回写（账号 %s 绑定续期）", acc.ID)
	}
}

// publishBilibiliVideo B站视频发布核心流程（参考 KBSZR BilibiliDistributor）。
//
// 流程（6步）：
//  1. 下载视频到临时文件
//  2. 启动 Stealth 浏览器 + cookie 注入 + 登录态预检
//  3. 上传视频（input[type=file] 隐藏控件）
//  4. 等待上传/转码完成（编辑器出现）
//  5. 填文案（humanize.HumanAction 反检测）
//  6. 点击发布
func publishBilibiliVideo(ctx context.Context, job entity.PublishJob, cookie string) (videoURL, newCookie string, err error) {
	// ① 下载视频到临时文件
	videoPaths, cleanup, dlErr := downloadMediaToTemp(job.MediaURLs)
	if dlErr != nil {
		return "", "", fmt.Errorf("下载视频失败: %w", dlErr)
	}
	if len(videoPaths) == 0 {
		return "", "", fmt.Errorf("未找到视频文件")
	}
	defer cleanup()
	videoPath := videoPaths[0]

	// ② 启动 Stealth 浏览器 + cookie 注入
	opts := humanize.StealthOptions()
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	sessionCtx, cancel := context.WithTimeout(browserCtx, 5*time.Minute) // B站上传较慢，给更多时间
	defer cancel()

	ha := humanize.New(sessionCtx)
	var currentURL string
	// 2026-08-28 DRY_RUN 实测修复：cookie 注入缺失（重构成 Stealth 版时丢掉了
	// network.Enable+SetCookies——cookie 参数收了但从未进浏览器，必然回落登录页）。
	// 与抖音通道同款顺序：Enable → SetCookies(.bilibili.com) → 指纹 → 导航（同一 context）
	if e := chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".bilibili.com")),
		chromedp.ActionFunc(func(c context.Context) error { return humanize.InjectFingerprint(c) }),
		chromedp.Navigate("https://member.bilibili.com/platform/upload/video/frame"),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&currentURL),
	); e != nil {
		return "", "", fmt.Errorf("打开B站发布页失败: %w", e)
	}
	if strings.Contains(currentURL, "passport") || strings.Contains(currentURL, "login") {
		// 2026-08-28 DRY_RUN 排查：预检失败留证（实际跳转 URL + 截图——盲猜不如看页面）
		log.Printf("[PublishAuto:bilibili] 登录态预检失败，实际 URL=%s", currentURL)
		_ = os.MkdirAll("data", 0o755)
		var failShot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&failShot)); e == nil {
			p := fmt.Sprintf("data/publish-bili-loginfail-%s.png", job.ID)
			_ = os.WriteFile(p, failShot, 0o644)
			log.Printf("[PublishAuto:bilibili] 失败截图 %s", p)
		}
		return "", "", fmt.Errorf("cookie 失效（重定向登录页），请重新绑定B站账号")
	}
	log.Printf("[PublishAuto:bilibili] 登录态预检通过，发布页已打开")

	// ③ 上传视频（2026-08-28 修复：旧 WaitVisible 对 B站隐藏 input 死等到 5 分钟超时——
	// 改用 panda 校准的四级降级，presence 判定不阻塞）
	if e := setUploadFileCascade(sessionCtx, videoPath); e != nil {
		return "", "", fmt.Errorf("上传视频失败: %w", e)
	}
	log.Printf("[PublishAuto:bilibili] 视频已提交上传，等待转码…")

	// ④ 等待上传/转码完成
	editorSel := waitFirstVisible(sessionCtx, 180*time.Second, // B站转码较慢
		`#video-title-input`, `input[placeholder*="标题"]`, `[contenteditable=true]`)
	if editorSel == "" {
		return "", "", fmt.Errorf("等待上传完成超时（编辑器未出现）")
	}
	log.Printf("[PublishAuto:bilibili] 上传/转码完成（编辑器选择器=%s）", editorSel)

	// ⑤ 填标题（panda 校准候选链 + 事件派发兜底）
	title := clampRunes(job.Title, 80)
	if !fillFirstEditable(sessionCtx, title, append([]string{editorSel}, biliTitleSels...)...) {
		if e := ha.Click(editorSel); e == nil {
			if te := ha.Type(editorSel, title); te != nil {
				log.Printf("[PublishAuto:bilibili] 标题填充失败（不阻断）: %v", te)
			}
		}
	}

	// 填描述（2026-08-28 修复：旧 #video-desc-input WaitVisible 无独立超时——元素不存在
	// 死等 5 分钟。改 panda 候选链 + fillFirstEditable（不阻塞，未命中仅日志））
	if job.Content != "" {
		if !fillFirstEditable(sessionCtx, clampRunes(job.Content, 250), biliDescSels...) {
			log.Printf("[PublishAuto:bilibili] 未定位描述输入框（候选均未命中，跳过描述）")
		}
	}

	// ⑤b 标签（2026-08-28 DOM 快照实证：input.input-val placeholder="按回车键Enter创建标签"
	// ——回车确认模式；候选链前排实证选择器）
	if len(job.Tags) > 0 {
		tagSel := waitFirstVisible(sessionCtx, 10*time.Second, biliTagSels...)
		log.Printf("[PublishAuto:bilibili] 标签框探测结果 sel=%q tags=%d", tagSel, len(job.Tags))
		if tagSel != "" {
			// 2026-08-28 截图实证修复：ha.Type 逐字符 SendKeys 对中文双插入
			//（keydown.text + input 事件各一次 →「测测试试」）。改 JS 设值（单次无损）
			// + CDP 回车确认（ASCII 键无双插入问题）
			var filled int
			for i, tag := range job.Tags {
				if i >= 10 {
					break // B站标签上限 10 个
				}
				if fillFirstEditable(sessionCtx, strings.TrimPrefix(tag, "#"), tagSel) {
					chromedp.Sleep(300 * time.Millisecond)
					_ = chromedp.Run(sessionCtx, chromedp.SendKeys(tagSel, "\r", chromedp.ByQuery))
					chromedp.Sleep(300 * time.Millisecond)
					filled++
				} else {
					log.Printf("[PublishAuto:bilibili] 标签 %q 设值失败（不阻断）", tag)
					break
				}
			}
			if filled > 0 {
				log.Printf("[PublishAuto:bilibili] 已填 %d 个标签（JS设值+回车，选择器=%s）", filled, tagSel)
			}
		} else {
			log.Printf("[PublishAuto:bilibili] 未定位到标签输入框（候选均未命中）")
		}
	}

	// ⑤c 分区（Plan-14 #5：Category 字段贯通——B站投稿分区必选）。
	// B站分区是自定义下拉（非原生 select）：点击触发 → 按文本选项点击。未配置时
	// 尝试默认选第一项（页面常有预选分区，无操作即用默认）。
	if job.Category != "" {
		var catDone bool
		_ = chromedp.Run(sessionCtx, chromedp.Evaluate(fmt.Sprintf(`(() => {
			// 候选：分区选择器容器 → 点击展开 → 文本匹配选项
			const triggers = document.querySelectorAll('[class*="category"], [class*="zone"] [class*="select"], .select-wrapper');
			for (const t of triggers) {
				if (t.offsetParent === null) continue;
				t.click();
				break;
			}
			return true;
		})()`), &catDone))
		_ = chromedp.Sleep(1 * time.Second)
		var picked bool
		_ = chromedp.Run(sessionCtx, chromedp.Evaluate(fmt.Sprintf(`(() => {
			const opts = document.querySelectorAll('li, [class*="option"], [class*="item"]');
			for (const o of opts) {
				if (o.offsetParent === null) continue;
				if ((o.textContent || '').trim().includes(%q)) { o.click(); return true; }
			}
			return false;
		})()`, job.Category), &picked))
		if !picked {
			log.Printf("[PublishAuto:bilibili] 分区 %q 未选中（选择器待真机校准）", job.Category)
		} else {
			log.Printf("[PublishAuto:bilibili] 分区已选: %s", job.Category)
		}
	}

	// ⑤a2 封面上传（panda 校准完整流程）：点「添加封面」→ 图片 input 上传 →
	// 「封面制作」×2（进入裁剪器）→ 勾选第一个 checkbox → 「完成」确认
	if job.CoverURL != "" {
		if coverPaths, coverCleanup, cErr := downloadMediaToTemp([]string{job.CoverURL}); cErr == nil {
			if uploadCoverImage(sessionCtx, coverPaths[0]) {
				// 封面制作弹窗交互（panda 实测两连点进入裁剪器）
				for i := 0; i < 2; i++ {
					if ok, _ := evalBool(sessionCtx, textVisibleJS("封面制作")); ok {
						_ = chromedp.Run(sessionCtx, chromedp.Evaluate(`(() => {
							const els = [...document.querySelectorAll('button, span, div')].filter(e =>
								e.children.length === 0 && (e.textContent || '').trim() === '封面制作' && e.offsetParent !== null);
							if (els.length) { els[0].click(); return true; }
							return false;
						})()`, nil))
						chromedp.Sleep(1 * time.Second)
					}
				}
				// 勾选封面 checkbox（2026-08-28 修复：chromedp.Click 内部 WaitVisible
				// 对遮挡元素死等 ~5 分钟吃光预算——改 JS click 不等待可见性）
				_, _ = evalBool(sessionCtx, `(() => {
					const cb = document.querySelector('.bcc-checkbox-checkbox');
					if (cb) { cb.click(); return true; }
					return false;
				})()`)
				chromedp.Sleep(800 * time.Millisecond)
				// 完成确认（panda：getByText('完成', {exact:true})，找不到不阻断——封面可能已选）
				_ = chromedp.Run(sessionCtx, chromedp.Evaluate(`(() => {
					const els = [...document.querySelectorAll('button, [role=button], span')].filter(e =>
						e.children.length === 0 && (e.textContent || '').trim() === '完成' && e.offsetParent !== null);
					if (els.length) { els[els.length-1].click(); return true; }
					return false;
				})()`, nil))
				chromedp.Sleep(1 * time.Second)
				log.Printf("[PublishAuto:bilibili] 封面流程已走完（panda 校准）")
			}
			coverCleanup()
		} else {
			log.Printf("[PublishAuto:bilibili] 封面下载失败（跳过）: %v", cErr)
		}
	}

	// ⑤b 平台必选项（panda 真机校准）：创作声明——B站 2026 版发布页必选，
	// 未选择时「立即投稿」被禁用。固定选「个人观点，仅供参考」。
	selectDeclarationOption(sessionCtx, "请选择符合您视频内容的创作声明", "个人观点，仅供参考", "确定")
	// ⑤b 创作声明（DOM 快照取证：bcc-select 组件——input[placeholder*=创作声明]
	// 触发展开 bcc-option 列表；选中后 input.value 显示所选文本）。未选时「立即投稿」被禁用
	selectBiliDeclaration(sessionCtx, "个人观点，仅供参考")
	// panda 校准：声明后可能弹「内容无需标注」确认按钮
	dismissBannerDialog(sessionCtx, "内容无需标注", "内容无需标注")

	// DRY_RUN（完成标准：发布按钮就绪才算成功——探测到可点的「立即投稿」即全链路通，
	// 绝不点击；截图 + 表单 DOM 快照留证）
	if publishDryRun {
		if ready, detail := probePublishButton(sessionCtx, "bilibili"); ready {
			log.Printf("[PublishAuto:bilibili] ✅ DRY_RUN 完成：发布按钮已就绪 %s（未点击）", detail)
		} else {
			log.Printf("[PublishAuto:bilibili] ❌ DRY_RUN 失败：发布按钮未就绪 %s", detail)
		}
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil {
			p := fmt.Sprintf("data/publish-dryrun-bilibili-%s.png", job.ID)
			_ = os.WriteFile(p, shot, 0o644)
			log.Printf("[PublishAuto:bilibili] DRY_RUN 完成（未发布）——截图 %s", p)
		}
		var domHTML string
		if e := chromedp.Run(sessionCtx, chromedp.Evaluate(
			`document.querySelector('.video-upload-form, [class*="upload-form"], form, body')?.outerHTML || ''`, &domHTML)); e == nil && domHTML != "" {
			p := fmt.Sprintf("data/publish-dryrun-bilibili-%s.html", job.ID)
			_ = os.WriteFile(p, []byte(domHTML), 0o644)
			log.Printf("[PublishAuto:bilibili] 表单 DOM 快照 %s（%d 字节）", p, len(domHTML))
		}
		return "dryrun://bilibili", "", nil
	}

	// ⑥ 点击发布（2026-08-28 真发实锤：按钮在 micro-app 内嵌 iframe——主文档
	// markButtonByText 标记不到会死等 5 分钟。全文档穿透 JS click（跨 iframe 的
	// 坐标点击不可行，JS click 在目标文档内执行）
	scrollToBottom(sessionCtx)
	published := false
	for _, text := range []string{"立即投稿", "投稿"} {
		if ok, _ := evalBool(sessionCtx, fmt.Sprintf(`(() => {
			const docs = [document];
			const walk = (doc) => { for (const f of doc.querySelectorAll('iframe')) {
				try { if (f.contentDocument) { docs.push(f.contentDocument); walk(f.contentDocument); } } catch (e) {} } };
			walk(document);
			for (const m of document.querySelectorAll('micro-app')) { if (m.shadowRoot) docs.push(m.shadowRoot); }
			for (const doc of docs) {
				// 2026-08-28 DOM 快照实锤：「立即投稿」是 span.submit-add——不是 button！
				// 按钮类元素 + 文本叶子（span/div）双通道匹配
				const bySel = doc.querySelector('span.submit-add, .submit-add, [class*="submit-add"]');
				if (bySel && bySel.offsetParent !== null) {
					bySel.scrollIntoView({block: 'center'}); bySel.click(); return true;
				}
				const leaves = [...doc.querySelectorAll('button, [role=button], span, div')].filter(b => {
					const t = (b.textContent || '').trim();
					return t === %q && b.children.length === 0 && b.offsetParent !== null;
				});
				if (leaves.length) {
					const target = leaves[0].closest('[class*="submit"], [class*="publish"], button') || leaves[0];
					target.scrollIntoView({block: 'center'}); target.click(); return true;
				}
			}
			return false;
		})()`, text)); ok {
			log.Printf("[PublishAuto:bilibili] 已点击「%s」（全文档穿透）", text)
			published = true
			break
		}
	}
	if !published {
		// 失败诊断（2026-08-28 真发排查：dump 全文档按钮清单 + 截图——按钮可能
		// disabled/文本变体/在更深的容器）
		var btnDump string
		_ = chromedp.Run(sessionCtx, chromedp.Evaluate(`(() => {
			const docs = [document];
			const walk = (doc) => { for (const f of doc.querySelectorAll('iframe')) {
				try { if (f.contentDocument) { docs.push(f.contentDocument); walk(f.contentDocument); } } catch (e) {} } };
			walk(document);
			const out = [];
			docs.forEach((doc, di) => {
				for (const b of doc.querySelectorAll('button, [role=button]')) {
					const t = (b.textContent || '').trim().slice(0, 20);
					if (t) out.push('[doc' + di + '] ' + t + (b.disabled ? '(禁用)' : '') + (b.offsetParent ? '' : '(隐藏)'));
				}
			});
			return out.slice(0, 30).join(' | ');
		})()`, &btnDump))
		log.Printf("[PublishAuto:bilibili] 按钮清单：%s", btnDump)
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil {
			p := fmt.Sprintf("data/publish-bili-btnfail-%s.png", job.ID)
			_ = os.WriteFile(p, shot, 0o644)
			log.Printf("[PublishAuto:bilibili] 按钮失败截图 %s", p)
		}
		return "", "", fmt.Errorf("发布按钮未找到（立即投稿/投稿 全文档穿透均未命中）——见按钮清单日志与截图")
	}
	_ = ha

	// ⑦ 发布结果确认（panda 校准：严格断言「稿件投递成功」文本，30s 超时；
	// 成功后跳稿件管理页再提取视频链接）
	confirmCtx, confirmCancel := context.WithTimeout(sessionCtx, 60*time.Second)
	defer confirmCancel()
	submitted := false
	for i := 0; i < 15; i++ {
		if ok, _ := evalBool(confirmCtx, `(() => {
			const docs = [document];
			const walk = (doc) => { for (const f of doc.querySelectorAll('iframe')) {
				try { if (f.contentDocument) { docs.push(f.contentDocument); walk(f.contentDocument); } } catch (e) {} } };
			walk(document);
			for (const m of document.querySelectorAll('micro-app')) { if (m.shadowRoot) docs.push(m.shadowRoot); }
			return docs.some(doc => [...doc.querySelectorAll('*')].some(e =>
				e.children.length === 0 && (e.textContent || '').trim().includes('稿件投递成功') && e.offsetParent !== null));
		})()`); ok {
			submitted = true
			break
		}
		chromedp.Sleep(2 * time.Second)
	}
	if !submitted {
		// 2026-08-29 设计修正（用户实锤"published 但没发出"）：确认信号缺失时绝不能
		// 标成功——返回错误让 job 如实 failed（fail-safe：宁可误报失败让人工核对，
		// 不可误报成功掩盖问题。真发点击本身不可靠——span 点击可能没触发 Vue）
		return "", "", fmt.Errorf("发布确认信号未捕获（「稿件投递成功」文本 30s 未出现——点击可能未生效或平台拦截，请到B站创作中心人工核对后重试）")
	}

	// 获取发布结果（B站发布后跳稿件管理页）
	videoURL, err = extractVideoURLAfterPublish(confirmCtx)
	if err != nil {
		videoURL = "https://member.bilibili.com/platform/upload/video/frame" // 降级
	}

	// 读取最新 cookie
	newCookie = readBackCookie(sessionCtx)

	log.Printf("[PublishAuto:bilibili] 发布完成（submitted=%v），URL=%s", submitted, videoURL)
	return videoURL, newCookie, nil
}

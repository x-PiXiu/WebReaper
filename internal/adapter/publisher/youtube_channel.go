// youtube_channel.go YouTube 发布通道（panda-video-automations-publisher YouTube
// upload spec 的 chromedp 移植，2026-08）。
//
// 平台要点（panda 真机实测知识）：
//   - 上传页 studio.youtube.com/channel/me/videos/upload；Google 反检测必须做足
//     （--disable-blink-features=AutomationControlled + en-US locale + 旧金山时区 +
//     navigator 覆写），否则 Studio 报"浏览器不安全"拒绝上传
//   - 表单控件是 aria-label（中英双语候选），非 placeholder
//   - 可见性向导：radio「不是面向儿童的」→ 继续×3 → radio「公开」→ 发布 → 关闭
//   - 成功断言：上传完成对话框中按标题定位视频卡片
//
// 账号：Google 登录无法扫码——cookie 由管理后台手动导入（账号库通用 cookie 字段，
// 种到 .youtube.com/.google.com 双域）；半自动模式返回 Studio 上传页 URL。
package publisher

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/publisher/humanize"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// YouTubeAutoChannel YouTube 发布通道（半自动 + 全自动）。
type YouTubeAutoChannel struct {
	accountRepo port.AccountRepository
	vault       port.CookieVault
}

var _ port.PublishChannel = (*YouTubeAutoChannel)(nil)
var _ port.ChannelInfoProvider = (*YouTubeAutoChannel)(nil)
var _ port.AutoPublishChannel = (*YouTubeAutoChannel)(nil)

// SetAccountStore 注入账号存储（可选：发布后 cookie 滚动回写）。
func (c *YouTubeAutoChannel) SetAccountStore(ar port.AccountRepository, v port.CookieVault) {
	c.accountRepo, c.vault = ar, v
}

func NewYouTubeAutoChannel() *YouTubeAutoChannel { return &YouTubeAutoChannel{} }

func (c *YouTubeAutoChannel) Platform() string { return "youtube" }
func (c *YouTubeAutoChannel) SupportedMediaType() []string {
	return []string{"video"}
}
func (c *YouTubeAutoChannel) SupportedContentTypes() []string {
	return []string{entity.ContentTypeVideo}
}
func (c *YouTubeAutoChannel) DisplayName() string { return "YouTube" }
func (c *YouTubeAutoChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeVideo: {TitleMaxRunes: 100, MinVideos: 1},
	}
}

// PublishSemiAuto 半自动：返回 YouTube Studio 上传页（用户手动完成）。
func (c *YouTubeAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://studio.youtube.com/channel/me/videos/upload", nil
}

// PublishAuto 全自动：chromedp RPA（Google 反检测加强）。
func (c *YouTubeAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (string, error) {
	videoURL, newCookie, err := publishYouTubeVideo(ctx, job, cookie)
	if err != nil {
		return "", err
	}
	if c.accountRepo != nil && c.vault != nil && newCookie != "" && job.AccountID != "" {
		if acc, fErr := c.accountRepo.FindByID(context.Background(), job.TenantID, job.AccountID); fErr == nil && acc.ID != "" {
			if enc, eErr := c.vault.Encrypt(newCookie); eErr == nil {
				acc.CookieEncrypted = enc
				_ = c.accountRepo.Save(context.Background(), acc)
			}
		}
	}
	return videoURL, nil
}

// publishYouTubeVideo YouTube 全自动发布（panda YouTube upload spec 移植）。
//
//	① 下载视频 → ② Stealth 浏览器（Google 反检测：en-US/旧金山时区/navigator 覆写）
//	→ ③ cookie 双域注入（.youtube.com/.google.com）→ ④ 上传
//	→ ⑤ 填标题/描述（aria-label 中英候选）→ ⑥ 可见性向导 → ⑦ 发布+关闭
//	→ ⑧ 成功断言（按标题定位视频卡片）
func publishYouTubeVideo(ctx context.Context, job entity.PublishJob, cookie string) (videoURL, newCookie string, err error) {
	if len(job.MediaURLs) == 0 {
		return "", "", fmt.Errorf("视频文件缺失（MediaURLs 为空）——全自动发布需要 mp4 的 URL")
	}
	paths, cleanup, dErr := downloadMediaToTemp(job.MediaURLs)
	if dErr != nil {
		return "", "", fmt.Errorf("下载视频失败: %w", dErr)
	}
	defer cleanup()
	videoPath := paths[0]
	log.Printf("[PublishAuto:youtube] 视频已就绪 %s", videoPath)

	// ② Stealth 浏览器 + Google 反检测三件套（panda 实测：缺任一 Studio 报浏览器不安全）
	opts := humanize.StealthOptions()
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("lang", "en-US"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 8*time.Minute)
	defer sessionCancel()

	ha := humanize.New(sessionCtx)
	// 首次 Run + locale/时区伪装 + cookie 双域注入 + 导航
	var currentURL string
	if e := chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".youtube.com")),
		network.SetCookies(parseCookies(cookie, ".google.com")),
		emulation.SetLocaleOverride().WithLocale("en-US"),
		emulation.SetTimezoneOverride("America/Los_Angeles"),
		chromedp.ActionFunc(func(c context.Context) error { return humanize.InjectFingerprint(c) }),
		chromedp.Navigate("https://studio.youtube.com/channel/me/videos/upload"),
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&currentURL),
	); e != nil {
		return "", "", fmt.Errorf("打开 YouTube Studio 失败: %w", e)
	}
	if strings.Contains(currentURL, "accounts.google.com") {
		return "", "", fmt.Errorf("cookie 失效（回落 Google 登录页）——请重新导入 YouTube 账号 cookie")
	}
	log.Printf("[PublishAuto:youtube] 登录态预检通过，上传页已打开")

	// ④ 上传（panda：Studio 的 input 在点"选择文件"后出现——先试直传再点按钮）
	if ok, _ := evalBool(sessionCtx, `document.querySelectorAll('input[type=file]').length > 0`); !ok {
		// 触发文件选择按钮（中英文案候选）
		for _, text := range []string{"Select files", "选择文件", "UPLOAD VIDEOS", "上传视频"} {
			if done, _ := evalBool(sessionCtx, fmt.Sprintf(`(() => {
				const btns = [...document.querySelectorAll('button, a, [role=button]')].filter(b =>
					(b.textContent || '').trim().includes(%q) && b.offsetParent !== null);
				if (btns.length) { btns[0].click(); return true; }
				return false;
			})()`, text)); done {
				break
			}
		}
		chromedp.Sleep(2 * time.Second)
	}
	if e := setUploadFileCascade(sessionCtx, videoPath); e != nil {
		return "", "", fmt.Errorf("上传视频失败: %w", e)
	}
	log.Printf("[PublishAuto:youtube] 视频已提交上传，等待处理…")

	// ⑤ 等表单 + 填标题/描述（panda：aria-label 中英候选）
	titleSel := waitFirstVisible(sessionCtx, 180*time.Second,
		`input[aria-label="Title"]`, `input[aria-label="标题"]`,
		`input[aria-label*="Tell viewers about your video"]`, `#textbox > input`)
	if titleSel == "" {
		return "", "", fmt.Errorf("等待上传表单超时（标题框未出现）——建议 PUBLISH_DRY_RUN 核查")
	}
	waitForProcessingDone(sessionCtx, 15*time.Second)
	title := clampRunes(job.Title, 100)
	if !fillFirstEditable(sessionCtx, title, titleSel,
		`input[aria-label*="Title"]`, `input[aria-label*="标题"]`) {
		_ = ha.Click(titleSel)
		_ = ha.Type(titleSel, title)
	}
	if job.Content != "" {
		fillFirstEditable(sessionCtx, clampRunes(job.Content, 5000),
			`textarea[aria-label*="Description"]`, `textarea[aria-label*="说明"]`, `#description textarea`)
	}

	// DRY_RUN：截图留证不发布
	if publishDryRun {
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil {
			p := fmt.Sprintf("data/publish-dryrun-youtube-%s.png", job.ID)
			_ = os.WriteFile(p, shot, 0o644)
			log.Printf("[PublishAuto:youtube] DRY_RUN 完成（未发布）——截图 %s", p)
		}
		return "dryrun://youtube", readBackCookieByDomain(sessionCtx, ".youtube.com"), nil
	}

	// ⑥ 可见性向导（panda 固定流 + privacy 透传）：radio 非儿童内容 → 继续×3 →
	// 按 job.Privacy 选择可见性（public 公开 / unlisted 不公开 / private 私享）
	clickTextOption(sessionCtx, "No, it's not made for kids", "不是面向儿童的")
	for i := 0; i < 3; i++ {
		clickByText(sessionCtx, "Next", "下一步")
		chromedp.Sleep(1 * time.Second)
	}
	switch job.Privacy {
	case "unlisted":
		clickTextOption(sessionCtx, "Unlisted", "不公开")
	case "private":
		clickTextOption(sessionCtx, "Private", "私享")
	default: // 空/public
		clickTextOption(sessionCtx, "Public", "公开")
	}

	// ⑦ 发布 + 关闭（panda：Publish → Close）
	scrollToBottom(sessionCtx)
	clickByText(sessionCtx, "Publish", "发布")
	chromedp.Sleep(3 * time.Second)
	clickByText(sessionCtx, "Close", "关闭")
	log.Printf("[PublishAuto:youtube] 已走完发布向导，确认结果…")

	// ⑧ 成功断言（panda：上传完成对话框按标题定位视频卡片）
	confirmCtx, confirmCancel := context.WithTimeout(sessionCtx, 60*time.Second)
	defer confirmCancel()
	var href string
	_ = chromedp.Run(confirmCtx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const links = [...document.querySelectorAll('a')].filter(a =>
			(a.textContent || '').includes(%q) && (a.href || '').includes('watch'));
		return links.length ? links[0].href : '';
	})()`, job.Title), &href))
	readBack := readBackCookieByDomain(confirmCtx, ".youtube.com")
	if href == "" {
		return "", readBack, fmt.Errorf("未捕获发布后视频链接（可能已发出——请到 YouTube Studio 人工核对）")
	}
	return href, readBack, nil
}

// clickTextOption 按候选文本点击 radio/选项（中英双语）。
func clickTextOption(ctx context.Context, texts ...string) {
	for _, t := range texts {
		if done, _ := evalBool(ctx, fmt.Sprintf(`(() => {
			const els = [...document.querySelectorAll('[role=radio], input[type=radio], label, [role=option]')].filter(e => {
				const label = (e.getAttribute('aria-label') || e.textContent || '').trim();
				return label.includes(%q) && e.offsetParent !== null;
			});
			if (els.length) { els[0].click(); return true; }
			return false;
		})()`, t)); done {
			chromedp.Sleep(800 * time.Millisecond)
			return
		}
	}
}

// clickByText 按候选文本点击按钮（中英双语）。
func clickByText(ctx context.Context, texts ...string) {
	for _, t := range texts {
		if done, _ := evalBool(ctx, fmt.Sprintf(`(() => {
			const btns = [...document.querySelectorAll('button, [role=button]')].filter(b =>
				(b.textContent || '').trim() === %q && b.offsetParent !== null && !b.disabled);
			if (btns.length) { btns[0].click(); return true; }
			return false;
		})()`, t)); done {
			chromedp.Sleep(800 * time.Millisecond)
			return
		}
	}
}

package publisher

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/publisher/humanize"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 视频号发布通道（半自动 + 全自动）----
//
// 架构（与抖音/快手同模式）：半自动 + 全自动合并在同一个 Channel。
// 能力声明：video（主力）+ image（图文轮播）。
//
// 反检测（Level 2）：全自动发布走 humanize.HumanAction（人类行为模拟 + 指纹伪装）。

// WeixinAutoChannel 视频号发布通道（半自动 + 全自动）。
type WeixinAutoChannel struct {
	accountRepo port.AccountRepository // 可选：cookie 回写
	vault       port.CookieVault
}

var _ port.PublishChannel = (*WeixinAutoChannel)(nil)
var _ port.ChannelInfoProvider = (*WeixinAutoChannel)(nil)
var _ port.AutoPublishChannel = (*WeixinAutoChannel)(nil)

// SetAccountStore 注入账号存储（可选）。
func (c *WeixinAutoChannel) SetAccountStore(ar port.AccountRepository, v port.CookieVault) {
	c.accountRepo, c.vault = ar, v
}

func NewWeixinAutoChannel() *WeixinAutoChannel { return &WeixinAutoChannel{} }

func (c *WeixinAutoChannel) Platform() string             { return "weixin" }
func (c *WeixinAutoChannel) SupportedMediaType() []string { return []string{"video", "image"} }
func (c *WeixinAutoChannel) SupportedContentTypes() []string {
	return []string{entity.ContentTypeVideo, entity.ContentTypeImage}
}

func (c *WeixinAutoChannel) DisplayName() string { return "视频号" }
func (c *WeixinAutoChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeVideo: {TitleMaxRunes: 40, MinVideos: 1},
		entity.ContentTypeImage: {TitleMaxRunes: 40, MinImages: 1},
	}
}

// PublishSemiAuto 半自动：生成视频号发布页 URL（用户手动上传+发布）。
func (c *WeixinAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://channels.weixin.qq.com/platform/media/upload", nil
}

// PublishAuto 全自动：chromedp + HumanAction 反检测层，RPA 发布视频。
func (c *WeixinAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (string, error) {
	videoURL, newCookie, err := publishWeixinVideo(ctx, job, cookie)
	if err != nil {
		return "", err
	}
	// cookie 滚动回写
	c.writebackCookie(context.Background(), job, newCookie)
	return videoURL, nil
}

// writebackCookie 把浏览器最新 cookie 加密回写账号。
func (c *WeixinAutoChannel) writebackCookie(ctx context.Context, job entity.PublishJob, newCookie string) {
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
		log.Printf("[PublishAuto:weixin] cookie 已滚动回写（账号 %s 绑定续期）", acc.ID)
	}
}

// publishWeixinVideo 视频号视频发布核心流程（参考 KBSZR WeixinDistributor）。
//
// 流程（6步）：
//  1. 下载视频到临时文件
//  2. 启动 Stealth 浏览器 + cookie 注入 + 登录态预检
//  3. 上传视频（input[type=file] 隐藏控件）
//  4. 等待上传/转码完成（编辑器出现）
//  5. 填文案（humanize.HumanAction 反检测）
//  6. 点击发表
func publishWeixinVideo(ctx context.Context, job entity.PublishJob, cookie string) (videoURL, newCookie string, err error) {
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
	sessionCtx, cancel := context.WithTimeout(browserCtx, 4*time.Minute)
	defer cancel()

	ha := humanize.New(sessionCtx)
	var currentURL string
	if e := chromedp.Run(sessionCtx,
		chromedp.ActionFunc(func(c context.Context) error { return humanize.InjectFingerprint(c) }),
		chromedp.Navigate("https://channels.weixin.qq.com/platform/media/upload"),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&currentURL),
	); e != nil {
		return "", "", fmt.Errorf("打开视频号发布页失败: %w", e)
	}
	if strings.Contains(currentURL, "login") || strings.Contains(currentURL, "passport") {
		return "", "", fmt.Errorf("cookie 失效（重定向登录页），请重新绑定视频号账号")
	}
	log.Printf("[PublishAuto:weixin] 登录态预检通过，发布页已打开")

	// ③ 上传视频（三级降级校准：input 可能隐藏——presence 直传）
	if e := setUploadFileCascade(sessionCtx, videoPath); e != nil {
		return "", "", fmt.Errorf("上传视频失败: %w", e)
	}
	log.Printf("[PublishAuto:weixin] 视频已提交上传，等待转码…")

	// ④ 等待上传完成（panda 校准双信号）：「取消上传」按钮出现=上传中，
	// 再等它消失=上传完成（比编辑器出现更准）；编辑器出现做兜底信号
	uploadDone := false
	for i := 0; i < 60; i++ { // 120s
		cancelVisible, _ := evalBool(sessionCtx, textVisibleJS("取消上传"))
		if cancelVisible {
			chromedp.Sleep(2 * time.Second)
			continue
		}
		if i > 3 { // 至少等过一小段，避免上传尚未开始就误判
			uploadDone = true
			break
		}
		chromedp.Sleep(2 * time.Second)
	}
	if !uploadDone {
		editorSel := waitFirstVisible(sessionCtx, 30*time.Second,
			`.input-editor[contenteditable=true]`, `[contenteditable=true]`, `.ql-editor`)
		if editorSel == "" {
			return "", "", fmt.Errorf("等待上传完成超时（取消上传信号与编辑器均未出现）")
		}
	}
	log.Printf("[PublishAuto:weixin] 上传/转码完成")

	// ⑤ 填文案（panda 校准）：视频号标题只留汉字并截 16 字（平台 UI 限制，
	// 非汉字字符会被丢弃导致截断错乱——先清洗再截）；contenteditable 需事件派发
	title := cleanChineseTitle(job.Title, 16)
	if !fillFirstEditable(sessionCtx, title, weixinTitleSels...) {
		editorSel := waitFirstVisible(sessionCtx, 5*time.Second,
			`.input-editor[contenteditable=true]`, `[contenteditable=true]`)
		if editorSel != "" {
			if e := ha.Click(editorSel); e == nil {
				_ = ha.Type(editorSel, title)
			}
		}
	}

	// ⑤b 位置必须关闭（panda 校准：视频号发布默认可能带定位——点开位置
	// 下拉选「不显示位置」，带定位发布是本地生活场景之外的噪音）
	dismissWeixinLocation(sessionCtx)

	// DRY_RUN
	if publishDryRun {
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil {
			p := fmt.Sprintf("data/publish-dryrun-weixin-%s.png", job.ID)
			_ = os.WriteFile(p, shot, 0o644)
			log.Printf("[PublishAuto:weixin] DRY_RUN 完成（未发布）——截图 %s", p)
		}
		return "dryrun://weixin", "", nil
	}

	// ⑥ 点击发表（panda 校准文案候选：发表/立即发布/发布/确认发布/提交；
	// 修复旧逻辑反转 bug——逐候选尝试标记，成功即 humanize 点击）
	published := false
	for _, text := range []string{"发表", "立即发布", "发布", "确认发布", "提交"} {
		if e := markButtonByText(sessionCtx, text, "取消"); e == nil {
			if e := ha.Click(`[data-wr-publish-btn]`); e == nil {
				published = true
				log.Printf("[PublishAuto:weixin] 已点击「%s」按钮", text)
				break
			}
			// humanize 失败——CDP 直点兜底
			if e2 := chromedp.Run(sessionCtx, chromedp.Click(`[data-wr-publish-btn]`, chromedp.ByQuery)); e2 == nil {
				published = true
				log.Printf("[PublishAuto:weixin] 已点击「%s」按钮（CDP 直点）", text)
				break
			}
		}
	}
	if !published {
		return "", "", fmt.Errorf("定位发表按钮失败（发表/立即发布/发布/确认发布/提交均未命中）")
	}

	// 等待发布完成（panda：微信无成功断言信号——等 5s 后尽量提取）
	time.Sleep(5 * time.Second)

	// ⑦ 获取发布结果
	videoURL, err = extractVideoURLAfterPublish(sessionCtx)
	if err != nil {
		videoURL = "https://channels.weixin.qq.com/platform/media/upload" // 降级
	}

	// 读取最新 cookie
	newCookie = readBackCookie(sessionCtx)

	log.Printf("[PublishAuto:weixin] 发布完成，URL=%s（panda 校准注：微信无成功断言信号，建议人工核对）", videoURL)
	return videoURL, newCookie, nil
}

// cleanChineseTitle 视频号标题清洗（panda 校准）：只保留汉字并截断 n 字；
// 清洗后为空则回退原始标题（panda WeixinVideo spec cleanTitle 实测规则）。
func cleanChineseTitle(s string, n int) string {
	var b []rune
	for _, r := range s {
		if (r >= 0x4e00 && r <= 0x9fff) || (r >= 0x3400 && r <= 0x4dbf) || (r >= 0xf900 && r <= 0xfaff) {
			b = append(b, r)
		}
	}
	if len(b) == 0 {
		return s
	}
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}

// dismissWeixinLocation 关闭视频号发布页的位置挂载（panda 校准：L464-537——
// 点开位置选择 → 选「不显示位置」；找不到位置控件则静默跳过）。
func dismissWeixinLocation(ctx context.Context) {
	done, _ := evalBool(ctx, `(() => {
		const triggers = [...document.querySelectorAll(
			'[placeholder*="位置"], [placeholder*="所在"], [class*="location"], [class*="Location"]'
		)].filter(e => e.offsetParent !== null);
		if (!triggers.length) return false;
		triggers[0].click();
		return true;
	})()`)
	if !done {
		return
	}
	chromedp.Sleep(1 * time.Second)
	picked, _ := evalBool(ctx, `(() => {
		const opts = [...document.querySelectorAll('li, [class*="option"], [class*="item"], span, div')].filter(e =>
			e.children.length === 0 && (e.textContent || '').trim() === '不显示位置' && e.offsetParent !== null);
		if (!opts.length) return false;
		opts[0].click();
		return true;
	})()`)
	if picked {
		log.Printf("[PublishAuto:weixin] 位置已关闭（不显示位置）")
		chromedp.Sleep(800 * time.Millisecond)
	}
}

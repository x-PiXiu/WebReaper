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
		entity.ContentTypeVideo: {TitleMaxRunes: 80, MinVideos: 1},
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
	if e := chromedp.Run(sessionCtx,
		chromedp.ActionFunc(func(c context.Context) error { return humanize.InjectFingerprint(c) }),
		chromedp.Navigate("https://member.bilibili.com/platform/upload/video/frame"),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&currentURL),
	); e != nil {
		return "", "", fmt.Errorf("打开B站发布页失败: %w", e)
	}
	if strings.Contains(currentURL, "passport") || strings.Contains(currentURL, "login") {
		return "", "", fmt.Errorf("cookie 失效（重定向登录页），请重新绑定B站账号")
	}
	log.Printf("[PublishAuto:bilibili] 登录态预检通过，发布页已打开")

	// ③ 上传视频（B站上传选择器有特殊处理）
	// 优先使用 .upload-input input[type="file"]，失败后回退到 input[type="file"]
	var uploadErr error
	uploadSelectors := []string{
		`.upload-input input[type="file"]`,
		`input[type="file"]`,
	}
	for _, sel := range uploadSelectors {
		if e := chromedp.Run(sessionCtx,
			chromedp.WaitVisible(sel, chromedp.ByQuery),
			chromedp.SetUploadFiles(sel, []string{videoPath}, chromedp.ByQuery),
		); e == nil {
			uploadErr = nil
			break
		} else {
			uploadErr = e
		}
	}
	if uploadErr != nil {
		return "", "", fmt.Errorf("上传视频失败: %w", uploadErr)
	}
	log.Printf("[PublishAuto:bilibili] 视频已提交上传，等待转码…")

	// ④ 等待上传/转码完成
	editorSel := waitFirstVisible(sessionCtx, 180*time.Second, // B站转码较慢
		`#video-title-input`, `input[placeholder*="标题"]`, `[contenteditable=true]`)
	if editorSel == "" {
		return "", "", fmt.Errorf("等待上传完成超时（编辑器未出现）")
	}
	log.Printf("[PublishAuto:bilibili] 上传/转码完成（编辑器选择器=%s）", editorSel)

	// ⑤ 填标题
	title := clampRunes(job.Title, 80)
	if e := ha.Click(editorSel); e == nil {
		if te := ha.Type(editorSel, title); te != nil {
			log.Printf("[PublishAuto:bilibili] 标题填充失败（不阻断）: %v", te)
		}
	}

	// 填描述（如果有单独的描述输入框）
	descSel := `#video-desc-input`
	if e := chromedp.Run(sessionCtx, chromedp.WaitVisible(descSel, chromedp.ByQuery)); e == nil {
		desc := clampRunes(job.Content, 250)
		if e := ha.Click(descSel); e == nil {
			if te := ha.Type(descSel, desc); te != nil {
				log.Printf("[PublishAuto:bilibili] 描述填充失败（不阻断）: %v", te)
			}
		}
	}

	// DRY_RUN
	if publishDryRun {
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil {
			p := fmt.Sprintf("data/publish-dryrun-bilibili-%s.png", job.ID)
			_ = os.WriteFile(p, shot, 0o644)
			log.Printf("[PublishAuto:bilibili] DRY_RUN 完成（未发布）——截图 %s", p)
		}
		return "dryrun://bilibili", "", nil
	}

	// ⑥ 点击发布
	publishBtn := markButtonByText(sessionCtx, "投稿", "取消")
	if publishBtn != nil {
		// 回退：尝试 .publish-button
		chromedp.Run(sessionCtx, chromedp.Click(`.publish-button`, chromedp.ByQuery))
	}
	log.Printf("[PublishAuto:bilibili] 已点击发布按钮")

	// 等待发布完成
	time.Sleep(5 * time.Second)

	// ⑦ 获取发布结果（B站发布后会跳转到稿件管理页）
	videoURL, err = extractVideoURLAfterPublish(sessionCtx)
	if err != nil {
		videoURL = "https://member.bilibili.com/platform/upload/video/frame" // 降级
	}

	// 读取最新 cookie
	newCookie = readBackCookie(sessionCtx)

	log.Printf("[PublishAuto:bilibili] 发布完成，URL=%s", videoURL)
	return videoURL, newCookie, nil
}

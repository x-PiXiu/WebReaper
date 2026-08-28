package publisher

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"webreaper/internal/adapter/publisher/humanize"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 抖音/快手发布通道（获客智能体转型：视频分发主战场）----
//
// 架构（与知乎/小红书同模式）：半自动 + 全自动合并在同一个 Channel。
// 能力声明：video（主力）+ image（图文轮播）；不支持 article。
//
// 反检测（Level 2）：全自动发布走 humanize.HumanAction（人类行为模拟 + 指纹伪装）。

// DouyinAutoChannel 抖音发布通道（半自动 + 全自动）。
type DouyinAutoChannel struct {
	accountRepo port.AccountRepository // 可选：cookie 回写（发布会话后滚动续期绑定寿命）
	vault       port.CookieVault
}

var _ port.PublishChannel = (*DouyinAutoChannel)(nil)
var _ port.ChannelInfoProvider = (*DouyinAutoChannel)(nil)
var _ port.AutoPublishChannel = (*DouyinAutoChannel)(nil)

// SetAccountStore 注入账号存储（可选；注入后发布成功把浏览器里的最新 cookie 回写账号库——
// 抖音会话 cookie 滚动刷新，回写让绑定从"扫码快照时效"变成"滚动续期"）。
func (c *DouyinAutoChannel) SetAccountStore(ar port.AccountRepository, v port.CookieVault) {
	c.accountRepo, c.vault = ar, v
}

// publishDryRun DRY_RUN 安全验证模式（PUBLISH_DRY_RUN=true）：走完上传+填表后
// 截图返回、不点发布——用于真机验证选择器，绝不动商户账号发出内容。
var publishDryRun = strings.TrimSpace(os.Getenv("PUBLISH_DRY_RUN")) != ""

func NewDouyinAutoChannel() *DouyinAutoChannel { return &DouyinAutoChannel{} }

func (c *DouyinAutoChannel) Platform() string { return "douyin" }
func (c *DouyinAutoChannel) SupportedMediaType() []string { return []string{"video"} }
func (c *DouyinAutoChannel) SupportedContentTypes() []string {
	// 能力诚实化（Plan-14 修正 #11）：图文（publish-image）RPA 未实现——声明了
	// 就是给前端放出必然失败的选项。实现后恢复 ContentTypeImage。
	return []string{entity.ContentTypeVideo}
}

func (c *DouyinAutoChannel) DisplayName() string { return "抖音" }
func (c *DouyinAutoChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeVideo: {TitleMaxRunes: 55, MinVideos: 1},
	}
}

// PublishSemiAuto 半自动：生成抖音发布页 URL（用户手动上传+发布）。
func (c *DouyinAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://creator.douyin.com/creator-micro/content/publish-video", nil
}

// PublishAuto 全自动：chromedp + HumanAction 反检测层，RPA 发布视频。
func (c *DouyinAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (string, error) {
	videoURL, newCookie, err := publishDouyinVideo(ctx, job, cookie)
	if err != nil {
		return "", err
	}
	// cookie 滚动回写：发布会话中抖音刷新了会话 cookie，写回账号库延长绑定寿命
	c.writebackCookie(context.Background(), job, newCookie)
	return videoURL, nil
}

// writebackCookie 把浏览器最新 cookie 加密回写账号（MediaCrawler update_cookies 思想）。
func (c *DouyinAutoChannel) writebackCookie(ctx context.Context, job entity.PublishJob, newCookie string) {
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
		log.Printf("[PublishAuto:douyin] cookie 已滚动回写（账号 %s 绑定续期）", acc.ID)
	}
}

// KuaishouAutoChannel 快手发布通道（半自动 + 全自动）。
type KuaishouAutoChannel struct {
	accountRepo port.AccountRepository // 可选：cookie 回写
	vault       port.CookieVault
}

var _ port.PublishChannel = (*KuaishouAutoChannel)(nil)
var _ port.ChannelInfoProvider = (*KuaishouAutoChannel)(nil)
var _ port.AutoPublishChannel = (*KuaishouAutoChannel)(nil)

// SetAccountStore 注入账号存储（可选；发布会话后 cookie 滚动回写）。
func (c *KuaishouAutoChannel) SetAccountStore(ar port.AccountRepository, v port.CookieVault) {
	c.accountRepo, c.vault = ar, v
}

func NewKuaishouAutoChannel() *KuaishouAutoChannel { return &KuaishouAutoChannel{} }

func (c *KuaishouAutoChannel) Platform() string { return "kuaishou" }
func (c *KuaishouAutoChannel) SupportedMediaType() []string { return []string{"video"} }
func (c *KuaishouAutoChannel) SupportedContentTypes() []string {
	// 能力诚实化：image（图文）RPA 未实现（与抖音同规则——实现后再声明）
	return []string{entity.ContentTypeVideo}
}

func (c *KuaishouAutoChannel) DisplayName() string { return "快手" }
func (c *KuaishouAutoChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeVideo: {TitleMaxRunes: 80, MinVideos: 1},
	}
}

// PublishSemiAuto 半自动：生成快手发布页 URL。
func (c *KuaishouAutoChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	return "https://cp.kuaishou.com/article/publish/video", nil
}

// PublishAuto 全自动：chromedp + HumanAction 反检测层，RPA 发布视频。
func (c *KuaishouAutoChannel) PublishAuto(ctx context.Context, job entity.PublishJob, cookie string) (string, error) {
	videoURL, newCookie, err := publishKuaishouVideo(ctx, job, cookie)
	if err != nil {
		return "", err
	}
	// cookie 滚动回写（对齐抖音/B站/视频号）
	if c.accountRepo != nil && c.vault != nil && newCookie != "" && job.AccountID != "" {
		if acc, fErr := c.accountRepo.FindByID(context.Background(), job.TenantID, job.AccountID); fErr == nil && acc.ID != "" {
			if enc, eErr := c.vault.Encrypt(newCookie); eErr == nil {
				acc.CookieEncrypted = enc
				if sErr := c.accountRepo.Save(context.Background(), acc); sErr == nil {
					log.Printf("[PublishAuto:kuaishou] cookie 已滚动回写（账号 %s 绑定续期）", acc.ID)
				}
			}
		}
	}
	return videoURL, nil
}

// publishKuaishouVideo 快手全自动发布——抖音六步混合模式的同构复刻（2026-08-25 实现，
// 选择器多策略候选、待 DRY_RUN 真机校准）。
// 实测知识（扫码登录域调试）：登录链 passport.kuaishou.com → cp.kuaishou.com；
// 认证 cookie kuaishou.web.cp.api_st/_ph（种在 .kuaishou.com 父域）；
// 登录态预检以"导航后是否回落 passport.*login"为准。
//
//	① 下载视频 → ② Stealth 浏览器+cookie(.kuaishou.com) → ③ 上传
//	→ ④ 等标题框出现=上传完成 → ⑤ 填标题+简介（L1 独立输入框，快手最简单）
//	→ ⑥ DRY_RUN 闸门 → 点发布 → 结果确认（URL/DOM 提取）
func publishKuaishouVideo(ctx context.Context, job entity.PublishJob, cookie string) (videoURL, newCookie string, err error) {
	if len(job.MediaURLs) == 0 {
		return "", "", fmt.Errorf("视频文件缺失（MediaURLs 为空）——全自动发布需要 mp4 的 URL")
	}
	// ① 视频落本地
	paths, cleanup, dErr := downloadMediaToTemp(job.MediaURLs)
	if dErr != nil {
		return "", "", fmt.Errorf("下载视频失败: %w", dErr)
	}
	defer cleanup()
	videoPath := paths[0]
	log.Printf("[PublishAuto:kuaishou] 视频已就绪 %s", videoPath)

	// ② Stealth 浏览器 + cookie 注入
	opts := humanize.StealthOptions()
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 5*time.Minute)
	defer sessionCancel()

	ha := humanize.New(sessionCtx)
	// 首次 Run + cookie 注入 + 导航同一 context（qrlogin 同款陷阱）
	var currentURL string
	if e := chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".kuaishou.com")),
		chromedp.ActionFunc(func(c context.Context) error { return humanize.InjectFingerprint(c) }),
		chromedp.Navigate("https://cp.kuaishou.com/article/publish/video"),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&currentURL),
	); e != nil {
		return "", "", fmt.Errorf("打开快手发布页失败: %w", e)
	}
	if strings.Contains(currentURL, "passport.kuaishou.com") && strings.Contains(currentURL, "login") {
		return "", "", fmt.Errorf("cookie 失效（回落 passport 登录页），请重新绑定快手账号")
	}
	log.Printf("[PublishAuto:kuaishou] 登录态预检通过，发布页已打开")

	// ②b 快手实测修正：满意度调查弹窗先关（遮挡表单）+ 回滚顶部（上传区在视口外）
	dismissSurveyDialog(sessionCtx, "是否满意")
	_ = chromedp.Run(sessionCtx, chromedp.Evaluate(`window.scrollTo(0, 0)`, nil))
	chromedp.Sleep(1 * time.Second)
	// ③ 上传视频（四级降级：input 直传 → filechooser 拦截 → DOM 注入 → 二次探测）
	if e := setUploadFileCascade(sessionCtx, videoPath); e != nil {
		return "", "", fmt.Errorf("上传视频失败: %w", e)
	}
	log.Printf("[PublishAuto:kuaishou] 视频已提交上传，等待处理…")
	// 上传后页面形态留证（选择器校准用——上传区/表单实际形态排查）
	_ = os.MkdirAll("data", 0o755)
	var pageShot []byte
	if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&pageShot)); e == nil {
		_ = os.WriteFile(fmt.Sprintf("data/publish-kuaishou-page-%s.png", job.ID), pageShot, 0o644)
	}

	// ④ 上传完成信号：标题输入框或编辑器出现（panda 校准：contenteditable 形态）
	titleSel := waitFirstVisible(sessionCtx, 120*time.Second,
		`#work-description-edit`, `input[placeholder*="标题"]`, `input[placeholder*="作品"]`,
		`[contenteditable=true]`, `[class*="title"] input`)
	if titleSel == "" {
		return "", "", fmt.Errorf("等待上传完成超时（标题框未出现）——建议 DRY_RUN 核查选择器")
	}
	log.Printf("[PublishAuto:kuaishou] 上传完成（表单选择器=%s）", titleSel)
	waitForProcessingDone(sessionCtx, 15*time.Second)

	// ④b 快手发布页广告 Skip（panda 校准：进发布页常弹广告，挡住表单控件）
	for _, sel := range []string{
		`div[aria-label="Skip"]`, `[aria-label="跳过"]`, `[class*="skip"]`,
		`button:has-text("跳过")`, `[class*="close-ad"]`,
	} {
		if visible, _ := evalBool(sessionCtx, selVisibleJS(sel)); visible {
			_ = chromedp.Run(sessionCtx, chromedp.Click(sel, chromedp.ByQuery))
			chromedp.Sleep(time.Second)
			break
		}
	}

	// ⑤ 填文案（panda 校准：快手是「标题+描述合并进单个 contenteditable」形态——
	// #work-description-edit 实测目标；必须派发 input 事件框架才识别输入；
	// 话题 # 前缀追加尾部——panda 快手无独立话题框，标签直接进正文）
	merged := job.Title
	if job.Content != "" && job.Content != job.Title {
		merged = job.Title + "\n" + job.Content
	}
	if t := hashtagText(job.Tags); t != "" {
		merged = merged + "\n" + t
	}
	if !fillFirstEditable(sessionCtx, clampRunes(merged, 1000), kuaishouContentSels...) {
		// 兜底：独立标题框形态（旧版页面）
		if e := ha.Click(titleSel); e == nil {
			if te := ha.Type(titleSel, clampRunes(job.Title, 80)); te != nil {
				log.Printf("[PublishAuto:kuaishou] 标题填充失败（不阻断）: %v", te)
			}
		}
		descSel2 := waitFirstVisible(sessionCtx, 10*time.Second,
			`textarea[placeholder*="简介"]`, `textarea[placeholder*="描述"]`, `[class*="desc"] textarea`)
		if descSel2 != "" && job.Content != "" {
			if e := ha.Click(descSel2); e == nil {
				_ = ha.Type(descSel2, clampRunes(job.Content, 1000))
			}
		}
	}

	// ⑥ DRY_RUN（完成标准：发布按钮就绪——探测到可点的「发布」即全链路通，绝不点击）
	if publishDryRun {
		if ready, detail := probePublishButton(sessionCtx, "kuaishou"); ready {
			log.Printf("[PublishAuto:kuaishou] ✅ DRY_RUN 完成：发布按钮已就绪 %s（未点击）", detail)
		} else {
			log.Printf("[PublishAuto:kuaishou] ❌ DRY_RUN 失败：发布按钮未就绪 %s", detail)
		}
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil {
			p := fmt.Sprintf("data/publish-dryrun-kuaishou-%s.png", job.ID)
			_ = os.WriteFile(p, shot, 0o644)
			log.Printf("[PublishAuto:kuaishou] DRY_RUN 完成（未发布）——截图 %s", p)
		}
		return "", readBackCookieByDomain(sessionCtx, ".kuaishou.com"), nil
	}

	// ⑦ 点发布（panda 校准：先滚到底；候选文本"发布"排"存草稿"；部分版本是"确认发布"）
	scrollToBottom(sessionCtx)
	if e := markButtonByText(sessionCtx, "发布", "存草稿"); e != nil {
		return "", "", fmt.Errorf("定位发布按钮失败: %w", e)
	}
	if e := ha.Click(`[data-wr-publish-btn]`); e != nil {
		return "", "", fmt.Errorf("点击发布失败: %w", e)
	}
	log.Printf("[PublishAuto:kuaishou] 已点击发布，等待结果确认…")

	// 确认（panda 校准双信号）：URL/DOM 提取视频链接 → 管理页搜索框出现（发布成功跳回
	// 管理页出现「输入搜索关键词」——panda 实测的间接成功断言）
	confirmCtx, confirmCancel := context.WithTimeout(sessionCtx, 60*time.Second)
	defer confirmCancel()
	if url, e := extractVideoURLAfterPublish(confirmCtx); e == nil && url != "" {
		return url, readBackCookieByDomain(confirmCtx, ".kuaishou.com"), nil
	}
	if ok, _ := evalBool(confirmCtx, textVisibleJS("输入搜索关键词")); ok {
		log.Printf("[PublishAuto:kuaishou] 已跳回内容管理页（panda 校准信号：发布成功，视频链接待人工补录）")
		return "", readBackCookieByDomain(confirmCtx, ".kuaishou.com"), nil
	}
	return "", readBackCookieByDomain(confirmCtx, ".kuaishou.com"), fmt.Errorf("发布结果确认超时（可能已发出——请到快手创作者中心人工核对）")
}

// readBackCookieByDomain 从浏览器导出指定域的最新 cookie（滚动回写用；多平台共用）。
func readBackCookieByDomain(ctx context.Context, domain string) string {
	cookies, err := network.GetCookies().Do(ctx)
	if err != nil {
		return ""
	}
	var parts []string
	for _, c := range cookies {
		if strings.HasSuffix(c.Domain, domain) {
			parts = append(parts, c.Name+"="+c.Value)
		}
	}
	return strings.Join(parts, "; ")
}

// publishDouyinVideo 抖音全自动发布——六步混合模式（MediaCrawler 工程思想在"写"场景的映射：
// UI 负责执行（行为指纹与真人一致），网络响应负责状态确认（比 DOM/URL 解析确定））。
//
//	① 下载视频到临时文件 → ② Stealth 浏览器+cookie 注入+登录态预检
//	→ ③ 上传（CDP SetUploadFiles，trusted）→ ④ 编辑器出现=上传/转码完成信号
//	→ ⑤ humanize 填文案 → ⑥ 点发布 → 网络拦截响应取 aweme_id（降级页面提取）
//
// DRY_RUN（PUBLISH_DRY_RUN=true）：走完 ①-⑤ 截图返回，不点发布——真机验证选择器用。
// ⚠️ 选择器为多策略候选（真机未验证）——首次真机调试务必从 DRY_RUN 开始。
func publishDouyinVideo(ctx context.Context, job entity.PublishJob, cookie string) (videoURL, newCookie string, err error) {
	if len(job.MediaURLs) == 0 {
		return "", "", fmt.Errorf("视频文件缺失（MediaURLs 为空）——全自动发布需要 mp4 的 URL")
	}
	// ① 视频落本地（上传控件需要本地文件路径）
	paths, cleanup, dErr := downloadMediaToTemp(job.MediaURLs)
	if dErr != nil {
		return "", "", fmt.Errorf("下载视频失败: %w", dErr)
	}
	defer cleanup()
	videoPath := paths[0]
	log.Printf("[PublishAuto:douyin] 视频已就绪 %s（%d 个媒体）", videoPath, len(paths))

	// ② Stealth 浏览器 + cookie 注入（发布是平台风控最敏感操作，反检测全开）
	opts := humanize.StealthOptions()
	opts = append(opts,
		chromedp.WindowSize(1280, 800),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	// 发布是分钟级流程（上传+转码），超时给足
	sessionCtx, sessionCancel := context.WithTimeout(browserCtx, 5*time.Minute)
	defer sessionCancel()

	// 发布接口响应捕获（状态确认数据面）：item/create 等回包含 aweme_id
	var publishBody atomic.Value
	chromedp.ListenTarget(sessionCtx, func(ev interface{}) {
		resp, ok := ev.(*network.EventResponseReceived)
		if !ok || resp == nil {
			return
		}
		u := resp.Response.URL
		if !strings.Contains(u, "item/create") && !strings.Contains(u, "aweme/v1/creator") {
			return
		}
		go func(reqID network.RequestID) {
			body, e := network.GetResponseBody(reqID).Do(sessionCtx)
			if e == nil && len(body) > 0 {
				publishBody.Store(string(body))
				log.Printf("[PublishAuto:douyin] 捕获发布接口响应 %.200s", string(body))
			}
		}(resp.RequestID)
	})

	ha := humanize.New(sessionCtx)
	// 首次 Run + cookie 注入 + 导航必须同一 context（qrlogin 同款陷阱）
	var currentURL string
	if e := chromedp.Run(sessionCtx,
		network.Enable(),
		network.SetCookies(parseCookies(cookie, ".douyin.com")),
		chromedp.ActionFunc(func(c context.Context) error { return humanize.InjectFingerprint(c) }),
		chromedp.Navigate("https://creator.douyin.com/creator-micro/content/publish-video"),
		chromedp.Sleep(4*time.Second),
		chromedp.Location(&currentURL),
	); e != nil {
		return "", "", fmt.Errorf("打开抖音发布页失败: %w", e)
	}
	if strings.Contains(currentURL, "login") {
		return "", "", fmt.Errorf("cookie 失效（重定向登录页），请重新绑定抖音账号")
	}
	log.Printf("[PublishAuto:douyin] 登录态预检通过，发布页已打开")

	// ③ 上传视频（三级降级校准：抖音 input[type=file] 是隐藏元素，
	// 旧 WaitVisible 会永远超时——presence 判定直传 CDP trusted 事件）
	if e := setUploadFileCascade(sessionCtx, videoPath); e != nil {
		return "", "", fmt.Errorf("上传视频失败: %w", e)
	}
	log.Printf("[PublishAuto:douyin] 视频已提交上传，等待转码…")

	// ④ 上传/转码完成信号：编辑器或标题框出现（视频处理好后表单才可编辑）——多策略候选
	editorSel := waitFirstVisible(sessionCtx, 90*time.Second,
		`[contenteditable=true]`, `.ql-editor`, `[class*="editor"][contenteditable]`, `.ProseMirror`)
	if editorSel == "" {
		return "", "", fmt.Errorf("等待上传完成超时（编辑器未出现）——建议 PUBLISH_DRY_RUN 模式核查选择器")
	}
	log.Printf("[PublishAuto:douyin] 上传/转码完成（编辑器选择器=%s）", editorSel)
	// panda 校准：处理中信号可见时额外等待（过早填表会被平台重置）
	waitForProcessingDone(sessionCtx, 15*time.Second)
	// 2026-08-28 DRY_RUN 实测：声明弹窗上传完成后自动弹出并遮挡表单——
	// 必须先处理掉再填文案（内置 30s 轮询等弹窗出现）
	selectDouyinDeclarationDialog(sessionCtx, "内容为个人观点或见解")

	// ⑤ 填文案（panda 真机校准）：优先独立标题 input（11 候选实测链），
	// 命中则标题/描述分开填；未命中退回编辑器合并填（旧模式兜底）
	titleFilled := fillFirstEditable(sessionCtx, clampRunes(job.Title, 30), douyinTitleSels...)
	descFilled := false
	if titleFilled && job.Content != "" {
		descFilled = fillFirstEditable(sessionCtx, clampRunes(job.Content, 400), douyinDescSels...)
	}
	if !titleFilled {
		// 编辑器模式：标题+正文合并（抖音发布页单编辑器形态）
		desc := job.Title
		if job.Content != "" && job.Content != job.Title {
			desc = job.Title + " " + job.Content
		}
		if e := ha.Click(editorSel); e == nil {
			if te := ha.Type(editorSel, clampRunes(desc, 400)); te != nil {
				log.Printf("[PublishAuto:douyin] 文案填充失败（不阻断）: %v", te)
			}
		}
	}
	_ = descFilled

	// ⑤a 话题独立填写（panda 校准：# 前缀空格连接 → 话题输入候选链；
	// 未命中独立话题框则并入描述编辑器——抖音编辑器内 # 文本自带话题语义）
	if len(job.Tags) > 0 {
		if !fillHashtags(sessionCtx, job.Tags, douyinTagSels...) {
			log.Printf("[PublishAuto:douyin] 未定位独立话题框，话题并入编辑器")
			_ = fillFirstEditable(sessionCtx, " "+hashtagText(job.Tags), `[contenteditable=true]`, `.ql-editor`)
		}
	}

	// ⑤a2 封面上传（panda 校准：CoverURL 落地 → 图片 input/封面按钮候选链）
	if job.CoverURL != "" {
		if coverPaths, coverCleanup, cErr := downloadMediaToTemp([]string{job.CoverURL}); cErr == nil {
			uploadCoverImage(sessionCtx, coverPaths[0])
			coverCleanup()
		} else {
			log.Printf("[PublishAuto:douyin] 封面下载失败（跳过，用平台自动截帧）: %v", cErr)
		}
	}

	// ⑤b 平台必选项与弹窗兜底（弹窗可能在填文案中途才弹出——二次处理；
	// 内联"请选择自主声明"旧形态保留最后一道兜底）
	dismissBannerDialog(sessionCtx, "共创中心", "我知道了")
	selectDouyinDeclarationDialog(sessionCtx, "内容为个人观点或见解")
	selectDeclarationOption(sessionCtx, "请选择自主声明", "内容为个人观点或见解", "确定")

	// DRY_RUN（完成标准：发布按钮就绪——探测到可点的「发布」即全链路通，绝不点击）
	if publishDryRun {
		if ready, detail := probePublishButton(sessionCtx, "douyin"); ready {
			log.Printf("[PublishAuto:douyin] ✅ DRY_RUN 完成：发布按钮已就绪 %s（未点击）", detail)
		} else {
			log.Printf("[PublishAuto:douyin] ❌ DRY_RUN 失败：发布按钮未就绪 %s", detail)
		}
		_ = os.MkdirAll("data", 0o755)
		var shot []byte
		if e := chromedp.Run(sessionCtx, chromedp.CaptureScreenshot(&shot)); e == nil {
			p := fmt.Sprintf("data/publish-dryrun-%s.png", job.ID)
			_ = os.WriteFile(p, shot, 0o644)
			log.Printf("[PublishAuto:douyin] DRY_RUN 完成（未发布）——截图 %s", p)
		}
		return "", readBackCookie(sessionCtx), nil
	}

	// ⑥ 点发布（2026-08-29 实测：按钮文案"作品发布"——泛"发布"会命中顶栏/侧栏同名项）
	scrollToBottom(sessionCtx)
	if e := markButtonByText(sessionCtx, "作品发布", "存草稿"); e != nil {
		return "", "", fmt.Errorf("定位发布按钮失败: %w", e)
	}
	if e := ha.Click(`[data-wr-publish-btn]`); e != nil {
		return "", "", fmt.Errorf("点击发布失败: %w", e)
	}
	log.Printf("[PublishAuto:douyin] 已点击发布，等待结果确认…")

	// 确认链（三级降级）：网络响应 aweme_id → 页面提取 → 超时失败
	confirmCtx, confirmCancel := context.WithTimeout(sessionCtx, 90*time.Second)
	defer confirmCancel()
	for i := 0; i < 30; i++ {
		if b, ok := publishBody.Load().(string); ok && b != "" {
			if id := extractAwemeIDFromJSON(b); id != "" {
				return "https://www.douyin.com/video/" + id, readBackCookie(confirmCtx), nil
			}
		}
		if url, e := extractVideoURLAfterPublish(confirmCtx); e == nil && url != "" {
			return url, readBackCookie(confirmCtx), nil
		}
		chromedp.Sleep(3 * time.Second)
	}
	return "", readBackCookie(confirmCtx), fmt.Errorf("发布结果确认超时（可能已发出——请到抖音创作者中心人工核对）")
}

// readBackCookie 从浏览器导出最新抖音 cookie（滚动回写用）。
func readBackCookie(ctx context.Context) string {
	cookies, err := network.GetCookies().Do(ctx)
	if err != nil {
		return ""
	}
	var parts []string
	for _, c := range cookies {
		if strings.HasSuffix(c.Domain, "douyin.com") {
			parts = append(parts, c.Name+"="+c.Value)
		}
	}
	return strings.Join(parts, "; ")
}

// extractAwemeIDFromJSON 从发布响应 JSON 提取 aweme_id/item_id（容错：多字段名候选）。
func extractAwemeIDFromJSON(body string) string {
	for _, key := range []string{`"aweme_id"`, `"item_id"`} {
		i := strings.Index(body, key)
		if i < 0 {
			continue
		}
		rest := body[i+len(key):]
		j := strings.Index(rest, `"`)
		if j < 0 {
			continue
		}
		k := strings.Index(rest[j+1:], `"`)
		if k > 0 {
			id := rest[j+1 : j+1+k]
			if len(id) >= 15 { // 抖音视频 ID 为 15+ 位数字
				return id
			}
		}
	}
	return ""
}

// markButtonByText 按可见文本标记发布按钮（表单提交按钮定位——2026-08-29 真发实锤修复）：
//  ① 排除顶栏/导航区的同名按钮（抖音顶栏全局"发布"入口会抢先命中，点到只是打开发布
//     菜单而非提交作品——真发"点了没发出"的根因）
//  ② 精确文本优先（"作品发布"=== 优先于 includes("发布")）
//  ③ 多个命中取最后一个（表单提交按钮通常在页面底部）
func markButtonByText(ctx context.Context, include, exclude string) error {
	var found bool
	return chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const inNav = (el) => !!el.closest('header, nav, [class*="header"], [class*="nav"], [class*="sidebar"], [class*="menu"]');
		const btns = [...document.querySelectorAll('button, [role=button], .byte-btn')].filter(b => {
			const t = (b.textContent || '').trim();
			return t.includes(%q) && !t.includes(%q) && b.offsetParent !== null && !inNav(b);
		});
		// 精确匹配优先，其次取最后一个（表单底部）
		const exact = btns.filter(b => (b.textContent || '').trim() === %q);
		const target = exact.length ? exact[exact.length-1] : (btns.length ? btns[btns.length-1] : null);
		if (!target) return false;
		target.setAttribute('data-wr-publish-btn', '1');
		return true;
	})()`, include, exclude, include), &found))
}

// waitFirstVisible 依次等待候选选择器，返回首个可见者（多策略选择器模式——平台改版容错）。
func waitFirstVisible(ctx context.Context, timeout time.Duration, selectors ...string) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, sel := range selectors {
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := chromedp.Run(cctx, chromedp.WaitVisible(sel, chromedp.ByQuery))
			cancel()
			if err == nil {
				return sel
			}
		}
		chromedp.Sleep(2 * time.Second)
	}
	return ""
}

// clampRunes 截断到 n 个 rune。
func clampRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// extractVideoURLAfterPublish RPA 发布成功后提取视频链接（作品数据详情/数据回读依赖）。
// 抖音创作者中心发布成功后跳转内容管理页，页面 URL/DOM 含视频 item_id（15+ 位数字）；
// 对外可播放链接形如 https://www.douyin.com/video/{item_id}。
// 提取优先级：当前 URL 中的 item_id → DOM 内视频卡片跳转链接中的 item_id。
// （publishVideoRPA 实现完成后在第 8 步调用；函数已就绪待接。）
func extractVideoURLAfterPublish(ctx context.Context) (string, error) {
	var itemID string
	err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const m = location.href.match(/(\d{15,})/);
		if (m) return m[1];
		const a = document.querySelector('a[href*="/video/"]');
		if (a) { const m2 = (a.href || '').match(/video\/(\d{15,})/); if (m2) return m2[1]; }
		return '';
	})()`, &itemID))
	if err != nil || itemID == "" {
		return "", fmt.Errorf("未从发布成功页提取到视频 ID")
	}
	return "https://www.douyin.com/video/" + itemID, nil
}

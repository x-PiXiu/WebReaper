// Package account 实现多平台发布账号域用例。
//
// 用例编排（应用级业务规则），只依赖 port 接口和 domain 实体。
//
// 核心用例：
//   - AccountUseCase：账号扫码绑定 / cookie 加密入库 / 列表 / 解绑
//   - PublishUseCase：半自动发布编排（选通道→借账号→生成预填URL→记录任务）
//
// 整洁架构要点：
//   - 用例层只依赖 port.QRLoginSession / port.CookieVault / port.PublishChannel 接口。
//   - 浏览器自动化（chromedp）、加密（AES-GCM）、平台差异全部关在适配器层。
//   - 换浏览器引擎/加密方案/平台 = 重写适配器，用例层零改动。
package account

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ============ 账号管理用例 ============

// AccountUseCase 账号扫码绑定 / 官方 OAuth 授权绑定 / 列表 / 解绑。
type AccountUseCase struct {
	accountRepo port.AccountRepository
	qrLogin     port.QRLoginSession // 扫码登录会话（浏览器自动化，cookie 通道）
	vault       port.CookieVault    // cookie/token 加密存储
	oauth       port.OAuthProvider  // 平台官方 OAuth（API 通道；nil=未配置，仅扫码可用）
	stateCodec  port.OAuthStateCodec
}

func NewAccountUseCase(ar port.AccountRepository, qr port.QRLoginSession, vault port.CookieVault) *AccountUseCase {
	return &AccountUseCase{accountRepo: ar, qrLogin: qr, vault: vault}
}

// SetOAuth 注入官方 OAuth 授权（可选；抖音开放平台等）。
func (uc *AccountUseCase) SetOAuth(oauth port.OAuthProvider, codec port.OAuthStateCodec) {
	uc.oauth = oauth
	uc.stateCodec = codec
}

// ---- 官方 OAuth 授权绑定（抖音 API 通道）----

// BuildOAuthURL 生成官方授权页地址（用户在授权页扫码确认，抖音回调后自动完成绑定）。
func (uc *AccountUseCase) BuildOAuthURL(tenantID, userID string) (string, error) {
	if uc.oauth == nil || uc.stateCodec == nil {
		return "", fmt.Errorf("官方 OAuth 授权未配置（需 DOUYIN_CLIENT_KEY/SECRET）")
	}
	state := uc.stateCodec.SignState(tenantID + "|" + userID)
	return uc.oauth.ConnectURL(state), nil
}

// HandleOAuthCallback 处理抖音授权回调：验 state → code 换 token → 拉用户信息 → 落库账号。
// 返回 (账号ID, 显示名)。open_id 已绑定时覆盖更新（同一抖音号重新授权=续期，不重复建号）。
func (uc *AccountUseCase) HandleOAuthCallback(ctx context.Context, code, state string) (string, string, error) {
	if uc.oauth == nil || uc.stateCodec == nil {
		return "", "", fmt.Errorf("官方 OAuth 授权未配置")
	}
	if code == "" || state == "" {
		return "", "", fmt.Errorf("回调缺少 code/state 参数")
	}
	payload, err := uc.stateCodec.VerifyState(state)
	if err != nil {
		return "", "", fmt.Errorf("授权状态校验失败（请重新发起绑定）: %w", err)
	}
	tenantID, _, _ := strings.Cut(payload, "|")

	token, err := uc.oauth.ExchangeCode(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("授权码换取 token 失败: %w", err)
	}

	// token 加密落库（与 cookie 同一 AES-GCM 保险库）
	encAccess, aErr := uc.vault.Encrypt(token.AccessToken)
	encRefresh, rErr := uc.vault.Encrypt(token.RefreshToken)
	if aErr != nil || rErr != nil {
		return "", "", fmt.Errorf("加密 token 失败: %v / %v", aErr, rErr)
	}

	// 显示名：拉抖音昵称，失败降级平台名
	displayName := platformDisplayName("douyin")
	if info, iErr := uc.oauth.UserInfo(ctx, token.AccessToken, token.OpenID); iErr == nil && info.Nickname != "" {
		displayName = info.Nickname
	}

	now := time.Now()
	acc := entity.Account{
		ID:              fmt.Sprintf("acc-%d", now.UnixNano()),
		TenantID:        tenantID,
		Platform:        "douyin",
		DisplayName:     displayName,
		Health:          entity.AccountHealthActive,
		LoginMethod:     "douyin",
		AuthType:        entity.AccountAuthOAuth,
		AccessTokenEnc:  encAccess,
		RefreshTokenEnc: encRefresh,
		OpenID:          token.OpenID,
		ExpiresAt:       now.Add(time.Duration(max(token.ExpiresIn, 3600)) * time.Second),
		// 抖音 refresh_token 30 天（可续期 5 次）；响应未带时长时按 30 天计
		RefreshExpiresAt: now.Add(time.Duration(max(token.RefreshExpiresIn, 30*24*3600)) * time.Second),
		BoundAt:          now,
		LastUsedAt:       now,
	}

	// 同一 open_id 重新授权 = 续期：复用原账号记录，避免账号池重复
	if existing, fErr := uc.accountRepo.FindByOpenID(ctx, tenantID, "douyin", token.OpenID); fErr == nil && existing.ID != "" {
		acc.ID = existing.ID
		acc.BoundAt = existing.BoundAt
	}
	if sErr := uc.accountRepo.Save(ctx, acc); sErr != nil {
		return "", "", fmt.Errorf("保存授权账号失败: %w", sErr)
	}
	return acc.ID, acc.DisplayName, nil
}

// StartQRLogin 启动扫码登录，返回会话 ID（二维码图片通过 PollQRLogin 异步获取）。
// method 指定登录方式（如 wechat/qq/weibo），空=平台默认扫码。
func (uc *AccountUseCase) StartQRLogin(ctx context.Context, platform, method string) (sessionID string, err error) {
	if uc.qrLogin == nil {
		return "", fmt.Errorf("扫码登录未启用（浏览器自动化适配器未配置）")
	}
	return uc.qrLogin.StartLogin(ctx, platform, method)
}

// PollQRLoginResult 是轮询扫码状态的完整结果（含二维码图片）。
type PollQRLoginResult struct {
	Status      string    // preparing / waiting / scanned / success / expired / error
	QRImage     string    // 二维码截图 base64（waiting 阶段有值）
	AccountID   string    // 仅 success 时有值
	AccountName string    // 仅 success 时有值（登录后的账号显示名）
	ExpiresAt   time.Time // 仅 success 时有值（cookie 过期时间）
}

// PollQRLogin 轮询扫码状态。登录成功时自动创建账号并加密存 cookie。
// method 指定登录方式（wechat/qq/weibo），用于记录账号的登录形式。
func (uc *AccountUseCase) PollQRLogin(ctx context.Context, tenantID, sessionID, platform, method string) (PollQRLoginResult, error) {
	if uc.qrLogin == nil {
		return PollQRLoginResult{Status: "error"}, fmt.Errorf("扫码登录未启用")
	}
	result, err := uc.qrLogin.PollStatus(ctx, sessionID)
	if err != nil {
		return PollQRLoginResult{Status: "error"}, err
	}

	// 非 success 状态：透传状态和二维码图片
	if result.Status != "success" {
		return PollQRLoginResult{Status: result.Status, QRImage: result.QRImage}, nil
	}

	// 登录成功：加密 cookie 并创建账号
	if result.Cookie == "" {
		return PollQRLoginResult{Status: "error"}, fmt.Errorf("登录成功但 cookie 为空")
	}
	encCookie, encErr := uc.vault.Encrypt(result.Cookie)
	if encErr != nil {
		return PollQRLoginResult{Status: "error"}, fmt.Errorf("加密 cookie 失败: %w", encErr)
	}
	now := time.Now()
	// 账号名：优先用从页面提取的，降级用平台名
	displayName := result.AccountName
	if displayName == "" {
		displayName = platformDisplayName(platform)
	}
	acc := entity.Account{
		ID:              fmt.Sprintf("acc-%d", now.UnixNano()),
		TenantID:        tenantID,
		Platform:        platform,
		DisplayName:     displayName,
		CookieEncrypted: encCookie,
		Health:          entity.AccountHealthActive,
		LoginMethod:     method,
		ExpiresAt:       result.ExpiresAt,
		BoundAt:         now,
		LastUsedAt:      now,
	}
	if saveErr := uc.accountRepo.Save(ctx, acc); saveErr != nil {
		return PollQRLoginResult{Status: "error"}, fmt.Errorf("保存账号失败: %w", saveErr)
	}
	// 如果账号名为空，等几秒让后台 goroutine 提取账号名后再查一次
	if result.AccountName == "" {
		for i := 0; i < 5; i++ {
			time.Sleep(2 * time.Second)
			r2, _ := uc.qrLogin.PollStatus(ctx, sessionID)
			if r2.AccountName != "" {
				displayName = r2.AccountName
				acc.DisplayName = displayName
				_ = uc.accountRepo.Save(ctx, acc) // 更新账号名
				break
			}
		}
	}
	// 登录成功后关闭浏览器会话
	_ = uc.qrLogin.Cleanup(ctx, sessionID)
	return PollQRLoginResult{Status: "success", AccountID: acc.ID, AccountName: displayName, ExpiresAt: result.ExpiresAt}, nil
}

// CleanupSession 关闭扫码会话（用户取消或超时）。
func (uc *AccountUseCase) CleanupSession(ctx context.Context, sessionID string) error {
	if uc.qrLogin == nil {
		return nil
	}
	return uc.qrLogin.Cleanup(ctx, sessionID)
}

// List 列出租户的全部账号。
func (uc *AccountUseCase) List(ctx context.Context, tenantID string) ([]entity.Account, error) {
	return uc.accountRepo.ListByTenant(ctx, tenantID)
}

// Delete 解绑账号。
func (uc *AccountUseCase) Delete(ctx context.Context, tenantID, accountID string) error {
	return uc.accountRepo.Delete(ctx, tenantID, accountID)
}

// CheckAccountHealth 检查所有账号的凭据过期状态（OAuth 账号双时钟自动管理）。
//   - access_token 时钟（ExpiresAt，抖音 15 天）：到期前 48h 用 refresh_token 刷新
//   - refresh_token 时钟（RefreshExpiresAt，抖音 30 天可续 5 次）：关闭前 7 天续期
//     （RenewRefreshToken；需控制台申请 renew_refresh_token 权限，无权限则忽略并静默滑向重新授权）
//   - 双时钟都失败/续期额度耗尽（单次授权最长 195 天）→ 标记 expired，用户重新授权
// cookie 账号：过期直接标记 expired，前端展示"重新绑定"。
// 由定时任务每 10 分钟调用一次。
func (uc *AccountUseCase) CheckAccountHealth(ctx context.Context) {
	accounts, err := uc.accountRepo.ListAll(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	for _, acc := range accounts {
		if acc.Health != entity.AccountHealthActive {
			continue
		}
		if !acc.IsOAuth() {
			if acc.ExpiresAt.IsZero() || now.After(acc.ExpiresAt) {
				if !acc.ExpiresAt.IsZero() {
					_ = uc.accountRepo.UpdateHealth(ctx, acc.ID, entity.AccountHealthExpired)
				}
			}
			continue
		}
		if uc.oauth == nil {
			continue
		}
		// refresh_token 续期窗口快关（7 天内）→ 先续期（旧 token 失效，必须回写新 token）
		if !acc.RefreshExpiresAt.IsZero() && now.After(acc.RefreshExpiresAt.Add(-7*24*time.Hour)) {
			uc.renewOAuthRefreshToken(ctx, acc)
			// 回读续期后的新窗口再判断 access_token
			if updated, fErr := uc.accountRepo.FindByID(ctx, acc.TenantID, acc.ID); fErr == nil && updated.RefreshExpiresAt.After(now) {
				acc = updated
			}
		}
		// access_token 到期前 48h → 刷新
		if !acc.ExpiresAt.IsZero() && now.After(acc.ExpiresAt) {
			_ = uc.accountRepo.UpdateHealth(ctx, acc.ID, entity.AccountHealthExpired)
			continue
		}
		if !acc.ExpiresAt.IsZero() && now.After(acc.ExpiresAt.Add(-48*time.Hour)) {
			uc.refreshOAuthAccount(ctx, acc)
		}
	}
}

// renewOAuthRefreshToken 续期 refresh_token（旧 token 立即失效；无 renew 权限/额度耗尽则不动，
// 30 天窗口自然滑到 expired 由用户重新授权）。
func (uc *AccountUseCase) renewOAuthRefreshToken(ctx context.Context, acc entity.Account) {
	refreshToken, err := uc.vault.Decrypt(acc.RefreshTokenEnc)
	if err != nil || refreshToken == "" {
		return
	}
	renewed, err := uc.oauth.RenewRefreshToken(ctx, refreshToken)
	if err != nil {
		// 无 renew_refresh_token 权限（个体户常见）或 5 次额度耗尽——不标记过期，
		// 让 refresh_token 30 天窗口自然到期（期间 access_token 刷新仍可用）
		return
	}
	encRefresh, rErr := uc.vault.Encrypt(renewed.RefreshToken)
	if rErr != nil {
		return
	}
	acc.RefreshTokenEnc = encRefresh
	if !acc.RefreshExpiresAt.IsZero() { // 基于旧窗口顺延，避免响应时长缺失时丢基准
		acc.RefreshExpiresAt = acc.RefreshExpiresAt.Add(time.Duration(max(renewed.RefreshExpiresIn, 30*24*3600)) * time.Second)
	} else {
		acc.RefreshExpiresAt = time.Now().Add(time.Duration(max(renewed.RefreshExpiresIn, 30*24*3600)) * time.Second)
	}
	_ = uc.accountRepo.Save(ctx, acc)
}

// refreshOAuthAccount 用 refresh_token 刷新 access_token 并回写（抖音刷新 access_token
// 不改变 refresh_token 有效期；若响应轮换了 refresh_token 则一并回写）。
func (uc *AccountUseCase) refreshOAuthAccount(ctx context.Context, acc entity.Account) {
	refreshToken, err := uc.vault.Decrypt(acc.RefreshTokenEnc)
	if err != nil || refreshToken == "" {
		_ = uc.accountRepo.UpdateHealth(ctx, acc.ID, entity.AccountHealthExpired)
		return
	}
	token, err := uc.oauth.RefreshToken(ctx, refreshToken)
	if err != nil {
		// refresh_token 失效（error_code 10010）：需要用户重新授权
		_ = uc.accountRepo.UpdateHealth(ctx, acc.ID, entity.AccountHealthExpired)
		return
	}
	encAccess, aErr := uc.vault.Encrypt(token.AccessToken)
	if aErr != nil {
		return // 加密失败保持现状，下次巡检重试
	}
	acc.AccessTokenEnc = encAccess
	if token.RefreshToken != "" && token.RefreshToken != refreshToken {
		if encRefresh, rErr := uc.vault.Encrypt(token.RefreshToken); rErr == nil {
			acc.RefreshTokenEnc = encRefresh // 平台轮换了 refresh_token，必须回写
		}
	}
	acc.ExpiresAt = time.Now().Add(time.Duration(max(token.ExpiresIn, 3600)) * time.Second)
	_ = uc.accountRepo.Save(ctx, acc)
}

// platformDisplayName 平台显示名。
func platformDisplayName(platform string) string {
	switch platform {
	case "zhihu":
		return "知乎账号"
	case "xiaohongshu":
		return "小红书账号"
	case "douyin":
		return "抖音账号"
	case "kuaishou":
		return "快手账号"
	default:
		return platform + " 账号"
	}
}

// ============ 发布用例（半自动 + 全自动） ============

// PublishUseCase 发布编排。
// semi-auto: 生成预填 URL，用户手动确认
// auto: 后台 chromedp 自动填充内容并点击发送
type PublishUseCase struct {
	jobRepo        port.PublishJobRepository
	registry       port.PublishChannelRegistry
	accountRepo    port.AccountRepository // 全自动模式需要查账号拿 cookie
	vault          port.CookieVault       // 全自动模式需要解密 cookie
	monitorTrigger port.MonitorTrigger    // 可选：发布效果追踪（发布后触发监测对比提及率）
	accountPool    port.AccountPool       // 可选：账号池调度（自动选最优账号）
	publicBaseURL  string                 // 公开站根地址（发布内容尾部带公开站链接用）
}

func NewPublishUseCase(jr port.PublishJobRepository, reg port.PublishChannelRegistry, ar port.AccountRepository, vault port.CookieVault) *PublishUseCase {
	return &PublishUseCase{jobRepo: jr, registry: reg, accountRepo: ar, vault: vault}
}

// SetMonitorTrigger 注入监测触发器（可选；发布效果追踪用）。
func (uc *PublishUseCase) SetMonitorTrigger(mt port.MonitorTrigger) {
	uc.monitorTrigger = mt
}

// SetAccountPool 注入账号池（可选；自动选号用）。
func (uc *PublishUseCase) SetAccountPool(ap port.AccountPool) {
	uc.accountPool = ap
}

// SetPublicBaseURL 注入公开站根地址（发布内容尾部带公开站链接，加速爬虫发现）。
func (uc *PublishUseCase) SetPublicBaseURL(baseURL string) {
	uc.publicBaseURL = baseURL
}

// ChannelCapabilities 发布通道能力清单（前端能力驱动的数据源：
// 选内容形态 → 过滤可用平台 → 按约束动态生成检查清单）。
// 新平台 = 注册新 Channel（声明 DisplayName/SupportedContentTypes/Constraints），
// 本清单自动出现，前端零改动。
func (uc *PublishUseCase) ChannelCapabilities() []entity.ChannelInfo {
	if uc.registry == nil {
		return nil
	}
	channels := uc.registry.List()
	out := make([]entity.ChannelInfo, 0, len(channels))
	for _, ch := range channels {
		info := entity.ChannelInfo{
			Platform:     ch.Platform(),
			Name:         ch.Platform(),
			ContentTypes: ch.SupportedContentTypes(),
			SemiAuto:     true,
		}
		if _, ok := ch.(port.AutoPublishChannel); ok {
			info.Auto = true
		}
		if ip, ok := ch.(port.ChannelInfoProvider); ok {
			info.Name = ip.DisplayName()
			info.Constraints = ip.Constraints()
		}
		if len(info.ContentTypes) == 0 {
			info.ContentTypes = ch.SupportedMediaType() // 向后兼容兜底
		}
		out = append(out, info)
	}
	return out
}

// PublishInput 发布请求输入。
type PublishInput struct {
	TenantID  string
	AccountID string
	Platform  string
	ContentID string
	BrandID   string // 品牌ID（发布效果追踪用）
	Title     string
	Content   string
	Mode      string // semi-auto / auto
	// ScheduledAt 排期发布时间（零值 = 立即发布；将来时间 = 定时发送）。
	ScheduledAt time.Time
	// StoreAddress 门店地址（本地生活 P3：内容层本地曝光信号）。
	StoreAddress string
	// ContentType 内容形态（image/video/article/audio）；空=平台默认
	ContentType string
	// MediaURLs 媒体文件 URL 列表（图文=图片、视频=mp4、音频=mp3）
	MediaURLs []string
	// CoverURL 封面图 URL
	CoverURL string
}

// appendPublicLink 在发布内容尾部追加公开站链接（纯函数，可单测）。
//
// 设计动机（外链加速爬虫发现）：
//   搜索引擎爬虫通过外链发现新页面——发布到知乎的内容带公开站链接，
//   知乎页面被索引时爬虫会顺着链接发现公开站文章页。
//
// 平台差异（防封号红线）：
//   - zhihu：追加 markdown 风格链接（知乎支持）
//   - xiaohongshu 等其他平台：不追加（外部链接触发限流/违规判定）
func appendPublicLink(content, platform, baseURL, contentID string) string {
	if contentID == "" || baseURL == "" {
		return content
	}
	if platform != "zhihu" {
		return content
	}
	url := strings.TrimRight(baseURL, "/") + "/public/articles/" + contentID
	return content + "\n\n---\n> 本文源自：" + url
}

// Publish 执行发布：按 mode 分流半自动/全自动。
func (uc *PublishUseCase) Publish(ctx context.Context, in PublishInput) (entity.PublishJob, error) {
	if uc.registry == nil {
		return entity.PublishJob{}, fmt.Errorf("发布通道未配置")
	}

	mode := in.Mode
	if mode == "" {
		mode = entity.PublishModeSemiAuto
	}

	ch, err := uc.registry.Get(in.Platform)
	if err != nil {
		return entity.PublishJob{}, fmt.Errorf("获取发布通道失败: %w", err)
	}

	// 发布前处理：Markdown 转纯文本（社媒平台不渲染 Markdown）。
	// 先过滤 think 标签（兜底：历史内容可能在生成期未被过滤，防止思考过程泄漏到平台）。
	publishContent := pkg.MarkdownToPlainText(pkg.StripThinkTags(in.Content))
	// 公开站链接兜尾（平台差异：知乎加外链加速爬虫发现；小红书加外链易触发限流，不加）
	publishContent = appendPublicLink(publishContent, in.Platform, uc.publicBaseURL, in.ContentID)
	// 门店地址注入（本地生活 P3：内容层本地曝光信号——正文尾部附"📍 地址"。
	// 全平台安全（纯文本内容，不触平台规则）；平台"添加定位"能力见 P4，暂缓）。
	if in.StoreAddress != "" {
		publishContent = strings.TrimRight(publishContent, "\n") + "\n\n📍 门店地址：" + in.StoreAddress
	}
	// 标题：优先用调用方传入（内容工作台已带标题），空或过短则从正文提取
	publishTitle := pkg.StripThinkTags(in.Title)
	if publishTitle == "" || len(publishTitle) < 4 {
		publishTitle = pkg.ExtractTitle(publishContent)
	}

	now := time.Now()
	job := entity.PublishJob{
		ID:        fmt.Sprintf("pj-%d", now.UnixNano()),
		TenantID:  in.TenantID,
		AccountID: in.AccountID,
		Platform:  in.Platform,
		ContentID: in.ContentID,
		BrandID:   in.BrandID,
		Title:     publishTitle,
		Content:   publishContent,
		Mode:      mode,
		Status:    entity.PublishStatusPending,
		CreatedAt: now,
		StoreAddress: in.StoreAddress,
		ContentType:  in.ContentType,
		MediaURLs:    in.MediaURLs,
		CoverURL:     in.CoverURL,
	}

	// 定时发送：ScheduledAt 在未来 → 仅落库 pending，到期由调度任务执行发布
	if in.ScheduledAt.After(now) {
		job.ScheduledAt = in.ScheduledAt
		if err := uc.jobRepo.Save(ctx, job); err != nil {
			return entity.PublishJob{}, fmt.Errorf("保存排期任务失败: %w", err)
		}
		return job, nil
	}

	// 发布前基线：取品牌最近一次监测的平均提及率（真实历史数据；无记录保持 0）
	if in.BrandID != "" && uc.monitorTrigger != nil {
		if base, bErr := uc.monitorTrigger.BaselineRate(ctx, in.TenantID, in.BrandID); bErr == nil {
			job.PreMentionRate = base
		}
	}

	if mode == entity.PublishModeAuto {
		// 全自动模式
		return uc.publishAuto(ctx, job, ch)
	}

	// 半自动模式（旧逻辑不变）
	acc := entity.Account{ID: in.AccountID, Platform: in.Platform}
	externalURL, err := ch.PublishSemiAuto(ctx, job, acc)
	if err != nil {
		job.Status = entity.PublishStatusFailed
		job.ErrorMsg = err.Error()
		_ = uc.jobRepo.Save(ctx, job)
		return job, fmt.Errorf("生成发布链接失败: %w", err)
	}
	job.ExternalURL = externalURL
	if err := uc.jobRepo.Save(ctx, job); err != nil {
		return job, fmt.Errorf("保存发布任务失败: %w", err)
	}
	return job, nil
}

// publishAuto 全自动发布：查账号→解密cookie→后台goroutine执行chromedp。
func (uc *PublishUseCase) publishAuto(ctx context.Context, job entity.PublishJob, ch port.PublishChannel) (entity.PublishJob, error) {
	// 检查通道是否支持全自动
	autoCh, ok := ch.(port.AutoPublishChannel)
	if !ok {
		// 降级到半自动
		job.Mode = entity.PublishModeSemiAuto
		acc := entity.Account{ID: job.AccountID, Platform: job.Platform}
		externalURL, err := ch.PublishSemiAuto(ctx, job, acc)
		if err != nil {
			job.Status = entity.PublishStatusFailed
			job.ErrorMsg = "平台不支持全自动发布，半自动也失败: " + err.Error()
			_ = uc.jobRepo.Save(ctx, job)
			return job, fmt.Errorf("平台不支持全自动发布")
		}
		job.ExternalURL = externalURL
		_ = uc.jobRepo.Save(ctx, job)
		return job, nil
	}

	// 查账号——如果没指定 AccountID 且有账号池，自动选最优账号
	var acc entity.Account
	var err error
	if job.AccountID == "" && uc.accountPool != nil {
		// 自动选号（最久未使用优先）
		acc, err = uc.accountPool.Acquire(ctx, job.TenantID, job.Platform)
		if err != nil {
			job.Status = entity.PublishStatusFailed
			job.ErrorMsg = "自动选号失败: " + err.Error()
			_ = uc.jobRepo.Save(ctx, job)
			return job, fmt.Errorf("自动选号失败: %w", err)
		}
		job.AccountID = acc.ID // 记录实际使用的账号
	} else {
		// 手动指定账号
		acc, err = uc.accountRepo.FindByID(ctx, job.TenantID, job.AccountID)
		if err != nil {
			job.Status = entity.PublishStatusFailed
			job.ErrorMsg = "账号不存在: " + err.Error()
			_ = uc.jobRepo.Save(ctx, job)
			return job, fmt.Errorf("账号不存在: %w", err)
		}
	}

	// 解密 cookie
	cookie, err := uc.vault.Decrypt(acc.CookieEncrypted)
	if err != nil {
		job.Status = entity.PublishStatusFailed
		job.ErrorMsg = "解密cookie失败: " + err.Error()
		_ = uc.jobRepo.Save(ctx, job)
		return job, fmt.Errorf("解密cookie失败: %w", err)
	}

	// 保存 job（status=running），立即返回给前端
	job.Status = entity.PublishStatusRunning
	if err := uc.jobRepo.Save(ctx, job); err != nil {
		return job, fmt.Errorf("保存发布任务失败: %w", err)
	}

	// 后台 goroutine 执行全自动发布 + 发布效果追踪
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		articleURL, err := autoCh.PublishAuto(bgCtx, job, cookie)
		if err != nil {
			_ = uc.jobRepo.UpdateStatus(bgCtx, job.TenantID, job.ID, entity.PublishStatusFailed, "", err.Error())
			return
		}

		// 发布成功：更新状态 + 记录发布时间 + 文章URL
		job.Status = entity.PublishStatusPublished
		job.ExternalURL = articleURL
		job.PublishedAt = time.Now()
		_ = uc.jobRepo.Save(bgCtx, job)

		// 归还账号到池（更新 LastUsedAt）
		if uc.accountPool != nil {
			_ = uc.accountPool.Release(bgCtx, acc)
		}

		// 发布效果追踪：延迟触发监测，对比前后提及率
		if uc.monitorTrigger != nil {
			uc.trackPublishEffect(bgCtx, job)
		}
	}()

	return job, nil
}

// trackPublishEffect 发布效果追踪：延迟触发监测，对比发布前后提及率。
// trackPublishEffect 发布效果追踪：触发监测，对比发布前后提及率。
//
// 说明：不等待"收录延迟"——AgentProbe 是实时搜索+综合（Agent 现场爬取全网），
// 不依赖真实 AI 引擎的索引收录，发布即可测；若未来接 DirectProbe（真实引擎直测），
// 再按引擎收录周期引入延迟。
func (uc *PublishUseCase) trackPublishEffect(ctx context.Context, job entity.PublishJob) {
	if job.BrandID == "" {
		return
	}

	// 触发监测，获取发布后提及率
	postRate, err := uc.monitorTrigger.TriggerMonitor(ctx, job.TenantID, job.BrandID)
	if err != nil {
		// 失败不静默：记录到 job，前端发布记录可见
		_ = uc.jobRepo.UpdateStatus(ctx, job.TenantID, job.ID, entity.PublishStatusPublished,
			job.ExternalURL, "效果追踪失败: "+err.Error())
		return
	}

	// 更新 job 的发布后提及率
	job.PostMentionRate = postRate
	_ = uc.jobRepo.Save(ctx, job)
}

// GetJobStatus 查询单个发布任务状态（前端轮询用）。
func (uc *PublishUseCase) GetJobStatus(ctx context.Context, tenantID, jobID string) (entity.PublishJob, error) {
	jobs, err := uc.jobRepo.ListByTenant(ctx, tenantID, 100)
	if err != nil {
		return entity.PublishJob{}, err
	}
	for _, j := range jobs {
		if j.ID == jobID {
			return j, nil
		}
	}
	return entity.PublishJob{}, fmt.Errorf("job not found: %s", jobID)
}

// ExecuteScheduledJob 执行到期的排期发布任务（调度任务调用）。
// 按 job.Mode 分派：全自动 → publishAuto；半自动 → 生成预填链接。
// 与 Publish 的立即执行路径共用同一分派逻辑（无重复实现）。
func (uc *PublishUseCase) ExecuteScheduledJob(ctx context.Context, tenantID, jobID string) (entity.PublishJob, error) {
	job, err := uc.GetJobStatus(ctx, tenantID, jobID)
	if err != nil {
		return entity.PublishJob{}, err
	}
	if job.Status != entity.PublishStatusPending {
		return job, nil // 非待发布（已执行/已取消）跳过
	}
	ch, err := uc.registry.Get(job.Platform)
	if err != nil {
		return entity.PublishJob{}, fmt.Errorf("获取发布通道失败: %w", err)
	}
	job.Status = entity.PublishStatusRunning
	_ = uc.jobRepo.Save(ctx, job)

	if job.Mode == entity.PublishModeAuto {
		return uc.publishAuto(ctx, job, ch)
	}
	// 半自动：生成预填链接（与 Publish 半自动路径一致）
	acc := entity.Account{ID: job.AccountID, Platform: job.Platform}
	externalURL, err := ch.PublishSemiAuto(ctx, job, acc)
	if err != nil {
		job.Status = entity.PublishStatusFailed
		job.ErrorMsg = err.Error()
		_ = uc.jobRepo.Save(ctx, job)
		return job, fmt.Errorf("生成发布链接失败: %w", err)
	}
	job.ExternalURL = externalURL
	job.Status = entity.PublishStatusPending // 半自动仍待用户确认
	_ = uc.jobRepo.Save(ctx, job)
	return job, nil
}

// ReMonitor 发布效果复测：重新触发品牌监测，更新发布后提及率。
//
// 使用场景（GEO 收录周期）：
//   内容发布后搜索引擎收录需要数天~数周，发布瞬间的追踪看不到收录效果。
//   建议发布 1-2 周后点"复测"——重新监测品牌提及率并更新 post_mention_rate，
//   与 pre_mention_rate（发布前基线）对比验证"发布 → 收录 → 被引用"闭环。
func (uc *PublishUseCase) ReMonitor(ctx context.Context, tenantID, jobID string) (entity.PublishJob, error) {
	job, err := uc.GetJobStatus(ctx, tenantID, jobID)
	if err != nil {
		return entity.PublishJob{}, err
	}
	if uc.monitorTrigger == nil {
		return entity.PublishJob{}, fmt.Errorf("监测触发器未配置")
	}
	postRate, err := uc.monitorTrigger.TriggerMonitor(ctx, job.TenantID, job.BrandID)
	if err != nil {
		return entity.PublishJob{}, fmt.Errorf("复测失败: %w", err)
	}
	job.PostMentionRate = postRate
	if err := uc.jobRepo.Save(ctx, job); err != nil {
		return entity.PublishJob{}, fmt.Errorf("保存复测结果失败: %w", err)
	}
	return job, nil
}

// ListJobs 列出发布任务记录。
func (uc *PublishUseCase) ListJobs(ctx context.Context, tenantID string, limit int) ([]entity.PublishJob, error) {
	return uc.jobRepo.ListByTenant(ctx, tenantID, limit)
}

// MarkPublished 用户在平台确认发布后，前端调此方法标记任务为已发布。
func (uc *PublishUseCase) MarkPublished(ctx context.Context, tenantID, jobID string) error {
	return uc.jobRepo.UpdateStatus(ctx, tenantID, jobID, entity.PublishStatusPublished, "", "")
}

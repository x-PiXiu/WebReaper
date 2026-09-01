package generation

// voice_materializer.go —— 31号 L2：音色物化层（"样本为源、注册为缓存"）。
//
// 定位：所有需要音色的生成请求的唯一入口（EndpointVoiceMaterializer）。
// 两种物化形态：
//   - form=sample  → 已由 maybeRewriteSampleSynthesis 承担（样本合成通道，缺口C）
//   - form=vidu_id → ensureViduVoiceID（本文件）：lip_sync 文本驱动的唯一 ID 刚性场景
//
// 设计要点（Docs/Plans/31-音色注册与lip_sync音色保障设计.md）：
//   - Vidu 注册是"可重建的缓存"：同 ID 复注册幂等，7 天不用即删 → 窗口判定 + 按需重建
//   - 宁报错不变声：所有不可用场景显式失败（无静默替换）
//   - 租户归属校验：clone 行仅归属租户可用（修复 FindByVoiceID 无租户过滤的越权缺口）
//   - singleflight：同音色并发到期重建只触发一次（进程内；多实例幂等兜底）

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"webreaper/internal/domain/entity"
)

// viduRegGroup 同 ID 注册的并发去重（31号 §4.2；进程内——多实例各注册一次幂等无害）。
var viduRegGroup singleflight.Group

const (
	// defaultViduVoiceWindowHours 默认注册缓存窗口：上游 7 天过期留 1 天余量。
	defaultViduVoiceWindowHours = 144
	// defaultVoiceAuditionText 注册/续期试听文案（Vidu audio-clone text 必填 ≤1000 字）。
	defaultVoiceAuditionText = "你好，欢迎体验我们的服务。"
	// registerTimeout 注册调用预算（暂定 30s——上游实测时延后校准，见 31号 §六 #3）。
	registerTimeout = 30 * time.Second
)

// ensureViduVoiceID L2 form=vidu_id：保障 voiceID 在 Vidu 侧可用（注册/续期）后原样返回。
//
// 检查链：特性开关 → 库内查询（不在库=上游原生 ID 直传）→ 租户归属 → 停用状态
// → 窗口命中（零开销）→ 样本就绪 → singleflight 幂等注册。
// 失败语义（宁报错不变声）：
//   - "音色不存在"（不泄露他人音色存在性）
//   - "音色已停用"
//   - "音色样本转存未完成，请稍后重试"
//   - "音色注册到生成服务失败，请重试"（重试=幂等重建）
func (uc *GenerationUseCase) ensureViduVoiceID(ctx context.Context, tenantID, voiceID string) (string, error) {
	if voiceID == "" {
		return "", nil
	}
	// 特性开关（31号 §六 #6）：关闭时回退旧行为（直传 ID）；
	// 音色仓储未注入（部分装配/测试场景）同样直传——无库可查即无从保障
	if uc.voiceRepo == nil || !uc.generationBoolDefault(ctx, entity.SettingKeyGenVoiceMaterializer, true) {
		return voiceID, nil
	}
	v, err := uc.voiceRepo.FindByVoiceID(ctx, voiceID)
	if err != nil {
		// 不在我们库（Vidu 上游预置 / MiMo 预置）——归属厂商原生认识，直传
		return voiceID, nil
	}
	// 租户归属：clone 行仅归属租户可用（越权返回"不存在"，不泄露存在性）
	if v.Scope == "clone" && v.TenantID != tenantID {
		return "", fmt.Errorf("音色不存在")
	}
	if v.Status != "" && v.Status != "active" {
		return "", fmt.Errorf("音色已停用")
	}
	// scope=vidu（上游参考源）：Vidu 原生认识，直传
	if v.Scope != "clone" && v.Scope != "platform" {
		return voiceID, nil
	}
	// 窗口命中：注册缓存有效，零额外调用
	if window := uc.viduRegWindow(ctx); v.ViduRegisteredAt != nil && time.Since(*v.ViduRegisteredAt) < window {
		return voiceID, nil
	}
	// 注册前样本必须永久化（data:URI/空样本不可注册）
	if !strings.HasPrefix(v.SampleURL, "http://") && !strings.HasPrefix(v.SampleURL, "https://") {
		return "", errors.New("音色样本转存未完成，请稍后重试")
	}
	if _, err, _ := viduRegGroup.Do("vidu-reg:"+voiceID, func() (any, error) {
		return nil, uc.registerVoiceOnVidu(ctx, v)
	}); err != nil {
		return "", err
	}
	return voiceID, nil
}

// registerVoiceOnVidu 同 ID 复注册（幂等 + 续期 7 天）。直连 Vidu provider——
// 不经能力路由（voice-clone 能力可能路由到 MiMo，而 MiMo 无注册制）。
// 产物 demo_audio 丢弃（注册关系已建立即达成目的；不落库不转存）。
func (uc *GenerationUseCase) registerVoiceOnVidu(ctx context.Context, v entity.GenerationVoice) error {
	provider, ok := uc.providers["vidu"]
	if !ok || provider == nil {
		return errors.New("音色注册到生成服务失败：Vidu 服务未配置，请重试")
	}
	adapter, err := uc.registry.Get(ctx, "voice_clone")
	if err != nil {
		return fmt.Errorf("音色注册到生成服务失败：%w", err)
	}
	model := uc.getDefaultModel(ctx, "voice_clone")
	cap, err := uc.registry.Capability(ctx, "voice_clone", model)
	if err != nil {
		return fmt.Errorf("音色注册到生成服务失败：%w", err)
	}
	audition := uc.settingString(ctx, entity.SettingKeyGenVoiceAuditionText, defaultVoiceAuditionText)
	params := entity.GenerationParams{
		"audio_url": v.SampleURL,
		"voice_id":  v.VoiceID,
		"text":      audition,
	}
	if err := adapter.Validate(ctx, cap, params); err != nil {
		return fmt.Errorf("音色注册参数无效：%w", err)
	}
	body, err := adapter.BuildRequest(ctx, model, params, "")
	if err != nil {
		return fmt.Errorf("音色注册到生成服务失败：%w", err)
	}
	uc.inlineLocalMedia(ctx, body) // 本站私网样本 → base64 内联（Vidu 拉不到 localhost）

	regCtx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()
	res, err := provider.Submit(regCtx, adapter.Endpoint(), body)
	if err != nil {
		log.Printf("[voiceMaterializer] 注册失败 voice=%s err=%v", v.VoiceID, err)
		return errors.New("音色注册到生成服务失败，请重试")
	}
	if res.State == entity.TaskStateFailed {
		log.Printf("[voiceMaterializer] 注册被上游拒绝 voice=%s taskID=%s", v.VoiceID, res.TaskID)
		return errors.New("音色注册到生成服务失败，请重试")
	}
	now := time.Now()
	if uErr := uc.voiceRepo.UpdateViduRegisteredAt(ctx, v.VoiceID, &now); uErr != nil {
		// 注册已成功、时间戳写失败：下次会冗余注册一次（幂等无害），不作为失败
		log.Printf("[voiceMaterializer] 注册成功但时间戳写入失败 voice=%s err=%v", v.VoiceID, uErr)
	}
	log.Printf("[voiceMaterializer] 注册/续期成功 voice=%s provider_state=%s", v.VoiceID, res.State)
	return nil
}

// invalidateViduRegistration 缓存失效（31号 L4-④ 故障自愈的前半段）：
// 厂商侧报"音色不存在"→ 置空时间戳 → 下次提交触发重建。
func (uc *GenerationUseCase) invalidateViduRegistration(ctx context.Context, voiceID string) {
	if voiceID == "" || uc.voiceRepo == nil {
		return
	}
	if err := uc.voiceRepo.UpdateViduRegisteredAt(ctx, voiceID, nil); err != nil {
		log.Printf("[voiceMaterializer] 缓存失效写入失败 voice=%s err=%v", voiceID, err)
	} else {
		log.Printf("[voiceMaterializer] 注册缓存已失效（下次提交重建）voice=%s", voiceID)
	}
}

// warmUpViduRegistration 31号 L4-② 创建后异步预热：非 Vidu 创建的音色在落库后
// 后台补注册（正常情况几秒内就绪，首次口播命中窗口零开销）。
// 异步且失败静默——按需保障（ensureViduVoiceID）兜底。
func (uc *GenerationUseCase) warmUpViduRegistration(tenantID, voiceID string) {
	if voiceID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), registerTimeout+10*time.Second)
		defer cancel()
		if _, err := uc.ensureViduVoiceID(ctx, tenantID, voiceID); err != nil {
			log.Printf("[voiceMaterializer] 异步预热失败（按需保障兜底）voice=%s err=%v", voiceID, err)
		}
	}()
}

// WarmUpVoiceRegistration 平台音色创建后预热（31号 L4-② 的 admin 链路接线——
// 上传/URL/from-vidu 三条创建路径共用；平台音色 tenantID 为空串）。
func (uc *GenerationUseCase) WarmUpVoiceRegistration(voiceID string) {
	uc.warmUpViduRegistration("", voiceID)
}

// viduRegWindow 注册缓存窗口（可配；默认 144h）。
func (uc *GenerationUseCase) viduRegWindow(ctx context.Context) time.Duration {
	raw := uc.settingString(ctx, entity.SettingKeyGenViduVoiceWindow, strconv.Itoa(defaultViduVoiceWindowHours))
	if h, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && h > 0 {
		return time.Duration(h) * time.Hour
	}
	return defaultViduVoiceWindowHours * time.Hour
}

// maybeInvalidateVoiceOnVendorMiss L4-④ 故障自愈：lip_sync 失败且错误含音色缺失特征时
// 失效注册缓存，下次提交自动重建（重试即自愈）。
// 匹配保守（子串 voice/not found）——真实上游错误码校准后收紧（31号 §六 #12）。
func (uc *GenerationUseCase) maybeInvalidateVoiceOnVendorMiss(ctx context.Context, task entity.GenerationTask) {
	if task.SubType != "lip_sync" || task.State != entity.TaskStateFailed || uc.voiceRepo == nil {
		return
	}
	voiceID, _ := taskParamsVoiceID(task)
	if voiceID == "" {
		return
	}
	msg := strings.ToLower(task.ErrMsg)
	if strings.Contains(msg, "voice") && (strings.Contains(msg, "not found") ||
		strings.Contains(msg, "不存在") || strings.Contains(msg, "未找到")) {
		uc.invalidateViduRegistration(ctx, voiceID)
	}
}

// taskParamsVoiceID 从任务参数提取 voice_id（lip_sync 文本驱动）。
func taskParamsVoiceID(task entity.GenerationTask) (string, bool) {
	var p map[string]any
	if err := json.Unmarshal([]byte(task.ParamsJSON), &p); err != nil {
		return "", false
	}
	vid, _ := p["voice_id"].(string)
	return vid, vid != ""
}

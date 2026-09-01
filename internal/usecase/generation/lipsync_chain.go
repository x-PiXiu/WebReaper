package generation

// lipsync_chain.go —— 31号 §4.2 口播路径（对齐 23 号产品设计）+ §L4-⑤ 备胎道。
//
// 统一模型（与 01/23 号业务文档一致，2026-09-01 用户定案：只保留一条路）：
//   画面 = 分身资产，阶段0 一次性预生成（10s 不说话形象视频，用户预览/删了重建）
//   口播 = 对口型复用画面（lip_sync），每次口播不重新生成任何画面
//     A 文本+音色 → tts 样本合成 → lip_sync(audio_url)   [两段，备胎道同构]
//     B 文本直生   → lip_sync(text, voice_id)             [单段，音色过 L2 保障]
//     C 上传音频   → lip_sync(audio_url)                  [单段，免音色]
//
// gen_lipsync_auto_chain 开关（默认关）：开启后 subjects+（文案|音频）提交 →
// 解析分身形象视频复用 → 直接提交 lip_sync（不产生 reference2video 任务）。
// 形象视频缺失 = 显式报错引导用户回分身管理等待/重建——不回退现场生成。
//
// 备胎道（gen_lipsync_two_step，默认关）：lip_sync 注册类失败自动降级 =
// A 路径（tts 样本合成 → 音频驱动 lip_sync），零厂商 ID 依赖。

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
)

// lipsyncChain 口播复用路径装配（endpoint_selector subjects 分支产出 __chain_* 参数）。
type lipsyncChain struct {
	Text     string // 口播文案（B 路径）
	AudioURL string // 上传音频（C 路径，优先于文本）
	VoiceID  string // 显式选定音色（B 路径；空则回落分身绑定音色——23号 B 路径语义）
}

// extractLipsyncChain 从 selector 产物参数提取口播复用配置；非该模式返回 nil。
func extractLipsyncChain(params entity.GenerationParams) *lipsyncChain {
	if flag, _ := params["__chain"].(bool); !flag {
		return nil
	}
	c := &lipsyncChain{}
	c.Text, _ = params["__chain_text"].(string)
	c.AudioURL, _ = params["__chain_audio_url"].(string)
	c.VoiceID, _ = params["__chain_voice_id"].(string)
	if c.Text == "" && c.AudioURL == "" {
		return nil
	}
	return c
}

// submitReuseLipSync 画面复用口播（31号 §4.2，对齐 23 号目标态）：
// 分身形象视频直接作为 lip_sync 输入——单段提交（B/C 路径），不生成画面任务。
// 形象视频缺失 → 显式报错（回分身管理等待/重建），绝不回退现场生成。
func (uc *GenerationUseCase) submitReuseLipSync(ctx context.Context, in UnifiedSubmitInput, subjects any, c lipsyncChain) (entity.GenerationTask, error) {
	if uc.subjectAssetRepo == nil {
		return entity.GenerationTask{}, fmt.Errorf("分身资产服务未配置，请联系管理员")
	}
	serverID := firstSubjectServerID(subjects)
	if serverID == "" {
		return entity.GenerationTask{}, fmt.Errorf("提交参数缺少分身标识（subjects[].server_id）")
	}
	asset, err := uc.subjectAssetRepo.FindByServerID(ctx, serverID)
	if err != nil {
		return entity.GenerationTask{}, fmt.Errorf("分身不存在或已删除，请重新选择出镜分身")
	}
	if asset.Status != "" && asset.Status != "active" {
		return entity.GenerationTask{}, fmt.Errorf("分身「%s」已下架，请重新选择出镜分身", asset.Name)
	}
	if !strings.HasPrefix(asset.AvatarVideoURL, "http://") && !strings.HasPrefix(asset.AvatarVideoURL, "https://") {
		return entity.GenerationTask{}, fmt.Errorf("分身「%s」形象视频未就绪——请在分身管理中等待生成完成（或删除后重建）再创作", asset.Name)
	}

	params := entity.GenerationParams{"video_url": asset.AvatarVideoURL}
	if c.AudioURL != "" {
		params["audio_url"] = c.AudioURL // C 路径：上传音频直驱
	} else {
		params["text"] = c.Text // B 路径：文本驱动（voice_id 经 Submit 内 L2 保障）
		voice := c.VoiceID
		if voice == "" {
			voice = asset.VoiceID // 23号 B 路径默认：分身绑定音色
		}
		if voice != "" {
			params["voice_id"] = voice
		}
	}
	return uc.Submit(ctx, SubmitInput{
		TenantID: in.TenantID, BrandID: in.BrandID,
		SubType: "lip_sync", Params: params,
		Watermark: in.Watermark, OffPeak: in.OffPeak,
	})
}

// firstSubjectServerID 解析 subjects[0].server_id（主体一致性路径的主主体）。
// 容忍 []any(map)/[]map[string]any 两种形态；解析失败返回空由调用方报错。
func firstSubjectServerID(subjects any) string {
	list, ok := subjects.([]any)
	if !ok || len(list) == 0 {
		if typed, ok2 := subjects.([]map[string]any); ok2 && len(typed) > 0 {
			vid, _ := typed[0]["server_id"].(string)
			return vid
		}
		return ""
	}
	m, ok := list[0].(map[string]any)
	if !ok {
		return ""
	}
	vid, _ := m["server_id"].(string)
	return vid
}

// submitLipSyncViaTTS 备胎道（31号 L4-⑤）＝23号 A 路径同构：lip_sync 注册类失败
// 自动降级——tts 样本合成（厂商无关）→ 音频驱动 lip_sync，零厂商 ID 依赖。
// 返回链头 tts 任务（客户端跟踪链；最终 lip_sync 任务以 source_task_id 关联）。
// ⚠️ 已知限制：备胎道路径暂不携带单阶段 B-Roll（视频链尾才可合成，tts 链头不匹配）。
func (uc *GenerationUseCase) submitLipSyncViaTTS(ctx context.Context, in SubmitInput, voiceID string) (entity.GenerationTask, error) {
	text, _ := in.Params["text"].(string)
	videoURL, _ := in.Params["video_url"].(string)
	if text == "" || videoURL == "" {
		return entity.GenerationTask{}, fmt.Errorf("备胎道缺少参数（text/video_url）")
	}
	ttsTask, err := uc.Submit(ctx, SubmitInput{
		TenantID: in.TenantID, BrandID: in.BrandID, SubType: "tts",
		Params: entity.GenerationParams{
			"text":                   text,
			"voice_setting_voice_id": voiceID, // 样本合成改写在 Submit 内自动处理（厂商无关）
		},
	})
	if err != nil {
		return entity.GenerationTask{}, fmt.Errorf("备胎道 TTS 提交失败: %w", err)
	}
	log.Printf("[lipsync-chain] 备胎道启动（注册失败自动降级）: voice=%s tts=%s", voiceID, ttsTask.ID)

	go func() {
		cctx, cancel := context.WithTimeout(context.Background(), 16*time.Minute)
		defer cancel()
		uc.chainAudioLipSyncAfterTTS(cctx, in.TenantID, in.BrandID, ttsTask.ID, videoURL)
	}()
	return ttsTask, nil
}

// chainAudioLipSyncAfterTTS 等待 tts 终态 → 取音频产物 → 音频驱动 lip_sync（A 路径第②段）。
func (uc *GenerationUseCase) chainAudioLipSyncAfterTTS(ctx context.Context, tenantID, brandID, ttsTaskID, videoURL string) {
	deadline := time.Now().Add(15 * time.Minute)
	var tts entity.GenerationTask
	for time.Now().Before(deadline) {
		task, err := uc.repo.FindByID(ctx, tenantID, ttsTaskID)
		if err != nil {
			log.Printf("[lipsync-chain] 备胎道查询 TTS 失败: %v", err)
			return
		}
		if entity.IsTerminal(task.State) {
			if task.State != entity.TaskStateSuccess {
				log.Printf("[lipsync-chain] 备胎道 TTS 未成功（%s），链终止", task.State)
				return
			}
			tts = task
			break
		}
		time.Sleep(5 * time.Second)
	}
	if tts.ID == "" {
		log.Printf("[lipsync-chain] 备胎道等待 TTS 超时: %s", ttsTaskID)
		return
	}
	audioURL := firstTaskCreationURL(tts)
	if audioURL == "" {
		log.Printf("[lipsync-chain] 备胎道 TTS 无产物 URL，链终止: %s", ttsTaskID)
		return
	}
	step2, err := uc.Submit(ctx, SubmitInput{
		TenantID: tenantID, BrandID: brandID, SubType: "lip_sync",
		Params: entity.GenerationParams{
			"video_url":      videoURL,
			"audio_url":      audioURL,
			"source_task_id": ttsTaskID,
		},
	})
	if err != nil {
		log.Printf("[lipsync-chain] 备胎道 lip_sync 提交失败: %v", err)
		return
	}
	log.Printf("[lipsync-chain] 备胎道 lip_sync 已提交: tts=%s lipsync=%s", ttsTaskID, step2.ID)
}

// firstTaskCreationURL 任务首个产物的可用 URL（stored_url 优先，24h 转存前用原 url）。
func firstTaskCreationURL(task entity.GenerationTask) string {
	var creations []map[string]any
	if err := json.Unmarshal([]byte(task.CreationsJSON), &creations); err != nil || len(creations) == 0 {
		return ""
	}
	for _, key := range []string{"stored_url", "url"} {
		if v, _ := creations[0][key].(string); strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			return v
		}
	}
	return ""
}

// isRegistrationFailure 注册类失败判定（备胎道触发条件——仅基础设施失败，
// 用户错误（越权/停用/样本未就绪）不降级，直接报错）。
func isRegistrationFailure(err error) bool {
	return err != nil && strings.Contains(err.Error(), "音色注册到生成服务失败")
}

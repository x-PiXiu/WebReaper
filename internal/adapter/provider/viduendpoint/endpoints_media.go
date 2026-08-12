package viduendpoint

import (
	"context"
	"fmt"
	"strings"

	"webreaper/internal/domain/entity"
)

// ---- 图片生成 /ent/v2/reference2image ----
// 参数：model/prompt(≤2000 必填)/images（viduq2: 0-7 张，0=纯文生图；viduq1: 1-7 张）/callback_url

type text2imageAdapter struct{}

func (text2imageAdapter) Type() string     { return "text2image" }
func (text2imageAdapter) Category() string { return entity.GenerationTypeImage }
func (text2imageAdapter) Endpoint() string { return "/ent/v2/reference2image" }

func (text2imageAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if len(getString(p, "prompt")) == 0 {
		return errPromptRequired
	}
	if len([]rune(getString(p, "prompt"))) > 2000 {
		return errPromptTooLong
	}
	imgs := getStrings(p, "images")
	if len(imgs) > 7 {
		return fmt.Errorf("图片 images 最多 7 张，收到 %d 张", len(imgs))
	}
	// viduq1 必须至少 1 张参考图；viduq2 允许 0 张（纯文生图）
	if cap.Model == "viduq1" && len(imgs) == 0 {
		return fmt.Errorf("viduq1 参考生图需 1-7 张图片")
	}
	return nil
}

func (text2imageAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"model":  model,
		"prompt": getString(p, "prompt"),
	}
	if imgs := getStrings(p, "images"); len(imgs) > 0 {
		body["images"] = imgs
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 文生音频 /ent/v2/text2audio ----
// 参数：model(audio1.0)/prompt(≤1500 必填)/duration(2-10 默认 10)/seed/callback_url

type text2audioAdapter struct{}

func (text2audioAdapter) Type() string     { return "text2audio" }
func (text2audioAdapter) Category() string { return entity.GenerationTypeAudio }
func (text2audioAdapter) Endpoint() string { return "/ent/v2/text2audio" }

func (text2audioAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if len(getString(p, "prompt")) == 0 {
		return errPromptRequired
	}
	if len([]rune(getString(p, "prompt"))) > 1500 {
		return fmt.Errorf("提示词超过 1500 字符上限")
	}
	return validateDuration(getInt(p, "duration"), cap, "时长 duration")
}

func (text2audioAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"model":  model,
		"prompt": getString(p, "prompt"),
	}
	if v := getInt(p, "duration"); v > 0 {
		body["duration"] = v
	}
	if v := getInt(p, "seed"); v > 0 {
		body["seed"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 可控文生音效 /ent/v2/timing2audio ----
// 参数：model(audio1.0)/timing_prompts[{from,to,prompt} 事件数组 必填]/duration(2-10)/seed

type soundEffectAdapter struct{}

func (soundEffectAdapter) Type() string     { return "sound_effect" }
func (soundEffectAdapter) Category() string { return entity.GenerationTypeAudio }
func (soundEffectAdapter) Endpoint() string { return "/ent/v2/timing2audio" }

func (soundEffectAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	events := getTimingPrompts(p)
	if len(events) == 0 {
		return fmt.Errorf("音效事件 timing_prompts 必填（至少 1 个）")
	}
	duration := getInt(p, "duration")
	if duration <= 0 {
		duration = 10 // 默认
	}
	for i, e := range events {
		prompt := getString(e, "prompt")
		if len([]rune(prompt)) > 1500 {
			return fmt.Errorf("音效事件 %d 提示词超过 1500 字符上限", i+1)
		}
		from, to := getFloat(e, "from"), getFloat(e, "to")
		if from < 0 || to > float64(duration) || from > to {
			return fmt.Errorf("音效事件 %d 时间区间需在 [0,%d] 内且 from≤to", i+1, duration)
		}
	}
	return validateDuration(duration, cap, "时长 duration")
}

func (soundEffectAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"model":          model,
		"timing_prompts": getTimingPrompts(p),
	}
	if v := getInt(p, "duration"); v > 0 {
		body["duration"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 语音合成 /ent/v2/audio-tts ----
// 参数：text(≤10000 必填，支持 <#x#> 停顿标记)/voice_setting_voice_id(必填)/voice_setting_speed(0.5-2)/
//       voice_setting_volume(0-10)/voice_setting_pitch(-12~12)/voice_setting_emotion(7 种)

type ttsAdapter struct{}

func (ttsAdapter) Type() string     { return "tts" }
func (ttsAdapter) Category() string { return entity.GenerationTypeAudio }
func (ttsAdapter) Endpoint() string { return "/ent/v2/audio-tts" }

var ttsEmotions = []string{"happy", "sad", "angry", "fearful", "disgusted", "surprised", "calm"}

func (ttsAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	text := getString(p, "text")
	if text == "" {
		return fmt.Errorf("合成文本 text 必填")
	}
	if len([]rune(text)) > 10000 {
		return fmt.Errorf("合成文本超过 10000 字符上限")
	}
	if getString(p, "voice_setting_voice_id") == "" {
		return fmt.Errorf("音色 voice_setting_voice_id 必填")
	}
	if v := getFloat(p, "voice_setting_speed"); v != 0 && (v < 0.5 || v > 2) {
		return fmt.Errorf("语速 voice_setting_speed 需在 0.5-2 之间")
	}
	if v := getInt(p, "voice_setting_volume"); v != 0 && (v < 0 || v > 10) {
		return fmt.Errorf("音量 voice_setting_volume 需在 0-10 之间")
	}
	if v := getInt(p, "voice_setting_pitch"); v < -12 || v > 12 {
		return fmt.Errorf("语调 voice_setting_pitch 需在 -12~12 之间")
	}
	if e := getString(p, "voice_setting_emotion"); e != "" && !containsStr(ttsEmotions, e) {
		return fmt.Errorf("情绪 voice_setting_emotion 可选 %v", ttsEmotions)
	}
	return nil
}

func (ttsAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"text":                  getString(p, "text"),
		"voice_setting_voice_id": getString(p, "voice_setting_voice_id"),
	}
	if v := getFloat(p, "voice_setting_speed"); v != 0 {
		body["voice_setting_speed"] = v
	}
	if v := getInt(p, "voice_setting_volume"); v != 0 {
		body["voice_setting_volume"] = v
	}
	if v := getInt(p, "voice_setting_pitch"); v != 0 {
		body["voice_setting_pitch"] = v
	}
	if v := getString(p, "voice_setting_emotion"); v != "" {
		body["voice_setting_emotion"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 声音复刻 /ent/v2/audio-clone ----
// 参数：audio_url(必填 mp3/m4a/wav 10s-5min ≤20MB)/voice_id(必填 8-256 规则)/
//       prompt_audio_url/prompt_text/text(≤1000 必填 试听)

type voiceCloneAdapter struct{}

func (voiceCloneAdapter) Type() string     { return "voice_clone" }
func (voiceCloneAdapter) Category() string { return entity.GenerationTypeAudio }
func (voiceCloneAdapter) Endpoint() string { return "/ent/v2/audio-clone" }

func (voiceCloneAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if getString(p, "audio_url") == "" {
		return fmt.Errorf("原音频 audio_url 必填（mp3/m4a/wav，10 秒-5 分钟）")
	}
	vid := getString(p, "voice_id")
	if vid == "" {
		return fmt.Errorf("声音 ID voice_id 必填（8-256 位，字母/数字/横线/下划线）")
	}
	if n := len(vid); n < 8 || n > 256 {
		return fmt.Errorf("voice_id 长度需 8-256 位")
	}
	if c := vid[0]; (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
		return fmt.Errorf("voice_id 首字符必须为英文字母")
	}
	for _, c := range vid {
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '-' && c != '_' {
			return fmt.Errorf("voice_id 含非法字符 %q", string(c))
		}
	}
	if text := getString(p, "text"); text == "" || len([]rune(text)) > 1000 {
		return fmt.Errorf("试听文本 text 必填且不超过 1000 字符")
	}
	return nil
}

func (voiceCloneAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"audio_url": getString(p, "audio_url"),
		"voice_id":  getString(p, "voice_id"),
		"text":      getString(p, "text"),
	}
	if v := getString(p, "prompt_audio_url"); v != "" {
		body["prompt_audio_url"] = v
	}
	if v := getString(p, "prompt_text"); v != "" {
		body["prompt_text"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 辅助 ----

// getTimingPrompts 提取 timing_prompts 事件数组。
func getTimingPrompts(p entity.GenerationParams) []map[string]any {
	if v, ok := p["timing_prompts"].([]map[string]any); ok {
		return v
	}
	if v, ok := p["timing_prompts"].([]any); ok {
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func getFloat(p map[string]any, key string) float64 {
	switch v := p[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%g", &f); err == nil {
			return f
		}
	}
	return 0
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// ---- 媒体端点能力向量 ----

var text2imageCaps = []entity.ModelCapability{
	{Model: "viduq2", Family: "q2", ImageSlots: -1, MaxPromptLen: 2000}, // 0-7 张（0=纯文生图）
	{Model: "viduq1", Family: "q1", ImageSlots: -1, MaxPromptLen: 2000}, // 1-7 张
}

var text2audioCaps = []entity.ModelCapability{
	{Model: "audio1.0", Family: "audio1.0", Durations: [2]int{2, 10}, MaxPromptLen: 1500},
}

var soundEffectCaps = []entity.ModelCapability{
	{Model: "audio1.0", Family: "audio1.0", Durations: [2]int{2, 10}, MaxPromptLen: 1500},
}

var ttsCaps = []entity.ModelCapability{
	{Model: "default", Family: "tts", MaxPromptLen: 10000}, // 无 model 参数——统一默认
}

var voiceCloneCaps = []entity.ModelCapability{
	{Model: "default", Family: "voice", MaxPromptLen: 1000}, // 无 model 参数——统一默认
}

package viduendpoint

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
)

// ---- 文生视频 /ent/v2/text2video ----
// 参数：model/prompt/duration/resolution/seed/audio/audio_type/bgm/style/aspect_ratio/movement_amplitude/watermark/off_peak/payload

type text2videoAdapter struct{}

func (text2videoAdapter) Type() string     { return "text2video" }
func (text2videoAdapter) Category() string { return entity.GenerationTypeVideo }
func (text2videoAdapter) Endpoint() string { return "/ent/v2/text2video" }
func (text2videoAdapter) SupportsCallback() bool { return true } // 文档声明 callback_url

func (text2videoAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if len(getString(p, "prompt")) == 0 {
		return errPromptRequired
	}
	return baseValidate(ctx, cap, p)
}

func (text2videoAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"model":  model,
		"prompt": getString(p, "prompt"),
	}
	ensureStringParam(body, "resolution", p, "720p")
	ensureStringParam(body, "aspect_ratio", p, "16:9")
	if v := getInt(p, "duration"); v > 0 {
		body["duration"] = v
	}
	if v := getInt(p, "seed"); v > 0 {
		body["seed"] = v
	}
	if v := getString(p, "style"); v != "" {
		body["style"] = v
	}
	if v := getString(p, "audio_type"); v != "" {
		body["audio_type"] = v
	}
	if getBool(p, "audio") {
		body["audio"] = true
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 图生视频 /ent/v2/img2video ----
// 参数：model/images[1]/prompt/duration/resolution/seed/audio/audio_type/movement_amplitude(仅q1/2.0)

type img2videoAdapter struct{}

func (img2videoAdapter) Type() string     { return "img2video" }
func (img2videoAdapter) Category() string { return entity.GenerationTypeVideo }
func (img2videoAdapter) Endpoint() string { return "/ent/v2/img2video" }
func (img2videoAdapter) SupportsCallback() bool { return true } // 文档声明 callback_url

func (img2videoAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if err := validateImageSlots(getStrings(p, "images"), cap, "图片 images"); err != nil {
		return err
	}
	return baseValidate(ctx, cap, p)
}

func (img2videoAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"model":  model,
		"images": getStrings(p, "images"),
	}
	if v := getString(p, "prompt"); v != "" {
		body["prompt"] = v
	}
	ensureStringParam(body, "resolution", p, "720p")
	if v := getInt(p, "duration"); v > 0 {
		body["duration"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 首尾帧 /ent/v2/start-end2video ----
// 参数：model/images[2 首+尾]/prompt/duration/resolution/movement_amplitude(仅q1/2.0)

type startEnd2videoAdapter struct{}

func (startEnd2videoAdapter) Type() string     { return "start_end2video" }
func (startEnd2videoAdapter) Category() string { return entity.GenerationTypeVideo }
func (startEnd2videoAdapter) Endpoint() string { return "/ent/v2/start-end2video" }
func (startEnd2videoAdapter) SupportsCallback() bool { return true } // 文档声明 callback_url

func (startEnd2videoAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if err := validateImageSlots(getStrings(p, "images"), cap, "图片 images"); err != nil {
		return err
	}
	return baseValidate(ctx, cap, p)
}

func (startEnd2videoAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"model":  model,
		"images": getStrings(p, "images"),
	}
	if v := getString(p, "prompt"); v != "" {
		body["prompt"] = v
	}
	ensureStringParam(body, "resolution", p, "720p")
	if v := getInt(p, "duration"); v > 0 {
		body["duration"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 参考生视频 /ent/v2/reference2video ----
// 两种模式：subjects 主体模式（prompt 内 @name 引用）与 images 非主体模式（1-7 图）。
// 仅 viduq2-pro 支持 videos 视频参考（1 个 8 秒 或 2 个 5 秒）。

type reference2videoAdapter struct{}

func (reference2videoAdapter) Type() string     { return "reference2video" }
func (reference2videoAdapter) Category() string { return entity.GenerationTypeVideo }
func (reference2videoAdapter) Endpoint() string { return "/ent/v2/reference2video" }
func (reference2videoAdapter) SupportsCallback() bool { return true } // 文档声明 callback_url

// PickModel 模型自动切换（08 计划 D3）：图片主体→q3 系（效果最好）；
// 视频主体→唯一支持的 q2-pro（VideoSlots>0）。用户全程不感知模型存在。
func (reference2videoAdapter) PickModel(models []entity.ModelCapability, p entity.GenerationParams) string {
	hasVideo := subjectHasVideo(p)
	best, bestRank := "", -1
	for _, m := range models {
		if hasVideo && m.VideoSlots <= 0 {
			continue // 视频主体不被该模型支持
		}
		if r := familyRank(m.Family); r > bestRank {
			bestRank, best = r, m.Model
		}
	}
	if best == "" {
		// 无任何模型支持视频主体——退回最高家族，由后续校验/上游报可读错误
		for _, m := range models {
			if r := familyRank(m.Family); r > bestRank {
				bestRank, best = r, m.Model
			}
		}
	}
	return best
}

// subjectHasVideo 主体参数是否携带视频（内联 videos 数组）。
// server_id 引用的视频主体无法从参数判断类型——由上游按主体实际内容处理。
func subjectHasVideo(p entity.GenerationParams) bool {
	for _, s := range getSubjects(p) {
		if v, ok := s["videos"].([]any); ok && len(v) > 0 {
			return true
		}
		if v, ok := s["videos"].([]string); ok && len(v) > 0 {
			return true
		}
	}
	return false
}

// familyRank 模型家族优先级（越大越好）：q3 > q2 > q1 > vidu2.0。
func familyRank(family string) int {
	switch family {
	case "q3":
		return 3
	case "q2":
		return 2
	case "q1":
		return 1
	default:
		return 0
	}
}

func (reference2videoAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if subs := getSubjects(p); len(subs) > 0 {
		// 主体模式
		if !cap.SupportsSubjects {
			return errSubjectsUnsupported
		}
		if getString(p, "prompt") == "" {
			return errPromptRequired
		}
	} else {
		// 非主体模式：images 1-7
		imgs := getStrings(p, "images")
		if len(imgs) < 1 || len(imgs) > 7 {
			return errImagesRange
		}
	}
	return baseValidate(ctx, cap, p)
}

func (reference2videoAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{"model": model}
	if subs := getSubjects(p); len(subs) > 0 {
		body["subjects"] = subs
	} else {
		body["images"] = getStrings(p, "images")
	}
	if v := getString(p, "prompt"); v != "" {
		body["prompt"] = v
	}
	ensureStringParam(body, "aspect_ratio", p, "16:9")
	if v := getInt(p, "duration"); v > 0 {
		body["duration"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 智能多帧 /ent/v2/multiframe ----
// 参数：model/start_image[1]/image_settings[2-9 各含 key_image/prompt/duration 2-7]/resolution/aspect_ratio

type multiframeAdapter struct{}

func (multiframeAdapter) Type() string     { return "multiframe" }
func (multiframeAdapter) Category() string { return entity.GenerationTypeVideo }
func (multiframeAdapter) Endpoint() string { return "/ent/v2/multiframe" }
func (multiframeAdapter) SupportsCallback() bool { return true } // 文档声明 callback_url

func (multiframeAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if getString(p, "start_image") == "" {
		return errStartImageRequired
	}
	if n := len(getKeyFrames(p)); n < 2 || n > 9 {
		return errKeyFramesRange
	}
	return baseValidate(ctx, cap, p)
}

func (multiframeAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"model":          model,
		"start_image":    getString(p, "start_image"),
		"image_settings": getKeyFrames(p),
	}
	ensureStringParam(body, "resolution", p, "720p")
	if v := getString(p, "aspect_ratio"); v != "" {
		body["aspect_ratio"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 数字人 /ent/v2/digital-human ----
// 参数：model/image[1]/prompt(≤2000)/audio_url 或 text+voice_id（audio_url 优先）/resolution

type digitalHumanAdapter struct{}

func (digitalHumanAdapter) Type() string     { return "digital_human" }
func (digitalHumanAdapter) Category() string { return entity.GenerationTypeDigitalHuman }
func (digitalHumanAdapter) Endpoint() string { return "/ent/v2/digital-human" }
func (digitalHumanAdapter) SupportsCallback() bool { return true } // 文档声明 callback_url

func (digitalHumanAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if len(getStrings(p, "images")) != 1 && getString(p, "image") == "" {
		return errDigitalHumanImageRequired
	}
	if v := getString(p, "prompt"); len([]rune(v)) > 2000 {
		return errPromptTooLong
	}
	return validateEnum(getString(p, "resolution"), cap.Resolutions, "分辨率 resolution")
}

func (digitalHumanAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	img := getString(p, "image")
	if img == "" {
		if imgs := getStrings(p, "images"); len(imgs) > 0 {
			img = imgs[0]
		}
	}
	body := map[string]any{"model": model, "image": img}
	if v := getString(p, "prompt"); v != "" {
		body["prompt"] = v
	}
	if v := getString(p, "audio_url"); v != "" {
		body["audio_url"] = v
	} else if v := getString(p, "text"); v != "" {
		body["text"] = v
		if voice := getString(p, "voice_id"); voice != "" {
			body["voice_id"] = voice
		}
	}
	ensureStringParam(body, "resolution", p, "720p")
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

// ---- 对口型 /ent/v2/lip-sync ----
// 参数：video_url(必填)/audio_url 或 text(二选一，audio 优先)/speed(0.5-2 仅文本)/
// voice_id(仅文本)/volume(0-10 仅文本)/ref_photo_url(多脸时指定目标人物)。
// 无 model/payload 参数（文档未声明——注入会被严格校验拒 400）；异步任务端点。
// 真人出镜主链路：用户拍的不说话视频 + TTS 音频 → 口型匹配成片。

type lipSyncAdapter struct{}

func (lipSyncAdapter) Type() string     { return "lip_sync" }
func (lipSyncAdapter) Category() string { return entity.GenerationTypeVideo }
func (lipSyncAdapter) Endpoint() string { return "/ent/v2/lip-sync" }
func (lipSyncAdapter) SupportsCallback() bool { return true } // 文档声明 callback_url

func (lipSyncAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if getString(p, "video_url") == "" {
		return errLipSyncVideoRequired
	}
	if getString(p, "audio_url") == "" && getString(p, "text") == "" {
		return errLipSyncAudioOrTextRequired
	}
	if v := getString(p, "text"); v != "" && (len([]rune(v)) < 4 || len([]rune(v)) > 2000) {
		return errLipSyncTextRange
	}
	if v := getFloat(p, "speed"); v != 0 && (v < 0.5 || v > 2) {
		return fmt.Errorf("语速 speed 需在 0.5-2 之间")
	}
	if v := getInt(p, "volume"); v != 0 && (v < 0 || v > 10) {
		return fmt.Errorf("音量 volume 需在 0-10 之间")
	}
	return nil
}

func (lipSyncAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	// 注意：不注入 payload——对口型参数表未声明该字段（严格校验会 400）
	body := map[string]any{"video_url": getString(p, "video_url")}
	if v := getString(p, "audio_url"); v != "" {
		// 音频驱动：audio_url 优先，同传时上游以音频为准（speed/voice_id/volume 失效）
		body["audio_url"] = v
	} else {
		// 文本驱动：text + 可选 voice_id/speed/volume
		body["text"] = getString(p, "text")
		if v := getString(p, "voice_id"); v != "" {
			body["voice_id"] = v
		}
		if v := getFloat(p, "speed"); v != 0 {
			body["speed"] = v
		}
		if v := getInt(p, "volume"); v != 0 {
			body["volume"] = v
		}
	}
	if v := getString(p, "ref_photo_url"); v != "" {
		body["ref_photo_url"] = v
	}
	return body, nil
}

// ---- 主体 API /ent/v2/subjects ----
// 参数：name/images(≤3)/videos(≤1 个 5 秒，仅 q2-pro 参考生支持)/voice_id。
// 同步端点：响应直接返回主体对象（id=server_id），无 task_id 轮询语义——
// usecase 提交成功即终态；主体供 reference2video 的 subjects[].server_id 复用。

type subjectAdapter struct{}

func (subjectAdapter) Type() string     { return "subject" }
func (subjectAdapter) Category() string { return entity.GenerationTypeOther }
func (subjectAdapter) Endpoint() string { return "/ent/v2/subjects" }
func (subjectAdapter) IsSync() bool     { return true }

func (subjectAdapter) Validate(ctx context.Context, cap entity.ModelCapability, p entity.GenerationParams) error {
	if getString(p, "name") == "" {
		return errSubjectNameRequired
	}
	imgs := getStrings(p, "images")
	vids := getStrings(p, "videos")
	// 图片与视频至少一项（纯图 1-3 张 / 纯视频 1 个；同时存在时仅视频生效）
	if len(imgs) == 0 && len(vids) == 0 {
		return errSubjectMediaRequired
	}
	if len(imgs) > 3 {
		return errSubjectImagesRange
	}
	if len(vids) > 1 {
		return errSubjectVideosRange
	}
	return nil
}

func (subjectAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	// 注意：不注入 payload——主体 API 参数表未声明该字段，严格校验下未声明
	// 字段直接 400 BadRequest（同步端点也无回调关联需求，payload 无用武之地）
	body := map[string]any{"name": getString(p, "name")}
	if imgs := getStrings(p, "images"); len(imgs) > 0 {
		body["images"] = imgs
	}
	if vids := getStrings(p, "videos"); len(vids) > 0 {
		body["videos"] = vids
	}
	if v := getString(p, "voice_id"); v != "" {
		body["voice_id"] = v
	}
	return body, nil
}

package viduendpoint

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- 文生视频 /ent/v2/text2video ----
// 参数：model/prompt/duration/resolution/seed/audio/audio_type/bgm/callback_url/style/aspect_ratio/movement_amplitude/watermark/off_peak/payload/meta_data

type text2videoAdapter struct{}

func (text2videoAdapter) Type() string     { return "text2video" }
func (text2videoAdapter) Endpoint() string { return "/ent/v2/text2video" }

func (text2videoAdapter) Category() string { return entity.GenerationTypeVideo }

func (text2videoAdapter) Validate(ctx context.Context, model string, p entity.GenerationParams) error {
	cap, err := capabilityFor(text2videoCaps, model)
	if err != nil {
		return err
	}
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
func (img2videoAdapter) Endpoint() string { return "/ent/v2/img2video" }

func (img2videoAdapter) Category() string { return entity.GenerationTypeVideo }

func (img2videoAdapter) Validate(ctx context.Context, model string, p entity.GenerationParams) error {
	cap, err := capabilityFor(img2videoCaps, model)
	if err != nil {
		return err
	}
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
func (startEnd2videoAdapter) Endpoint() string { return "/ent/v2/start-end2video" }

func (startEnd2videoAdapter) Category() string { return entity.GenerationTypeVideo }

func (startEnd2videoAdapter) Validate(ctx context.Context, model string, p entity.GenerationParams) error {
	cap, err := capabilityFor(startEnd2videoCaps, model)
	if err != nil {
		return err
	}
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
func (reference2videoAdapter) Endpoint() string { return "/ent/v2/reference2video" }

func (reference2videoAdapter) Category() string { return entity.GenerationTypeVideo }

func (reference2videoAdapter) Validate(ctx context.Context, model string, p entity.GenerationParams) error {
	cap, err := capabilityFor(reference2videoCaps, model)
	if err != nil {
		return err
	}
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
func (multiframeAdapter) Endpoint() string { return "/ent/v2/multiframe" }

func (multiframeAdapter) Category() string { return entity.GenerationTypeVideo }

func (multiframeAdapter) Validate(ctx context.Context, model string, p entity.GenerationParams) error {
	cap, err := capabilityFor(multiframeCaps, model)
	if err != nil {
		return err
	}
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
func (digitalHumanAdapter) Endpoint() string { return "/ent/v2/digital-human" }

func (digitalHumanAdapter) Category() string { return entity.GenerationTypeDigitalHuman }

func (digitalHumanAdapter) Validate(ctx context.Context, model string, p entity.GenerationParams) error {
	cap, err := capabilityFor(digitalHumanCaps, model)
	if err != nil {
		return err
	}
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

// ---- 主体 API /ent/v2/subjects ----
// 参数：name/images(≤3)/videos(≤1 个 5 秒，仅 q2-pro)/voice_id；返回 server_id。
// 主体创建后可供 reference2video 的 subjects[].server_id 复用。

type subjectAdapter struct{}

func (subjectAdapter) Type() string     { return "subject" }
func (subjectAdapter) Endpoint() string { return "/ent/v2/subjects" }

func (subjectAdapter) Category() string { return entity.GenerationTypeOther }

func (subjectAdapter) Validate(ctx context.Context, model string, p entity.GenerationParams) error {
	if getString(p, "name") == "" {
		return errSubjectNameRequired
	}
	if n := len(getStrings(p, "images")); n < 1 || n > 3 {
		return errSubjectImagesRange
	}
	return nil
}

func (subjectAdapter) BuildRequest(ctx context.Context, model string, p entity.GenerationParams, payload string) (map[string]any, error) {
	body := map[string]any{
		"name":   getString(p, "name"),
		"images": getStrings(p, "images"),
	}
	if v := getString(p, "voice_id"); v != "" {
		body["voice_id"] = v
	}
	if payload != "" {
		body["payload"] = payload
	}
	return body, nil
}

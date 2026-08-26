package viduendpoint

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

// ---- text2video BuildRequest ----

func TestText2VideoBuildRequest(t *testing.T) {
	a := text2videoAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"prompt":   "品牌宣传视频",
		"duration": 8,
		"seed":     42,
		"style":    "cinematic",
		"audio":    true,
	}, "payload-1")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["model"], "viduq3-pro")
	assertEq(t, body["prompt"], "品牌宣传视频")
	assertEq(t, body["duration"], 8)
	assertEq(t, body["seed"], 42)
	assertEq(t, body["style"], "cinematic")
	assertEq(t, body["audio"], true)
	assertEq(t, body["payload"], "payload-1")
	assertEq(t, body["resolution"], "1080p") // 默认值
	assertEq(t, body["aspect_ratio"], "16:9") // 默认值
}

func TestText2VideoBuildRequest_NoOptional(t *testing.T) {
	a := text2videoAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq2", entity.GenerationParams{
		"prompt": "测试",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	if _, ok := body["duration"]; ok {
		t.Error("未传 duration 不应出现在 body")
	}
	if _, ok := body["seed"]; ok {
		t.Error("未传 seed 不应出现在 body")
	}
	if _, ok := body["payload"]; ok {
		t.Error("空 payload 不应出现在 body")
	}
}

// ---- img2video BuildRequest ----

func TestImg2VideoBuildRequest(t *testing.T) {
	a := img2videoAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"images":   []string{"https://cdn.example.com/img.jpg"},
		"prompt":   "图生视频",
		"duration": 5,
	}, "p-1")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["model"], "viduq3-pro")
	imgs := body["images"].([]string)
	assertEq(t, len(imgs), 1)
	assertEq(t, imgs[0], "https://cdn.example.com/img.jpg")
	assertEq(t, body["prompt"], "图生视频")
	assertEq(t, body["duration"], 5)
	assertEq(t, body["payload"], "p-1")
}

// ---- startEnd2video BuildRequest ----

func TestStartEnd2VideoBuildRequest(t *testing.T) {
	a := startEnd2videoAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"images": []string{"https://cdn.example.com/start.jpg", "https://cdn.example.com/end.jpg"},
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	imgs := body["images"].([]string)
	assertEq(t, len(imgs), 2)
}

// ---- reference2video BuildRequest ----

func TestReference2VideoBuildRequest_SubjectsMode(t *testing.T) {
	a := reference2videoAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"subjects": []map[string]any{
			{"name": "s1", "server_id": "subj-123"},
		},
		"prompt": "@s1 在跳舞",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["model"], "viduq3-pro")
	assertEq(t, body["prompt"], "@s1 在跳舞")
	subs := body["subjects"].([]map[string]any)
	assertEq(t, len(subs), 1)
	assertEq(t, subs[0]["server_id"], "subj-123")
	if _, ok := body["images"]; ok {
		t.Error("主体模式不应出现 images")
	}
}

func TestReference2VideoBuildRequest_ImageMode(t *testing.T) {
	a := reference2videoAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"images": []string{"a.jpg", "b.jpg", "c.jpg"},
		"prompt": "参考生视频",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	imgs := body["images"].([]string)
	assertEq(t, len(imgs), 3)
	if _, ok := body["subjects"]; ok {
		t.Error("图片模式不应出现 subjects")
	}
}

// ---- multiframe BuildRequest ----

func TestMultiframeBuildRequest(t *testing.T) {
	a := multiframeAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"start_image": "https://cdn.example.com/start.jpg",
		"image_settings": []map[string]any{
			{"key_image": "https://cdn.example.com/k1.jpg", "prompt": "帧1", "duration": 3},
			{"key_image": "https://cdn.example.com/k2.jpg", "prompt": "帧2", "duration": 4},
		},
		"aspect_ratio": "9:16",
	}, "p-1")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["start_image"], "https://cdn.example.com/start.jpg")
	assertEq(t, body["aspect_ratio"], "9:16")
	assertEq(t, body["payload"], "p-1")
	frames := body["image_settings"].([]map[string]any)
	assertEq(t, len(frames), 2)
}

// ---- digitalHuman BuildRequest ----

func TestDigitalHumanBuildRequest_AudioDriven(t *testing.T) {
	a := digitalHumanAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"image":     "https://cdn.example.com/portrait.jpg",
		"audio_url": "https://cdn.example.com/audio.mp3",
		"prompt":    "数字人口播",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["image"], "https://cdn.example.com/portrait.jpg")
	assertEq(t, body["audio_url"], "https://cdn.example.com/audio.mp3")
	assertEq(t, body["prompt"], "数字人口播")
	if _, ok := body["text"]; ok {
		t.Error("audio_url 存在时不应出现 text")
	}
}

func TestDigitalHumanBuildRequest_TextDriven(t *testing.T) {
	a := digitalHumanAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq3-pro", entity.GenerationParams{
		"images":   []string{"https://cdn.example.com/portrait.jpg"},
		"text":     "你好世界",
		"voice_id": "female-shaonv",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	// images[0] 应降级为 image
	assertEq(t, body["image"], "https://cdn.example.com/portrait.jpg")
	assertEq(t, body["text"], "你好世界")
	assertEq(t, body["voice_id"], "female-shaonv")
	if _, ok := body["audio_url"]; ok {
		t.Error("text 驱动时不应出现 audio_url")
	}
}

// ---- lipSync BuildRequest ----

func TestLipSyncBuildRequest_AudioDriven(t *testing.T) {
	a := lipSyncAdapter{}
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"video_url": "https://cdn.example.com/video.mp4",
		"audio_url": "https://cdn.example.com/audio.mp3",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["video_url"], "https://cdn.example.com/video.mp4")
	assertEq(t, body["audio_url"], "https://cdn.example.com/audio.mp3")
	if _, ok := body["text"]; ok {
		t.Error("audio_url 存在时不应出现 text")
	}
	if _, ok := body["model"]; ok {
		t.Error("lip_sync 不应注入 model（文档未声明）")
	}
}

func TestLipSyncBuildRequest_TextDriven(t *testing.T) {
	a := lipSyncAdapter{}
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"video_url":     "https://cdn.example.com/video.mp4",
		"text":          "这是一段口播文案内容",
		"voice_id":      "female-shaonv",
		"speed":         1.2,
		"volume":        8,
		"ref_photo_url": "https://cdn.example.com/ref.jpg",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["text"], "这是一段口播文案内容")
	assertEq(t, body["voice_id"], "female-shaonv")
	assertEq(t, body["speed"], 1.2)
	assertEq(t, body["volume"], 8)
	assertEq(t, body["ref_photo_url"], "https://cdn.example.com/ref.jpg")
	if _, ok := body["audio_url"]; ok {
		t.Error("text 驱动时不应出现 audio_url")
	}
}

// ---- subject BuildRequest ----

func TestSubjectBuildRequest_ImagesAndVoice(t *testing.T) {
	a := subjectAdapter{}
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"name":     "品牌分身",
		"images":   []string{"https://cdn.example.com/face1.jpg", "https://cdn.example.com/face2.jpg"},
		"voice_id": "female-shaonv",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["name"], "品牌分身")
	imgs := body["images"].([]string)
	assertEq(t, len(imgs), 2)
	assertEq(t, body["voice_id"], "female-shaonv")
	if _, ok := body["payload"]; ok {
		t.Error("subject 不应注入 payload")
	}
}

func TestSubjectBuildRequest_VideoOnly(t *testing.T) {
	a := subjectAdapter{}
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"name":   "视频主体",
		"videos": []string{"https://cdn.example.com/ref.mp4"},
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	vids := body["videos"].([]string)
	assertEq(t, len(vids), 1)
	if _, ok := body["images"]; ok {
		t.Error("纯视频模式不应出现 images")
	}
}

// ---- text2image BuildRequest ----

func TestText2ImageBuildRequest_WithImages(t *testing.T) {
	a := text2imageAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq2", entity.GenerationParams{
		"prompt": "生成图片",
		"images": []string{"https://cdn.example.com/ref1.jpg", "https://cdn.example.com/ref2.jpg"},
	}, "p-1")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["model"], "viduq2")
	assertEq(t, body["prompt"], "生成图片")
	imgs := body["images"].([]string)
	assertEq(t, len(imgs), 2)
	assertEq(t, body["payload"], "p-1")
}

func TestText2ImageBuildRequest_PureText(t *testing.T) {
	a := text2imageAdapter{}
	body, err := a.BuildRequest(context.Background(), "viduq2", entity.GenerationParams{
		"prompt": "纯文生图",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	if _, ok := body["images"]; ok {
		t.Error("纯文生图不应出现 images")
	}
}

// ---- text2audio BuildRequest ----

func TestText2AudioBuildRequest(t *testing.T) {
	a := text2audioAdapter{}
	body, err := a.BuildRequest(context.Background(), "audio1.0", entity.GenerationParams{
		"prompt":   "生成音效",
		"duration": 5,
		"seed":     123,
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["model"], "audio1.0")
	assertEq(t, body["prompt"], "生成音效")
	assertEq(t, body["duration"], 5)
	assertEq(t, body["seed"], 123)
	if _, ok := body["payload"]; ok {
		t.Error("text2audio 不应注入 payload")
	}
}

// ---- soundEffect BuildRequest ----

func TestSoundEffectBuildRequest(t *testing.T) {
	a := soundEffectAdapter{}
	body, err := a.BuildRequest(context.Background(), "audio1.0", entity.GenerationParams{
		"timing_prompts": []map[string]any{
			{"from": 0, "to": 3, "prompt": "脚步声"},
			{"from": 5, "to": 8, "prompt": "门响"},
		},
		"duration": 10,
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["model"], "audio1.0")
	assertEq(t, body["duration"], 10)
	events := body["timing_prompts"].([]map[string]any)
	assertEq(t, len(events), 2)
}

// ---- tts BuildRequest ----

func TestTTSBuildRequest(t *testing.T) {
	a := ttsAdapter{}
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"text":                   "你好世界",
		"voice_setting_voice_id": "female-shaonv",
		"voice_setting_speed":    1.5,
		"voice_setting_volume":   8,
		"voice_setting_pitch":    3,
		"voice_setting_emotion":  "happy",
	}, "p-1")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["text"], "你好世界")
	assertEq(t, body["voice_setting_voice_id"], "female-shaonv")
	assertEq(t, body["voice_setting_speed"], 1.5)
	assertEq(t, body["voice_setting_volume"], 8)
	assertEq(t, body["voice_setting_pitch"], 3)
	assertEq(t, body["voice_setting_emotion"], "happy")
	assertEq(t, body["payload"], "p-1")
}

func TestTTSBuildRequest_Minimal(t *testing.T) {
	a := ttsAdapter{}
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"text":                   "最简参数",
		"voice_setting_voice_id": "default",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	if _, ok := body["voice_setting_speed"]; ok {
		t.Error("未传 speed 不应出现在 body")
	}
	if _, ok := body["payload"]; ok {
		t.Error("空 payload 不应出现在 body")
	}
}

// ---- voiceClone BuildRequest ----

func TestVoiceCloneBuildRequest(t *testing.T) {
	a := voiceCloneAdapter{}
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"audio_url":        "https://cdn.example.com/voice.mp3",
		"voice_id":         "my-voice-001",
		"prompt_audio_url": "https://cdn.example.com/prompt.mp3",
		"text":             "试听文本",
	}, "")
	if err != nil {
		t.Fatalf("BuildRequest 失败: %v", err)
	}
	assertEq(t, body["audio_url"], "https://cdn.example.com/voice.mp3")
	assertEq(t, body["voice_id"], "my-voice-001")
	assertEq(t, body["prompt_audio_url"], "https://cdn.example.com/prompt.mp3")
	assertEq(t, body["text"], "试听文本")
}

// ---- helper ----

func assertEq(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Errorf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}

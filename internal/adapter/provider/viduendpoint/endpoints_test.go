package viduendpoint

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

func TestText2VideoValidate(t *testing.T) {
	a := text2videoAdapter{}
	ctx := context.Background()

	// viduq1 仅支持 1080p（caps[3]=viduq1）
	err := a.Validate(ctx, text2videoCaps[3], entity.GenerationParams{"prompt": "x", "resolution": "720p"})
	if err == nil {
		t.Error("viduq1 传 720p 应报错（仅 1080p）")
	}
	if err := a.Validate(ctx, text2videoCaps[3], entity.GenerationParams{"prompt": "x", "resolution": "1080p"}); err != nil {
		t.Errorf("viduq1 1080p 应通过: %v", err)
	}
	// 缺 prompt
	if err := a.Validate(ctx, text2videoCaps[0], entity.GenerationParams{}); err == nil {
		t.Error("缺 prompt 应报错")
	}
	// 时长范围（q3 1-16：17 应报错）
	if err := a.Validate(ctx, text2videoCaps[0], entity.GenerationParams{"prompt": "x", "duration": 17}); err == nil {
		t.Error("viduq3-pro 时长 17s 应报错（上限 16）")
	}
}

func TestRegistryCapabilityUnregisteredModel(t *testing.T) {
	// 模型存在性由 Registry（DB 驱动）负责——端点策略不再检查
	r := NewRegistry()
	if _, err := r.Capability(ctxForTest(), "text2video", "viduq9"); err == nil {
		t.Error("未注册模型 Capability 应报错")
	}
	if _, err := r.Capability(ctxForTest(), "text2video", "viduq3-pro"); err != nil {
		t.Errorf("已注册模型应通过: %v", err)
	}
}

func ctxForTest() context.Context { return context.Background() }

func TestImg2VideoImageSlots(t *testing.T) {
	a := img2videoAdapter{}
	ctx := context.Background()
	// 图生视频恰好 1 张
	if err := a.Validate(ctx, img2videoCaps[0], entity.GenerationParams{"images": []string{"a", "b"}}); err == nil {
		t.Error("图生视频 2 张图应报错（恰 1 张）")
	}
	if err := a.Validate(ctx, img2videoCaps[0], entity.GenerationParams{"images": []string{"a"}}); err != nil {
		t.Errorf("图生视频 1 张应通过: %v", err)
	}
}

func TestStartEnd2VideoImageSlots(t *testing.T) {
	a := startEnd2videoAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, startEnd2videoCaps[0], entity.GenerationParams{"images": []string{"a"}}); err == nil {
		t.Error("首尾帧 1 张图应报错（恰 2 张）")
	}
	if err := a.Validate(ctx, startEnd2videoCaps[0], entity.GenerationParams{"images": []string{"a", "b"}}); err != nil {
		t.Errorf("首尾帧 2 张应通过: %v", err)
	}
}

func TestReference2VideoSubjects(t *testing.T) {
	a := reference2videoAdapter{}
	ctx := context.Background()
	// viduq1 不支持主体模式
	err := a.Validate(ctx, reference2videoCaps[5], entity.GenerationParams{
		"subjects": []map[string]any{{"name": "s1", "images": []string{"a"}}},
		"prompt":   "@s1 在跳舞",
	})
	if err == nil {
		t.Error("viduq1 主体模式应报错（仅 q3/q2-pro 支持）")
	}
	// q3 主体模式通过
	if err := a.Validate(ctx, reference2videoCaps[1], entity.GenerationParams{
		"subjects": []map[string]any{{"name": "s1", "images": []string{"a"}}},
		"prompt":   "@s1 在跳舞",
	}); err != nil {
		t.Errorf("viduq3 主体模式应通过: %v", err)
	}
	// 非主体模式 images 1-7
	if err := a.Validate(ctx, reference2videoCaps[1], entity.GenerationParams{"images": []string{}}); err == nil {
		t.Error("images 为空应报错")
	}
}

func TestMultiframeKeyFrames(t *testing.T) {
	a := multiframeAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, multiframeCaps[0], entity.GenerationParams{
		"start_image": "s",
		"image_settings": []map[string]any{{"key_image": "k1"}, {"key_image": "k2"}},
	}); err != nil {
		t.Errorf("2 个关键帧应通过: %v", err)
	}
	if err := a.Validate(ctx, multiframeCaps[0], entity.GenerationParams{
		"start_image": "s",
		"image_settings": []map[string]any{{"key_image": "k1"}},
	}); err == nil {
		t.Error("1 个关键帧应报错（需 2-9）")
	}
	if err := a.Validate(ctx, multiframeCaps[0], entity.GenerationParams{
		"image_settings": []map[string]any{{"key_image": "k1"}, {"key_image": "k2"}},
	}); err == nil {
		t.Error("缺 start_image 应报错")
	}
}

func TestDigitalHumanValidate(t *testing.T) {
	a := digitalHumanAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, digitalHumanCaps[0], entity.GenerationParams{"images": []string{"face.jpg"}}); err != nil {
		t.Errorf("数字人 1 图应通过: %v", err)
	}
	if err := a.Validate(ctx, digitalHumanCaps[0], entity.GenerationParams{}); err == nil {
		t.Error("数字人缺图应报错")
	}
}

func TestSubjectValidate(t *testing.T) {
	a := subjectAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, entity.ModelCapability{Model: "viduq3-pro"}, entity.GenerationParams{"name": "主体A", "images": []string{"a", "b", "c", "d"}}); err == nil {
		t.Error("主体图 >3 张应报错")
	}
	if err := a.Validate(ctx, entity.ModelCapability{Model: "viduq3-pro"}, entity.GenerationParams{"name": "主体A", "images": []string{"a"}}); err != nil {
		t.Errorf("主体 1 图应通过: %v", err)
	}
	// 纯视频主体（无图）——q2-pro 视频主体场景
	if err := a.Validate(ctx, entity.ModelCapability{Model: "viduq2-pro"}, entity.GenerationParams{"name": "主体B", "videos": []string{"v.mp4"}}); err != nil {
		t.Errorf("纯视频主体应通过: %v", err)
	}
	// 图+视频同传（上游仅视频生效——放行由 Vidu 处理）
	if err := a.Validate(ctx, entity.ModelCapability{Model: "viduq2-pro"}, entity.GenerationParams{
		"name": "主体C", "images": []string{"a"}, "videos": []string{"v.mp4"},
	}); err != nil {
		t.Errorf("图+视频同传应通过: %v", err)
	}
	// 图视频全无 / 视频超 1 个
	if err := a.Validate(ctx, entity.ModelCapability{}, entity.GenerationParams{"name": "主体D"}); err == nil {
		t.Error("缺图缺视频应报错")
	}
	if err := a.Validate(ctx, entity.ModelCapability{}, entity.GenerationParams{"name": "主体E", "videos": []string{"a", "b"}}); err == nil {
		t.Error("视频 >1 个应报错")
	}
}

func TestSubjectBuildRequest(t *testing.T) {
	a := subjectAdapter{}
	if !a.IsSync() {
		t.Error("主体端点应标记为同步端点（提交即终态，不进轮询）")
	}	// 图+视频+音色全量组装
	body, err := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"name": "主体A", "images": []string{"a", "b"}, "videos": []string{"v.mp4"}, "voice_id": "vox-1",
	}, "payload-x")
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if body["name"] != "主体A" || len(body["images"].([]string)) != 2 || len(body["videos"].([]string)) != 1 || body["voice_id"] != "vox-1" {
		t.Errorf("主体请求体字段不符: %v", body)
	}
	// payload 参数表未声明——不得注入（严格校验会 400 BadRequest）
	if _, ok := body["payload"]; ok {
		t.Error("主体端点不应注入未声明的 payload 字段")
	}
	// 纯视频主体：不应携带空 images 字段
	body, _ = a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"name": "主体B", "videos": []string{"v.mp4"},
	}, "")
	if _, ok := body["images"]; ok {
		t.Errorf("纯视频主体不应携带 images 字段: %v", body)
	}
}

func TestCallbackEndpointDeclarations(t *testing.T) {
	// 仅文档声明 callback_url 的端点支持回调注入（对其余端点注入未声明参数有被拒风险）
	// 数据源：各端点文档参数表（同步接口 tts/voice_clone/subject 无回调）
	r := NewRegistry()
	supports := map[string]bool{
		"text2video": true, "img2video": true, "start_end2video": true,
		"reference2video": true, "multiframe": true, "digital_human": true,
		"text2image": true, "text2audio": true, "sound_effect": true,
		"tts": false, "voice_clone": false, "subject": false,
	}
	for subType, want := range supports {
		adapter, err := r.Get(ctxForTest(), subType)
		if err != nil {
			t.Fatalf("取端点 %s 失败: %v", subType, err)
		}
		cb, ok := adapter.(port.CallbackEndpoint)
		got := ok && cb.SupportsCallback()
		if got != want {
			t.Errorf("端点 %s 回调支持应为 %v，得到 %v", subType, want, got)
		}
	}
}

func TestUndocumentedPayloadNotSent(t *testing.T) {
	// 参数表未声明 payload 的端点不得注入——严格校验下未声明字段直接 400 BadRequest
	ctx := context.Background()
	// subject：同步端点，参数表仅 name/images/videos/voice_id
	body, _ := subjectAdapter{}.BuildRequest(ctx, "", entity.GenerationParams{
		"name": "A", "images": []string{"x"},
	}, "gen-should-not-appear")
	if _, ok := body["payload"]; ok {
		t.Error("subject 端点注入了未声明的 payload 字段（会 400）")
	}
	// text2audio / sound_effect：文档参数表无 payload
	body, _ = text2audioAdapter{}.BuildRequest(ctx, "audio1.0", entity.GenerationParams{
		"prompt": "x",
	}, "gen-should-not-appear")
	if _, ok := body["payload"]; ok {
		t.Error("text2audio 端点注入了未声明的 payload 字段（会 400）")
	}
	body, _ = soundEffectAdapter{}.BuildRequest(ctx, "audio1.0", entity.GenerationParams{
		"timing_prompts": []map[string]any{{"from": 0, "to": 2, "prompt": "x"}},
	}, "gen-should-not-appear")
	if _, ok := body["payload"]; ok {
		t.Error("sound_effect 端点注入了未声明的 payload 字段（会 400）")
	}
	// 对照：声明了 payload 的端点照常透传
	body, _ = text2videoAdapter{}.BuildRequest(ctx, "viduq2", entity.GenerationParams{
		"prompt": "x",
	}, "gen-ok")
	if body["payload"] != "gen-ok" {
		t.Error("text2video 端点 payload 应照常透传")
	}
}

func TestLipSyncValidate(t *testing.T) {
	a := lipSyncAdapter{}
	ctx := context.Background()
	// 音频驱动
	if err := a.Validate(ctx, lipSyncCaps[0], entity.GenerationParams{
		"video_url": "https://x/v.mp4", "audio_url": "https://x/a.mp3",
	}); err != nil {
		t.Errorf("音频驱动应通过: %v", err)
	}
	// 文本驱动
	if err := a.Validate(ctx, lipSyncCaps[0], entity.GenerationParams{
		"video_url": "https://x/v.mp4", "text": "你好欢迎使用",
	}); err != nil {
		t.Errorf("文本驱动应通过: %v", err)
	}
	// 缺视频 / 音文全缺 / 文本过短
	if err := a.Validate(ctx, lipSyncCaps[0], entity.GenerationParams{"audio_url": "a"}); err == nil {
		t.Error("缺 video_url 应报错")
	}
	if err := a.Validate(ctx, lipSyncCaps[0], entity.GenerationParams{"video_url": "v"}); err == nil {
		t.Error("音频文本全缺应报错")
	}
	if err := a.Validate(ctx, lipSyncCaps[0], entity.GenerationParams{"video_url": "v", "text": "短"}); err == nil {
		t.Error("文本 <4 字符应报错")
	}
}

func TestLipSyncBuildRequest(t *testing.T) {
	a := lipSyncAdapter{}
	// 音频驱动：不携带文本驱动参数（speed/voice_id 失效）
	body, _ := a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"video_url": "https://x/v.mp4", "audio_url": "https://x/a.mp3",
		"text": "同传文本", "voice_id": "vox",
	}, "payload-x")
	if body["audio_url"] == nil || body["text"] != nil {
		t.Errorf("音频驱动应以 audio_url 为准（text 不发），得到 %v", body)
	}
	// 文本驱动：携带 voice_id/speed/volume
	body, _ = a.BuildRequest(context.Background(), "", entity.GenerationParams{
		"video_url": "https://x/v.mp4", "text": "你好欢迎使用",
		"voice_id": "female-shaonv", "speed": 1.2, "volume": 3,
	}, "")
	if body["voice_id"] != "female-shaonv" || body["speed"] != 1.2 || body["volume"] != 3 {
		t.Errorf("文本驱动应携带音色/语速/音量，得到 %v", body)
	}
	if _, ok := body["payload"]; ok {
		t.Error("对口型参数表未声明 payload——不得注入（会 400）")
	}
}

func TestReference2VideoPickModel(t *testing.T) {
	a := reference2videoAdapter{}
	models := reference2videoCaps
	// 图片主体 → q3 系（效果最好）
	got := a.PickModel(models, entity.GenerationParams{
		"subjects": []map[string]any{{"name": "s", "images": []string{"a"}}},
	})
	if got != "viduq3-turbo" && got != "viduq3" && got != "viduq3-mix" {
		t.Errorf("图片主体应选 q3 系，得到 %q", got)
	}
	// 视频主体 → 唯一支持的 q2-pro
	got = a.PickModel(models, entity.GenerationParams{
		"subjects": []map[string]any{{"name": "s", "videos": []any{"v.mp4"}}},
	})
	if got != "viduq2-pro" {
		t.Errorf("视频主体应选 viduq2-pro（唯一 VideoSlots>0），得到 %q", got)
	}
}

func TestSeedDefaultsConvergeClosedModes(t *testing.T) {
	// 收敛后：新部署 seed 的 8 个关闭模式 Enabled=false，保留的 5 端点 true
	for subType := range ClosedDefaultModes {
		if subType == "" {
			continue
		}
	}
	kept := []string{"reference2video", "subject", "lip_sync", "tts", "voice_clone"}
	for _, st := range kept {
		if ClosedDefaultModes[st] {
			t.Errorf("保留端点 %s 不应在关闭清单", st)
		}
	}
	want := []string{"text2video", "img2video", "digital_human", "text2image"}
	for _, st := range want {
		if !ClosedDefaultModes[st] {
			t.Errorf("收敛端点 %s 应在关闭清单", st)
		}
	}
}

package viduendpoint

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

func TestText2ImageValidate(t *testing.T) {
	a := text2imageAdapter{}
	ctx := context.Background()
	// viduq2 允许 0 张（纯文生图）
	if err := a.Validate(ctx, text2imageCaps[0], entity.GenerationParams{"prompt": "一只猫"}); err != nil {
		t.Errorf("viduq2 纯文生图应通过: %v", err)
	}
	// viduq1 必须 1-7 张
	if err := a.Validate(ctx, text2imageCaps[1], entity.GenerationParams{"prompt": "一只猫"}); err == nil {
		t.Error("viduq1 无参考图应报错")
	}
	if err := a.Validate(ctx, text2imageCaps[1], entity.GenerationParams{"prompt": "一只猫", "images": []string{"a"}}); err != nil {
		t.Errorf("viduq1 1 张参考图应通过: %v", err)
	}
	// 缺 prompt
	if err := a.Validate(ctx, text2imageCaps[0], entity.GenerationParams{}); err == nil {
		t.Error("缺 prompt 应报错")
	}
	// 超过 7 张
	if err := a.Validate(ctx, text2imageCaps[0], entity.GenerationParams{"prompt": "x", "images": []string{"1", "2", "3", "4", "5", "6", "7", "8"}}); err == nil {
		t.Error("8 张图应报错")
	}
}

func TestText2AudioValidate(t *testing.T) {
	a := text2audioAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, text2audioCaps[0], entity.GenerationParams{"prompt": "清晨的鸟叫声"}); err != nil {
		t.Errorf("合法参数应通过: %v", err)
	}
	if err := a.Validate(ctx, text2audioCaps[0], entity.GenerationParams{"prompt": "x", "duration": 11}); err == nil {
		t.Error("时长 11s 应报错（上限 10）")
	}
	if err := a.Validate(ctx, text2audioCaps[0], entity.GenerationParams{}); err == nil {
		t.Error("缺 prompt 应报错")
	}
}

func TestSoundEffectValidate(t *testing.T) {
	a := soundEffectAdapter{}
	ctx := context.Background()
	events := []map[string]any{{"from": 0.0, "to": 3.0, "prompt": "门铃声"}}
	if err := a.Validate(ctx, soundEffectCaps[0], entity.GenerationParams{"timing_prompts": events, "duration": 10}); err != nil {
		t.Errorf("合法事件应通过: %v", err)
	}
	// 时间越界
	bad := []map[string]any{{"from": 0.0, "to": 12.0, "prompt": "x"}}
	if err := a.Validate(ctx, soundEffectCaps[0], entity.GenerationParams{"timing_prompts": bad, "duration": 10}); err == nil {
		t.Error("to > duration 应报错")
	}
	// 空事件
	if err := a.Validate(ctx, soundEffectCaps[0], entity.GenerationParams{"duration": 10}); err == nil {
		t.Error("空 timing_prompts 应报错")
	}
}

func TestTTSValidate(t *testing.T) {
	a := ttsAdapter{}
	ctx := context.Background()
	p := entity.GenerationParams{"text": "你好", "voice_setting_voice_id": "vidu01"}
	if err := a.Validate(ctx, ttsCaps[0], p); err != nil {
		t.Errorf("合法 TTS 应通过: %v", err)
	}
	if err := a.Validate(ctx, ttsCaps[0], entity.GenerationParams{"text": "你好"}); err == nil {
		t.Error("缺音色应报错")
	}
	bad := entity.GenerationParams{"text": "你好", "voice_setting_voice_id": "vidu01", "voice_setting_speed": 3.0}
	if err := a.Validate(ctx, ttsCaps[0], bad); err == nil {
		t.Error("语速 3 应报错（0.5-2）")
	}
}

func TestVoiceCloneValidate(t *testing.T) {
	a := voiceCloneAdapter{}
	ctx := context.Background()
	p := entity.GenerationParams{"audio_url": "https://x/a.mp3", "voice_id": "myvoice001", "text": "试听"}
	if err := a.Validate(ctx, voiceCloneCaps[0], p); err != nil {
		t.Errorf("合法复刻应通过: %v", err)
	}
	// voice_id 过短
	bad := entity.GenerationParams{"audio_url": "https://x/a.mp3", "voice_id": "ab", "text": "试听"}
	if err := a.Validate(ctx, voiceCloneCaps[0], bad); err == nil {
		t.Error("voice_id 过短应报错")
	}
	// 非法字符
	bad2 := entity.GenerationParams{"audio_url": "https://x/a.mp3", "voice_id": "my voice bad!", "text": "试听"}
	if err := a.Validate(ctx, voiceCloneCaps[0], bad2); err == nil {
		t.Error("voice_id 含空格应报错")
	}
	// 数字开头
	bad3 := entity.GenerationParams{"audio_url": "https://x/a.mp3", "voice_id": "1myvoice01", "text": "试听"}
	if err := a.Validate(ctx, voiceCloneCaps[0], bad3); err == nil {
		t.Error("voice_id 数字开头应报错")
	}
}

package generation

import (
	"context"
	"strings"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// fakeVoiceLibrary 最小音色仓储（缺口C改写钩子测试用）。
type fakeVoiceLibrary struct {
	voices map[string]entity.GenerationVoice
}

func (f *fakeVoiceLibrary) ListForUser(context.Context, string) ([]entity.GenerationVoice, error) {
	return nil, nil
}
func (f *fakeVoiceLibrary) ListForAdmin(context.Context, string) ([]entity.GenerationVoice, error) {
	return nil, nil
}
func (f *fakeVoiceLibrary) SeedIfEmpty(context.Context, []entity.GenerationVoice) (int, error) {
	return 0, nil
}
func (f *fakeVoiceLibrary) Upsert(_ context.Context, v entity.GenerationVoice) error {
	f.voices[v.VoiceID] = v
	return nil
}
func (f *fakeVoiceLibrary) GetDefault(context.Context) (entity.GenerationVoice, error) {
	return entity.GenerationVoice{}, nil
}
func (f *fakeVoiceLibrary) SetDefault(context.Context, string) error { return nil }
func (f *fakeVoiceLibrary) FindByVoiceID(_ context.Context, voiceID string) (entity.GenerationVoice, error) {
	if v, ok := f.voices[voiceID]; ok {
		return v, nil
	}
	return entity.GenerationVoice{}, pkg.ErrNotFound
}

func TestMaybeRewriteSampleSynthesis_CloneVoice(t *testing.T) {
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, &fakeRepo{})
	uc.SetVoiceRepo(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"voice-clone-abc": {VoiceID: "voice-clone-abc", Scope: "clone",
			SampleURL: "https://media.example.com/t1/c-123.mp3"},
	}})

	in := SubmitInput{
		TenantID: "t1", SubType: "tts",
		Params: entity.GenerationParams{
			"text":                  "你好欢迎光临",
			"voice_setting_voice_id": "voice-clone-abc",
		},
	}
	if !uc.maybeRewriteSampleSynthesis(context.Background(), &in) {
		t.Fatal("克隆音色 tts 应触发样本合成改写")
	}
	if in.SubType != "voice_clone" {
		t.Errorf("subType 应改写为 voice_clone，得到 %s", in.SubType)
	}
	if u, _ := in.Params["audio_url"].(string); u != "https://media.example.com/t1/c-123.mp3" {
		t.Errorf("audio_url 应为已存样本永久 URL: %v", in.Params["audio_url"])
	}
	if v, _ := in.Params["voice_id"].(string); v != "voice-clone-abc" {
		t.Errorf("voice_id 应保留原克隆 ID: %v", in.Params["voice_id"])
	}
}

func TestMaybeRewriteSampleSynthesis_PlatformVoice(t *testing.T) {
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, &fakeRepo{})
	uc.SetVoiceRepo(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"platform-123": {VoiceID: "platform-123", Scope: "platform",
			SampleURL: "https://media.example.com/platform.mp3"},
	}})
	in := SubmitInput{SubType: "tts", Params: entity.GenerationParams{
		"text": "平台音色测试", "voice_setting_voice_id": "platform-123"}}
	if !uc.maybeRewriteSampleSynthesis(context.Background(), &in) {
		t.Fatal("平台音色（各厂商均未注册的 ID）必须走样本合成")
	}
}

func TestMaybeRewriteSampleSynthesis_NotHitCases(t *testing.T) {
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, &fakeRepo{})
	cloneWithSample := &fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"voice-clone-abc": {VoiceID: "voice-clone-abc", Scope: "clone",
			SampleURL: "https://media.example.com/a.mp3"},
		"voice-clone-nosample": {VoiceID: "voice-clone-nosample", Scope: "clone"},
	}}
	uc.SetVoiceRepo(cloneWithSample)

	cases := []SubmitInput{
		// 非 tts
		{SubType: "lip_sync", Params: entity.GenerationParams{"voice_id": "voice-clone-abc"}},
		// 音色不存在（官方 vidu 音色——FindByVoiceID 返回 NotFound）
		{SubType: "tts", Params: entity.GenerationParams{"text": "x", "voice_setting_voice_id": "female-shaonv"}},
		// 克隆音色但样本未转存（无 http sample_url）
		{SubType: "tts", Params: entity.GenerationParams{"text": "x", "voice_setting_voice_id": "voice-clone-nosample"}},
		// 文本超 1000（样本合成通道上限——保持原 tts）
		{SubType: "tts", Params: entity.GenerationParams{
			"text":                  strings.Repeat("超", 1001),
			"voice_setting_voice_id": "voice-clone-abc"}},
	}
	for i, in := range cases {
		if uc.maybeRewriteSampleSynthesis(context.Background(), &in) {
			t.Errorf("case %d 不应触发改写（subType=%s）", i, in.SubType)
		}
	}
}

// TestSubmitRewritesCloneVoiceTTS 端到端：tts+克隆音色提交 → 落库任务为 voice_clone
// 且 params 携带样本 URL（厂商隔离——不把克隆 voice_id 交给厂商 TTS）。
func TestSubmitRewritesCloneVoiceTTS(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, repo)
	uc.SetVoiceRepo(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"voice-clone-abc": {VoiceID: "voice-clone-abc", Scope: "clone",
			SampleURL: "https://media.example.com/t1/c-123.mp3"},
	}})
	task, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "tts",
		Params: entity.GenerationParams{
			"text":                  "你好",
			"voice_setting_voice_id": "voice-clone-abc",
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if task.SubType != "voice_clone" {
		t.Errorf("落库任务应为 voice_clone，得到 %s", task.SubType)
	}
	if !strings.Contains(task.ParamsJSON, "media.example.com") {
		t.Errorf("params 应携带样本 URL: %s", task.ParamsJSON)
	}
}

package generation

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// fakeVoiceLibrary 最小音色仓储（缺口C改写钩子 + 31号 L2 物化层测试用）。
type fakeVoiceLibrary struct {
	voices map[string]entity.GenerationVoice
	// updated 记录 UpdateViduRegisteredAt 调用（nil 时间=失效）
	updated map[string]*time.Time
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
func (f *fakeVoiceLibrary) UpdateViduRegisteredAt(_ context.Context, voiceID string, t *time.Time) error {
	if f.updated == nil {
		f.updated = map[string]*time.Time{}
	}
	f.updated[voiceID] = t
	return nil
}
func (f *fakeVoiceLibrary) DeleteClone(_ context.Context, tenantID, voiceID string) error {
	if v, ok := f.voices[voiceID]; ok && v.Scope == "clone" && v.TenantID == tenantID {
		delete(f.voices, voiceID)
	}
	return nil
}

// fakeViduProvider 注册用假 Vidu provider（map key "vidu"；Submit 同步成功）。
type fakeViduProvider struct{ submitted *map[string]any }

func (fakeViduProvider) Name() string { return "vidu" }
func (p *fakeViduProvider) Submit(_ context.Context, _ string, body map[string]any) (port.SubmitResult, error) {
	if p.submitted != nil {
		*p.submitted = body
	}
	return port.SubmitResult{TaskID: "reg-1", State: entity.TaskStateSuccess}, nil
}
func (fakeViduProvider) Poll(context.Context, string) (port.GenerationStatus, error) {
	return port.GenerationStatus{State: entity.TaskStateSuccess}, nil
}
func (fakeViduProvider) Cancel(context.Context, string) error    { return nil }
func (fakeViduProvider) VerifyCallback(context.Context, http.Header, []byte, string) error { return nil }
func (fakeViduProvider) QueryCredits(context.Context) (int, error) { return 999, nil }
func (fakeViduProvider) TranslateError(string) string            { return "" }

func newMaterializerUC(vl *fakeVoiceLibrary) (*GenerationUseCase, *fakeViduProvider) {
	vp := &fakeViduProvider{}
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{
		"fake": fakeProvider{}, "vidu": vp,
	}, fakeRegistry{}, &fakeRepo{})
	uc.SetVoiceRepo(vl)
	return uc, vp
}

// ---- 缺口C + 31号 L3：样本合成改写（厂商感知） ----

func TestMaybeRewriteSampleSynthesis_CloneVoice(t *testing.T) {
	uc, _ := newMaterializerUC(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"voice-clone-abc": {VoiceID: "voice-clone-abc", Scope: "clone", TenantID: "t1",
			SampleURL: "https://media.example.com/t1/c-123.mp3"},
	}})

	in := SubmitInput{
		TenantID: "t1", SubType: "tts",
		Params: entity.GenerationParams{
			"text":                   "你好欢迎光临",
			"voice_setting_voice_id": "voice-clone-abc",
		},
	}
	rewrote, err := uc.maybeRewriteSampleSynthesis(context.Background(), &in, fakeProvider{})
	if err != nil || !rewrote {
		t.Fatalf("克隆音色 tts 应触发样本合成改写: rewrote=%v err=%v", rewrote, err)
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
	uc, _ := newMaterializerUC(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"platform-123": {VoiceID: "platform-123", Scope: "platform",
			SampleURL: "https://media.example.com/platform.mp3"},
	}})
	in := SubmitInput{SubType: "tts", Params: entity.GenerationParams{
		"text": "平台音色测试", "voice_setting_voice_id": "platform-123"}}
	rewrote, err := uc.maybeRewriteSampleSynthesis(context.Background(), &in, fakeProvider{})
	if err != nil || !rewrote {
		t.Fatalf("平台音色（各厂商均未注册的 ID）必须走样本合成: rewrote=%v err=%v", rewrote, err)
	}
}

// TestMaybeRewriteSampleSynthesis_MiMoLongText MiMo 通道超长文本也样本化
//（mimo-v2.5-tts 仅 9 预置 ID；voiceclone 无字数限制——2026-09-01 实测 2049 字 OK）。
func TestMaybeRewriteSampleSynthesis_MiMoLongText(t *testing.T) {
	uc, _ := newMaterializerUC(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"platform-123": {VoiceID: "platform-123", Scope: "platform",
			SampleURL: "https://media.example.com/p.mp3"},
	}})
	in := SubmitInput{TenantID: "t1", SubType: "tts", Params: entity.GenerationParams{
		"text": strings.Repeat("超", 1500), "voice_setting_voice_id": "platform-123"}}
	mimo := mimoNamedProvider{}
	rewrote, err := uc.maybeRewriteSampleSynthesis(context.Background(), &in, mimo)
	if err != nil || !rewrote {
		t.Fatalf("MiMo 通道超长文本应仍样本化: rewrote=%v err=%v", rewrote, err)
	}
	if in.SubType != "voice_clone" {
		t.Errorf("subType 应改写为 voice_clone，得到 %s", in.SubType)
	}
}

// mimoNamedProvider Name()==xiaomi-mimo 的哑 provider（只做厂商判定）。
type mimoNamedProvider struct{}

func (mimoNamedProvider) Name() string { return "xiaomi-mimo" }
func (mimoNamedProvider) Submit(context.Context, string, map[string]any) (port.SubmitResult, error) {
	return port.SubmitResult{}, nil
}
func (mimoNamedProvider) Poll(context.Context, string) (port.GenerationStatus, error) {
	return port.GenerationStatus{}, nil
}
func (mimoNamedProvider) Cancel(context.Context, string) error                    { return nil }
func (mimoNamedProvider) VerifyCallback(context.Context, http.Header, []byte, string) error { return nil }
func (mimoNamedProvider) QueryCredits(context.Context) (int, error)               { return 0, nil }
func (mimoNamedProvider) TranslateError(string) string                            { return "" }

func TestMaybeRewriteSampleSynthesis_NotHitCases(t *testing.T) {
	uc, _ := newMaterializerUC(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"voice-clone-abc": {VoiceID: "voice-clone-abc", Scope: "clone", TenantID: "",
			SampleURL: "https://media.example.com/a.mp3"},
		"voice-clone-nosample": {VoiceID: "voice-clone-nosample", Scope: "clone", TenantID: ""},
	}})
	ctx := context.Background()

	// 非 tts：不改写不报错
	in := SubmitInput{SubType: "lip_sync", Params: entity.GenerationParams{"voice_id": "voice-clone-abc"}}
	if r, err := uc.maybeRewriteSampleSynthesis(ctx, &in, fakeProvider{}); r || err != nil {
		t.Errorf("lip_sync 不应触发改写: r=%v err=%v", r, err)
	}
	// 音色不在库（上游原生 ID）：不改写不报错
	in = SubmitInput{SubType: "tts", Params: entity.GenerationParams{"text": "x", "voice_setting_voice_id": "female-shaonv"}}
	if r, err := uc.maybeRewriteSampleSynthesis(ctx, &in, fakeProvider{}); r || err != nil {
		t.Errorf("库外音色应保持原流程: r=%v err=%v", r, err)
	}
	// 克隆音色样本未转存（Vidu 通道）→ 注册前置失败 → 显式报错（宁报错不变声）
	in = SubmitInput{SubType: "tts", Params: entity.GenerationParams{"text": "x", "voice_setting_voice_id": "voice-clone-nosample"}}
	if _, err := uc.maybeRewriteSampleSynthesis(ctx, &in, fakeProvider{}); err == nil {
		t.Error("样本未转存应显式报错而非静默保持原流程")
	}
	// 租户越权（clone 行属他租户）→ 显式"音色不存在"
	uc2, _ := newMaterializerUC(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"voice-clone-abc": {VoiceID: "voice-clone-abc", Scope: "clone", TenantID: "other",
			SampleURL: "https://media.example.com/a.mp3"},
	}})
	in = SubmitInput{TenantID: "t1", SubType: "tts", Params: entity.GenerationParams{
		"text": "x", "voice_setting_voice_id": "voice-clone-abc"}}
	if _, err := uc2.maybeRewriteSampleSynthesis(ctx, &in, fakeProvider{}); err == nil || err.Error() != "音色不存在" {
		t.Errorf("跨租户克隆音色应报'音色不存在'，得到 err=%v", err)
	}
}

// TestSubmitRewritesCloneVoiceTTS 端到端：tts+克隆音色提交 → 落库任务为 voice_clone
// 且 params 携带样本 URL（厂商隔离——不把克隆 voice_id 交给厂商 TTS）。
func TestSubmitRewritesCloneVoiceTTS(t *testing.T) {
	repo := &fakeRepo{}
	uc, _ := newMaterializerUC(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"voice-clone-abc": {VoiceID: "voice-clone-abc", Scope: "clone", TenantID: "t1",
			SampleURL: "https://media.example.com/t1/c-123.mp3"},
	}})
	uc.repo = repo
	task, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "tts",
		Params: entity.GenerationParams{
			"text":                   "你好",
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

// ---- 31号 L2：ensureViduVoiceID（form=vidu_id 物化） ----

func TestEnsureViduVoiceID_WindowHitSkipsRegister(t *testing.T) {
	recent := time.Now().Add(-1 * time.Hour) // 1 小时前注册，远在 144h 窗口内
	vl := &fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"platform-1": {VoiceID: "platform-1", Scope: "platform", Status: "active",
			SampleURL: "https://m.example.com/s.mp3", ViduRegisteredAt: &recent},
	}}
	uc, vp := newMaterializerUC(vl)
	vid, err := uc.ensureViduVoiceID(context.Background(), "t1", "platform-1")
	if err != nil || vid != "platform-1" {
		t.Fatalf("窗口命中应直接放行: vid=%s err=%v", vid, err)
	}
	if vp.submitted != nil && *vp.submitted != nil {
		t.Error("窗口命中不应触发注册调用")
	}
}

func TestEnsureViduVoiceID_ExpiredRegistersAndRecords(t *testing.T) {
	stale := time.Now().Add(-7 * 24 * time.Hour) // 7 天前——已过期
	vl := &fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"platform-1": {VoiceID: "platform-1", Scope: "platform", Status: "active",
			SampleURL: "https://m.example.com/s.mp3", ViduRegisteredAt: &stale},
	}}
	uc, _ := newMaterializerUC(vl)
	vid, err := uc.ensureViduVoiceID(context.Background(), "t1", "platform-1")
	if err != nil || vid != "platform-1" {
		t.Fatalf("过期应触发注册后放行: vid=%s err=%v", vid, err)
	}
	// 注册请求体的组装是 viduendpoint.voiceCloneAdapter 的契约（该包自测）——
	// 本层断言：注册被触发（见日志）且时间戳已记录
	if ts, ok := vl.updated["platform-1"]; !ok || ts == nil {
		t.Error("注册成功应记录 vidu_registered_at")
	}
}

func TestEnsureViduVoiceID_NeverRegistered(t *testing.T) {
	vl := &fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"vc-new": {VoiceID: "vc-new", Scope: "clone", TenantID: "t1", Status: "active",
			SampleURL: "https://m.example.com/n.mp3"},
	}}
	uc, _ := newMaterializerUC(vl)
	if _, err := uc.ensureViduVoiceID(context.Background(), "t1", "vc-new"); err != nil {
		t.Fatalf("未注册（NULL）应首次注册成功: %v", err)
	}
	if ts, ok := vl.updated["vc-new"]; !ok || ts == nil {
		t.Error("首次注册应记录时间戳")
	}
}

func TestEnsureViduVoiceID_TenantViolation(t *testing.T) {
	vl := &fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"vc-other": {VoiceID: "vc-other", Scope: "clone", TenantID: "t2", Status: "active",
			SampleURL: "https://m.example.com/o.mp3"},
	}}
	uc, _ := newMaterializerUC(vl)
	_, err := uc.ensureViduVoiceID(context.Background(), "t1", "vc-other")
	if err == nil || err.Error() != "音色不存在" {
		t.Fatalf("跨租户克隆音色应报'音色不存在'，得到 %v", err)
	}
}

func TestEnsureViduVoiceID_DisabledAndNotReady(t *testing.T) {
	vl := &fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{
		"vc-dis":  {VoiceID: "vc-dis", Scope: "clone", TenantID: "t1", Status: "disabled"},
		"vc-wait": {VoiceID: "vc-wait", Scope: "clone", TenantID: "t1", Status: "active", SampleURL: ""},
	}}
	uc, _ := newMaterializerUC(vl)
	if _, err := uc.ensureViduVoiceID(context.Background(), "t1", "vc-dis"); err == nil || err.Error() != "音色已停用" {
		t.Errorf("停用音色应报'音色已停用'，得到 %v", err)
	}
	if _, err := uc.ensureViduVoiceID(context.Background(), "t1", "vc-wait"); err == nil {
		t.Error("样本未就绪应显式报错")
	}
}

func TestEnsureViduVoiceID_UnknownIDPassthrough(t *testing.T) {
	uc, _ := newMaterializerUC(&fakeVoiceLibrary{voices: map[string]entity.GenerationVoice{}})
	// 库外 ID（上游预置）——直传不报错
	vid, err := uc.ensureViduVoiceID(context.Background(), "t1", "female-shaonv")
	if err != nil || vid != "female-shaonv" {
		t.Fatalf("库外上游 ID 应直传: vid=%s err=%v", vid, err)
	}
	// 空 ID——返回空不报错
	if vid, err := uc.ensureViduVoiceID(context.Background(), "t1", ""); err != nil || vid != "" {
		t.Fatalf("空 ID 应返回空: vid=%q err=%v", vid, err)
	}
}

package generation

// lipsync_chain_test.go —— 31号 §4.2 画面复用口播路径 + §L4-⑤ 备胎道单测。
// 定案（2026-09-01）：只保留"画面预生成（阶段0）+ 口播复用"一条路——
// 口播不生成画面任务；形象视频缺失显式报错。

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// chainSettingRepo 最小设置仓储（flag 按 key 返回）。
type chainSettingRepo struct {
	values map[string]string
}

func (r *chainSettingRepo) Get(_ context.Context, key string) (entity.SystemSetting, error) {
	if v, ok := r.values[key]; ok {
		return entity.SystemSetting{Key: key, Value: v}, nil
	}
	return entity.SystemSetting{}, pkg.ErrNotFound
}
func (r *chainSettingRepo) Save(context.Context, entity.SystemSetting) error { return nil }

// chainSubjectRepo 最小分身资产仓储（按 server_id 返回预置资产）。
type chainSubjectRepo struct {
	byServer map[string]entity.SubjectAsset
}

func (r *chainSubjectRepo) Upsert(context.Context, entity.SubjectAsset) error { return nil }
func (r *chainSubjectRepo) FindByID(context.Context, string) (entity.SubjectAsset, error) {
	return entity.SubjectAsset{}, pkg.ErrNotFound
}
func (r *chainSubjectRepo) FindByServerID(_ context.Context, serverID string) (entity.SubjectAsset, error) {
	if a, ok := r.byServer[serverID]; ok {
		return a, nil
	}
	return entity.SubjectAsset{}, pkg.ErrNotFound
}
func (r *chainSubjectRepo) ListByTenant(context.Context, string, string, string, int, int) ([]entity.SubjectAsset, int64, error) {
	return nil, 0, nil
}
func (r *chainSubjectRepo) UpdateAvatarVideoURL(context.Context, string, string) error { return nil }
func (r *chainSubjectRepo) UpdateStatus(context.Context, string, string) error        { return nil }
func (r *chainSubjectRepo) Delete(context.Context, string) error                      { return nil }

func TestExtractLipsyncChain(t *testing.T) {
	if c := extractLipsyncChain(entity.GenerationParams{"prompt": "x"}); c != nil {
		t.Error("非口播复用参数应返回 nil")
	}
	if c := extractLipsyncChain(entity.GenerationParams{"__chain": true}); c != nil {
		t.Error("复用标记但无文本/音频应返回 nil")
	}
	c := extractLipsyncChain(entity.GenerationParams{
		"__chain": true, "__chain_text": "文案", "__chain_voice_id": "v1",
	})
	if c == nil || c.Text != "文案" || c.VoiceID != "v1" {
		t.Errorf("复用参数提取错误: %+v", c)
	}
}

// TestSelectorAutoChainMarkers 开关联动：关=现状（prompt=文案，无链标记）；
// 开=链标记（文案/音色进 __chain_*，不再有画面 prompt/audio 装配）。
func TestSelectorAutoChainMarkers(t *testing.T) {
	mediaStore := &MockMediaAssetStore{materials: []entity.MediaAsset{}}
	selector := NewEndpointSelector(mediaStore, &MockTemplateRepository{})
	req := entity.UnifiedGenerationRequest{
		Text: "大家好欢迎光临",
		Params: map[string]any{
			"subjects": []any{map[string]any{"name": "主分身", "server_id": "srv-001"}},
			"voice_id": "platform-9",
		},
	}

	res, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("开关关时 subjects+文本应正常: %v", err)
	}
	if res.SubType != "reference2video" || res.Params["prompt"] != "大家好欢迎光临" {
		t.Errorf("开关关应保持现状: subType=%s prompt=%v", res.SubType, res.Params["prompt"])
	}
	if _, ok := res.Params["__chain"]; ok {
		t.Error("开关关不应装配复用标记")
	}

	selector.SetSettingRepo(&chainSettingRepo{values: map[string]string{
		entity.SettingKeyGenLipsyncAutoChain: "true",
	}})
	res, err = selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("开关开时应放行: %v", err)
	}
	if res.Params["__chain_text"] != "大家好欢迎光临" || res.Params["__chain_voice_id"] != "platform-9" {
		t.Error("文案/音色应进 __chain_* 复用标记")
	}
}

// TestSelectorAutoChainAudio subjects+音频：开关开时放行（N1 守卫升级）。
func TestSelectorAutoChainAudio(t *testing.T) {
	mediaStore := &MockMediaAssetStore{materials: []entity.MediaAsset{
		{ID: "mat-audio-001", Type: entity.MaterialTypeAudio, SourceURL: "https://example.com/v.mp3"},
	}}
	selector := NewEndpointSelector(mediaStore, &MockTemplateRepository{})
	req := entity.UnifiedGenerationRequest{
		Materials: []string{"mat-audio-001"},
		Params:    map[string]any{"subjects": []any{map[string]any{"server_id": "srv-001"}}},
	}
	if _, err := selector.Select(context.Background(), req); err == nil {
		t.Fatal("开关关时 subjects+音频应保持 N1 拒绝")
	}
	selector.SetSettingRepo(&chainSettingRepo{values: map[string]string{
		entity.SettingKeyGenLipsyncAutoChain: "true",
	}})
	res, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("开关开时 subjects+音频应放行: %v", err)
	}
	if res.Params["__chain_audio_url"] != "https://example.com/v.mp3" {
		t.Error("音频应进 __chain_audio_url")
	}
}

// TestSubmitReuseLipSync_BText 复用路径 B（文本驱动）：
// 形象视频直接为 lip_sync 输入；未显式选音色 → 分身绑定音色（23号 B 路径）。
func TestSubmitReuseLipSync_BText(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, repo)
	uc.SetSubjectAssetRepo(&chainSubjectRepo{byServer: map[string]entity.SubjectAsset{
		"srv-001": {ServerID: "srv-001", Name: "主厨分身",
			AvatarVideoURL: "https://m.example.com/avatar.mp4", VoiceID: "vc-chef-1"},
	}})
	subjects := []any{map[string]any{"name": "主厨分身", "server_id": "srv-001"}}
	task, err := uc.submitReuseLipSync(context.Background(),
		UnifiedSubmitInput{TenantID: "t1", BrandID: "b1"}, subjects, lipsyncChain{Text: "大家好"})
	if err != nil {
		t.Fatalf("复用路径提交失败: %v", err)
	}
	if task.SubType != "lip_sync" {
		t.Errorf("应直接提交 lip_sync（不生成画面任务），得到 %s", task.SubType)
	}
	if !strings.Contains(task.ParamsJSON, "avatar.mp4") {
		t.Errorf("video_url 应为分身形象视频: %s", task.ParamsJSON)
	}
	if !strings.Contains(task.ParamsJSON, "大家好") {
		t.Errorf("文案应进 lip_sync text: %s", task.ParamsJSON)
	}
	if !strings.Contains(task.ParamsJSON, "vc-chef-1") {
		t.Errorf("未显式选音色应用分身绑定音色（23号 B 路径）: %s", task.ParamsJSON)
	}
	// 只有一个任务——不存在画面生成任务
	for _, saved := range repo.saved {
		if saved.SubType == "reference2video" {
			t.Error("复用路径不应产生 reference2video 任务（只保留一条路）")
		}
	}
}

// TestSubmitReuseLipSync_CAudio 复用路径 C（音频驱动）：audio_url 优先于文本。
func TestSubmitReuseLipSync_CAudio(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, repo)
	uc.SetSubjectAssetRepo(&chainSubjectRepo{byServer: map[string]entity.SubjectAsset{
		"srv-001": {ServerID: "srv-001", AvatarVideoURL: "https://m.example.com/a.mp4"},
	}})
	task, err := uc.submitReuseLipSync(context.Background(),
		UnifiedSubmitInput{TenantID: "t1"}, []any{map[string]any{"server_id": "srv-001"}},
		lipsyncChain{Text: "备用", AudioURL: "https://m.example.com/up.mp3"})
	if err != nil {
		t.Fatalf("C 路径提交失败: %v", err)
	}
	if !strings.Contains(task.ParamsJSON, "up.mp3") {
		t.Errorf("音频驱动应携带 audio_url: %s", task.ParamsJSON)
	}
	if strings.Contains(task.ParamsJSON, "备用") {
		t.Errorf("音频驱动不应再传文本: %s", task.ParamsJSON)
	}
}

// TestSubmitReuseLipSync_AvatarMissing 形象视频缺失 → 显式报错（不回退现场生成）。
func TestSubmitReuseLipSync_AvatarMissing(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, repo)
	uc.SetSubjectAssetRepo(&chainSubjectRepo{byServer: map[string]entity.SubjectAsset{
		"srv-none": {ServerID: "srv-none", Name: "未就绪分身", AvatarVideoURL: ""},
	}})
	_, err := uc.submitReuseLipSync(context.Background(),
		UnifiedSubmitInput{TenantID: "t1"}, []any{map[string]any{"server_id": "srv-none"}},
		lipsyncChain{Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "形象视频未就绪") {
		t.Fatalf("形象视频缺失应显式报错引导回分身管理，得到: %v", err)
	}
	if len(repo.saved) > 0 {
		t.Error("缺失场景不应产生任何任务")
	}
}

// TestFirstSubjectServerID subjects 解析容错。
func TestFirstSubjectServerID(t *testing.T) {
	if id := firstSubjectServerID([]any{map[string]any{"server_id": "s1"}}); id != "s1" {
		t.Errorf("[]any(map) 解析失败: %q", id)
	}
	if id := firstSubjectServerID([]map[string]any{{"server_id": "s2"}}); id != "s2" {
		t.Errorf("[]map 解析失败: %q", id)
	}
	if id := firstSubjectServerID("garbage"); id != "" {
		t.Errorf("非法形态应返回空: %q", id)
	}
	if id := firstSubjectServerID([]any{}); id != "" {
		t.Errorf("空列表应返回空: %q", id)
	}
}

// TestIsRegistrationFailure 备胎道触发判定：仅注册类失败降级，用户错误直接报错。
func TestIsRegistrationFailure(t *testing.T) {
	if !isRegistrationFailure(fmt.Errorf("音色注册到生成服务失败，请重试")) {
		t.Error("注册失败应判定为可降级")
	}
	if isRegistrationFailure(fmt.Errorf("音色不存在")) {
		t.Error("越权错误不应降级（用户错误）")
	}
	if isRegistrationFailure(fmt.Errorf("音色已停用")) {
		t.Error("停用错误不应降级（用户错误）")
	}
}

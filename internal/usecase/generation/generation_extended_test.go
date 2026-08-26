package generation

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ====================================================================
// BE-GEN-06 回归：UnifiedSubmit 透传 Refs → translateRefs 生效
// ====================================================================

// refTrackingAdapter 记录 translateRefs 处理后的 params（验证 @引用 被翻译）。
type refTrackingAdapter struct {
	fakeSyncAdapter
	lastParams entity.GenerationParams
}

func (a *refTrackingAdapter) Validate(_ context.Context, _ entity.ModelCapability, p entity.GenerationParams) error {
	a.lastParams = p
	return nil
}

type refTrackingRegistry struct {
	fakeRegistry
	adapter *refTrackingAdapter
}

func (r refTrackingRegistry) Get(_ context.Context, _ string) (port.EndpointAdapter, error) {
	return r.adapter, nil
}
func (r refTrackingRegistry) Capability(_ context.Context, _ string, model string) (entity.ModelCapability, error) {
	return entity.ModelCapability{Model: model, MaxPromptLen: 5000, ImageSlots: 7}, nil
}

// refSelector 始终返回 text2video（UnifiedSubmit 链路测试用）。
type refSelector struct{}

func (refSelector) Select(_ context.Context, req entity.UnifiedGenerationRequest) (entity.EndpointSelectResult, error) {
	params := entity.GenerationParams{"prompt": req.Text}
	for k, v := range req.Params {
		params[k] = v
	}
	return entity.EndpointSelectResult{SubType: "text2video", Params: params}, nil
}

func TestUnifiedSubmit_PassesRefs(t *testing.T) {
	ad := &refTrackingAdapter{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		refTrackingRegistry{adapter: ad},
		&fakeRepo{},
	)
	uc.SetEndpointSelector(refSelector{})

	_, err := uc.UnifiedSubmit(context.Background(), UnifiedSubmitInput{
		TenantID: "t1",
		BrandID:  "b1",
		Text:     "用 @产品图 生成视频",
		Refs: []entity.PromptRef{
			{ID: "mat-1", Name: "产品图", URL: "https://cdn.example.com/product.jpg", Kind: entity.RefKindImage},
		},
	})
	if err != nil {
		t.Fatalf("UnifiedSubmit 失败: %v", err)
	}
	// @引用 应被翻译：prompt 中 @产品图 → 产品图；images 应包含引用 URL
	if prompt, ok := ad.lastParams["prompt"].(string); ok {
		if strings.Contains(prompt, "@产品图") {
			t.Error("@引用 标记应被 translateRefs 去除 @ 前缀")
		}
	}
	if imgs, ok := ad.lastParams["images"].([]string); ok {
		found := false
		for _, u := range imgs {
			if u == "https://cdn.example.com/product.jpg" {
				found = true
			}
		}
		if !found {
			t.Errorf("图片引用应写入 images，得到 %v", imgs)
		}
	} else {
		t.Error("images 参数缺失——translateRefs 未处理图片引用")
	}
}

func TestUnifiedSubmit_NilRefs_NoError(t *testing.T) {
	ad := &refTrackingAdapter{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		refTrackingRegistry{adapter: ad},
		&fakeRepo{},
	)
	uc.SetEndpointSelector(refSelector{})

	_, err := uc.UnifiedSubmit(context.Background(), UnifiedSubmitInput{
		TenantID: "t1",
		BrandID:  "b1",
		Text:     "纯文本无引用",
	})
	if err != nil {
		t.Fatalf("Refs=nil 不应报错: %v", err)
	}
}

// ====================================================================
// BE-GEN-01 回归：params.model 优先级
// ====================================================================

// modelCaptureProvider 记录提交时使用的模型。
type modelCaptureProvider struct {
	fakeProvider
	capturedModel string
}

func (p *modelCaptureProvider) Submit(_ context.Context, _ string, body map[string]any) (port.SubmitResult, error) {
	// model 可能在 body 顶层或嵌套在 params 中
	if m, ok := body["model"].(string); ok && m != "" {
		p.capturedModel = m
	} else if params, ok := body["params"].(map[string]any); ok {
		if m, ok := params["model"].(string); ok {
			p.capturedModel = m
		}
	}
	return port.SubmitResult{TaskID: "t-1"}, nil
}

type modelCaptureAdapter struct {
	fakeAdapter
}

func (modelCaptureAdapter) Validate(context.Context, entity.ModelCapability, entity.GenerationParams) error {
	return nil
}

type modelCaptureRegistry struct {
	fakeRegistry
}

func (modelCaptureRegistry) Get(_ context.Context, _ string) (port.EndpointAdapter, error) {
	return modelCaptureAdapter{}, nil
}

func (modelCaptureRegistry) Capability(_ context.Context, _ string, model string) (entity.ModelCapability, error) {
	return entity.ModelCapability{Model: model, MaxPromptLen: 5000, ImageSlots: 7}, nil
}

func TestSubmit_ParamsModelPriority(t *testing.T) {
	// BE-GEN-01：params.model 应优先于自动选择
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		modelCaptureRegistry{},
		repo,
	)

	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1",
		SubType:  "text2image",
		Params:   entity.GenerationParams{"prompt": "测试", "model": "viduq2"},
	})
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	// 验证 task.Model 使用了 params.model 而非自动选择
	if len(repo.saved) == 0 {
		t.Fatal("task 应落库")
	}
	last := repo.saved[len(repo.saved)-1]
	if last.Model != "viduq2" {
		t.Errorf("params.model 应优先使用，task.Model=%q", last.Model)
	}
}

// ====================================================================
// BE-GEN-02 回归：text2image 路径参考图
// ====================================================================

func TestEndpointSelector_Text2Image_WithMaterials(t *testing.T) {
	mediaStore := &MockMediaAssetStore{
		materials: []entity.MediaAsset{
			{ID: "mat-001", Type: entity.MaterialTypeImage, SourceURL: "https://cdn.example.com/ref1.jpg"},
			{ID: "mat-002", Type: entity.MaterialTypeImage, SourceURL: "https://cdn.example.com/ref2.jpg"},
		},
	}
	selector := NewEndpointSelector(mediaStore, &MockTemplateRepository{})

	req := entity.UnifiedGenerationRequest{
		BrandID:   "b1",
		Text:      "生成图片",
		Type:      "image",
		Materials: []string{"mat-001", "mat-002"},
	}
	result, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("Select 失败: %v", err)
	}
	if result.SubType != "text2image" {
		t.Errorf("subType 应为 text2image，得到 %q", result.SubType)
	}
	imgs, ok := result.Params["images"].([]string)
	if !ok || len(imgs) != 2 {
		t.Fatalf("images 应包含 2 张参考图，得到 %v", result.Params["images"])
	}
}

// ====================================================================
// mergeAdvancedParams / advancedParamAllowed 测试
// ====================================================================

func TestAdvancedParamAllowed(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"seed", true},
		{"model", true},
		{"images", true},       // BE-GEN-05
		{"subjects", true},     // BE-SUBJ-01
		{"voice_setting_speed", true},
		{"voice_setting_volume", true},
		{"voice_setting_voice_id", true},
		{"unknown_key", false},
		{"prompt", false},
		{"callback_url", false},
	}
	for _, c := range cases {
		if got := advancedParamAllowed(c.key); got != c.want {
			t.Errorf("advancedParamAllowed(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestMergeAdvancedParams(t *testing.T) {
	params := entity.GenerationParams{"prompt": "测试", "duration": 5}
	user := map[string]any{
		"seed":     42,
		"model":    "viduq2",
		"prompt":   "被忽略",
		"duration": 10,
		"nil_val":  nil,
		"hack":     "bad",
	}
	mergeAdvancedParams(params, user)

	if params["seed"] != 42 {
		t.Errorf("seed 应被合并，得到 %v", params["seed"])
	}
	if params["model"] != "viduq2" {
		t.Errorf("model 应被合并，得到 %v", params["model"])
	}
	if params["prompt"] != "测试" {
		t.Errorf("prompt 不在白名单不应被覆盖，得到 %v", params["prompt"])
	}
	if params["duration"] != 10 {
		t.Errorf("duration 应被用户值覆盖，得到 %v", params["duration"])
	}
	if _, ok := params["hack"]; ok {
		t.Error("白名单外字段不应被合并")
	}
	if _, ok := params["nil_val"]; ok {
		t.Error("nil 值不应被合并")
	}
}

func TestMergeAdvancedParams_VoiceSettingPrefix(t *testing.T) {
	params := entity.GenerationParams{}
	user := map[string]any{
		"voice_setting_speed":    1.5,
		"voice_setting_volume":   0.8,
		"voice_setting_voice_id": "female-shaonv",
	}
	mergeAdvancedParams(params, user)

	if params["voice_setting_speed"] != 1.5 {
		t.Errorf("voice_setting_speed 应被合并，得到 %v", params["voice_setting_speed"])
	}
	if params["voice_setting_voice_id"] != "female-shaonv" {
		t.Errorf("voice_setting_voice_id 应被合并，得到 %v", params["voice_setting_voice_id"])
	}
}

// ====================================================================
// isPrivateHost / toDataURI 测试
// ====================================================================

func TestIsPrivateHost(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"http://localhost:8082/media/x.png", true},
		{"http://127.0.0.1:8082/media/x.png", true},
		{"http://[::1]:8082/media/x.png", true},
		{"https://www.google.com/img.png", false},
		{"", false},
		{"://invalid", false},
	}
	for _, c := range cases {
		got := isPrivateHost(c.url)
		if got != c.want {
			t.Errorf("isPrivateHost(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestDataURI(t *testing.T) {
	data := []byte("PNGDATA")
	uri := toDataURI(data, "image/png")
	if !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Errorf("data URI 前缀错误: %s", uri[:30])
	}
	decoded, err := base64.StdEncoding.DecodeString(uri[len("data:image/png;base64,"):])
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	if string(decoded) != "PNGDATA" {
		t.Errorf("解码内容不匹配: %s", decoded)
	}
}

func TestDataURI_EmptyMime(t *testing.T) {
	uri := toDataURI([]byte("x"), "")
	if !strings.HasPrefix(uri, "data:application/octet-stream;base64,") {
		t.Errorf("空 mime 应降级为 application/octet-stream: %s", uri[:40])
	}
}

// ====================================================================
// localizePrivateMaterials 测试
// ====================================================================

func TestLocalizePrivateMaterials_LocalURL(t *testing.T) {
	rp := &recordingProvider{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": rp},
		inlineRegistry{},
		&fakeRepo{},
	)
	uc.SetAssetStore(inlineStore{})

	params := entity.GenerationParams{
		"image": "http://localhost:8082/media/t1/x.png",
	}
	err := uc.localizePrivateMaterials(context.Background(), params)
	if err != nil {
		t.Fatalf("localizePrivateMaterials 失败: %v", err)
	}
	img, _ := params["image"].(string)
	if !strings.HasPrefix(img, "data:image/png;base64,") {
		t.Errorf("本地 URL 应内联为 data URI，得到 %q", img[:40])
	}
}

func TestLocalizePrivateMaterials_PublicURL_Unchanged(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	uc.SetAssetStore(inlineStore{})

	params := entity.GenerationParams{
		"image": "https://cdn.example.com/img.png",
	}
	err := uc.localizePrivateMaterials(context.Background(), params)
	if err != nil {
		t.Fatalf("localizePrivateMaterials 失败: %v", err)
	}
	if params["image"] != "https://cdn.example.com/img.png" {
		t.Errorf("公网 URL 不应改动，得到 %q", params["image"])
	}
}

func TestLocalizePrivateMaterials_ImagesArray(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	uc.SetAssetStore(inlineStore{})

	params := entity.GenerationParams{
		"images": []string{
			"http://localhost:8082/media/t1/local.png",
			"https://cdn.example.com/public.png",
		},
	}
	err := uc.localizePrivateMaterials(context.Background(), params)
	if err != nil {
		t.Fatalf("localizePrivateMaterials 失败: %v", err)
	}
	imgs := params["images"].([]string)
	if !strings.HasPrefix(imgs[0], "data:image/png;base64,") {
		t.Errorf("本地 URL 应内联，得到 %q", imgs[0][:40])
	}
	if imgs[1] != "https://cdn.example.com/public.png" {
		t.Errorf("公网 URL 不应改动，得到 %q", imgs[1])
	}
}

func TestLocalizePrivateMaterials_NilAssetStore(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	params := entity.GenerationParams{
		"image": "http://localhost:8082/media/t1/x.png",
	}
	err := uc.localizePrivateMaterials(context.Background(), params)
	if err != nil {
		t.Fatalf("asset=nil 不应报错: %v", err)
	}
	if params["image"] != "http://localhost:8082/media/t1/x.png" {
		t.Error("asset=nil 时不应改动 params")
	}
}

// ====================================================================
// Submit 断路点测试
// ====================================================================

// failingValidateAdapter Validate 总是失败。
type failingValidateAdapter struct{ fakeAdapter }

func (failingValidateAdapter) Validate(context.Context, entity.ModelCapability, entity.GenerationParams) error {
	return fmt.Errorf("校验失败: viduq1 需要 1-7 张图片")
}

type failingValidateRegistry struct{ fakeRegistry }

func (failingValidateRegistry) Get(_ context.Context, _ string) (port.EndpointAdapter, error) {
	return failingValidateAdapter{}, nil
}

func TestSubmit_ValidateFailure(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		failingValidateRegistry{},
		&fakeRepo{},
	)
	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "text2image",
		Params: entity.GenerationParams{"prompt": "测试"},
	})
	if err == nil || !strings.Contains(err.Error(), "校验失败") {
		t.Errorf("Validate 失败应返回错误，得到 %v", err)
	}
}

// failingBuildAdapter BuildRequest 总是失败。
type failingBuildAdapter struct{ fakeAdapter }

func (failingBuildAdapter) Validate(context.Context, entity.ModelCapability, entity.GenerationParams) error {
	return nil
}
func (failingBuildAdapter) BuildRequest(context.Context, string, entity.GenerationParams, string) (map[string]any, error) {
	return nil, fmt.Errorf("参数组装失败")
}

type failingBuildRegistry struct{ fakeRegistry }

func (failingBuildRegistry) Get(_ context.Context, _ string) (port.EndpointAdapter, error) {
	return failingBuildAdapter{}, nil
}

func TestSubmit_BuildRequestFailure_TaskFailed(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		failingBuildRegistry{},
		repo,
	)
	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "text2video",
		Params: entity.GenerationParams{"prompt": "测试"},
	})
	if err == nil || !strings.Contains(err.Error(), "参数组装失败") {
		t.Errorf("BuildRequest 失败应返回错误，得到 %v", err)
	}
	if len(repo.saved) == 0 {
		t.Fatal("BuildRequest 失败时 task 应落库")
	}
	last := repo.saved[len(repo.saved)-1]
	if last.State != entity.TaskStateFailed {
		t.Errorf("task 状态应为 failed，得到 %q", last.State)
	}
}

// failingSubmitProvider provider.Submit 总是失败。
type failingSubmitProvider struct{ fakeProvider }

func (failingSubmitProvider) Submit(context.Context, string, map[string]any) (port.SubmitResult, error) {
	return port.SubmitResult{}, fmt.Errorf("Vidu API 不可达")
}

func TestSubmit_ProviderSubmitFailure_TaskFailed(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": failingSubmitProvider{}},
		inlineRegistry{},
		repo,
	)
	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "text2video",
		Params: entity.GenerationParams{"prompt": "测试"},
	})
	if err == nil || !strings.Contains(err.Error(), "Vidu API 不可达") {
		t.Errorf("provider.Submit 失败应返回错误，得到 %v", err)
	}
	if len(repo.saved) == 0 {
		t.Fatal("provider.Submit 失败时 task 应落库")
	}
	last := repo.saved[len(repo.saved)-1]
	if last.State != entity.TaskStateFailed {
		t.Errorf("task 状态应为 failed，得到 %q", last.State)
	}
}

// shortPromptRegistry 能力向量 MaxPromptLen=10（测试 prompt 超长拒绝）。
type shortPromptRegistry struct{ fakeRegistry }

func (shortPromptRegistry) Get(_ context.Context, _ string) (port.EndpointAdapter, error) {
	return inlineAdapter{}, nil
}
func (shortPromptRegistry) Capability(_ context.Context, _ string, model string) (entity.ModelCapability, error) {
	return entity.ModelCapability{Model: model, MaxPromptLen: 10}, nil
}

func TestSubmit_PromptTooLong(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		shortPromptRegistry{},
		&fakeRepo{},
	)
	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "text2video",
		Params: entity.GenerationParams{"prompt": "这是一段超过十个字符的很长提示词"},
	})
	if err == nil || !strings.Contains(err.Error(), "字符上限") {
		t.Errorf("超长 prompt 应被拒绝，得到 %v", err)
	}
}

// failingSaveRepo Save 总是失败。
type failingSaveRepo struct{ fakeRepo }

func (r *failingSaveRepo) Save(context.Context, entity.GenerationTask) error {
	return fmt.Errorf("connection refused")
}

func TestSubmit_RepoSaveFailure(t *testing.T) {
	repo := &failingSaveRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)
	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "text2video",
		Params: entity.GenerationParams{"prompt": "测试"},
	})
	if err == nil || !strings.Contains(err.Error(), "任务保存失败") {
		t.Errorf("repo.Save 失败应返回错误，得到 %v", err)
	}
}

func TestSubmit_DedupByHash(t *testing.T) {
	// 第一次提交
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)
	in := SubmitInput{
		TenantID: "t1", SubType: "text2video",
		Params: entity.GenerationParams{"prompt": "相同参数"},
	}
	task1, err := uc.Submit(context.Background(), in)
	if err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}

	// 模拟去重：替换 repo 使 FindPendingByHash 返回已有任务
	hash := paramsHash(in.SubType, "default", entity.GenerationParams{"prompt": "相同参数"})
	dedupRepo := &dedupFakeRepo{pendingHash: hash, pendingTask: task1}
	uc.repo = dedupRepo

	task2, err := uc.Submit(context.Background(), in)
	if err != nil {
		t.Fatalf("去重提交失败: %v", err)
	}
	if task2.ID != task1.ID {
		t.Errorf("相同参数应复用已有 task，得到不同 ID: %s vs %s", task1.ID, task2.ID)
	}
}

// dedupFakeRepo 支持去重查询的 repo。
type dedupFakeRepo struct {
	fakeRepo
	pendingHash string
	pendingTask entity.GenerationTask
}

func (r *dedupFakeRepo) FindPendingByHash(_ context.Context, _, hash string) ([]entity.GenerationTask, error) {
	if r.pendingHash != "" && hash == r.pendingHash {
		return []entity.GenerationTask{r.pendingTask}, nil
	}
	return nil, nil
}

// ====================================================================
// applyTemplateDefaults 测试
// ====================================================================

// fakeTemplateRepo 返回预设模板。
type fakeTemplateRepo struct {
	templates map[string]entity.GenerationTemplate
}

func (r *fakeTemplateRepo) Save(context.Context, entity.GenerationTemplate) error { return nil }
func (r *fakeTemplateRepo) FindByID(_ context.Context, id string) (entity.GenerationTemplate, error) {
	if tpl, ok := r.templates[id]; ok {
		return tpl, nil
	}
	return entity.GenerationTemplate{}, fmt.Errorf("not found: %s", id)
}
func (r *fakeTemplateRepo) ListByTenant(context.Context, string) ([]entity.GenerationTemplate, error) {
	return nil, nil
}
func (r *fakeTemplateRepo) ListAll(context.Context) ([]entity.GenerationTemplate, error) {
	var out []entity.GenerationTemplate
	for _, t := range r.templates {
		out = append(out, t)
	}
	return out, nil
}
func (r *fakeTemplateRepo) Delete(context.Context, string) error { return nil }

func TestApplyTemplateDefaults(t *testing.T) {
	repo := &fakeTemplateRepo{
		templates: map[string]entity.GenerationTemplate{
			"tpl-1": {
				ID:      "tpl-1",
				Enabled: true,
				DefaultParams: map[string]any{
					"duration":     float64(10), // JSON 反序列化为 float64
					"quality":      "high",
					"aspect_ratio": "9:16",
				},
			},
		},
	}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	uc.SetTemplateRepo(repo)

	in := UnifiedSubmitInput{
		TenantID: "t1",
		Template: "tpl-1",
	}
	uc.applyTemplateDefaults(context.Background(), &in)

	if in.Duration != 10 {
		t.Errorf("模板 duration 应填充为 10，得到 %d", in.Duration)
	}
	if in.Quality != "high" {
		t.Errorf("模板 quality 应填充为 high，得到 %q", in.Quality)
	}
	if in.AspectRatio != "9:16" {
		t.Errorf("模板 aspect_ratio 应填充为 9:16，得到 %q", in.AspectRatio)
	}
}

func TestApplyTemplateDefaults_UserOverrides(t *testing.T) {
	repo := &fakeTemplateRepo{
		templates: map[string]entity.GenerationTemplate{
			"tpl-1": {
				ID:      "tpl-1",
				Enabled: true,
				DefaultParams: map[string]any{
					"duration": float64(10),
				},
			},
		},
	}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	uc.SetTemplateRepo(repo)

	in := UnifiedSubmitInput{
		TenantID: "t1",
		Template: "tpl-1",
		Duration: 20,
	}
	uc.applyTemplateDefaults(context.Background(), &in)

	if in.Duration != 20 {
		t.Errorf("用户显式 duration 不应被模板覆盖，得到 %d", in.Duration)
	}
}

// ====================================================================
// HandleCallback 测试
// ====================================================================

// callbackRepo 支持按 payload 和 providerTaskID 查询。
type callbackRepo struct {
	fakeRepo
	tasks map[string]entity.GenerationTask
}

func (r *callbackRepo) Save(_ context.Context, t entity.GenerationTask) error {
	r.tasks[t.TenantID+"/"+t.ID] = t
	return nil
}

func (r *callbackRepo) FindByID(_ context.Context, _, id string) (entity.GenerationTask, error) {
	// HandleCallback 传空 tenantID，按 payload(id) 查
	for _, t := range r.tasks {
		if t.Payload == id || t.ID == id {
			return t, nil
		}
	}
	return entity.GenerationTask{}, fmt.Errorf("not found: %s", id)
}

func (r *callbackRepo) FindByProviderTaskID(_ context.Context, providerTaskID string) (entity.GenerationTask, error) {
	for _, t := range r.tasks {
		if t.ProviderTaskID == providerTaskID {
			return t, nil
		}
	}
	return entity.GenerationTask{}, fmt.Errorf("not found: %s", providerTaskID)
}

func TestHandleCallback_PayloadLookup(t *testing.T) {
	task := entity.GenerationTask{
		ID:             "gen-123",
		TenantID:       "t1",
		State:          entity.TaskStateProcessing,
		ProviderTaskID: "vidu-456",
		Payload:        "gen-123",
		SubType:        "text2video",
	}
	repo := &callbackRepo{tasks: map[string]entity.GenerationTask{"t1/gen-123": task}}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)

	result := port.GenerationStatus{
		State: entity.TaskStateSuccess,
		Creations: []entity.CreationItem{
			{ID: "c-1", URL: "https://cdn.example.com/video.mp4"},
		},
	}
	_, err := uc.HandleCallback(context.Background(), "gen-123", "vidu-456", result)
	if err != nil {
		t.Fatalf("HandleCallback 失败: %v", err)
	}
	saved := repo.tasks["t1/gen-123"]
	if saved.State != entity.TaskStateSuccess {
		t.Errorf("回调后状态应为 success，得到 %q", saved.State)
	}
}

func TestHandleCallback_TerminalIdempotent(t *testing.T) {
	task := entity.GenerationTask{
		ID:       "gen-123",
		TenantID: "t1",
		State:    entity.TaskStateSuccess,
		Payload:  "gen-123",
	}
	repo := &callbackRepo{tasks: map[string]entity.GenerationTask{"t1/gen-123": task}}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)

	result := port.GenerationStatus{State: entity.TaskStateFailed}
	_, err := uc.HandleCallback(context.Background(), "gen-123", "vidu-999", result)
	if err != nil {
		t.Fatalf("终态回调不应报错: %v", err)
	}
	saved := repo.tasks["t1/gen-123"]
	if saved.State != entity.TaskStateSuccess {
		t.Errorf("终态 task 不应被回调覆盖，得到 %q", saved.State)
	}
}

func TestHandleCallback_FallbackToProviderTaskID(t *testing.T) {
	task := entity.GenerationTask{
		ID:             "gen-789",
		TenantID:       "t1",
		State:          entity.TaskStateProcessing,
		ProviderTaskID: "vidu-abc",
		Payload:        "gen-789",
	}
	repo := &callbackRepo{tasks: map[string]entity.GenerationTask{"t1/gen-789": task}}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)

	// payload 不存在，回退到 providerTaskID（用 failed 状态避免"成功但无产物"校验）
	result := port.GenerationStatus{State: entity.TaskStateFailed, ErrCode: "timeout"}
	_, err := uc.HandleCallback(context.Background(), "nonexistent", "vidu-abc", result)
	if err != nil {
		t.Fatalf("回退到 providerTaskID 查找不应报错: %v", err)
	}
	saved := repo.tasks["t1/gen-789"]
	if saved.State != entity.TaskStateFailed {
		t.Errorf("回退查找成功后应更新状态，得到 %q", saved.State)
	}
}

// ====================================================================
// PollDue 正常轮询测试
// ====================================================================

// pollingProvider 支持 Poll 返回指定状态。
type pollingProvider struct {
	fakeProvider
	status port.GenerationStatus
	err    error
}

func (p *pollingProvider) Poll(context.Context, string) (port.GenerationStatus, error) {
	return p.status, p.err
}

// pollingRepo 支持 ListActive + Save。
type pollingRepo struct {
	fakeRepo
	active []entity.GenerationTask
	saved  map[string]entity.GenerationTask
}

func (r *pollingRepo) ListActive(context.Context, int) ([]entity.GenerationTask, error) {
	return r.active, nil
}

func (r *pollingRepo) Save(_ context.Context, t entity.GenerationTask) error {
	if r.saved == nil {
		r.saved = make(map[string]entity.GenerationTask)
	}
	r.saved[t.TenantID+"/"+t.ID] = t
	return nil
}

func TestPollDue_SuccessTransition(t *testing.T) {
	task := entity.GenerationTask{
		ID:             "gen-poll-1",
		TenantID:       "t1",
		State:          entity.TaskStateProcessing,
		ProviderTaskID: "vidu-poll-1",
		SubType:        "text2video",
		Provider:       "fake",
		UpdatedAt:      time.Now().Add(-5 * time.Minute),
	}
	repo := &pollingRepo{active: []entity.GenerationTask{task}}
	provider := &pollingProvider{
		status: port.GenerationStatus{
			State: entity.TaskStateSuccess,
			Creations: []entity.CreationItem{
				{ID: "c-1", URL: "https://cdn.example.com/result.mp4"},
			},
		},
	}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": provider},
		inlineRegistry{},
		repo,
	)

	count, err := uc.PollDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("PollDue 失败: %v", err)
	}
	if count != 1 {
		t.Errorf("应轮询 1 个任务，得到 %d", count)
	}
	saved := repo.saved["t1/gen-poll-1"]
	if saved.State != entity.TaskStateSuccess {
		t.Errorf("轮询后状态应为 success，得到 %q", saved.State)
	}
}

func TestPollDue_ProviderPollError_SkipTask(t *testing.T) {
	task := entity.GenerationTask{
		ID:             "gen-poll-2",
		TenantID:       "t1",
		State:          entity.TaskStateProcessing,
		ProviderTaskID: "vidu-poll-2",
		Provider:       "fake",
		UpdatedAt:      time.Now().Add(-5 * time.Minute),
	}
	repo := &pollingRepo{active: []entity.GenerationTask{task}}
	provider := &pollingProvider{
		err: fmt.Errorf("Vidu API 超时"),
	}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": provider},
		inlineRegistry{},
		repo,
	)

	count, err := uc.PollDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("PollDue 不应因单任务失败而报错: %v", err)
	}
	if count != 0 {
		t.Errorf("失败任务不应计入轮询数，得到 %d", count)
	}
}

func TestPollDue_EmptyActive(t *testing.T) {
	repo := &pollingRepo{active: nil}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)
	count, err := uc.PollDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("空队列不应报错: %v", err)
	}
	if count != 0 {
		t.Errorf("空队列应返回 0，得到 %d", count)
	}
}

// ====================================================================
// submitSubject 主体注册路径测试
// ====================================================================

func TestSubmitSubject_Basic(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)

	task, err := uc.submitSubject(context.Background(), UnifiedSubmitInput{
		TenantID: "t1",
		BrandID:  "b1",
		Text:     "品牌分身",
	})
	if err != nil {
		t.Fatalf("submitSubject 失败: %v", err)
	}
	if task.SubType != "subject" {
		t.Errorf("subType 应为 subject，得到 %q", task.SubType)
	}
	if task.State != entity.TaskStateSuccess {
		t.Errorf("同步端点应立即 success，得到 %q", task.State)
	}
}

func TestSubmitSubject_EmptyText_Error(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	_, err := uc.submitSubject(context.Background(), UnifiedSubmitInput{
		TenantID: "t1",
		Text:     "",
	})
	if err == nil || !strings.Contains(err.Error(), "主体名称") {
		t.Errorf("空 text 应报错，得到 %v", err)
	}
}

func TestSubmitSubject_WithMaterials(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	uc.SetAssetStore(&MockMediaAssetStore{
		materials: []entity.MediaAsset{
			{ID: "mat-1", Type: entity.MaterialTypeImage, SourceURL: "https://cdn.example.com/face1.jpg"},
			{ID: "mat-2", Type: entity.MaterialTypeImage, SourceURL: "https://cdn.example.com/face2.jpg"},
			{ID: "mat-3", Type: entity.MaterialTypeVideo, SourceURL: "https://cdn.example.com/vid.mp4"}, // 非图片，应忽略
		},
	})

	task, err := uc.submitSubject(context.Background(), UnifiedSubmitInput{
		TenantID:  "t1",
		BrandID:   "b1",
		Text:      "品牌分身",
		Materials: []string{"mat-1", "mat-2", "mat-3"},
	})
	if err != nil {
		t.Fatalf("submitSubject 失败: %v", err)
	}
	if task.State != entity.TaskStateSuccess {
		t.Errorf("应立即 success，得到 %q", task.State)
	}
}

func TestSubmitSubject_WithVoiceID(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)

	_, err := uc.submitSubject(context.Background(), UnifiedSubmitInput{
		TenantID: "t1",
		BrandID:  "b1",
		Text:     "品牌分身",
		Params:   map[string]any{"voice_id": "female-shaonv"},
	})
	if err != nil {
		t.Fatalf("submitSubject 失败: %v", err)
	}
	if len(repo.saved) == 0 {
		t.Fatal("task 应落库")
	}
	last := repo.saved[len(repo.saved)-1]
	// 验证 params 包含 voice_id
	if !strings.Contains(last.ParamsJSON, "female-shaonv") {
		t.Errorf("params 应包含 voice_id，得到 %s", last.ParamsJSON)
	}
}

// ====================================================================
// applyStatus 状态机直接测试
// ====================================================================

func TestApplyStatus_SuccessWithCreations(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)

	task := &entity.GenerationTask{
		ID:       "gen-as-1",
		TenantID: "t1",
		State:    entity.TaskStateProcessing,
	}
	st := port.GenerationStatus{
		State: entity.TaskStateSuccess,
		Creations: []entity.CreationItem{
			{ID: "c-1", URL: "https://cdn.example.com/result.mp4"},
		},
	}
	if err := uc.applyStatus(context.Background(), task, st); err != nil {
		t.Fatalf("applyStatus 失败: %v", err)
	}
	if task.State != entity.TaskStateSuccess {
		t.Errorf("状态应为 success，得到 %q", task.State)
	}
	if task.FinishedAt == nil {
		t.Error("终态应带 FinishedAt")
	}
	if !strings.Contains(task.CreationsJSON, "result.mp4") {
		t.Errorf("creations 应包含产物 URL: %s", task.CreationsJSON)
	}
}

func TestApplyStatus_SuccessNoCreations_Error(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	task := &entity.GenerationTask{
		ID:       "gen-as-2",
		TenantID: "t1",
		State:    entity.TaskStateProcessing,
	}
	st := port.GenerationStatus{
		State:     entity.TaskStateSuccess,
		Creations: nil, // 无产物
	}
	err := uc.applyStatus(context.Background(), task, st)
	if err == nil || !strings.Contains(err.Error(), "无生成物") {
		t.Errorf("成功但无产物应报错，得到 %v", err)
	}
}

func TestApplyStatus_FailedState(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	task := &entity.GenerationTask{
		ID:       "gen-as-3",
		TenantID: "t1",
		State:    entity.TaskStateProcessing,
		SubType:  "text2video",
	}
	st := port.GenerationStatus{
		State:   entity.TaskStateFailed,
		ErrCode: "TooManyRequests",
	}
	if err := uc.applyStatus(context.Background(), task, st); err != nil {
		t.Fatalf("applyStatus 失败: %v", err)
	}
	if task.State != entity.TaskStateFailed {
		t.Errorf("状态应为 failed，得到 %q", task.State)
	}
	if task.FinishedAt == nil {
		t.Error("终态应带 FinishedAt")
	}
	if task.ErrCode != "TooManyRequests" {
		t.Errorf("ErrCode 应保留原始值，得到 %q", task.ErrCode)
	}
}

func TestApplyStatus_IntermediateState(t *testing.T) {
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		&fakeRepo{},
	)
	task := &entity.GenerationTask{
		ID:       "gen-as-4",
		TenantID: "t1",
		State:    entity.TaskStateQueueing,
	}
	st := port.GenerationStatus{
		State: entity.TaskStateProcessing,
	}
	if err := uc.applyStatus(context.Background(), task, st); err != nil {
		t.Fatalf("applyStatus 失败: %v", err)
	}
	if task.State != entity.TaskStateProcessing {
		t.Errorf("中间态应更新，得到 %q", task.State)
	}
	if task.FinishedAt != nil {
		t.Error("中间态不应带 FinishedAt")
	}
}

func TestApplyStatus_NotifiesTerminal(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewGenerationUseCase(
		map[string]port.GenerationProvider{"fake": fakeProvider{}},
		inlineRegistry{},
		repo,
	)
	notif := &fakeNotifier{}
	uc.SetTaskNotifier(notif)

	task := &entity.GenerationTask{
		ID:       "gen-as-5",
		TenantID: "t1",
		State:    entity.TaskStateProcessing,
	}
	st := port.GenerationStatus{
		State: entity.TaskStateSuccess,
		Creations: []entity.CreationItem{
			{ID: "c-1", URL: "https://cdn.example.com/r.mp4"},
		},
	}
	if err := uc.applyStatus(context.Background(), task, st); err != nil {
		t.Fatalf("applyStatus 失败: %v", err)
	}
	if len(notif.calls) != 1 || !strings.HasSuffix(notif.calls[0], "/success") {
		t.Errorf("终态应恰好通知一次，得到 %v", notif.calls)
	}
}

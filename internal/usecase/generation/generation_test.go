package generation

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		code string
		want RetryClass
	}{
		{"TooManyRequests", RetryAuto},
		{"SystemThrottling", RetryAuto},
		{"InternalServiceFailure", RetryAuto},
		{"CreditInsufficient", RetryManual},
		{"TaskPromptPolicyViolation", RetryManual},
		{"ImageCheckFaceFailed", RetryTerminal},
		{"VideoFormatInvalid", RetryTerminal},
	}
	for _, c := range cases {
		if got := ClassifyError(c.code); got != c.want {
			t.Errorf("ClassifyError(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestCanAutoRetry(t *testing.T) {
	if !CanAutoRetry("TooManyRequests", 0) {
		t.Error("限流首次应可自动重试")
	}
	if CanAutoRetry("TooManyRequests", 3) {
		t.Error("超过 3 次不应再自动重试")
	}
	if CanAutoRetry("CreditInsufficient", 0) {
		t.Error("积分不足不应自动重试")
	}
}

func TestParamsHash(t *testing.T) {
	p1 := entity.GenerationParams{"prompt": "测试", "duration": 5, "images": []string{"a", "b"}}
	p2 := entity.GenerationParams{"images": []string{"a", "b"}, "duration": 5, "prompt": "测试"}
	h1 := paramsHash("text2video", "viduq3-pro", p1)
	h2 := paramsHash("text2video", "viduq3-pro", p2)
	if h1 != h2 {
		t.Errorf("参数顺序不同哈希应一致: %s vs %s", h1, h2)
	}
	h3 := paramsHash("text2video", "viduq2", p1)
	if h1 == h3 {
		t.Error("模型不同哈希应不同")
	}
}

func TestIsTerminal(t *testing.T) {
	if !entity.IsTerminal(entity.TaskStateSuccess) || !entity.IsTerminal(entity.TaskStateFailed) {
		t.Error("success/failed 应为终态")
	}
	if entity.IsTerminal(entity.TaskStateProcessing) {
		t.Error("processing 不应为终态")
	}
}

// ---- 同步端点（主体 API）提交即终态 ----

// fakeSyncAdapter 模拟主体类同步端点：实现 port.SyncSubmitter。
type fakeSyncAdapter struct{}

func (fakeSyncAdapter) Type() string                                           { return "subject" }
func (fakeSyncAdapter) Category() string                                      { return entity.GenerationTypeOther }
func (fakeSyncAdapter) Endpoint() string                                      { return "/ent/v2/subjects" }
func (fakeSyncAdapter) IsSync() bool                                          { return true }
func (fakeSyncAdapter) Validate(context.Context, entity.ModelCapability, entity.GenerationParams) error {
	return nil
}
func (fakeSyncAdapter) BuildRequest(_ context.Context, _ string, _ entity.GenerationParams, _ string) (map[string]any, error) {
	return map[string]any{"name": "x"}, nil
}

// fakeAdapter 普通异步端点（对照组）。
type fakeAdapter struct{ fakeSyncAdapter }

func (fakeAdapter) IsSync() bool { return false }

type fakeRegistry struct{}

func (fakeRegistry) Get(_ context.Context, subType string) (port.EndpointAdapter, error) {
	if subType == "subject" {
		return fakeSyncAdapter{}, nil
	}
	return fakeAdapter{}, nil
}
func (fakeRegistry) Types() []string { return []string{"subject"} }
func (fakeRegistry) Capability(_ context.Context, _ string, model string) (entity.ModelCapability, error) {
	if model == "" {
		model = "default"
	}
	return entity.ModelCapability{Model: model, MaxPromptLen: 5000}, nil
}
func (fakeRegistry) Models(context.Context, string) ([]string, error) { return []string{"default"}, nil }
func (fakeRegistry) AllSpecs(context.Context) []entity.GenerationSpec  { return nil }

type fakeProvider struct{}

func (fakeProvider) Name() string { return "fake" }
func (fakeProvider) Submit(context.Context, string, map[string]any) (port.SubmitResult, error) {
	// 主体 API 语义：同步返回资源 id（协议层已兼容无 task_id 的响应）
	return port.SubmitResult{TaskID: "subj-123", Credits: 2}, nil
}
func (fakeProvider) Poll(context.Context, string) (port.GenerationStatus, error) {
	// 若同步端点被错误地推进轮询，这里显式失败以暴露短路
	return port.GenerationStatus{}, fmt.Errorf("同步端点不应进入轮询")
}
func (fakeProvider) Cancel(context.Context, string) error                { return nil }
func (fakeProvider) QueryCredits(context.Context) (int, error)           { return 0, nil }
func (fakeProvider) TranslateError(string) string                        { return "" }
func (fakeProvider) VerifyCallback(context.Context, http.Header, []byte, string) error {
	return nil
}

type fakeRepo struct {
	saved []entity.GenerationTask
}

func (r *fakeRepo) Save(_ context.Context, t entity.GenerationTask) error {
	r.saved = append(r.saved, t)
	return nil
}
func (r *fakeRepo) FindByID(context.Context, string, string) (entity.GenerationTask, error) {
	return entity.GenerationTask{}, fmt.Errorf("not found")
}
func (r *fakeRepo) FindByProviderTaskID(context.Context, string) (entity.GenerationTask, error) {
	return entity.GenerationTask{}, fmt.Errorf("not found")
}
func (r *fakeRepo) FindPendingByHash(context.Context, string, string) ([]entity.GenerationTask, error) {
	return nil, nil
}
func (r *fakeRepo) List(context.Context, string, int) ([]entity.GenerationTask, error) { return nil, nil }
func (r *fakeRepo) ListActive(context.Context, int) ([]entity.GenerationTask, error)   { return nil, nil }
func (r *fakeRepo) ListFailed(context.Context, int) ([]entity.GenerationTask, error)   { return nil, nil }
func (r *fakeRepo) DeleteTerminalOlderThan(context.Context, time.Time) (int64, error)  { return 0, nil }
func (r *fakeRepo) Delete(context.Context, string, string) error                       { return nil }
func (r *fakeRepo) ListBySubType(context.Context, string, string, string, int) ([]entity.GenerationTask, error) {
	return nil, nil
}
func (r *fakeRepo) ListTransferPending(context.Context, time.Time, int) ([]entity.GenerationTask, error) {
	return nil, nil
}

func (r *fakeRepo) FindSuccessTaskByMediaURL(context.Context, string) (entity.GenerationTask, error) {
	return entity.GenerationTask{}, pkg.ErrNotFound
}

func (r *fakeRepo) ListRecentSuccessAll(context.Context, int) ([]entity.GenerationTask, error) {
	return nil, nil
}

func TestSubmitSyncEndpointImmediateSuccess(t *testing.T) {
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, &fakeRepo{})
	task, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "subject",
		Params: entity.GenerationParams{"name": "主体A", "images": []string{"a"}},
	})
	if err != nil {
		t.Fatalf("同步端点提交失败: %v", err)
	}
	if task.State != entity.TaskStateSuccess {
		t.Errorf("同步端点提交后应立即 success，得到 %q", task.State)
	}
	if task.ProviderTaskID != "subj-123" {
		t.Errorf("ProviderTaskID 应为服务商资源 id: %q", task.ProviderTaskID)
	}
	if !strings.Contains(task.CreationsJSON, "subj-123") {
		t.Errorf("creations 应携带资源 id 供前端引用: %s", task.CreationsJSON)
	}
	if task.FinishedAt == nil {
		t.Error("同步端点终态应带 FinishedAt")
	}
}

// syncAudioProvider 模拟语音合成/声音复刻类同步接口：创建响应即终态并携带产物。
type syncAudioProvider struct{ fakeProvider }

func (syncAudioProvider) Submit(context.Context, string, map[string]any) (port.SubmitResult, error) {
	return port.SubmitResult{
		TaskID: "tts-abc", Credits: 3, State: entity.TaskStateSuccess,
		Creations: []entity.CreationItem{{ID: "tts-abc", URL: "https://vidu.local/tts.mp3"}},
	}, nil
}

func TestSubmitSyncAudioImmediateResult(t *testing.T) {
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": syncAudioProvider{}}, fakeRegistry{}, &fakeRepo{})
	task, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "tts",
		Params: entity.GenerationParams{"text": "你好", "voice_setting_voice_id": "female-shaonv"},
	})
	if err != nil {
		t.Fatalf("同步音频端点提交失败: %v", err)
	}
	if task.State != entity.TaskStateSuccess {
		t.Errorf("同步接口创建响应 success 时应立即终态，得到 %q", task.State)
	}
	if !strings.Contains(task.CreationsJSON, "tts.mp3") {
		t.Errorf("创建响应产物应直接入任务: %s", task.CreationsJSON)
	}
	if task.FinishedAt == nil {
		t.Error("同步终态应带 FinishedAt")
	}
}

// fakeNotifier 记录终态通知（验证触发点与不重复）。
type fakeNotifier struct {
	calls []string
}

func (n *fakeNotifier) NotifyTaskTerminal(_ context.Context, t entity.GenerationTask) {
	n.calls = append(n.calls, t.ID+"/"+t.State)
}

func TestSubmitSyncEndpointNotifiesOnce(t *testing.T) {
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, fakeRegistry{}, &fakeRepo{})
	notif := &fakeNotifier{}
	uc.SetTaskNotifier(notif)
	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "subject",
		Params: entity.GenerationParams{"name": "主体A", "images": []string{"a"}},
	})
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if len(notif.calls) != 1 || !strings.HasSuffix(notif.calls[0], "/success") {
		t.Errorf("同步终态应恰好通知一次 success，得到 %v", notif.calls)
	}
}

// callbackAdapter 可声明回调支持的端点桩（记录收到的请求体以断言注入）。
type callbackAdapter struct {
	fakeSyncAdapter
	supports bool
	lastBody map[string]any
}

func (a *callbackAdapter) SupportsCallback() bool { return a.supports }
func (a *callbackAdapter) BuildRequest(_ context.Context, _ string, _ entity.GenerationParams, _ string) (map[string]any, error) {
	a.lastBody = map[string]any{"prompt": "x"}
	return a.lastBody, nil
}

type callbackRegistry struct {
	fakeRegistry
	cb    *callbackAdapter // 声明支持回调
	other *callbackAdapter // 不支持回调
}

func (r callbackRegistry) Get(_ context.Context, subType string) (port.EndpointAdapter, error) {
	if subType == "text2video" {
		return r.cb, nil
	}
	return r.other, nil
}

func TestCallbackURLInjection(t *testing.T) {
	cbAd := &callbackAdapter{supports: true}
	otherAd := &callbackAdapter{supports: false}
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, callbackRegistry{cb: cbAd, other: otherAd}, &fakeRepo{})
	uc.SetCallbackURL("https://pub.example.com/api/v1/generation/callback")

	// 支持回调的端点：注入
	if _, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "text2video", Params: entity.GenerationParams{"prompt": "x"},
	}); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if got := cbAd.lastBody["callback_url"]; got != "https://pub.example.com/api/v1/generation/callback" {
		t.Errorf("支持回调的端点应注入 callback_url，得到 %v", got)
	}

	// 不支持回调的端点：不注入（避免未声明参数被上游拒绝）
	if _, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "img2video", Params: entity.GenerationParams{"images": []string{"a"}},
	}); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if _, ok := otherAd.lastBody["callback_url"]; ok {
		t.Error("不支持回调的端点不应注入 callback_url")
	}

	// 未配置回调地址：支持回调的端点也不注入（纯轮询）
	cbAd2 := &callbackAdapter{supports: true}
	uc2 := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, callbackRegistry{cb: cbAd2, other: otherAd}, &fakeRepo{})
	if _, err := uc2.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "text2video", Params: entity.GenerationParams{"prompt": "x"},
	}); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if _, ok := cbAd2.lastBody["callback_url"]; ok {
		t.Error("未配置回调地址时不应注入 callback_url")
	}
}

// inlineStore 假媒体存储：只有 ReadLocal 有行为（本地 URL → 字节）。
type inlineStore struct{}

func (inlineStore) SaveFile(context.Context, string, string, string, []byte, string, string) (entity.MediaAsset, error) {
	return entity.MediaAsset{}, nil
}
func (inlineStore) List(context.Context, string, string) ([]entity.MediaAsset, error) { return nil, nil }
func (inlineStore) Delete(context.Context, string, string) error                     { return nil }
func (inlineStore) DownloadAndStore(context.Context, string, string, map[string]string) (string, error) {
	return "", nil
}
func (inlineStore) CleanupBefore(context.Context, time.Time, map[string]bool) (int, error) { return 0, nil }
func (inlineStore) ReadLocal(_ context.Context, url string) ([]byte, string, bool) {
	if strings.HasPrefix(url, "http://localhost:8082/media/") {
		return []byte("PNGDATA"), "image/png", true
	}
	return nil, "", false
}

// recordingProvider 记录提交请求体（断言内联结果）。
type recordingProvider struct {
	fakeProvider
	lastBody map[string]any
}

func (p *recordingProvider) Submit(ctx context.Context, endpoint string, body map[string]any) (port.SubmitResult, error) {
	p.lastBody = body
	return p.fakeProvider.Submit(ctx, endpoint, body)
}

// inlineAdapter 透传 images 的主体端点桩（内联测试用）。
type inlineAdapter struct{ fakeSyncAdapter }

func (inlineAdapter) BuildRequest(_ context.Context, _ string, p entity.GenerationParams, _ string) (map[string]any, error) {
	body := map[string]any{"name": p["name"]}
	if imgs, ok := p["images"].([]string); ok {
		body["images"] = imgs
	}
	return body, nil
}

type inlineRegistry struct{ fakeRegistry }

func (inlineRegistry) Get(context.Context, string) (port.EndpointAdapter, error) {
	return inlineAdapter{}, nil
}

func TestInlineLocalMedia(t *testing.T) {
	rp := &recordingProvider{}
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": rp}, inlineRegistry{}, &fakeRepo{})
	uc.SetAssetStore(inlineStore{})

	_, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "subject",
		Params: entity.GenerationParams{
			"name":   "主体A",
			"images": []string{"http://localhost:8082/media/t1/x.png", "https://cdn.example.com/keep.png"},
		},
	})
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	imgs := rp.lastBody["images"].([]string)
	// 本站 URL → base64 data URI 内联（Vidu 拉不到 localhost——同步端点创建即 400）
	if !strings.HasPrefix(imgs[0], "data:image/png;base64,") {
		t.Errorf("本地 URL 应内联为 data URI，得到 %q", imgs[0])
	}
	// 外部 URL（公网可达）→ 保持原样
	if imgs[1] != "https://cdn.example.com/keep.png" {
		t.Errorf("外部 URL 不应改动，得到 %q", imgs[1])
	}
}

var _ = context.Background

// autoPickAdapter 实现模型自选的端点桩（傻瓜式：客户端不传 model）。
type autoPickAdapter struct{ fakeSyncAdapter }

func (autoPickAdapter) PickModel([]entity.ModelCapability, entity.GenerationParams) string { return "best-model" }

type autoPickRegistry struct{ fakeRegistry }

func (autoPickRegistry) Get(_ context.Context, subType string) (port.EndpointAdapter, error) {
	if subType == "reference2video" {
		return autoPickAdapter{}, nil
	}
	return fakeSyncAdapter{}, nil
}

func TestSubmitModelAutoPick(t *testing.T) {
	uc := NewGenerationUseCase(map[string]port.GenerationProvider{"fake": fakeProvider{}}, autoPickRegistry{}, &fakeRepo{})
	task, err := uc.Submit(context.Background(), SubmitInput{
		TenantID: "t1", SubType: "reference2video", Model: "", // 傻瓜式：不传模型
		Params:   entity.GenerationParams{"prompt": "做菜"},
	})
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if task.Model != "best-model" {
		t.Errorf("自动选择的模型应落任务（防重哈希/回显一致），得到 %q", task.Model)
	}
}

var _ = context.Background

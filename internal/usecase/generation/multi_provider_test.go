package generation

import (
	"context"
	"net/http"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// MockGenerationProvider 用于测试的 mock provider。
type MockGenerationProvider struct {
	name     string
	submitFn func(ctx context.Context, endpoint string, body map[string]any) (port.SubmitResult, error)
}

func (m *MockGenerationProvider) Name() string { return m.name }

func (m *MockGenerationProvider) Submit(ctx context.Context, endpoint string, body map[string]any) (port.SubmitResult, error) {
	if m.submitFn != nil {
		return m.submitFn(ctx, endpoint, body)
	}
	return port.SubmitResult{TaskID: "mock-task-1"}, nil
}

func (m *MockGenerationProvider) Poll(ctx context.Context, taskID string) (port.GenerationStatus, error) {
	return port.GenerationStatus{State: entity.TaskStateSuccess}, nil
}

func (m *MockGenerationProvider) Cancel(ctx context.Context, taskID string) error {
	return nil
}

func (m *MockGenerationProvider) VerifyCallback(ctx context.Context, header http.Header, body []byte, requestURI string) error {
	return nil
}

func (m *MockGenerationProvider) QueryCredits(ctx context.Context) (int, error) {
	return 1000, nil
}

func (m *MockGenerationProvider) TranslateError(code string) string {
	return "mock error: " + code
}

// MockCapabilityResolver 用于测试的能力路由解析器。
type MockCapabilityResolver struct {
	caps map[string]entity.ResolvedCap
}

func (m *MockCapabilityResolver) Resolve(ctx context.Context, capID string) (entity.ResolvedCap, error) {
	if cap, ok := m.caps[capID]; ok {
		return cap, nil
	}
	return entity.ResolvedCap{}, nil
}

func TestGetProvider_VideoSubType(t *testing.T) {
	// 测试：视频端点应该路由到 vidu
	viduProvider := &MockGenerationProvider{name: "vidu"}
	mimoProvider := &MockGenerationProvider{name: "xiaomi-mimo"}

	providers := map[string]port.GenerationProvider{
		"vidu":        viduProvider,
		"xiaomi-mimo": mimoProvider,
	}

	resolver := &MockCapabilityResolver{
		caps: map[string]entity.ResolvedCap{
			"video": {VendorID: "vidu"},
			"tts":   {VendorID: "xiaomi-mimo"},
		},
	}

	uc := NewGenerationUseCase(providers, nil, nil)
	uc.SetCapabilityResolver(resolver)

	// 测试视频端点
	provider, err := uc.getProvider(context.Background(), "text2video")
	if err != nil {
		t.Fatalf("getProvider failed: %v", err)
	}
	if provider.Name() != "vidu" {
		t.Errorf("Expected provider 'vidu', got '%s'", provider.Name())
	}
}

func TestGetProvider_TTSSubType(t *testing.T) {
	// 测试：TTS端点应该路由到 xiaomi-mimo
	viduProvider := &MockGenerationProvider{name: "vidu"}
	mimoProvider := &MockGenerationProvider{name: "xiaomi-mimo"}

	providers := map[string]port.GenerationProvider{
		"vidu":        viduProvider,
		"xiaomi-mimo": mimoProvider,
	}

	resolver := &MockCapabilityResolver{
		caps: map[string]entity.ResolvedCap{
			"video": {VendorID: "vidu"},
			"tts":   {VendorID: "xiaomi-mimo"},
		},
	}

	uc := NewGenerationUseCase(providers, nil, nil)
	uc.SetCapabilityResolver(resolver)

	// 测试TTS端点
	provider, err := uc.getProvider(context.Background(), "tts")
	if err != nil {
		t.Fatalf("getProvider failed: %v", err)
	}
	if provider.Name() != "xiaomi-mimo" {
		t.Errorf("Expected provider 'xiaomi-mimo', got '%s'", provider.Name())
	}
}

func TestGetProvider_NoResolver(t *testing.T) {
	// 测试：没有resolver时使用默认provider
	viduProvider := &MockGenerationProvider{name: "vidu"}

	providers := map[string]port.GenerationProvider{
		"vidu": viduProvider,
	}

	uc := NewGenerationUseCase(providers, nil, nil)

	// 测试任意端点
	provider, err := uc.getProvider(context.Background(), "text2video")
	if err != nil {
		t.Fatalf("getProvider failed: %v", err)
	}
	if provider.Name() != "vidu" {
		t.Errorf("Expected provider 'vidu', got '%s'", provider.Name())
	}
}

func TestGetProvider_ResolverFallback(t *testing.T) {
	// 测试：resolver查询失败时使用默认provider
	viduProvider := &MockGenerationProvider{name: "vidu"}

	providers := map[string]port.GenerationProvider{
		"vidu": viduProvider,
	}

	// 空resolver，查询会返回空结果
	resolver := &MockCapabilityResolver{
		caps: map[string]entity.ResolvedCap{},
	}

	uc := NewGenerationUseCase(providers, nil, nil)
	uc.SetCapabilityResolver(resolver)

	// 测试任意端点
	provider, err := uc.getProvider(context.Background(), "text2video")
	if err != nil {
		t.Fatalf("getProvider failed: %v", err)
	}
	if provider.Name() != "vidu" {
		t.Errorf("Expected provider 'vidu', got '%s'", provider.Name())
	}
}

func TestSubTypeToCapIDMapping(t *testing.T) {
	// 测试：subType到capID的映射
	tests := []struct {
		subType string
		capID   string
	}{
		{"text2video", "video"},
		{"img2video", "video"},
		{"reference2video", "video"},
		{"tts", "tts"},
		{"voice_clone", "voice-clone"},
		{"text2image", "image"},
		{"digital_human", "digital-human"},
	}

	for _, tt := range tests {
		capID, ok := subTypeToCapID[tt.subType]
		if !ok {
			t.Errorf("subTypeToCapID[%s] not found", tt.subType)
			continue
		}
		if capID != tt.capID {
			t.Errorf("subTypeToCapID[%s] = %s, want %s", tt.subType, capID, tt.capID)
		}
	}
}

package llmconfig

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

var _ port.LLMConfigRepository = (*fakeRepo)(nil)

type fakeRepo struct {
	saved map[string]entity.LLMConfig
}

func (f *fakeRepo) Save(_ context.Context, cfg entity.LLMConfig) error {
	if f.saved == nil { f.saved = map[string]entity.LLMConfig{} }
	f.saved[cfg.Name] = cfg
	return nil
}
func (f *fakeRepo) FindByName(_ context.Context, name string) (entity.LLMConfig, error) {
	if c, ok := f.saved[name]; ok { return c, nil }
	return entity.LLMConfig{}, nil
}
func (f *fakeRepo) List(_ context.Context) ([]entity.LLMConfig, error) {
	out := make([]entity.LLMConfig, 0, len(f.saved))
	for _, c := range f.saved { out = append(out, c) }
	return out, nil
}
func (f *fakeRepo) Delete(_ context.Context, name string) error { delete(f.saved, name); return nil }
func (f *fakeRepo) FindByUsage(_ context.Context, _ string) (entity.LLMConfig, error) {
	for _, cfg := range f.saved {
		return cfg, nil
	}
	return entity.LLMConfig{}, pkg.ErrNotFound
}

// TestCreate_RejectsMissingAPIKey 验证领域校验（IsValid：name/api_key/model 非空）。
func TestCreate_RejectsMissingAPIKey(t *testing.T) {
	uc := NewLLMConfigUseCase(&fakeRepo{})
	_, err := uc.Create(context.Background(), CreateInput{
		Name: "x", APIKey: "", Model: "m",
	})
	if err == nil {
		t.Error("expected error for empty api_key, got nil")
	}
}

// TestEnsureDefault_DoesNotOverwrite 验证已存在 default 时不覆盖（用户可能在 UI 改过）。
func TestEnsureDefault_DoesNotOverwrite(t *testing.T) {
	repo := &fakeRepo{saved: map[string]entity.LLMConfig{
		"default": {Name: "default", APIKey: "user-modified-key", Model: "user-model"},
	}}
	uc := NewLLMConfigUseCase(repo)

	_ = uc.EnsureDefault(context.Background(), entity.LLMConfig{
		Name: "default", APIKey: "env-key", Model: "env-model",
	})
	if repo.saved["default"].APIKey != "user-modified-key" {
		t.Errorf("EnsureDefault 覆盖了用户配置：api_key=%q", repo.saved["default"].APIKey)
	}
}

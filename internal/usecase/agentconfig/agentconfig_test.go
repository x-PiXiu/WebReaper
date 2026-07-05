package agentconfig

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

var _ port.AgentConfigRepository = (*fakeRepo)(nil)

type fakeRepo struct {
	saved map[string]entity.AgentConfig
}

func (f *fakeRepo) Save(_ context.Context, cfg entity.AgentConfig) error {
	if f.saved == nil { f.saved = map[string]entity.AgentConfig{} }
	f.saved[cfg.Name] = cfg
	return nil
}
func (f *fakeRepo) FindByName(_ context.Context, name string) (entity.AgentConfig, error) {
	return entity.AgentConfig{}, nil
}
func (f *fakeRepo) List(_ context.Context) ([]entity.AgentConfig, error) {
	out := make([]entity.AgentConfig, 0, len(f.saved))
	for _, c := range f.saved { out = append(out, c) }
	return out, nil
}
func (f *fakeRepo) Delete(_ context.Context, name string) error { delete(f.saved, name); return nil }

// TestCreate_FillsDefaultMaxIterations 验证创建时填充默认 MaxIterations（业务默认值下沉验证）。
func TestCreate_FillsDefaultMaxIterations(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewAgentConfigUseCase(repo)

	cfg, err := uc.Create(context.Background(), CreateInput{
		Name: "a1", SystemPrompt: "hello", // MaxIterations 留空
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if cfg.MaxIterations != entity.DefaultMaxIterations {
		t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, entity.DefaultMaxIterations)
	}
}

// TestCreate_RejectsInvalid 验证空 system_prompt 会被领域规则拒绝。
func TestCreate_RejectsInvalid(t *testing.T) {
	uc := NewAgentConfigUseCase(&fakeRepo{})
	_, err := uc.Create(context.Background(), CreateInput{Name: "a1", SystemPrompt: ""})
	if err == nil {
		t.Error("expected error for empty system_prompt, got nil")
	}
}

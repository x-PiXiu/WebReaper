package agent

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

// MockPromptTemplateRepository 提示词模板仓储Mock。
type MockPromptTemplateRepository struct {
	templates map[string]entity.PromptTemplate
}

func (m *MockPromptTemplateRepository) Save(ctx context.Context, template entity.PromptTemplate) error {
	m.templates[template.Key] = template
	return nil
}

func (m *MockPromptTemplateRepository) Get(ctx context.Context, key string) (entity.PromptTemplate, error) {
	if t, ok := m.templates[key]; ok {
		return t, nil
	}
	return entity.PromptTemplate{}, nil
}

func (m *MockPromptTemplateRepository) List(ctx context.Context) ([]entity.PromptTemplate, error) {
	var result []entity.PromptTemplate
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result, nil
}

func TestPromptBuilder_Build_WithPrefix(t *testing.T) {
	// 测试：有前缀配置
	promptRepo := &MockPromptTemplateRepository{
		templates: map[string]entity.PromptTemplate{
			entity.PromptKeyAgentPrefix: {
				Key:     entity.PromptKeyAgentPrefix,
				Content: "这是前缀内容",
			},
		},
	}
	builder := NewPromptBuilder(promptRepo)

	result, err := builder.Build(context.Background(), "tenant-1", "full_access")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// 验证包含前缀内容
	if !contains(result, "这是前缀内容") {
		t.Errorf("Expected result to contain prefix content")
	}

	// 验证包含核心规则
	if !contains(result, "你是获客智能体") {
		t.Errorf("Expected result to contain core rules")
	}
}

func TestPromptBuilder_Build_WithSuffix(t *testing.T) {
	// 测试：有后缀配置
	promptRepo := &MockPromptTemplateRepository{
		templates: map[string]entity.PromptTemplate{
			entity.PromptKeyAgentSuffix: {
				Key:     entity.PromptKeyAgentSuffix,
				Content: "这是后缀内容",
			},
		},
	}
	builder := NewPromptBuilder(promptRepo)

	result, err := builder.Build(context.Background(), "tenant-1", "full_access")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// 验证包含后缀内容
	if !contains(result, "这是后缀内容") {
		t.Errorf("Expected result to contain suffix content")
	}
}

func TestPromptBuilder_Build_WithRules(t *testing.T) {
	// 测试：有补充规则配置
	promptRepo := &MockPromptTemplateRepository{
		templates: map[string]entity.PromptTemplate{
			entity.PromptKeyAgentRules: {
				Key:     entity.PromptKeyAgentRules,
				Content: "这是补充规则",
			},
		},
	}
	builder := NewPromptBuilder(promptRepo)

	result, err := builder.Build(context.Background(), "tenant-1", "full_access")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// 验证包含补充规则
	if !contains(result, "这是补充规则") {
		t.Errorf("Expected result to contain rules content")
	}
}

func TestPromptBuilder_Build_PermissionLevels(t *testing.T) {
	// 测试：不同权限级别
	promptRepo := &MockPromptTemplateRepository{
		templates: map[string]entity.PromptTemplate{},
	}
	builder := NewPromptBuilder(promptRepo)

	// 测试 full_access
	result, err := builder.Build(context.Background(), "tenant-1", "full_access")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !contains(result, "full_access") {
		t.Errorf("Expected result to contain 'full_access'")
	}

	// 测试 semi_auto
	result, err = builder.Build(context.Background(), "tenant-1", "semi_auto")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !contains(result, "semi_auto") {
		t.Errorf("Expected result to contain 'semi_auto'")
	}

	// 测试 manual_confirm
	result, err = builder.Build(context.Background(), "tenant-1", "manual_confirm")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if !contains(result, "manual_confirm") {
		t.Errorf("Expected result to contain 'manual_confirm'")
	}
}

func TestPromptBuilder_Build_NoRepo(t *testing.T) {
	// 测试：没有配置仓储
	builder := NewPromptBuilder(nil)

	result, err := builder.Build(context.Background(), "tenant-1", "full_access")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// 验证包含核心规则
	if !contains(result, "你是获客智能体") {
		t.Errorf("Expected result to contain core rules")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

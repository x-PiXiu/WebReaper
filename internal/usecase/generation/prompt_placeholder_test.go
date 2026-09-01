package generation

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

// mockSubjectAssetRepo 模拟主体资产仓储。
type mockSubjectAssetRepo struct {
	assets []entity.SubjectAsset
}

func (m *mockSubjectAssetRepo) Upsert(ctx context.Context, asset entity.SubjectAsset) error {
	return nil
}
func (m *mockSubjectAssetRepo) FindByID(ctx context.Context, id string) (entity.SubjectAsset, error) {
	return entity.SubjectAsset{}, nil
}
func (m *mockSubjectAssetRepo) FindByServerID(ctx context.Context, serverID string) (entity.SubjectAsset, error) {
	return entity.SubjectAsset{}, nil
}
func (m *mockSubjectAssetRepo) ListByTenant(ctx context.Context, tenantID, scope, kind string, limit, offset int) ([]entity.SubjectAsset, int64, error) {
	return m.assets, int64(len(m.assets)), nil
}
func (m *mockSubjectAssetRepo) UpdateAvatarVideoURL(ctx context.Context, serverID, avatarVideoURL string) error {
	return nil
}
func (m *mockSubjectAssetRepo) UpdateStatus(ctx context.Context, id, status string) error {
	return nil
}
func (m *mockSubjectAssetRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func TestPlaceholderTranslator_NoPlaceholders(t *testing.T) {
	repo := &mockSubjectAssetRepo{}
	translator := NewPlaceholderTranslator(repo)

	params := entity.GenerationParams{
		"prompt": "普通文本描述，没有占位符",
	}

	result, err := translator.Translate(context.Background(), "vidu", "text2video", params, nil, "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["prompt"] != "普通文本描述，没有占位符" {
		t.Errorf("prompt should be unchanged, got: %v", result["prompt"])
	}
	if _, ok := result["subjects"]; ok {
		t.Error("subjects should not be set")
	}
}

func TestPlaceholderTranslator_SubjectRef_Vidu(t *testing.T) {
	repo := &mockSubjectAssetRepo{
		assets: []entity.SubjectAsset{
			{Name: "我的分身", ServerID: "server-123"},
		},
	}
	translator := NewPlaceholderTranslator(repo)

	params := entity.GenerationParams{
		"prompt": "@我的分身 在厨房做菜",
	}

	result, err := translator.Translate(context.Background(), "vidu", "reference2video", params, nil, "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Vidu 模式：保留 @name 在 prompt 中
	prompt, _ := result["prompt"].(string)
	if prompt != "@我的分身 在厨房做菜" {
		t.Errorf("Vidu should keep @name in prompt, got: %s", prompt)
	}

	// 应该有 subjects 数组
	subjects, ok := result["subjects"].([]any)
	if !ok {
		t.Fatal("subjects should be set for Vidu")
	}
	if len(subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(subjects))
	}
	subj, ok := subjects[0].(map[string]any)
	if !ok {
		t.Fatal("subject should be map")
	}
	if subj["name"] != "我的分身" {
		t.Errorf("subject name should be '我的分身', got: %v", subj["name"])
	}
	if subj["server_id"] != "server-123" {
		t.Errorf("subject server_id should be 'server-123', got: %v", subj["server_id"])
	}
}

func TestPlaceholderTranslator_SubjectRef_OtherProvider(t *testing.T) {
	repo := &mockSubjectAssetRepo{
		assets: []entity.SubjectAsset{
			{Name: "我的分身", ServerID: "server-123"},
		},
	}
	translator := NewPlaceholderTranslator(repo)

	params := entity.GenerationParams{
		"prompt": "@我的分身 在厨房做菜",
	}

	result, err := translator.Translate(context.Background(), "xiaomi-mimo", "reference2video", params, nil, "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 其他厂商模式：移除 @name
	prompt, _ := result["prompt"].(string)
	if prompt != "我的分身 在厨房做菜" {
		t.Errorf("Other provider should remove @name, got: %s", prompt)
	}

	// 不应该有 subjects 数组
	if _, ok := result["subjects"]; ok {
		t.Error("subjects should not be set for non-Vidu provider")
	}
}

func TestPlaceholderTranslator_MultipleSubjects(t *testing.T) {
	repo := &mockSubjectAssetRepo{
		assets: []entity.SubjectAsset{
			{Name: "主体A", ServerID: "server-aaa"},
			{Name: "主体B", ServerID: "server-bbb"},
		},
	}
	translator := NewPlaceholderTranslator(repo)

	params := entity.GenerationParams{
		"prompt": "@主体A 和 @主体B 一起吃饭",
	}

	result, err := translator.Translate(context.Background(), "vidu", "reference2video", params, nil, "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	subjects, ok := result["subjects"].([]any)
	if !ok {
		t.Fatal("subjects should be set")
	}
	if len(subjects) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(subjects))
	}
}

func TestPlaceholderTranslator_SubjectNotFound(t *testing.T) {
	repo := &mockSubjectAssetRepo{
		assets: []entity.SubjectAsset{}, // 空的，找不到主体
	}
	translator := NewPlaceholderTranslator(repo)

	params := entity.GenerationParams{
		"prompt": "@不存在的主体 做动作",
	}

	result, err := translator.Translate(context.Background(), "vidu", "reference2video", params, nil, "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 未找到主体，不应该有 subjects 数组
	if _, ok := result["subjects"]; ok {
		t.Error("subjects should not be set when subject not found")
	}

	// prompt 应该保留原样
	prompt, _ := result["prompt"].(string)
	if prompt != "@不存在的主体 做动作" {
		t.Errorf("prompt should be unchanged when subject not found, got: %s", prompt)
	}
}

func TestPlaceholderTranslator_MaterialRef(t *testing.T) {
	repo := &mockSubjectAssetRepo{}
	translator := NewPlaceholderTranslator(repo)

	params := entity.GenerationParams{
		"prompt": "@产品图 生成视频",
	}
	refs := []entity.PromptRef{
		{ID: "img1", Name: "产品图", URL: "https://example.com/img.jpg", Kind: entity.RefKindImage},
	}

	// 使用 subject 端点（支持图片引用）
	result, err := translator.Translate(context.Background(), "vidu", "subject", params, refs, "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 素材引用应该被翻译为 images 参数
	images, ok := result["images"].([]string)
	if !ok {
		t.Fatal("images should be set")
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0] != "https://example.com/img.jpg" {
		t.Errorf("image URL mismatch: %s", images[0])
	}
}

func TestPlaceholderTranslator_MixedRefs(t *testing.T) {
	repo := &mockSubjectAssetRepo{
		assets: []entity.SubjectAsset{
			{Name: "主播", ServerID: "server-xyz"},
		},
	}
	translator := NewPlaceholderTranslator(repo)

	params := entity.GenerationParams{
		"prompt": "@主播 在 @背景图 前讲解",
	}
	refs := []entity.PromptRef{
		{ID: "img1", Name: "背景图", URL: "https://example.com/bg.jpg", Kind: entity.RefKindImage},
	}

	// 使用 subject 端点（支持图片引用）
	result, err := translator.Translate(context.Background(), "vidu", "subject", params, refs, "tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 主体引用
	subjects, ok := result["subjects"].([]any)
	if !ok {
		t.Fatal("subjects should be set")
	}
	if len(subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(subjects))
	}

	// 素材引用
	images, ok := result["images"].([]string)
	if !ok {
		t.Fatal("images should be set")
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
}

func TestParseSubjectNamesFromPrompt(t *testing.T) {
	tests := []struct {
		prompt string
		want   []string
	}{
		{"@主体A 做动作", []string{"主体A"}},
		{"@主体A 和 @主体B 互动", []string{"主体A", "主体B"}},
		{"没有占位符", nil},
		{"@name1 @name2 @name3", []string{"name1", "name2", "name3"}},
	}

	for _, tt := range tests {
		got := ParseSubjectNamesFromPrompt(tt.prompt)
		if len(got) != len(tt.want) {
			t.Errorf("ParseSubjectNamesFromPrompt(%q) = %v, want %v", tt.prompt, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ParseSubjectNamesFromPrompt(%q)[%d] = %v, want %v", tt.prompt, i, got[i], tt.want[i])
			}
		}
	}
}

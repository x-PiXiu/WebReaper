package generation

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
)

// MockMediaAssetStore 素材存储Mock。
type MockMediaAssetStore struct {
	materials []entity.MediaAsset
}

func (m *MockMediaAssetStore) SaveFile(ctx context.Context, tenantID, brandID, ownerType string, data []byte, mime, ext string) (entity.MediaAsset, error) {
	return entity.MediaAsset{}, nil
}
func (m *MockMediaAssetStore) List(ctx context.Context, tenantID, ownerType string) ([]entity.MediaAsset, error) {
	return m.materials, nil
}
func (m *MockMediaAssetStore) Delete(ctx context.Context, tenantID, assetID string) error {
	return nil
}
func (m *MockMediaAssetStore) DownloadAndStore(ctx context.Context, tenantID, sourceURL string, meta map[string]string) (string, error) {
	return "", nil
}
func (m *MockMediaAssetStore) CleanupBefore(ctx context.Context, before time.Time, excludeURLs map[string]bool) (int, error) {
	return 0, nil
}
func (m *MockMediaAssetStore) ReadLocal(ctx context.Context, url string) (data []byte, mime string, ok bool) {
	return nil, "", false
}

// MockTemplateRepository 模板仓储Mock。
type MockTemplateRepository struct{}

func (m *MockTemplateRepository) Save(ctx context.Context, template entity.GenerationTemplate) error {
	return nil
}
func (m *MockTemplateRepository) FindByID(ctx context.Context, id string) (entity.GenerationTemplate, error) {
	return entity.GenerationTemplate{}, nil
}
func (m *MockTemplateRepository) ListByTenant(ctx context.Context, tenantID string) ([]entity.GenerationTemplate, error) {
	return nil, nil
}
func (m *MockTemplateRepository) ListAll(ctx context.Context) ([]entity.GenerationTemplate, error) {
	return nil, nil
}
func (m *MockTemplateRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func TestEndpointSelector_Select_TextOnly(t *testing.T) {
	// 测试：只有文本 → text2video
	mediaStore := &MockMediaAssetStore{
		materials: []entity.MediaAsset{},
	}
	templateRepo := &MockTemplateRepository{}
	selector := NewEndpointSelector(mediaStore, templateRepo)

	req := entity.UnifiedGenerationRequest{
		BrandID: "brand-1",
		Text:    "品牌宣传视频",
	}

	result, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if result.SubType != "text2video" {
		t.Errorf("Expected subType 'text2video', got '%s'", result.SubType)
	}

	if result.Params["prompt"] != "品牌宣传视频" {
		t.Errorf("Expected prompt '品牌宣传视频', got '%v'", result.Params["prompt"])
	}
}

func TestEndpointSelector_Select_SingleImage(t *testing.T) {
	// 测试：1张图片+文本 → img2video
	mediaStore := &MockMediaAssetStore{
		materials: []entity.MediaAsset{
			{
				ID:        "mat-001",
				Type:      entity.MaterialTypeImage,
				SourceURL: "https://example.com/image1.jpg",
			},
		},
	}
	templateRepo := &MockTemplateRepository{}
	selector := NewEndpointSelector(mediaStore, templateRepo)

	req := entity.UnifiedGenerationRequest{
		BrandID:   "brand-1",
		Text:      "品牌宣传视频",
		Materials: []string{"mat-001"},
	}

	result, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if result.SubType != "img2video" {
		t.Errorf("Expected subType 'img2video', got '%s'", result.SubType)
	}

	images, ok := result.Params["images"].([]string)
	if !ok || len(images) != 1 {
		t.Errorf("Expected 1 image, got %v", result.Params["images"])
	}
}

func TestEndpointSelector_Select_TwoImages(t *testing.T) {
	// 测试：2张图片 → start_end2video
	mediaStore := &MockMediaAssetStore{
		materials: []entity.MediaAsset{
			{
				ID:        "mat-001",
				Type:      entity.MaterialTypeImage,
				SourceURL: "https://example.com/image1.jpg",
			},
			{
				ID:        "mat-002",
				Type:      entity.MaterialTypeImage,
				SourceURL: "https://example.com/image2.jpg",
			},
		},
	}
	templateRepo := &MockTemplateRepository{}
	selector := NewEndpointSelector(mediaStore, templateRepo)

	req := entity.UnifiedGenerationRequest{
		BrandID:   "brand-1",
		Text:      "品牌宣传视频",
		Materials: []string{"mat-001", "mat-002"},
	}

	result, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if result.SubType != "start_end2video" {
		t.Errorf("Expected subType 'start_end2video', got '%s'", result.SubType)
	}
}

func TestEndpointSelector_Select_ImageAndAudio(t *testing.T) {
	// 测试：1张图片+1个音频 → lip_sync（digital_human 端点已废弃 2026-08-27，
	// 图+音频统一走对口型——远端改造后本用例断言未同步，现按现行为修正）
	mediaStore := &MockMediaAssetStore{
		materials: []entity.MediaAsset{
			{
				ID:        "mat-001",
				Type:      entity.MaterialTypeImage,
				SourceURL: "https://example.com/image1.jpg",
			},
			{
				ID:        "mat-002",
				Type:      entity.MaterialTypeAudio,
				SourceURL: "https://example.com/audio1.mp3",
			},
		},
	}
	templateRepo := &MockTemplateRepository{}
	selector := NewEndpointSelector(mediaStore, templateRepo)

	req := entity.UnifiedGenerationRequest{
		BrandID:   "brand-1",
		Text:      "品牌宣传视频",
		Materials: []string{"mat-001", "mat-002"},
	}

	result, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if result.SubType != "lip_sync" {
		t.Errorf("Expected subType 'lip_sync', got '%s'", result.SubType)
	}
}

func TestEndpointSelector_Select_VideoAndAudio(t *testing.T) {
	// 测试：1个视频+1个音频 → lip_sync
	mediaStore := &MockMediaAssetStore{
		materials: []entity.MediaAsset{
			{
				ID:        "mat-001",
				Type:      entity.MaterialTypeVideo,
				SourceURL: "https://example.com/video1.mp4",
			},
			{
				ID:        "mat-002",
				Type:      entity.MaterialTypeAudio,
				SourceURL: "https://example.com/audio1.mp3",
			},
		},
	}
	templateRepo := &MockTemplateRepository{}
	selector := NewEndpointSelector(mediaStore, templateRepo)

	req := entity.UnifiedGenerationRequest{
		BrandID:   "brand-1",
		Text:      "品牌宣传视频",
		Materials: []string{"mat-001", "mat-002"},
	}

	result, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if result.SubType != "lip_sync" {
		t.Errorf("Expected subType 'lip_sync', got '%s'", result.SubType)
	}
}

func TestEndpointSelector_Select_ThreeImages(t *testing.T) {
	// 测试：3张图片 → reference2video
	mediaStore := &MockMediaAssetStore{
		materials: []entity.MediaAsset{
			{
				ID:        "mat-001",
				Type:      entity.MaterialTypeImage,
				SourceURL: "https://example.com/image1.jpg",
			},
			{
				ID:        "mat-002",
				Type:      entity.MaterialTypeImage,
				SourceURL: "https://example.com/image2.jpg",
			},
			{
				ID:        "mat-003",
				Type:      entity.MaterialTypeImage,
				SourceURL: "https://example.com/image3.jpg",
			},
		},
	}
	templateRepo := &MockTemplateRepository{}
	selector := NewEndpointSelector(mediaStore, templateRepo)

	req := entity.UnifiedGenerationRequest{
		BrandID:   "brand-1",
		Text:      "品牌宣传视频",
		Materials: []string{"mat-001", "mat-002", "mat-003"},
	}

	result, err := selector.Select(context.Background(), req)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if result.SubType != "reference2video" {
		t.Errorf("Expected subType 'reference2video', got '%s'", result.SubType)
	}
}

func TestConvertPauseMarkers(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "无标点原样返回",
			input: "你好我是vidu",
			want:  "你好我是vidu",
		},
		{
			name:  "中文逗号转0.5s停顿",
			input: "你好，我是vidu",
			want:  "你好<#0.5#>我是vidu",
		},
		{
			name:  "中文句号转1s停顿",
			input: "你好。我是vidu",
			want:  "你好<#1#>我是vidu",
		},
		{
			name:  "省略号转2s停顿",
			input: "你好。。。我是vidu",
			want:  "你好<#2#>我是vidu",
		},
		{
			name:  "换行转1.5s停顿",
			input: "你好\n我是vidu",
			want:  "你好<#1.5#>我是vidu",
		},
		{
			name:  "混合标点",
			input: "你好，我是vidu。很高兴见到你！",
			want:  "你好<#0.5#>我是vidu<#1#>很高兴见到你<#1#>",
		},
		{
			name:  "保留手写标记",
			input: "你好<#3#>我是vidu",
			want:  "你好<#3#>我是vidu",
		},
		{
			name:  "手写标记+自然标点混合",
			input: "你好<#3#>我是vidu，很高兴",
			want:  "你好<#3#>我是vidu<#0.5#>很高兴",
		},
		{
			name:  "连续逗号取一次停顿",
			input: "你好，，，我是vidu",
			want:  "你好<#0.5#>我是vidu",
		},
		{
			name:  "英文标点也处理",
			input: "Hello. I am vidu!",
			want:  "Hello<#1#> I am vidu<#1#>",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := convertPauseMarkers(c.input)
			if got != c.want {
				t.Errorf("convertPauseMarkers(%q)\n  got:  %q\n  want: %q", c.input, got, c.want)
			}
		})
	}
}

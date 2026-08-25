package account

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

func TestDefaultPersonaIsolator_Isolate(t *testing.T) {
	ctx := context.Background()

	// 创建人设仓储
	personaRepo := NewPersonaRepositoryImpl()

	// 添加测试人设
	personaRepo.Save(ctx, &entity.Persona{
		ID:        "persona_1",
		TenantID:  "tenant_1",
		Name:      "活泼美妆博主",
		Type:      entity.PersonaTypeBeauty,
		ToneStyle: entity.ToneStyleLively,
		BannedWords: []string{"最好", "第一"},
		PreferredTags: []string{"#美妆", "#护肤"},
	})

	personaRepo.Save(ctx, &entity.Persona{
		ID:        "persona_2",
		TenantID:  "tenant_1",
		Name:      "专业护肤达人",
		Type:      entity.PersonaTypeBeauty,
		ToneStyle: entity.ToneStyleProfessional,
		BannedWords: []string{},
		PreferredTags: []string{"#护肤", "#成分"},
	})

	isolator := NewDefaultPersonaIsolator(personaRepo)

	tests := []struct {
		name      string
		personaID string
		content   string
		wantContain string
		wantNotContain string
	}{
		{
			name:      "活泼风格-添加前缀",
			personaID: "persona_1",
			content:   "这款产品很好用",
			wantContain: "，这款产品很好用", // 应该有风格前缀
		},
		{
			name:      "禁用词过滤",
			personaID: "persona_1",
			content:   "这是最好的产品",
			wantNotContain: "最好",
		},
		{
			name:      "专业风格-添加前缀",
			personaID: "persona_2",
			content:   "这款产品很好用",
			wantContain: "从专业角度来看，",
		},
		{
			name:      "空人设ID-不处理",
			personaID: "",
			content:   "原始内容",
			wantContain: "原始内容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := isolator.Isolate(ctx, tt.content, tt.personaID)
			if err != nil {
				t.Errorf("Isolate() error = %v", err)
				return
			}
			if tt.wantContain != "" && !contains(result, tt.wantContain) {
				t.Errorf("Isolate() = %v, should contain %v", result, tt.wantContain)
			}
			if tt.wantNotContain != "" && contains(result, tt.wantNotContain) {
				t.Errorf("Isolate() = %v, should not contain %v", result, tt.wantNotContain)
			}
		})
	}
}

func TestDefaultPersonaIsolator_FilterBannedWords(t *testing.T) {
	personaRepo := NewPersonaRepositoryImpl()
	isolator := NewDefaultPersonaIsolator(personaRepo)

	tests := []struct {
		name        string
		content     string
		bannedWords []string
		want        string
	}{
		{
			name:        "单个禁用词",
			content:     "这是最好的产品",
			bannedWords: []string{"最好"},
			want:        "这是***的产品",
		},
		{
			name:        "多个禁用词",
			content:     "这是最好的第一名产品",
			bannedWords: []string{"最好", "第一"},
			want:        "这是***的***名产品",
		},
		{
			name:        "无禁用词",
			content:     "这是好产品",
			bannedWords: []string{},
			want:        "这是好产品",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isolator.filterBannedWords(tt.content, tt.bannedWords)
			if result != tt.want {
				t.Errorf("filterBannedWords() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestDefaultPersonaIsolator_ApplyToneStyle(t *testing.T) {
	personaRepo := NewPersonaRepositoryImpl()
	isolator := NewDefaultPersonaIsolator(personaRepo)

	tests := []struct {
		name      string
		content   string
		toneStyle string
		wantPrefix string
	}{
		{
			name:      "活泼风格",
			content:   "测试内容",
			toneStyle: entity.ToneStyleLively,
			wantPrefix: "，测试内容",
		},
		{
			name:      "专业风格",
			content:   "测试内容",
			toneStyle: entity.ToneStyleProfessional,
			wantPrefix: "从专业角度来看，测试内容",
		},
		{
			name:      "亲切风格",
			content:   "测试内容",
			toneStyle: entity.ToneStyleWarm,
			wantPrefix: "大家好，测试内容",
		},
		{
			name:      "沉稳风格",
			content:   "测试内容",
			toneStyle: entity.ToneStyleSteady,
			wantPrefix: "需要注意的是，测试内容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isolator.applyToneStyle(tt.content, tt.toneStyle)
			if !contains(result, "测试内容") {
				t.Errorf("applyToneStyle() = %v, should contain original content", result)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}

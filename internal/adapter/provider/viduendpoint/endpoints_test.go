package viduendpoint

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

func TestText2VideoValidate(t *testing.T) {
	a := text2videoAdapter{}
	ctx := context.Background()

	// viduq1 仅支持 1080p
	err := a.Validate(ctx, "viduq1", entity.GenerationParams{"prompt": "x", "resolution": "720p"})
	if err == nil {
		t.Error("viduq1 传 720p 应报错（仅 1080p）")
	}
	if err := a.Validate(ctx, "viduq1", entity.GenerationParams{"prompt": "x", "resolution": "1080p"}); err != nil {
		t.Errorf("viduq1 1080p 应通过: %v", err)
	}
	// 缺 prompt
	if err := a.Validate(ctx, "viduq3-pro", entity.GenerationParams{}); err == nil {
		t.Error("缺 prompt 应报错")
	}
	// 未注册模型
	if err := a.Validate(ctx, "viduq9", entity.GenerationParams{"prompt": "x"}); err == nil {
		t.Error("未注册模型应报错")
	}
}

func TestImg2VideoImageSlots(t *testing.T) {
	a := img2videoAdapter{}
	ctx := context.Background()
	// 图生视频恰好 1 张
	if err := a.Validate(ctx, "viduq3-pro", entity.GenerationParams{"images": []string{"a", "b"}}); err == nil {
		t.Error("图生视频 2 张图应报错（恰 1 张）")
	}
	if err := a.Validate(ctx, "viduq3-pro", entity.GenerationParams{"images": []string{"a"}}); err != nil {
		t.Errorf("图生视频 1 张应通过: %v", err)
	}
}

func TestStartEnd2VideoImageSlots(t *testing.T) {
	a := startEnd2videoAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, "viduq3-pro", entity.GenerationParams{"images": []string{"a"}}); err == nil {
		t.Error("首尾帧 1 张图应报错（恰 2 张）")
	}
	if err := a.Validate(ctx, "viduq3-pro", entity.GenerationParams{"images": []string{"a", "b"}}); err != nil {
		t.Errorf("首尾帧 2 张应通过: %v", err)
	}
}

func TestReference2VideoSubjects(t *testing.T) {
	a := reference2videoAdapter{}
	ctx := context.Background()
	// viduq1 不支持主体模式
	err := a.Validate(ctx, "viduq1", entity.GenerationParams{
		"subjects": []map[string]any{{"name": "s1", "images": []string{"a"}}},
		"prompt":   "@s1 在跳舞",
	})
	if err == nil {
		t.Error("viduq1 主体模式应报错（仅 q3/q2-pro 支持）")
	}
	// q3 主体模式通过
	if err := a.Validate(ctx, "viduq3", entity.GenerationParams{
		"subjects": []map[string]any{{"name": "s1", "images": []string{"a"}}},
		"prompt":   "@s1 在跳舞",
	}); err != nil {
		t.Errorf("viduq3 主体模式应通过: %v", err)
	}
	// 非主体模式 images 1-7
	if err := a.Validate(ctx, "viduq3", entity.GenerationParams{"images": []string{}}); err == nil {
		t.Error("images 为空应报错")
	}
}

func TestMultiframeKeyFrames(t *testing.T) {
	a := multiframeAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, "viduq2-pro", entity.GenerationParams{
		"start_image": "s",
		"image_settings": []map[string]any{{"key_image": "k1"}, {"key_image": "k2"}},
	}); err != nil {
		t.Errorf("2 个关键帧应通过: %v", err)
	}
	if err := a.Validate(ctx, "viduq2-pro", entity.GenerationParams{
		"start_image": "s",
		"image_settings": []map[string]any{{"key_image": "k1"}},
	}); err == nil {
		t.Error("1 个关键帧应报错（需 2-9）")
	}
	if err := a.Validate(ctx, "viduq2-pro", entity.GenerationParams{
		"image_settings": []map[string]any{{"key_image": "k1"}, {"key_image": "k2"}},
	}); err == nil {
		t.Error("缺 start_image 应报错")
	}
}

func TestDigitalHumanValidate(t *testing.T) {
	a := digitalHumanAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, "viduq2-turbo", entity.GenerationParams{"images": []string{"face.jpg"}}); err != nil {
		t.Errorf("数字人 1 图应通过: %v", err)
	}
	if err := a.Validate(ctx, "viduq2-turbo", entity.GenerationParams{}); err == nil {
		t.Error("数字人缺图应报错")
	}
}

func TestSubjectValidate(t *testing.T) {
	a := subjectAdapter{}
	ctx := context.Background()
	if err := a.Validate(ctx, "viduq3-pro", entity.GenerationParams{"name": "主体A", "images": []string{"a", "b", "c", "d"}}); err == nil {
		t.Error("主体图 >3 张应报错")
	}
	if err := a.Validate(ctx, "viduq3-pro", entity.GenerationParams{"name": "主体A", "images": []string{"a"}}); err != nil {
		t.Errorf("主体 1 图应通过: %v", err)
	}
}

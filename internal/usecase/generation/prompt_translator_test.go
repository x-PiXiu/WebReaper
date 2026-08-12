package generation

import (
	"reflect"
	"testing"

	"webreaper/internal/domain/entity"
)

// TestTranslateRefs 提示词翻译层：@引用（图/音频/视频）→ 各端点参数格式。
func TestTranslateRefs(t *testing.T) {
	img := entity.PromptRef{ID: "a.png", Name: "a.png", URL: "http://x/a.png", Kind: entity.RefKindImage}
	audio := entity.PromptRef{ID: "b.mp3", Name: "b.mp3", URL: "http://x/b.mp3", Kind: entity.RefKindAudio}
	video := entity.PromptRef{ID: "c.mp4", Name: "c.mp4", URL: "http://x/c.mp4", Kind: entity.RefKindVideo}

	cases := []struct {
		name    string
		subType string
		cap     entity.ModelCapability
		params  entity.GenerationParams
		refs    []entity.PromptRef
		want    entity.GenerationParams
		wantErr bool
	}{
		{
			name: "图生视频：图引用并入 images + prompt 去除 @标记",
			subType: "img2video",
			cap: entity.ModelCapability{ImageSlots: 1},
			params: entity.GenerationParams{"prompt": "把 @a.png 变成动画"},
			refs: []entity.PromptRef{img},
			want: entity.GenerationParams{
				"prompt": "把 a.png 变成动画",
				"images": []string{"http://x/a.png"},
			},
		},
		{
			name: "文生视频：图引用报错（不支持）",
			subType: "text2video",
			cap: entity.ModelCapability{ImageSlots: 0},
			params: entity.GenerationParams{"prompt": "x"},
			refs: []entity.PromptRef{img},
			wantErr: true,
		},
		{
			name: "数字人：图→image 音频→audio_url",
			subType: "digital_human",
			cap: entity.ModelCapability{ImageSlots: 1},
			params: entity.GenerationParams{"prompt": "x"},
			refs: []entity.PromptRef{img, audio},
			want: entity.GenerationParams{
				"prompt": "x",
				"image": "http://x/a.png",
				"audio_url": "http://x/b.mp3",
			},
		},
		{
			name: "数字人：用户已填 image 不覆盖",
			subType: "digital_human",
			cap: entity.ModelCapability{ImageSlots: 1},
			params: entity.GenerationParams{"image": "http://x/manual.png"},
			refs: []entity.PromptRef{img},
			want: entity.GenerationParams{"image": "http://x/manual.png"},
		},
		{
			name: "声音克隆：音频→audio_url",
			subType: "voice_clone",
			cap: entity.ModelCapability{},
			params: entity.GenerationParams{"voice_id": "v123", "text": "hi"},
			refs: []entity.PromptRef{audio},
			want: entity.GenerationParams{
				"voice_id": "v123", "text": "hi",
				"audio_url": "http://x/b.mp3",
			},
		},
		{
			name: "声音克隆：缺音频引用报错",
			subType: "voice_clone",
			cap: entity.ModelCapability{},
			params: entity.GenerationParams{"voice_id": "v123", "text": "hi"},
			refs: []entity.PromptRef{img},
			wantErr: true,
		},
		{
			name: "智能多帧：图→start_image",
			subType: "multiframe",
			cap: entity.ModelCapability{},
			params: entity.GenerationParams{},
			refs: []entity.PromptRef{img},
			want: entity.GenerationParams{"start_image": "http://x/a.png"},
		},
		{
			name: "参考生视频 q2-pro：视频→videos 图→images",
			subType: "reference2video",
			cap: entity.ModelCapability{VideoSlots: 1, ImageSlots: -1},
			params: entity.GenerationParams{"prompt": "x"},
			refs: []entity.PromptRef{video, img},
			want: entity.GenerationParams{
				"prompt": "x",
				"videos": []string{"http://x/c.mp4"},
				"images": []string{"http://x/a.png"},
			},
		},
		{
			name: "无引用：原样返回",
			subType: "text2video",
			cap: entity.ModelCapability{},
			params: entity.GenerationParams{"prompt": "x"},
			refs: nil,
			want: entity.GenerationParams{"prompt": "x"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := translateRefs(c.subType, c.cap, c.params, c.refs)
			if c.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际通过: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应报错: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("翻译结果不符\n got: %#v\nwant: %#v", got, c.want)
			}
		})
	}
}

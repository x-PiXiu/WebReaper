package viduendpoint

import (
	"errors"

	"webreaper/internal/domain/entity"
)

// 端点校验错误（产品级消息——前端可直接展示）。
var (
	errPromptRequired      = errors.New("提示词 prompt 必填")
	errImagesRange         = errors.New("图片 images 需 1-7 张")
	errSubjectsUnsupported = errors.New("该模型不支持主体模式（subjects）")
	errStartImageRequired  = errors.New("首帧图 start_image 必填")
	errKeyFramesRange      = errors.New("关键帧 image_settings 需 2-9 个")
	errDigitalHumanImageRequired = errors.New("数字人图片 image 必填（1 张）")
	errPromptTooLong       = errors.New("提示词超过 2000 字符上限")
	errSubjectNameRequired = errors.New("主体名称 name 必填")
	errSubjectImagesRange  = errors.New("主体图片需 1-3 张")
)

// getSubjects 提取 subjects 数组（reference2video 主体模式）。
func getSubjects(p entity.GenerationParams) []map[string]any {
	if v, ok := p["subjects"].([]map[string]any); ok {
		return v
	}
	if v, ok := p["subjects"].([]any); ok {
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// getKeyFrames 提取 image_settings（multiframe 关键帧数组）。
func getKeyFrames(p entity.GenerationParams) []map[string]any {
	if v, ok := p["image_settings"].([]map[string]any); ok {
		return v
	}
	if v, ok := p["image_settings"].([]any); ok {
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// ---- 能力向量表（数据源：Docs/第三方/Vidu/创建视频任务/Vidu端点完整参数限制.md）----

var text2videoCaps = []entity.ModelCapability{
	{Model: "viduq3-pro", Family: "q3", Durations: [2]int{1, 16}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "3:4", "4:3", "1:1"}, AudioDefault: true, AudioTypes: []string{"all", "speech_only", "sound_effect_only"}, MaxPromptLen: 5000},
	{Model: "viduq3-turbo", Family: "q3", Durations: [2]int{1, 16}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "3:4", "4:3", "1:1"}, AudioDefault: true, AudioTypes: []string{"all", "speech_only", "sound_effect_only"}, MaxPromptLen: 5000},
	{Model: "viduq2", Family: "q2", Durations: [2]int{1, 10}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "3:4", "4:3", "1:1"}, AudioDefault: false, SupportsBGM: true, MaxPromptLen: 5000},
	{Model: "viduq1", Family: "q1", Durations: [2]int{5, 5}, Resolutions: []string{"1080p"},
		AspectRatios: []string{"16:9", "9:16", "1:1"}, AudioDefault: false, SupportsMovement: true, MaxPromptLen: 5000},
}

var img2videoCaps = []entity.ModelCapability{
	{Model: "viduq3-pro", Family: "q3", Durations: [2]int{1, 16}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: true, ImageSlots: 1, MaxPromptLen: 5000},
	{Model: "viduq3-turbo", Family: "q3", Durations: [2]int{1, 16}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: true, ImageSlots: 1, MaxPromptLen: 5000},
	{Model: "viduq3-pro-fast", Family: "q3", Durations: [2]int{1, 16}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: true, ImageSlots: 1, MaxPromptLen: 5000},
	{Model: "viduq2-pro", Family: "q2", Durations: [2]int{1, 10}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 1, MaxPromptLen: 5000},
	{Model: "viduq2-pro-fast", Family: "q2", Durations: [2]int{1, 10}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 1, MaxPromptLen: 5000},
	{Model: "viduq2-turbo", Family: "q2", Durations: [2]int{1, 10}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 1, MaxPromptLen: 5000},
	{Model: "viduq1", Family: "q1", Durations: [2]int{5, 5}, Resolutions: []string{"1080p"}, AudioDefault: false, ImageSlots: 1, SupportsMovement: true},
	{Model: "viduq1-classic", Family: "q1", Durations: [2]int{5, 5}, Resolutions: []string{"1080p"}, AudioDefault: false, ImageSlots: 1, SupportsMovement: true},
	{Model: "vidu2.0", Family: "vidu2.0", Durations: [2]int{4, 8}, Resolutions: []string{"360p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 1, SupportsMovement: true},
}

var startEnd2videoCaps = []entity.ModelCapability{
	{Model: "viduq3-pro", Family: "q3", Durations: [2]int{1, 16}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: true, ImageSlots: 2, MaxPromptLen: 5000},
	{Model: "viduq3-turbo", Family: "q3", Durations: [2]int{1, 16}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: true, ImageSlots: 2, MaxPromptLen: 5000},
	{Model: "viduq2-pro", Family: "q2", Durations: [2]int{1, 8}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 2, MaxPromptLen: 5000},
	{Model: "viduq2-pro-fast", Family: "q2", Durations: [2]int{1, 8}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 2, MaxPromptLen: 5000},
	{Model: "viduq2-turbo", Family: "q2", Durations: [2]int{1, 8}, Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 2, MaxPromptLen: 5000},
	{Model: "viduq1", Family: "q1", Durations: [2]int{5, 5}, Resolutions: []string{"1080p"}, AudioDefault: false, ImageSlots: 2, SupportsMovement: true},
	{Model: "viduq1-classic", Family: "q1", Durations: [2]int{5, 5}, Resolutions: []string{"1080p"}, AudioDefault: false, ImageSlots: 2, SupportsMovement: true},
	{Model: "vidu2.0", Family: "vidu2.0", Durations: [2]int{4, 8}, Resolutions: []string{"360p", "720p", "1080p"}, AudioDefault: false, ImageSlots: 2, SupportsMovement: true},
}

var reference2videoCaps = []entity.ModelCapability{
	{Model: "viduq3-turbo", Family: "q3", Durations: [2]int{3, 16}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, AudioDefault: true, SupportsSubjects: true, ImageSlots: -1, MaxPromptLen: 2000},
	{Model: "viduq3", Family: "q3", Durations: [2]int{3, 16}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, AudioDefault: true, SupportsSubjects: true, ImageSlots: -1, MaxPromptLen: 2000},
	{Model: "viduq3-mix", Family: "q3", Durations: [2]int{3, 16}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, AudioDefault: false, SupportsSubjects: true, ImageSlots: -1, MaxPromptLen: 2000},
	{Model: "viduq2-pro", Family: "q2", Durations: [2]int{0, 10}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, AudioDefault: false, SupportsSubjects: true, ImageSlots: -1, VideoSlots: 2, MaxPromptLen: 2000},
	{Model: "viduq2", Family: "q2", Durations: [2]int{1, 10}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, AudioDefault: false, ImageSlots: -1, MaxPromptLen: 2000},
	{Model: "viduq1", Family: "q1", Durations: [2]int{5, 5}, Resolutions: []string{"1080p"},
		AspectRatios: []string{"16:9", "9:16", "1:1"}, AudioDefault: false, ImageSlots: -1, SupportsMovement: true, MaxPromptLen: 2000},
	{Model: "vidu2.0", Family: "vidu2.0", Durations: [2]int{4, 4}, Resolutions: []string{"360p", "720p"},
		AspectRatios: []string{"16:9", "9:16", "1:1"}, AudioDefault: false, ImageSlots: -1, SupportsMovement: true, MaxPromptLen: 2000},
}

var multiframeCaps = []entity.ModelCapability{
	{Model: "viduq2-pro", Family: "q2", Durations: [2]int{2, 7}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, ImageSlots: -1}, // start_image + image_settings
	{Model: "viduq2-turbo", Family: "q2", Durations: [2]int{2, 7}, Resolutions: []string{"540p", "720p", "1080p"},
		AspectRatios: []string{"16:9", "9:16", "4:3", "3:4", "1:1"}, ImageSlots: -1},
}

var digitalHumanCaps = []entity.ModelCapability{
	{Model: "viduq2-turbo", Family: "q2", Resolutions: []string{"540p", "720p", "1080p"}, ImageSlots: 1, MaxPromptLen: 2000},
	{Model: "viduq2-pro", Family: "q2", Resolutions: []string{"540p", "720p", "1080p"}, ImageSlots: 1, MaxPromptLen: 2000},
}

// init 注册能力向量（registry 注册端点后由 Registry 持有）。
func init() {
	// 能力表由 NewRegistry 显式注册（保持包内无全局状态依赖顺序问题）。
}

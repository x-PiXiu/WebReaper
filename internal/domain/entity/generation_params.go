// generation_params.go 高频端点强类型参数（27号优化——替代 map[string]any）。
//
// 设计动机：
//   - GenerationParams map[string]any 类型安全缺失，运行时才发现字段拼写错误
//   - 高频端点（text2video/tts/digital_human/subject）定义强类型 struct
//   - 保留 map[string]any 作为兜底（低频端点/透传参数）
//   - 强类型 params 可序列化为 JSON 存入 params_json 字段
package entity

import "encoding/json"

// ---- 通用参数 ----

// CommonParams 所有端点共享的基础参数。
type CommonParams struct {
	Prompt   string `json:"prompt,omitempty"`    // 文本提示词
	Duration int    `json:"duration,omitempty"`  // 时长（秒）
	Model    string `json:"model,omitempty"`     // 模型名（空=自动选择）
	Watermark bool  `json:"watermark,omitempty"` // 是否加水印
	OffPeak  bool   `json:"off_peak,omitempty"`  // 错峰模式
	Payload  string `json:"payload,omitempty"`   // 透传关联键
	Seed     int    `json:"seed,omitempty"`      // 随机种子
}

// ---- 视频生成参数 ----

// Text2VideoParams 文生视频参数。
type Text2VideoParams struct {
	CommonParams
	AspectRatio string   `json:"aspect_ratio,omitempty"` // 画面比例（16:9/9:16/1:1）
	Resolution  string   `json:"resolution,omitempty"`   // 分辨率（1080p/720p）
	AudioType   string   `json:"audio_type,omitempty"`   // 音频类型
	BGM         string   `json:"bgm,omitempty"`          // 背景音乐URL
	Images      []string `json:"images,omitempty"`       // 参考图（可选）
}

// Img2VideoParams 图生视频参数。
type Img2VideoParams struct {
	CommonParams
	Images      []string `json:"images"`                 // 首帧图片（必填，1张）
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
}

// StartEnd2VideoParams 首尾帧视频参数。
type StartEnd2VideoParams struct {
	CommonParams
	Images      []string `json:"images"` // 首尾帧图片（必填，2张）
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
}

// Reference2VideoParams 参考生视频参数。
type Reference2VideoParams struct {
	CommonParams
	Images      []string            `json:"images,omitempty"`   // 参考图（1-7张）
	Videos      []string            `json:"videos,omitempty"`   // 参考视频（q2-pro）
	Subjects    []SubjectRef        `json:"subjects,omitempty"` // 主体引用
	AspectRatio string              `json:"aspect_ratio,omitempty"`
	Resolution  string              `json:"resolution,omitempty"`
	Movement    string              `json:"movement_amplitude,omitempty"` // 运动幅度
}

// SubjectRef 主体引用（reference2video 的 subjects 数组元素）。
type SubjectRef struct {
	Name     string   `json:"name"`               // 主体名称
	ServerID string   `json:"server_id,omitempty"` // 已注册主体的ID
	Images   []string `json:"images,omitempty"`    // 主体图片（创建时用）
}

// ---- 图片生成参数 ----

// Text2ImageParams 文生图参数。
type Text2ImageParams struct {
	CommonParams
	Images     []string `json:"images,omitempty"`     // 参考图（0-7张）
	Resolution string   `json:"resolution,omitempty"` // 分辨率
	Style      string   `json:"style,omitempty"`      // 风格
}

// ---- 音频生成参数 ----

// TTSParams 语音合成参数。
type TTSParams struct {
	Text      string `json:"text"`                          // 合成文本（必填）
	VoiceID   string `json:"voice_setting_voice_id"`        // 音色ID（必填）
	Speed     int    `json:"voice_setting_speed,omitempty"` // 语速（0-100）
	Volume    int    `json:"voice_setting_volume,omitempty"`// 音量（0-100）
	Emotion   string `json:"voice_setting_emotion,omitempty"` // 情感
}

// VoiceCloneParams 声音克隆参数。
type VoiceCloneParams struct {
	VoiceID  string `json:"voice_id"`          // 音色ID（必填，唯一标识）
	AudioURL string `json:"audio_url"`         // 音频样本URL（必填）
	Text     string `json:"text,omitempty"`    // 合成文本（可选，用于生成试听）
}

// ---- 数字人参数 ----

// DigitalHumanParams 数字人口播参数。
type DigitalHumanParams struct {
	CommonParams
	Image    string `json:"image"`              // 人物图片（必填）
	AudioURL string `json:"audio_url"`          // 音频URL（必填，音频驱动模式）
	Text     string `json:"text,omitempty"`     // 文本（可选，表情控制）
	VoiceID  string `json:"voice_id,omitempty"` // 音色ID（可选，文本驱动模式）
}

// ---- 主体参数 ----

// SubjectParams 主体创建参数。
type SubjectParams struct {
	Name      string   `json:"name"`                    // 主体名称（必填）
	Images    []string `json:"images"`                  // 参考图（必填，1-7张）
	VoiceID   string   `json:"voice_id,omitempty"`      // 绑定音色
	Kind      string   `json:"kind,omitempty"`          // 类型（person/scene）
	SceneDesc string   `json:"scene_description,omitempty"` // 场景描述（环境主体用）
	SceneImage string  `json:"scene_image,omitempty"`       // 场景图（环境主体用）
}

// ---- 对口型参数 ----

// LipSyncParams 对口型参数。
type LipSyncParams struct {
	VideoURL string `json:"video_url"`           // 视频URL（必填）
	AudioURL string `json:"audio_url,omitempty"` // 音频URL（音频驱动模式）
	Text     string `json:"text,omitempty"`      // 文本（文本驱动模式）
	VoiceID  string `json:"voice_id,omitempty"`  // 音色ID（文本驱动模式）
	RefPhoto string `json:"ref_photo_url,omitempty"` // 参考照片（多脸指定）
}

// ---- 参数转换工具 ----

// ToMap 将强类型参数转换为 map[string]any（兼容现有接口）。
func ToMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// ParamsFromMap 将 map[string]any 反序列化为强类型参数。
func ParamsFromMap(m map[string]any, v any) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

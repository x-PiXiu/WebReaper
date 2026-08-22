package integration

import "webreaper/internal/domain/entity"

// DefaultVendors 默认厂商种子数据（首次启动 seed 进 integration_vendors 表）。
var DefaultVendors = []entity.IntegrationVendor{
	{
		ID: "xiaomi-mimo", Name: "小米 MiMo", Protocol: "openai-chat",
		BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", // token-plan 专属端点（OpenAI 兼容）
		Enabled: false, // 需管理员配置 Key 后启用
	},
	{
		ID: "siliconflow", Name: "硅基流动", Protocol: "openai",
		BaseURL: "https://api.siliconflow.cn/v1",
		Enabled: false,
	},
	{
		ID: "openai", Name: "OpenAI", Protocol: "openai",
		BaseURL: "https://api.openai.com/v1",
		Enabled: false,
	},
}

// DefaultCapabilities 默认能力路由种子数据（首次启动 seed 进 integration_capabilities 表）。
// IsDefault: 硅基流动 ASR 默认（免费推荐）；小米 MiMo 的能力默认关闭。
var DefaultCapabilities = []entity.IntegrationCapability{
	// ASR：硅基流动 SenseVoice（推荐，免费）
	{ID: "asr#siliconflow", CapID: "asr", VendorID: "siliconflow",
		Endpoint: "/audio/transcriptions", Model: "FunAudioLLM/SenseVoiceSmall",
		IsDefault: true, Enabled: true},
	// ASR：小米 MiMo（备选——openai-chat 协议，JSON+base64）
	{ID: "asr#xiaomi-mimo", CapID: "asr", VendorID: "xiaomi-mimo",
		Endpoint: "/chat/completions", Model: "mimo-v2.5-asr",
		IsDefault: false, Enabled: false,
		ExtraJSON: `{"response_style":"chat","asr_options_language":"auto"}`},
	// LLM：小米 MiMo
	{ID: "llm-chat#xiaomi-mimo", CapID: "llm-chat", VendorID: "xiaomi-mimo",
		Endpoint: "/chat/completions", Model: "mimo-v2.5-pro",
		IsDefault: false, Enabled: false},
	// TTS：小米 MiMo 标准（9 种预置音色，同步返回 base64 音频）
	{ID: "tts#xiaomi-mimo", CapID: "tts", VendorID: "xiaomi-mimo",
		Endpoint: "/chat/completions", Model: "mimo-v2.5-tts",
		IsDefault: false, Enabled: false,
		ExtraJSON: `{"audio_format":"wav"}`},
	// 音色设计：小米 MiMo（自然语言描述风格→音频，无需预置/克隆）
	{ID: "tts-design#xiaomi-mimo", CapID: "tts-design", VendorID: "xiaomi-mimo",
		Endpoint: "/chat/completions", Model: "mimo-v2.5-tts-voicedesign",
		IsDefault: false, Enabled: false,
		ExtraJSON: `{"audio_format":"wav"}`},
	// 声音克隆：小米 MiMo（音频样本 base64 → 克隆音色音频，同步返回）
	{ID: "voice-clone#xiaomi-mimo", CapID: "voice-clone", VendorID: "xiaomi-mimo",
		Endpoint: "/chat/completions", Model: "mimo-v2.5-tts-voiceclone",
		IsDefault: false, Enabled: false,
		ExtraJSON: `{"audio_format":"wav"}`},
}

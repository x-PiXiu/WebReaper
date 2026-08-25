package port

import "context"

// ---- 视频文案提取管线（08 计划 D4：参考视频 → 说话内容 → 口播文案）----
//
// 流程：resolve（分享链→直链）→ download（下载入库）→ extract（字幕轨/音轨）
//       → transcribe（ASR）→ rewrite（LLM 清洗/改写双产出——复用 AIGenerator）。
// 端口按可替换技术件划分：平台解析（RPA/接口）、ffmpeg（本地二进制）、
// ASR（OpenAI 兼容云服务，动态配置）。

// VideoLinkResolver 分享链/网页链 → 视频直链（下载源）。
// 实现：douyinweb（RPA 详情接口）等平台适配器。
type VideoLinkResolver interface {
	// SupportedPlatforms 支持的平台标识（douyin/kuaishou…）。
	SupportedPlatforms() []string
	// Resolve 解析分享链/网页链：返回可直接下载的视频直链 + 标题 + 平台。
	// 不支持的链接返回错误（调用方提示改用直接上传）。
	// localPath 非空 = 解析器已在自身上下文完成下载（如快手 chromedp：CDN pkey
	// 签名绑定浏览器会话，URL 离开上下文即失效）——调用方跳过 safeDownload 直接用。
	Resolve(ctx context.Context, tenantID, rawURL string) (videoURL, title, platform, localPath string, err error)
}

// MediaAVTool 本地媒体处理（ffmpeg/ffprobe 封装——字幕探测与音轨剥离）。
//
// 设计（08 计划 D4）：优先抽软字幕轨（免费且 100% 精确——抖音/快手几乎全是
// 硬字幕走不通，B 站等有软字幕来源可白嫖）；无字幕轨则抽音轨（16kHz 单声道
// mp3——绕开 ASR 接口 25MB 上限）。二进制缺失时 Available=false，用例层降级
// （≤25MB 视频整文件直传 ASR）。
type MediaAVTool interface {
	// Available ffmpeg 是否可用（false=走降级路径）。
	Available() bool
	// ExtractSubtitle 探测并抽取软字幕轨；ok=false 表示无字幕轨（走音轨路径）。
	ExtractSubtitle(ctx context.Context, mediaPath string) (text string, ok bool, err error)
	// ExtractAudio 抽音轨为 16kHz 单声道 mp3（ASR 输入格式），返回文件路径。
	ExtractAudio(ctx context.Context, mediaPath string) (audioPath string, err error)
	// SegmentAudio 把音频按 segSeconds 切段（16kHz 单声道 mp3），返回段文件路径列表。
	// 长音频分段转写用（半小时+ 音视频超出 ASR 上限的解决方案）。
	SegmentAudio(ctx context.Context, audioPath string, segSeconds int) ([]string, error)
}

// SpeechTranscriber 语音识别（音频 → 文本）。
// 实现：OpenAI 兼容 /audio/transcriptions（硅基流动 SenseVoice 等），动态配置。
type SpeechTranscriber interface {
	// Transcribe 识别音频文件（mp3/wav/m4a…）。fileSize 用于 provider 上限预检。
	Transcribe(ctx context.Context, audioPath string, mime string, fileSize int64) (text string, err error)
}

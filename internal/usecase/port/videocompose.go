// videocompose 口播视频画面插入（B-Roll）端口的接口与数据结构（22 号计划）。
//
// 整洁架构定位：本文件由【用例层】声明并拥有；adapter/mediaav 实现 ComposeInsertSegments
// /DetectSpeechSegments（ffmpeg 细节），usecase/videocompose 实现 Composer（业务编排）。
package port

import "context"

// InsertSegmentSpec 插入片段合成指令（adapter 层消费——路径均为本地已下载文件）。
type InsertSegmentSpec struct {
	StartMs   int64  // 窗口起点（毫秒——已按 §4 配对规则换算，含静音中点语义）
	EndMs     int64  // 窗口终点（毫秒）
	MediaPath string // 片段本地路径（视频或图片——图片走 -loop 1 输入形态）
}

// SpeechSegment 静音检测产出的语音段。
type SpeechSegment struct {
	StartMs int64
	EndMs   int64
}

// MediaAVTool 的扩展方法（追加到 video_transcript.go 的接口声明处，此处仅文档说明）：
//   ComposeInsertSegments(ctx, mainVideoPath, segs, outPath) error
//   DetectSpeechSegments(ctx, mediaPath) ([]SpeechSegment, error)
//   ProbeHasVideoStream(ctx, mediaPath) (bool, error)

// Composer B-Roll 合成编排接口（generation.UnifiedSubmit 的 compose 类型经此分发；
// 实现：usecase/videocompose.UseCase——main 装配时注入）。
type Composer interface {
	// SubmitCompose 提交合成：校验（§5.3）→ 创建 compose 任务（异步执行）→ 返回任务。
	// sourceTask 须为已有时间轴的成片任务；timeline 缺失返回可读错误引导先定位。
	SubmitCompose(ctx context.Context, in ComposeInput) (ComposeResult, error)

	// LocateTimeline 台词时间轴定位（按需触发）：静音检测 + 台词配对 / ASR 自动分行。
	// force=true 忽略缓存重跑；linesOverride 非空时仅修正各行文字（时间窗不动）。
	LocateTimeline(ctx context.Context, tenantID, taskID string, force bool, linesOverride []TimelineLineOverride) ([]TimelineLine, string, error)

	// GetTimeline 读取已定位时间轴（未定位返回错误）。
	GetTimeline(ctx context.Context, tenantID, taskID string) ([]TimelineLine, string, error)
}

// ComposeInput 合成提交输入（统一 submit 的 compose 类型参数）。
type ComposeInput struct {
	TenantID     string
	BrandID      string
	SourceTaskID string          // 源成片任务 ID
	Segments     []ComposeSegment // 客户端只传句号与素材 URL
}

// ComposeSegment 客户端提交的片段指令（时间窗由后端从 timeline 换算）。
type ComposeSegment struct {
	SentenceIndex int    `json:"sentence_index"`
	MediaURL      string `json:"media_url"`
}

// ComposeResult 提交结果。
type ComposeResult struct {
	TaskID string
	State  string
}

// TimelineLine 台词时间轴行（22 计划 §5.2——持久化到 generation_tasks.timeline_json）。
type TimelineLine struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
}

// TimelineLineOverride 文字修正（只改 text 不改时间窗）。
type TimelineLineOverride struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// 时间轴来源与配对方式标记（timeline_json 顶层元信息）。
const (
	TimelineScriptSourceParams = "params" // 台词来自任务 params.script（A/B 音频路径）
	TimelineScriptSourceASR    = "asr"    // 台词来自语音识别自动分行（C 上传音频路径）
	TimelineAlignDirect       = "direct"  // 段句数一致直接配对
	TimelineAlignEstimated    = "estimated" // 比例合并/拆分（精度降级标记）
)

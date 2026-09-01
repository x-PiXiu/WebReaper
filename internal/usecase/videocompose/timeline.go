// Package videocompose 口播视频画面插入（B-Roll）业务编排（22 号计划阶段二/三）。
//
// 职责：
//   - timeline.go：台词时间轴定位（静音检测 + 台词配对 / C 路径 ASR 自动分行）
//   - compose.go：合成任务编排（校验/SSRF 下载/并发闸门/链式继承/异步执行）
//
// 整洁架构：依赖 port 接口（GenerationTaskRepo/MediaAVTool/SpeechTranscriber），
// 实现 port.Composer——generation.UseCase 经该接口分发 compose 类型（main 装配注入）。
package videocompose

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// UseCase B-Roll 编排用例（实现 port.Composer）。
type UseCase struct {
	tasks       GenerationTaskRepo // 任务读写（窄接口——只用需要的方法）
	av          port.MediaAVTool
	transcriber port.SpeechTranscriber
	store       CreationUploader     // 产物上传（可选——main 装配注入）
	assets      port.MediaAssetStore // 本站素材直读（可选——本站 URL 免走 SSRF 拒绝的 HTTP 自下载）
}

// SetAssetStore 注入素材存储：stored_url/上传素材是本站地址（localhost:8082/media/...），
// safeDownloadMedia 的 SSRF 防护会拒绝环回地址——本站 URL 改走 ReadLocal 本地文件直读。
func (uc *UseCase) SetAssetStore(s port.MediaAssetStore) {
	uc.assets = s
}

// GenerationTaskRepo 任务仓储窄接口（依赖倒置——videocompose 不感知完整仓储）。
type GenerationTaskRepo interface {
	Save(ctx context.Context, t entity.GenerationTask) error
	FindByID(ctx context.Context, tenantID, id string) (entity.GenerationTask, error)
}

var _ port.Composer = (*UseCase)(nil)

// NewUseCase 创建。
func NewUseCase(tasks GenerationTaskRepo, av port.MediaAVTool, transcriber port.SpeechTranscriber) *UseCase {
	return &UseCase{tasks: tasks, av: av, transcriber: transcriber}
}

// SetStore 注入产物上传（可选；未注入时合成产物无法入库——提交时报明确错误）。
func (uc *UseCase) SetStore(store CreationUploader) { uc.store = store }

// uploadCreation 产物上传（store 未注入报明确错误）。
func (uc *UseCase) uploadCreation(ctx context.Context, tenantID, path string) (string, error) {
	if uc.store == nil {
		return "", fmt.Errorf("媒体存储未配置——请联系管理员")
	}
	return uc.store.Upload(ctx, tenantID, path, "video")
}

// timelineMeta timeline_json 的持久化结构（行 + 元信息）。
type timelineMeta struct {
	ScriptSource string              `json:"script_source"` // params | asr
	AlignMode    string              `json:"align_mode"`    // direct | estimated
	Lines        []port.TimelineLine `json:"lines"`
}

// GetTimeline 读取已定位时间轴（未定位返回可读错误）。
func (uc *UseCase) GetTimeline(ctx context.Context, tenantID, taskID string) ([]port.TimelineLine, string, error) {
	task, err := uc.tasks.FindByID(ctx, tenantID, taskID)
	if err != nil {
		return nil, "", fmt.Errorf("任务不存在")
	}
	meta, ok := parseTimeline(task.TimelineJSON)
	if !ok {
		return nil, "", fmt.Errorf("该成片尚未定位台词时间轴——请先调用 POST timeline 定位")
	}
	return meta.Lines, meta.ScriptSource, nil
}

// LocateTimeline 定位（按需触发，结果随任务缓存）。
//
// 流程：成片下载/读本地 → 抽音轨 → 静音检测（三级阈值）→ 台词行来源分支：
//   - params.script 非空（A/B 路径）→ 段/行配对（M==N 直配 / M>N 合并 / M<N 拆分）
//   - params.script 空（C 上传音频）→ ASR 全文 → 按段时长比例切行（行=段直配）
func (uc *UseCase) LocateTimeline(ctx context.Context, tenantID, taskID string, force bool, linesOverride []port.TimelineLineOverride) ([]port.TimelineLine, string, error) {
	task, err := uc.tasks.FindByID(ctx, tenantID, taskID)
	if err != nil {
		return nil, "", fmt.Errorf("任务不存在")
	}

	// 文字修正模式：只改各行 text（时间窗不动——22 计划 §5.4①）
	if len(linesOverride) > 0 {
		meta, ok := parseTimeline(task.TimelineJSON)
		if !ok {
			return nil, "", fmt.Errorf("尚未定位，无法修正文字——请先定位")
		}
		for _, o := range linesOverride {
			if o.Index < 0 || o.Index >= len(meta.Lines) {
				return nil, "", fmt.Errorf("修正行号越界: %d", o.Index)
			}
			meta.Lines[o.Index].Text = o.Text
		}
		if err := uc.saveTimeline(ctx, task, meta); err != nil {
			return nil, "", err
		}
		return meta.Lines, meta.ScriptSource, nil
	}

	// 缓存命中（非 force）
	if meta, ok := parseTimeline(task.TimelineJSON); ok && !force {
		return meta.Lines, meta.ScriptSource, nil
	}

	// 成片本地路径（产物 URL → 下载由 compose.go 的安全下载；定位阶段直接用产物地址
	// 抽流——ffmpeg 支持 http 输入，但 SSRF 防护下走本地：先安全下载）
	mediaPath, cleanup, err := uc.safeDownloadTaskMedia(ctx, task)
	if err != nil {
		return nil, "", fmt.Errorf("成片获取失败: %w", err)
	}
	defer cleanup()

	// 静音检测（三级阈值在 adapter 内）
	segs, err := uc.av.DetectSpeechSegments(ctx, mediaPath)
	if err != nil {
		return nil, "", fmt.Errorf("静音检测失败: %w", err)
	}
	if len(segs) == 0 {
		return nil, "", fmt.Errorf("未检测到语音内容（成片无音频或全程静音）")
	}

	script := taskScript(task)
	var meta timelineMeta
	if strings.TrimSpace(script) != "" {
		// A/B 路径：台词行与语音段配对
		lines := splitLines(script)
		meta = alignSegments(segs, lines)
	} else {
		// C 路径：ASR 全文 → 按段时长比例切行
		if uc.transcriber == nil {
			return nil, "", fmt.Errorf("语音识别未配置——上传音频类成片无法自动定位台词")
		}
		audioPath, aErr := uc.av.ExtractAudio(ctx, mediaPath)
		if aErr != nil {
			return nil, "", fmt.Errorf("抽音轨失败: %w", aErr)
		}
		text, tErr := uc.transcriber.Transcribe(ctx, audioPath, "audio/mpeg", 0)
		if tErr != nil {
			return nil, "", fmt.Errorf("语音识别失败: %w", tErr)
		}
		meta = splitBySegments(segs, text)
	}

	if err := uc.saveTimeline(ctx, task, meta); err != nil {
		return nil, "", err
	}
	return meta.Lines, meta.ScriptSource, nil
}

// saveTimeline 序列化并回写任务。
func (uc *UseCase) saveTimeline(ctx context.Context, task entity.GenerationTask, meta timelineMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	task.TimelineJSON = string(b)
	return uc.tasks.Save(ctx, task)
}

// parseTimeline 反序列化（空/损坏视为未定位）。
func parseTimeline(s string) (timelineMeta, bool) {
	var meta timelineMeta
	if strings.TrimSpace(s) == "" {
		return meta, false
	}
	if err := json.Unmarshal([]byte(s), &meta); err != nil || len(meta.Lines) == 0 {
		return meta, false
	}
	return meta, true
}

// taskScript 从任务参数取台词（lipsync 提交时 params.script——A/B 路径携带）。
func taskScript(t entity.GenerationTask) string {
	var p map[string]any
	if err := json.Unmarshal([]byte(t.ParamsJSON), &p); err != nil {
		return ""
	}
	if s, ok := p["script"].(string); ok {
		return s
	}
	return ""
}

// splitLines 台词按行切（一行一句——与前端 transcriptLines 同规则）。
func splitLines(script string) []string {
	var out []string
	for _, l := range strings.Split(script, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// alignSegments 语音段 × 台词行配对（22 计划 §4-2，含窗口边界静音中点语义）。
//
// 返回 meta：M==N direct / 其他 estimated。
func alignSegments(segs []port.SpeechSegment, lines []string) timelineMeta {
	meta := timelineMeta{}
	var raw []port.TimelineLine
	switch {
	case len(segs) == len(lines): // 直配
		meta.AlignMode = port.TimelineAlignDirect
		for i, seg := range segs {
			raw = append(raw, port.TimelineLine{Index: i, Text: lines[i], StartMs: seg.StartMs, EndMs: seg.EndMs})
		}
	case len(segs) > len(lines): // 句中停顿被切开 → 按字符比例合并相邻段
		meta.AlignMode = port.TimelineAlignEstimated
		raw = mergeSegments(segs, lines)
	default: // 相邻句连读 → 按字符比例拆分段
		meta.AlignMode = port.TimelineAlignEstimated
		raw = splitSegments(segs, lines)
	}
	meta.Lines = applyBoundarySemantics(segs, raw)
	return meta
}

// mergeSegments M>N：按各行字符数占比把 M 个段聚成 N 组。
func mergeSegments(segs []port.SpeechSegment, lines []string) []port.TimelineLine {
	totalChars := 0
	for _, l := range lines {
		totalChars += utf8.RuneCountInString(l)
	}
	var out []port.TimelineLine
	segIdx := 0
	for i, l := range lines {
		share := utf8.RuneCountInString(l)
		start := segs[segIdx].StartMs
		// 该行应占的段数 = 剩余段的按字符比例取整（末行兜底取剩余全部）
		var want int
		if i == len(lines)-1 {
			want = len(segs) - segIdx
		} else {
			want = int(float64(len(segs)) * float64(share) / float64(totalChars))
			if want < 1 {
				want = 1
			}
			if segIdx+want > len(segs) {
				want = len(segs) - segIdx
			}
		}
		end := segs[segIdx+want-1].EndMs
		out = append(out, port.TimelineLine{Index: i, Text: l, StartMs: start, EndMs: end})
		segIdx += want
	}
	return out
}

// splitSegments M<N：按各行字符数占比把段时间切开。
func splitSegments(segs []port.SpeechSegment, lines []string) []port.TimelineLine {
	total := segs[len(segs)-1].EndMs - segs[0].StartMs
	totalChars := 0
	for _, l := range lines {
		totalChars += utf8.RuneCountInString(l)
	}
	var out []port.TimelineLine
	cursor := segs[0].StartMs
	for i := range lines {
		start := cursor
		var end int64
		if i == len(lines)-1 {
			end = segs[len(segs)-1].EndMs
		} else {
			// 累进字符占比切点（行首=前累计占比、行尾=含本行累计占比）
			acc := 0
			for _, c := range lines[:i+1] {
				acc += utf8.RuneCountInString(c)
			}
			end = segs[0].StartMs + int64(float64(total)*float64(acc)/float64(totalChars))
			cursor = end
		}
		out = append(out, port.TimelineLine{Index: i, Text: lines[i], StartMs: start, EndMs: end})
	}
	return out
}

// splitBySegments C 路径 ASR 分行：全文按段时长比例切行（行=段天然一一对应）。
func splitBySegments(segs []port.SpeechSegment, fullText string) timelineMeta {
	meta := timelineMeta{ScriptSource: port.TimelineScriptSourceASR}
	text := strings.TrimSpace(fullText)
	if text == "" {
		meta.Lines = nil
		return meta
	}
	runes := []rune(text)
	totalMs := segs[len(segs)-1].EndMs - segs[0].StartMs
	cursor := 0
	for i, seg := range segs {
		var end int
		if i == len(segs)-1 {
			end = len(runes)
		} else {
			dur := seg.EndMs - seg.StartMs
			end = cursor + int(float64(len(runes))*float64(dur)/float64(totalMs))
			if end <= cursor {
				end = cursor + 1
			}
			if end > len(runes) {
				end = len(runes)
			}
		}
		meta.Lines = append(meta.Lines, port.TimelineLine{
			Index: i, Text: strings.TrimSpace(string(runes[cursor:end])),
			StartMs: seg.StartMs, EndMs: seg.EndMs,
		})
		cursor = end
	}
	return meta
}

// applyBoundarySemantics 窗口边界落点（22 计划 §4-2）：
// 起点=上一静音段中点（首行 start-150ms clamp≥0）；终点=本句语音结束（不动）。
// 静音中点用上一行 EndMs 与本行 StartMs 的中点近似（段边界即静音边界）。
func applyBoundarySemantics(segs []port.SpeechSegment, lines []port.TimelineLine) []port.TimelineLine {
	out := make([]port.TimelineLine, len(lines))
	copy(out, lines)
	for i := range out {
		if i > 0 && lines[i-1].EndMs <= out[i].StartMs {
			// 静音中点（含无缝拼接时=Start 不变——不引入额外前移）
			out[i].StartMs = (lines[i-1].EndMs + out[i].StartMs) / 2
		} else if out[i].StartMs > 150 {
			out[i].StartMs -= 150 // 首行：前置余量（clamp≥0）
		} else {
			out[i].StartMs = 0
		}
	}
	// 防御：确保前移不越过上一行终点（无缝时中点=Start 已保≥prev End）
	for i := 1; i < len(out); i++ {
		if out[i].StartMs < lines[i-1].EndMs {
			out[i].StartMs = lines[i-1].EndMs
		}
	}
	return out
}

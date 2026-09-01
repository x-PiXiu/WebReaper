// generation_tools.go 生成任务管理 Agent 工具（27 号优化——补全智能体能力）。
//
// 工具清单：
//   - list_generation_tasks：查询生成任务列表（含状态/类型/产物）
//   - cancel_generation_task：取消未终态的生成任务
//   - locate_timeline：定位台词时间轴（B-Roll 插入前置步骤）
//   - insert_broll：插入 B-Roll 画面片段
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// ---- list_generation_tasks 查询生成任务列表 ----

type ListGenerationTasksTool struct {
	uc *generation.GenerationUseCase
}

func NewListGenerationTasksTool(uc *generation.GenerationUseCase) *ListGenerationTasksTool {
	return &ListGenerationTasksTool{uc: uc}
}

func (t *ListGenerationTasksTool) Name() string { return "list_generation_tasks" }
func (t *ListGenerationTasksTool) Description() string {
	return "查询当前商户的生成任务列表（含状态、类型、产物URL）。" +
		"用户问「我的视频生成好了吗」「查看生成记录」「有哪些任务」时调用。"
}

func (t *ListGenerationTasksTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        t.Name(),
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"limit": {Type: "number", Description: "返回数量上限（默认 10，最大 50）"},
		},
	}
}

func (t *ListGenerationTasksTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if args.Limit <= 0 || args.Limit > 50 {
		args.Limit = 10
	}

	tasks, err := t.uc.List(ctx, tenantID, args.Limit)
	if err != nil {
		return entity.DataItem{}, err
	}

	var sb strings.Builder
	for i, task := range tasks {
		stateLabel := task.State
		switch task.State {
		case entity.TaskStateSuccess:
			stateLabel = "✅ 已完成"
		case entity.TaskStateFailed:
			stateLabel = "❌ 失败"
		case entity.TaskStateProcessing:
			stateLabel = "⏳ 处理中"
		case entity.TaskStateQueueing:
			stateLabel = "⏳ 排队中"
		case entity.TaskStateCancelled:
			stateLabel = "🚫 已取消"
		}
		fmt.Fprintf(&sb, "%d. [%s] %s | %s | %s\n",
			i+1, stateLabel, task.SubType, task.Model,
			task.CreatedAt.Format("01-02 15:04"))
		if task.ErrMsg != "" {
			fmt.Fprintf(&sb, "   错误: %s\n", task.ErrMsg)
		}
		// 产物 URL
		var creations []map[string]any
		if json.Unmarshal([]byte(task.CreationsJSON), &creations) == nil && len(creations) > 0 {
			if u, ok := creations[0]["stored_url"].(string); ok && u != "" {
				fmt.Fprintf(&sb, "   产物: %s\n", u)
			} else if u, ok := creations[0]["url"].(string); ok && u != "" {
				fmt.Fprintf(&sb, "   产物: %s\n", u)
			}
		}
		fmt.Fprintf(&sb, "   ID: %s\n", task.ID)
	}

	if sb.Len() == 0 {
		sb.WriteString("暂无生成任务记录。")
	}
	return textItem("gen-tasks", "生成任务列表", sb.String()), nil
}

// ---- cancel_generation_task 取消生成任务 ----

type CancelGenerationTaskTool struct {
	uc *generation.GenerationUseCase
}

func NewCancelGenerationTaskTool(uc *generation.GenerationUseCase) *CancelGenerationTaskTool {
	return &CancelGenerationTaskTool{uc: uc}
}

func (t *CancelGenerationTaskTool) Name() string { return "cancel_generation_task" }
func (t *CancelGenerationTaskTool) Description() string {
	return "取消一个正在排队或处理中的生成任务。" +
		"用户说「取消那个任务」「停止生成」时调用。需要提供任务 ID。"
}

func (t *CancelGenerationTaskTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        t.Name(),
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"task_id": {Type: "string", Description: "要取消的任务 ID"},
		},
		Required: []string{"task_id"},
	}
}

func (t *CancelGenerationTaskTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.TaskID == "" {
		return entity.DataItem{}, fmt.Errorf("缺少 task_id 参数")
	}

	if err := t.uc.Cancel(ctx, tenantID, args.TaskID); err != nil {
		return entity.DataItem{}, err
	}
	return textItem("cancel-ok", "任务已取消", fmt.Sprintf("任务 %s 已成功取消。", args.TaskID)), nil
}

// ---- locate_timeline 定位台词时间轴 ----

type LocateTimelineTool struct {
	composer port.Composer
}

func NewLocateTimelineTool(composer port.Composer) *LocateTimelineTool {
	return &LocateTimelineTool{composer: composer}
}

func (t *LocateTimelineTool) Name() string { return "locate_timeline" }
func (t *LocateTimelineTool) Description() string {
	return "为已生成的口播视频定位台词时间轴（静音检测 + 台词配对）。" +
		"这是 B-Roll 画面插入的前置步骤。用户说「定位时间轴」「分析台词」「准备插入画面」时调用。"
}

func (t *LocateTimelineTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        t.Name(),
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"task_id": {Type: "string", Description: "成片任务 ID（必须是已成功的生成任务）"},
			"force":   {Type: "boolean", Description: "是否强制重新定位（默认 false，已有缓存直接返回）"},
		},
		Required: []string{"task_id"},
	}
}

func (t *LocateTimelineTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args struct {
		TaskID string `json:"task_id"`
		Force  bool   `json:"force"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.TaskID == "" {
		return entity.DataItem{}, fmt.Errorf("缺少 task_id 参数")
	}

	lines, source, err := t.composer.LocateTimeline(ctx, tenantID, args.TaskID, args.Force, nil)
	if err != nil {
		return entity.DataItem{}, err
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "台词时间轴定位完成（来源: %s，共 %d 行）：\n", source, len(lines))
	for _, line := range lines {
		startSec := float64(line.StartMs) / 1000
		endSec := float64(line.EndMs) / 1000
		fmt.Fprintf(&sb, "  [%05.1fs-%05.1fs] 第%d句: %s\n", startSec, endSec, line.Index+1, line.Text)
	}
	sb.WriteString("\n现在可以调用 insert_broll 在指定句子处插入 B-Roll 画面。")
	return textItem("timeline-ok", "时间轴定位完成", sb.String()), nil
}

// ---- insert_broll 插入 B-Roll 画面 ----

type InsertBRollTool struct {
	composer port.Composer
}

func NewInsertBRollTool(composer port.Composer) *InsertBRollTool {
	return &InsertBRollTool{composer: composer}
}

func (t *InsertBRollTool) Name() string { return "insert_broll" }
func (t *InsertBRollTool) Description() string {
	return "在口播视频的指定句子处插入 B-Roll 画面片段。" +
		"需要先调用 locate_timeline 定位时间轴。用户说「在第3句插入画面」「添加 B-Roll」时调用。"
}

func (t *InsertBRollTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        t.Name(),
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"source_task_id": {Type: "string", Description: "源成片任务 ID（已定位时间轴的任务）"},
			"segments":       {Type: "string", Description: "插入片段 JSON 数组，每项含 sentence_index（句号，从0开始）和 media_url（画面URL）"},
		},
		Required: []string{"source_task_id", "segments"},
	}
}

func (t *InsertBRollTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args struct {
		SourceTaskID string `json:"source_task_id"`
		Segments     string `json:"segments"` // JSON 数组字符串
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.SourceTaskID == "" {
		return entity.DataItem{}, fmt.Errorf("缺少 source_task_id 参数")
	}
	if args.Segments == "" {
		return entity.DataItem{}, fmt.Errorf("缺少 segments 参数（JSON 数组）")
	}

	// 解析 segments
	var segs []struct {
		SentenceIndex int    `json:"sentence_index"`
		MediaURL      string `json:"media_url"`
	}
	if err := json.Unmarshal([]byte(args.Segments), &segs); err != nil {
		return entity.DataItem{}, fmt.Errorf("segments 格式错误（需 JSON 数组）: %w", err)
	}
	if len(segs) == 0 {
		return entity.DataItem{}, fmt.Errorf("segments 不能为空")
	}

	composeSegs := make([]port.ComposeSegment, len(segs))
	for i, s := range segs {
		composeSegs[i] = port.ComposeSegment{
			SentenceIndex: s.SentenceIndex,
			MediaURL:      s.MediaURL,
		}
	}

	res, err := t.composer.SubmitCompose(ctx, port.ComposeInput{
		TenantID:     tenantID,
		SourceTaskID: args.SourceTaskID,
		Segments:     composeSegs,
	})
	if err != nil {
		return entity.DataItem{}, err
	}

	return textItem("broll-ok", "B-Roll 合成已提交",
		fmt.Sprintf("合成任务已创建（ID: %s，状态: %s）。合成在后台执行，完成后产物会自动入库。"+
			"可通过 list_generation_tasks 查看进度。", res.TaskID, res.State)), nil
}

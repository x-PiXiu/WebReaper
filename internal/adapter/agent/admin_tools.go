// admin_tools.go 管理后台专属 Agent 工具（Admin LLM Tools——管理员在 AI 对话中
// 用自然语言管理系统）。
//
// 与 merchant_tools.go 同模式：实现 port.Tool 接口，由 router 按角色注册。
// 仅 role=admin 的会话可调用——跨租户操作、系统诊断、资产统计。
//
// 工具清单：
//   - admin_system_health      → 系统健康总览（任务/积分/资产/配置）
//   - admin_list_failed_tasks  → 最近失败的生成任务（跨租户诊断）
//   - admin_cancel_task        → 取消任意租户的卡住任务
//   - admin_list_platform_voices → 平台音色列表（含默认标记）
//   - admin_list_official_subjects → 官方主体列表
//   - admin_vidu_credits       → Vidu 积分余额查询
//   - admin_tenant_usage       → 某租户本月生成用量
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// ---- admin_system_health 系统健康总览 ----

type AdminSystemHealthTool struct {
	genUC       *generation.GenerationUseCase
	voiceRepo   port.VoiceLibrary
	subjectRepo port.SubjectAssetRepository
	provider    port.GenerationProvider
}

func NewAdminSystemHealthTool(
	genUC *generation.GenerationUseCase,
	voices port.VoiceLibrary,
	subjects port.SubjectAssetRepository,
	provider port.GenerationProvider,
) *AdminSystemHealthTool {
	return &AdminSystemHealthTool{genUC: genUC, voiceRepo: voices, subjectRepo: subjects, provider: provider}
}

func (t *AdminSystemHealthTool) Name() string { return "admin_system_health" }
func (t *AdminSystemHealthTool) Description() string {
	return "获取系统整体健康状态：活跃生成任务数、排队积压、最近失败数、Vidu积分余额、" +
		"平台音色/主体数量。管理员问「系统怎么样」「现在有多少任务在跑」「积分还剩多少」时调用。"
}
func (t *AdminSystemHealthTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: t.Name(), Description: t.Description()}
}
func (t *AdminSystemHealthTool) Execute(ctx context.Context, _ string) (entity.DataItem, error) {
	var parts []string

	// 任务
	if t.genUC != nil {
		active, _ := t.genUC.ListActiveAll(ctx, 100)
		failed, _ := t.genUC.ListRecentFailed(ctx, 20)
		queueing := 0
		for _, task := range active {
			if task.State == entity.TaskStateQueueing || task.State == entity.TaskStateCreated {
				queueing++
			}
		}
		parts = append(parts, fmt.Sprintf("生成任务：活跃 %d（排队 %d）| 最近失败 %d", len(active), queueing, len(failed)))
	}

	// Vidu
	if t.provider != nil {
		credits, err := t.provider.QueryCredits(ctx)
		if err != nil {
			parts = append(parts, fmt.Sprintf("Vidu：积分查询失败(%v)", err))
		} else {
			parts = append(parts, fmt.Sprintf("Vidu：积分余额 %d", credits))
		}
	}

	// 资产
	if t.voiceRepo != nil {
		pv, _ := t.voiceRepo.ListForAdmin(ctx, "platform")
		def, _ := t.voiceRepo.GetDefault(ctx)
		defName := "未设置"
		if def.VoiceID != "" {
			defName = def.Name
		}
		parts = append(parts, fmt.Sprintf("平台音色：%d 个（默认：%s）", len(pv), defName))
	}
	if t.subjectRepo != nil {
		subs, _, _ := t.subjectRepo.ListByTenant(ctx, "", "official", "", 100, 0)
		parts = append(parts, fmt.Sprintf("官方主体：%d 个", len(subs)))
	}

	return entity.DataItem{Content: strings.Join(parts, "\n")}, nil
}

// ---- admin_list_failed_tasks 最近失败任务 ----

type AdminListFailedTasksTool struct {
	genUC *generation.GenerationUseCase
}

func NewAdminListFailedTasksTool(genUC *generation.GenerationUseCase) *AdminListFailedTasksTool {
	return &AdminListFailedTasksTool{genUC: genUC}
}

func (t *AdminListFailedTasksTool) Name() string { return "admin_list_failed_tasks" }
func (t *AdminListFailedTasksTool) Description() string {
	return "列出最近失败的生成任务（跨租户），含错误原因。管理员问「最近哪些任务失败了」「有什么报错」时调用。"
}
func (t *AdminListFailedTasksTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        t.Name(),
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"limit": {Type: "number", Description: "返回数量（默认 10，最大 30）"},
		},
	}
}
func (t *AdminListFailedTasksTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args struct{ Limit int }
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if args.Limit <= 0 || args.Limit > 30 {
		args.Limit = 10
	}
	tasks, err := t.genUC.ListRecentFailed(ctx, args.Limit)
	if err != nil {
		return entity.DataItem{}, err
	}
	if len(tasks) == 0 {
		return entity.DataItem{Content: "没有失败的生成任务 ✅"}, nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("最近 %d 个失败任务：\n", len(tasks)))
	for _, task := range tasks {
		sb.WriteString(fmt.Sprintf("- [%s] %s | %s | 错误: %s\n",
			task.CreatedAt.Format("01-02 15:04"), task.ID[:20], task.SubType,
			truncateStr(task.ErrMsg, 80)))
	}
	return entity.DataItem{Content: sb.String()}, nil
}

// ---- admin_cancel_task 取消任务 ----

type AdminCancelTaskTool struct {
	genUC *generation.GenerationUseCase
}

func NewAdminCancelTaskTool(genUC *generation.GenerationUseCase) *AdminCancelTaskTool {
	return &AdminCancelTaskTool{genUC: genUC}
}
func (t *AdminCancelTaskTool) Name() string { return "admin_cancel_task" }
func (t *AdminCancelTaskTool) Description() string {
	return "取消一个未终态的生成任务（跨租户——管理员干预卡住的任务）。需要任务 ID。"
}
func (t *AdminCancelTaskTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        t.Name(),
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"task_id": {Type: "string", Description: "任务 ID（gen-xxx 格式）"},
		},
	}
}
func (t *AdminCancelTaskTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args struct{ TaskID string }
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.TaskID == "" {
		return entity.DataItem{}, fmt.Errorf("缺少 task_id 参数")
	}
	// Cancel：空 tenantID = 管理端跨租户取消（repo 层空值不校验归属）
	if err := t.genUC.Cancel(ctx, "", args.TaskID); err != nil {
		return entity.DataItem{}, fmt.Errorf("取消失败: %w", err)
	}
	return entity.DataItem{Content: fmt.Sprintf("任务 %s 已取消", args.TaskID)}, nil
}

// ---- admin_list_platform_voices 平台音色列表 ----

type AdminListPlatformVoicesTool struct {
	voiceRepo port.VoiceLibrary
}

func NewAdminListPlatformVoicesTool(voices port.VoiceLibrary) *AdminListPlatformVoicesTool {
	return &AdminListPlatformVoicesTool{voiceRepo: voices}
}
func (t *AdminListPlatformVoicesTool) Name() string { return "admin_list_platform_voices" }
func (t *AdminListPlatformVoicesTool) Description() string {
	return "列出平台音色（scope=platform），含默认音色标记。管理员问「有哪些平台音色」「默认音色是哪个」时调用。"
}
func (t *AdminListPlatformVoicesTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: t.Name(), Description: t.Description()}
}
func (t *AdminListPlatformVoicesTool) Execute(ctx context.Context, _ string) (entity.DataItem, error) {
	voices, err := t.voiceRepo.ListForAdmin(ctx, "platform")
	if err != nil {
		return entity.DataItem{}, err
	}
	if len(voices) == 0 {
		return entity.DataItem{Content: "暂无平台音色——请先在管理后台创建"}, nil
	}
	var sb strings.Builder
	def, _ := t.voiceRepo.GetDefault(ctx)
	for _, v := range voices {
		tag := ""
		if v.VoiceID == def.VoiceID {
			tag = " ← 默认"
		}
		status := ""
		if v.Status != "active" {
			status = "（停用）"
		}
		sb.WriteString(fmt.Sprintf("- %s [%s]%s%s\n", v.Name, v.VoiceID[:min(20, len(v.VoiceID))], tag, status))
	}
	return entity.DataItem{Content: sb.String()}, nil
}

// ---- admin_list_official_subjects 官方主体列表 ----

type AdminListOfficialSubjectsTool struct {
	subjectRepo port.SubjectAssetRepository
}

func NewAdminListOfficialSubjectsTool(subjects port.SubjectAssetRepository) *AdminListOfficialSubjectsTool {
	return &AdminListOfficialSubjectsTool{subjectRepo: subjects}
}
func (t *AdminListOfficialSubjectsTool) Name() string { return "admin_list_official_subjects" }
func (t *AdminListOfficialSubjectsTool) Description() string {
	return "列出官方主体（scope=official），含类型/状态/形象视频。管理员问「有哪些官方分身」时调用。"
}
func (t *AdminListOfficialSubjectsTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: t.Name(), Description: t.Description()}
}
func (t *AdminListOfficialSubjectsTool) Execute(ctx context.Context, _ string) (entity.DataItem, error) {
	subs, _, err := t.subjectRepo.ListByTenant(ctx, "", "official", "", 100, 0)
	if err != nil {
		return entity.DataItem{}, err
	}
	if len(subs) == 0 {
		return entity.DataItem{Content: "暂无官方主体——请先在管理后台创建"}, nil
	}
	var sb strings.Builder
	for _, s := range subs {
		video := "无"
		if s.AvatarVideoURL != "" {
			video = "已生成"
		}
		sb.WriteString(fmt.Sprintf("- %s [%s] %s | 形象视频: %s\n", s.Name, s.Kind, s.Status, video))
	}
	return entity.DataItem{Content: sb.String()}, nil
}

// ---- admin_vidu_credits Vidu 积分 ----

type AdminViduCreditsTool struct {
	provider port.GenerationProvider
}

func NewAdminViduCreditsTool(provider port.GenerationProvider) *AdminViduCreditsTool {
	return &AdminViduCreditsTool{provider: provider}
}
func (t *AdminViduCreditsTool) Name() string { return "admin_vidu_credits" }
func (t *AdminViduCreditsTool) Description() string {
	return "查询 Vidu API 积分余额。管理员问「积分还剩多少」「余额」时调用。"
}
func (t *AdminViduCreditsTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: t.Name(), Description: t.Description()}
}
func (t *AdminViduCreditsTool) Execute(ctx context.Context, _ string) (entity.DataItem, error) {
	if t.provider == nil {
		return entity.DataItem{}, fmt.Errorf("生成服务商未配置")
	}
	credits, err := t.provider.QueryCredits(ctx)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("查询失败: %w", err)
	}
	return entity.DataItem{Content: fmt.Sprintf("Vidu 积分余额：%d", credits)}, nil
}

// ---- 辅助 ----

// AdminSystemPrompt 管理员会话的系统提示词增强（注入实时系统上下文）。
// 管理员在 chat 里问"系统怎么样"时，LLM 已有最新数据可直接回答（无需 tool call）。
func AdminSystemPrompt(healthSummary string) string {
	return fmt.Sprintf(`你是智宸AI平台的管理助手。你可以帮助管理员：
- 查看系统健康状态（任务、积分、资产）
- 管理官方音色和主体
- 诊断失败的生成任务
- 查询租户用量

当前系统状态（实时）：
%s

回答管理员的问题时，优先使用上述实时数据。如果需要执行操作，请给出具体步骤指引。`,
		healthSummary)
}

// BuildHealthSummary 构建健康摘要文本（供 AdminSystemPrompt 注入）。
func BuildHealthSummary(
	genUC *generation.GenerationUseCase,
	voices port.VoiceLibrary,
	subjects port.SubjectAssetRepository,
	provider port.GenerationProvider,
) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tool := NewAdminSystemHealthTool(genUC, voices, subjects, provider)
	item, err := tool.Execute(ctx, "{}")
	if err != nil {
		return "状态获取失败"
	}
	return item.Content
}

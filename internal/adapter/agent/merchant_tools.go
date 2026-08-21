// Package agent 的商户主 Agent 工具集（Agent-as-Tool 架构第一批：先全普通工具跑通）。
//
// 主从 Agent 设计（共识文档）：
//   - 商户只与主 Agent（获客管家）对话；主 Agent 全工具视野，ReAct 循环自主编排
//   - 工具 = 现有 usecase 的薄封装（读类直查；写类带安全边界）
//   - 租户隔离：一律从 ctx 取（port.ToolTenantFrom），不信任 LLM 传参
//   - publish_work 软确认：无 confirmed 参数返回发布计划请用户确认——
//     工具返回值强制，模型无法绕过（未确认永远拿不到执行结果）
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/hotvideo"
	"webreaper/internal/usecase/knowledge"
	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/works"
)

// ---- 公共辅助 ----

func tenantOrErr(ctx context.Context) (string, error) {
	t := port.ToolTenantFrom(ctx)
	if t == "" {
		return "", fmt.Errorf("租户上下文缺失（请从商户对话入口调用）")
	}
	return t, nil
}

// textItem 快速构造文本结果 DataItem。
func textItem(id, title, content string) entity.DataItem {
	now := time.Now()
	return entity.DataItem{
		ID: id, Title: title, Content: content, RawContent: content,
		Status: entity.ItemStatusPendingReview, CreatedAt: now, UpdatedAt: now,
	}
}

func marshal(v any) string { b, _ := json.Marshal(v); return string(b) }

// ---- ① query_brands 查品牌/人设档案 ----

type QueryBrandsTool struct{ brandRepo port.BrandRepository }

func NewQueryBrandsTool(br port.BrandRepository) *QueryBrandsTool { return &QueryBrandsTool{brandRepo: br} }

func (t *QueryBrandsTool) Name() string { return "query_brands" }
func (t *QueryBrandsTool) Description() string {
	return "查询商户的品牌/人设档案列表（含行业、定位、核心卖点、竞品）。" +
		"用户问「我的人设/品牌是什么」「帮我看下档案」或发布/创作前确认品牌信息时调用。"
}

func (t *QueryBrandsTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	brands, err := t.brandRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return entity.DataItem{}, err
	}
	var sb strings.Builder
	for i, b := range brands {
		fmt.Fprintf(&sb, "%d. %s（行业：%s）\n   定位：%s\n   卖点：%s\n   竞品：%s\n   ID：%s\n",
			i+1, b.Name, b.Industry, b.Positioning, strings.Join(b.CoreSelling, "、"),
			strings.Join(b.Competitors, "、"), b.ID)
	}
	if sb.Len() == 0 {
		sb.WriteString("（商户还没有人设档案——建议引导去「人设档案」创建）")
	}
	return textItem(fmt.Sprintf("qb-%d", time.Now().UnixNano()), "品牌档案", sb.String()), nil
}

func (t *QueryBrandsTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: "query_brands", Description: t.Description(), Properties: map[string]port.PropSpec{}}
}

// ---- ② discover_hot_videos 热门同款发现 ----

type DiscoverHotVideosTool struct{ uc *hotvideo.HotVideoUseCase }

func NewDiscoverHotVideosTool(uc *hotvideo.HotVideoUseCase) *DiscoverHotVideosTool {
	return &DiscoverHotVideosTool{uc: uc}
}

func (t *DiscoverHotVideosTool) Name() string { return "discover_hot_videos" }
func (t *DiscoverHotVideosTool) Description() string {
	return "发现与品牌同赛道、最近很火的爆款短视频（真实搜索+播放点赞数据+拍摄同款选题建议）。" +
		"用户说「看看同行都在拍什么」「有什么爆款可以抄」「帮我找参考视频」时调用。" +
		"参数：brand_id（品牌 ID，可从 query_brands 获取）。"
}

type hotVideosArgs struct {
	BrandID string `json:"brand_id"`
}

func (t *DiscoverHotVideosTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args hotVideosArgs
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if args.BrandID == "" {
		return entity.DataItem{}, fmt.Errorf("brand_id is required（可先调 query_brands 获取）")
	}
	videos, err := t.uc.ListHotVideos(ctx, tenantID, args.BrandID, false)
	if err != nil {
		return entity.DataItem{}, err
	}
	var sb strings.Builder
	for i, v := range videos {
		fmt.Fprintf(&sb, "%d. %s\n   播放：%s | 同款选题：%s\n   链接：%s\n",
			i+1, v.Title, v.Platform, v.Topic, v.URL)
	}
	return textItem(fmt.Sprintf("hv-%d", time.Now().UnixNano()), fmt.Sprintf("热门同款（%d 条）", len(videos)), sb.String()), nil
}

func (t *DiscoverHotVideosTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name: "discover_hot_videos", Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"brand_id": {Type: "string", Description: "品牌 ID（query_brands 可查）"},
		},
		Required: []string{"brand_id"},
	}
}

// ---- ③ list_works 作品库 ----

type ListWorksTool struct{ uc *works.WorksUseCase }

func NewListWorksTool(uc *works.WorksUseCase) *ListWorksTool { return &ListWorksTool{uc: uc} }

func (t *ListWorksTool) Name() string { return "list_works" }
func (t *ListWorksTool) Description() string {
	return "查询商户的作品库（文章+视频/图片/音频产物，含草稿/待发布/已发布状态与互动数据）。" +
		"用户问「我有哪些作品」「有什么可以发的」「看看我的视频」时调用；发布前用它选作品。"
}

func (t *ListWorksTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	items, err := t.uc.ListWorks(ctx, tenantID)
	if err != nil {
		return entity.DataItem{}, err
	}
	var sb strings.Builder
	for i, w := range items {
		line := fmt.Sprintf("%d. [%s|%s] %s", i+1, w.Kind, statusLabel(w.Status), w.Title)
		if w.Views > 0 {
			line += fmt.Sprintf("（播放 %d · 赞 %d）", w.Views, w.Likes)
		}
		sb.WriteString(line + "\n   作品ID：" + w.ID + "\n")
	}
	if sb.Len() == 0 {
		sb.WriteString("（还没有作品——建议引导去内容合成创作）")
	}
	return textItem(fmt.Sprintf("lw-%d", time.Now().UnixNano()), fmt.Sprintf("作品库（%d 个）", len(items)), sb.String()), nil
}

func statusLabel(s string) string {
	switch s {
	case "draft":
		return "草稿"
	case "generating":
		return "生成中"
	case "ready":
		return "待发布"
	case "published":
		return "已发布"
	}
	return s
}

func (t *ListWorksTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: "list_works", Description: t.Description(), Properties: map[string]port.PropSpec{}}
}

// ---- ④ query_analytics 作品数据 ----

type QueryAnalyticsTool struct{ uc *account.PublishUseCase }

func NewQueryAnalyticsTool(uc *account.PublishUseCase) *QueryAnalyticsTool {
	return &QueryAnalyticsTool{uc: uc}
}

func (t *QueryAnalyticsTool) Name() string { return "query_analytics" }
func (t *QueryAnalyticsTool) Description() string {
	return "查询作品数据汇总：已发布数、播放/点赞/评论总量、AI 提及（各模型推荐商户的比例）。" +
		"用户问「效果怎么样」「播放量多少」「AI 会推荐我吗」时调用。"
}

func (t *QueryAnalyticsTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	s, err := t.uc.AnalyticsSummary(ctx, tenantID)
	if err != nil {
		return entity.DataItem{}, err
	}
	content := fmt.Sprintf("已发布作品：%d 个\n累计播放：%d · 点赞：%d · 评论：%d\n近 14 天发布节奏：%s\n（互动数据依赖回读快照，无数据时为 0）",
		s.Totals.Published, s.Totals.Views, s.Totals.Likes, s.Totals.Comments,
		strings.Join(func() []string {
			var ds []string
			for _, p := range s.Trend {
				if p.Published > 0 {
					ds = append(ds, fmt.Sprintf("%s×%d", p.Day, p.Published))
				}
			}
			return ds
		}(), "、"))
	return textItem(fmt.Sprintf("qa-%d", time.Now().UnixNano()), "作品数据汇总", content), nil
}

func (t *QueryAnalyticsTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: "query_analytics", Description: t.Description(), Properties: map[string]port.PropSpec{}}
}

// ---- ⑤ trigger_monitor 触发 AI 监测 ----

type TriggerMonitorTool struct{ uc *geo.MonitorUseCase }

func NewTriggerMonitorTool(uc *geo.MonitorUseCase) *TriggerMonitorTool {
	return &TriggerMonitorTool{uc: uc}
}

func (t *TriggerMonitorTool) Name() string { return "trigger_monitor" }
func (t *TriggerMonitorTool) Description() string {
	return "对品牌发起一次 AI 效果监测（检测各 AI 模型是否推荐该品牌，返回平均提及率）。" +
		"用户说「测一下 AI 推不推荐我」「看看 AI 效果」时调用。消耗监测额度。参数：brand_id。"
}

func (t *TriggerMonitorTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args hotVideosArgs
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if args.BrandID == "" {
		return entity.DataItem{}, fmt.Errorf("brand_id is required")
	}
	rate, err := t.uc.TriggerMonitor(ctx, tenantID, args.BrandID)
	if err != nil {
		return entity.DataItem{}, err
	}
	return textItem(fmt.Sprintf("tm-%d", time.Now().UnixNano()), "监测完成",
		fmt.Sprintf("监测完成：品牌平均提及率 %.1f%%（详细报告在「作品数据」页查看）", rate*100)), nil
}

func (t *TriggerMonitorTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name: "trigger_monitor", Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"brand_id": {Type: "string", Description: "品牌 ID"},
		},
		Required: []string{"brand_id"},
	}
}

// ---- ⑥ publish_work 发布作品（软确认机制） ----

type PublishWorkTool struct {
	publish     *account.PublishUseCase
	works       *works.WorksUseCase
	contentRepo port.OptimizedContentRepository
	pending     *PendingPublishStore // 硬确认 pending 层
}

func NewPublishWorkTool(pu *account.PublishUseCase, wu *works.WorksUseCase, cr port.OptimizedContentRepository, ps *PendingPublishStore) *PublishWorkTool {
	return &PublishWorkTool{publish: pu, works: wu, contentRepo: cr, pending: ps}
}

func (t *PublishWorkTool) Name() string { return "publish_work" }
func (t *PublishWorkTool) Description() string {
	return "把作品发布到社媒平台（抖音/快手/知乎/小红书）。⚠️ 两步确认制：" +
		"第一次调用不带 confirmed 参数 → 返回发布计划，你必须向用户复述并等用户明确同意；" +
		"用户同意后带 confirmed=true 重新调用才会执行。参数：work_id（list_works 获取）、" +
		"platform（douyin/kuaishou/zhihu/xiaohongshu）、mode（semi-auto 生成链接/auto 全自动）、confirmed。"
}

type publishWorkArgs struct {
	WorkID    string `json:"work_id"`
	Platform  string `json:"platform"`
	Mode      string `json:"mode"`
	Confirmed bool   `json:"confirmed"`
}

func (t *PublishWorkTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args publishWorkArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.WorkID == "" || args.Platform == "" {
		return entity.DataItem{}, fmt.Errorf("work_id 和 platform 必填（work_id 用 list_works 查）")
	}
	if args.Mode == "" {
		args.Mode = "semi-auto"
	}

	// 找作品（校验归属租户）
	items, err := t.works.ListWorks(ctx, tenantID)
	if err != nil {
		return entity.DataItem{}, err
	}
	var target *works.WorkItem
	for i := range items {
		if items[i].ID == args.WorkID {
			target = &items[i]
			break
		}
	}
	if target == nil {
		return entity.DataItem{}, fmt.Errorf("作品 %s 不存在（用 list_works 重新查）", args.WorkID)
	}

	// 确认闸：未确认 → 组装完整输入存 pending（plan_id 返回给前端渲染确认卡片）。
	// 硬确认 = 前端卡片点「确认发布」走 REST 端点取 plan 执行（与对话链路分离，
	// 模型无法伪造）；confirmed=true 软确认路径保留（对话内明确同意），双保险。
	if !args.Confirmed {
		input := t.buildInput(tenantID, args, target)
		planID := t.pending.Save(input, target.Title)
		return textItem(fmt.Sprintf("pw-pending-%d", time.Now().UnixNano()), "⚠️ 待用户确认的发布计划",
			fmt.Sprintf("发布计划已生成（plan_id=%s）：\n- 作品：《%s》（%s，%s）\n- 平台：%s\n- 模式：%s\n请向用户复述以上计划；前端已显示确认卡片，用户点「确认发布」即执行，或用户在对话中明确同意后你带 confirmed=true 重新调用。",
				planID, target.Title, target.Kind, statusLabel(target.Status), platformLabel(args.Platform), args.Mode)), nil
	}

	// 组装发布输入（文章类从内容库取正文；多媒体带产物 URL）
	in := t.buildInput(tenantID, args, target)
	if in.Content == "" && len(in.MediaURLs) == 0 {
		return entity.DataItem{}, fmt.Errorf("作品无可发布内容（文章无正文/多媒体无产物）")
	}

	job, err := t.publish.Publish(ctx, in)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("发布失败: %w", err)
	}
	result := fmt.Sprintf("发布任务已创建：%s《%s》→ %s（%s）", job.ID, job.Title, platformLabel(args.Platform), statusLabel("published"))
	if job.Mode == "semi-auto" && job.ExternalURL != "" {
		result += "\n半自动模式：请引导用户打开发布链接完成最后一步：" + job.ExternalURL
	}
	return textItem(fmt.Sprintf("pw-%d", time.Now().UnixNano()), "发布已执行", result), nil
}

// buildInput 组装发布输入（文章类从内容库取正文）。
func (t *PublishWorkTool) buildInput(tenantID string, args publishWorkArgs, target *works.WorkItem) account.PublishInput {
	in := account.PublishInput{
		TenantID:    tenantID,
		Platform:    args.Platform,
		Mode:        args.Mode,
		Title:       target.Title,
		ContentID:   target.ContentID,
		BrandID:     target.BrandID,
		ContentType: target.Kind,
		MediaURLs:   target.MediaURLs,
	}
	if target.Kind == "article" && target.ContentID != "" {
		if c, cErr := t.contentRepo.FindByID(context.Background(), tenantID, target.ContentID); cErr == nil {
			in.Content = c.OptimizedText
		}
	}
	return in
}

func platformLabel(p string) string {
	m := map[string]string{"douyin": "抖音", "kuaishou": "快手", "zhihu": "知乎", "xiaohongshu": "小红书"}
	if l, ok := m[p]; ok {
		return l
	}
	return p
}

func (t *PublishWorkTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name: "publish_work", Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"work_id":   {Type: "string", Description: "作品 ID（list_works 查询）"},
			"platform":  {Type: "string", Description: "平台：douyin/kuaishou/zhihu/xiaohongshu"},
			"mode":      {Type: "string", Description: "semi-auto（默认，生成链接）/ auto（全自动）"},
			"confirmed": {Type: "boolean", Description: "用户明确同意后传 true 才会执行发布"},
		},
		Required: []string{"work_id", "platform"},
	}
}

// ---- ⑦ query_accounts 账号绑定状态 ----

type QueryAccountsTool struct{ uc *account.AccountUseCase }

func NewQueryAccountsTool(uc *account.AccountUseCase) *QueryAccountsTool {
	return &QueryAccountsTool{uc: uc}
}

func (t *QueryAccountsTool) Name() string { return "query_accounts" }
func (t *QueryAccountsTool) Description() string {
	return "查询商户绑定的社媒平台账号（平台/健康状态/绑定方式）。" +
		"发布前检查账号可用性、用户问「我的抖音绑定了吗」「账号怎么过期了」时调用。"
}

func (t *QueryAccountsTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	accs, err := t.uc.List(ctx, tenantID)
	if err != nil {
		return entity.DataItem{}, err
	}
	var sb strings.Builder
	for _, a := range accs {
		authType := "浏览器通道"
		if a.IsOAuth() {
			authType = "官方通道"
		}
		health := map[string]string{"active": "✅ 可用", "expired": "⚠️ 已过期需重新绑定", "banned": "❌ 已封禁"}[a.Health]
		fmt.Fprintf(&sb, "- %s「%s」 %s（%s）\n", platformLabel(a.Platform), a.DisplayName, health, authType)
	}
	if sb.Len() == 0 {
		sb.WriteString("（还没有绑定任何平台账号——建议引导去「发布中心」绑定）")
	}
	return textItem(fmt.Sprintf("qac-%d", time.Now().UnixNano()), "平台账号", sb.String()), nil
}

func (t *QueryAccountsTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{Name: "query_accounts", Description: t.Description(), Properties: map[string]port.PropSpec{}}
}

// ---- ⑧ query_knowledge 品牌知识库 ----

type QueryKnowledgeTool struct{ uc *knowledge.KnowledgeUseCase }

func NewQueryKnowledgeTool(uc *knowledge.KnowledgeUseCase) *QueryKnowledgeTool {
	return &QueryKnowledgeTool{uc: uc}
}

func (t *QueryKnowledgeTool) Name() string { return "query_knowledge" }
func (t *QueryKnowledgeTool) Description() string {
	return "查询品牌的知识库素材（商户上传的品牌资料：产品介绍、价格、特色等）。" +
		"创作内容需要品牌事实依据、用户问「我的产品资料」时调用。参数：brand_id。"
}

func (t *QueryKnowledgeTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args hotVideosArgs
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if args.BrandID == "" {
		return entity.DataItem{}, fmt.Errorf("brand_id is required")
	}
	materials, _, err := t.uc.ListBrandMaterials(ctx, tenantID, args.BrandID)
	if err != nil {
		return entity.DataItem{}, err
	}
	var sb strings.Builder
	for i, m := range materials {
		content := []rune(m.Content)
		if len(content) > 200 {
			content = content[:200]
		}
		fmt.Fprintf(&sb, "%d. %s\n%s\n\n", i+1, m.Title, string(content))
	}
	if sb.Len() == 0 {
		sb.WriteString("（该品牌还没有知识库素材——建议引导在「人设档案·知识库」上传）")
	}
	return textItem(fmt.Sprintf("qk-%d", time.Now().UnixNano()), fmt.Sprintf("知识库素材（%d 条）", len(materials)), sb.String()), nil
}

func (t *QueryKnowledgeTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name: "query_knowledge", Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"brand_id": {Type: "string", Description: "品牌 ID"},
		},
		Required: []string{"brand_id"},
	}
}

// ---- ⑨ growth_advisor 增长顾问（子 Agent 示范：数据组合 + 领域专家提示词）----
//
// Agent-as-Tool 二期：子 Agent 是"特殊工具"——不暴露底层查询给主 Agent 细调，
// 而是内置领域方法论（增长诊断：感知数据 → 归因 → 建议），主 Agent 只派发任务。

type GrowthAdvisorTool struct {
	brandRepo port.BrandRepository
	works     *works.WorksUseCase
	publish   *account.PublishUseCase
	accounts  *account.AccountUseCase
	aiGen     port.AIGenerator
}

func NewGrowthAdvisorTool(br port.BrandRepository, wu *works.WorksUseCase,
	pu *account.PublishUseCase, au *account.AccountUseCase, ai port.AIGenerator) *GrowthAdvisorTool {
	return &GrowthAdvisorTool{brandRepo: br, works: wu, publish: pu, accounts: au, aiGen: ai}
}

func (t *GrowthAdvisorTool) Name() string { return "growth_advisor" }
func (t *GrowthAdvisorTool) Description() string {
	return "增长顾问（子 Agent）：综合分析商户的品牌、作品、发布数据、账号状态，" +
		"输出增长诊断（现状→问题→下一步动作，一次只建议一件事）。" +
		"用户说「帮我看看接下来该做什么」「运营得怎么样」「给点建议」时调用。参数：brand_id（可选）。"
}

type growthAdvisorArgs struct {
	BrandID string `json:"brand_id"`
}

func (t *GrowthAdvisorTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	tenantID, err := tenantOrErr(ctx)
	if err != nil {
		return entity.DataItem{}, err
	}
	var args growthAdvisorArgs
	_ = json.Unmarshal([]byte(argsJSON), &args)

	// 感知层：数据组合（主 Agent 无需逐个工具查询）
	var facts strings.Builder
	if brands, bErr := t.brandRepo.ListByTenant(ctx, tenantID); bErr == nil {
		for _, b := range brands {
			if args.BrandID != "" && b.ID != args.BrandID {
				continue
			}
			fmt.Fprintf(&facts, "【品牌】%s（行业:%s 定位:%s）\\n", b.Name, b.Industry, b.Positioning)
		}
	}
	if items, wErr := t.works.ListWorks(ctx, tenantID); wErr == nil {
		draft, ready, published := 0, 0, 0
		for _, w := range items {
			switch w.Status {
			case "draft":
				draft++
			case "ready":
				ready++
			case "published":
				published++
			}
		}
		fmt.Fprintf(&facts, "【作品】共%d（草稿%d 待发布%d 已发布%d）\n", len(items), draft, ready, published)
	}
	if s, sErr := t.publish.AnalyticsSummary(ctx, tenantID); sErr == nil {
		fmt.Fprintf(&facts, "【数据】累计播放%d 赞%d 评%d\n", s.Totals.Views, s.Totals.Likes, s.Totals.Comments)
	}
	if accs, aErr := t.accounts.List(ctx, tenantID); aErr == nil {
		for _, a := range accs {
			fmt.Fprintf(&facts, "【账号】%s %s（%s）\n", platformLabel(a.Platform), a.DisplayName, a.Health)
		}
	}

	// 方法论层：领域专家提示词
	prompt := fmt.Sprintf(`你是本地商户的增长顾问。基于以下真实经营数据做诊断：

%s

输出（口语化、老板能懂、总共不超过 150 字）：
1. 现状一句话（有数字说数字）
2. 最关键的一个问题
3. 下一步最该做的一件事（具体到点什么按钮/拍什么内容）`, facts.String())

	resp, err := t.aiGen.ChatStream(ctx, "", "", []port.ChatMessage{{Role: "user", Content: prompt}}, nil)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("增长诊断生成失败: %w", err)
	}
	// 剥离思考块——工具结果给主 Agent 的必须是干净结论（think 噪音会污染上级上下文）
	return textItem(fmt.Sprintf("ga-%d", time.Now().UnixNano()), "增长诊断", pkg.StripThinkTags(resp)), nil
}

func (t *GrowthAdvisorTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name: "growth_advisor", Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"brand_id": {Type: "string", Description: "品牌 ID（可选，不传分析全店）"},
		},
	}
}

// 编译期断言：全部实现 port.CrawlerTool。
var (
	_ port.CrawlerTool = (*QueryBrandsTool)(nil)
	_ port.CrawlerTool = (*DiscoverHotVideosTool)(nil)
	_ port.CrawlerTool = (*ListWorksTool)(nil)
	_ port.CrawlerTool = (*QueryAnalyticsTool)(nil)
	_ port.CrawlerTool = (*TriggerMonitorTool)(nil)
	_ port.CrawlerTool = (*PublishWorkTool)(nil)
	_ port.CrawlerTool = (*QueryAccountsTool)(nil)
	_ port.CrawlerTool = (*QueryKnowledgeTool)(nil)
	_ port.CrawlerTool = (*GrowthAdvisorTool)(nil)
)

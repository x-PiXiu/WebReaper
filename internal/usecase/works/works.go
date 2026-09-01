// Package works 实现作品库聚合用例（我的作品页——"我的创作资产全景"）。
//
// 三源合并（获客智能体作品语义 = 可发布成片，非素材库中间产物）：
//  1. 文章：OptimizedContent（draft/approved → 草稿；关联发布记录 → 已发布）
//  2. 成片：GenerationTask（success 且为 deliverable 成片——lip_sync / reference2video / digital_human 等）
//     素材库文生图/TTS/片段等 success 任务不进作品库（见 isDeliverableTask）
//  3. 发布状态：PublishJob 按 content_id（文章）与 media_urls 交集（多媒体）关联
//  4. 互动数据：video_metrics 最新快照（有则填充——"有就显示没有就不显示"）
package works

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/moderation"
	"webreaper/internal/usecase/port"
)

// WorkItem 作品条目（我的作品页消费）。
type WorkItem struct {
	ID          string    `json:"id"`           // c-{contentID} / g-{taskID}
	Kind        string    `json:"kind"`         // article / video / image / audio
	Title       string    `json:"title"`
	BrandID     string    `json:"brand_id,omitempty"`
	ContentID   string    `json:"content_id,omitempty"` // 跳发布中心关联（文章类）
	Status      string    `json:"status"`               // draft / generating / ready / published
	MediaURLs   []string  `json:"media_urls,omitempty"` // 多媒体产物 URL（跳发布中心预填）
	CoverURL    string    `json:"cover_url,omitempty"`
	Platforms   []string  `json:"platforms,omitempty"` // 发布过的平台
	Views       int64     `json:"views"`               // 互动数据（快照，无则 0）
	Likes       int64     `json:"likes"`
	Comments    int64     `json:"comments"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	ParentTaskID string    `json:"parent_task_id,omitempty"` // B-Roll 血缘：compose 产物的源片任务 ID
	// 32号 P2 终批：处置标注态（条目保留+原因+申诉入口；发布由服务端拦截兜底）
	ModeratedAction string `json:"moderated_action,omitempty"` // hidden / deleted
	ModeratedReason string `json:"moderated_reason,omitempty"`
	AppealStatus    string `json:"appeal_status,omitempty"`    // none/pending/rejected
}

// WorksUseCase 作品库聚合。
type WorksUseCase struct {
	contentRepo port.OptimizedContentRepository
	taskRepo    port.GenerationTaskRepository
	jobRepo     port.PublishJobRepository
	metricRepo  port.VideoMetricRepository       // 可选（未注入则互动数据为 0）
	modRepo     port.WorkModerationRepository    // 可选（32号：未注入则处置/过滤能力关闭）
	moderator   *moderation.Moderator            // 可选（32号 P2 终批：申诉文本机审）
}

func NewWorksUseCase(cr port.OptimizedContentRepository, tr port.GenerationTaskRepository,
	jr port.PublishJobRepository, mr port.VideoMetricRepository) *WorksUseCase {
	return &WorksUseCase{contentRepo: cr, taskRepo: tr, jobRepo: jr, metricRepo: mr}
}

// SetModerationRepo 注入作品处置仓储（32号：用户端过滤 + 管理端巡查/处置）。
func (uc *WorksUseCase) SetModerationRepo(r port.WorkModerationRepository) {
	if r != nil {
		uc.modRepo = r
	}
}

// ModerationEnabled 处置能力是否就绪（路由注册与前端能力探测用）。
func (uc *WorksUseCase) ModerationEnabled() bool { return uc.modRepo != nil }

// moderatedByKey 租户在效处置记录索引（hidden/deleted/flagged——申诉流后用户端改为
// 标注态展示：条目保留 + 处置信息可见 + 申诉入口；发布拦截仍在服务端双端点兜底。
// flagged（机审待复核）也进索引——产物隔离：用户端显示"审核中"而非正常可见）。
func (uc *WorksUseCase) moderatedByKey(ctx context.Context, tenantID string) map[string]entity.WorkModeration {
	if uc.modRepo == nil {
		return nil
	}
	ms, err := uc.modRepo.ListByTenant(ctx, tenantID)
	if err != nil || len(ms) == 0 {
		return nil
	}
	m := make(map[string]entity.WorkModeration, len(ms))
	for _, v := range ms {
		if v.Active() || v.Action == entity.WorkActionFlagged {
			m[v.WorkKey] = v
		}
	}
	return m
}

// HideWork 下架/逻辑删除作品（32号：管理端处置；重复处置幂等覆盖）。
// action ∈ {hidden, deleted}；reason 必填（审计）。
func (uc *WorksUseCase) HideWork(ctx context.Context, workKey, kind, tenantID, action, reason, operator string) error {
	if uc.modRepo == nil {
		return fmt.Errorf("作品处置服务未配置")
	}
	if workKey == "" {
		return fmt.Errorf("缺少作品标识")
	}
	if reason == "" {
		return fmt.Errorf("处置原因必填")
	}
	if action != entity.WorkActionHidden && action != entity.WorkActionDeleted {
		return fmt.Errorf("无效处置动作：%s", action)
	}
	if kind == "" {
		kind = "video"
	}
	if operator == "" {
		operator = "admin"
	}
	return uc.modRepo.Upsert(ctx, entity.WorkModeration{
		ID: "wm-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		WorkKey: workKey, WorkKind: kind, TenantID: tenantID,
		Action: action, Reason: reason, Source: "admin", Operator: operator,
	})
}

// ListAppealsForAdmin 申诉待复核队列（32号 P2 终批：appeal_status=pending 倒序）。
func (uc *WorksUseCase) ListAppealsForAdmin(ctx context.Context, limit int) ([]entity.WorkModeration, error) {
	if uc.modRepo == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	ms, err := uc.modRepo.ListRecent(ctx, 400)
	if err != nil {
		return nil, err
	}
	out := make([]entity.WorkModeration, 0, limit)
	for _, m := range ms {
		if m.AppealStatus == entity.AppealPending {
			out = append(out, m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ListFlaggedForAdmin 机审待复核队列（32号 P2：flagged 记录倒序——含非成片 key
// 如克隆文案/配音文案等，管理员统一在此放行或处置）。
func (uc *WorksUseCase) ListFlaggedForAdmin(ctx context.Context, limit int) ([]entity.WorkModeration, error) {
	if uc.modRepo == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	ms, err := uc.modRepo.ListRecent(ctx, 400)
	if err != nil {
		return nil, err
	}
	out := make([]entity.WorkModeration, 0, limit)
	for _, m := range ms {
		if m.Action == entity.WorkActionFlagged {
			out = append(out, m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// SetModerator 注入内容机审（可选；32号 P2 终批——申诉文本过机审，
// 防申诉通道成为违规内容展示位）。
func (uc *WorksUseCase) SetModerator(m *moderation.Moderator) {
	if m != nil {
		uc.moderator = m
	}
}

// AppealWork 用户申诉被处置作品（32号 P2 终批）。
// 防滥用：申诉中不可重复；被维持（rejected）24h 后可再申诉。
// 申诉文本异步过机审（独立 key appeal-{workKey}，不覆盖处置主记录）。
func (uc *WorksUseCase) AppealWork(ctx context.Context, tenantID, workKey, text string) error {
	if uc.modRepo == nil {
		return fmt.Errorf("申诉服务未配置")
	}
	if workKey == "" || strings.TrimSpace(text) == "" {
		return fmt.Errorf("缺少作品标识或申诉理由")
	}
	if len([]rune(strings.TrimSpace(text))) > 500 {
		return fmt.Errorf("申诉理由过长（≤500 字）")
	}
	m, err := uc.modRepo.FindByKey(ctx, workKey)
	if err != nil || !m.Active() {
		return fmt.Errorf("该作品不存在有效处置记录，无需申诉")
	}
	// 租户归属：admin 处置时带租户则校验（未带的记录放行——v1 宽松，见 32号 §六）
	if m.TenantID != "" && tenantID != "" && m.TenantID != tenantID {
		return fmt.Errorf("作品不存在")
	}
	switch m.AppealStatus {
	case entity.AppealPending:
		return fmt.Errorf("该作品申诉审核中，请耐心等待")
	case entity.AppealRejected:
		if m.AppealedAt != nil && time.Since(*m.AppealedAt) < 24*time.Hour {
			return fmt.Errorf("该作品申诉已被维持，24 小时后可再次申诉")
		}
	}
	now := time.Now()
	if uErr := uc.modRepo.UpdateAppeal(ctx, workKey, entity.AppealPending, strings.TrimSpace(text), &now); uErr != nil {
		return uErr
	}
	// 申诉文本机审（异步标记，不阻断申诉提交；独立 key 不覆盖处置记录）
	if uc.moderator != nil {
		uc.moderator.ModerateTextAsync(tenantID, "appeal-"+workKey, m.WorkKind, text)
	}
	return nil
}

// RejectAppeal 管理员维持处置（申诉终审驳回；记录保留，用户 24h 后可再申诉）。
func (uc *WorksUseCase) RejectAppeal(ctx context.Context, workKey string) error {
	if uc.modRepo == nil {
		return fmt.Errorf("作品处置服务未配置")
	}
	m, err := uc.modRepo.FindByKey(ctx, workKey)
	if err != nil {
		return fmt.Errorf("作品不存在处置记录")
	}
	if m.AppealStatus != entity.AppealPending {
		return fmt.Errorf("该作品没有待审核的申诉")
	}
	return uc.modRepo.UpdateAppeal(ctx, workKey, entity.AppealRejected, m.AppealText, m.AppealedAt)
}

// RestoreWork 恢复作品（清除处置记录；申诉采纳复用此路径——恢复即记录清除）。
func (uc *WorksUseCase) RestoreWork(ctx context.Context, workKey string) error {
	if uc.modRepo == nil {
		return fmt.Errorf("作品处置服务未配置")
	}
	if workKey == "" {
		return fmt.Errorf("缺少作品标识")
	}
	return uc.modRepo.Delete(ctx, workKey)
}

// AdminWorkItem 管理端作品巡查视图（32号）：成片条目 + 归属租户 + 处置状态。
type AdminWorkItem struct {
	WorkItem
	TenantID         string `json:"tenant_id"`
	ModerationAction string `json:"moderation_action,omitempty"` // hidden / deleted（空=未处置）
	ModerationReason string `json:"moderation_reason,omitempty"`
}

// ListRecentForAdmin 跨租户作品巡查流（32号）：最近成功成片倒序 + 处置状态关联。
// 文章类管理走既有 /admin/contents（一审一听一改），本流聚焦成片。
func (uc *WorksUseCase) ListRecentForAdmin(ctx context.Context, limit int) ([]AdminWorkItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	tasks, err := uc.taskRepo.ListRecentSuccessAll(ctx, limit*2) // 预留 deliverable 过滤损耗
	if err != nil {
		return nil, err
	}
	modByWork := map[string]entity.WorkModeration{}
	if uc.modRepo != nil {
		if ms, mErr := uc.modRepo.ListRecent(ctx, 500); mErr == nil {
			for _, m := range ms {
				modByWork[m.WorkKey] = m
			}
		}
	}
	out := make([]AdminWorkItem, 0, limit)
	for _, t := range tasks {
		if t.State != entity.TaskStateSuccess || !isDeliverableTask(t) {
			continue
		}
		creations := parseCreations(t.CreationsJSON)
		if len(creations) == 0 {
			continue
		}
		kind := t.Type
		if kind == entity.GenerationTypeDigitalHuman || kind == entity.GenerationTypeOther {
			kind = entity.GenerationTypeVideo
		}
		var urls []string
		var cover string
		for _, cr := range creations {
			urls = append(urls, cr.URL)
			if cover == "" {
				cover = cr.CoverURL
			}
		}
		it := AdminWorkItem{
			WorkItem: WorkItem{
				ID: "g-" + t.ID, Kind: kind, Title: titleFromTask(t, kind),
				BrandID: t.BrandID, Status: "ready",
				MediaURLs: urls, CoverURL: cover, CreatedAt: t.CreatedAt,
			},
			TenantID: t.TenantID,
		}
		if m, ok := modByWork["g-"+t.ID]; ok {
			it.ModerationAction = m.Action // 含 flagged（机审待复核——不 Active：不隐藏不拦发布）
			it.ModerationReason = m.Reason
		}
		out = append(out, it)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// ListWorks 聚合租户全部作品（按创建时间倒序）。
func (uc *WorksUseCase) ListWorks(ctx context.Context, tenantID string) ([]WorkItem, error) {
	// 发布状态索引：content_id → jobs（文章关联）；mediaURL → job（多媒体交集关联）
	jobs, err := uc.jobRepo.ListByTenant(ctx, tenantID, 200)
	if err != nil {
		return nil, err
	}
	jobsByContent := map[string][]entity.PublishJob{}
	jobByMedia := map[string]entity.PublishJob{}
	for _, j := range jobs {
		if j.Status != entity.PublishStatusPublished {
			continue
		}
		if j.ContentID != "" {
			jobsByContent[j.ContentID] = append(jobsByContent[j.ContentID], j)
		}
		for _, u := range j.MediaURLs {
			jobByMedia[u] = j
		}
	}
	// 互动数据索引：job_id → 最新快照
	metricByJob := map[string]entity.VideoMetric{}
	if uc.metricRepo != nil {
		if ms, mErr := uc.metricRepo.LatestByTenant(ctx, tenantID); mErr == nil {
			for _, m := range ms {
				metricByJob[m.JobID] = m
			}
		}
	}

	items := make([]WorkItem, 0, 32)

	// 32号：在效处置过滤（hidden/deleted 作品用户端不可见——列表/详情/预填同源）
	hidden := uc.moderatedByKey(ctx, tenantID)

	applyPublish := func(it *WorkItem, js []entity.PublishJob, matched *entity.PublishJob) {
		if matched != nil && matched.ID != "" {
			it.Status = "published"
			it.Platforms = []string{matched.Platform}
			t := matched.PublishedAt
			if t.IsZero() {
				t = matched.CreatedAt
			}
			it.PublishedAt = &t
			if m, ok := metricByJob[matched.ID]; ok {
				it.Views, it.Likes, it.Comments = m.Views, m.Likes, m.Comments
			}
			return
		}
		if len(js) > 0 {
			it.Status = "published"
			for _, j := range js {
				it.Platforms = append(it.Platforms, j.Platform)
			}
			t := js[0].PublishedAt
			if t.IsZero() {
				t = js[0].CreatedAt
			}
			it.PublishedAt = &t
		}
	}

	// 源 1：文章
	contents, err := uc.contentRepo.ListByTenant(ctx, tenantID, 200)
	if err == nil {
		for _, c := range contents {
			it := WorkItem{
				ID:        "c-" + c.ID,
				Kind:      "article",
				Title:     c.Title,
				BrandID:   c.BrandID,
				ContentID: c.ID,
				Status:    "draft",
				CreatedAt: c.CreatedAt,
			}
			if js := jobsByContent[c.ID]; len(js) > 0 {
				applyPublish(&it, js, nil)
			} else if c.Status == "published" {
				it.Status = "ready" // 内容标记已发布但无发布记录（历史数据）→ 待发布态展示
			}
			// 32号 P2 终批：被处置作品改为标注态（保留条目+原因+申诉入口），
			// 发布拦截由服务端双端点兜底——用户知情权与申诉通道。
			if m := hidden["c-"+c.ID]; m.Action != "" {
				it.ModeratedAction = m.Action
				it.ModeratedReason = m.Reason
				it.AppealStatus = m.AppealStatus
			}
			items = append(items, it)
		}
	}

	// 源 2：可发布成片（success 且非素材库中间产物）
	tasks, err := uc.taskRepo.List(ctx, tenantID, 200)
	if err == nil {
		for _, t := range tasks {
			if t.State != entity.TaskStateSuccess {
				continue // 生成中/失败的任务不进作品库（任务列表页管）
			}
			if !isDeliverableTask(t) {
				continue // 文生图/TTS/素材片段等仅进素材库
			}
			creations := parseCreations(t.CreationsJSON)
			if len(creations) == 0 {
				continue
			}
			kind := t.Type
			if kind == entity.GenerationTypeDigitalHuman || kind == entity.GenerationTypeOther {
				kind = entity.GenerationTypeVideo
			}
			var urls []string
			var cover string
			for _, cr := range creations {
				urls = append(urls, cr.URL)
				if cover == "" {
					cover = cr.CoverURL
				}
			}
			it := WorkItem{
				ID:        "g-" + t.ID,
				Kind:      kind,
				Title:     titleFromTask(t, kind),
				BrandID:   t.BrandID,
				Status:    "ready",
				MediaURLs: urls,
				CoverURL:  cover,
				CreatedAt: t.CreatedAt,
			}
			// B-Roll 血缘：compose 产物携带源片任务 ID（前端"已插画面/B-Roll"标记与链式入口用）
			if strings.EqualFold(strings.TrimSpace(t.SubType), "compose") {
				var pp struct {
					SourceTaskID string `json:"source_task_id"`
				}
				if json.Unmarshal([]byte(t.ParamsJSON), &pp) == nil && pp.SourceTaskID != "" {
					it.ParentTaskID = pp.SourceTaskID
				}
			}
			// 32号 P2 终批：被处置成片标注态（同文章源）
			if m := hidden["g-"+t.ID]; m.Action != "" {
				it.ModeratedAction = m.Action
				it.ModeratedReason = m.Reason
				it.AppealStatus = m.AppealStatus
			}
			// 发布关联：任一产物 URL 出现在已发布 job 的 media_urls 里
			var matched *entity.PublishJob
			for _, u := range urls {
				if j, ok := jobByMedia[u]; ok {
					j := j
					matched = &j
					break
				}
			}
			applyPublish(&it, nil, matched)
			items = append(items, it)
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

// parseCreations 解析 creations JSON（[{url,stored_url,cover_url}]）。
// BE-WORK-01：优先取 stored_url（永久 OSS 地址），fallback 到 url（24h 临时地址）。
func parseCreations(raw string) []struct{ URL, CoverURL string } {
	if raw == "" {
		return nil
	}
	var arr []struct {
		URL      string `json:"url"`
		Stored   string `json:"stored_url"`
		CoverURL string `json:"cover_url"`
	}
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	out := make([]struct{ URL, CoverURL string }, 0, len(arr))
	for _, a := range arr {
		u := a.Stored
		if u == "" {
			u = a.URL
		}
		if u != "" {
			out = append(out, struct{ URL, CoverURL string }{u, a.CoverURL})
		}
	}
	return out
}

// AdminWorkDetail 管理端作品详情（32号 F2：巡查流"看内容"能力——审核判定需要
// 看到媒体本体与生成文案，仅列表不够）。
type AdminWorkDetail struct {
	AdminWorkItem
	SubType      string   `json:"sub_type"`              // 生成端点（lip_sync/tts/...）
	Model        string   `json:"model,omitempty"`
	Provider     string   `json:"provider,omitempty"`
	Text         string   `json:"text,omitempty"`        // 生成文案（审核核心——违规判定对象）
	DurationSec  int      `json:"duration_sec,omitempty"`
	VoiceID      string   `json:"voice_id,omitempty"`    // 使用的音色
	ErrorMsg     string   `json:"error_msg,omitempty"`   // 失败原因（成功作品为空）
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	// 处置与申诉完整记录
	ModerationSource  string     `json:"moderation_source,omitempty"`  // admin / machine
	ModerationOperator string    `json:"moderation_operator,omitempty"`
	ModeratedAt       *time.Time `json:"moderated_at,omitempty"`
	AppealStatus      string     `json:"appeal_status,omitempty"`
	AppealText        string     `json:"appeal_text,omitempty"`
	AppealedAt        *time.Time `json:"appealed_at,omitempty"`
}

// GetWorkDetailForAdmin 管理端作品详情：任务全量字段 + 处置/申诉完整记录。
func (uc *WorksUseCase) GetWorkDetailForAdmin(ctx context.Context, workKey string) (*AdminWorkDetail, error) {
	if uc.modRepo == nil {
		return nil, fmt.Errorf("作品处置服务未配置")
	}
	if !strings.HasPrefix(workKey, "g-") {
		return nil, fmt.Errorf("详情仅支持成片作品（g-{任务ID}）；文章类请走内容管理")
	}
	taskID := strings.TrimPrefix(workKey, "g-")
	t, err := uc.taskRepo.FindByID(ctx, "", taskID) // 跨租户查（tenantID 空=admin 旁路）
	if err != nil {
		return nil, fmt.Errorf("作品不存在或已清理")
	}
	if t.State != entity.TaskStateSuccess || !isDeliverableTask(t) {
		return nil, fmt.Errorf("该任务非成功成片")
	}
	var mod entity.WorkModeration
	hasMod := false
	if m, mErr := uc.modRepo.FindByKey(ctx, workKey); mErr == nil {
		mod = m
		hasMod = true
	}
	var params struct {
		Text     string `json:"text"`
		VoiceID  string `json:"voice_id"`
		Duration int    `json:"duration"`
	}
	_ = json.Unmarshal([]byte(t.ParamsJSON), &params)
	creations := parseCreations(t.CreationsJSON)
	var urls []string
	var cover string
	for _, cr := range creations {
		urls = append(urls, cr.URL)
		if cover == "" {
			cover = cr.CoverURL
		}
	}
	kind := t.Type
	if kind == entity.GenerationTypeDigitalHuman || kind == entity.GenerationTypeOther {
		kind = entity.GenerationTypeVideo
	}
	d := &AdminWorkDetail{
		AdminWorkItem: AdminWorkItem{
			WorkItem: WorkItem{
				ID: workKey, Kind: kind, Title: titleFromTask(t, kind),
				BrandID: t.BrandID, Status: "ready",
				MediaURLs: urls, CoverURL: cover, CreatedAt: t.CreatedAt,
			},
			TenantID: t.TenantID,
		},
		SubType: t.SubType, Model: t.Model, Provider: t.Provider,
		Text: params.Text, VoiceID: params.VoiceID,
		DurationSec: params.Duration, FinishedAt: t.FinishedAt,
	}
	if hasMod {
		d.ModerationAction = mod.Action
		d.ModerationReason = mod.Reason
		d.ModerationSource = mod.Source
		d.ModerationOperator = mod.Operator
		d.ModeratedAt = &mod.UpdatedAt
		d.AppealStatus = mod.AppealStatus
		d.AppealText = mod.AppealText
		d.AppealedAt = mod.AppealedAt
	}
	return d, nil
}

// materialSubTypes 素材库生成类端点——产物仅进素材库，不进「我的作品」。
var materialSubTypes = map[string]bool{
	"text2image": true, "tts": true, "text2audio": true, "sound_effect": true,
	"voice_clone": true, "subject": true, "multiframe": true,
}

// deliverableSubTypes 工作台成片类端点——可进「我的作品」待发布。
// BE-WORK-02：text2video/img2video/start_end2video 从 materialSubTypes 移入，
// 这些端点的产物是最终视频，应出现在「我的作品」中。
var deliverableSubTypes = map[string]bool{
	"lip_sync": true, "reference2video": true, "digital_human": true,
	"text2video": true, "img2video": true, "start_end2video": true,
	"compose": true, // B-Roll 合成成片（22/23 号计划：源片保留，合成片进作品库）
}

// isDeliverableTask 判断 success 生成任务是否属于可发布成片（非素材库中间产物）。
func isDeliverableTask(t entity.GenerationTask) bool {
	var params map[string]any
	_ = json.Unmarshal([]byte(t.ParamsJSON), &params)
	// 链式形象视频（25 号阶段二）：分身预览中间产物，不进「我的作品」
	if v, ok := params["avatar_video"]; ok {
		if b, ok := v.(bool); ok && b {
			return false
		}
	}
	if v, ok := params["deliverable"]; ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	if v, ok := params["work_product"]; ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	sub := strings.ToLower(strings.TrimSpace(t.SubType))
	if sub == "" {
		if s, ok := params["__sub_type"].(string); ok {
			sub = strings.ToLower(strings.TrimSpace(s))
		}
	}
	if deliverableSubTypes[sub] {
		return true
	}
	if materialSubTypes[sub] {
		return false
	}
	return false
}

// titleFromTask 从任务参数提炼标题（prompt 前缀；无则按类型兜底）。
func titleFromTask(t entity.GenerationTask, kind string) string {
	var params struct {
		Prompt string `json:"prompt"`
		Text   string `json:"text"`
	}
	_ = json.Unmarshal([]byte(t.ParamsJSON), &params)

	// 优先级：用户自定义标题（custom_title——用户改名需求） > 台词首句 > prompt > 按类型兜底
	// 自动标题加时间后缀（MM-DD HH:mm）区分同一天生成的多个作品
	title := ""

	// ① 用户自定义标题（RenameTask 写入）
	var customTitle string
	_ = json.Unmarshal([]byte(t.ParamsJSON), &struct {
		CustomTitle *string `json:"custom_title"`
	}{CustomTitle: &customTitle})
	if customTitle != "" {
		return customTitle
	}

	// ② 自动提取：script/text 首句
	if s := strings.TrimSpace(params.Text); s != "" {
		if idx := strings.IndexAny(s, "。！？\n"); idx > 0 {
			title = s[:idx]
		} else {
			title = s
		}
	}
	if title == "" {
		title = strings.TrimSpace(params.Prompt)
	}
	// 去掉 @引用标记和停顿标记（形象展示 prompt 的前缀噪音）
	title = strings.TrimSpace(strings.Split(title, " ")[0])
	if idx := strings.Index(title, "<#"); idx > 0 {
		title = title[:idx]
	}

	runes := []rune(strings.TrimSpace(title))
	timeSuffix := t.CreatedAt.Format("01-02 15:04")

	if len(runes) == 0 {
		fallback := map[string]string{"video": "视频作品", "image": "图片作品", "audio": "音频作品"}
		if name, ok := fallback[kind]; ok {
			return name + " " + timeSuffix
		}
		return "多媒体作品 " + timeSuffix
	}
	if len(runes) > 15 {
		return string(runes[:15]) + "… " + timeSuffix
	}
	return string(runes) + " " + timeSuffix
}

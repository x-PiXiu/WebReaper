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
	"sort"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
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
}

// WorksUseCase 作品库聚合。
type WorksUseCase struct {
	contentRepo port.OptimizedContentRepository
	taskRepo    port.GenerationTaskRepository
	jobRepo     port.PublishJobRepository
	metricRepo  port.VideoMetricRepository // 可选（未注入则互动数据为 0）
}

func NewWorksUseCase(cr port.OptimizedContentRepository, tr port.GenerationTaskRepository,
	jr port.PublishJobRepository, mr port.VideoMetricRepository) *WorksUseCase {
	return &WorksUseCase{contentRepo: cr, taskRepo: tr, jobRepo: jr, metricRepo: mr}
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
}

// isDeliverableTask 判断 success 生成任务是否属于可发布成片（非素材库中间产物）。
func isDeliverableTask(t entity.GenerationTask) bool {
	var params map[string]any
	_ = json.Unmarshal([]byte(t.ParamsJSON), &params)
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
	title := params.Prompt
	if title == "" {
		title = params.Text
	}
	runes := []rune(strings.TrimSpace(title))
	if len(runes) > 30 {
		return string(runes[:30]) + "…"
	}
	if len(runes) == 0 {
		fallback := map[string]string{"video": "视频作品", "image": "图片作品", "audio": "音频作品"}
		if name, ok := fallback[kind]; ok {
			return name
		}
		return "多媒体作品"
	}
	return string(runes)
}

package scheduledtask

import (
	"context"
	"fmt"
	"sort"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/notification"
	"webreaper/internal/usecase/port"
)

// ---- ③ 排期发布任务（定时发送）----

// ScheduledPublishTask 扫描到期排期任务并自动执行发布（每 5 分钟）。
type ScheduledPublishTask struct {
	jobRepo   port.PublishJobRepository
	publishUC *account.PublishUseCase
	notify    *notification.NotifyUseCase
	logger    port.Logger
}

func NewScheduledPublishTask(jobRepo port.PublishJobRepository, publishUC *account.PublishUseCase, notify *notification.NotifyUseCase, logger port.Logger) *ScheduledPublishTask {
	return &ScheduledPublishTask{jobRepo: jobRepo, publishUC: publishUC, notify: notify, logger: logger}
}

func (t *ScheduledPublishTask) Name() string            { return "scheduled-publish" }
func (t *ScheduledPublishTask) Interval() time.Duration { return 5 * time.Minute }

func (t *ScheduledPublishTask) Execute(ctx context.Context) error {
	jobs, err := t.jobRepo.ListScheduledDue(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("扫描排期任务失败: %w", err)
	}
	for _, job := range jobs {
		executed, xErr := t.publishUC.ExecuteScheduledJob(ctx, job.TenantID, job.ID)
		if xErr != nil {
			t.logger.Error("排期发布失败", port.Err(xErr), port.String("job", job.ID))
			continue
		}
		t.logger.Info("排期发布已执行", port.String("job", job.ID), port.String("platform", job.Platform))
		// 通知：全自动发布完成通知；半自动生成链接后通知用户确认
		if t.notify != nil {
			status := "已自动发布"
			link := "/m/distribution"
			if executed.ExternalURL != "" {
				link = executed.ExternalURL
			}
			_ = t.notify.Push(ctx, job.TenantID, entity.NotificationTypeScheduledPublish,
				fmt.Sprintf("排期发布完成：%s", job.Title),
				fmt.Sprintf("内容已按计划发布到 %s（%s）", job.Platform, status), link)
		}
	}
	return nil
}

// ---- ④ 自动复测任务（效果追踪闭环）----

// AutoRecheckTask 定期扫描"已发布但未复测"的发布任务（发布 N 天后自动复测提及率）。
//
// 设计（付费闭环最后一步自动化）：
//   - 已发布（published）且 PostMentionRate 未更新（==0）且发布时间超过 RecheckAfter
//   - 自动执行复测（ReMonitor）→ 结果通过站内通知推送商户
//   - 复测需要 LLM 监测（成本）——平台开关控制（复用 auto_monitor 总闸）
type AutoRecheckTask struct {
	jobRepo      port.PublishJobRepository
	publishUC    *account.PublishUseCase
	notify       *notification.NotifyUseCase
	settingRepo  port.SystemSettingRepository
	recheckAfter time.Duration // 发布后多久自动复测（默认 7 天）
	logger       port.Logger
}

func NewAutoRecheckTask(jobRepo port.PublishJobRepository, publishUC *account.PublishUseCase, notify *notification.NotifyUseCase, settingRepo port.SystemSettingRepository, logger port.Logger) *AutoRecheckTask {
	return &AutoRecheckTask{
		jobRepo: jobRepo, publishUC: publishUC, notify: notify,
		settingRepo: settingRepo, recheckAfter: 7 * 24 * time.Hour, logger: logger,
	}
}

func (t *AutoRecheckTask) Name() string            { return "auto-recheck-mention" }
func (t *AutoRecheckTask) Interval() time.Duration { return 6 * time.Hour }

func (t *AutoRecheckTask) Execute(ctx context.Context) error {
	// 平台总闸（自动监测类任务统一受控）
	if t.settingRepo != nil {
		if s, err := t.settingRepo.Get(ctx, entity.SettingKeyAutoMonitor); err == nil && s.Value != "true" {
			return nil
		}
	}
	cutoff := time.Now().Add(-t.recheckAfter)
	jobs, err := t.jobRepo.ListByTenant(ctx, "", 0)
	if err != nil {
		// ListByTenant 空租户 = 全局（admin 旁路语义），失败降级
		return fmt.Errorf("扫描发布任务失败: %w", err)
	}
	rechecked := 0
	for _, job := range jobs {
		if job.Status != entity.PublishStatusPublished || job.PostMentionRate > 0 {
			continue // 未发布或已复测
		}
		if job.PublishedAt.IsZero() || job.PublishedAt.After(cutoff) {
			continue // 未到复测时间
		}
		updated, rErr := t.publishUC.ReMonitor(ctx, job.TenantID, job.ID)
		if rErr != nil {
			t.logger.Error("自动复测失败", port.Err(rErr), port.String("job", job.ID))
			continue
		}
		rechecked++
		if t.notify != nil {
			_ = t.notify.Push(ctx, job.TenantID, entity.NotificationTypeRecheckDone,
				"自动复测完成：发布效果验证",
				fmt.Sprintf("「%s」发布后提及率 %s → %s",
					job.Title,
					pct(job.PreMentionRate), pct(updated.PostMentionRate)),
				"/m/distribution")
		}
	}
	if rechecked > 0 {
		t.logger.Info("自动复测完成", port.Int("jobs", rechecked))
	}
	return nil
}

func pct(v float64) string {
	return fmt.Sprintf("%.0f%%", v*100)
}

// ---- ⑤ 监测变化通知评估（纯函数，可单测）----

// MonitorAlert 监测变化告警。
type MonitorAlert struct {
	Type    string // mention_drop / competitor_overtake
	Title   string
	Content string
}

// EvaluateBrandMonitor 评估品牌监测变化（对比本次 vs 上次平均提及率 + 竞品反超）。
//
// 规则：
//   - mention_drop：平均提及率下降 > 0.2（显著下滑，主动唤醒）
//   - competitor_overtake：本次存在竞品提及率 > 品牌提及率 且 品牌 < 0.3
func EvaluateBrandMonitor(brandName string, beforeAvg, afterAvg float64, competitorRates map[string]float64) []MonitorAlert {
	var alerts []MonitorAlert
	if beforeAvg-afterAvg > 0.2 {
		alerts = append(alerts, MonitorAlert{
			Type:  entity.NotificationTypeMentionDrop,
			Title: fmt.Sprintf("「%s」AI 提及率显著下降", brandName),
			Content: fmt.Sprintf("较上期下降 %d 个百分点（%s → %s），建议立即优化内容",
				int((beforeAvg-afterAvg)*100), pct(beforeAvg), pct(afterAvg)),
		})
	}
	if afterAvg < 0.3 {
		for name, rate := range competitorRates {
			if rate > afterAvg {
				alerts = append(alerts, MonitorAlert{
					Type:    entity.NotificationTypeCompetitorOvertake,
					Title:   fmt.Sprintf("竞品「%s」提及率反超", name),
					Content: fmt.Sprintf("你 %s vs 竞品 %s——竞品在 AI 回答中被更多推荐", pct(afterAvg), pct(rate)),
				})
				break // 每品牌最多一条反超提醒（避免刷屏）
			}
		}
	}
	return alerts
}

// ---- ⑥ 每日监测任务扩展：监测后变化通知 ----

// MonitorNotifier 包装监测后的变化评估与通知（供 DailyMonitorTask 复用）。
type MonitorNotifier struct {
	resultRepo port.MonitoringResultRepository
	notify     *notification.NotifyUseCase
	logger     port.Logger
}

func NewMonitorNotifier(resultRepo port.MonitoringResultRepository, notify *notification.NotifyUseCase, logger port.Logger) *MonitorNotifier {
	return &MonitorNotifier{resultRepo: resultRepo, notify: notify, logger: logger}
}

// BaselineAvg 取品牌上一批监测的平均提及率（变化通知基线；无历史返回 0）。
func (n *MonitorNotifier) BaselineAvg(ctx context.Context, tenantID, brandID string) float64 {
	trend, err := n.resultRepo.Trend(ctx, tenantID, brandID, 500)
	if err != nil || len(trend) == 0 {
		return 0
	}
	return latestAvgByBrand(trend)
}

// EvaluateAndNotify 对某品牌执行变化评估并推送通知（监测完成后调用）。
// beforeAvg 由调用方提供（上次监测的平均提及率）；本次结果来自 results。
func (n *MonitorNotifier) EvaluateAndNotify(ctx context.Context, tenantID, brandName string, beforeAvg float64, results []entity.MonitoringResult) {
	if n.notify == nil || len(results) == 0 {
		return
	}
	// 本次平均提及率
	total, cnt := 0.0, 0
	competitorRates := map[string]float64{}
	for _, r := range results {
		total += r.MentionRate
		cnt++
		for name, rate := range r.CompetitorRates {
			if rate > competitorRates[name] {
				competitorRates[name] = rate
			}
		}
	}
	if cnt == 0 {
		return
	}
	afterAvg := total / float64(cnt)

	alerts := EvaluateBrandMonitor(brandName, beforeAvg, afterAvg, competitorRates)
	for _, a := range alerts {
		_ = n.notify.Push(ctx, tenantID, a.Type, a.Title, a.Content, "/m/keywords")
	}
}

// avgRateFromResults 计算一组监测结果的平均提及率（无结果返回 0）。
func avgRateFromResults(results []entity.MonitoringResult) float64 {
	if len(results) == 0 {
		return 0
	}
	total := 0.0
	for _, r := range results {
		total += r.MentionRate
	}
	return total / float64(len(results))
}

// latestAvgByBrand 取品牌最近一批监测的平均提及率（趋势评估基线）。
// 实现：Trend 取最近记录，按 probed_at 排序取最新一批（同批次=同一轮监测）。
func latestAvgByBrand(results []entity.MonitoringResult) float64 {
	if len(results) == 0 {
		return 0
	}
	sorted := make([]entity.MonitoringResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ProbedAt.After(sorted[j].ProbedAt) })
	// 最新一条的 ProbedAt 视为"上一批"时间，取同一时刻的记录（±1s）
	latest := sorted[0].ProbedAt
	var batch []entity.MonitoringResult
	for _, r := range sorted {
		if r.ProbedAt.Sub(latest) > -2*time.Second && r.ProbedAt.Sub(latest) < 2*time.Second {
			batch = append(batch, r)
		}
	}
	return avgRateFromResults(batch)
}

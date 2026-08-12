// Package generation 提供统一生成用例（协议层——Vidu 全量接入核心）。
//
// 设计（Docs/Plans/03 计划文档）：所有端点共享同一任务协议——提交/回调/轮询/
// 取消/重试。本用例不依赖任何服务商细节：服务商差异在 port.GenerationProvider
// （策略），端点差异在 port.EndpointAdapter（策略注册表），模型差异在
// ModelCapability（能力向量）。新增端点/模型/服务商 = 适配器层工作，用例零改动。
package generation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// GenerationUseCase 统一生成用例。
type GenerationUseCase struct {
	provider port.GenerationProvider
	registry port.EndpointRegistry
	repo     port.GenerationTaskRepository
	asset    port.MediaAssetStore // 可选；nil=不转存（产物仅保留 24h URL）
	quotaGate port.QuotaStore     // 可选；generation 场景配额
	usageRec  port.UsageRecorder  // 可选；generation 场景计量
	// submitSem 并发节流（P3）：限制同时提交到上游的请求数，防瞬时高峰触发
	// Vidu QuotaExceeded/429。nil=不节流（向后兼容）。容量由 SetConcurrency 配置。
	submitSem chan struct{}
	// callbackNonces 防重放 nonce 去重表（单机内存 TTL；多实例换 Redis——port 预留）
	callbackNonces map[string]time.Time
}

// NewGenerationUseCase 创建统一生成用例。
func NewGenerationUseCase(provider port.GenerationProvider, registry port.EndpointRegistry, repo port.GenerationTaskRepository) *GenerationUseCase {
	return &GenerationUseCase{
		provider: provider, registry: registry, repo: repo,
		callbackNonces: map[string]time.Time{},
	}
}

// SetAssetStore 注入媒体资产存储（可选；nil=产物不转存）。
func (uc *GenerationUseCase) SetAssetStore(s port.MediaAssetStore) {
	if s != nil {
		uc.asset = s
	}
}

// SetQuotaGate 注入配额门（可选；generation 场景按次限额）。
func (uc *GenerationUseCase) SetQuotaGate(g port.QuotaStore) {
	if g != nil {
		uc.quotaGate = g
	}
}

// SetUsageRecorder 注入用量记录器（可选；generation 场景计量）。
func (uc *GenerationUseCase) SetUsageRecorder(r port.UsageRecorder) {
	if r != nil {
		uc.usageRec = r
	}
}

// SetConcurrency 设置提交并发上限（P3 节流——防 Vidu QuotaExceeded/429）。
// n<=0 表示不节流。建议设为 Vidu 套餐允许的并发数（如 5）。
func (uc *GenerationUseCase) SetConcurrency(n int) {
	if n > 0 {
		uc.submitSem = make(chan struct{}, n)
	}
}

// SubmitInput 提交生成任务的输入（API 契约由 handler 转换）。
type SubmitInput struct {
	TenantID string
	BrandID  string
	SubType  string
	Model    string
	Params   entity.GenerationParams
	Refs     []entity.PromptRef // @引用素材（服务端翻译层按端点格式映射）
	OffPeak  bool
	Watermark bool
}

// Submit 提交生成任务：校验（能力向量+端点策略）→ 防重 → 提交 → 落库。
func (uc *GenerationUseCase) Submit(ctx context.Context, in SubmitInput) (entity.GenerationTask, error) {
	if uc.provider == nil || uc.registry == nil || uc.repo == nil {
		return entity.GenerationTask{}, fmt.Errorf("生成服务未配置（需 VIDU_API_KEY）")
	}
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, in.TenantID, "generation"); err != nil {
			return entity.GenerationTask{}, err
		}
	}

	adapter, err := uc.registry.Get(ctx, in.SubType)
	if err != nil {
		return entity.GenerationTask{}, err
	}
	// 能力唯一来源：Registry（DB 驱动，管理后台可热改）——策略不持有能力表
	cap, err := uc.registry.Capability(ctx, in.SubType, in.Model)
	if err != nil {
		return entity.GenerationTask{}, err
	}
	// 提示词翻译层：@引用（图片/音频/视频）→ 该端点需要的参数格式
	params, err := translateRefs(in.SubType, cap, in.Params, in.Refs)
	if err != nil {
		return entity.GenerationTask{}, err
	}
	if err := adapter.Validate(ctx, cap, params); err != nil {
		return entity.GenerationTask{}, err
	}
	// 提示词长度上限（能力向量）
	if prompt := getPrompt(params); len([]rune(prompt)) > cap.MaxPromptLen {
		return entity.GenerationTask{}, fmt.Errorf("提示词超过 %d 字符上限", cap.MaxPromptLen)
	}

	// 防重复提交：同租户同参数哈希的未终态任务直接复用
	hash := paramsHash(in.SubType, in.Model, params)
	if pending, pErr := uc.repo.FindPendingByHash(ctx, in.TenantID, hash); pErr == nil && len(pending) > 0 {
		return pending[0], nil
	}

	now := time.Now()
	task := entity.GenerationTask{
		ID:        fmt.Sprintf("gen-%d", now.UnixNano()),
		TenantID:  in.TenantID,
		BrandID:   in.BrandID,
		Type:      adapter.Category(),
		SubType:   in.SubType,
		Model:     in.Model,
		Provider:  uc.provider.Name(),
		State:     entity.TaskStateCreated,
		ParamsHash: hash,
		OffPeak:   in.OffPeak,
		Watermark: in.Watermark,
		CreatedAt: now,
		UpdatedAt: now,
	}
	paramsJSON, _ := json.Marshal(params)
	task.ParamsJSON = string(paramsJSON)
	task.Payload = task.ID // 透传本地任务 ID——回调免查表

	// 提交（超时后任务可能已创建——先落库再提交，未知状态由轮询对齐）
	if err := uc.repo.Save(ctx, task); err != nil {
		return entity.GenerationTask{}, fmt.Errorf("任务保存失败: %w", err)
	}
	body, bErr := adapter.BuildRequest(ctx, in.Model, params, task.Payload)
	if bErr != nil {
		task.State = entity.TaskStateFailed
		task.ErrMsg = "参数组装失败: " + bErr.Error()
		task.FinishedAt = nowPtr(time.Now())
		_ = uc.repo.Save(ctx, task)
		return task, bErr
	}
	// 并发节流：信号量限流提交到上游（防瞬时高峰触发 Vidu QuotaExceeded/429）
	if uc.submitSem != nil {
		select {
		case uc.submitSem <- struct{}{}:
			defer func() { <-uc.submitSem }()
		case <-ctx.Done():
			return task, ctx.Err()
		}
	}
	taskID, credits, err := uc.provider.Submit(ctx, adapter.Endpoint(), body)
	if err != nil {
		// 提交失败：标记失败（可人工重试），保留任务供前端"重新生成"
		task.State = entity.TaskStateFailed
		task.ErrMsg = fmt.Sprintf("提交失败: %v", err)
		task.FinishedAt = nowPtr(now)
		_ = uc.repo.Save(ctx, task)
		return task, fmt.Errorf("提交失败: %w", err)
	}
	task.ProviderTaskID = taskID
	task.Credits = credits
	task.State = entity.TaskStateQueueing
	_ = uc.repo.Save(ctx, task)
	return task, nil
}

// HandleCallback 处理回调（验签由 handler 完成——本方法只做幂等状态推进）。
func (uc *GenerationUseCase) HandleCallback(ctx context.Context, payload string, status port.GenerationStatus) (entity.GenerationTask, error) {
	// payload 透传关联：本地 task_id 直接定位（O(1) 免查表）
	task, err := uc.repo.FindByID(ctx, "", payload)
	if err != nil {
		// 兜底：按 provider_task_id 查（老任务/无透传场景）
		task, err = uc.repo.FindByProviderTaskID(ctx, payload)
		if err != nil {
			return entity.GenerationTask{}, fmt.Errorf("回调任务不存在: %w", err)
		}
	}
	if entity.IsTerminal(task.State) {
		return task, nil // 终态幂等：重复回调忽略
	}
	if err := uc.applyStatus(ctx, &task, status); err != nil {
		return task, err
	}
	return task, nil
}

// PollDue 轮询未终态任务（调度器/ticker 驱动；阶段 1 单机扫描）。
func (uc *GenerationUseCase) PollDue(ctx context.Context, limit int) (int, error) {
	tasks, err := uc.repo.ListActive(ctx, limit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range tasks {
		// 回调已到达且为终态 → 跳过（双通道幂等）
		if entity.IsTerminal(t.State) {
			continue
		}
		if t.ProviderTaskID == "" {
			// 提交后未知状态（超时）：先查询对齐——provider_task_id 未知则跳过等重试
			continue
		}
		status, pErr := uc.provider.Poll(ctx, t.ProviderTaskID)
		if pErr != nil {
			continue // 单任务轮询失败不中断
		}
		if err := uc.applyStatus(ctx, &t, status); err == nil {
			n++
		}
	}
	return n, nil
}

// Cancel 取消任务。
func (uc *GenerationUseCase) Cancel(ctx context.Context, tenantID, taskID string) error {
	task, err := uc.repo.FindByID(ctx, tenantID, taskID)
	if err != nil {
		return err
	}
	if entity.IsTerminal(task.State) {
		return nil
	}
	if task.ProviderTaskID != "" {
		_ = uc.provider.Cancel(ctx, task.ProviderTaskID)
	}
	task.State = entity.TaskStateCancelled
	now := time.Now()
	task.FinishedAt = &now
	return uc.repo.Save(ctx, task)
}

// Get / List 查询（任务列表页）。
func (uc *GenerationUseCase) Get(ctx context.Context, tenantID, taskID string) (entity.GenerationTask, error) {
	return uc.repo.FindByID(ctx, tenantID, taskID)
}

func (uc *GenerationUseCase) List(ctx context.Context, tenantID string, limit int) ([]entity.GenerationTask, error) {
	return uc.repo.List(ctx, tenantID, limit)
}

// Types 可用端点类型（前端表单驱动）。
func (uc *GenerationUseCase) Types() []string { return uc.registry.Types() }

// Models 某端点可用模型。
func (uc *GenerationUseCase) Models(ctx context.Context, subType string) ([]string, error) {
	return uc.registry.Models(ctx, subType)
}

// Capabilities 某端点全部模型的能力向量（前端表单渲染：时长/分辨率/图片槽位/主体…）。
func (uc *GenerationUseCase) Capabilities(ctx context.Context, subType string) ([]entity.ModelCapability, error) {
	models, err := uc.registry.Models(ctx, subType)
	if err != nil {
		return nil, err
	}
	out := make([]entity.ModelCapability, 0, len(models))
	for _, m := range models {
		cap, err := uc.registry.Capability(ctx, subType, m)
		if err != nil {
			continue // 单个模型能力缺失不阻断整体（极端情况，DB 脏数据容错）
		}
		out = append(out, cap)
	}
	return out, nil
}

// CheckCallbackNonce 回调 nonce 防重放（handler 调用；5 分钟 TTL 去重）。
func (uc *GenerationUseCase) CheckCallbackNonce(nonce string) bool {
	now := time.Now()
	if t, ok := uc.callbackNonces[nonce]; ok && now.Sub(t) < 5*time.Minute {
		return false // 重复 nonce
	}
	uc.callbackNonces[nonce] = now
	// 清理过期（简单全量；量小可接受）
	if len(uc.callbackNonces) > 1000 {
		for k, v := range uc.callbackNonces {
			if now.Sub(v) > 5*time.Minute {
				delete(uc.callbackNonces, k)
			}
		}
	}
	return true
}

// CleanupOldTasks 清理早于 retainDays 天的终态任务 + 过期素材文件（P3 任务清理策略）。
// 由定时任务（generation-cleanup，24h）调用；活跃任务不动。
func (uc *GenerationUseCase) CleanupOldTasks(ctx context.Context, retainDays int) (tasks int64, files int, err error) {
	if uc.repo == nil {
		return 0, 0, nil
	}
	if retainDays <= 0 {
		retainDays = 30
	}
	before := time.Now().AddDate(0, 0, -retainDays)
	tasks, err = uc.repo.DeleteTerminalOlderThan(ctx, before)
	if err != nil {
		return 0, 0, err
	}
	// 素材文件清理（同阈值；LocalMediaStore 按文件 mtime 判断）
	if uc.asset != nil {
		files, _ = uc.asset.CleanupBefore(ctx, before)
	}
	return tasks, files, nil
}

// applyStatus 状态机推进（幂等核心）：success/failed 终态；queueing/processing 中间态。
func (uc *GenerationUseCase) applyStatus(ctx context.Context, task *entity.GenerationTask, st port.GenerationStatus) error {
	switch st.State {
	case entity.TaskStateSuccess:
		if len(st.Creations) == 0 {
			return fmt.Errorf("任务成功但无生成物")
		}
		creationsJSON, _ := json.Marshal(st.Creations)
		task.CreationsJSON = string(creationsJSON)
		task.State = entity.TaskStateSuccess
		task.ErrCode = ""
		task.ErrMsg = ""
		task.FinishedAt = nowPtr(time.Now())
		// 转存（24h URL 永久化；失败不阻断终态——前端标记"产物待转存"）
		if uc.asset != nil {
			for i := range st.Creations {
				if stored, sErr := uc.asset.DownloadAndStore(ctx, task.TenantID, st.Creations[i].URL, nil); sErr == nil {
					st.Creations[i].StoredURL = stored
				}
			}
			creationsJSON, _ = json.Marshal(st.Creations)
			task.CreationsJSON = string(creationsJSON)
		}
	case entity.TaskStateFailed:
		task.State = entity.TaskStateFailed
		task.ErrCode = st.ErrCode
		task.ErrMsg = uc.provider.TranslateError(st.ErrCode)
		if task.ErrMsg == "" {
			task.ErrMsg = "生成失败"
		}
		task.FinishedAt = nowPtr(time.Now())
	default: // created/queueing/processing
		task.State = st.State
	}
	task.UpdatedAt = time.Now()
	return uc.repo.Save(ctx, *task)
}

// ---- 纯函数辅助（可单测）----

// RetryClass 失败可重试分类。
type RetryClass int

const (
	RetryAuto    RetryClass = iota // 自动重试（限流/内部错误）
	RetryManual                   // 人工重试（积分/风控——提示后前端"重新生成"）
	RetryTerminal                 // 不可重试（素材问题）
)

// ClassifyError 失败分类（TooManyRequests 自动退避；风控/积分人工；素材问题终态）。
func ClassifyError(code string) RetryClass {
	switch code {
	case "TooManyRequests", "SystemThrottling", "InternalServiceFailure", "QuotaExceeded", "OperationInProcess":
		return RetryAuto
	case "CreditInsufficient", "TaskPromptPolicyViolation", "AuditSubmitIllegal", "CreationPolicyViolation",
		"ModelUnavailable", "UserCancelled", "TaskNotFound":
		return RetryManual
	default:
		return RetryTerminal
	}
}

// CanAutoRetry 是否允许自动重试（指数退避 1/5/30 分钟，≤3 次）。
func CanAutoRetry(code string, retryCount int) bool {
	return ClassifyError(code) == RetryAuto && retryCount < 3
}

// paramsHash 参数哈希（防重复提交：subType+model+规范化参数）。
func paramsHash(subType, model string, params entity.GenerationParams) string {
	// 规范化：按键排序（排除 payload/回调类动态字段）
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "callback_url" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(subType)
	sb.WriteString("|")
	sb.WriteString(model)
	for _, k := range keys {
		sb.WriteString("|")
		sb.WriteString(k)
		sb.WriteString("=")
		if v, ok := params[k].(string); ok {
			sb.WriteString(v)
		} else if b, err := json.Marshal(params[k]); err == nil {
			sb.WriteString(string(b))
		}
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

func getPrompt(p entity.GenerationParams) string {
	if v, ok := p["prompt"].(string); ok {
		return v
	}
	return ""
}

func nowPtr(t time.Time) *time.Time { return &t }

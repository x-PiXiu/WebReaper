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
	"encoding/base64"
	"regexp"
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
	// nonceStore 回调防重放（R2 无状态化：内存实现多实例失效——Redis 实现
	// SETNX+EX 原子判重；构造默认内存，main 按 Redis 可用性切换）
	nonceStore port.CallbackNonceStore
	// notifier 任务终态通知（可选；nil=不通知——异步任务完成主动唤醒商户）
	notifier port.TaskNotifier
	// callbackURL 公网回调地址（可选；空=纯轮询。注入到支持回调的端点请求体——
	// Vidu 任务状态变化时主动 POST，轮询降级为兜底通道，双通道幂等合并）
	callbackURL string
}

// NewGenerationUseCase 创建统一生成用例。
func NewGenerationUseCase(provider port.GenerationProvider, registry port.EndpointRegistry, repo port.GenerationTaskRepository) *GenerationUseCase {
	return &GenerationUseCase{
		provider: provider, registry: registry, repo: repo,
		nonceStore: newMemoryNonceStore(),
	}
}

// memoryNonceStore 包内默认（单机；与 adapter/cache.MemoryNonceStore 同语义——
// 用例包不 import adapter，保持依赖向内）。
type memoryNonceStore struct {
	nonces map[string]time.Time
}

func newMemoryNonceStore() *memoryNonceStore { return &memoryNonceStore{nonces: map[string]time.Time{}} }

func (s *memoryNonceStore) Seen(_ context.Context, nonce string) bool {
	now := time.Now()
	if t, ok := s.nonces[nonce]; ok && now.Sub(t) < 5*time.Minute {
		return false
	}
	s.nonces[nonce] = now
	if len(s.nonces) > 1000 {
		for k, v := range s.nonces {
			if now.Sub(v) > 5*time.Minute {
				delete(s.nonces, k)
			}
		}
	}
	return true
}

// SetNonceStore 注入 nonce 判重存储（可选；Redis 实现多实例安全）。
func (uc *GenerationUseCase) SetNonceStore(s port.CallbackNonceStore) {
	if s != nil {
		uc.nonceStore = s
	}
}

// SetTaskNotifier 注入任务终态通知（可选；异步任务完成/失败时站内信唤醒商户）。
func (uc *GenerationUseCase) SetTaskNotifier(n port.TaskNotifier) {
	if n != nil {
		uc.notifier = n
	}
}

// SetCallbackURL 注入公网回调地址（可选；空=纯轮询）。
// 需为 Vidu 可达的公网 URL（含路由前缀），如 https://x.com/webreaper/api/v1/generation/callback。
func (uc *GenerationUseCase) SetCallbackURL(u string) {
	uc.callbackURL = strings.TrimRight(u, "/")
}

// injectCallbackURL 回调地址注入：仅文档声明 callback_url 的端点（CallbackEndpoint），
// 其余端点注入未声明参数有被上游拒绝的风险——不注入，走纯轮询。
func (uc *GenerationUseCase) injectCallbackURL(adapter port.EndpointAdapter, body map[string]any) {
	if uc.callbackURL == "" {
		return
	}
	if cb, ok := adapter.(port.CallbackEndpoint); ok && cb.SupportsCallback() {
		body["callback_url"] = uc.callbackURL
	}
}

// pickModelFor 聚合端点全部启用模型的能力向量，交给端点策略挑选（傻瓜式：
// 客户端不传 model）。单默认能力端点（subject/tts 等）由 Registry 的
// "model 空 + 单条默认"匹配规则处理，不走此路径。
func (uc *GenerationUseCase) pickModelFor(ctx context.Context, subType string, sel port.ModelAutoSelector, params entity.GenerationParams) string {
	names, err := uc.registry.Models(ctx, subType)
	if err != nil {
		return ""
	}
	caps := make([]entity.ModelCapability, 0, len(names))
	for _, n := range names {
		if c, cErr := uc.registry.Capability(ctx, subType, n); cErr == nil {
			caps = append(caps, c)
		}
	}
	return sel.PickModel(caps, params)
}

// inlineMediaKeys 请求体中承载媒体 URL 的字段（数组与单值两种形态）。
var inlineMediaKeys = [2][]string{
	{"images", "videos"},                        // 数组字段
	{"image", "start_image", "audio_url"},       // 单值字段
}

// inlineLocalMedia 本站托管素材 → base64 data URI 内联（body 级——ParamsJSON/
// 防重哈希仍记原 URL）。Vidu 拉不到 localhost/内网 URL：同步端点（主体创建）
// 创建即拉素材，不可达直接 400 BadRequest；异步端点也会在任务内失败。
// 仅替换本地存储能读到的 URL；外部 URL（图床/OSS，本身公网可达）不动。
// 超 15MB 的文件保留 URL（Vidu 上限为 decode 后 20M，超限交给上游报可读错误）。
func (uc *GenerationUseCase) inlineLocalMedia(ctx context.Context, body map[string]any) {
	if uc.asset == nil {
		return
	}
	for _, key := range inlineMediaKeys[0] {
		if arr, ok := body[key].([]string); ok {
			for i, u := range arr {
				arr[i] = uc.inlineOneMedia(ctx, u)
			}
		}
	}
	for _, key := range inlineMediaKeys[1] {
		if u, ok := body[key].(string); ok {
			body[key] = uc.inlineOneMedia(ctx, u)
		}
	}
}

func (uc *GenerationUseCase) inlineOneMedia(ctx context.Context, u string) string {
	if u == "" || strings.HasPrefix(u, "data:") {
		return u // 已是 data URI 或空值
	}
	data, mime, ok := uc.asset.ReadLocal(ctx, u)
	if !ok || len(data) == 0 || len(data) > 15<<20 {
		return u
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// notifyTerminal 终态通知（fire-and-forget——通知失败不影响状态机）。
func (uc *GenerationUseCase) notifyTerminal(ctx context.Context, task entity.GenerationTask) {
	if uc.notifier == nil {
		return
	}
	uc.notifier.NotifyTaskTerminal(ctx, task)
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
	// 模型自动选择（傻瓜式端点）：model 传空时由端点策略按原始参数挑选
	//（如 reference2video：图片主体→q3 / 视频主体→q2-pro）。须在翻译层之前——
	// 翻译层按选定模型的能力向量决定视频引用的映射。
	model := in.Model
	if model == "" {
		if sel, ok := adapter.(port.ModelAutoSelector); ok {
			model = uc.pickModelFor(ctx, in.SubType, sel, in.Params)
		}
	}
	// 能力唯一来源：Registry（DB 驱动，管理后台可热改）——策略不持有能力表
	cap, err := uc.registry.Capability(ctx, in.SubType, model)
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
	hash := paramsHash(in.SubType, model, params)
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
		Model:     model,
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
	body, bErr := adapter.BuildRequest(ctx, model, params, task.Payload)
	if bErr != nil {
		task.State = entity.TaskStateFailed
		task.ErrMsg = "参数组装失败: " + bErr.Error()
		task.FinishedAt = nowPtr(time.Now())
		_ = uc.repo.Save(ctx, task)
		return task, bErr
	}
	uc.injectCallbackURL(adapter, body)
	uc.inlineLocalMedia(ctx, body)
	// 并发节流：信号量限流提交到上游（防瞬时高峰触发 Vidu QuotaExceeded/429）
	if uc.submitSem != nil {
		select {
		case uc.submitSem <- struct{}{}:
			defer func() { <-uc.submitSem }()
		case <-ctx.Done():
			return task, ctx.Err()
		}
	}
	res, err := uc.provider.Submit(ctx, adapter.Endpoint(), body)
	if err != nil {
		// 提交失败：标记失败（可人工重试），保留任务供前端"重新生成"
		task.State = entity.TaskStateFailed
		task.ErrMsg = fmt.Sprintf("提交失败: %v", err)
		task.FinishedAt = nowPtr(now)
		_ = uc.repo.Save(ctx, task)
		return task, fmt.Errorf("提交失败: %w", err)
	}
	task.ProviderTaskID = res.TaskID
	task.Credits = res.Credits
	uc.applySubmitResult(ctx, &task, adapter, res)
	_ = uc.repo.Save(ctx, task)
	// 计量（F3：generation 场景按次计费的数据地基——失败仅忽略，不影响主流程）
	uc.recordUsage(ctx, task.TenantID, task.Model, res.Credits)
	return task, nil
}

// applySubmitResult 提交结果落任务状态（三类语义归一）：
//   - SyncSubmitter 端点（主体 API）：响应即终态、无轮询语义——产物为资源 ID
//   - SubmitResult.State 终态（同步接口：语音合成/声音复刻）：创建响应已携带
//     产物（file_url/demo_audio）——复用 applyStatus 落终态并触发 24h URL 转存
//   - 其余（queueing/无 state 字段）：进轮询（PollDue 推进）
func (uc *GenerationUseCase) applySubmitResult(ctx context.Context, task *entity.GenerationTask, adapter port.EndpointAdapter, res port.SubmitResult) {
	if sync, ok := adapter.(port.SyncSubmitter); ok && sync.IsSync() {
		task.State = entity.TaskStateSuccess
		task.FinishedAt = nowPtr(time.Now())
		task.UpdatedAt = time.Now()
		// 主体无媒体产物：creations[0].id = 服务商资源 ID（主体 server_id，
		// reference2video 的 subjects[].server_id 引用值）
		creationsJSON, _ := json.Marshal([]entity.CreationItem{{ID: res.TaskID}})
		task.CreationsJSON = string(creationsJSON)
		uc.notifyTerminal(ctx, *task)
		return
	}
	switch res.State {
	case entity.TaskStateSuccess, entity.TaskStateFailed:
		// 同步接口创建即终态——防"success 但响应未携带产物"卡住 applyStatus
		// 的"成功无生成物"校验，兜底一条 ID 型产物
		if res.State == entity.TaskStateSuccess && len(res.Creations) == 0 {
			res.Creations = []entity.CreationItem{{ID: res.TaskID}}
		}
		_ = uc.applyStatus(ctx, task, port.GenerationStatus{State: res.State, Creations: res.Creations})
	default:
		task.State = entity.TaskStateQueueing
		task.UpdatedAt = time.Now()
	}
}

// recordUsage 记一次生成用量（usages 表——成本分析/配额核对的唯一数据源；
// 此前 usageRec 字段注入了也从不被调用，属于死代码，本方法补齐调用链）。
func (uc *GenerationUseCase) recordUsage(ctx context.Context, tenantID, model string, credits int) {
	if uc.usageRec == nil {
		return
	}
	_ = uc.usageRec.RecordUsage(ctx, entity.UsageRecord{
		TenantID:    tenantID,
		Scene:       "generation",
		LLMConfigName: uc.provider.Name(),
		Model:       model,
		// Vidu 无 token 概念：LLMCalls=1 按次计数；credits 记入 CompletionTokens
		// 供成本分析按积分核算（字段语义在 usages 侧按 scene 解释）
		CompletionTokens: credits,
		LLMCalls:         1,
	})
}

// HandleCallback 处理回调（验签由 handler 完成——本方法只做幂等状态推进）。
// 任务定位双路径：payload 透传（声明了 payload 参数的端点——本地 task_id 直达）
// → provider_task_id 兜底（回调体的 id 字段——payload 未声明的端点如文生音频）。
func (uc *GenerationUseCase) HandleCallback(ctx context.Context, payload, providerTaskID string, status port.GenerationStatus) (entity.GenerationTask, error) {
	task, err := uc.repo.FindByID(ctx, "", payload)
	if err != nil {
		if providerTaskID != "" {
			// 兜底：按回调体 id（服务商任务 ID）定位
			if task, err = uc.repo.FindByProviderTaskID(ctx, providerTaskID); err != nil {
				return entity.GenerationTask{}, fmt.Errorf("回调任务不存在: %w", err)
			}
		} else {
			// 老任务/无透传场景：payload 字段也可能是服务商任务 ID
			task, err = uc.repo.FindByProviderTaskID(ctx, payload)
			if err != nil {
				return entity.GenerationTask{}, fmt.Errorf("回调任务不存在: %w", err)
			}
		}
	}
	if entity.IsTerminal(task.State) {
		return task, nil // 终态幂等：重复回调忽略
	}
	// 回调到达标记（F-fix：设计意图"回调后轮询提前停"的观测字段——此前从未写入）
	task.CallbackReceived = true
	now := time.Now()
	task.CallbackAt = &now
	_ = uc.repo.Save(ctx, task)
	if err := uc.applyStatus(ctx, &task, status); err != nil {
		return task, err
	}
	return task, nil
}

// stuckTimeout 卡死任务超时：视频生成分钟级完成，2 小时无状态更新视为上游
// 失联（此前 processing 永不终态的任务会被 20s 轮询无限扫描）。
const stuckTimeout = 2 * time.Hour

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
		// 卡死超时（F-fix）：长时间无状态更新 → 判失败（RetryTerminal，不自动重试）
		if time.Since(t.UpdatedAt) > stuckTimeout && !t.UpdatedAt.IsZero() {
			t.State = entity.TaskStateFailed
			t.ErrCode = "LocalStuckTimeout"
			t.ErrMsg = "任务超时：上游长时间无响应，请重新生成"
			now := time.Now()
			t.FinishedAt = &now
			t.UpdatedAt = now
			if err := uc.repo.Save(ctx, t); err == nil {
				n++
			}
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

// retryBackoff 自动重试退避表（与 CanAutoRetry 注释一致：1/5/30 分钟，≤3 次）。
var retryBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}

// RetryDue 自动重试执行器（F-fix：ClassifyError/CanAutoRetry 此前只停留在纯函数，
// 无任何调用方——本方法由 generation-poll 驱动，补齐"限流/内部错误自动退避重提"闭环）。
// 流程：取 failed 任务 → 过滤可自动重试（分类+次数+退避窗口）→ 重提上游 → 回到 queueing。
func (uc *GenerationUseCase) RetryDue(ctx context.Context, limit int) (int, error) {
	if uc.provider == nil || uc.registry == nil || uc.repo == nil {
		return 0, nil
	}
	failed, err := uc.repo.ListFailed(ctx, limit)
	if err != nil || len(failed) == 0 {
		return 0, err
	}
	n := 0
	for _, task := range failed {
		if !CanAutoRetry(task.ErrCode, task.RetryCount) {
			continue
		}
		// 退避窗口：距上次失败不足 backoff[RetryCount] 则跳过（本轮不动）
		if task.FinishedAt != nil && time.Since(*task.FinishedAt) < retryBackoff[min(task.RetryCount, len(retryBackoff)-1)] {
			continue
		}
		if uc.retrySubmit(ctx, &task) {
			n++
		}
	}
	return n, nil
}

// retrySubmit 重提单个任务（重建上游请求；成功→queueing 且 RetryCount+1，
// 失败→RetryCount+1 保留 failed 供下轮窗口后再试，超 3 次由 CanAutoRetry 拦停）。
func (uc *GenerationUseCase) retrySubmit(ctx context.Context, task *entity.GenerationTask) bool {
	adapter, err := uc.registry.Get(ctx, task.SubType)
	if err != nil {
		return false
	}
	// 模型能力校验：被管理后台停用的模型不再自动重试（可人工"重新生成"换模型）
	if _, cErr := uc.registry.Capability(ctx, task.SubType, task.Model); cErr != nil {
		return false
	}
	params := entity.GenerationParams{}
	_ = json.Unmarshal([]byte(task.ParamsJSON), &params)
	body, err := adapter.BuildRequest(ctx, task.Model, params, task.Payload)
	if err != nil {
		return false
	}
	uc.injectCallbackURL(adapter, body)
	uc.inlineLocalMedia(ctx, body)
	// 复用提交信号量：自动重试不侵占商户提交的即时配额之外的上游并发
	if uc.submitSem != nil {
		select {
		case uc.submitSem <- struct{}{}:
			defer func() { <-uc.submitSem }()
		case <-ctx.Done():
			return false
		}
	}
	res, err := uc.provider.Submit(ctx, adapter.Endpoint(), body)
	task.RetryCount++
	task.UpdatedAt = time.Now()
	if err != nil {
		task.ErrMsg = "自动重试提交失败: " + err.Error()
		_ = uc.repo.Save(ctx, *task)
		return false
	}
	task.ProviderTaskID = res.TaskID
	task.Credits = res.Credits
	task.State = entity.TaskStateQueueing
	task.ErrCode = ""
	task.ErrMsg = ""
	task.FinishedAt = nil
	uc.applySubmitResult(ctx, task, adapter, res)
	_ = uc.repo.Save(ctx, *task)
	uc.recordUsage(ctx, task.TenantID, task.Model, res.Credits)
	return true
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

// DeleteTask 删除任务的本地产记录（资产库"删除数字人"等场景）。
// 非终态任务先尽力取消上游（失败不阻断——本地记录仍删除，避免卡死任务删不掉）；
// Vidu 无删除主体 API，主体类删除仅移除本地展示记录。
func (uc *GenerationUseCase) DeleteTask(ctx context.Context, tenantID, taskID string) error {
	task, err := uc.repo.FindByID(ctx, tenantID, taskID)
	if err != nil {
		return err
	}
	if !entity.IsTerminal(task.State) && task.ProviderTaskID != "" {
		_ = uc.provider.Cancel(ctx, task.ProviderTaskID)
	}
	return uc.repo.Delete(ctx, tenantID, taskID)
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

// CheckCallbackNonce 回调 nonce 防重放（handler 调用；内存/Redis 实现同语义）。
func (uc *GenerationUseCase) CheckCallbackNonce(ctx context.Context, nonce string) bool {
	return uc.nonceStore.Seen(ctx, nonce)
}

// CleanupOldTasks 清理早于 retainDays 天的终态任务 + 过期素材文件（P3 任务清理策略）。
// 由定时任务（generation-cleanup，24h）调用；活跃任务不动。
// R1：清理素材前先收集仍被引用的产物 URL（活跃任务+近期任务）做排除——
// 此前按 mtime 直接删，可能删掉商户分发中心还在用的视频源文件。
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
	// 素材清理（同阈值；带引用排除）
	if uc.asset != nil {
		exclude := uc.referencedMediaURLs(ctx)
		files, _ = uc.asset.CleanupBefore(ctx, before, exclude)
	}
	return tasks, files, nil
}

// urlPattern 提取任务数据中的媒体 URL（creations[].url/stored_url 与翻译后的引用参数）。
var urlPattern = regexp.MustCompile(`https?://[^\s"\\,}\]]+`)

// referencedMediaURLs 收集仍可能被引用的媒体 URL：活跃任务全部 + 近期任务
//（保留窗口内的终态任务未删，其产物 URL 仍会出现在前端/分发中心）。
func (uc *GenerationUseCase) referencedMediaURLs(ctx context.Context) map[string]bool {
	exclude := map[string]bool{}
	active, _ := uc.repo.ListActive(ctx, 200)
	recent, _ := uc.repo.List(ctx, "", 500)
	for _, t := range append(active, recent...) {
		for _, u := range urlPattern.FindAllString(t.ParamsJSON+" "+t.CreationsJSON, -1) {
			exclude[strings.TrimRight(u, ".")] = true
		}
	}
	return exclude
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
	if err := uc.repo.Save(ctx, *task); err != nil {
		return err
	}
	// 终态转换恰好一次（PollDue/HandleCallback 均有 IsTerminal 幂等护栏）——
	// 在此通知不会重复；异步任务的完成感知差距（不留在页面就不知道结果）由此闭合
	uc.notifyTerminal(ctx, *task)
	return nil
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

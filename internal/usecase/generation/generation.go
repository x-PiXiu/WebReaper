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
	"log"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// subTypeToCapID 端点类型 → 能力ID映射。
// 用于从 CapabilityResolver 查询该端点应该使用哪个厂商。
var subTypeToCapID = map[string]string{
	"text2video":      "video",
	"img2video":       "video",
	"start_end2video": "video",
	"reference2video": "video",
	"multiframe":      "video",
	"digital_human":   "digital-human",
	"lip_sync":        "video",
	"subject":         "video",
	"text2image":      "image",
	"tts":             "tts",
	"voice_clone":     "voice-clone",
	"text2audio":      "audio",
	"sound_effect":    "audio",
}

// GenerationUseCase 统一生成用例。
//
// 设计（多厂商动态选择）：
//   - providers：多个厂商的 provider 实现（vidu/xiaomi-mimo/kling/...）
//   - resolver：能力路由解析器，根据端点类型查询应该使用哪个厂商
//   - getProvider()：根据 subType 动态选择 provider
type GenerationUseCase struct {
	providers map[string]port.GenerationProvider // 多厂商 provider
	resolver  port.CapabilityResolver           // 能力路由解析器（可选；nil=使用默认provider）
	registry  port.EndpointRegistry
	repo      port.GenerationTaskRepository
	asset     port.MediaAssetStore // 可选；nil=不转存（产物仅保留 24h URL）
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
	// settingRepo 系统设置（可选；傻瓜式默认值通道——watermark/off_peak 等管理后台全局默认）
	settingRepo port.SystemSettingRepository
	// templateRepo 生成模板（可选；傻瓜式默认值通道——模板 default_params 填充未显式指定的参数）
	templateRepo port.TemplateRepository
	// callbackURL 公网回调地址（可选；空=纯轮询。注入到支持回调的端点请求体——
	// Vidu 任务状态变化时主动 POST，轮询降级为兜底通道，双通道幂等合并）
	callbackURL string
	// endpointSelector 端点选择器（可选；nil=不支持统一提交）
	endpointSelector port.EndpointSelector
	// composer B-Roll 合成编排（可选；nil=compose 类型不可用——main 装配注入 videocompose.UseCase）
	composer port.Composer
	// defaultProvider 默认厂商（当 resolver 未配置或查询失败时使用）
	defaultProvider string
	// urlResolver 素材 URL 可达性解析（可选；nil=不转换——SRP：URL 判断+格式转换移至 Adapter 层）
	urlResolver port.MaterialURLResolver
}

// NewGenerationUseCase 创建统一生成用例（支持多厂商）。
func NewGenerationUseCase(providers map[string]port.GenerationProvider, registry port.EndpointRegistry, repo port.GenerationTaskRepository) *GenerationUseCase {
	// 确定默认厂商（第一个非nil的provider）
	defaultProvider := ""
	for name, p := range providers {
		if p != nil {
			defaultProvider = name
			break
		}
	}

	return &GenerationUseCase{
		providers:       providers,
		registry:        registry,
		repo:            repo,
		nonceStore:      newMemoryNonceStore(),
		defaultProvider: defaultProvider,
	}
}

// SetCapabilityResolver 注入能力路由解析器（可选；nil=使用默认provider）。
func (uc *GenerationUseCase) SetCapabilityResolver(r port.CapabilityResolver) {
	if r != nil {
		uc.resolver = r
	}
}

// getProvider 根据端点类型动态选择 provider。
//
// 选择逻辑：
//  1. 如果配置了 resolver，根据能力路由查询应该使用哪个厂商
//  2. 如果 resolver 未配置或查询失败，使用默认 provider
//  3. 如果默认 provider 也不可用，返回错误
func (uc *GenerationUseCase) getProvider(ctx context.Context, subType string) (port.GenerationProvider, error) {
	// 1. 尝试通过 resolver 查询
	if uc.resolver != nil {
		capID, ok := subTypeToCapID[subType]
		if !ok {
			capID = "video" // 默认为视频能力
		}

		cap, err := uc.resolver.Resolve(ctx, capID)
		if err == nil && cap.VendorID != "" {
			if provider, ok := uc.providers[cap.VendorID]; ok && provider != nil {
				return provider, nil
			}
		}
	}

	// 2. 使用默认 provider
	if uc.defaultProvider != "" {
		if provider, ok := uc.providers[uc.defaultProvider]; ok && provider != nil {
			return provider, nil
		}
	}

	// 3. 遍历 providers，返回第一个可用的
	for _, provider := range uc.providers {
		if provider != nil {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("没有可用的生成服务提供商")
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

// SetComposer 注入 B-Roll 合成编排（可选；未注入则 compose 类型报不可用）。
func (uc *GenerationUseCase) SetComposer(c port.Composer) { uc.composer = c }

// SetEndpointSelector 注入端点选择器（可选；未注入则不支持统一提交）。
func (uc *GenerationUseCase) SetEndpointSelector(s port.EndpointSelector) {
	if s != nil {
		uc.endpointSelector = s
	}
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

// getDefaultModel 从数据库获取默认模型。
// 当 EndpointAdapter 没有实现 ModelAutoSelector 时使用。
func (uc *GenerationUseCase) getDefaultModel(ctx context.Context, subType string) string {
	// BE-GEN-03 + 新3：优先取 is_default=true 且 max_prompt_len>0 的模型。
	// 此前按字母序取第一个（viduq1 排在 viduq2 前），且多个 is_default=true
	// 时可能选中 max_prompt_len=0 的旧模型（如 vidu2.0），导致 prompt 被拒绝。
	bestDefault := ""
	bestDefaultPrompt := 0
	for _, spec := range uc.registry.AllSpecs(ctx) {
		if spec.SubType != subType || !spec.Enabled || !spec.IsDefault || spec.Model == "" {
			continue
		}
		cap, err := uc.registry.Capability(ctx, subType, spec.Model)
		if err != nil {
			continue
		}
		if cap.MaxPromptLen > bestDefaultPrompt {
			bestDefault = spec.Model
			bestDefaultPrompt = cap.MaxPromptLen
		}
	}
	if bestDefault != "" {
		return bestDefault
	}
	// 从 registry 获取所有可用模型
	names, err := uc.registry.Models(ctx, subType)
	if err != nil || len(names) == 0 {
		return ""
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
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
//
// 多厂商动态选择：根据 subType 通过 CapabilityResolver 查询应该使用哪个厂商，
// 然后从 providers 中获取对应的 provider 进行调用。
func (uc *GenerationUseCase) Submit(ctx context.Context, in SubmitInput) (entity.GenerationTask, error) {
	if uc.registry == nil || uc.repo == nil {
		return entity.GenerationTask{}, fmt.Errorf("生成服务未配置")
	}

	// BE-GEN-04：私网素材 base64 内联与落库分离——
	// 此处在 Submit 入口做内联会导致 params_json（TEXT 列 64KB）超长（Error 1406）。
	// 正确时序：落库用原始 URL 的 params 副本 → BuildRequest 出口才做内联（发给 Vidu）。
	// 公网部署 URL 原样、行为不变。
	//（内联延迟到 BuildRequest 前，见下方 localizeCall点）

	// 动态选择 provider
	provider, err := uc.getProvider(ctx, in.SubType)
	if err != nil {
		return entity.GenerationTask{}, fmt.Errorf("获取生成服务提供商失败: %w", err)
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
	// BE-GEN-01：params.model 优先于自动选择（UI 选定模型经白名单合并进 params，
	// 此处提升到选模层——此前不读导致 UI 选 viduq2 落到字母序 viduq1）
	model := in.Model
	if model == "" {
		if pm, ok := in.Params["model"].(string); ok && pm != "" {
			model = pm
		}
	}
	if model == "" {
		if sel, ok := adapter.(port.ModelAutoSelector); ok {
			model = uc.pickModelFor(ctx, in.SubType, sel, in.Params)
		}
		if model == "" {
			model = uc.getDefaultModel(ctx, in.SubType)
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
		Provider:  provider.Name(),  // 使用动态选择的 provider
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
	// BE-GEN-04 补充：BuildRequest 出口做私网素材内联（仅影响发给 Vidu 的 body，
	// 不落 params_json——落库序列化已在上方用原始 URL 的 params 完成）
	if params != nil {
		if lErr := uc.localizePrivateMaterials(ctx, params); lErr != nil {
			return entity.GenerationTask{}, lErr
		}
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
	// 并发节流：信号量限流提交到上游（防瞬时高峰触发 QuotaExceeded/429）
	if uc.submitSem != nil {
		select {
		case uc.submitSem <- struct{}{}:
			defer func() { <-uc.submitSem }()
		case <-ctx.Done():
			return task, ctx.Err()
		}
	}
	// 使用动态选择的 provider 提交
	res, err := provider.Submit(ctx, adapter.Endpoint(), body)
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
	uc.recordUsage(ctx, task.TenantID, task.Provider, task.Model, res.Credits)
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
func (uc *GenerationUseCase) recordUsage(ctx context.Context, tenantID, providerName, model string, credits int) {
	if uc.usageRec == nil {
		return
	}
	_ = uc.usageRec.RecordUsage(ctx, entity.UsageRecord{
		TenantID:    tenantID,
		Scene:       "generation",
		LLMConfigName: providerName,  // 使用传入的 provider 名称
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
		// 动态选择 provider
		provider, pErr := uc.getProvider(ctx, t.SubType)
		if pErr != nil {
			continue
		}
		status, pErr := provider.Poll(ctx, t.ProviderTaskID)
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
	if uc.registry == nil || uc.repo == nil {
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
	// 动态选择 provider
	provider, pErr := uc.getProvider(ctx, task.SubType)
	if pErr != nil {
		task.ErrMsg = "自动重试失败: " + pErr.Error()
		_ = uc.repo.Save(ctx, *task)
		return false
	}
	res, err := provider.Submit(ctx, adapter.Endpoint(), body)
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
	uc.recordUsage(ctx, task.TenantID, task.Provider, task.Model, res.Credits)
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
		// 动态选择 provider
		provider, pErr := uc.getProvider(ctx, task.SubType)
		if pErr == nil {
			_ = provider.Cancel(ctx, task.ProviderTaskID)
		}
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
		// 动态选择 provider
		provider, pErr := uc.getProvider(ctx, task.SubType)
		if pErr == nil {
			_ = provider.Cancel(ctx, task.ProviderTaskID)
		}
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
		// BE-ASSET-01：转存失败记录日志，便于排查"任务 success 但无文件"问题
		if uc.asset != nil {
			for i := range st.Creations {
				if stored, sErr := uc.asset.DownloadAndStore(ctx, task.TenantID, st.Creations[i].URL, nil); sErr == nil {
					st.Creations[i].StoredURL = stored
				} else {
					log.Printf("[ApplyStatus][WARN] 转存失败 task=%s url=%s err=%v", task.ID, truncateURL(st.Creations[i].URL), sErr)
				}
			}
			creationsJSON, _ = json.Marshal(st.Creations)
			task.CreationsJSON = string(creationsJSON)
		}
		case entity.TaskStateFailed:
			task.State = entity.TaskStateFailed
			task.ErrCode = st.ErrCode
			// 动态选择 provider 翻译错误
			provider, pErr := uc.getProvider(ctx, task.SubType)
			if pErr == nil {
				task.ErrMsg = provider.TranslateError(st.ErrCode)
			}
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

// UnifiedSubmitInput 统一提交输入（客户端不需要选择端点/模型）。
type UnifiedSubmitInput struct {
	TenantID  string   // 租户ID（从JWT获取）
	BrandID   string   // 品牌ID
	Text      string   // 文本描述
	Materials []string // 素材ID列表
	Template  string   // 模板ID（可选）
	Type      string   // 生成类型（可选：video/image/audio/voice）
	Duration  int      // 时长（可选）
	Quality   string   // 质量（可选）
	// AspectRatio 画面比例（9:16 等——竖版封面/配图必需；此前全链丢弃致恒 16:9）
	AspectRatio string
	// Params 高级参数透传通道（seed/style/voice_setting_* 等白名单 key——
	// selector 的 applyDefaults 出口合并，用户显式值覆盖默认）
	Params map[string]any
	// Refs @引用素材清单（prompt 中 @名称 标记 + 结构化引用——
	// translateRefs 按端点×能力向量翻译为上游参数格式）。
	// BE-GEN-06：统一提交此前不传 Refs，导致 @引用 被静默忽略。
	Refs []entity.PromptRef
	// Watermark/OffPeak 任务级开关（傻瓜式客户端不暴露——管理后台/默认值控制；
	// 此前 SubmitInput 有字段但统一提交无通道，勾选静默无效）
	Watermark bool
	OffPeak   bool
	// SubType 显式端点覆盖（空=selector 按素材自动选择）。当前支持 "subject"：
	// 数字分身主体注册（Vidu /ent/v2/subjects 同步端点——name+形象照+音色）。
	SubType string
}

// submitSubject 数字分身主体注册（/ent/v2/subjects 同步端点）。
// name=Text；形象照取素材图（≤3，URL 直传形态兼容 E4）；voice_id 从 Params。
// 提交即终态：server_id 在返回任务的 creations[0].id——reference2video 复用。
//
// BE-SUBJ-02/03/04/05 修复：
//   - 优先从 Params 直传 images/videos（前端 buildSubjectRegisterPayload 已写）
//   - 再从 materials + asset.List 补全（ID/URL 匹配）
//   - List 失败返回明确错误（不静默吞掉）
//   - asset==nil 时仍可创建（纯 Params 直传模式）
func (uc *GenerationUseCase) submitSubject(ctx context.Context, in UnifiedSubmitInput) (entity.GenerationTask, error) {
	if in.Text == "" {
		return entity.GenerationTask{}, fmt.Errorf("主体名称（text）必填")
	}
	params := entity.GenerationParams{"name": in.Text}

	// ① 优先从 Params 直传（前端 buildSubjectRegisterPayload 已写 images/videos URL）
	if imgs, ok := in.Params["images"].([]string); ok && len(imgs) > 0 {
		params["images"] = imgs
	}
	if vids, ok := in.Params["videos"].([]string); ok && len(vids) > 0 {
		params["videos"] = vids
	}

	// ② 再从 materials + asset.List 补全（ID/URL 匹配）
	if uc.asset != nil && len(in.Materials) > 0 {
		assets, err := uc.asset.List(ctx, in.TenantID, entity.AssetTypeMaterial)
		if err != nil {
			// BE-SUBJ-04：List 失败返回明确错误，不静默吞掉
			return entity.GenerationTask{}, fmt.Errorf("读取素材库失败: %w", err)
		}
		idMap := map[string]bool{}
		for _, id := range in.Materials {
			idMap[id] = true
		}
		var images, videos []string
		for _, a := range assets {
			if !idMap[a.ID] && !containsStrAny(in.Materials, a.SourceURL) {
				continue
			}
			switch a.Type {
			case entity.MaterialTypeImage:
				if len(images) < 3 {
					images = append(images, a.SourceURL)
				}
			case entity.MaterialTypeVideo:
				if len(videos) < 1 {
					videos = append(videos, a.SourceURL)
				}
			}
		}
		// 合并（Params 直传优先，素材库补充）
		if len(images) > 0 {
			if existing, ok := params["images"].([]string); ok {
				params["images"] = append(existing, images...)
			} else {
				params["images"] = images
			}
		}
		if len(videos) > 0 {
			if existing, ok := params["videos"].([]string); ok {
				params["videos"] = append(existing, videos...)
			} else {
				params["videos"] = videos
			}
		}
	}

	// ③ voice_id
	if v, ok := in.Params["voice_id"].(string); ok && v != "" {
		params["voice_id"] = v
	}

	return uc.Submit(ctx, SubmitInput{
		TenantID: in.TenantID, BrandID: in.BrandID,
		SubType: "subject", Model: "", Params: params,
		Watermark: in.Watermark, OffPeak: in.OffPeak,
	})
}

// containsStrAny 列表包含（URL 直传素材匹配用）。
// BE-SUBJ-06：除全等匹配外，增加 URL path 后缀匹配——浏览器所见 URL 的
// base（PUBLIC_BASE_URL / localhost:8082）与素材库 SourceURL 不一致时，
// 按 path 部分（/media/{tenant}/{date}/{file}）匹配仍能命中。
func containsStrAny(list []string, s string) bool {
	sPath := urlPathOf(s)
	for _, x := range list {
		if x == s {
			return true
		}
		// path 后缀匹配（两端都含 /media/ 时比较 path 部分）
		if sPath != "" {
			if xPath := urlPathOf(x); xPath != "" && xPath == sPath {
				return true
			}
		}
	}
	return false
}

// urlPathOf 提取 URL 的 path 部分（含 /media/ 前缀）；非 URL 或无 /media/ 返回空。
func urlPathOf(rawURL string) string {
	if !strings.Contains(rawURL, "/media/") {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Path
}

// ---- 私网素材本地化（本地开发模式：Vidu 云端拉不到 localhost/内网素材 URL）----

// materialURLKeys params 中承载素材 URL 的字段（单值形态）。
var materialURLKeys = []string{"image", "audio_url", "video_url", "start_image", "ref_photo_url", "prompt_audio_url"}

// maxInlineMaterialBytes base64 内联的素材大小上限（Vidu POST body 20MB 限制，
// base64 膨胀 ~1.33x——8MB 文件 ≈ 10.7MB body，留足其他字段余量。
// 超限视频在本地模式明确报错引导配置公网 PUBLIC_BASE_URL）。
const maxInlineMaterialBytes = 8 << 20

// isPrivateHost 判断 URL host 是否私网/环回（厂商云端不可达）。
func isPrivateHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]" {
		return true
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return true // 解析失败按私网处理（保守——本地 hosts 场景）
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// toDataURI 文件字节 → 厂商 data URI（data:<mime>;base64,<payload>——Vidu 的
// images/audio_url/video_url 等字段通用格式）。
func toDataURI(data []byte, mime string) string {
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// localizePrivateMaterials 私网素材 URL → 上游厂商可访问形式（Submit 前统一转换）。
//
// 架构演进（SRP）：URL 可达性判断 + 数据格式转换 已提取为 port.MaterialURLResolver
// 接口（adapter/media/url_resolver.go）。此处委托 urlResolver 处理，Use Case 层
// 只做流程编排。兼容模式：urlResolver 未注入时回落到内置实现。
func (uc *GenerationUseCase) localizePrivateMaterials(ctx context.Context, params entity.GenerationParams) error {
	resolver := uc.urlResolver
	if resolver == nil && uc.asset != nil {
		// 兼容模式：未注入 urlResolver 时使用内置实现（后续迁移完成后移除）
		resolver = &inlineURLResolver{asset: uc.asset, maxBytes: maxInlineMaterialBytes}
	}
	if resolver == nil {
		return nil
	}
	resolve := func(rawURL string) (string, bool, error) {
		return resolver.Resolve(ctx, rawURL)
	}
	for _, k := range materialURLKeys {
		if s, ok := params[k].(string); ok {
			nv, changed, err := resolve(s)
			if err != nil {
				return err
			}
			if changed {
				params[k] = nv
			}
		}
	}
	if arr, ok := params["images"].([]string); ok {
		for i, s := range arr {
			nv, changed, err := resolve(s)
			if err != nil {
				return err
			}
			if changed {
				arr[i] = nv
			}
		}
	}
	if arr, ok := params["images"].([]any); ok { // JSON 反序列化形态
		for i, v := range arr {
			if s, ok := v.(string); ok {
				nv, changed, err := resolve(s)
				if err != nil {
					return err
				}
				if changed {
					arr[i] = nv
				}
			}
		}
	}
	return nil
}

// inlineURLResolver 内置 URL 解析器（兼容模式，后续迁移完成后移除）。
type inlineURLResolver struct {
	asset     port.MediaAssetStore
	maxBytes  int
}

func (r *inlineURLResolver) Resolve(ctx context.Context, rawURL string) (string, bool, error) {
	if rawURL == "" || !isPrivateHost(rawURL) {
		return rawURL, false, nil
	}
	if !strings.Contains(rawURL, "/media/") {
		return rawURL, false, nil
	}
	data, mime, ok := r.asset.ReadLocal(ctx, rawURL)
	if !ok {
		return rawURL, false, nil
	}
	if len(data) > r.maxBytes {
		return "", false, fmt.Errorf("素材 %s 为 %dMB，超出本地内联上限 %dMB——请配置公网可达的 PUBLIC_BASE_URL", truncateURL(rawURL), len(data)>>20, r.maxBytes>>20)
	}
	log.Printf("[LocalizeMaterials][DEBUG] %s → base64 data URI（mime=%s %d字节）", truncateURL(rawURL), mime, len(data))
	return toDataURI(data, mime), true, nil
}

// truncateURL 日志/报错用 URL 截断。
func truncateURL(u string) string {
	if len(u) > 80 {
		return u[:80] + "…"
	}
	return u
}

// SetSettingRepo 注入系统设置仓储（可选；傻瓜式默认值通道）。
func (uc *GenerationUseCase) SetSettingRepo(sr port.SystemSettingRepository) { uc.settingRepo = sr }

// SetTemplateRepo 注入生成模板仓储（可选；模板默认参数通道）。
func (uc *GenerationUseCase) SetTemplateRepo(tr port.TemplateRepository) { uc.templateRepo = tr }

// SetURLResolver 注入素材 URL 解析器（可选；SRP：URL 可达性判断移至 Adapter 层）。
func (uc *GenerationUseCase) SetURLResolver(r port.MaterialURLResolver) { uc.urlResolver = r }

// applyTemplateDefaults 模板默认参数填充（傻瓜式：客户端不传的参数取模板 default_params——
// 此前 template 字段全链零消费，管理后台配的 prompt 前缀/风格等默认全部丢失）。
// 只填未显式指定的 key；模板不存在/未启用静默跳过（提交仍按显式参数走）。
func (uc *GenerationUseCase) applyTemplateDefaults(ctx context.Context, in *UnifiedSubmitInput) {
	if uc.templateRepo == nil || in.Template == "" || in.SubType == "subject" {
		return
	}
	tpl, err := uc.templateRepo.FindByID(ctx, in.Template)
	if err != nil || !tpl.Enabled {
		return
	}
	if in.Params == nil {
		in.Params = map[string]any{}
	}
	for k, v := range tpl.DefaultParams {
		switch k {
		case "duration":
			if in.Duration == 0 {
				if d, ok := v.(float64); ok {
					in.Duration = int(d)
				}
			}
		case "quality", "resolution":
			if in.Quality == "" {
				if q, ok := v.(string); ok {
					in.Quality = q
				}
			}
		case "aspect_ratio":
			if in.AspectRatio == "" {
				if a, ok := v.(string); ok {
					in.AspectRatio = a
				}
			}
		case "type":
			if in.Type == "" {
				if t, ok := v.(string); ok {
					in.Type = t
				}
			}
		default:
			// 其余高级参数（prompt 前缀/风格/运镜等）：未显式指定才采用模板值
			if _, exists := in.Params[k]; !exists {
				in.Params[k] = v
			}
		}
	}
}

// generationBoolDefault 系统设置布尔默认值（"1"/"true" 为真；未配置/无仓储回落 fallback）。
func (uc *GenerationUseCase) generationBoolDefault(ctx context.Context, key string, fallback bool) bool {
	if uc.settingRepo == nil {
		return fallback
	}
	st, err := uc.settingRepo.Get(ctx, key)
	if err != nil || st.Value == "" {
		return fallback
	}
	return st.Value == "1" || st.Value == "true"
}

// UnifiedSubmit 统一提交（傻瓜式：客户端不需要选择端点/模型）。
//
// 流程：
//  1. EndpointSelector根据素材自动选择端点
//  2. 调用原有的Submit方法
//
// 使用场景：
//   - 客户端只需要上传素材、输入文本
//   - 系统自动选择端点、模型、参数
func (uc *GenerationUseCase) UnifiedSubmit(ctx context.Context, in UnifiedSubmitInput) (entity.GenerationTask, error) {
	if uc.endpointSelector == nil {
		return entity.GenerationTask{}, fmt.Errorf("端点选择器未配置")
	}

	// 显式端点：主体注册（数字分身一键创建——server_id 落任务 creations[0].id）
	if in.SubType == "subject" {
		return uc.submitSubject(ctx, in)
	}

	// 显式端点：B-Roll 画面插入合成（本地 ffmpeg——22 号计划；不走端点选择器）
	if in.SubType == "compose" {
		return uc.submitCompose(ctx, in)
	}

	// 模板默认参数填充（管理后台模板的 default_params——未显式指定才采用）
	uc.applyTemplateDefaults(ctx, &in)

	// 傻瓜式默认值：watermark/off_peak 客户端不暴露——未显式指定时取管理后台
	// 系统设置（gen_default_watermark / gen_default_off_peak），均未配置回落 false
	if !in.Watermark {
		in.Watermark = uc.generationBoolDefault(ctx, "gen_default_watermark", false)
	}
	if !in.OffPeak {
		in.OffPeak = uc.generationBoolDefault(ctx, "gen_default_off_peak", false)
	}

	// 1. 构建统一请求
	req := entity.UnifiedGenerationRequest{
		TenantID:    in.TenantID, // 素材查询按租户隔离
		BrandID:     in.BrandID,
		Text:        in.Text,
		Materials:   in.Materials,
		Template:    in.Template,
		Type:        in.Type,
		Duration:    in.Duration,
		Quality:     in.Quality,
		AspectRatio: in.AspectRatio,
		Params:      in.Params,
	}

	// 2. 端点自动选择
	selectResult, err := uc.endpointSelector.Select(ctx, req)
	if err != nil {
		return entity.GenerationTask{}, fmt.Errorf("端点选择失败: %w", err)
	}

	// 3. 调用原有的Submit方法
	// BE-GEN-06：透传 Refs——translateRefs 按端点×能力向量翻译 @引用
	return uc.Submit(ctx, SubmitInput{
		TenantID: in.TenantID,
		BrandID:  in.BrandID,
		SubType:  selectResult.SubType,
		Model:    "", // 空=自动选择
		Params:   selectResult.Params,
		Refs:     in.Refs,
		Watermark: in.Watermark,
		OffPeak:   in.OffPeak,
	})
}

# 03-Vidu 全量接入：统一生成架构与实施计划

> 状态：📋 **计划阶段（未实施）** | 日期：2026-08-12（v1.1 校验补全） | 关联：`Docs/第三方/Vidu/`（官方 API 文档全集）
> 背景：Vidu 提供视频/图片/音频/数字人等多模态生成能力。现有 `provider/vidu.go` 仅支持 text2video/img2video 两个端点、三参数签名、纯轮询——无法承载全量接入。本文档给出整洁架构下的统一生成架构（协议层 + 端点策略 + 能力向量），并明确任务执行演进路径（进程内 → 队列 → 微服务）与 Redis 使用策略。
>
> **v1.1 校验补全**（本版新增）：① 端点全景补 6 个"创建其他任务"端点（主体 API/视频延长/智能超清/模板成片/动作同步/对口型/推荐提示词）；② 回调安全协议细节（HMAC-SHA256 复合签名字符串 + Date 新鲜度 + nonce 防重放）；③ payload 透传关联本地任务；④ 提交后未知状态对齐；⑤ 失败分类与重试策略；⑥ 任务轮询调度设计；⑦ 双配额体系（平台配额 + Vidu 积分对账）；⑧ API Key 管理策略；⑨ 素材与音色资产管理；⑩ 并发节流 / 任务清理 / 产品化参数决策。

---

## 一、Vidu API 事实分析（设计输入）

### 1.1 统一任务模型（最大架构红利）

所有端点共享同一生命周期：`POST 提交 → task_id → 轮询 GET /tasks/{id}/creations 或回调推送 → creations[url]`。

```
状态机：created → queueing → processing → success / failed / cancelled
状态推进：回调（Vidu POST 推送 + 签名验签，失败重试 3 次）＋ 轮询（调度器驱动）——双通道必须幂等合并
```

**回调签名（防伪关键）**：`HMAC-SHA256(secret_key, signingString)`，signingString 由六段拼接（末尾换行）：

```
http_method\n + http_uri\n + canonical_query_string\n + "vidu"\n + Date(GMT)\n + signed_headers(按 X-HMAC-SIGNED-HEADERS 顺序: key:value\n…)
```

请求头：`Date`（GMT）、`x-request-nonce`（随机 UUID 防重放）、`X-HMAC-SIGNED-HEADERS`、`X-HMAC-SIGNATURE`、`X-HMAC-ALGORITHM: hmac-sha256`、`X-HMAC-ACCESS-KEY: vidu`。验签时必须：① 按算法重算比对（格式敏感：无多余空格、末尾换行、URI 以 / 开头、query 无 ?）；② **校验 Date 新鲜度**（如 ±5 分钟，防重放）；③ **nonce 去重表**（短期内存/Redis 集合）。

### 1.2 端点全景（17 个，分五类）

| 分类 | 端点 | 关键约束 |
|---|---|---|
| **视频·主** | text2video / img2video / start-end2video / reference2video / multiframe | 图生恰 1 图；首尾帧恰 2 图（分辨率比 0.8-1.25）；参考生视频有**主体/非主体两套结构**；多帧 2-9 张逐帧 prompt/duration |
| **视频·辅助** | 视频延长 / 智能超清（尊享）/ 模板成片 | 在已有生成物上二次加工；模板成片 = 电商/通用成片解决方案（多素材入参） |
| **图像** | 图片生成（text2image） | — |
| **音频** | text2audio / 可控文生音效 / 声音复刻 / 语音合成 | 声音复刻需音频素材（≤限制）；tts/数字人共用 voice_id（音色列表外部飞书表格） |
| **数字人/其他** | digital-human / 动作同步 / 对口型 / 主体 API / 推荐提示词 | 数字人：图 + (audio_url 或 text+voice_id) 二选一；**主体 API 创建 server_id**（参考生视频可复用）；对口型/动作同步为独立端点 |

**主体 API 是参考生视频的闭环前提**：参考生视频 `subjects[].server_id` 引用主体 API 创建的持久主体（图片/视频/文字主体），或现场传 images 建临时主体——端点策略需支持两种模式。

### 1.3 模型×参数矩阵（三个变化轴正交）

| 变化轴 | 现状 | 变化频率 |
|---|---|---|
| 端点行为 | 17 端点（含辅助端点） | 中（Vidu 持续新增） |
| 模型能力 | viduq1/q1-classic/q2/q2-pro/q2-pro-fast/q2-turbo/q3-pro/q3-turbo/q3-pro-fast/q3-mix/vidu2.0/audio1.0… | **高**（每次出新模型） |
| 参数约束 | 同端点不同模型：duration 范围（1-16/1-10/固定5）、images 数量（1/2/1-7/2-9）、audio 默认值（q3=true 其他 false）、分辨率枚举、aspect_ratio 子集、仅 q2-pro 支持视频参考 | 高（模型×参数矩阵） |

> 设计输入完整性：除 `Vidu端点完整参数限制.md` 外，**必须交叉核对** `Docs/第三方/Vidu/模型参数对照/`（视频/图片/音频三份参数对照表）与各端点文档——能力向量表以此为准，避免单文档缺漏（如 text2video 的 aspect_ratio 与 reference2video 的差异）。

### 1.4 媒体与配额事实

- 输入图片：URL 或 Base64（≤50MB/张，POST body ≤20MB）；视频参考仅 q2-pro（≤100MB）；音频素材（声音复刻）
- 生成物 url/cover_url/watermarked_url **24h 过期**（必须转存）
- 错误码 30+：`CreditInsufficient`（积分不足）/ `TaskPromptPolicyViolation`（提示词风控）/ `QuotaExceeded`（并发超限）/ `TooManyRequests` / `ImageCheckFaceFailed` / `AuditFailed`…
- **积分系统**：创建响应带 `credits`（本次消耗）、`GET 查询积分` 可查余额；off_peak 错峰模式积分更低（48h 内完成）
- **payload 透传**：创建时可传 ≤1MB 任意字符串，查询/回调原样返回——**本地任务关联键**（见 §2.6）
- **watermark**：true/false + wm_position(1-4) + wm_url（自定义水印图）

### 1.5 对文档方案（parameter_schema_json）的评估

文档第八节给出"参数 schema + 端点表"配置化方案。**部分采纳，不整体采用**，四个硬伤：

| 硬伤 | 表现 | 后果 |
|---|---|---|
| ① 条件依赖无法表达 | audio 默认值按模型系列不同、movement_amplitude 仅 q1/2.0 生效 | 配置膨胀成"残缺规则引擎"（when…then…） |
| ② 结构化输入无法表达 | 多帧逐帧 prompt、参考生视频 subjects 嵌套数组 | 平铺 schema 校验器无从下手 |
| ③ 校验≠组装 | 图片 Base64 编码、subjects 结构组装、POST body 限制 | schema 只能校验，组装还得写代码=两套系统 |
| ④ 前端渲染 | 图生 1 图 / 首尾帧 2 图 / 多帧 N 图+逐帧参数的表单差异大 | 通用 schema 渲染器做不出可用表单 |

**结论**：schema 只适合表达"参数长什么样"；"模型能干什么"用类型化能力向量；"端点怎么做"用策略代码。三者各归其位。

---

## 二、目标架构（三层隔离）

### 2.1 分层与依赖方向

```
┌─ 实体层（零依赖）─────────────────────────────────────────────┐
│  GenerationTask     统一生成任务（type/sub_type/model/state/…）  │
│  ModelCapability    模型能力向量（类型化约束，可配置覆盖）         │
│  MediaAsset         媒体资产（素材上传/生成物转存）               │
└───────────────────────────────────────────────────────────────┘
┌─ 用例层（只依赖实体 + 自声明端口）──────────────────────────────┐
│  GenerationUseCase（协议层——写一次，全部端点共享）：              │
│    Submit(spec) / HandleCallback / PollDue / Cancel / Retry    │
│    ↕ port.GenerationProvider   （服务商策略：提交/轮询/取消/验签）  │
│    ↕ port.EndpointRegistry     （端点策略注册表）                 │
│    ↕ port.MediaAssetStore      （素材托管 + 产物转存适配器）       │
│    ↕ port.TaskQueue / TaskLock （任务执行（见 §三））              │
└───────────────────────────────────────────────────────────────┘
┌─ 适配器层 ───────────────────────────────────────────────────┐
│  provider/vidu/                                              │
│    vidu_provider.go    ← 协议层实现（HTTP/轮询/验签/错误翻译/积分查询）│
│    endpoints/          ← 每端点一个策略对象（17 个）              │
│      text2video.go  img2video.go  start_end2video.go          │
│      reference2video.go  multiframe.go  text2image.go         │
│      text2audio.go  tts.go  voice_clone.go  sound_effect.go   │
│      digital_human.go  lip_sync.go  motion_sync.go            │
│      extend.go  upscale.go  template.go  subject.go  suggest_prompt.go │
│    capabilities.go    ← 模型能力向量表（Go struct，编译期安全）    │
│    callback.go        ← 回调验签（HMAC-SHA256 复合串 + nonce 表） │
│  storage/local_media.go   ← 素材托管 + 转存（P2 换 OSS 零改动）    │
│  repository/generation_task_repo.go / media_asset_repo.go     │
└───────────────────────────────────────────────────────────────┘
┌─ 框架层 ─────────────────────────────────────────────────────┐
│  main 装配 + router：POST /api/v1/generation/{type}            │
│                        POST /api/v1/generation/callback        │
│                        POST /api/v1/media/assets（素材上传）     │
└───────────────────────────────────────────────────────────────┘
```

**依赖铁律**：`usecase/generation` 不 import `adapter/provider/vidu`。换服务商 = 新增 provider 包 + main 装配一行，用例零改动。

### 2.2 三个关键抽象

**① 协议层（90% 代码，写一次）**：任务表、状态机、回调验签、轮询调度、转存、错误翻译、积分对账。`ViduProvider` 只实现 `Submit/Poll/Cancel/VerifyCallback/QueryCredits`，端点路径由策略提供。

**② 端点策略（一个对象一个端点，行为差异有归属）**：

```go
// 端口定义（归用例所有）
type EndpointAdapter interface {
    Type() string // "text2video" / "img2video" / "subject" …
    // 该端点对某模型的参数校验——内部直查能力向量 + 结构规则（如首尾帧恰 2 张）
    Validate(ctx context.Context, model string, params GenerationParams) error
    // 组装请求体——图片 Base64/subjects 结构/视频参考/payload 透传（行为，不是数据）
    BuildRequest(ctx context.Context, model string, params GenerationParams) (map[string]any, error)
}
```

**③ 模型能力向量（类型化约束，非裸 JSON schema）**：

```go
// capabilities.go —— 每模型一行；系列公共默认 + 个体覆盖
var text2videoCapabilities = []ModelCapability{
    {Model: "viduq3-pro", Family: "q3", Durations: [2]int{1, 16},
     Resolutions: []string{"540p", "720p", "1080p"}, AudioDefault: true,
     AspectRatios: []string{"16:9", "9:16", "3:4", "4:3", "1:1"}},
    {Model: "viduq2", Family: "q2", Durations: [2]int{1, 10},
     AudioDefault: false, …},
    {Model: "viduq1", Family: "q1", Durations: [2]int{5, 5},
     Resolutions: []string{"1080p"}, …},
}
```

校验是**直查**（`duration ∈ [cap.Durations]`）而非解释执行 JSON 规则；新增模型 = 加一行 struct（编译期安全）；结构化约束（ImageSlots=2 首尾帧、SupportsSubjects、SupportsVideoRef）有类型归属；条件逻辑留在策略代码。

**融合点**：能力向量默认内置于代码；管理后台可对单模型做**配置覆盖**（临时禁用/调参——运营策略热更新），主校验永远走类型化代码，配置只做覆盖不做主逻辑。

### 2.3 模式选型（按需求推导）

| 变化因子 | 模式 | 依据 |
|---|---|---|
| 服务商（Vidu→可灵→即梦） | 策略模式 `GenerationProvider` | 协议层与端点解耦，换商零改动 |
| 端点行为（Vidu 加端点） | 策略模式 `EndpointAdapter` + 注册表 | 新增端点 = 新策略文件 + 注册一行 |
| 模型能力（高频变化） | 能力向量表（代码定义 + 配置覆盖） | 类型安全、直查校验、可热配 |
| 任务生命周期 | 状态机（显式状态 + 幂等推进） | 回调/轮询双通道必须幂等合并 |
| 素材/产物存储（24h URL / 用户上传） | 适配器 `MediaAssetStore` | 本地先行，OSS 后换 |
| 17 端点统一入口 | 门面 `GenerationUseCase.Submit` | 调用方只见一张脸 |
| 任务执行（进程内→队列→微服务） | 模板方法 + 端口预留（见 §三） | 推迟决策，装配层演进 |

### 2.4 数据模型（迁移 038）

```sql
generation_tasks (
  id, tenant_id, brand_id,
  type,                -- video/image/audio/digital_human/other
  sub_type,            -- 17 端点枚举
  model, provider, provider_task_id,
  state,               -- created/queueing/processing/success/failed/cancelled
  err_code, err_msg,   -- 翻译后的产品级消息（含可重试标记）
  params_json,         -- 完整提交参数（幂等重放）
  payload,             -- 透传给 Vidu 的本地关联键（本地 task_id）
  creations_json,      -- [{url, cover_url, watermarked_url, stored_url, stored_at}]
  credits, off_peak, watermark,
  callback_received, callback_at,
  retry_count,         -- 重试次数（失败分类后自动重试）
  created_at, updated_at, finished_at
)

generation_specs (     -- 端点/模型注册表（管理后台可维护，含启用开关）
  sub_type, model, endpoint, enabled,
  capabilities_json,   -- 能力向量 JSON（管理后台覆盖源；代码为默认值）
  updated_at
)

media_assets (         -- 素材与产物资产（v1.1 补全）
  id, tenant_id, brand_id, owner_type,  -- material/creation
  source_url, stored_url, mime, size_bytes,
  meta_json,           -- 音色 voice_id / 主体 server_id 关联
  created_at, expires_at
)
```

### 2.5 回调安全协议（v1.1 补全）

```
Vidu POST /api/v1/generation/callback
  → 读取 X-HMAC-SIGNATURE / X-HMAC-SIGNED-HEADERS / Date / x-request-nonce
  → ① Date 新鲜度校验（±5 分钟，防重放）
  → ② nonce 去重（内存 TTL 集合——5 分钟内重复 nonce 拒绝）
  → ③ 按 signingString 六段格式重算 HMAC-SHA256，比对
  → ④ 通过后按 payload（本地 task_id）定位任务，幂等推进状态机
```

验签失败返回 4xx（不落库）；成功返回 200（Vidu 收到后停止重试）。**回调 URL 公网不可达时（本地开发/内网）自动降级纯轮询**（callback_url 不传或传空——Vidu 允许可选）。

### 2.6 payload 透传关联（v1.1 补全）

创建任务时把**本地 task_id** 写入 `payload` 字段（≤1MB）→ 回调/查询响应原样返回 → 回调处理免查表直接定位（O(1) 且天然抗 provider_task_id 漂移）。查询轮询仍按 provider_task_id 兜底。

### 2.7 提交后未知状态对齐（v1.1 补全）

HTTP 超时/网络断 → 任务可能已在 Vidu 侧创建（无幂等键）：
- 提交前：查同租户同 `sub_type+model+params_hash` 的 pending 任务 → 存在则复用（防重复提交）
- 提交超时：以本地 task_id 进入轮询队列，按 provider_task_id 未知则**先查询对齐**（GET tasks/{id} 不存在 → 才允许重试新建）

---

## 三、任务执行策略与微服务演进路径（避免过度设计）

### 3.1 现状与目标

现状 `usecase/video` pipeline 是**进程内 goroutine**（重启丢任务、无横向扩展）。目标：任务表持久化 + 调度驱动。

### 3.2 演进路径（三步走，每一步都是"换适配器"而非重构）

```
阶段 1（本轮实施，单机）        阶段 2（单机瓶颈，多实例）       阶段 3（真需要独立部署）
┌───────────────────────┐   ┌──────────────────────────┐   ┌──────────────────────────┐
│ GenerationUseCase     │   │ GenerationUseCase         │   │ generation 独立服务        │
│   ├ 任务表持久化        │   │   ├ 任务表持久化           │   │   （代码已按边界切好：     │
│   ├ 轮询 ticker 池     │ → │   ├ TaskQueue→Redis 实现  │ → │    usecase 不依赖框架，    │
│   ├ 进程内执行器        │   │   ├ TaskLock→RedisLock    │   │    复制即走）             │
│   └ goroutine 池       │   │   └ 多实例消费队列         │   └ 独立部署/独立扩缩容       │
└───────────────────────┘   └──────────────────────────┘   └──────────────────────────┘
```

- **现在不做微服务拆分**：单机起步阶段，生成任务吞吐受 Vidu 侧限流约束（非计算瓶颈），拆分无收益只有成本
- **边界现在画对**：`TaskQueue` / `TaskLock` 端口已存在（内存实现就绪），阶段 2 换 Redis = 装配层一行——"推迟决策"
- 阶段 3 触发条件（满足任一再拆）：独立容量瓶颈 / 独立扩缩容需求 / 多团队并行冲突

### 3.3 轮询调度设计（v1.1 补全——scheduler 6h 间隔不适用）

生成任务分钟级完成，6h 级 scheduler 不适用。阶段 1 用**任务轮询 ticker 池**：

```
PollDue 驱动：每 15-30s 扫描 processing/queueing 状态任务（限 N 条/轮，防打爆 Vidu）
  → 超时保护：超过模型最大预估时长×2 仍未终态 → 标记 stalled 并告警（不无限轮询）
  → 退避：连续失败（TooManyRequests）按指数退避，回调到达后停止轮询
```

轮询间隔/并发数可配（`generation_specs` 或 env）。回调与轮询都推进同一状态机（幂等）。

### 3.4 失败分类与重试策略（v1.1 补全）

| 分类 | 错误码示例 | 策略 |
|---|---|---|
| **可自动重试** | TooManyRequests / InternalServiceFailure / 网络超时 / OperationInProcess | 指数退避重试（1/5/30 分钟，≤3 次），retry_count 落库 |
| **可人工重试** | CreditInsufficient / AuditFailed / TaskPromptPolicyViolation | 终态 failed + 产品级提示（充值/改提示词），前端"重新生成"= 新任务 |
| **不可重试** | ImageCheckFaceFailed / MultiFaceDetected / VideoFormatInvalid | 终态 failed + 明确素材问题提示（换图/换音频），不消耗用户操作成本 |

### 3.5 双配额体系与计费（v1.1 补全）

- **平台配额**：generation 新场景（`usages.scene=generation` 按次计数）——免费/专业/团队配额分级（产品定价另行决定），超限 402
- **Vidu 积分**：`ViduProvider.QueryCredits` 查询余额；创建响应 `credits` 回写任务表；**对账**：每日/手动汇总 `Σcredits` vs 平台成本（billing CostAnalysis 增加 generation 场景 + credits 字段）
- **前端展示**：任务列表显示"本次消耗 X 积分"；余额不足时创建前拦截（调 QueryCredits）

### 3.6 Redis 使用策略（v1.1 明确）

| 场景 | 该用吗 | 说明 |
|---|---|---|
| 任务队列（阶段 2） | ✅ 第一候选 | 投递/消费解耦；消费幂等以任务表 state 为准 |
| 分布式锁（阶段 2） | ✅ 启用前修 token 校验 | 现 RedisLock.Unlock 用 Del 无校验会误删锁——启用时改 Lua（GET 比对后 DEL） |
| 任务状态存储 | ❌ | MySQL 事务化状态机即可，Redis 双写有分歧风险 |
| 生成物缓存 | ❌ | 24h URL 转存为文件，无缓存热点 |
| 回调 nonce 去重表 | ⚠️ 阶段 1 内存 TTL 即可 | 多实例时换 Redis Set（单机内存够用） |

**当前正确姿势**：单机 NoopLock + 内存队列（最小正确系统）；go.mod redis 依赖（现为 indirect）在阶段 2 启用前保持"预留"状态，避免僵尸依赖（要么启用要么删除——建议保留并在 README 标注启用时机）。

### 3.7 API Key 管理（v1.1 补全）

- **阶段 1**：全局 env `VIDU_API_KEY`（单租户共享，参照现有 VIDU_API_KEY 先例）
- **阶段 2**：租户级 key 落库（`tenant_settings`）→ provider 每次调用取租户 key（空则回落全局）——参照 LLMConfig 30s TTL 缓存先例热更新
- **安全**：Key 只存后端，永不下发前端；回调验签密钥 = 创建任务时所用 key（租户级回调需按租户验签——验签器按 payload 中租户查 key）

---

## 四、实施步骤

### P0：骨架（协议层 + 视频 5 端点 + 数字人 + 主体 API 打样）

| 事项 | 涉及 | 工作量 |
|---|---|---|
| 实体：GenerationTask / ModelCapability / GenerationParams / MediaAsset | `domain/entity/generation.go` | M |
| 端口：GenerationProvider / EndpointAdapter / EndpointRegistry / MediaAssetStore | `usecase/port/generation.go` | M |
| 用例：GenerationUseCase（Submit/Callback/Poll/Cancel/Retry + 状态机 + 失败分类 + 幂等） | `usecase/generation/` | L |
| 端点策略：text2video / img2video / start_end2video / reference2video / multiframe / digital_human / subject（主体 API） | `adapter/provider/vidu/endpoints/` | L |
| 能力向量表（q1/q2/q3/2.0 全系列 × 已接端点；交叉核对模型参数对照表） | `adapter/provider/vidu/capabilities.go` | M |
| ViduProvider 协议层重写（提交/payload 透传/轮询/取消/积分查询/错误码翻译表） | `adapter/provider/vidu/vidu_provider.go` | M |
| 回调验签（HMAC-SHA256 复合串 + Date 新鲜度 + nonce 去重）+ 回调路由 + 幂等推进 | `adapter/provider/vidu/callback.go` + handler | M |
| 迁移 038（generation_tasks + generation_specs + media_assets） | `migrations/038_generation.sql` | S |
| 轮询 ticker 池（15-30s + 超时保护 + 退避） | `usecase/generation/poller.go` | M |
| 提交防重（params_hash 查重）+ 未知状态对齐 | 用例层 | S |
| Mock 增强（MockProvider 按能力向量模拟进度 + 回调模拟，前端可演示） | `adapter/provider/mock_generation.go` | S |

### P1：辅助端点 + 音频全端点 + 素材/转存

| 事项 | 涉及 | 工作量 |
|---|---|---|
| 端点策略：extend / upscale / template / lip_sync / motion_sync / suggest_prompt | 同上 | M |
| 端点策略：text2image / text2audio / tts / voice_clone / sound_effect | 同上 | M |
| 素材上传：POST /api/v1/media/assets（用户传图/音频 → 托管 URL → 供 Vidu 引用） | handler + storage | M |
| 音色/声音资产管理（voice_id 列表、复刻记录） | 用例层 + media_assets 表 | S |
| 转存：LocalMediaStore（下载 24h URL → 本地静态目录，重试 3 次 + 失败标记"产物过期"） | `adapter/storage/local_media.go` | M |

### P2：前端 + 管理后台

| 事项 | 涉及 | 工作量 |
|---|---|---|
| 创作工作台：视频/图片/音频/数字人 Tab，能力向量驱动表单（时长/分辨率/图片槽位/主体选择）；**替代老视频工作台路由（/m/video → /m/creation）** | `web/src/pages/merchant/Creation.tsx` | L |
| 任务列表：状态/积分消耗/错误提示（分类图标）/重新生成按钮 | 同上 | M |
| 管理后台：generation_specs 配置页（端点/模型/能力覆盖/启用开关）+ API Key 管理 + 积分对账报表 | `web/src/pages/admin/Generation.tsx` | M |
| 回调签名密钥管理 + 回调日志 | 后端 + admin 页 | S |

### P3：收尾 + 预研

| 事项 | 涉及 | 工作量 |
|---|---|---|
| 老 video_tasks 迁移（双写兼容期 → 单写） | repo + 用例 | M |
| 解决方案级工作流（一键成片/MV/电商成片——多任务编排：图→视频→音频→合成） | `usecase/generation/workflow.go` | L |
| 提示词优化器（参照 Vidu-提示词工程总结 + GEO 内容格式模板——文生视频 prompt 模板/优化） | 用例层 + admin 模板管理 | M |
| 任务清理策略（generation_tasks 保留 30 天 + creations 文件清理，定时任务） | scheduledtask | S |
| 并发节流（提交信号量，防 Vidu QuotaExceeded） | 用例层 | S |
| 阶段 2 预研：TaskQueue Redis 实现 + RedisLock token 校验（写测试不启用） | `adapter/queue/redis_queue.go` | M |

---

## 五、产品化参数决策（v1.1 新增——需产品确认）

| 参数 | 决策点 | 建议默认 |
|---|---|---|
| watermark | 免费版强制水印？付费去水印？ | 免费=强制（wm_position=3 默认水印）；pro/team 可选 |
| off_peak | 用户可选省积分模式（48h 完成）？ | 默认 false（即时）；高级选项暴露 |
| credits 展示 | 任务列表/详情显示消耗积分 | 显示 + 余额不足前置拦截 |
| 素材留存 | 用户上传素材保留多久 | 30 天（与任务一致） |

---

## 六、验收口径

1. **全端点可提交**：17 端点（含辅助端点），Mock/Vidu 双 provider 任务正常流转
2. **参数校验正确**：错误参数返回产品级消息（"viduq1 仅支持 1080p"）
3. **双通道幂等**：回调先到 + 轮询后到（或反向），状态只推进一次、终态不变
4. **回调安全**：篡改签名/过期 Date/重复 nonce 均拒绝；正常回调成功推进
5. **失败分类**：TooManyRequests 自动退避重试；风控/积分不足终态+可读提示
6. **产物不丢**：success 后转存成功，24h 后仍可访问；转存失败标记"产物过期"
7. **防重复**：同参数 pending 任务复用；提交超时先对齐后重试
8. **积分对账**：任务 credits 落库，报表可按租户汇总
9. **前端可用**：创作工作台按所选模型渲染正确表单；老视频页路由迁移后行为不变
10. **回归**：老 video 工作台行为不变（P3 迁移前）

---

## 附录：文档校验记录（v1.1）

| # | 校验发现 | 补充位置 |
|---|---|---|
| 1 | "创建其他任务"6 端点遗漏（主体 API 是参考生视频闭环前提） | §1.2 / P0-P1 |
| 2 | 回调签名算法细节（HMAC 复合串/Date/nonce）未展开 | §1.1 / §2.5 |
| 3 | payload 透传关联本地任务缺失 | §2.6 |
| 4 | 提交超时后未知状态未处理 | §2.7 |
| 5 | 失败分类与重试策略缺失 | §3.4 |
| 6 | 轮询调度设计缺失（scheduler 6h 不适用） | §3.3 |
| 7 | 双配额（平台 quota + Vidu 积分对账）缺失 | §3.5 |
| 8 | API Key 管理策略缺失 | §3.7 |
| 9 | 素材/音色资产管理缺失 | P1 / §2.4 media_assets |
| 10 | 并发节流/任务清理/产品化参数缺失 | P3 / §五 |
| 11 | 模型参数对照表未纳入设计输入 | §1.3 |
| 12 | 提示词工程（Vidu 提示词总结）未规划 | P3 |

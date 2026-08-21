// 后端 DTO 的 TypeScript 镜像（通用平台版）。

// ---- 统一响应信封 ----
export interface ApiEnvelope<T> {
  code: number
  msg: string
  data?: T
}

// ---- 认证 ----
export interface LoginRequest { username: string; password: string }
export interface RegisterRequest { username: string; password: string; role?: string; tenant_id?: string }
// 登录响应含用户身份（前端据此分流到商户端/管理端）
export interface LoginResponse {
  token: string
  role: 'admin' | 'merchant'
  tenant_id: string
  username: string
  must_change_password?: boolean // 仍在用默认口令（F1-5）——管理端常驻提醒改密
}
export interface RegisterResponse { user_id: string; role: string; tenant_id: string }

// ---- 任务 ----
export interface EnqueueTaskRequest { type: string; input: unknown }
export interface EnqueueTaskResponse { task_id: string }

// ---- 聊天 ----
export interface ChatMessage { role: string; content: string }

// ---- Agent 配置 ----
export interface AgentConfig {
  name: string
  system_prompt: string
  tools: string[]
  llm_config_name?: string  // 引用的 LLMConfig.name（留空用 default）
  max_iterations?: number
}

// ---- LLM 配置（独立聚合根，多厂商多模型；admin 视图，含密钥）----
export interface LLMConfig {
  name: string       // 唯一标识，如 "default"、"minimax-m2"
  provider: string   // 厂商标签：minimax/openai/zhipu/deepseek
  api_key: string
  base_url: string   // 如 https://api.minimaxi.com/v1
  model: string      // 如 MiniMax-M2.5
  cost_per_mtok: number // 每百万 tokens 参考成本（分；成本分析按引擎细分）
  usage?: string     // 用途：""=聊天模型（默认），"vision"=视觉模型（浏览器截图分析）
}

// ---- 引擎名单（商户端可见——仅展示字段，不含厂商密钥）----
export interface EngineOption {
  name: string
  provider: string
  model: string
}

// ---- 聊天会话（后端持久化，按用户隔离）----
export interface Conversation {
  id: string
  title: string
  agent_name?: string
  user_id?: string
  created_at?: string
  updated_at?: string
}

// ---- 聊天消息（后端持久化）----
export interface ChatMessageRecord {
  id: string
  conversation_id: string
  role: string                 // user / assistant
  content: string              // assistant 含 <think> 原文，前端 parseBlocks 还原
  tool_calls?: string          // 工具调用块 JSON（对应前端 blocks/tools）
  created_at?: string
}

// ---- 采集配置（运行时可调）----
export interface CrawlConfig {
  request_interval_ms: number  // 请求间隔（毫秒）
  request_timeout_ms: number   // 单请求超时（毫秒）
  max_retries: number          // 最大重试次数
  respect_robots: boolean      // 是否遵守 robots.txt
}

// ---- 外部推送系统 ----
export interface ExternalSystem {
  name: string
  description?: string
  endpoint: string             // 完整 API 地址
  method?: string              // HTTP 方法，默认 POST
  headers?: string             // 请求头 JSON，如 {"X-API-Key":"xxx"}
  mode?: string                // 推送模式：raw（原样转发）/ mapping（字段映射）
  field_mapping?: string       // mapping 模式的字段映射 JSON
  body_template?: string       // raw 模式的请求体示例（用于 UI 提示）
  content_type?: string        // 接收的数据类型标记
  enabled?: boolean
  updated_at?: string
}

export interface PublishResult {
  success: boolean
  external_id?: string
  error?: string
}

export interface PublishRecord {
  id: string
  system_name: string
  success: boolean
  external_id?: string
  error_msg?: string
  result_at?: string
}

// ---- 工具面板 ----
export interface ToolView {
  name: string
  description: string
  enabled: boolean
}

// ---- 平台总览统计 ----
export interface StatsView {
  // 平台规模（SaaS 总览数字卡片）
  users: number               // 平台商户总数
  brands: number              // 品牌资产总数
  keywords: number            // 关键词总数
  monitor_results: number     // 监测结果总数（累计探测）
  optimized_contents: number  // 优化内容总数
  published_contents: number  // 已发布公开内容数
  publish_jobs: number        // 发布任务总数
}

// ---- GEO：品牌资产 ----
export interface Brand {
  id: string
  tenant_id: string
  name: string
  positioning: string
  core_selling: string[]
  competitors: string[]
  biz_type?: string  // local（本地生意，默认）/ online（线上业务）
  industry?: string  // 行业（如 餐饮/美业/线上服务）——知识库素材检索的过滤维度
  website_url?: string  // 官网地址（online 品牌 NAP）
  created_at: string
}

// ---- 平台知识库（Docs/Plans/04：按行业采集素材 + 向量检索溯源）----
export interface KnowledgeEmbeddingConfig {
  model: string        // 嵌入模型（如 embedding-3）
  base_url: string     // OpenAI 兼容端点（如 https://open.bigmodel.cn/api/paas/v4）
  api_key: string
  dimensions: number   // 向量维度（0=模型默认；智谱 embedding-3 默认 2048，可设 256-2048）
  vector_db: string    // mysql（默认）/ milvus
  milvus_host: string
  milvus_port: string
  milvus_collection: string
  updated_at: string
}

export interface IndustryCrawlConfig {
  industry: string
  keywords: string[]   // 采集关键词组（每组一轮搜索）
  per_round: number    // 每轮每关键词入库上限
}

export interface KnowledgeMaterialView {
  id: string
  industry: string
  title: string
  source_url: string
  summary: string
  crawl_keyword: string
  status: string
  has_vector: boolean
  created_at: string
}

export interface KnowledgeStats {
  total_materials: number
}

export interface KnowledgeCrawlInterval {
  interval_minutes: number // 采集间隔（分钟，30-1440；默认 360=6h）
}

// 竞品推荐候选（附近同行 POI 按评分/距离排序）
export interface CompetitorSuggestion {
  name: string
  rating: number
  distance_m: number
  address: string
  category: string
}

// ---- GEO：门店档案（本地生活地基）----
export interface StoreLocation {
  id: string
  tenant_id: string
  brand_id: string
  name: string
  address: string
  city: string
  district: string
  adcode: string
  lat: number
  lng: number
  phone: string
  hours: string
  price_level: string
  biz_type: string          // LocalBusiness/Restaurant/Cafe/Bar/Store
  geo_status: string        // pending/ok/failed
  has_geo: boolean
  created_at: string
  updated_at: string
}

// ---- GEO：附近同行双榜（现实世界地图榜 + AI 竞品榜）----
export interface MapRankEntry {
  name: string
  address: string
  distance_m: number
  rating: number            // 0=无评分数据
  category: string
  open_status: string
  lat: number
  lng: number
  // 门店卡扩展（v5 show_fields=business,navi）
  city_name?: string
  ad_name?: string
  cost?: string             // 人均消费
  business_area?: string    // 商圈
  open_time_today?: string  // 今日营业时间
  tag?: string              // 特色菜（美食 POI）
  tel?: string              // 电话
  entr_location?: string    // 入口经纬度
  photo_url?: string        // 首张照片
  // 驾车耗时（P2 距离测量补全；0=未测得）
  drive_distance_m?: number   // 驾车距离（米）
  drive_duration_sec?: number // 驾车耗时（秒）
}

export interface AIRankEntry {
  name: string
  rate: number              // 竞品平均提及率（0~1）
  sample_cnt: number
  mentioned?: boolean       // 是否被 AI 提及（false=未上榜——全量补位）
  mention_cnt?: number      // 提及次数（探查口径）
  is_own?: boolean          // 是否自己的品牌（金色高亮 + 我的品牌标签）
}

export interface NearbyRanking {
  store: StoreLocation | null
  map_ranking: MapRankEntry[]
  ai_ranking: AIRankEntry[]
  own_rate: number          // 自己的 AI 提及率（-1=无数据）
  map_available: boolean
  search_keyword: string
  // AI 榜来源与覆盖（v2：AI 榜单探查——全量补位 + 上榜率）
  ai_rank_from_probe?: boolean  // true=来自探查；false/缺省=旧逻辑（监测竞品提及率）
  ai_rank_probed_at?: string    // 探查时间
  ai_rank_total?: number        // 附近同行总数
  ai_rank_mentioned?: number    // 被 AI 提及数（上榜率 = mentioned/total）
  ai_rank_sample?: number       // 探查采样次数
}

// ---- GEO：关键词 ----
export interface Keyword {
  id: string
  tenant_id: string
  brand_id: string
  term: string
  intent: string
  created_at: string
}

// ---- GEO：监测结果 ----
export interface MonitoringResult {
  id: string
  tenant_id: string
  brand_id: string
  keyword_id: string
  engine_name: string
  sample_count: number
  mention_count: number
  mention_rate: number       // 0~1
  avg_position: number
  sentiment: string          // positive/neutral/negative
  competitors: string[]
  competitor_rates: Record<string, number> // 竞品提及率（对比坐标系）
  competitor_sentiments: Record<string, string> // 竞品情感（positive/neutral/negative——对标视图语义维度）
  confidence: number
  probed_at: string
  raw_sample: string
  sources?: string[]         // 引用来源（链接/平台名，P5-01 归因）——旧数据可能缺失，消费端 || [] 兜底
  self_source_count?: number // 自营公开站被引用次数（>0 = 内容真的被 AI 引用）——旧数据可能缺失，按 0 兜底
  first_pick_count?: number  // 被提及且位次=1 的采样数（首选率分子）——旧数据缺失按 0
  semantic_degraded?: boolean // 采样中出现过解析降级（情感/位次可能失真）——旧数据缺失按 false
}

// ---- GEO：健康报告（后端聚合单一事实源；前端 geoHealth 为降级兜底）----
export interface HealthIndicatorView {
  mention_coverage: number
  sentiment_score: number
  first_pick_rate: number
  content_asset: number
  source_integrity: number
}

export interface CompetitorThreatView {
  name: string
  avg_rate: number  // 0-100
  sentiment: string // positive/negative/''（中性）
}

export interface HealthReportView {
  total: number
  indicators: HealthIndicatorView
  prev_total: number | null
  has_prev: boolean
  competitor: {
    self_avg: number // 0-1
    comp_avg: number // 0-1
    gap_pct: number  // 百分点（+领先/-落后）
    size: number
    threats: CompetitorThreatView[] // 按提及率降序
  }
  brands: Array<{
    brand_id: string
    brand_name: string
    total: number          // 与总分同口径（三处展示位统一）
    avg_mention_rate: number // 0-1
  }>
}

// ---- 行业全景看板（admin：跨商户聚合，v3 P2）----
export interface IndustryOverviewView {
  industries: Array<{ industry: string; avg_rate: number; brand_count: number }> // 按平均提及率降序
  reputation: Array<{ brand_name: string; industry: string; positive_rate: number; sample_count: number }> // 按正面占比降序
  top_sources: Array<{ domain: string; count: number }> // 按被引次数降序
}

// ---- GEO：行动建议（P5-05）----
export interface Advice {
  level: 'high' | 'medium' | 'low'
  message: string
  page: string
}

// ---- GEO：品牌监测总览 ----
export interface BrandOverview {
  brand_id: string
  brand_name: string
  avg_mention_rate: number
  keyword_count: number
  last_probed_at: string
  trend: MonitoringResult[]
}

// ---- GEO：优化内容 ----
export interface GeoScore {
  total: number
  authority: number
  specificity: number
  structure: number
  uniqueness: number
  recency: number
}

export interface OptimizedContent {
  id: string
  tenant_id: string
  brand_id: string
  keyword_id: string
  title: string             // 内容标题（发布到平台用）
  original_text: string
  optimized_text: string
  version: number
  score: GeoScore
  // 优化模式下的前后对比反馈（optimize 接口返回；generate 无）
  score_before?: GeoScore
  recommendations?: string[]
  status: string
  index_status?: string  // 收录状态：pending（已提交未收录）/ indexed（已收录）/ error（查询失败）
  indexed_at?: string    // 收录确认时间
  created_at: string
}

// ---- 热门同款视频（人设档案 tab：LLM+搜索发现）----
export interface HotVideo {
  title: string
  url: string
  platform: string      // douyin / kuaishou / xiaohongshu / bilibili / web
  hot_point: string     // 为什么火（可抄的点）
  topic: string         // 拍摄同款选题建议（预填创作）
}

// ---- 作品数据页聚合（/m/analytics 数据源）----
export interface WorkSummaryItem {
  job_id: string
  title: string
  platform: string
  content_type: string  // video / image / article
  status: string
  external_url: string  // 视频链接（空=手动发布未追踪）
  published_at: string
  views: number
  likes: number
  comments: number
  shares: number
}

export interface AnalyticsTrendPoint {
  day: string           // MM-DD
  published: number
  views: number
}

export interface AnalyticsSummary {
  totals: { published: number; views: number; likes: number; comments: number }
  trend: AnalyticsTrendPoint[]
  works: WorkSummaryItem[]
}

// ---- GEO：平台账号（扫码绑定 / 官方 OAuth 授权）----
export interface Account {
  id: string
  tenant_id: string
  platform: string         // zhihu / xiaohongshu / ...
  display_name: string
  health: string           // active / expired / banned
  login_method: string     // zhihu / wechat / qq / weibo
  auth_type?: string       // cookie（浏览器通道）/ oauth（官方 API 通道）
  expires_at: string       // 凭据过期时间（cookie 或 access_token）
  bound_at: string
  last_used_at: string
}

// ---- GEO：发布任务 ----
export interface PublishJob {
  id: string
  account_id: string
  platform: string
  content_id: string
  brand_id: string
  title: string
  mode: string             // semi-auto / auto
  status: string           // pending / running / published / failed
  content_type?: string    // video / image / article（详情 Drawer 形态展示）
  external_url: string
  error_msg: string
  created_at: string
  published_at: string     // 发布成功时间
  pre_mention_rate: number  // 发布前提及率
  post_mention_rate: number // 发布后提及率
}

// ---- 用户管理（管理端）----
export interface UserView {
  id: string
  username: string
  role: string
  tenant_id: string
  brand_count?: number  // F3-1 运营聚合：该租户品牌数
  last_active?: string  // 最近一次监测时间（空=从未使用——沉睡商户信号）
}

// ---- GEO：AI 榜缓存条目（F4 品牌卡徽章——只读缓存不烧配额）----
export interface AIRankItemView {
  name: string
  rate: number
  mentioned: boolean
  avg_pos: number
}


// ---- 收录管理（管理后台）----
export interface IndexingSubmitLog {
  id: string
  channel: string        // indexnow / baidu / all
  url: string
  status: string         // success / failed
  error_msg: string
  submitted_at: string
}

// ---- 统一生成（Vidu 全量接入：视频/图片/音频/数字人）----

// 模型能力向量（端点×模型矩阵——DB 驱动，管理后台可热改）
export interface ModelCapability {
  model: string
  family: string
  endpoint?: string
  durations: [number, number]     // 时长范围 [min,max]（0 表示不支持自定义）
  resolutions?: string[]
  aspect_ratios?: string[]
  audio_default?: boolean
  audio_types?: string[]
  image_slots?: number            // 图片槽位：0=不需要 1=单图 2=双图 -1=动态(1-7)
  video_slots?: number
  supports_bgm?: boolean
  supports_subjects?: boolean
  supports_movement?: boolean
  max_prompt_len?: number
}

export interface GenerationType {
  sub_type: string               // 端点类型（text2video/img2video/…）
  models: Array<{
    model: string
    capability: ModelCapability
  }>
}

// 发布通道能力清单（服务端 ChannelRegistry 导出——发布页能力驱动的数据源）
export interface PublishChannelConstraints {
  title_max_runes?: number // 标题最大字数（0/缺省=不限）
  min_images?: number      // 最少配图数（0/缺省=不要求）
  min_videos?: number      // 最少视频数（0/缺省=不要求）
}
export interface PublishChannelView {
  platform: string
  name: string
  content_types: string[]                      // article/image/video/audio
  constraints?: Record<string, PublishChannelConstraints> // key=内容形态
  semi_auto: boolean
  auto: boolean
}

// 生成模式开关状态（admin：sub_type 聚合）
export interface GenerationModeView {
  sub_type: string
  tier: 'default' | 'advanced' | 'closed' // 推荐档位
  enabled: boolean                        // 全部模型启用
  partial: boolean                        // 部分模型启用
  model_count: number
}

// 生成规格（管理后台：Vidu 端点×模型矩阵——DB 驱动 30s 热生效）
export interface GenerationSpec {
  sub_type: string
  model: string
  endpoint: string
  enabled: boolean
  capabilities_json: string
  has_override?: boolean
  updated_at?: string
}

export interface GenerationCreation {
  id: string
  url: string
  cover_url?: string
  watermarked_url?: string
  stored_url?: string            // 转存后的永久 URL（无 = 未转存）
}

export interface GenerationTask {
  id: string
  tenant_id: string
  brand_id: string
  type: string                   // video / image / audio / digital_human / other
  sub_type: string
  model: string
  provider: string
  provider_task_id: string
  state: string                  // created / queueing / processing / success / failed / cancelled
  err_code: string
  err_msg: string
  params: Record<string, unknown>
  creations: GenerationCreation[]
  credits: number
  off_peak: boolean
  watermark: boolean
  retry_count: number
  created_at: string
  finished_at: string | null
}

// 媒体资产（素材上传 + 产物转存）
export interface MediaAsset {
  id: string
  tenant_id: string
  brand_id: string
  owner_type: string             // material / creation
  url: string                    // 可访问 URL（素材列表/上传响应契约）
  mime: string
  size_bytes: number
  created_at: string
}

// 提示词 @引用（客户端从素材库选择 → 服务端翻译层按端点格式映射）
export interface PromptRef {
  id: string
  name: string                   // 素材名（prompt 中 @名称 标记）
  url: string
  kind: 'image' | 'audio' | 'video'
}

// 厂商配置（管理后台按厂商管理）
export interface ProviderConfig {
  provider: string
  api_key: string                // 掩码（管理后台展示）
  has_key: boolean
  base_url: string
  enabled: boolean
  updated_at: string
}

// ---- 经济系统（订阅 / 计费 / 配额）----

export interface Plan {
  id: string
  name: string
  level: string                // free / pro / team
  price_cents: number          // 月费（分）
  quotas: Record<string, number>  // 场景→配额（-1=无限）
  features: string[]
  status: string               // active / archived
  sort_order: number
  created_at?: string          // 仅响应返回；保存请求不发送（后端维护）
  updated_at?: string          // 仅响应返回；保存请求不发送（后端维护）
}

export interface Subscription {
  id: string
  tenant_id: string
  plan_id: string
  status: string               // active / expired / cancelled
  period_start: string
  period_end: string
  created_at: string
  updated_at: string
}

export interface Order {
  id: string
  tenant_id: string
  plan_id: string
  amount_cents: number
  status: string               // pending / paid / refunded / failed
  payment_gateway: string
  payment_id: string
  created_at: string
  paid_at: string
}

export interface RevenueSummary {
  total_revenue_cents: number
  month_revenue_cents: number
  paid_orders: number
  active_subscriptions: number
  plan_distribution: Record<string, number>
}

// ---- X-01 成本分析（admin 报表：收入 vs 成本双报表）----
export interface SceneCost {
  scene: string
  calls: number            // LLM 调用次数（或业务动作计数）
  total_tokens: number     // token 总量（非 LLM 场景为 0）
  est_cost_cents: number   // 估算成本（分）
}

export interface CostAnalysis {
  days: number
  per_m_token_cents: number  // 参考单价（分/百万 tokens）
  scenes: SceneCost[]
  total_calls: number
  total_tokens: number
  total_cost_cents: number   // 估算总成本（分）
}

export interface UsageEntry {
  limit: number                // -1=无限
  used: number
}

export interface MyUsageSummary {
  subscription: Subscription | null
  plan: Plan
  usages: Record<string, UsageEntry>
}

// ---- 地址联想（P1 输入提示）----
export interface LocationTip {
  name: string
  address: string
  district: string
  adcode: string
  location: string   // "lng,lat"
  poi_id: string
}

// ---- 自动盯盘配置（商户端可自控）----
export interface AutoMonitorConfig {
  frequency: 'daily' | 'half_day' | 'weekly' // 每天 / 每 12 小时 / 每周
  sample_size: number        // 每关键词采样次数（3/5/10）
  engine_name?: string       // 盯盘引擎（空=default）
  notify_drop_threshold: number // 提及率下降通知阈值（百分点）
  notify_overtake: boolean   // 竞品反超通知开关
}

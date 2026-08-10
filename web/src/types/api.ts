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
}
export interface RegisterResponse { user_id: string; role: string; tenant_id: string }

// ---- 任务 ----
export interface EnqueueTaskRequest { type: string; input: unknown }
export interface EnqueueTaskResponse { task_id: string }
export interface TaskView {
  id: string; type: string; status: string; error?: string; output?: string; progress?: string
}

// ---- 聊天 ----
export interface ChatMessage { role: string; content: string }

// ---- Agent 配置 ----
export interface AgentConfig {
  name: string
  system_prompt: string
  tools: string[]
  llm_config_name?: string  // 引用的 LLMConfig.name（留空用 default）
  max_iterations?: number
  auto_save?: boolean       // 自动把对话回复落库为 DataItem
  field_mapping?: string    // 自动落库字段映射 JSON
}

// ---- LLM 配置（独立聚合根，多厂商多模型）----
export interface LLMConfig {
  name: string       // 唯一标识，如 "default"、"minimax-m2"
  provider: string   // 厂商标签：minimax/openai/zhipu/deepseek
  api_key: string
  base_url: string   // 如 https://api.minimaxi.com/v1
  model: string      // 如 MiniMax-M2.5
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

// ---- 数据项 ----
export interface DataItem {
  id: string
  collection_id: string
  title: string
  content: string
  summary: string
  tags: string[]
  source_url: string
  raw_content: string
  status: string // pending_review / approved / rejected
  metadata: Record<string, string>
  created_at: string
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
  data_items: number          // 采集数据项总数
  // 数据资产明细（趋势/分布图）
  status_breakdown: Record<string, number>  // 数据项状态分布
  daily_trend: { date: string; count: number }[]      // 近14天数据项趋势
  source_distribution: { name: string; count: number }[]  // 数据源分布
  top_tags: { name: string; count: number }[]             // 标签Top8
}

// ---- GEO：品牌资产 ----
export interface Brand {
  id: string
  tenant_id: string
  name: string
  positioning: string
  core_selling: string[]
  competitors: string[]
  created_at: string
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
  confidence: number
  probed_at: string
  raw_sample: string
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
  created_at: string
}

// ---- GEO：平台账号（扫码绑定）----
export interface Account {
  id: string
  tenant_id: string
  platform: string         // zhihu / xiaohongshu / ...
  display_name: string
  health: string           // active / expired / banned
  login_method: string     // zhihu / wechat / qq / weibo
  expires_at: string       // cookie 过期时间
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

// ---- 视频生成工作台 ----
export interface VideoTask {
  id: string
  tenant_id: string
  brand_id?: string
  mode: string            // text / material
  prompt: string
  material_url: string
  status: string          // pending / generating / dubbing / composing / ready / failed
  video_url: string
  voice_text: string
  voice_url: string
  final_url: string
  duration_sec: number
  error: string
  created_at: string
  updated_at: string
}

export interface VideoJob {
  id: string
  tenant_id: string
  task_id: string
  account_id: string
  platform: string
  status: string          // pending / publishing / published / failed
  external_url: string
  error: string
  created_at: string
}

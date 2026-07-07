// 后端 DTO 的 TypeScript 镜像（通用平台版）。

// ---- 统一响应信封 ----
export interface ApiEnvelope<T> {
  code: number
  msg: string
  data?: T
}

// ---- 认证 ----
export interface LoginRequest { username: string; password: string }
export interface RegisterRequest { username: string; password: string }
export interface LoginResponse { token: string }
export interface RegisterResponse { user_id: string }

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

// ---- 采集集合 ----
export interface Collection {
  id: string
  name: string
  agent_name: string
  task_id: string
  status: string
  item_count: number
  created_at: string
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

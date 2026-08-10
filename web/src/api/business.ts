import { apiClient } from './client'
import type { TaskView, AgentConfig, LLMConfig, Collection, DataItem, Conversation, ChatMessageRecord, CrawlConfig, ExternalSystem, PublishResult, PublishRecord, ToolView, StatsView, Brand, Keyword, MonitoringResult, BrandOverview, OptimizedContent, UserView, Account, PublishJob } from '../types/api'

// 通用平台 API 封装。

export const businessApi = {
  // ---- 任务 ----
  getTask: (id: string) =>
    apiClient.get<unknown, TaskView>(`/api/v1/tasks/${id}`),

  listTasks: () =>
    apiClient.get<unknown, TaskView[]>('/api/v1/tasks'),

  // ---- Agent 配置 ----
  listAgentConfigs: () =>
    apiClient.get<unknown, AgentConfig[]>('/api/v1/agents'),

  createAgentConfig: (data: AgentConfig) =>
    apiClient.post<unknown, AgentConfig>('/api/v1/agents', data),

  updateAgentConfig: (name: string, data: Partial<AgentConfig>) =>
    apiClient.put<unknown, AgentConfig>(`/api/v1/agents/${name}`, data),

  deleteAgentConfig: (name: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/agents/${name}`),

  // ---- LLM 配置 ----
  listLLMConfigs: () =>
    apiClient.get<unknown, LLMConfig[]>('/api/v1/llm-configs'),

  createLLMConfig: (data: LLMConfig) =>
    apiClient.post<unknown, LLMConfig>('/api/v1/llm-configs', data),

  updateLLMConfig: (name: string, data: Partial<LLMConfig>) =>
    apiClient.put<unknown, LLMConfig>(`/api/v1/llm-configs/${name}`, data),

  deleteLLMConfig: (name: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/llm-configs/${name}`),

  // ---- 聊天会话（后端持久化，按用户隔离）----
  listConversations: () =>
    apiClient.get<unknown, Conversation[]>('/api/v1/conversations'),

  createConversation: (data: { id: string; title?: string; agent_name?: string }) =>
    apiClient.post<unknown, Conversation>('/api/v1/conversations', data),

  getMessages: (convId: string) =>
    apiClient.get<unknown, ChatMessageRecord[]>(`/api/v1/conversations/${convId}/messages`),

  saveMessage: (convId: string, data: { id: string; role: string; content: string; tool_calls?: string }) =>
    apiClient.post<unknown, ChatMessageRecord>(`/api/v1/conversations/${convId}/messages`, data),

  renameConversation: (convId: string, title: string) =>
    apiClient.put<unknown, unknown>(`/api/v1/conversations/${convId}`, { title }),

  deleteConversation: (convId: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/conversations/${convId}`),

  // ---- 采集配置（运行时可调）----
  getCrawlConfig: () =>
    apiClient.get<unknown, CrawlConfig>('/api/v1/crawl-config'),

  // ---- 全局工具列表 ----
  listTools: () =>
    apiClient.get<unknown, ToolView[]>('/api/v1/tools'),

  updateCrawlConfig: (data: Partial<CrawlConfig>) =>
    apiClient.put<unknown, CrawlConfig>('/api/v1/crawl-config', data),

  // ---- 外部推送系统 ----
  listExternalSystems: () =>
    apiClient.get<unknown, ExternalSystem[]>('/api/v1/external-systems'),

  createExternalSystem: (data: ExternalSystem) =>
    apiClient.post<unknown, ExternalSystem>('/api/v1/external-systems', data),

  deleteExternalSystem: (name: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/external-systems/${name}`),

  publishToExternal: (dataItemId: string, systemName: string) =>
    apiClient.post<unknown, PublishResult>('/api/v1/external-systems/publish', {
      data_item_id: dataItemId, system_name: systemName,
    }),

  listPublishRecords: (dataItemId: string) =>
    apiClient.get<unknown, PublishRecord[]>(`/api/v1/data-items/${dataItemId}/publish-records`),

  // ---- 采集集合 ----
  listCollections: () =>
    apiClient.get<unknown, Collection[]>('/api/v1/collections'),

  // ---- 数据项 ----
  listDataItems: () =>
    apiClient.get<unknown, DataItem[]>('/api/v1/data-items'),

  approveItem: (id: string) =>
    apiClient.post<unknown, unknown>(`/api/v1/data-items/${id}/approve`),

  rejectItem: (id: string) =>
    apiClient.post<unknown, unknown>(`/api/v1/data-items/${id}/reject`),

  deleteDataItem: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/data-items/${id}`),

  // 把 LLM 对话生成的结构化内容落库为 DataItem（打通"对话生成→自动落库"闭环）
  createDataItemFromContent: (data: { content: string; field_mapping?: string; source_url?: string }) =>
    apiClient.post<unknown, DataItem>('/api/v1/data-items/from-content', data),

  // ---- 工具面板 ----
  toggleTool: (name: string, enabled: boolean) =>
    apiClient.put<unknown, { name: string; enabled: boolean }>(`/api/v1/tools/${name}/toggle`, { enabled }),

  // ---- 仪表盘统计 ----
  getStats: () =>
    apiClient.get<unknown, StatsView>('/api/v1/stats'),

  // ---- GEO 品牌 ----
  listBrands: () =>
    apiClient.get<unknown, Brand[]>('/api/v1/geo/brands'),

  createBrand: (data: { name: string; positioning?: string; core_selling?: string[]; competitors?: string[] }) =>
    apiClient.post<unknown, Brand>('/api/v1/geo/brands', data),

  deleteBrand: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/geo/brands/${id}`),

  // ---- GEO 关键词 ----
  listKeywords: (brandId: string) =>
    apiClient.get<unknown, Keyword[]>(`/api/v1/geo/brands/${brandId}/keywords`),

  addKeyword: (brandId: string, data: { term: string; intent?: string }) =>
    apiClient.post<unknown, Keyword>(`/api/v1/geo/brands/${brandId}/keywords`, data),

  // AI 根据品牌定位自动生成候选关键词
  generateKeywords: (brandId: string) =>
    apiClient.post<unknown, { keywords: string[] }>(`/api/v1/geo/brands/${brandId}/keywords/generate`),

  // 关键词蒸馏（五种来源：brand/text/seed/file/web）
  distillKeywords: (data: {
    source: 'brand' | 'text' | 'seed' | 'file' | 'web'
    brand_id?: string
    text?: string
    seeds?: string[]
    topic?: string
    llm_config_name?: string
  }) =>
    apiClient.post<unknown, { keywords: string[] }>('/api/v1/geo/keywords/distill', data),

  // 跨品牌列出所有关键词（关键词管理页用）
  listAllKeywords: () =>
    apiClient.get<unknown, Keyword[]>('/api/v1/geo/keywords'),

  // 删除关键词
  deleteKeyword: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/geo/keywords/${id}`),

  // GEO 诊断（分析品牌为什么没被 AI 提及，给改进建议）
  diagnoseBrand: (brandId: string, keywordId?: string) =>
    apiClient.post<unknown, {
      brand_id: string
      keyword_id: string
      content_coverage: number
      brand_appearance_rate: number
      competitor_stats: { name: string; appearance_rate: number; avg_position: number }[]
      suggestions: string[]
    }>(`/api/v1/geo/brands/${brandId}/diagnose`, { keyword_id: keywordId }),

  // ---- GEO 监测 ----
  runMonitor: (data: { brand_id: string; engine_name?: string; sample_size?: number }) =>
    apiClient.post<unknown, MonitoringResult[]>('/api/v1/geo/monitor', data),

  getLatestMonitor: (keywordId: string) =>
    apiClient.get<unknown, MonitoringResult[]>(`/api/v1/geo/monitor/${keywordId}`),

  // 品牌批量监测结果
  getLatestMonitorByBrand: (brandId: string) =>
    apiClient.get<unknown, MonitoringResult[]>(`/api/v1/geo/brands/${brandId}/monitor-results`),

  // 租户全部监测结果（关键词一览页用，不依赖品牌筛选）
  getAllMonitorResults: () =>
    apiClient.get<unknown, MonitoringResult[]>('/api/v1/geo/monitor-results'),

  // 单关键词即时监测（比品牌级批量更快）
  monitorKeyword: (data: { keyword_id: string; engine_name?: string; sample_size?: number }) =>
    apiClient.post<unknown, MonitoringResult>('/api/v1/geo/monitor-keyword', data),

  getBrandOverview: (brandId: string, name?: string) =>
    apiClient.get<unknown, BrandOverview>(`/api/v1/geo/brands/${brandId}/overview${name ? '?name=' + encodeURIComponent(name) : ''}`),

  // ---- GEO 内容优化 ----
  optimizeContent: (data: { brand_id: string; keyword_id?: string; original_text: string; keyword: string; llm_config_name?: string }) =>
    apiClient.post<unknown, OptimizedContent>('/api/v1/geo/optimize', data),

  listContents: (brandId: string) =>
    apiClient.get<unknown, OptimizedContent[]>(`/api/v1/geo/brands/${brandId}/contents`),

  // 从零生成内容（根据品牌信息+关键词，AI原创一篇 GEO 文章；支持单/多关键词组合）
  generateContent: (brandId: string, data: { keywords: string[]; brand_info?: string; llm_config_name?: string }) =>
    apiClient.post<unknown, OptimizedContent>(`/api/v1/geo/brands/${brandId}/contents/generate`, data),

  // ---- GEO 平台账号（扫码绑定）----
  listAccounts: () =>
    apiClient.get<unknown, Account[]>('/api/v1/geo/accounts'),

  startQRLogin: (platform: string, method?: string) =>
    apiClient.post<unknown, { session_id: string; platform: string; method: string }>('/api/v1/geo/accounts/qr-login', { platform, method }),

  pollQRLogin: (sessionId: string, platform: string, method?: string) =>
    apiClient.get<unknown, { status: string; qr_image: string; account_id: string; account_name: string; expires_at: string }>(`/api/v1/geo/accounts/qr-login/${sessionId}?platform=${platform}${method ? '&method=' + method : ''}`),

  cancelQRLogin: (sessionId: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/geo/accounts/qr-login/${sessionId}`),

  deleteAccount: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/geo/accounts/${id}`),

  // ---- GEO 内容发布（半自动）----
  publishContent: (data: { account_id?: string; platform: string; content_id?: string; title?: string; content?: string; mode?: string }) =>
    apiClient.post<unknown, PublishJob>('/api/v1/geo/publish', data),

  listPublishJobs: () =>
    apiClient.get<unknown, PublishJob[]>('/api/v1/geo/publish-jobs'),

  markPublished: (jobId: string) =>
    apiClient.post<unknown, unknown>(`/api/v1/geo/publish-jobs/${jobId}/published`),

  getPublishJobStatus: (jobId: string) =>
    apiClient.get<unknown, { id: string; status: string; external_url: string; error_msg: string; platform: string }>(`/api/v1/geo/publish-jobs/${jobId}/status`),

  // ---- 用户管理（管理端）----
  listUsers: () =>
    apiClient.get<unknown, UserView[]>('/api/v1/admin/users'),

  createMerchant: (data: { username: string; password: string; tenant_id?: string }) =>
    apiClient.post<unknown, { user_id: string; role: string; tenant_id: string }>('/api/v1/admin/users', data),

  deleteUser: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/admin/users/${id}`),

  // ---- Tavily 搜索配置（管理端）----
  getTavilyStatus: () =>
    apiClient.get<unknown, { registered: boolean; enabled: boolean }>('/api/v1/admin/tavily-status'),

  updateTavilyKey: (data: { enabled: boolean; api_key?: string }) =>
    apiClient.put<unknown, { name: string; enabled: boolean; note: string }>('/api/v1/admin/tavily-key', data),
}

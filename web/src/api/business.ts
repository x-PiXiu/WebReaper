import { apiClient } from './client'
import type { AgentConfig, LLMConfig, Conversation, ChatMessageRecord, CrawlConfig, ToolView, StatsView, Brand, Keyword, MonitoringResult, BrandOverview, OptimizedContent, UserView, Account, PublishJob, IndexingSubmitLog, VideoTask, VideoJob, Plan, Subscription, Order, RevenueSummary, MyUsageSummary, StoreLocation, NearbyRanking, Advice, CostAnalysis, LocationTip, AutoMonitorConfig, CompetitorSuggestion } from '../types/api'

// 通用平台 API 封装。

export const businessApi = {
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
  // ---- 数据项 ----
  // ---- 工具面板 ----
  toggleTool: (name: string, enabled: boolean) =>
    apiClient.put<unknown, { name: string; enabled: boolean }>(`/api/v1/tools/${name}/toggle`, { enabled }),

  // ---- 仪表盘统计 ----
  getStats: () =>
    apiClient.get<unknown, StatsView>('/api/v1/stats'),

  // ---- GEO 品牌 ----
  listBrands: () =>
    apiClient.get<unknown, Brand[]>('/api/v1/geo/brands'),

  createBrand: (data: { name: string; positioning?: string; core_selling?: string[]; competitors?: string[]; biz_type?: string; website_url?: string }) =>
    apiClient.post<unknown, Brand>('/api/v1/geo/brands', data),

  updateBrand: (id: string, data: { name?: string; positioning?: string; core_selling?: string[]; competitors?: string[]; biz_type?: string; website_url?: string }) =>
    apiClient.put<unknown, Brand>(`/api/v1/geo/brands/${id}`, data),

  deleteBrand: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/geo/brands/${id}`),

  // 竞品自动推荐——两种来源：poi（附近同行，local only）/ monitoring（监测结果蒸馏，local+online）
  suggestCompetitors: (brandId: string, source?: string, limit?: number) => {
    const params = new URLSearchParams()
    if (source) params.set('source', source)
    if (limit) params.set('limit', String(limit))
    const qs = params.toString()
    return apiClient.get<unknown, CompetitorSuggestion[]>(`/api/v1/geo/brands/${brandId}/competitor-suggestions${qs ? '?' + qs : ''}`)
  },

  // ---- GEO 门店档案（本地生活地基）----
  listStoreLocations: (brandId: string) =>
    apiClient.get<unknown, StoreLocation[]>(`/api/v1/geo/brands/${brandId}/store-locations`),

  createStoreLocation: (brandId: string, data: { name?: string; address: string; phone?: string; hours?: string; price_level?: string; biz_type?: string }) =>
    apiClient.post<unknown, StoreLocation>(`/api/v1/geo/brands/${brandId}/store-locations`, data),

  updateStoreLocation: (brandId: string, storeId: string, data: { name?: string; address?: string; phone?: string; hours?: string; price_level?: string; biz_type?: string }) =>
    apiClient.put<unknown, StoreLocation>(`/api/v1/geo/brands/${brandId}/store-locations/${storeId}`, data),

  deleteStoreLocation: (brandId: string, storeId: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/geo/brands/${brandId}/store-locations/${storeId}`),

  reGeocodeStoreLocation: (brandId: string, storeId: string) =>
    apiClient.post<unknown, StoreLocation>(`/api/v1/geo/brands/${brandId}/store-locations/${storeId}/re-geocode`),

  // ---- GEO 附近同行双榜（现实世界地图榜 + AI 竞品榜）----
  getNearbyCompetitors: (brandId: string, types?: string) =>
    apiClient.get<unknown, NearbyRanking>(`/api/v1/geo/brands/${brandId}/nearby-competitors`, { params: { types } }),

  // AI 榜单探查（v2：AI 真实搜索附近同行并归因上榜，缓存 24h；返回新双榜视图）
  runAIRankProbe: (brandId: string, types?: string) =>
    apiClient.post<unknown, NearbyRanking>(`/api/v1/geo/brands/${brandId}/ai-rank-probe`, { types }),

  // ---- GEO 行动建议（P5-05：给老板"下一步做什么"）----
  getAdvice: (brandId: string) =>
    apiClient.get<unknown, { advices: Advice[] }>(`/api/v1/geo/brands/${brandId}/advice`),

  // ---- GEO 内容引用统计（P5-02：每篇被 AI 引用几次，归因细化到篇）----
  getContentCitations: (brandId: string) =>
    apiClient.get<unknown, Record<string, number>>(`/api/v1/geo/brands/${brandId}/citations`),

  // ---- 地址联想（P1 输入提示：门店建档边输入边联想）----
  suggestLocations: (q: string, city?: string, location?: string) =>
    apiClient.get<unknown, LocationTip[]>(`/api/v1/geo/location/suggest`, { params: { q, city, location } }),

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

  // 多引擎批量监测（采样矩阵：每引擎独立采样——"豆包测 3 次 vs 千问测 3 次"对比）
  monitorMulti: (data: { keyword_id: string; engine_names: string[]; sample_size?: number }) =>
    apiClient.post<unknown, MonitoringResult[]>('/api/v1/geo/monitor-multi', data),

  getBrandOverview: (brandId: string, name?: string) =>
    apiClient.get<unknown, BrandOverview>(`/api/v1/geo/brands/${brandId}/overview${name ? '?name=' + encodeURIComponent(name) : ''}`),

  // ---- GEO 内容优化 ----
  optimizeContent: (data: { brand_id: string; keyword_id?: string; original_text: string; keyword: string; llm_config_name?: string; target_engine?: string; format?: string }) =>
    apiClient.post<unknown, OptimizedContent>('/api/v1/geo/optimize', data),

  listContents: (brandId: string) =>
    apiClient.get<unknown, OptimizedContent[]>(`/api/v1/geo/brands/${brandId}/contents`),

  // 从零生成内容（根据品牌信息+关键词，AI原创一篇 GEO 文章；支持单/多关键词组合）
  generateContent: (brandId: string, data: { keywords: string[]; brand_info?: string; llm_config_name?: string; target_engine?: string; use_diagnose?: boolean; format?: string }) =>
    apiClient.post<unknown, OptimizedContent>(`/api/v1/geo/brands/${brandId}/contents/generate`, data),

  // 内容状态流转：draft ↔ published（published 后公开站可访问，AI 引擎可爬取）
  setContentStatus: (brandId: string, contentId: string, status: 'draft' | 'published') =>
    apiClient.post<unknown, OptimizedContent>(`/api/v1/geo/brands/${brandId}/contents/${contentId}/status`, { status }),

  // 删除内容（内容工作台/管理后台）
  deleteContent: (brandId: string, contentId: string) =>
    apiClient.delete<unknown, { deleted: boolean }>(`/api/v1/geo/brands/${brandId}/contents/${contentId}`),

  // 商户端自助补提交收录（IndexNow——重新通知搜索引擎抓取已发布内容）
  resubmitIndex: (brandId: string, contentId: string) =>
    apiClient.post<unknown, { submitted: boolean }>(`/api/v1/geo/brands/${brandId}/contents/${contentId}/resubmit-index`),

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
  publishContent: (data: { account_id?: string; platform: string; content_id?: string; brand_id?: string; title?: string; content?: string; mode?: string; scheduled_at?: string }) =>
    apiClient.post<unknown, PublishJob>('/api/v1/geo/publish', data),

  listPublishJobs: () =>
    apiClient.get<unknown, PublishJob[]>('/api/v1/geo/publish-jobs'),

  markPublished: (jobId: string) =>
    apiClient.post<unknown, unknown>(`/api/v1/geo/publish-jobs/${jobId}/published`),

  getPublishJobStatus: (jobId: string) =>
    apiClient.get<unknown, { id: string; status: string; external_url: string; error_msg: string; platform: string }>(`/api/v1/geo/publish-jobs/${jobId}/status`),

  // 发布效果复测：重新触发品牌监测并更新发布后提及率（建议收录周期 1-2 周后使用）
  reMonitorJob: (jobId: string) =>
    apiClient.post<unknown, PublishJob>(`/api/v1/geo/publish-jobs/${jobId}/re-monitor`),

  // 收录管理（管理后台）
  getIndexingConfig: () =>
    apiClient.get<unknown, { index_now_key: string; baidu_site: string; baidu_token: string; updated_at: string }>('/api/v1/admin/indexing/config'),
  updateIndexingConfig: (data: { index_now_key?: string; baidu_site?: string; baidu_token?: string }) =>
    apiClient.put<unknown, { ok: boolean }>('/api/v1/admin/indexing/config', data),
  listIndexingLogs: () =>
    apiClient.get<unknown, IndexingSubmitLog[]>('/api/v1/admin/indexing/logs'),
  reSubmitAllIndexing: () =>
    apiClient.post<unknown, { submitted: number; failed: number }>('/api/v1/admin/indexing/re-submit'),
  // IndexNow 密钥自动生成（协议：密钥由网站所有者生成 = 所有权证明，系统代为生成 GUID 并托管 key 文件）
  generateIndexingKey: () =>
    apiClient.post<unknown, { index_now_key: string }>('/api/v1/admin/indexing/generate-key'),
  // 验证 key 文件可公开访问（搜索引擎视角）
  verifyIndexingKey: () =>
    apiClient.get<unknown, { url: string; reachable: boolean; content_match: boolean; status_code: number; error: string }>('/api/v1/admin/indexing/verify-key'),

  // ---- 用户管理（管理端）----
  listUsers: () =>
    apiClient.get<unknown, UserView[]>('/api/v1/admin/users'),

  createMerchant: (data: { username: string; password: string; tenant_id?: string }) =>
    apiClient.post<unknown, { user_id: string; role: string; tenant_id: string }>('/api/v1/admin/users', data),

  deleteUser: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/admin/users/${id}`),

  // ---- 全平台资源管理（admin 旁路：显式全局查询端点，不走商户租户上下文）----
  adminListBrands: () =>
    apiClient.get<unknown, Brand[]>('/api/v1/admin/brands'),

  adminListContents: (status?: string) =>
    apiClient.get<unknown, OptimizedContent[]>(`/api/v1/admin/contents${status ? `?status=${status}` : ''}`),

  adminDeleteBrand: (id: string) =>
    apiClient.delete<unknown, { deleted: boolean }>(`/api/v1/admin/brands/${id}`),

  adminSetContentStatus: (contentId: string, status: 'draft' | 'published') =>
    apiClient.post<unknown, OptimizedContent>(`/api/v1/admin/contents/${contentId}/status`, { status }),

  adminDeleteContent: (contentId: string) =>
    apiClient.delete<unknown, { deleted: boolean }>(`/api/v1/admin/contents/${contentId}`),

  // ---- 平台系统设置（管理端：运行时开关）----
  getAutoMonitor: () =>
    apiClient.get<unknown, { auto_monitor_enabled: boolean }>('/api/v1/admin/settings/auto-monitor'),
  setAutoMonitor: (enabled: boolean) =>
    apiClient.put<unknown, { auto_monitor_enabled: boolean }>('/api/v1/admin/settings/auto-monitor', { enabled }),

  // ---- 站内通知（主动唤醒）----
  listNotifications: () =>
    apiClient.get<unknown, { id: string; type: string; title: string; content: string; link: string; read: boolean; created_at: string }[]>('/api/v1/notifications'),
  notificationUnreadCount: () =>
    apiClient.get<unknown, { unread: number }>('/api/v1/notifications/unread-count'),
  markNotificationRead: (id?: string) =>
    apiClient.post<unknown, { ok: boolean }>(`/api/v1/notifications/${id || 'all'}/read`),

  // ---- 商户端自动盯盘（租户级）----
  getTenantAutoMonitor: () =>
    apiClient.get<unknown, { tenant_enabled: boolean; platform_enabled: boolean; config: AutoMonitorConfig }>('/api/v1/geo/monitor-auto'),
  // 开关 + 盯盘配置（频率/采样/通知阈值——一次保存）
  setTenantAutoMonitor: (data: { enabled: boolean; config?: AutoMonitorConfig }) =>
    apiClient.put<unknown, { tenant_enabled: boolean; config: AutoMonitorConfig }>('/api/v1/geo/monitor-auto', data),

  // ---- Tavily 搜索配置（管理端）----
  getTavilyStatus: () =>
    apiClient.get<unknown, { registered: boolean; enabled: boolean }>('/api/v1/admin/tavily-status'),

  updateTavilyKey: (data: { enabled: boolean; api_key?: string }) =>
    apiClient.put<unknown, { name: string; enabled: boolean; note: string }>('/api/v1/admin/tavily-key', data),

  // ---- 视频生成工作台（Vidu 流水线）----
  submitVideoTask: (data: { mode: string; prompt?: string; material_url?: string; brand_id?: string; voice_text?: string }) =>
    apiClient.post<unknown, VideoTask>('/api/v1/video/tasks', data),

  getVideoTask: (id: string) =>
    apiClient.get<unknown, VideoTask>(`/api/v1/video/tasks/${id}`),

  listVideoTasks: () =>
    apiClient.get<unknown, VideoTask[]>('/api/v1/video/tasks'),

  publishVideoTask: (data: { task_id: string; platform: string; account_id?: string }) =>
    apiClient.post<unknown, VideoJob>('/api/v1/video/tasks/publish', data),

  listVideoJobs: () =>
    apiClient.get<unknown, VideoJob[]>('/api/v1/video/jobs'),

  // ---- 经济系统（套餐/订阅/订单/用量）----

  // 商户端
  listActivePlans: () =>
    apiClient.get<unknown, { plans: Plan[] }>('/api/v1/billing/plans'),
  getMyPlan: () =>
    apiClient.get<unknown, { subscription: Subscription | null; hint?: string }>('/api/v1/billing/my-plan'),
  getMyUsage: () =>
    apiClient.get<unknown, MyUsageSummary>('/api/v1/billing/usage'),
  listMyOrders: () =>
    apiClient.get<unknown, { orders: Order[] }>('/api/v1/billing/orders'),
  createOrder: (planId: string) =>
    apiClient.post<unknown, { order: Order; payment_url: string }>('/api/v1/billing/orders', { plan_id: planId }),
  confirmOrder: (orderId: string) =>
    apiClient.post<unknown, { subscription: Subscription }>(`/api/v1/billing/orders/${orderId}/confirm`),

  // admin 端
  adminListPlans: () =>
    apiClient.get<unknown, { plans: Plan[] }>('/api/v1/admin/billing/plans'),
  adminSavePlan: (plan: Plan) =>
    apiClient.post<unknown, Plan>('/api/v1/admin/billing/plans', plan),
  adminDeletePlan: (id: string) =>
    apiClient.delete<unknown, { id: string }>(`/api/v1/admin/billing/plans/${id}`),
  adminListSubscriptions: () =>
    apiClient.get<unknown, { subscriptions: Subscription[] }>('/api/v1/admin/billing/subscriptions'),
  adminAssignPlan: (tenant: string, planId: string) =>
    apiClient.put<unknown, { subscription: Subscription }>(`/api/v1/admin/billing/subscriptions/${tenant}`, { plan_id: planId }),
  adminListOrders: () =>
    apiClient.get<unknown, { orders: Order[] }>('/api/v1/admin/billing/orders'),
  adminRevenueReport: () =>
    apiClient.get<unknown, RevenueSummary>('/api/v1/admin/billing/revenue'),
  // X-01 成本分析（收入 vs 成本双报表）
  adminCostAnalysis: (days = 30) =>
    apiClient.get<unknown, CostAnalysis>(`/api/v1/admin/billing/cost-analysis?days=${days}`),
  adminGetPaymentConfig: () =>
    apiClient.get<unknown, { config: Record<string, string> }>('/api/v1/admin/billing/payment-config'),
  adminSetPaymentConfig: (cfg: { gateway: string; pid: string; key: string; notify_url: string; return_url: string }) =>
    apiClient.put<unknown, { saved: boolean }>('/api/v1/admin/billing/payment-config', cfg),

  // admin 提示词模板管理（格式指令/生成/优化 prompt 热更新）
  // 后端返回 data: { templates: [...] }（对象包裹）——前端解包为数组
  adminListPromptTemplates: () =>
    apiClient.get<unknown, { templates: { key: string; version: number; content: string; updated_at: string }[] }>('/api/v1/admin/prompt-templates').then((r) => r.templates),
  adminUpdatePromptTemplate: (key: string, content: string) =>
    apiClient.put<unknown, { key: string }>(`/api/v1/admin/prompt-templates/${key}`, { content }),
}

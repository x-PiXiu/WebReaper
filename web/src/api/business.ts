import { apiClient } from './client'
import { submitGenerationTaskCompat, submitUnified } from './generationSubmit'
import type { AgentConfig, LLMConfig, EngineOption, HealthReportView, IndustryOverviewView, AIRankItemView, Conversation, ChatMessageRecord, ToolView, StatsView, Brand, Keyword, MonitoringResult, BrandOverview, OptimizedContent, UserView, Account, PublishJob, IndexingSubmitLog, GenerationType, GenerationTask, GenerationSpec, GenerationTemplate, MediaAsset, PromptRef, ProviderConfig, Plan, Subscription, Order, RevenueSummary, MyUsageSummary, StoreLocation, NearbyRanking, Advice, CostAnalysis, LocationTip, AutoMonitorConfig, CompetitorSuggestion, KnowledgeEmbeddingConfig, IndustryCrawlConfig, KnowledgeMaterialView, KnowledgeStats, KnowledgeCrawlInterval, PublishChannelView, GenerationModeView, AnalyticsSummary, WorkItem, GenerationVoice, IntegrationEntry, IntegrationGroup, IntegrationMeta, IntegrationVendor, IntegrationCapability, CrawlerAccount, CrawlerConfig, CrawlerTaskLog, CrawlResult, InspirationVideo, BrandPublishConfig, AccountBrandBinding, TaskTimeline } from '../types/api'

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

  // ---- LLM 配置（admin——完整视图含厂商密钥；商户端引擎选择用 listEngines）----
  listLLMConfigs: () =>
    apiClient.get<unknown, LLMConfig[]>('/api/v1/admin/llm-configs'),

  createLLMConfig: (data: LLMConfig) =>
    apiClient.post<unknown, LLMConfig>('/api/v1/admin/llm-configs', data),

  updateLLMConfig: (name: string, data: Partial<LLMConfig>) =>
    apiClient.put<unknown, LLMConfig>(`/api/v1/admin/llm-configs/${name}`, data),

  deleteLLMConfig: (name: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/admin/llm-configs/${name}`),

  // ---- 引擎名单（商户/聊天通用——仅 name/provider/model，不含厂商密钥）----
  listEngines: () =>
    apiClient.get<unknown, EngineOption[]>('/api/v1/merchant/engines'),

  // ---- GEO 健康报告（后端聚合单一事实源：总分/五指数/环比/竞品对标/品牌级分值）----
  getHealthReport: () =>
    apiClient.get<unknown, HealthReportView>('/api/v1/merchant/health-report'),

  // ---- 行业全景看板（admin：跨商户聚合——行业能见度/品牌美誉度/信源域名榜）----
  getIndustryOverview: () =>
    apiClient.get<unknown, IndustryOverviewView>('/api/v1/admin/merchant/industry-overview'),

  // ---- AI 榜缓存（F4 品牌卡徽章：只读最近一次探查，无缓存 available=false）----
  getAIRank: (brandId: string) =>
    apiClient.get<unknown, { available: boolean; probed_at?: string; items?: AIRankItemView[] }>(`/api/v1/merchant/brands/${brandId}/ai-rank`),

  // ---- 认证：修改当前登录用户密码（F1-5 默认口令治理）----
  changePassword: (data: { old_password: string; new_password: string }) =>
    apiClient.put<unknown, { changed: boolean }>('/api/v1/auth/password', data),

  // ---- 聊天会话（后端持久化，按用户隔离）----
  listConversations: () =>
    apiClient.get<unknown, Conversation[]>('/api/v1/conversations'),

  createConversation: (data: { id: string; title?: string; agent_name?: string }) =>
    apiClient.post<unknown, Conversation>('/api/v1/conversations', data),

  getMessages: (convId: string) =>
    apiClient.get<unknown, ChatMessageRecord[]>(`/api/v1/conversations/${convId}/messages`),

  saveMessage: (convId: string, data: { id: string; role: string; content: string; tool_calls?: string }) =>
    apiClient.post<unknown, ChatMessageRecord>(`/api/v1/conversations/${convId}/messages`, data),

  deleteConversation: (convId: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/conversations/${convId}`),

  // 会话重命名（标题是首句截断时用户可手动修正——P2-9-11）
  renameConversation: (convId: string, title: string) =>
    apiClient.put<unknown, unknown>(`/api/v1/conversations/${convId}`, { title }),

  // ---- 全局工具列表 ----
  listTools: () =>
    apiClient.get<unknown, ToolView[]>('/api/v1/tools'),

  // ---- 工具面板 ----
  toggleTool: (name: string, enabled: boolean) =>
    apiClient.put<unknown, { name: string; enabled: boolean }>(`/api/v1/tools/${name}/toggle`, { enabled }),

  // ---- 仪表盘统计 ----
  getStats: () =>
    apiClient.get<unknown, StatsView>('/api/v1/stats'),

  // ---- GEO 品牌 ----
  listBrands: () =>
    apiClient.get<unknown, Brand[]>('/api/v1/merchant/brands'),

  createBrand: (data: { name: string; positioning?: string; core_selling?: string[]; competitors?: string[]; biz_type?: string; industry?: string; website_url?: string }) =>
    apiClient.post<unknown, Brand>('/api/v1/merchant/brands', data),

  updateBrand: (id: string, data: { name?: string; positioning?: string; core_selling?: string[]; competitors?: string[]; biz_type?: string; industry?: string; website_url?: string }) =>
    apiClient.put<unknown, Brand>(`/api/v1/merchant/brands/${id}`, data),

  deleteBrand: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/merchant/brands/${id}`),

  // 竞品自动推荐——两种来源：poi（附近同行，local only）/ monitoring（监测结果蒸馏，local+online）
  suggestCompetitors: (brandId: string, source?: string, limit?: number) => {
    const params = new URLSearchParams()
    if (source) params.set('source', source)
    if (limit) params.set('limit', String(limit))
    const qs = params.toString()
    return apiClient.get<unknown, CompetitorSuggestion[]>(`/api/v1/merchant/brands/${brandId}/competitor-suggestions${qs ? '?' + qs : ''}`)
  },

  // ---- GEO 门店档案（本地生活地基）----
  listStoreLocations: (brandId: string) =>
    apiClient.get<unknown, StoreLocation[]>(`/api/v1/merchant/brands/${brandId}/store-locations`),

  createStoreLocation: (brandId: string, data: { name?: string; address: string; phone?: string; hours?: string; price_level?: string; biz_type?: string }) =>
    apiClient.post<unknown, StoreLocation>(`/api/v1/merchant/brands/${brandId}/store-locations`, data),

  updateStoreLocation: (brandId: string, storeId: string, data: { name?: string; address?: string; phone?: string; hours?: string; price_level?: string; biz_type?: string }) =>
    apiClient.put<unknown, StoreLocation>(`/api/v1/merchant/brands/${brandId}/store-locations/${storeId}`, data),

  deleteStoreLocation: (brandId: string, storeId: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/merchant/brands/${brandId}/store-locations/${storeId}`),

  reGeocodeStoreLocation: (brandId: string, storeId: string) =>
    apiClient.post<unknown, StoreLocation>(`/api/v1/merchant/brands/${brandId}/store-locations/${storeId}/re-geocode`),

  // ---- GEO 附近同行双榜（现实世界地图榜 + AI 竞品榜）----
  getNearbyCompetitors: (brandId: string, types?: string) =>
    apiClient.get<unknown, NearbyRanking>(`/api/v1/merchant/brands/${brandId}/nearby-competitors`, { params: { types } }),

  // AI 榜单探查（v2：AI 真实搜索附近同行并归因上榜，缓存 24h；返回新双榜视图）
  runAIRankProbe: (brandId: string, types?: string) =>
    apiClient.post<unknown, NearbyRanking>(`/api/v1/merchant/brands/${brandId}/ai-rank-probe`, { types }),

  // ---- GEO 行动建议（P5-05：给老板"下一步做什么"）----
  getAdvice: (brandId: string) =>
    apiClient.get<unknown, { advices: Advice[] }>(`/api/v1/merchant/brands/${brandId}/advice`),

  // ---- GEO 内容引用统计（P5-02：每篇被 AI 引用几次，归因细化到篇）----
  getContentCitations: (brandId: string) =>
    apiClient.get<unknown, Record<string, number>>(`/api/v1/merchant/brands/${brandId}/citations`),

  // ---- 地址联想（P1 输入提示：门店建档边输入边联想）----
  suggestLocations: (q: string, city?: string, location?: string) =>
    apiClient.get<unknown, LocationTip[]>(`/api/v1/merchant/location/suggest`, { params: { q, city, location } }),

  // ---- GEO 关键词 ----
  listKeywords: (brandId: string) =>
    apiClient.get<unknown, Keyword[]>(`/api/v1/merchant/brands/${brandId}/keywords`),

  addKeyword: (brandId: string, data: { term: string; intent?: string }) =>
    apiClient.post<unknown, Keyword>(`/api/v1/merchant/brands/${brandId}/keywords`, data),

  // 关键词蒸馏（六种来源：brand/text/seed/file/web/questions）
  distillKeywords: (data: {
    source: 'brand' | 'text' | 'seed' | 'file' | 'web' | 'questions'
    brand_id?: string
    text?: string
    seeds?: string[]
    topic?: string
    llm_config_name?: string
  }) =>
    apiClient.post<unknown, { keywords: string[] }>('/api/v1/merchant/keywords/distill', data),

  // 跨品牌列出所有关键词（关键词管理页用）
  listAllKeywords: () =>
    apiClient.get<unknown, Keyword[]>('/api/v1/merchant/keywords'),

  // 删除关键词
  deleteKeyword: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/merchant/keywords/${id}`),

  // ---- GEO 监测 ----
  // 租户全部监测结果（关键词一览页用，不依赖品牌筛选）
  getAllMonitorResults: () =>
    apiClient.get<unknown, MonitoringResult[]>('/api/v1/merchant/monitor-results'),

  // 单关键词即时监测（比品牌级批量更快）
  monitorKeyword: (data: { keyword_id: string; engine_name?: string; sample_size?: number }) =>
    apiClient.post<unknown, MonitoringResult>('/api/v1/merchant/monitor-keyword', data),

  // 多引擎批量监测（采样矩阵：每引擎独立采样——"豆包测 3 次 vs 千问测 3 次"对比）
  monitorMulti: (data: { keyword_id: string; engine_names: string[]; sample_size?: number }) =>
    apiClient.post<unknown, MonitoringResult[]>('/api/v1/merchant/monitor-multi', data),

  getBrandOverview: (brandId: string, name?: string) =>
    apiClient.get<unknown, BrandOverview>(`/api/v1/merchant/brands/${brandId}/overview${name ? '?name=' + encodeURIComponent(name) : ''}`),

  // ---- GEO 内容优化 ----
  optimizeContent: (data: { brand_id: string; keyword_id?: string; original_text: string; keyword: string; llm_config_name?: string; target_engine?: string; format?: string }) =>
    apiClient.post<unknown, OptimizedContent>('/api/v1/merchant/optimize', data),

  listContents: (brandId: string) =>
    apiClient.get<unknown, OptimizedContent[]>(`/api/v1/merchant/brands/${brandId}/contents`),

  // 从零生成内容（根据品牌信息+关键词，AI原创一篇 GEO 文章；支持单/多关键词组合）
  generateContent: (brandId: string, data: { keywords?: string[]; topic?: string; brand_info?: string; llm_config_name?: string; target_engine?: string; use_diagnose?: boolean; format?: string; citation_toggles?: string[] }) =>
    apiClient.post<unknown, OptimizedContent>(`/api/v1/merchant/brands/${brandId}/contents/generate`, data),

  // 内容状态流转：draft ↔ published（published 后公开站可访问，AI 引擎可爬取）
  setContentStatus: (brandId: string, contentId: string, status: 'draft' | 'published') =>
    apiClient.post<unknown, OptimizedContent>(`/api/v1/merchant/brands/${brandId}/contents/${contentId}/status`, { status }),

  // 商户端自助补提交收录（IndexNow——重新通知搜索引擎抓取已发布内容）
  resubmitIndex: (brandId: string, contentId: string) =>
    apiClient.post<unknown, { submitted: boolean }>(`/api/v1/merchant/brands/${brandId}/contents/${contentId}/resubmit-index`),

  // ---- GEO 平台账号（扫码绑定）----
  listAccounts: () =>
    apiClient.get<unknown, Account[]>('/api/v1/merchant/accounts'),

  startQRLogin: (platform: string, method?: string) =>
    apiClient.post<unknown, { session_id: string; platform: string; method: string }>('/api/v1/merchant/accounts/qr-login', { platform, method }),

  pollQRLogin: (sessionId: string, platform: string, method?: string, scene?: string) =>
    apiClient.get<unknown, { status: string; qr_image: string; account_id: string; account_name: string; expires_at: string }>(`/api/v1/merchant/accounts/qr-login/${sessionId}?platform=${platform}${method ? '&method=' + method : ''}${scene ? '&scene=' + scene : ''}`),

  cancelQRLogin: (sessionId: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/merchant/accounts/qr-login/${sessionId}`),

  deleteAccount: (id: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/merchant/accounts/${id}`),

  // 抖音官方 OAuth 授权（新窗口打开授权页扫码；回调由服务端处理并 302 跳回前端）
  getDouyinOAuthURL: () =>
    apiClient.get<unknown, { url: string }>('/api/v1/merchant/accounts/douyin/oauth/url'),

  // 作品库三源聚合（我的作品页）
  listWorks: () =>
    apiClient.get<unknown, WorkItem[]>('/api/v1/merchant/works'),

  // 作品数据页聚合（指标卡+趋势+已发布作品列表）
  getAnalyticsSummary: () =>
    apiClient.get<unknown, AnalyticsSummary>('/api/v1/merchant/works/analytics-summary'),

  // 发布计划硬确认（主 Agent 确认卡片；scheduled_at 可选=定时发布）
  confirmPublishPlan: (planId: string, scheduledAt?: string) =>
    apiClient.post<unknown, PublishJob>(`/api/v1/merchant/publish-plans/${planId}/confirm`, { scheduled_at: scheduledAt }),

  cancelPublishPlan: (planId: string) =>
    apiClient.post<unknown, unknown>(`/api/v1/merchant/publish-plans/${planId}/cancel`),

  // 发布通道管理（admin：双链路共存的手动切换）
  listPublishTransports: () =>
    apiClient.get<unknown, { platforms: Array<{ platform: string; available: string[]; override: string; mode: string }> }>(`/api/v1/admin/publish/transports`),

  setPublishTransport: (platform: string, kind: string) =>
    apiClient.put<unknown, { platform: string; override: string; mode: string }>(`/api/v1/admin/publish/transports/${platform}`, { kind }),

  // 手动回读单作品互动数据（详情 Drawer「立即刷新」）
  refreshJobMetrics: (jobId: string) =>
    apiClient.post<unknown, { job_id: string; views: number; likes: number; comments: number; shares: number; collected_at: string }>(`/api/v1/merchant/publish-jobs/${jobId}/refresh-metrics`),

  // 单作品指标时间序列（详情趋势图）
  getJobMetrics: (jobId: string) =>
    apiClient.get<unknown, Array<{ views: number; likes: number; comments: number; shares: number; collected_at: string }>>(`/api/v1/merchant/publish-jobs/${jobId}/metrics`),

  // ---- GEO 内容发布（半自动）----
  publishContent: (data: {
    account_id?: string
    platform: string
    content_id?: string
    brand_id?: string
    title?: string
    content?: string
    mode?: string
    scheduled_at?: string
    content_type?: string
    media_urls?: string[]
    cover_url?: string
    tags?: string[]
    category?: string
    store_address?: string
  }) => apiClient.post<unknown, PublishJob>('/api/v1/merchant/publish', data),

  listPublishJobs: (brandId?: string) =>
    apiClient.get<unknown, PublishJob[]>(`/api/v1/merchant/publish-jobs${brandId ? `?brand_id=${brandId}` : ''}`),

  markPublished: (jobId: string) =>
    apiClient.post<unknown, unknown>(`/api/v1/merchant/publish-jobs/${jobId}/published`),

  getPublishJobStatus: (jobId: string) =>
    apiClient.get<unknown, { id: string; status: string; external_url: string; error_msg: string; platform: string }>(`/api/v1/merchant/publish-jobs/${jobId}/status`),

  // 发布效果复测：重新触发品牌监测并更新发布后提及率（建议收录周期 1-2 周后使用）
  reMonitorJob: (jobId: string) =>
    apiClient.post<unknown, PublishJob>(`/api/v1/merchant/publish-jobs/${jobId}/re-monitor`),

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

  // ---- 平台知识库（管理后台：向量配置/行业采集/素材管理/向量重建）----
  getKnowledgeEmbeddingConfig: () =>
    apiClient.get<unknown, KnowledgeEmbeddingConfig>('/api/v1/admin/knowledge/embedding-config'),
  updateKnowledgeEmbeddingConfig: (data: Partial<KnowledgeEmbeddingConfig>) =>
    apiClient.put<unknown, { ok: boolean; note: string }>('/api/v1/admin/knowledge/embedding-config', data),
  getKnowledgeCrawlConfig: () =>
    apiClient.get<unknown, IndustryCrawlConfig[]>('/api/v1/admin/knowledge/crawl-config'),
  updateKnowledgeCrawlConfig: (data: IndustryCrawlConfig[]) =>
    apiClient.put<unknown, { ok: boolean }>('/api/v1/admin/knowledge/crawl-config', data),
  getKnowledgeStats: () =>
    apiClient.get<unknown, KnowledgeStats>('/api/v1/admin/knowledge/stats'),
  listKnowledgeMaterials: (params?: { industry?: string; limit?: number; offset?: number }) =>
    apiClient.get<unknown, KnowledgeMaterialView[]>('/api/v1/admin/knowledge/materials', { params }),
  deleteKnowledgeMaterial: (id: string) =>
    apiClient.delete<unknown, { ok: boolean }>(`/api/v1/admin/knowledge/materials/${id}`),
  // 重建素材向量（换 embedding 模型后存量向量失效——重建恢复检索正确性）
  reindexKnowledgeMaterials: (params?: { industry?: string; only_missing?: boolean }) =>
    apiClient.post<unknown, { processed: number; updated: number; failed: number; note: string }>(
      '/api/v1/admin/knowledge/reindex', null, { params }),
  // 采集间隔（分钟，30-1440；下个周期生效免重启）
  getKnowledgeCrawlInterval: () =>
    apiClient.get<unknown, KnowledgeCrawlInterval>('/api/v1/admin/knowledge/crawl-interval'),
  updateKnowledgeCrawlInterval: (interval_minutes: number) =>
    apiClient.put<unknown, { ok: boolean; note: string }>('/api/v1/admin/knowledge/crawl-interval', { interval_minutes }),

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

  // 浏览器可见性（RPA 发布/扫码登录时是否显示浏览器窗口——动态切换）
  getBrowserHeaded: () =>
    apiClient.get<unknown, { headed: boolean }>('/api/v1/admin/settings/browser-headed'),
  setBrowserHeaded: (headed: boolean) =>
    apiClient.put<unknown, { headed: boolean }>('/api/v1/admin/settings/browser-headed', { headed }),

  // ---- 站内通知（主动唤醒）----
  listNotifications: () =>
    apiClient.get<unknown, { id: string; type: string; title: string; content: string; link: string; read: boolean; created_at: string }[]>('/api/v1/notifications'),
  notificationUnreadCount: () =>
    apiClient.get<unknown, { unread: number }>('/api/v1/notifications/unread-count'),
  markNotificationRead: (id?: string) =>
    apiClient.post<unknown, { ok: boolean }>(`/api/v1/notifications/${id || 'all'}/read`),

  // ---- 商户端自动盯盘（租户级）----
  getTenantAutoMonitor: () =>
    apiClient.get<unknown, { tenant_enabled: boolean; platform_enabled: boolean; config: AutoMonitorConfig }>('/api/v1/merchant/monitor-auto'),
  // 开关 + 盯盘配置（频率/采样/通知阈值——一次保存）
  setTenantAutoMonitor: (data: { enabled: boolean; config?: AutoMonitorConfig }) =>
    apiClient.put<unknown, { tenant_enabled: boolean; config: AutoMonitorConfig }>('/api/v1/merchant/monitor-auto', data),

  // ---- Tavily 搜索配置（管理端）----
  getTavilyStatus: () =>
    apiClient.get<unknown, { registered: boolean; enabled: boolean }>('/api/v1/admin/tavily-status'),

  updateTavilyKey: (data: { enabled: boolean; api_key?: string }) =>
    apiClient.put<unknown, { name: string; enabled: boolean; note: string }>('/api/v1/admin/tavily-key', data),

  // ---- 统一生成（Docs/API/统一生成API文档.md：仅 POST /generation/submit）----
  // 字段：brand_id / text / materials / template / type / duration / quality / aspect_ratio

  // 端点类型 + 模型 + 能力向量（表单能力展示；提交不再传 model/sub_type）
  listGenerationTypes: () =>
    apiClient.get<unknown, { types: GenerationType[] }>('/api/v1/generation/types'),

  // 统一提交（傻瓜式：text + materials + type，服务端选端点/模型）
  submitGeneration: (data: {
    brand_id: string
    text?: string
    materials?: string[]
    template?: string
    type?: 'video' | 'image' | 'audio' | 'voice'
    duration?: number
    quality?: string
    aspect_ratio?: string
    params?: Record<string, unknown> // 高级参数（模型选择/音色等——与 generationSubmit 同通道）
    sub_type?: string // 显式端点覆盖（subject=数字分身主体注册；空=自动选择）
    watermark?: boolean
    off_peak?: boolean
  }) => submitUnified(data),

  /**
   * @deprecated 高级 POST /generation/tasks 已删除。
   * 保留此方法作兼容：内部映射到 submitGeneration（见 generationSubmit.ts）。
   */
  submitGenerationTask: (data: {
    brand_id?: string
    sub_type: string
    model: string
    params: Record<string, unknown>
    refs?: PromptRef[]
    off_peak?: boolean
    watermark?: boolean
  }) => submitGenerationTaskCompat(data),

  listGenerationTasks: () =>
    apiClient.get<unknown, { tasks: GenerationTask[] }>('/api/v1/generation/tasks'),

  getGenerationTask: (id: string) =>
    apiClient.get<unknown, GenerationTask>(`/api/v1/generation/tasks/${id}`),

  cancelGenerationTask: (id: string) =>
    apiClient.post<unknown, { cancelled: string }>(`/api/v1/generation/tasks/${id}/cancel`),

  deleteGenerationTask: (id: string) =>
    apiClient.delete<unknown, { deleted: string }>(`/api/v1/generation/tasks/${id}`),

  // 官方音色库
  listGenerationVoices: (params?: { language?: string; q?: string }) =>
    apiClient.get<unknown, { voices: GenerationVoice[] }>('/api/v1/generation/voices', { params }),

  // ---- 模板管理 ----

  // 查询可用模板（客户端）
  listTemplates: () =>
    apiClient.get<unknown, { templates: GenerationTemplate[] }>('/api/v1/generation/templates').then((r) => r.templates),

  // 查询单个模板
  getTemplate: (id: string) =>
    apiClient.get<unknown, GenerationTemplate>(`/api/v1/generation/templates/${id}`),

  // ---- 智能体 ----

  // 智能体对话
  agentChat: (data: { message: string; permission_level?: string }) =>
    apiClient.post<unknown, { reply: string }>('/api/v1/agent/chat', data),

  // ---- 视频文案提取（向导第①②步）----
  // 提取说话内容：三选一 video_url 直链 / share_url 分享链 / asset_url 本站上传资产
  extractTranscript: (data: { video_url?: string; share_url?: string; asset_url?: string; title?: string }) =>
    apiClient.post<unknown, { raw_text: string; raw_text_lines?: string[]; title: string; method: string }>('/api/v1/generation/transcript/extract', data),
  // 异步提取（长视频防 120s 连接超时）：提交拿 task_id，轮询 getTranscriptTask
  extractTranscriptAsync: (data: { video_url?: string; share_url?: string; title?: string }) =>
    apiClient.post<unknown, { task_id: string; status: string }>('/api/v1/generation/transcript/extract/async', data),
  getTranscriptTask: (id: string) =>
    apiClient.get<unknown, { status?: string; task_id?: string; raw_text?: string; raw_text_lines?: string[]; title?: string; method?: string }>(`/api/v1/generation/transcript/extract/tasks/${id}`),

  // 原文 → 双产出（clean=用原文按钮 / rewrite=默认填入）
  rewriteScript: (data: { raw_text: string; topic?: string; requirement?: string }) =>
    apiClient.post<unknown, { clean: string; rewrite: string }>('/api/v1/generation/transcript/rewrite', data),

  // ---- 口播 B-Roll（22/23 号计划：成片后按句插入画面）----
  // 台词时间轴：首次点「插入画面」时 POST 定位（静音检测，秒级）；支持重跑或仅修正文字（lines_override 不改切换点）
  locateTaskTimeline: (taskId: string, body?: { force?: boolean; lines_override?: { index: number; text: string }[] }) =>
    apiClient.post<unknown, TaskTimeline>(`/api/v1/generation/tasks/${taskId}/timeline`, body || {}),
  // 读取已定位时间轴（未定位时服务端 404）
  getTaskTimeline: (taskId: string) =>
    apiClient.get<unknown, TaskTimeline>(`/api/v1/generation/tasks/${taskId}/timeline`),
  // 提交插入合成（sub_type=compose 走统一提交分发；source_task_id/segments 须在 params 内；只传句号，时间窗由后端换算）
  submitCompose: (data: { source_task_id: string; segments: { sentence_index: number; media_url: string }[] }) =>
    apiClient.post<unknown, GenerationTask>('/api/v1/generation/submit', { sub_type: 'compose', params: data }),

  // 素材库（上传/列表/删除——本地托管，P2 换 OSS 前端零改动）
  uploadAsset: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return apiClient.post<unknown, { id: string; url: string; mime: string; size_bytes: number; owner_type: string }>('/api/v1/media/assets', form)
  },
  listAssets: (owner?: 'material' | 'creation' | 'all') =>
    apiClient.get<unknown, { assets: MediaAsset[] }>(`/api/v1/media/assets${owner && owner !== 'material' ? `?owner=${owner}` : ''}`).then((r) => r.assets),
  deleteAsset: (id: string) =>
    apiClient.delete<unknown, { deleted: string }>(`/api/v1/media/assets/${id}`),

  // 厂商配置（管理后台：按厂商设置 API Key / 启用开关——保存后热生效）
  listProviderConfigs: () =>
    apiClient.get<unknown, { providers: ProviderConfig[] }>('/api/v1/admin/provider-configs'),
  saveProviderConfig: (provider: string, data: { api_key?: string; base_url?: string; enabled?: boolean; extra_json?: string }) =>
    apiClient.put<unknown, { providers: ProviderConfig[] }>(`/api/v1/admin/provider-configs/${provider}`, data),
  // 厂商剩余积分（排查 CreditInsufficient；mock 模式返回演示值）
  getProviderCredits: (provider: string) =>
    apiClient.get<unknown, { provider: string; credits: number }>(`/api/v1/admin/provider-configs/${provider}/credits`),

  // 生成规格（管理后台：Vidu 端点×模型矩阵——DB 驱动 30s 热生效）
  // 品牌知识库（获客智能体转型：商户上传品牌文档，内容生成自动引用）
  listBrandKnowledge: (brandId: string) =>
    apiClient.get<unknown, { materials: Array<{ id: string; title: string; summary: string; has_vector: boolean; crawl_keyword: string; created_at: string }>; total: number }>(`/api/v1/merchant/brands/${brandId}/knowledge/materials`),
  uploadBrandKnowledge: (brandId: string, data: { title: string; content: string }) =>
    apiClient.post<unknown, { id: string; message: string }>(`/api/v1/merchant/brands/${brandId}/knowledge/materials`, data),
  deleteBrandKnowledge: (brandId: string, materialId: string) =>
    apiClient.delete<unknown, { deleted: boolean }>(`/api/v1/merchant/brands/${brandId}/knowledge/materials/${materialId}`),

  // 发布通道能力清单（能力驱动：平台过滤/动态检查清单）
  listPublishChannels: () =>
    apiClient.get<unknown, { channels: PublishChannelView[] }>('/api/v1/merchant/publish/channels'),

  // 多平台内容适配预览（ContentAdapter 只读）
  previewPublishAdapt: (data: { title?: string; content?: string; tags?: string[]; platforms: string[] }) =>
    apiClient.post<unknown, { previews: Array<{
      platform: string
      title?: string
      description?: string
      tags?: string[]
      cta?: string
      title_truncated?: boolean
      error?: string
    }> }>('/api/v1/merchant/publish/adapt-preview', data),

  getPublishDraft: (brandId: string) =>
    apiClient.get<unknown, { draft: string | null; updated_at?: string }>('/api/v1/merchant/publish/draft', { params: { brand_id: brandId } }),

  savePublishDraft: (brandId: string, draft: string) =>
    apiClient.put<unknown, { saved: boolean }>('/api/v1/merchant/publish/draft', { brand_id: brandId, draft }),

  deletePublishDraft: (brandId: string) =>
    apiClient.delete<unknown, { deleted: boolean }>('/api/v1/merchant/publish/draft', { params: { brand_id: brandId } }),

  getBrandPublishStats: (brandId: string) =>
    apiClient.get<unknown, {
      brand_id: string
      daily_usage: Record<string, number>
      quotas: Array<{ platform: string; used_today: number; max_per_day: number; remaining: number; at_limit: boolean }>
    }>(`/api/v1/merchant/brands/${brandId}/publish-stats`),

  // 生成模式开关（admin：sub_type 批量启停——商户端模式收敛）
  adminListGenerationModes: () =>
    apiClient.get<unknown, { modes: GenerationModeView[] }>('/api/v1/admin/generation/modes'),
  adminSetGenerationMode: (subType: string, enabled: boolean) =>
    apiClient.put<unknown, { saved: number }>(`/api/v1/admin/generation/modes/${subType}`, { enabled }),

  adminListGenerationSpecs: () =>
    apiClient.get<unknown, { specs: GenerationSpec[] }>('/api/v1/admin/generation/specs').then((r) => r.specs),
  adminSaveGenerationSpec: (subType: string, model: string, body: Partial<GenerationSpec>) =>
    apiClient.put<unknown, { saved: boolean }>(`/api/v1/admin/generation/specs/${subType}/${model}`, body),
  adminDeleteGenerationSpec: (subType: string, model: string) =>
    apiClient.delete<unknown, { deleted: boolean }>(`/api/v1/admin/generation/specs/${subType}/${model}`),
  adminSetDefaultModel: (subType: string, model: string, provider?: string) =>
    apiClient.put<unknown, { default: string }>(`/api/v1/admin/generation/specs/${subType}/${model}/default`, null, { params: { provider } }),

  // ---- LLM 默认模型切换 ----
  setLLMDefault: (name: string) =>
    apiClient.put<unknown, { default: string }>(`/api/v1/admin/llm-configs/${name}/default`),

  // ---- 第三方集成中心（08 计划 D7——能力路由模型）----
  // 列表（view=vendor 按厂商 / view=capability 按能力）
  listIntegrations: (view?: 'vendor' | 'capability') =>
    apiClient.get<unknown, { integrations: IntegrationEntry[]; groups: IntegrationGroup[]; view: string }>('/api/v1/admin/integrations', { params: { view } }),

  // 厂商详情（聚合所有区块数据）
  getIntegrationDetail: (id: string) =>
    apiClient.get<unknown, { meta: IntegrationMeta; sections: Record<string, any> }>(`/api/v1/admin/integrations/${id}`),

  // 厂商健康检查
  getIntegrationHealth: (id: string) =>
    apiClient.get<unknown, { id: string; status: string; detail: string; checked_at: string }>(`/api/v1/admin/integrations/${id}/health`),

  // Vidu 首选模型配置（D3 自动切换可配化）
  setViduPreferredModel: (data: { image_subject?: string; video_subject?: string }) =>
    apiClient.put<unknown, { saved: boolean }>('/api/v1/admin/integrations/vidu/preferred-model', data),

  // ---- 能力路由管理（新表 integration_vendors + integration_capabilities）----
  // 全部厂商（含能力条目）
  listIntegrationVendors: () =>
    apiClient.get<unknown, { vendors: Array<IntegrationVendor & { capabilities: IntegrationCapability[] }> }>('/api/v1/admin/integrations/vendors'),

  // 保存厂商（启停/改 Key/改端点）
  saveIntegrationVendor: (id: string, data: { name?: string; base_url?: string; api_key?: string; protocol?: string; enabled?: boolean }) =>
    apiClient.put<unknown, { saved: string }>(`/api/v1/admin/integrations/vendors/${id}`, data),

  // 全部能力路由
  listIntegrationCapabilities: () =>
    apiClient.get<unknown, { capabilities: IntegrationCapability[] }>('/api/v1/admin/integrations/capabilities'),

  // 设置某能力的默认厂商（同 capId 下互斥）
  setCapabilityDefault: (capId: string, vendorId: string) =>
    apiClient.put<unknown, { cap_id: string; default_vendor: string }>(`/api/v1/admin/integrations/capabilities/${capId}/default`, { cap_id: capId, vendor_id: vendorId }),

  // 保存能力条目（新增/编辑——id 传完整复合 ID 如 "asr#siliconflow"）
  saveIntegrationCapability: (id: string, data: { cap_id?: string; vendor_id?: string; model?: string; endpoint?: string; extra_json?: string; enabled?: boolean; is_default?: boolean }) =>
    apiClient.put<unknown, { saved: string }>('/api/v1/admin/integrations/capabilities/save', { id, ...data }),

  // 删除能力条目（id 传完整复合 ID 如 "asr#siliconflow"）
  deleteIntegrationCapability: (id: string) =>
    apiClient.delete<unknown, { deleted: string }>('/api/v1/admin/integrations/capabilities/delete', { data: { id } }),

  // ---- 经济系统（套餐/订阅/订单/用量）----

  // 商户端
  listActivePlans: () =>
    apiClient.get<unknown, { plans: Plan[] }>('/api/v1/billing/plans'),
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

  // admin 生成模板管理
  adminListGenerationTemplates: () =>
    apiClient.get<unknown, { templates: GenerationTemplate[] }>('/api/v1/admin/templates').then((r) => r.templates),
  adminCreateGenerationTemplate: (data: Partial<GenerationTemplate>) =>
    apiClient.post<unknown, GenerationTemplate>('/api/v1/admin/templates', data),
  adminUpdateGenerationTemplate: (id: string, data: Partial<GenerationTemplate>) =>
    apiClient.put<unknown, GenerationTemplate>(`/api/v1/admin/templates/${id}`, data),
  adminDeleteGenerationTemplate: (id: string) =>
    apiClient.delete<unknown, { deleted: string }>(`/api/v1/admin/templates/${id}`),

  // ---- 爬虫管理 API ----
  // 平台方账号管理
  adminListCrawlerAccounts: () =>
    apiClient.get<unknown, { accounts: CrawlerAccount[] }>('/api/v1/admin/crawler-accounts'),
  adminCreateCrawlerAccount: (data: Partial<CrawlerAccount>) =>
    apiClient.post<unknown, { msg: string }>('/api/v1/admin/crawler-accounts', data),
  adminUpdateCrawlerAccount: (id: number, data: Partial<CrawlerAccount>) =>
    apiClient.put<unknown, { msg: string }>(`/api/v1/admin/crawler-accounts/${id}`, data),
  adminDeleteCrawlerAccount: (id: number) =>
    apiClient.delete<unknown, { msg: string }>(`/api/v1/admin/crawler-accounts/${id}`),
  adminCheckCrawlerAccountHealth: (id: number, platform: string) =>
    apiClient.post<unknown, { account_id: number; platform: string; healthy: boolean; result: string; reason?: string }>(
      `/api/v1/admin/crawler-accounts/${id}/health?platform=${platform}`
    ),

  // 爬虫配置管理
  adminListCrawlerConfigs: () =>
    apiClient.get<unknown, { configs: CrawlerConfig[] }>('/api/v1/admin/crawlers'),
  adminGetCrawlerConfig: (platform: string) =>
    apiClient.get<unknown, CrawlerConfig>(`/api/v1/admin/crawlers/${platform}`),
  adminUpdateCrawlerConfig: (platform: string, data: Partial<CrawlerConfig>) =>
    apiClient.put<unknown, { msg: string }>(`/api/v1/admin/crawlers/${platform}`, data),
  adminTestCrawlerConnection: (platform: string) =>
    apiClient.post<unknown, { platform: string; alive: boolean }>(`/api/v1/admin/crawlers/${platform}/test`),
  adminTriggerCrawl: (platform: string, data: { brand_id: string; keywords: string[] }) =>
    apiClient.post<unknown, CrawlResult>(`/api/v1/admin/crawlers/${platform}/trigger`, data),

  // 任务监控
  adminListCrawlerTasks: (limit?: number) =>
    apiClient.get<unknown, { tasks: CrawlerTaskLog[] }>(`/api/v1/admin/crawlers/tasks?limit=${limit || 50}`),
  adminGetCrawlerTask: (id: number) =>
    apiClient.get<unknown, CrawlerTaskLog>(`/api/v1/admin/crawlers/tasks/${id}`),

  // 用户端灵感 API
  listInspirations: (params?: { brand_id?: string; platform?: string; keyword?: string; sort_by?: string; page?: number; page_size?: number }) =>
    apiClient.get<unknown, { total: number; page: number; page_size: number; items: InspirationVideo[]; status?: string; message?: string }>('/api/v1/inspirations', { params }),
  getInspiration: (id: string) =>
    apiClient.get<unknown, InspirationVideo>(`/api/v1/inspirations/${id}`),
  listInspirationPlatforms: () =>
    apiClient.get<unknown, { platforms: string[] }>('/api/v1/inspirations/platforms'),

  // Admin 灵感运营
  adminInspirationStats: () =>
    apiClient.get<unknown, { total_videos: number; total_brands: number; by_platform: { platform: string; count: number }[]; by_brand: { brand_id: string; count: number }[] }>('/api/v1/admin/inspirations/stats'),
  adminUpdateInspiration: (id: string, data: { is_pinned?: boolean; is_recommended?: boolean; admin_note?: string }) =>
    apiClient.put<unknown, InspirationVideo>(`/api/v1/admin/inspirations/${id}`, data),
  adminDeleteInspiration: (id: string) =>
    apiClient.delete<unknown, { msg: string; id: string }>(`/api/v1/admin/inspirations/${id}`),
  adminBatchInspirations: (data: { ids: string[]; action: 'delete' | 'pin' | 'recommend' }) =>
    apiClient.post<unknown, { msg: string; affected: number }>('/api/v1/admin/inspirations/batch', data),

  // 品牌发布配置
  getBrandPublishConfigs: (brandId: string) =>
    apiClient.get<unknown, BrandPublishConfig[]>(`/api/v1/merchant/brands/${brandId}/publish-config`),
  updateBrandPublishConfig: (brandId: string, config: Partial<BrandPublishConfig>) =>
    apiClient.put<unknown, BrandPublishConfig>(`/api/v1/merchant/brands/${brandId}/publish-config`, config),
  deleteBrandPublishConfig: (brandId: string, platform: string) =>
    apiClient.delete<unknown, void>(`/api/v1/merchant/brands/${brandId}/publish-config/${platform}`),
  bindAccountToBrand: (brandId: string, data: { account_id: string; platform: string; is_default: boolean }) =>
    apiClient.post<unknown, AccountBrandBinding>(`/api/v1/merchant/brands/${brandId}/publish-config/bindings`, data),
  unbindAccountFromBrand: (brandId: string, accountId: string) =>
    apiClient.delete<unknown, void>(`/api/v1/merchant/brands/${brandId}/publish-config/bindings/${accountId}`),
  getPublishStats: (brandId: string) =>
    apiClient.get<unknown, {
      brand_id: string
      daily_usage: Record<string, number>
      quotas: Array<{ platform: string; used_today: number; max_per_day: number; remaining: number; at_limit: boolean }>
    }>(`/api/v1/merchant/brands/${brandId}/publish-stats`),
}

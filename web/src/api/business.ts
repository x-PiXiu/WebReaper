import { apiClient } from './client'
import type { TaskView, AgentConfig, LLMConfig, Collection, DataItem, Conversation, ChatMessageRecord, CrawlConfig, ExternalSystem, PublishResult, PublishRecord, ToolView } from '../types/api'

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

  deleteAgentConfig: (name: string) =>
    apiClient.delete<unknown, unknown>(`/api/v1/agents/${name}`),

  // ---- LLM 配置 ----
  listLLMConfigs: () =>
    apiClient.get<unknown, LLMConfig[]>('/api/v1/llm-configs'),

  createLLMConfig: (data: LLMConfig) =>
    apiClient.post<unknown, LLMConfig>('/api/v1/llm-configs', data),

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
    apiClient.get<unknown, { name: string; description: string }[]>('/api/v1/tools'),

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

  // 把 LLM 对话生成的结构化内容落库为 DataItem（打通"对话生成→自动落库"闭环）
  createDataItemFromContent: (data: { content: string; field_mapping?: string; source_url?: string }) =>
    apiClient.post<unknown, DataItem>('/api/v1/data-items/from-content', data),

  // ---- 工具面板 ----
  listTools: () =>
    apiClient.get<unknown, ToolView[]>('/api/v1/tools'),

  toggleTool: (name: string, enabled: boolean) =>
    apiClient.put<unknown, { name: string; enabled: boolean }>(`/api/v1/tools/${name}/toggle`, { enabled }),
}

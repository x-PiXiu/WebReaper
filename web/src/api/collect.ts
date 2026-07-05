import { apiClient } from './client'
import type { EnqueueTaskRequest, EnqueueTaskResponse } from '../types/api'

// 任务投递 API。

export const collectApi = {
  enqueueTask: (data: EnqueueTaskRequest) =>
    apiClient.post<unknown, EnqueueTaskResponse>('/api/v1/tasks', data),
}

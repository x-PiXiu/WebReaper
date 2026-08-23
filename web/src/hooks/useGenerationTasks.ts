import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { GenerationTask } from '../types/api'

export const GENERATION_TASKS_KEY = ['generation-tasks'] as const

const PENDING_STATES = new Set(['created', 'queueing', 'processing'])

export function isGenerationTaskPending(state: string) {
  return PENDING_STATES.has(state)
}

/** 拉取生成任务列表（React Query 缓存共享） */
export async function fetchGenerationTasks(): Promise<GenerationTask[]> {
  const res = await businessApi.listGenerationTasks()
  return res.tasks
}

type Options = {
  /** false 禁用轮询；数字为固定间隔 ms */
  refetchInterval?: number | false
  enabled?: boolean
}

/**
 * 统一生成任务查询：有进行中任务时自动 3s 轮询，否则停止。
 */
export function useGenerationTasks(opts?: Options) {
  const { data, ...rest } = useQuery({
    queryKey: GENERATION_TASKS_KEY,
    queryFn: fetchGenerationTasks,
    enabled: opts?.enabled !== false,
    staleTime: 2000,
    refetchInterval: (query) => {
      if (opts?.refetchInterval === false) return false
      if (typeof opts?.refetchInterval === 'number') return opts.refetchInterval
      const tasks = query.state.data ?? []
      return tasks.some(t => isGenerationTaskPending(t.state)) ? 3000 : false
    },
  })

  const tasks = data ?? []
  return { tasks, ...rest }
}

export function useGenerationTask(taskId: string | undefined) {
  const { tasks, ...rest } = useGenerationTasks()
  const task = taskId ? tasks.find(t => t.id === taskId) : undefined
  return { task, tasks, ...rest }
}

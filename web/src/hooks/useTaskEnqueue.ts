import { useState, useCallback } from 'react'
import { message } from 'antd'
import { useQueryClient } from '@tanstack/react-query'
import { collectApi } from '../api/collect'

// useTaskEnqueue 封装异步任务投递，全局可复用。
//
// 设计动机（DRY + 单一职责）：
//   - 异步采集是横切能力：Chat 页的"后台采集"、Tasks 页的投递、未来多 Agent 编排都会用。
//   - 把投递逻辑、loading 态、成功/失败提示、列表刷新集中在这一处，
//     调用方只需 const { enqueue, loading } = useTaskEnqueue()。
//
// 用法：
//   const { enqueue, loading } = useTaskEnqueue()
//   await enqueue({ task: '采集某网站', tools: ['static_crawler'] })
//
// 投递成功后自动：
//   1. message.success 提示 task_id
//   2. invalidateQueries(['tasks']) 让任务监控页刷新
export function useTaskEnqueue() {
  const queryClient = useQueryClient()
  const [loading, setLoading] = useState(false)

  const enqueue = useCallback(async (params: {
    task: string
    tools?: string[]
    systemPrompt?: string
  }): Promise<string | null> => {
    setLoading(true)
    try {
      const res = await collectApi.enqueueTask({
        type: 'agent_run',
        input: {
          task: params.task,
          tools: params.tools || [],
          system_prompt: params.systemPrompt || '',
        },
      })
      message.success(`已提交后台采集任务：${res.task_id.slice(0, 8)}…（可在「任务监控」查看进度）`)
      // 刷新任务列表，让监控页立即看到新任务
      queryClient.invalidateQueries({ queryKey: ['tasks'] })
      return res.task_id
    } catch {
      // axios 拦截器已统一提示错误
      return null
    } finally {
      setLoading(false)
    }
  }, [queryClient])

  return { enqueue, loading }
}

import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { retryFailureMessage } from '../components/RetryHint'
import { isTaskDone, isTaskSuccess, taskPrimaryUrl } from '../utils/generationTask'
import { GENERATION_TASKS_KEY, useGenerationTask } from './useGenerationTasks'
import { message } from '../utils/antdApp'

type WatchOpts = {
  taskId?: string
  label?: string
  /** 成功时回调（已解析主产物 URL） */
  onSuccess?: (url: string) => void
  /** 失败时回调 */
  onFailed?: (msg: string) => void
  /** 是否 toast 提示（默认 true） */
  notify?: boolean
}

/**
 * 监听单个生成任务终态（依赖 useGenerationTasks 轮询）。
 * 适合提交后写入 taskId、由父组件 patch 草稿 URL 的场景。
 */
export function useGenerationTaskWatch({
  taskId,
  label = '生成',
  onSuccess,
  onFailed,
  notify = true,
}: WatchOpts) {
  const { task } = useGenerationTask(taskId)
  const queryClient = useQueryClient()
  const handled = useRef<string>('')

  useEffect(() => {
    if (!taskId || !task || !isTaskDone(task.state)) return
    if (handled.current === `${taskId}:${task.state}`) return
    handled.current = `${taskId}:${task.state}`

    if (isTaskSuccess(task.state)) {
      const url = taskPrimaryUrl(task)
      if (url) {
        onSuccess?.(url)
        if (notify) message.success(`${label}已完成`)
      }
      queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
      return
    }

    const msg = retryFailureMessage(task, `${label}失败`)
    onFailed?.(msg)
    if (notify) message.error(msg)
  }, [task, taskId, label, onSuccess, onFailed, notify, queryClient])

  const pending = !!(taskId && task && !isTaskDone(task.state))
  const success = !!(taskId && task && isTaskSuccess(task.state))
  const failed = !!(taskId && task && task.state === 'failed')

  return { task, pending, success, failed, url: task ? taskPrimaryUrl(task) : null }
}

import { isGenerationTaskPending, useGenerationTask } from '../../hooks/useGenerationTasks'
import { isTaskDone, isTaskSuccess } from '../../utils/generationTask'
import { generationPendingLabel } from '../../utils/generationTaskLabel'
import { retryFailureMessage } from '../RetryHint'
import { GenerationFailedBar } from './GenerationFailedBar'
import { TaskStatusBar } from './TaskStatusBar'

type Props = {
  taskId?: string
  resultReady?: boolean
  doneLabel?: string
  fallbackPending?: string
  onClearFailed?: () => void
  onRetry?: () => void
}

/** 绑定单个生成任务的状态条（阶段文案 + 失败提示） */
export function GenerationTaskStatusBar({
  taskId,
  resultReady,
  doneLabel = '已完成',
  fallbackPending = '生成中',
  onClearFailed,
  onRetry,
}: Props) {
  const { task } = useGenerationTask(taskId)

  if (resultReady) {
    return <TaskStatusBar done doneLabel={doneLabel} />
  }

  if (!taskId) return null

  if (task && task.state === 'failed') {
    return (
      <GenerationFailedBar
        message={retryFailureMessage(task, `${fallbackPending}失败`)}
        onClear={onClearFailed}
        onRetry={onRetry}
      />
    )
  }

  const pending = !task || isGenerationTaskPending(task.state) || !isTaskDone(task.state)
  if (!pending) {
    if (task && isTaskSuccess(task.state)) {
      return <TaskStatusBar done doneLabel={doneLabel} />
    }
    return null
  }

  return (
    <TaskStatusBar
      pending
      pendingLabel={generationPendingLabel(task, fallbackPending)}
    />
  )
}

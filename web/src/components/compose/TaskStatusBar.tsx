import { Spin } from 'antd'

type Props = {
  pending?: boolean
  done?: boolean
  pendingLabel?: string
  doneLabel?: string
}

/** 生成任务状态条（生成中 / 已完成） */
export function TaskStatusBar({ pending, done, pendingLabel = '生成中…', doneLabel = '已完成' }: Props) {
  if (!pending && !done) return null
  const state = pending ? 'pending' : 'done'
  return (
    <div className={`cf-task-bar cf-task-${state}`}>
      {pending && <Spin size="small" />}
      <span>{pending ? pendingLabel : doneLabel}</span>
    </div>
  )
}

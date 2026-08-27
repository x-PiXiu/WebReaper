import { Button } from 'antd'
import { CloseCircleOutlined, ReloadOutlined } from '@ant-design/icons'

type Props = {
  message: string
  onClear?: () => void
  clearLabel?: string
  onRetry?: () => void
  retryLabel?: string
  compact?: boolean
}

/** 生成任务失败条（步骤内 / 列表项共用） */
export function GenerationFailedBar({
  message,
  onClear,
  clearLabel = '清除并重试',
  onRetry,
  retryLabel = '重试',
  compact,
}: Props) {
  return (
    <div className={`cf-task-bar cf-task-failed${compact ? ' cf-task-failed--compact' : ''}`}>
      <CloseCircleOutlined />
      <span className="cf-task-failed-msg">{message}</span>
      <span className="cf-task-failed-actions">
        {onRetry && (
          <Button type="link" size="small" icon={<ReloadOutlined />} onClick={onRetry}>
            {retryLabel}
          </Button>
        )}
        {onClear && (
          <Button type="link" size="small" onClick={onClear}>
            {clearLabel}
          </Button>
        )}
      </span>
    </div>
  )
}

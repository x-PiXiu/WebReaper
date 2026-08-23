import { useNavigate } from 'react-router-dom'
import { Button, Space, Typography } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { useGenerationTask, isGenerationTaskPending } from '../../hooks/useGenerationTasks'
import { taskPrimaryUrl } from '../../utils/generationTask'

const { Text } = Typography

const SUB_TYPE_LABELS: Record<string, string> = {
  lip_sync: '对口型', tts: '语音合成', reference2video: '参考生视频',
  text2video: '文生视频', img2video: '图生视频', text2image: '图片生成',
}

type Props = {
  taskId: string
  subType?: string
  initialStatus?: string
}

/** 对话中 generate_content 返回的任务跟踪卡片 */
export function GenerationTaskCard({ taskId, subType, initialStatus }: Props) {
  const navigate = useNavigate()
  const { task } = useGenerationTask(taskId)
  const state = task?.state || initialStatus || 'created'
  const pending = isGenerationTaskPending(state)
  const success = state === 'success'
  const failed = state === 'failed' || state === 'cancelled'
  const label = SUB_TYPE_LABELS[subType || task?.sub_type || ''] || subType || task?.sub_type || '内容生成'
  const url = task ? taskPrimaryUrl(task) : null

  return (
    <div className="chat-gen-task-card">
      <div className="chat-gen-task-head">
        <Space>
          {pending && <LoadingOutlined spin style={{ color: 'var(--wr-accent)' }} />}
          {success && <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} />}
          {failed && <CloseCircleOutlined style={{ color: 'var(--wr-error)' }} />}
          <Text strong>✨ {label}</Text>
        </Space>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {pending ? '生成中…' : success ? '已完成' : failed ? '失败' : state}
        </Text>
      </div>
      {task?.err_msg && failed && (
        <Text type="danger" style={{ fontSize: 12, display: 'block', marginTop: 6 }}>{task.err_msg}</Text>
      )}
      <Space wrap style={{ marginTop: 10 }}>
        {success && url && (
          <Button size="small" type="primary" href={url} target="_blank" rel="noopener noreferrer">查看成片</Button>
        )}
        {success && (
          <Button size="small" onClick={() => navigate('/m/distribution')}>去发布</Button>
        )}
        <Button size="small" type="link" onClick={() => navigate('/m/compose/tools?tab=media')}>任务中心</Button>
      </Space>
    </div>
  )
}

/** 从工具返回文本解析 task_id（JSON 或键值对） */
export function parseGenerateContentResult(raw: string): { taskId?: string; subType?: string; status?: string } {
  const trimmed = raw.trim()
  if (!trimmed) return {}
  try {
    const j = JSON.parse(trimmed)
    return {
      taskId: j.task_id || j.taskId,
      subType: j.sub_type || j.subType,
      status: j.status || j.state,
    }
  } catch {
    const idMatch = trimmed.match(/task_id["\s:=]+["']?([a-zA-Z0-9._-]+)/i)
    const subMatch = trimmed.match(/sub_type["\s:=]+["']?([a-zA-Z0-9_]+)/i)
    return {
      taskId: idMatch?.[1],
      subType: subMatch?.[1],
    }
  }
}

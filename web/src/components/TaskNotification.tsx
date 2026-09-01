import { useEffect, useState } from 'react'
import { Badge, Card, Space, Typography, Button, Progress } from 'antd'
import { BellOutlined, CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { useGenerationTasks } from '../hooks/useGenerationTasks'
import { notification } from '../utils/antdApp'
import type { GenerationTask } from '../types/api'

const { Text } = Typography

const TASK_STATE_LABELS: Record<string, string> = {
  created: '已创建',
  queueing: '排队中',
  processing: '生成中',
  success: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

function taskTitle(task: GenerationTask) {
  const type = task.sub_type || task.type || '内容'
  return String(type)
}

export default function TaskNotification() {
  const [notifiedTasks, setNotifiedTasks] = useState<Set<string>>(new Set())
  const { tasks } = useGenerationTasks()

  useEffect(() => {
    tasks.forEach((task) => {
      const isTerminal = ['success', 'failed', 'cancelled'].includes(task.state)
      const alreadyNotified = notifiedTasks.has(task.id)

      if (isTerminal && !alreadyNotified) {
        const isSuccess = task.state === 'success'
        notification.open({
          message: isSuccess ? '生成完成' : task.state === 'cancelled' ? '生成已取消' : '生成未成功',
          description: (
            <Space direction="vertical" size={4}>
              <Text>{taskTitle(task)} · {TASK_STATE_LABELS[task.state]}</Text>
              {task.err_msg && (
                <Text type="danger" style={{ fontSize: 12 }}>
                  {task.err_msg.length > 80 ? `${task.err_msg.slice(0, 80)}…` : task.err_msg}
                </Text>
              )}
              {isSuccess && (
                <Button type="link" size="small" style={{ padding: 0, height: 'auto' }} onClick={() => { window.location.href = '/m/works' }}>
                  去作品库查看
                </Button>
              )}
            </Space>
          ),
          icon: isSuccess
            ? <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} />
            : <CloseCircleOutlined style={{ color: 'var(--wr-danger)' }} />,
          duration: isSuccess ? 4.5 : 6,
          key: `gen-${task.id}`,
        })
        setNotifiedTasks((prev) => new Set(prev).add(task.id))
      }
    })
  }, [tasks, notifiedTasks])

  const pendingCount = tasks.filter((t) => ['created', 'queueing', 'processing'].includes(t.state)).length

  return (
    <Badge count={pendingCount} size="small">
      <Button
        type="text"
        icon={<BellOutlined />}
        aria-label="生成任务"
        onClick={() => { window.location.href = '/m/compose/tools?tab=media' }}
      />
    </Badge>
  )
}

export function TaskStatusCard({ task }: { task: GenerationTask }) {
  const isSuccess = task.state === 'success'
  const isProcessing = ['created', 'queueing', 'processing'].includes(task.state)
  const isFailed = task.state === 'failed'

  return (
    <Card size="small" style={{ marginBottom: 8 }}>
      <Space direction="vertical" size={4} style={{ width: '100%' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Space>
            {isProcessing && <LoadingOutlined spin />}
            {isSuccess && <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} />}
            {isFailed && <CloseCircleOutlined style={{ color: 'var(--wr-danger)' }} />}
            <Text strong>{TASK_STATE_LABELS[task.state]}</Text>
          </Space>
          <Text type="secondary" style={{ fontSize: 12 }}>{task.sub_type}</Text>
        </div>

        {isProcessing && (
          <Progress percent={task.state === 'processing' ? 60 : 30} size="small" status="active" />
        )}

        {isSuccess && task.creations?.length > 0 && (
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {task.creations.map((creation, index) => (
              <a
                key={index}
                href={creation.stored_url || creation.url}
                target="_blank"
                rel="noopener noreferrer"
                style={{ fontSize: 12 }}
              >
                查看结果
              </a>
            ))}
          </div>
        )}

        {isFailed && task.err_msg && (
          <Text type="danger" style={{ fontSize: 12 }}>{task.err_msg}</Text>
        )}
      </Space>
    </Card>
  )
}

export function TaskList() {
  const { tasks, isLoading } = useGenerationTasks()

  if (isLoading) return <div>加载中...</div>

  if (tasks.length === 0) {
    return (
      <div style={{ textAlign: 'center', padding: 20 }}>
        <Text type="secondary">暂无任务</Text>
      </div>
    )
  }

  return (
    <div>
      {tasks.map((task) => <TaskStatusCard key={task.id} task={task} />)}
    </div>
  )
}

import { useEffect, useState } from 'react'
import { notification, Badge, Card, Space, Typography, Button, Progress } from 'antd'
import { BellOutlined, CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { GenerationTask } from '../types/api'

const { Text } = Typography

// 任务状态中文
const TASK_STATE_LABELS: Record<string, string> = {
  created: '已创建',
  queueing: '排队中',
  processing: '生成中',
  success: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

// 任务状态颜色
const TASK_STATE_COLORS: Record<string, string> = {
  created: 'default',
  queueing: 'processing',
  processing: 'processing',
  success: 'success',
  failed: 'error',
  cancelled: 'default',
}

// 任务通知组件
export default function TaskNotification() {
  const queryClient = useQueryClient()
  const [notifiedTasks, setNotifiedTasks] = useState<Set<string>>(new Set())

  // 查询任务列表
  const { data: tasksData } = useQuery({
    queryKey: ['generation-tasks'],
    queryFn: () => businessApi.listGenerationTasks(),
    refetchInterval: 5000, // 5秒轮询
  })

  const tasks = tasksData?.tasks || []

  // 检查新完成的任务并发送通知
  useEffect(() => {
    tasks.forEach(task => {
      // 只通知终态任务（成功/失败/取消）
      const isTerminal = ['success', 'failed', 'cancelled'].includes(task.state)
      const alreadyNotified = notifiedTasks.has(task.id)

      if (isTerminal && !alreadyNotified) {
        // 发送通知
        const isSuccess = task.state === 'success'
        notification.open({
          message: isSuccess ? '生成完成' : '生成失败',
          description: (
            <Space direction="vertical" size={4}>
              <Text>{TASK_STATE_LABELS[task.state]}</Text>
              <Text type="secondary" style={{ fontSize: 12 }}>
                任务ID: {task.id}
              </Text>
              {task.err_msg && (
                <Text type="danger" style={{ fontSize: 12 }}>
                  {task.err_msg}
                </Text>
              )}
            </Space>
          ),
          icon: isSuccess ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : <CloseCircleOutlined style={{ color: '#ff4d4f' }} />,
          duration: 5,
        })

        // 标记为已通知
        setNotifiedTasks(prev => new Set(prev).add(task.id))
      }
    })
  }, [tasks, notifiedTasks])

  // 统计任务状态
  const pendingCount = tasks.filter(t => ['created', 'queueing', 'processing'].includes(t.state)).length
  const completedCount = tasks.filter(t => t.state === 'success').length
  const failedCount = tasks.filter(t => t.state === 'failed').length

  return (
    <Badge count={pendingCount} size="small">
      <Button
        type="text"
        icon={<BellOutlined />}
        onClick={() => {
          // 跳转到任务列表
          window.location.href = '/m/compose/tools?tab=media'
        }}
      />
    </Badge>
  )
}

// 任务状态卡片组件
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
            {isSuccess && <CheckCircleOutlined style={{ color: '#52c41a' }} />}
            {isFailed && <CloseCircleOutlined style={{ color: '#ff4d4f' }} />}
            <Text strong>{TASK_STATE_LABELS[task.state]}</Text>
          </Space>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {task.sub_type}
          </Text>
        </div>

        {isProcessing && (
          <Progress
            percent={task.state === 'processing' ? 60 : 30}
            size="small"
            status="active"
          />
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
          <Text type="danger" style={{ fontSize: 12 }}>
            {task.err_msg}
          </Text>
        )}
      </Space>
    </Card>
  )
}

// 任务列表组件
export function TaskList() {
  const { data: tasksData, isLoading } = useQuery({
    queryKey: ['generation-tasks'],
    queryFn: () => businessApi.listGenerationTasks(),
    refetchInterval: 5000,
  })

  const tasks = tasksData?.tasks || []

  if (isLoading) {
    return <div>加载中...</div>
  }

  if (tasks.length === 0) {
    return (
      <div style={{ textAlign: 'center', padding: 20 }}>
        <Text type="secondary">暂无任务</Text>
      </div>
    )
  }

  return (
    <div>
      {tasks.map(task => (
        <TaskStatusCard key={task.id} task={task} />
      ))}
    </div>
  )
}

import { useState } from 'react'
import { Card, Table, Tag, Typography, Empty, Spin, Button, Space } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import type { TaskView } from '../types/api'

const { Title, Text } = Typography

const statusColor: Record<string, string> = {
  pending: 'default',
  running: 'processing',
  succeeded: 'success',
  failed: 'error',
  cancelled: 'warning',
}
const statusLabel: Record<string, string> = {
  pending: '等待中', running: '执行中', succeeded: '成功', failed: '失败', cancelled: '已取消',
}

export default function Tasks() {
  const [expandedKeys, setExpandedKeys] = useState<string[]>([])

  // 任务列表：运行中有任务时自动轮询（5s），否则手动刷新
  const { data: tasks = [], isLoading, isError, refetch } = useQuery({
    queryKey: ['tasks'],
    queryFn: () => businessApi.listTasks(),
    refetchInterval: (query) => {
      const list = query.state.data as TaskView[] | undefined
      // 有运行中/等待中的任务时自动刷新
      const hasActive = list?.some(t => t.status === 'running' || t.status === 'pending')
      return hasActive ? 5000 : false
    },
  })

  const activeCount = tasks.filter(t => t.status === 'running' || t.status === 'pending').length

  const columns = [
    {
      title: '任务ID', dataIndex: 'id', key: 'id', width: 180,
      render: (id: string) => <Text code style={{ fontSize: 12 }}>{id.length > 16 ? id.slice(0, 8) + '…' + id.slice(-4) : id}</Text>,
    },
    {
      title: '类型', dataIndex: 'type', key: 'type', width: 120,
      render: (t: string) => <Tag>{t}</Tag>,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 100,
      render: (s: string, record: TaskView) => (
        <Space direction="vertical" size={2}>
          <Tag color={statusColor[s] || 'default'} icon={s === 'running' ? <Spin size="small" /> : undefined}>
            {statusLabel[s] || s}
          </Tag>
          {s === 'running' && record.progress && (
            <Text type="secondary" style={{ fontSize: 11 }}>{record.progress}</Text>
          )}
        </Space>
      ),
    },
    {
      title: '错误', dataIndex: 'error', key: 'error', ellipsis: true,
      render: (e: string) => e ? <Text type="danger" style={{ fontSize: 12 }}>{e}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: '操作', key: 'action', width: 80,
      render: (_: unknown, record: TaskView) =>
        (record.output || record.error) ? (
          <Button size="small" type="link" onClick={() => {
            const k = record.id
            setExpandedKeys(prev => prev.includes(k) ? prev.filter(x => x !== k) : [...prev, k])
          }}>
            {expandedKeys.includes(record.id) ? '收起' : '详情'}
          </Button>
        ) : <Text type="secondary">-</Text>,
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>任务监控</Title>
          <Text type="secondary" style={{ fontSize: 13 }}>
            查看异步采集任务的执行状态{activeCount > 0 ? `（${activeCount} 个进行中，自动刷新）` : ''}
          </Text>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>刷新</Button>
        </Space>
      </div>

      <Card>
        {isLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin tip="加载任务列表..." /></div>
        ) : isError ? (
          <div style={{ textAlign: 'center', padding: 48 }}>
            <Text type="danger">任务列表加载失败</Text>
            <div style={{ marginTop: 12 }}><Button onClick={() => refetch()}>重试</Button></div>
          </div>
        ) : tasks.length === 0 ? (
          <Empty description="暂无任务。在聊天页让 Agent 采集数据，或通过 API 投递任务后，这里会显示执行情况。" />
        ) : (
          <Table
            dataSource={tasks}
            columns={columns}
            rowKey="id"
            pagination={{ pageSize: 20, showSizeChanger: false }}
            size="middle"
            expandable={{
              expandedRowKeys: expandedKeys,
              onExpandedRowsChange: (keys) => setExpandedKeys(keys as string[]),
              expandRowByClick: false,
              expandedRowRender: (record: TaskView) => (
                <div style={{ padding: '8px 0' }}>
                  {record.output && (
                    <div style={{ marginBottom: 12 }}>
                      <Text strong>输出：</Text>
                      <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 13, background: 'var(--wr-bg-elevated)', padding: 12, borderRadius: 8, marginTop: 4, maxHeight: 400, overflowY: 'auto' }}>{record.output}</pre>
                    </div>
                  )}
                  {record.error && (
                    <div>
                      <Text type="danger" strong>错误详情：</Text>
                      <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 13, color: 'var(--wr-text-secondary)', marginTop: 4 }}>{record.error}</pre>
                    </div>
                  )}
                </div>
              ),
              rowExpandable: (record: TaskView) => !!(record.output || record.error),
            }}
          />
        )}
      </Card>
    </div>
  )
}

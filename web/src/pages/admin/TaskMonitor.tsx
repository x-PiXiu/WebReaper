import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Popconfirm, Segmented, Space, Table, Tag, Typography } from 'antd'
import { businessApi } from '../../api/business'
import { message } from '../../utils/antdApp'

const { Text } = Typography

const STATE_META: Record<string, { label: string; color: string }> = {
  created: { label: '排队中', color: 'default' },
  queueing: { label: '排队中', color: 'default' },
  processing: { label: '处理中', color: 'processing' },
  success: { label: '成功', color: 'green' },
  failed: { label: '失败', color: 'red' },
  cancelled: { label: '已取消', color: 'default' },
}

/**
 * 生成任务监控（管理后台——跨租户任务列表/筛选/取消）。
 */
function TaskMonitor({ embedded = false }: { embedded?: boolean }) {
  void embedded
  const queryClient = useQueryClient()
  const [state, setState] = useState('active')

  const { data, isLoading } = useQuery({
    queryKey: ['admin-tasks', state],
    queryFn: () => businessApi.adminListAllTasks({ state }).then((r) => r.tasks),
    refetchInterval: 10_000,
  })

  const doCancel = async (id: string) => {
    try {
      await businessApi.adminCancelTask(id)
      message.success('已取消')
      queryClient.invalidateQueries({ queryKey: ['admin-tasks'] })
    } catch { /* 拦截器已提示 */ }
  }

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Segmented
        value={state}
        onChange={(v) => setState(v as string)}
        options={[
          { value: 'active', label: '活跃任务' },
          { value: 'failed', label: '失败任务' },
        ]}
      />
      <Table
        rowKey="id"
        size="small"
        loading={isLoading}
        dataSource={data ?? []}
        pagination={{ pageSize: 20, showTotal: (t) => `共 ${t} 条` }}
        columns={[
          { title: '任务 ID', dataIndex: 'id', width: 200, render: (id: string) => <Text copyable style={{ fontSize: 12 }}>{id.slice(0, 24)}</Text> },
          { title: '租户', dataIndex: 'tenant_id', width: 140 },
          { title: '类型', dataIndex: 'sub_type', width: 100 },
          {
            title: '状态', dataIndex: 'state', width: 90, render: (st: string) => {
              const meta = STATE_META[st] ?? { label: st, color: 'default' }
              return <Tag color={meta.color}>{meta.label}</Tag>
            },
          },
          { title: '模型', dataIndex: 'model', width: 100 },
          { title: '错误', dataIndex: 'err_msg', ellipsis: true, render: (e: string) => e ? <Text type="danger" style={{ fontSize: 12 }}>{e}</Text> : '-' },
          { title: '创建时间', dataIndex: 'created_at', width: 140 },
          {
            title: '操作', width: 80, render: (_, r) =>
              !['success', 'failed', 'cancelled'].includes(r.state) ? (
                <Popconfirm title="确定取消？" okText="取消任务" okButtonProps={{ danger: true }} cancelText="返回" onConfirm={() => doCancel(r.id)}>
                  <Button size="small" danger>取消</Button>
                </Popconfirm>
              ) : null,
          },
        ]}
      />
    </Space>
  )
}

export default TaskMonitor

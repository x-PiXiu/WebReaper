import { useState } from 'react'
import { Typography, Card, Space, message, Tag, Modal, Input, Table, Button, Form, Select, InputNumber } from 'antd'
import { PlusOutlined, DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { CrawlerAccount } from '../../types/api'

const { Text } = Typography

const PLATFORM_OPTIONS = [
  { value: 'douyin', label: '抖音' },
  { value: 'kuaishou', label: '快手' },
  { value: 'bilibili', label: 'B站' },
]

const STATUS_COLORS: Record<string, string> = {
  active: 'green',
  expired: 'orange',
  banned: 'red',
}

const HEALTH_COLORS: Record<string, string> = {
  healthy: 'green',
  unhealthy: 'red',
  unknown: 'default',
}

// 平台方账号管理页面
export default function CrawlerAccounts() {
  const queryClient = useQueryClient()
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [form] = Form.useForm()

  // 查询账号列表
  const { data, isLoading } = useQuery({
    queryKey: ['admin-crawler-accounts'],
    queryFn: () => businessApi.adminListCrawlerAccounts(),
  })
  const accounts = data?.accounts || []

  // 创建账号
  const createMutation = useMutation({
    mutationFn: (data: Partial<CrawlerAccount>) => businessApi.adminCreateCrawlerAccount(data),
    onSuccess: () => {
      message.success('账号添加成功')
      setIsCreateModalOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-accounts'] })
    },
    onError: () => message.error('添加失败'),
  })

  // 删除账号
  const deleteMutation = useMutation({
    mutationFn: (id: number) => businessApi.adminDeleteCrawlerAccount(id),
    onSuccess: () => {
      message.success('账号已删除')
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-accounts'] })
    },
    onError: () => message.error('删除失败'),
  })

  // 健康检查
  const healthMutation = useMutation({
    mutationFn: ({ id, platform }: { id: number; platform: string }) =>
      businessApi.adminCheckCrawlerAccountHealth(id, platform),
    onSuccess: (data) => {
      message.success(`健康检查完成: ${data.healthy ? '健康' : '异常'}`)
      queryClient.invalidateQueries({ queryKey: ['admin-crawler-accounts'] })
    },
    onError: () => message.error('健康检查失败'),
  })

  // 表格列定义
  const columns = [
    {
      title: '平台',
      dataIndex: 'platform',
      key: 'platform',
      render: (platform: string) => {
        const labels: Record<string, string> = { douyin: '抖音', kuaishou: '快手', bilibili: 'B站' }
        return <Tag color="blue">{labels[platform] || platform}</Tag>
      },
    },
    {
      title: '账号名称',
      dataIndex: 'account_name',
      key: 'account_name',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={STATUS_COLORS[status] || 'default'}>{status}</Tag>
      ),
    },
    {
      title: '健康状态',
      dataIndex: 'health_check_result',
      key: 'health_check_result',
      render: (result: string) => (
        <Tag color={HEALTH_COLORS[result] || 'default'}>{result}</Tag>
      ),
    },
    {
      title: '今日用量',
      key: 'usage',
      render: (_: unknown, record: CrawlerAccount) => (
        <Text>{record.daily_usage_count} / {record.daily_usage_limit}</Text>
      ),
    },
    {
      title: '代理地址',
      dataIndex: 'proxy_address',
      key: 'proxy_address',
      render: (addr: string) => addr || <Text type="secondary">无</Text>,
    },
    {
      title: '最后使用',
      dataIndex: 'last_used_at',
      key: 'last_used_at',
      render: (t: string) => t ? new Date(t).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: unknown, record: CrawlerAccount) => (
        <Space>
          <Button
            size="small"
            icon={<ReloadOutlined />}
            onClick={() => healthMutation.mutate({ id: record.id, platform: record.platform })}
          >
            健康检查
          </Button>
          <Button
            size="small"
            danger
            icon={<DeleteOutlined />}
            onClick={() => {
              Modal.confirm({
                title: '确认删除',
                content: `确定删除账号 "${record.account_name}" 吗？`,
                onOk: () => deleteMutation.mutate(record.id),
              })
            }}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="平台方账号管理"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsCreateModalOpen(true)}>
            添加账号
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          平台方账号用于统一爬取热门视频数据。用户无需登录即可查看灵感广场。
        </Text>
        <Table
          columns={columns}
          dataSource={accounts}
          rowKey="id"
          loading={isLoading}
          pagination={false}
        />
      </Card>

      {/* 创建账号弹窗 */}
      <Modal
        title="添加平台方账号"
        open={isCreateModalOpen}
        onCancel={() => setIsCreateModalOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(values) => createMutation.mutate(values)}>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Select options={PLATFORM_OPTIONS} placeholder="选择平台" />
          </Form.Item>
          <Form.Item name="account_name" label="账号名称" rules={[{ required: true }]}>
            <Input placeholder="例如：抖音工作号1" />
          </Form.Item>
          <Form.Item name="cookie" label="Cookie" rules={[{ required: true }]}>
            <Input.TextArea rows={4} placeholder="从浏览器开发者工具复制 Cookie" />
          </Form.Item>
          <Form.Item name="user_agent" label="User-Agent">
            <Input placeholder="可选，留空使用默认值" />
          </Form.Item>
          <Form.Item name="proxy_address" label="代理地址">
            <Input placeholder="可选，例如 http://proxy:8080" />
          </Form.Item>
          <Form.Item name="daily_usage_limit" label="每日使用上限" initialValue={50}>
            <InputNumber min={1} max={1000} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Button, Space, Table, Tag, Typography, Select,
} from 'antd'
import {
  ReloadOutlined, LinkOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import type { PublishJob } from '../../../types/api'

const { Text } = Typography

interface Props {
  brandId: string
}

const PLATFORMS = [
  { value: 'all', label: '全部平台' },
  { value: 'douyin', label: '抖音' },
  { value: 'kuaishou', label: '快手' },
  { value: 'xiaohongshu', label: '小红书' },
  { value: 'weixin', label: '视频号' },
  { value: 'bilibili', label: 'B站' },
]

const STATUS_MAP: Record<string, { label: string; color: string }> = {
  pending: { label: '待发布', color: 'default' },
  scheduled: { label: '已排期', color: 'blue' },
  running: { label: '发布中', color: 'processing' },
  published: { label: '已发布', color: 'success' },
  failed: { label: '失败', color: 'error' },
}

/**
 * 发布历史 Tab
 * 查看品牌的发布任务历史和状态
 */
export default function PublishHistoryTab({ brandId }: Props) {
  const [platform, setPlatform] = useState<string>('all')
  const [status, setStatus] = useState<string>('all')

  // 获取发布任务列表（brand 维度——服务端按 brand_id 过滤，不再混入他品牌任务）
  const { data: jobs = [], isLoading, refetch } = useQuery({
    queryKey: ['publish-jobs', brandId],
    queryFn: () => businessApi.listPublishJobs(brandId),
    enabled: !!brandId,
  })

  // 过滤数据
  const filteredJobs = jobs.filter(job => {
    if (platform !== 'all' && job.platform !== platform) return false
    if (status !== 'all' && job.status !== status) return false
    return true
  })

  // 表格列定义
  const columns = [
    {
      title: '平台',
      dataIndex: 'platform',
      key: 'platform',
      width: 100,
      render: (platform: string) => {
        const p = PLATFORMS.find(p => p.value === platform)
        return <Tag>{p?.label || platform}</Tag>
      },
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
      render: (title: string) => (
        <Text ellipsis={{ tooltip: title }} style={{ maxWidth: 200 }}>
          {title || '-'}
        </Text>
      ),
    },
    {
      title: '内容类型',
      dataIndex: 'content_type',
      key: 'content_type',
      width: 100,
      render: (type: string) => {
        const typeMap: Record<string, string> = {
          video: '视频',
          image: '图文',
          article: '文章',
        }
        return <Tag>{typeMap[type] || type}</Tag>
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => {
        const s = STATUS_MAP[status] || { label: status, color: 'default' }
        return <Tag color={s.color}>{s.label}</Tag>
      },
    },
    {
      title: '传输方式',
      dataIndex: 'transport',
      key: 'transport',
      width: 100,
      render: (transport: string) => {
        const transportMap: Record<string, string> = {
          link: '半自动',
          rpa: '全自动',
          api: 'API',
        }
        return <Tag>{transportMap[transport] || transport}</Tag>
      },
    },
    {
      title: '发布链接',
      key: 'url',
      width: 120,
      render: (_: unknown, record: PublishJob) => {
        if (!record.external_url) return <Text type="secondary">-</Text>
        return (
          <Button
            type="link"
            size="small"
            icon={<LinkOutlined />}
            href={record.external_url}
            target="_blank"
          >
            查看
          </Button>
        )
      },
    },
    {
      title: '发布时间',
      dataIndex: 'published_at',
      key: 'published_at',
      width: 180,
      render: (time: string) => {
        if (!time) return <Text type="secondary">-</Text>
        return <Text>{new Date(time).toLocaleString()}</Text>
      },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time: string) => (
        <Text>{new Date(time).toLocaleString()}</Text>
      ),
    },
    {
      title: '错误信息',
      dataIndex: 'error_msg',
      key: 'error_msg',
      ellipsis: true,
      render: (msg: string) => {
        if (!msg) return <Text type="secondary">-</Text>
        return <Text type="danger" ellipsis={{ tooltip: msg }}>{msg}</Text>
      },
    },
  ]

  // 统计
  const stats = {
    total: filteredJobs.length,
    published: filteredJobs.filter(j => j.status === 'published').length,
    failed: filteredJobs.filter(j => j.status === 'failed').length,
    pending: filteredJobs.filter(j => j.status === 'pending' || j.status === 'scheduled').length,
  }

  return (
    <div className="publish-history-tab">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <Text strong>发布历史</Text>
          <Tag>共 {stats.total} 条</Tag>
          <Tag color="success">成功 {stats.published}</Tag>
          <Tag color="error">失败 {stats.failed}</Tag>
          <Tag color="processing">进行中 {stats.pending}</Tag>
        </Space>
        <Space>
          <Select
            style={{ width: 120 }}
            value={platform}
            onChange={setPlatform}
            options={PLATFORMS}
          />
          <Select
            style={{ width: 120 }}
            value={status}
            onChange={setStatus}
            options={[
              { value: 'all', label: '全部状态' },
              { value: 'pending', label: '待发布' },
              { value: 'scheduled', label: '已排期' },
              { value: 'running', label: '发布中' },
              { value: 'published', label: '已发布' },
              { value: 'failed', label: '失败' },
            ]}
          />
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
            刷新
          </Button>
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={filteredJobs}
        rowKey="id"
        loading={isLoading}
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
        locale={{ emptyText: '暂无发布记录' }}
        scroll={{ x: 1200 }}
      />
    </div>
  )
}

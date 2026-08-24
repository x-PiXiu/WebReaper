import { Typography, Card, Space, Tag, Table, Button, Select } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { CrawlerTaskLog } from '../../types/api'
import { useState } from 'react'

const { Text } = Typography

const STATUS_COLORS: Record<string, string> = {
  running: 'processing',
  success: 'success',
  failed: 'error',
}

const TRIGGER_LABELS: Record<string, string> = {
  scheduled: '定时任务',
  manual: '手动触发',
  first_time: '首次触发',
}

// 采集任务监控页面
export default function CrawlerTasks() {
  const [platform, setPlatform] = useState<string>('')

  // 查询任务列表
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['admin-crawler-tasks', platform],
    queryFn: () => businessApi.adminListCrawlerTasks(100),
  })
  const tasks = data?.tasks || []

  // 按平台筛选
  const filteredTasks = platform
    ? tasks.filter(t => t.platform === platform)
    : tasks

  // 表格列定义
  const columns = [
    {
      title: '任务ID',
      dataIndex: 'task_id',
      key: 'task_id',
      width: 200,
      render: (id: string) => <Text code style={{ fontSize: 12 }}>{id}</Text>,
    },
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
      title: '品牌',
      dataIndex: 'brand_id',
      key: 'brand_id',
      render: (id: string) => id || <Text type="secondary">-</Text>,
    },
    {
      title: '触发方式',
      dataIndex: 'trigger_type',
      key: 'trigger_type',
      render: (type: string) => (
        <Tag color="purple">{TRIGGER_LABELS[type] || type}</Tag>
      ),
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
      title: '关键词',
      dataIndex: 'keywords_used',
      key: 'keywords_used',
      render: (keywords: string[]) => (
        <Space wrap>
          {(keywords || []).slice(0, 2).map((k, i) => <Tag key={i}>{k}</Tag>)}
        </Space>
      ),
    },
    {
      title: '结果',
      key: 'result',
      render: (_: unknown, record: CrawlerTaskLog) => (
        <Space>
          <Text>找到 {record.videos_found}</Text>
          <Text type="success">新增 {record.videos_new}</Text>
          <Text type="secondary">更新 {record.videos_updated}</Text>
        </Space>
      ),
    },
    {
      title: '耗时',
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      render: (ms: number) => ms ? `${(ms / 1000).toFixed(1)}s` : '-',
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
      render: (t: string) => t ? new Date(t).toLocaleString() : '-',
    },
    {
      title: '错误信息',
      dataIndex: 'error_message',
      key: 'error_message',
      render: (msg: string) => msg ? (
        <Text type="danger" style={{ fontSize: 12, maxWidth: 200, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {msg}
        </Text>
      ) : null,
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="采集任务监控"
        extra={
          <Space>
            <Select
              value={platform}
              onChange={setPlatform}
              style={{ width: 120 }}
              options={[
                { value: '', label: '全部平台' },
                { value: 'douyin', label: '抖音' },
                { value: 'kuaishou', label: '快手' },
                { value: 'bilibili', label: 'B站' },
              ]}
            />
            <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
              刷新
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={filteredTasks}
          rowKey="id"
          loading={isLoading}
          pagination={{ pageSize: 20 }}
          scroll={{ x: 1200 }}
        />
      </Card>
    </div>
  )
}

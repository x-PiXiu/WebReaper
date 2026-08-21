import { useState } from 'react'
import { Typography, Table, Tag, Space, Button, Empty, message } from 'antd'
import { ExportOutlined, CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import WorkDetailDrawer, { type WorkDetailData } from '../../../components/WorkDetailDrawer'
import type { PublishJob } from '../../../types/api'

const { Text } = Typography

export const PLATFORM_NAMES: Record<string, string> = { zhihu: '知乎', xiaohongshu: '小红书' }

// 发布状态 → 显示（页面自动发布状态面板与发布记录表共用）
export function statusConfig(status: string) {
  switch (status) {
    case 'published': return { color: 'var(--wr-success)', label: '已发布', icon: <CheckCircleOutlined /> }
    case 'running': return { color: 'var(--wr-primary)', label: '自动发布中', icon: <LoadingOutlined /> }
    case 'pending': return { color: 'var(--wr-warning)', label: '待确认', icon: <ClockCircleOutlined /> }
    case 'failed': return { color: 'var(--wr-danger)', label: '失败', icon: <CloseCircleOutlined /> }
    default: return { color: 'var(--wr-text-muted)', label: status, icon: <ClockCircleOutlined /> }
  }
}

// 发布记录表格（社媒分发页③区）：列表 + 跳转/复测提及率/标记已发布。
// 复测与标记的副作用（成功提示/缓存失效）自包含，父组件只传数据与刷新回调。
export default function PublishJobTable({
  jobs,
  onRefresh,
  reMonitorPending,
}: {
  jobs: PublishJob[]
  onRefresh: () => void
  reMonitorPending: string | null
}) {
  const [detailWork, setDetailWork] = useState<WorkDetailData | null>(null)
  const handleMarkPublished = async (jobId: string) => {
    try {
      await businessApi.markPublished(jobId)
      onRefresh()
    } catch { /* 拦截器已提示 */ }
  }

  const handleReMonitor = async (jobId: string) => {
    try {
      const job = await businessApi.reMonitorJob(jobId)
      const diff = job.post_mention_rate - job.pre_mention_rate
      const sign = diff > 0 ? '+' : ''
      message.info(`复测完成：表现 ${(job.pre_mention_rate * 100).toFixed(1)}% → ${(job.post_mention_rate * 100).toFixed(1)}%（${sign}${(diff * 100).toFixed(1)}%）`)
      onRefresh()
    } catch { /* 拦截器已提示 */ }
  }

  const columns = [
    {
      title: '标题', dataIndex: 'title', key: 'title',
      render: (t: string) => <Text strong style={{ fontSize: 13 }}>{t || '-'}</Text>,
    },
    {
      title: '平台', dataIndex: 'platform', key: 'platform', width: 90,
      render: (p: string) => <Tag>{PLATFORM_NAMES[p] || p}</Tag>,
    },
    {
      title: '模式', dataIndex: 'mode', key: 'mode', width: 90,
      render: (m: string) => <Text type="secondary" style={{ fontSize: 12 }}>{m === 'semi-auto' ? '半自动' : m}</Text>,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 110,
      render: (s: string) => {
        const cfg = statusConfig(s)
        return <Space><span style={{ color: cfg.color }}>{cfg.icon}</span><Text style={{ color: cfg.color, fontSize: 12 }}>{cfg.label}</Text></Space>
      },
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'time', width: 150,
      render: (t: string) => <Text type="secondary" style={{ fontSize: 12 }}>{t ? new Date(t).toLocaleString() : '-'}</Text>,
    },
    {
      title: '表现变化', key: 'mention_rate', width: 140,
      render: (_: unknown, r: PublishJob) => {
        if (!r.post_mention_rate) return <Text type="secondary" style={{ fontSize: 12 }}>-</Text>
        const pre = (r.pre_mention_rate * 100).toFixed(1)
        const post = (r.post_mention_rate * 100).toFixed(1)
        const diff = r.post_mention_rate - r.pre_mention_rate
        const color = diff > 0 ? 'var(--wr-success)' : diff < 0 ? 'var(--wr-danger)' : 'var(--wr-text-muted)'
        return <Text style={{ fontSize: 12, color }}>{pre}% → {post}%{diff !== 0 && ` (${diff > 0 ? '+' : ''}${(diff * 100).toFixed(1)}%)`}</Text>
      },
    },
    {
      title: '操作', key: 'action', width: 200,
      render: (_: unknown, r: PublishJob) => (
        <Space>
          {r.external_url && (
            <Button size="small" type="link" icon={<ExportOutlined />} href={r.external_url} target="_blank">跳转</Button>
          )}
          <Button size="small" type="link" onClick={() => setDetailWork({
            title: r.title, platform: r.platform, content_type: r.content_type,
            external_url: r.external_url, published_at: r.published_at, status: r.status,
          })}>详情</Button>
          {r.status === 'published' && (
            <Button size="small" type="link" loading={reMonitorPending === r.id} onClick={() => handleReMonitor(r.id)}>复测表现</Button>
          )}
          {r.status === 'pending' && (
            <Button size="small" type="link" onClick={() => handleMarkPublished(r.id)}>标记已发布</Button>
          )}
        </Space>
      ),
    },
  ]

  if (jobs.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无发布记录" style={{ padding: 40 }} />
  }
  return (
    <>
      <Table dataSource={jobs} columns={columns} rowKey="id" pagination={{ pageSize: 10 }} size="small" />
      {/* 作品详情 Drawer（与作品数据页共用组件——互动数据回读上线后自动展示趋势） */}
      <WorkDetailDrawer open={!!detailWork} onClose={() => setDetailWork(null)} work={detailWork} />
    </>
  )
}

import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Modal, Space, Tag, Typography, message } from 'antd'
import { LinkOutlined, ReloadOutlined, SoundOutlined } from '@ant-design/icons'
import { LazyLine } from './charts/LazyCharts'
import { businessApi } from '../api/business'
import { PlatformBadge } from './PlatformBadge'
import { MODAL_W, modalBodyScroll } from '../ui/modalFit'

const { Text, Title } = Typography

const TYPE_LABEL: Record<string, string> = { video: '视频', image: '图文', article: '文章', audio: '音频' }

/** 详情弹窗的归一化数据（analytics 作品表 / 发布中心发布记录 两处映射后共用）。 */
export interface WorkDetailData {
  jobId?: string
  title: string
  platform: string
  content_type?: string
  external_url?: string
  published_at?: string
  status?: string
  views?: number
  likes?: number
  comments?: number
  shares?: number
}

/**
 * 作品详情弹窗（共用）：作品数据页表格 + 发布中心发布记录的「详情」入口。
 * 基本信息 + 视频链接 + 互动数据卡（回读快照填充）+ 趋势折线 +「立即刷新」。
 */
export default function WorkDetailDrawer({ open, onClose, work }: {
  open: boolean
  onClose: () => void
  work: WorkDetailData | null
}) {
  const queryClient = useQueryClient()
  const [refreshing, setRefreshing] = useState(false)

  const { data: metrics = [] } = useQuery({
    queryKey: ['job-metrics', work?.jobId],
    queryFn: () => businessApi.getJobMetrics(work!.jobId!).catch(() => []),
    enabled: open && !!work?.jobId,
  })

  if (!work) return null

  const m = work
  const cards = [
    { label: '播放', value: m.views || 0 },
    { label: '点赞', value: m.likes || 0 },
    { label: '评论', value: m.comments || 0 },
    { label: '分享', value: m.shares || 0 },
  ]
  const hasData = cards.some((c) => c.value > 0) || metrics.length > 0
  const trend = metrics.map((x) => ({
    day: new Date(x.collected_at).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }),
    播放: x.views,
    点赞: x.likes,
  }))

  const doRefresh = async () => {
    if (!work.jobId) return
    setRefreshing(true)
    try {
      const r = await businessApi.refreshJobMetrics(work.jobId)
      message.success(`已回读：播放 ${r.views.toLocaleString()} · 赞 ${r.likes.toLocaleString()}`)
      queryClient.invalidateQueries({ queryKey: ['job-metrics', work.jobId] })
      queryClient.invalidateQueries({ queryKey: ['analytics-summary'] })
      queryClient.invalidateQueries({ queryKey: ['geo-publish-jobs'] })
    } catch (e: any) {
      message.error(e?.response?.data?.msg || '回读失败（需该平台浏览器通道账号）')
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      width={MODAL_W.lg}
      title={work.title}
      footer={null}
      destroyOnClose
      styles={{ body: { ...modalBodyScroll.body, background: 'var(--wr-bg)' } }}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <div className="ip-panel" style={{ padding: 16 }}>
          <Space wrap>
            <PlatformBadge platform={work.platform} size={14} />
            {work.content_type && <Tag>{TYPE_LABEL[work.content_type] || work.content_type}</Tag>}
            {work.status === 'published' && <Tag color="green">已发布</Tag>}
          </Space>
          <div style={{ marginTop: 12, display: 'grid', gap: 6, fontSize: 13 }}>
            <Text type="secondary">
              发布时间：{work.published_at ? new Date(work.published_at).toLocaleString('zh-CN', { hour12: false }) : '—'}
            </Text>
            <Space size={8} wrap>
              <Text type="secondary">作品链接：</Text>
              {work.external_url ? (
                <Button size="small" icon={<LinkOutlined />} onClick={() => window.open(work.external_url, '_blank', 'noopener')}>
                  打开作品
                </Button>
              ) : (
                <Text type="secondary" style={{ fontSize: 12 }}>手动发布 · 链接未追踪</Text>
              )}
            </Space>
          </div>
        </div>

        <div className="ip-panel" style={{ padding: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Title level={5} style={{ marginTop: 0 }}>互动数据</Title>
            {work.jobId && work.external_url && (
              <Button size="small" icon={<ReloadOutlined />} loading={refreshing} onClick={doRefresh}>立即刷新</Button>
            )}
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8 }}>
            {cards.map((c) => (
              <div key={c.label} className="ip-metric-card" style={{ padding: 12, minHeight: 72 }}>
                <span className="ip-metric-label">{c.label}</span>
                <strong className="ip-metric-value" style={{ fontSize: 18 }}>{c.value.toLocaleString()}</strong>
              </div>
            ))}
          </div>
          {!hasData && (
            <Space style={{ marginTop: 12 }} size={6}>
              <SoundOutlined style={{ color: 'var(--wr-accent)' }} />
              <Text type="secondary" style={{ fontSize: 12 }}>
                暂无回读数据——每日自动回读需该平台浏览器通道账号，或点「立即刷新」实测
              </Text>
            </Space>
          )}
          {trend.length >= 2 && (
            <div style={{ marginTop: 14 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>互动趋势（回读快照）</Text>
              <LazyLine data={trend} xField="day" yField="播放" height={120} smooth color={['#5eead4']} />
            </div>
          )}
        </div>
      </Space>
    </Modal>
  )
}

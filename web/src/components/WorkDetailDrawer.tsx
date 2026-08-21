import { Button, Drawer, Space, Tag, Typography } from 'antd'
import { LinkOutlined, SoundOutlined } from '@ant-design/icons'

const { Text, Title } = Typography

const PLATFORM_LABEL: Record<string, string> = {
  douyin: '抖音',
  kuaishou: '快手',
  zhihu: '知乎',
  xiaohongshu: '小红书',
  gongzhonghao: '公众号',
}

const TYPE_LABEL: Record<string, string> = { video: '视频', image: '图文', article: '文章', audio: '音频' }

/** 详情 Drawer 的归一化数据（analytics 作品表 / 发布中心发布记录 两处映射后共用）。 */
export interface WorkDetailData {
  title: string
  platform: string
  content_type?: string
  external_url?: string
  published_at?: string
  status?: string
}

/**
 * 作品详情 Drawer（共用组件）：作品数据页表格 + 发布中心发布记录的「详情」入口。
 * 展示：基本信息 + 视频链接 + 互动数据卡（数据回读上线后自动填充趋势）。
 */
export default function WorkDetailDrawer({ open, onClose, work }: {
  open: boolean
  onClose: () => void
  work: WorkDetailData | null
}) {
  if (!work) return null
  const metrics = [
    { label: '播放', value: work ? 0 : 0 },
    { label: '点赞', value: 0 },
    { label: '评论', value: 0 },
    { label: '分享', value: 0 },
  ]
  return (
    <Drawer open={open} onClose={onClose} width={460} title={work.title} styles={{ body: { background: 'var(--wr-bg)' } }}>
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {/* 基本信息 */}
        <div className="ip-panel" style={{ padding: 16 }}>
          <Space wrap>
            <Tag color="cyan">{PLATFORM_LABEL[work.platform] || work.platform}</Tag>
            {work.content_type && <Tag>{TYPE_LABEL[work.content_type] || work.content_type}</Tag>}
            {work.status === 'published' && <Tag color="green">已发布</Tag>}
          </Space>
          <div style={{ marginTop: 12, display: 'grid', gap: 6, fontSize: 13 }}>
            <Text type="secondary">
              发布时间：{work.published_at ? new Date(work.published_at).toLocaleString('zh-CN', { hour12: false }) : '—'}
            </Text>
            <Space size={8}>
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

        {/* 互动数据（回读上线后填充趋势） */}
        <div className="ip-panel" style={{ padding: 16 }}>
          <Title level={5} style={{ marginTop: 0 }}>互动数据</Title>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8 }}>
            {metrics.map((m) => (
              <div key={m.label} className="ip-metric-card" style={{ padding: 12, minHeight: 72 }}>
                <span className="ip-metric-label">{m.label}</span>
                <strong className="ip-metric-value" style={{ fontSize: 18 }}>{m.value.toLocaleString()}</strong>
              </div>
            ))}
          </div>
          <Space style={{ marginTop: 12 }} size={6}>
            <SoundOutlined style={{ color: 'var(--wr-accent)' }} />
            <Text type="secondary" style={{ fontSize: 12 }}>
              互动数据回读即将上线——上线后此处自动展示每日播放/点赞/评论趋势
            </Text>
          </Space>
        </div>
      </Space>
    </Drawer>
  )
}

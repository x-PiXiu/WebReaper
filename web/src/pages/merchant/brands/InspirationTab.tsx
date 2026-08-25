import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  Button, Empty, Select, Space, Spin, Tag, Typography,
} from 'antd'
import {
  FireOutlined, LinkOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { PlatformBadge } from '../../../components/PlatformBadge'

const { Text, Paragraph } = Typography

const GRAPHIC_PLATFORMS = new Set(['xiaohongshu', 'web'])

interface Props {
  brandId: string
}

/**
 * 灵感 Tab：展示品牌同赛道的热门视频/图文灵感（来自爬虫采集）。
 * 紧凑卡片布局，支持平台筛选，点击可跳转灵感广场或直接复刻。
 */
export default function InspirationTab({ brandId }: Props) {
  const navigate = useNavigate()
  const [platform, setPlatform] = useState<string>('all')

  const { data, isLoading } = useQuery({
    queryKey: ['brand-inspirations', brandId, platform],
    queryFn: () => businessApi.listInspirations({
      brand_id: brandId,
      platform: platform !== 'all' ? platform : undefined,
      page_size: 12,
    }),
    staleTime: 5 * 60_000,
  })

  const videos = data?.items || []

  const platforms = ['douyin', 'kuaishou', 'bilibili', 'xiaohongshu']

  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 48 }}>
        <Spin tip="加载灵感中…" />
      </div>
    )
  }

  return (
    <div className="inspiration-tab">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <FireOutlined style={{ color: 'var(--wr-accent)' }} />
          <Text strong>同赛道灵感</Text>
          <Tag color="orange">{videos.length} 条</Tag>
        </Space>
        <Space>
          <Select
            style={{ minWidth: 120 }}
            size="small"
            value={platform}
            onChange={setPlatform}
            options={[
              { value: 'all', label: '全部平台' },
              ...platforms.map(p => ({ value: p, label: p })),
            ]}
          />
          <Button
            size="small"
            icon={<LinkOutlined />}
            onClick={() => navigate('/m/inspire')}
          >
            灵感广场
          </Button>
        </Space>
      </div>

      {videos.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="暂无灵感数据，系统会自动更新"
        >
          <Button type="primary" onClick={() => navigate('/m/inspire')}>
            去灵感广场
          </Button>
        </Empty>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 12 }}>
          {videos.map((v) => {
            const isGraphic = GRAPHIC_PLATFORMS.has(v.platform)
            return (
              <div
                key={v.id}
                className="wr-glass-card"
                style={{ padding: 12, cursor: 'pointer' }}
                onClick={() => navigate('/m/inspire')}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <PlatformBadge platform={v.platform} size={14} />
                  <Space size={4}>
                    <Tag style={{ margin: 0, fontSize: 11 }}>{isGraphic ? '图文' : '视频'}</Tag>
                    {v.viral_score > 0 && (
                      <Tag color="red" style={{ margin: 0, fontSize: 11 }}>🔥 {v.viral_score.toFixed(0)}</Tag>
                    )}
                  </Space>
                </div>
                <Text
                  strong
                  ellipsis={{ tooltip: v.title }}
                  style={{ display: 'block', marginBottom: 4, fontSize: 13 }}
                >
                  {v.title}
                </Text>
                {v.description && (
                  <Paragraph
                    type="secondary"
                    ellipsis={{ rows: 2 }}
                    style={{ fontSize: 12, marginBottom: 8 }}
                  >
                    {v.description}
                  </Paragraph>
                )}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    👁 {(v.play_count || 0).toLocaleString()} · ❤ {(v.digg_count || 0).toLocaleString()}
                  </Text>
                  <Space size={4}>
                    {v.author && (
                      <Text type="secondary" style={{ fontSize: 11 }}>{v.author}</Text>
                    )}
                  </Space>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {videos.length > 0 && (
        <div style={{ textAlign: 'center', marginTop: 16 }}>
          <Button type="link" onClick={() => navigate('/m/inspire')}>
            查看更多灵感 →
          </Button>
        </div>
      )}
    </div>
  )
}

import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Button, Empty, Input, Modal, Segmented, Select, Space, Spin, Tag, Typography, message,
} from 'antd'
import {
  EditOutlined, FireOutlined, PlayCircleOutlined, VideoCameraOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useBrandContext } from '../../../hooks/useBrands'
import { useComposeDraft } from '../../../store/composeDraft'
import { PRODUCT } from '../../../config/product'
import { PlatformBadge } from '../../../components/PlatformBadge'
import { PlatformIcon } from '../../../components/PlatformIcon'
import { getPlatformLabel } from '../../../data/platforms'
import type { InspirationVideo } from '../../../types/api'
import PageLoading from '../../../components/PageLoading'

const { Text, Paragraph } = Typography

const GRAPHIC_PLATFORMS = new Set(['xiaohongshu', 'web'])

type ContentKind = 'all' | 'video' | 'graphic'
type RemakeMode = 'video' | 'graphic'

function kindOf(v: InspirationVideo): RemakeMode {
  return GRAPHIC_PLATFORMS.has(v.platform) ? 'graphic' : 'video'
}

/**
 * 灵感广场：展示同赛道近期社交平台爆款短视频/图文，一键复刻进爆款获客双轨。
 * 数据来自灵感 API（平台爬虫采集，用户无需登录社媒账号）。
 */
export default function InspirationPlaza() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { brands, brandId, setCurrentBrand, isLoading: brandsLoading } = useBrandContext()
  const draft = useComposeDraft()

  const [kind, setKind] = useState<ContentKind>('all')
  const [platform, setPlatform] = useState<string>('all')
  const [remaking, setRemaking] = useState<{ item: InspirationVideo; mode: RemakeMode } | null>(null)
  const [topicDraft, setTopicDraft] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['inspirations', brandId, platform, kind],
    queryFn: () => businessApi.listInspirations({
      brand_id: brandId || undefined,
      platform: platform !== 'all' ? platform : undefined,
      page_size: 50,
    }),
    enabled: !!brandId,
    staleTime: 5 * 60_000,
  })

  const videos = data?.items || []

  const platforms = useMemo(() => {
    const set = new Set(videos.map((v) => v.platform).filter(Boolean))
    return Array.from(set)
  }, [videos])

  const filtered = useMemo(() => {
    return videos.filter((v) => {
      if (kind === 'video' && kindOf(v) !== 'video') return false
      if (kind === 'graphic' && kindOf(v) !== 'graphic') return false
      return true
    })
  }, [videos, kind])

  const openRemake = (item: InspirationVideo, mode: RemakeMode) => {
    setRemaking({ item, mode })
    const prefix = mode === 'graphic' ? '写一篇同款种草：' : '拍一条同款：'
    const topic = item.topics?.[0] || `${prefix}${item.title}`
    setTopicDraft(topic)
  }

  const confirmRemake = () => {
    if (!remaking) return
    const topic = topicDraft.trim()
    if (topic.length < 4) {
      message.warning('选题至少 4 个字')
      return
    }
    const { item, mode } = remaking
    draft.setTrack(mode)
    draft.patch({
      brandId,
      sourceUrl: item.video_url || undefined,
      refTitle: item.title,
      hotPoint: item.description?.slice(0, 100) || undefined,
      script: topic,
      transcript: [
        item.description ? `【简介】${item.description.slice(0, 200)}` : '',
        `【选题】${topic}`,
        item.video_url ? `【来源】${item.video_url}` : '',
      ].filter(Boolean).join('\n'),
      selectedTitle: item.title.slice(0, 40),
    })
    setRemaking(null)
    message.success(mode === 'graphic' ? '已带入发图文' : '已带入发视频')
    navigate(mode === 'graphic' ? '/m/compose/graphic' : '/m/compose/lipsync')
  }

  if (brandsLoading) return <PageLoading />

  return (
    <div className="wr-page-content ip-page inspire-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">{PRODUCT.nameEn} · Inspire</p>
          <h1>灵感广场</h1>
          <p className="ip-lead">
            同赛道近期社交平台爆款短视频与图文——看懂为什么火，一键复刻进发视频 / 发图文
          </p>
        </div>
        <Space wrap>
          <Select
            style={{ minWidth: 180 }}
            placeholder="选择人设"
            value={brandId}
            onChange={(v) => setCurrentBrand(v)}
            options={brands.map((b) => ({ value: b.id, label: b.name }))}
            notFoundContent="请先建人设档案"
          />
        </Space>
      </div>

      {!brandId ? (
        <div className="ip-panel" style={{ padding: 48, textAlign: 'center' }}>
          <Empty description="先建人设档案，才能按行业发现爆款灵感">
            <Button type="primary" className="ip-btn-primary" onClick={() => navigate('/m/brands')}>
              去建人设
            </Button>
          </Empty>
        </div>
      ) : (
        <>
          <div className="inspire-toolbar">
            <Segmented
              value={kind}
              onChange={(v) => setKind(v as ContentKind)}
              options={[
                { value: 'all', label: '全部' },
                { value: 'video', label: '短视频', icon: <VideoCameraOutlined /> },
                { value: 'graphic', label: '图文种草', icon: <EditOutlined /> },
              ]}
            />
            <Space wrap>
              <Select
                style={{ minWidth: 140 }}
                value={platform}
                onChange={setPlatform}
                options={[
                  { value: 'all', label: '全部平台' },
                  ...platforms.map((p) => ({ value: p, label: getPlatformLabel(p) })),
                ]}
                labelRender={(props) => {
                  if (props.value === 'all') return <span>全部平台</span>
                  return (
                    <span className="platform-option">
                      <PlatformIcon platform={String(props.value)} size={16} />
                      <span>{getPlatformLabel(String(props.value))}</span>
                    </span>
                  )
                }}
                optionRender={(opt) => (
                  opt.value === 'all' ? (
                    <span>{opt.label}</span>
                  ) : (
                    <span className="platform-option">
                      <PlatformIcon platform={String(opt.value)} size={16} />
                      <span>{opt.label}</span>
                    </span>
                  )
                )}
              />
              <Text type="secondary" style={{ fontSize: 12 }}>
                {filtered.length} 条灵感
              </Text>
            </Space>
          </div>

          {isLoading ? (
            <div className="ip-panel" style={{ padding: 64, textAlign: 'center' }}>
              <Spin tip="正在加载灵感…" size="large" />
            </div>
          ) : filtered.length === 0 ? (
            <div className="ip-panel" style={{ padding: 48 }}>
              <Empty description="暂无匹配灵感——完善人设行业/定位，或稍后再来看看">
                <Button type="primary" className="ip-btn-primary" onClick={() => queryClient.invalidateQueries({ queryKey: ['inspirations'] })}>
                  刷新
                </Button>
              </Empty>
            </div>
          ) : (
            <div className="inspire-grid ip-stagger">
              {filtered.map((v, i) => {
                const mode = kindOf(v)
                return (
                  <article key={(v.platform_video_id || v.title) + i} className="inspire-card">
                    {v.cover_url && (
                      <div className="inspire-card-cover">
                        <img src={v.cover_url} alt={v.title} loading="lazy" />
                      </div>
                    )}
                    <div className="inspire-card-top">
                      <PlatformBadge platform={v.platform} size={16} />
                      <Tag style={{ margin: 0 }}>{mode === 'graphic' ? '图文向' : '短视频'}</Tag>
                      {v.viral_score > 0 && (
                        <Tag color="red" style={{ margin: 0 }}>🔥 {v.viral_score.toFixed(0)}分</Tag>
                      )}
                    </div>
                    <Text strong className="inspire-card-title" ellipsis={{ tooltip: v.title }}>
                      {v.title}
                    </Text>
                    {v.description ? (
                      <Paragraph type="secondary" className="inspire-card-hot" ellipsis={{ rows: 3 }}>
                        <FireOutlined style={{ color: 'var(--wr-accent)', marginRight: 6 }} />
                        {v.description}
                      </Paragraph>
                    ) : (
                      <Paragraph type="secondary" className="inspire-card-hot">暂无简介</Paragraph>
                    )}
                    <div className="inspire-card-stats">
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        👁 {(v.play_count || 0).toLocaleString()} · ❤ {(v.digg_count || 0).toLocaleString()} · 💬 {(v.comment_count || 0).toLocaleString()}
                      </Text>
                    </div>
                    {v.author && (
                      <Text type="secondary" style={{ fontSize: 11 }}>作者：{v.author}</Text>
                    )}
                    <div className="inspire-card-actions">
                      <Button
                        size="small"
                        icon={<PlayCircleOutlined />}
                        disabled={!v.video_url}
                        onClick={() => v.video_url && window.open(v.video_url, '_blank', 'noopener')}
                      >
                        原帖
                      </Button>
                      <Button
                        size="small"
                        type="primary"
                        className="ip-btn-primary"
                        icon={<VideoCameraOutlined />}
                        onClick={() => openRemake(v, 'video')}
                      >
                        复刻视频
                      </Button>
                      <Button
                        size="small"
                        icon={<EditOutlined />}
                        onClick={() => openRemake(v, 'graphic')}
                      >
                        复刻图文
                      </Button>
                    </div>
                  </article>
                )
              })}
            </div>
          )}
        </>
      )}

      <Modal
        open={!!remaking}
        title={remaking?.mode === 'graphic' ? '复刻为图文' : '复刻为短视频'}
        okText={remaking?.mode === 'graphic' ? '进入发图文' : '进入发视频'}
        cancelText="取消"
        onOk={confirmRemake}
        onCancel={() => setRemaking(null)}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }} size={10}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            参考爆款：{remaking?.item.title}
          </Text>
          {remaking?.item.description && (
            <div className="inspire-topic">
              <Text style={{ fontSize: 12 }}>📝 {remaking.item.description.slice(0, 100)}</Text>
            </div>
          )}
          <Text style={{ fontSize: 13 }}>确认或改写选题后进入步骤引导：</Text>
          <Input.TextArea
            rows={3}
            value={topicDraft}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setTopicDraft(e.target.value)}
            maxLength={160}
            showCount
            placeholder="你的差异化选题"
          />
        </Space>
      </Modal>
    </div>
  )
}

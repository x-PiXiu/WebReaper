import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Button, Empty, Input, Modal, Segmented, Select, Space, Spin, Tag, Typography, message,
} from 'antd'
import {
  EditOutlined, FireOutlined, PlayCircleOutlined, SyncOutlined, VideoCameraOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useBrandContext } from '../../../hooks/useBrands'
import { useComposeDraft } from '../../../store/composeDraft'
import { PRODUCT } from '../../../config/product'
import { PlatformBadge } from '../../../components/PlatformBadge'
import { PlatformIcon } from '../../../components/PlatformIcon'
import { getPlatformLabel } from '../../../data/platforms'
import type { HotVideo } from '../../../types/api'
import PageLoading from '../../../components/PageLoading'

const { Text, Paragraph } = Typography

const GRAPHIC_PLATFORMS = new Set(['xiaohongshu', 'web'])

type ContentKind = 'all' | 'video' | 'graphic'
type RemakeMode = 'video' | 'graphic'

function kindOf(v: HotVideo): RemakeMode {
  return GRAPHIC_PLATFORMS.has(v.platform) ? 'graphic' : 'video'
}

/**
 * 灵感广场：展示同赛道近期社交平台爆款短视频/图文，一键复刻进爆款获客双轨。
 * 数据沿用热门同款 API（LLM + 站内/搜索发现，24h 缓存）。
 */
export default function InspirationPlaza() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { brands, brandId, setCurrentBrand, isLoading: brandsLoading } = useBrandContext()
  const draft = useComposeDraft()

  const [kind, setKind] = useState<ContentKind>('all')
  const [platform, setPlatform] = useState<string>('all')
  const [remaking, setRemaking] = useState<{ item: HotVideo; mode: RemakeMode } | null>(null)
  const [topicDraft, setTopicDraft] = useState('')
  const [refreshing, setRefreshing] = useState(false)

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['hot-videos', brandId],
    queryFn: () => businessApi.listHotVideos(brandId!),
    enabled: !!brandId,
    staleTime: 24 * 3600_000,
  })

  const videos = data?.videos || []

  const platforms = useMemo(() => {
    const set = new Set(videos.map((v) => v.platform).filter(Boolean))
    return Array.from(set)
  }, [videos])

  const filtered = useMemo(() => {
    return videos.filter((v) => {
      if (platform !== 'all' && v.platform !== platform) return false
      if (kind === 'video' && kindOf(v) !== 'video') return false
      if (kind === 'graphic' && kindOf(v) !== 'graphic') return false
      return true
    })
  }, [videos, kind, platform])

  const refresh = async () => {
    if (!brandId) {
      message.warning('请先选择人设档案')
      return
    }
    setRefreshing(true)
    message.loading({ content: '正在拉取社交平台最新爆款…', key: 'inspire', duration: 0 })
    try {
      await businessApi.listHotVideos(brandId, true)
      await queryClient.invalidateQueries({ queryKey: ['hot-videos', brandId] })
      message.success({ content: '灵感已更新', key: 'inspire' })
    } catch {
      message.error({ content: '更新失败，稍后再试', key: 'inspire' })
    } finally {
      setRefreshing(false)
    }
  }

  const openRemake = (item: HotVideo, mode: RemakeMode) => {
    setRemaking({ item, mode })
    const prefix = mode === 'graphic' ? '写一篇同款种草：' : '拍一条同款：'
    setTopicDraft(item.topic || `${prefix}${item.title}`)
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
      sourceUrl: item.url || undefined,
      refTitle: item.title,
      hotPoint: item.hot_point || undefined,
      script: topic,
      transcript: [
        item.hot_point ? `【为什么火】${item.hot_point}` : '',
        `【选题】${topic}`,
        item.url ? `【来源】${item.url}` : '',
      ].filter(Boolean).join('\n'),
      selectedTitle: item.title.slice(0, 40),
    })
    setRemaking(null)
    message.success(mode === 'graphic' ? '已带入发图文' : '已带入发视频')
    navigate(mode === 'graphic' ? '/m/compose/graphic' : '/m/compose/video')
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
          <Button
            icon={<SyncOutlined spin={refreshing || isFetching} />}
            loading={refreshing}
            disabled={!brandId}
            onClick={refresh}
          >
            刷新爆款
          </Button>
        </Space>
      </div>

      {!brandId ? (
        <div className="ip-panel" style={{ padding: 48, textAlign: 'center' }}>
          <Empty description="先建人设档案，才能按行业发现实时爆款">
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
                {filtered.length} 条灵感 · 约每日更新
              </Text>
            </Space>
          </div>

          {isLoading ? (
            <div className="ip-panel" style={{ padding: 64, textAlign: 'center' }}>
              <Spin tip="正在发现社交平台近期爆款…" size="large" />
            </div>
          ) : filtered.length === 0 ? (
            <div className="ip-panel" style={{ padding: 48 }}>
              <Empty description="暂无匹配灵感——完善人设行业/定位，或点「刷新爆款」重搜">
                <Button type="primary" className="ip-btn-primary" onClick={refresh}>刷新爆款</Button>
              </Empty>
            </div>
          ) : (
            <div className="inspire-grid ip-stagger">
              {filtered.map((v, i) => {
                const mode = kindOf(v)
                return (
                  <article key={(v.url || v.title) + i} className="inspire-card">
                    <div className="inspire-card-top">
                      <PlatformBadge platform={v.platform} size={16} />
                      <Tag style={{ margin: 0 }}>{mode === 'graphic' ? '图文向' : '短视频'}</Tag>
                    </div>
                    <Text strong className="inspire-card-title" ellipsis={{ tooltip: v.title }}>
                      {v.title}
                    </Text>
                    {v.hot_point ? (
                      <Paragraph type="secondary" className="inspire-card-hot" ellipsis={{ rows: 3 }}>
                        <FireOutlined style={{ color: 'var(--wr-accent)', marginRight: 6 }} />
                        {v.hot_point}
                      </Paragraph>
                    ) : (
                      <Paragraph type="secondary" className="inspire-card-hot">暂无爆点解读</Paragraph>
                    )}
                    {v.topic && (
                      <div className="inspire-topic">
                        <Text type="secondary" style={{ fontSize: 11 }}>复刻选题</Text>
                        <Text style={{ fontSize: 13, display: 'block', marginTop: 4 }}>{v.topic}</Text>
                      </div>
                    )}
                    <div className="inspire-card-actions">
                      <Button
                        size="small"
                        icon={<PlayCircleOutlined />}
                        disabled={!v.url}
                        onClick={() => v.url && window.open(v.url, '_blank', 'noopener')}
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
          {remaking?.item.hot_point && (
            <div className="inspire-topic">
              <Text style={{ fontSize: 12 }}>🔥 {remaking.item.hot_point}</Text>
            </div>
          )}
          <Text style={{ fontSize: 13 }}>确认或改写选题后进入步骤引导：</Text>
          <Input.TextArea
            rows={3}
            value={topicDraft}
            onChange={(e) => setTopicDraft(e.target.value)}
            maxLength={160}
            showCount
            placeholder="你的差异化选题"
          />
        </Space>
      </Modal>
    </div>
  )
}

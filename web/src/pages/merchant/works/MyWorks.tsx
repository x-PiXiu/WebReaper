import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input, Segmented, Space } from 'antd'
import { LikeOutlined, PlayCircleOutlined, PlusOutlined, SendOutlined, VideoCameraAddOutlined } from '@ant-design/icons'
import { usePublishableWorks } from '../../../hooks/usePublishableWorks'
import { brollLineage } from '../../../utils/publishableWorks'
import { MediaPreviewModal } from '../../../components/MediaPreviewModal'
import QueryBoundary from '../../../components/QueryBoundary'
import { cleanWorkTitle } from '../../../utils/workTitle'
import BrollDrawer, { type BrollSource } from '../../../components/compose/BrollDrawer'
import { VideoFrameCover } from '../../../components/VideoFrameCover'
import { ImageCover } from '../../../components/ImageCover'
import type { MediaAsset, WorkItem } from '../../../types/api'


type Filter = 'all' | 'draft' | 'ready' | 'published'

const PLATFORM_LABEL: Record<string, string> = {
  douyin: '抖音', kuaishou: '快手', zhihu: '知乎', xiaohongshu: '小红书', bilibili: 'B站', wechat: '微信',
}

const STATUS_CONFIG: Record<string, { label: string; color: string }> = {
  draft: { label: '草稿', color: 'default' },
  generating: { label: '生成中', color: 'processing' },
  ready: { label: '待发布', color: 'gold' },
  published: { label: '已发布', color: 'green' },
}

const KIND_CONFIG: Record<string, { label: string; emoji: string }> = {
  article: { label: '文章', emoji: '📝' },
  video: { label: '视频', emoji: '🎬' },
  image: { label: '图片', emoji: '🖼️' },
  audio: { label: '音频', emoji: '🎵' },
}

function distributionPath(w: WorkItem) {
  const q = new URLSearchParams()
  if (w.content_id) q.set('contentId', w.content_id)
  if (w.media_urls?.length) q.set('mediaUrls', w.media_urls.join(','))
  if (w.brand_id) q.set('brandId', w.brand_id)
  q.set('contentType', w.kind === 'article' ? 'article' : w.kind === 'image' ? 'image' : 'video')
  if (w.title) q.set('title', w.title)
  const s = q.toString()
  return s ? `/m/distribution?${s}` : '/m/distribution'
}

/**
 * 我的作品：三源聚合的真实作品库（文章 + 多媒体产物 + 发布状态 + 互动数据）。
 * 无数据空态引导去内容合成；待发布直达发布中心。
 */
export default function MyWorks() {
  const navigate = useNavigate()
  const [filter, setFilter] = useState<Filter>('all')
  const [q, setQ] = useState('')
  const [previewAsset, setPreviewAsset] = useState<MediaAsset | null>(null)
  const [brollSource, setBrollSource] = useState<BrollSource | null>(null)

  const { works = [], tasks, isLoading, isError, refetch } = usePublishableWorks()

  // B-Roll 血缘标记（§6.2）：compose 产物标"B-Roll"，被插过画面的源片标"已插画面"
  const { composeWorkIds, brollSourceWorkIds } = useMemo(() => brollLineage(tasks), [tasks])

  const list = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return works.filter((w) => {
      if (filter !== 'all' && w.status !== filter) return false
      if (needle && !w.title.toLowerCase().includes(needle)) return false
      return true
    })
  }, [works, filter, q])

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <h1>我的作品</h1>
          <p className="ip-lead">内容合成产出的可发布成片与文章——待发布可直达发布中心</p>
        </div>
        <Button type="primary" size="large" className="ip-btn-primary" icon={<PlusOutlined />} onClick={() => navigate('/m/compose')}>
          去内容合成
        </Button>
      </div>

      <div className="ip-toolbar">
        <Segmented
          value={filter}
          onChange={(v) => setFilter(v as Filter)}
          options={[
            { value: 'all', label: `全部 ${works.length}` },
            { value: 'draft', label: `草稿 ${works.filter((w) => w.status === 'draft').length}` },
            { value: 'ready', label: `待发布 ${works.filter((w) => w.status === 'ready').length}` },
            { value: 'published', label: `已发布 ${works.filter((w) => w.status === 'published').length}` },
          ]}
        />
        <Input.Search allowClear placeholder="搜索作品标题" style={{ maxWidth: 240 }} value={q} onChange={(e) => setQ(e.target.value)} />
      </div>

      <QueryBoundary
        loading={isLoading}
        error={isError}
        onRetry={() => refetch()}
        empty={list.length === 0}
        emptyText="还没有作品——去内容合成写第一篇文章或做第一个视频"
        emptyExtra={<Button type="primary" onClick={() => navigate('/m/compose')}>去内容合成</Button>}
      >
        <div className="mw-grid">
          {list.map((w) => {
            const st = STATUS_CONFIG[w.status] || { label: w.status, color: 'default' }
            const kd = KIND_CONFIG[w.kind] || { label: w.kind, emoji: '' }
            const title = cleanWorkTitle(w.title)
            const previewUrl = w.kind === 'video' || w.kind === 'image' ? w.media_urls?.[0] : undefined
            const canBroll = w.kind === 'video' && w.id.startsWith('g-')
            const coverStyle = w.cover_url
              ? { background: `linear-gradient(180deg, rgba(8,8,14,0.2), rgba(8,8,14,0.72)), url(${w.cover_url}) center/cover` }
              : undefined
            const previewBtn = previewUrl ? (
              <Button size="small" onClick={() => setPreviewAsset({
                    id: w.id,
                    tenant_id: '',
                    brand_id: w.brand_id || '',
                    owner_type: 'creation',
                    type: w.kind,
                    name: w.title,
                    url: previewUrl,
                    mime: w.kind === 'image' ? 'image/jpeg' : 'video/mp4',
                    size_bytes: 0,
                    width: 0,
                    height: 0,
                    duration: 0,
                    created_at: w.created_at,
                })}>
                  预览
                </Button>
            ) : null
            const publishBtn = w.status !== 'published' ? (
              <Button type="primary" size="small" icon={<SendOutlined />} onClick={() => navigate(distributionPath(w))}>
                去发布
              </Button>
            ) : null
            const brollBtn = canBroll ? (
              <Button size="small" icon={<VideoCameraAddOutlined />} onClick={() => setBrollSource({
                taskId: w.id.slice(2),
                title: w.title,
                videoUrl: w.media_urls?.[0],
              })}>
                插入画面
              </Button>
            ) : null
            const actions = (
              <>
                {publishBtn}
                {brollBtn}
                {previewBtn}
              </>
            )
            return (
              <div key={w.id} className={`mw-card mw-card--${w.status}`}>
                <div className="mw-cover" style={w.cover_url || previewUrl ? undefined : coverStyle}>
                  {w.kind === 'video' && w.media_urls?.[0] && (
                    <VideoFrameCover url={w.media_urls[0]} poster={w.cover_url} />
                  )}
                  {w.kind === 'image' && (w.cover_url || w.media_urls?.[0]) && (
                    <ImageCover url={w.cover_url || w.media_urls![0]} />
                  )}
                  {!previewUrl && (
                    <span className="mw-cover-title">{title}</span>
                  )}

                  <span className={`mw-status mw-status--${w.status}`}>{st.label}</span>
                  <span className="mw-kind">{kd.label}</span>
                  {composeWorkIds.has(w.id) && (
                    <span className="mw-kind mw-kind--broll" title="由 B-Roll 插入画面合成的成片">B-Roll</span>
                  )}
                  {brollSourceWorkIds.has(w.id) && (
                    <span className="mw-kind mw-kind--broll-source" title="该成片已插入过画面（有 B-Roll 衍生版本）">已插画面</span>
                  )}

                  <div className="mw-hover" aria-hidden>
                    <Space size={6} wrap style={{ justifyContent: 'center' }}>{actions}</Space>
                  </div>
                </div>

                <div className="mw-body">
                  <strong className="mw-title" title={w.title}>{title}</strong>
                  <div className="mw-meta">
                    {w.status === 'published' && w.platforms?.length ? (
                      <span className="mw-platforms">
                        {w.platforms.map((pf) => (
                          <i key={pf}>{PLATFORM_LABEL[pf] || pf}</i>
                        ))}
                      </span>
                    ) : (
                      <span className="mw-time">{new Date(w.created_at).toLocaleDateString('zh-CN')}</span>
                    )}
                    {w.views > 0 && <span className="mw-stat"><PlayCircleOutlined /> {w.views.toLocaleString()}</span>}
                    {w.likes > 0 && <span className="mw-stat"><LikeOutlined /> {w.likes.toLocaleString()}</span>}
                  </div>
                </div>

                {/* 静态主操作：无 hover 设备/快速直达 */}
                <div className="mw-actions-static">
                  {publishBtn}
                  {w.status === 'published' && previewBtn}
                </div>
              </div>
            )
          })}
        </div>
      </QueryBoundary>

      <MediaPreviewModal
        open={!!previewAsset}
        asset={previewAsset}
        onClose={() => setPreviewAsset(null)}
      />

      <BrollDrawer open={!!brollSource} source={brollSource} onClose={() => setBrollSource(null)} />

    </div>
  )
}

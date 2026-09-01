import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input } from 'antd'
import {
  FileTextOutlined, PictureOutlined, PlayCircleOutlined, PlusOutlined,
  RightOutlined, SendOutlined, SoundOutlined, VideoCameraOutlined, VideoCameraAddOutlined,
} from '@ant-design/icons'
import { usePublishableWorks } from '../../../hooks/usePublishableWorks'
import { brollLineage } from '../../../utils/publishableWorks'
import { MediaPreviewModal } from '../../../components/MediaPreviewModal'
import QueryBoundary from '../../../components/QueryBoundary'
import { cleanWorkTitle } from '../../../utils/workTitle'
import { VideoFrameCover } from '../../../components/VideoFrameCover'
import { ImageCover } from '../../../components/ImageCover'
import { distributionPathFromWork } from '../../../utils/distributionPath'
import type { MediaAsset, WorkItem } from '../../../types/api'

type Filter = 'all' | 'draft' | 'ready' | 'published'

const PLATFORM_LABEL: Record<string, string> = {
  douyin: '抖音', kuaishou: '快手', zhihu: '知乎', xiaohongshu: '小红书', bilibili: 'B站', wechat: '微信',
}

const STATUS_LABEL: Record<string, string> = {
  draft: '草稿',
  generating: '生成中',
  ready: '待发布',
  published: '已发布',
}

const KIND_META: Record<string, { label: string; icon: React.ReactNode; tone: string }> = {
  article: { label: '文章', icon: <FileTextOutlined />, tone: 'article' },
  video: { label: '视频', icon: <VideoCameraOutlined />, tone: 'video' },
  image: { label: '图片', icon: <PictureOutlined />, tone: 'image' },
  audio: { label: '音频', icon: <SoundOutlined />, tone: 'audio' },
}

type WorkCardProps = {
  work: WorkItem
  isCompose: boolean
  isBrollSource: boolean
  onOpen: () => void
  onPreview: (asset: MediaAsset) => void
  onPublish: () => void
}

function WorkCard({ work, isCompose, isBrollSource, onOpen, onPreview, onPublish }: WorkCardProps) {
  const title = cleanWorkTitle(work.title)
  const kind = KIND_META[work.kind] || { label: work.kind, icon: <FileTextOutlined />, tone: 'article' }
  const previewUrl = work.kind === 'video' || work.kind === 'image' ? work.media_urls?.[0] : undefined
  const canBroll = work.kind === 'video' && work.id.startsWith('g-')
  const statusLabel = STATUS_LABEL[work.status] || work.status
  const dateStr = new Date(work.created_at).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })

  const handlePreview = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!previewUrl) return
    onPreview({
      id: work.id,
      tenant_id: '',
      brand_id: work.brand_id || '',
      owner_type: 'creation',
      type: work.kind,
      name: work.title,
      url: previewUrl,
      mime: work.kind === 'image' ? 'image/jpeg' : 'video/mp4',
      size_bytes: 0,
      width: 0,
      height: 0,
      duration: 0,
      created_at: work.created_at,
    })
  }

  return (
    <article
      className={`mw-card mw-card--${work.status} mw-card--${kind.tone}`}
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onOpen() }}
    >
      <div className="mw-cover">
        {work.kind === 'video' && previewUrl && (
          <VideoFrameCover url={previewUrl} poster={work.cover_url} />
        )}
        {work.kind === 'image' && (work.cover_url || previewUrl) && (
          <ImageCover url={work.cover_url || previewUrl!} />
        )}
        {!previewUrl && (
          <div className="mw-cover-placeholder" aria-hidden>
            <span className="mw-cover-placeholder-icon">{kind.icon}</span>
          </div>
        )}

        <div className="mw-badge-row">
          <span className={`mw-status mw-status--${work.status}`}>{statusLabel}</span>
          <div className="mw-badge-row-right">
            {isCompose && <span className="mw-chip mw-chip--broll">B-Roll</span>}
            {isBrollSource && <span className="mw-chip mw-chip--broll-src">已插画面</span>}
            <span className="mw-chip mw-chip--kind">{kind.label}</span>
          </div>
        </div>

        {previewUrl && work.kind === 'video' && (
          <span className="mw-cover-play" aria-hidden><PlayCircleOutlined /></span>
        )}
      </div>

      <div className="mw-body">
        <h3 className="mw-title" title={work.title}>{title}</h3>
        <div className="mw-meta">
          <time className="mw-time">{dateStr}</time>
          {work.status === 'published' && work.platforms?.length ? (
            <span className="mw-platforms">
              {work.platforms.map((pf) => (
                <i key={pf}>{PLATFORM_LABEL[pf] || pf}</i>
              ))}
            </span>
          ) : null}
          {work.views > 0 && (
            <span className="mw-stat"><PlayCircleOutlined /> {work.views.toLocaleString()}</span>
          )}
        </div>

        <div className="mw-foot" onClick={(e) => e.stopPropagation()}>
          {canBroll && work.status !== 'published' && (
            <button type="button" className="mw-action" onClick={onOpen}>
              <VideoCameraAddOutlined /> 插入画面
            </button>
          )}
          {work.status !== 'published' && (
            <button type="button" className="mw-action mw-action--primary" onClick={onPublish}>
              <SendOutlined /> 去发布
            </button>
          )}
          {work.status === 'published' && previewUrl && (
            <button type="button" className="mw-action" onClick={handlePreview}>
              预览
            </button>
          )}
          <button type="button" className="mw-action mw-action--ghost" onClick={onOpen}>
            详情 <RightOutlined />
          </button>
        </div>
      </div>
    </article>
  )
}

const FILTER_OPTIONS: { value: Filter; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'draft', label: '草稿' },
  { value: 'ready', label: '待发布' },
  { value: 'published', label: '已发布' },
]

/**
 * 我的作品：三源聚合的真实作品库（文章 + 多媒体产物 + 发布状态 + 互动数据）。
 */
export default function MyWorks() {
  const navigate = useNavigate()
  const [filter, setFilter] = useState<Filter>('all')
  const [q, setQ] = useState('')
  const [previewAsset, setPreviewAsset] = useState<MediaAsset | null>(null)

  const { works = [], tasks, isLoading, isError, refetch } = usePublishableWorks()
  const { composeWorkIds, brollSourceWorkIds } = useMemo(() => brollLineage(tasks), [tasks])

  const counts = useMemo(() => ({
    all: works.length,
    draft: works.filter((w) => w.status === 'draft').length,
    ready: works.filter((w) => w.status === 'ready').length,
    published: works.filter((w) => w.status === 'published').length,
  }), [works])

  const list = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return works.filter((w) => {
      if (filter !== 'all' && w.status !== filter) return false
      if (needle && !w.title.toLowerCase().includes(needle)) return false
      return true
    })
  }, [works, filter, q])

  const openDetail = (w: WorkItem) => navigate(`/m/works/${encodeURIComponent(w.id)}`)

  return (
    <div className="wr-page-content ip-page mw-page">

      <header className="mw-head">
        <div className="mw-head-copy">
          <h1>我的作品</h1>
          <p>成片可插入画面后再发布，待发布可直达发布中心</p>
        </div>
        <Button
          type="primary"
          size="large"
          className="ip-btn-primary mw-head-cta"
          icon={<PlusOutlined />}
          onClick={() => navigate('/m/compose')}
        >
          去创作
        </Button>
      </header>

      <div className="mw-toolbar">
        <div className="mw-filters" role="tablist" aria-label="作品筛选">
          {FILTER_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              type="button"
              role="tab"
              aria-selected={filter === opt.value}
              className={`mw-filter${filter === opt.value ? ' is-active' : ''}`}
              onClick={() => setFilter(opt.value)}
            >
              {opt.label}
              <span className="mw-filter-count">{counts[opt.value]}</span>
            </button>
          ))}
        </div>
        <Input.Search
          allowClear
          className="mw-search"
          placeholder="搜索标题"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>

      <QueryBoundary
        loading={isLoading}
        error={isError}
        onRetry={() => refetch()}
        empty={list.length === 0}
        emptyText="还没有作品，先拍一条口播成片吧"
        emptyExtra={
          <Button type="primary" className="ip-btn-primary" icon={<PlusOutlined />} onClick={() => navigate('/m/compose/lipsync')}>
            去拍口播
          </Button>
        }
      >
        <div className="mw-grid">
          {list.map((w) => (
            <WorkCard
              key={w.id}
              work={w}
              isCompose={composeWorkIds.has(w.id)}
              isBrollSource={brollSourceWorkIds.has(w.id)}
              onOpen={() => openDetail(w)}
              onPreview={setPreviewAsset}
              onPublish={() => navigate(distributionPathFromWork(w))}
            />
          ))}
        </div>
      </QueryBoundary>

      <MediaPreviewModal
        open={!!previewAsset}
        asset={previewAsset}
        onClose={() => setPreviewAsset(null)}
      />
    </div>
  )
}

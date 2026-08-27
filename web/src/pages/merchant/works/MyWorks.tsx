import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Button, Empty, Input, Segmented, Space, Tag, Typography } from 'antd'
import { PlusOutlined, SendOutlined } from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { GenerationFailedBar } from '../../../components/compose/GenerationFailedBar'
import { retryFailureMessage } from '../../../components/RetryHint'
import { usePublishableWorks } from '../../../hooks/usePublishableWorks'
import { MediaPreviewModal } from '../../../components/MediaPreviewModal'
import { VideoFrameCover } from '../../../components/VideoFrameCover'
import { ImageCover } from '../../../components/ImageCover'
import type { GenerationTask, MediaAsset, WorkItem } from '../../../types/api'

const { Text } = Typography

type Filter = 'all' | 'draft' | 'ready' | 'published'

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

  const { works = [], isLoading } = usePublishableWorks()
  const { data: failedTasks = [] } = useQuery({
    queryKey: ['generation-tasks'],
    queryFn: () => businessApi.listGenerationTasks()
      .then((r) => r.tasks.filter((t) => t.state === 'failed').slice(0, 8))
      .catch(() => []),
    staleTime: 30_000,
  })

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
          <p className="ip-kicker">Library</p>
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

      {isLoading ? (
        <Empty description="加载中…" style={{ padding: 60 }} />
      ) : list.length === 0 ? (
        <Empty style={{ padding: 60 }} description="还没有作品——去内容合成写第一篇文章或做第一个视频">
          <Button type="primary" onClick={() => navigate('/m/compose')}>去内容合成</Button>
        </Empty>
      ) : (
        <div className="ip-works-grid">
          {list.map((w) => {
            const st = STATUS_CONFIG[w.status] || { label: w.status, color: 'default' }
            const kd = KIND_CONFIG[w.kind] || { label: w.kind, emoji: '📦' }
            return (
              <div key={w.id} className="ip-work-card">
                <div
                  className="ip-work-cover"
                  style={{
                    position: 'relative',
                    overflow: 'hidden',
                    background: w.kind === 'video' || w.kind === 'image'
                      ? undefined
                      : w.cover_url
                        ? `linear-gradient(180deg, rgba(0,0,0,0.1), rgba(0,0,0,0.55)), url(${w.cover_url}) center/cover`
                        : 'linear-gradient(145deg, #12121a, #1f2937)',
                    display: 'flex',
                    alignItems: 'flex-end',
                    padding: 12,
                  }}
                >
                  {w.kind === 'video' && w.media_urls?.[0] && (
                    <VideoFrameCover url={w.media_urls[0]} poster={w.cover_url} />
                  )}
                  {w.kind === 'image' && (w.cover_url || w.media_urls?.[0]) && (
                    <ImageCover url={w.cover_url || w.media_urls![0]} />
                  )}
                  <Tag color={st.color} style={{ margin: 0, position: 'relative', zIndex: 1 }}>{st.label}</Tag>
                  {w.status === 'published' && w.platforms?.length ? (
                    <span className="ip-ratio" style={{ marginLeft: 8, position: 'relative', zIndex: 1 }}>{w.platforms.join(' · ')}</span>
                  ) : null}
                </div>
                <div className="ip-work-body">
                  <Text strong style={{ fontSize: 14, display: 'block' }} ellipsis={{ tooltip: w.title }}>
                    {kd.emoji} {w.title}
                  </Text>
                  <div className="ip-work-metrics">
                    <span>{kd.label}</span>
                    {w.views > 0 && <span>播放 {w.views.toLocaleString()}</span>}
                    {w.likes > 0 && <span>赞 {w.likes.toLocaleString()}</span>}
                  </div>
                  <Space style={{ marginTop: 12 }}>
                    {w.status !== 'published' && (
                      <Button size="small" type="primary" ghost icon={<SendOutlined />} onClick={() => navigate(distributionPath(w))}>
                        去发布
                      </Button>
                    )}
                    {(w.kind === 'video' || w.kind === 'image') && w.media_urls?.[0] && (
                      <Button size="small" onClick={() => setPreviewAsset({
                        id: w.id,
                        tenant_id: '',
                        brand_id: w.brand_id || '',
                        owner_type: 'creation',
                        type: w.kind,
                        name: w.title,
                        url: w.media_urls![0],
                        mime: w.kind === 'image' ? 'image/jpeg' : 'video/mp4',
                        size_bytes: 0,
                        width: 0,
                        height: 0,
                        duration: 0,
                        created_at: w.created_at,
                      })}>预览</Button>
                    )}
                  </Space>
                </div>
              </div>
            )
          })}
        </div>
      )}

      <MediaPreviewModal
        open={!!previewAsset}
        asset={previewAsset}
        onClose={() => setPreviewAsset(null)}
      />

      {failedTasks.length > 0 && (
        <div style={{ marginTop: 32 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <Text strong>生成失败任务</Text>
            <Button type="link" size="small" onClick={() => navigate('/m/compose/tools?tab=media')}>打开任务中心</Button>
          </div>
          <div className="mw-failed-list" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {failedTasks.map((t: GenerationTask) => (
              <GenerationFailedBar
                key={t.id}
                compact
                message={`${t.sub_type} · ${t.model}：${retryFailureMessage(t, '生成失败')}`}
                onRetry={() => navigate('/m/compose/tools?tab=media')}
                retryLabel="去任务中心"
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

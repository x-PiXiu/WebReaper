import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import {
  Button,
  Checkbox,
  Empty,
  Input,
  Popconfirm,
  Spin,
  Upload,
} from 'antd'
import {
  ClockCircleOutlined,
  CloudUploadOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
  PictureOutlined,
  PlusOutlined,
  SearchOutlined,
  SoundOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { GenerateAssetModal } from '../../../components/assets/GenerateAssetModal'
import { ImageCover } from '../../../components/ImageCover'
import { MediaPreviewModal } from '../../../components/MediaPreviewModal'
import { VideoFrameCover } from '../../../components/VideoFrameCover'
import { CREATIVE_CDN } from '../../../config/creativeCdn'
import { GENERATION_TASKS_KEY, useGenerationTasks } from '../../../hooks/useGenerationTasks'
import { MEDIA_ASSETS_QUERY_KEY, useMediaAssets } from '../../../hooks/useMediaAssets'
import type { MediaAsset } from '../../../types/api'
import { message } from '../../../utils/antdApp'
import {
  inferMediaKind,
  isImageMedia,
  isVideoMedia,
  mediaAssetsFromGenerationTasks,
  mergeMediaByUrl,
} from '../../../utils/generationTask'
import { parseGenerationTaskParams } from '../../../utils/subjectTask'

type SourceTab = 'mine' | 'system'
type TypeFilter = 'all' | 'image' | 'video'

const QUOTA_LIMIT = 200

const SYSTEM_SHOWCASE: MediaAsset[] = [
  {
    id: 'sys-1',
    tenant_id: '',
    brand_id: '',
    type: 'image',
    name: '系统分镜 · 室内口播',
    url: CREATIVE_CDN.pipeline.copy,
    mime: 'image/jpeg',
    size_bytes: 0,
    width: 0,
    height: 0,
    duration: 0,
    owner_type: 'system',
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'sys-2',
    tenant_id: '',
    brand_id: '',
    type: 'image',
    name: '系统分镜 · 门店实拍',
    url: CREATIVE_CDN.pipeline.voice,
    mime: 'image/jpeg',
    size_bytes: 0,
    width: 0,
    height: 0,
    duration: 0,
    owner_type: 'system',
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'sys-3',
    tenant_id: '',
    brand_id: '',
    type: 'video',
    name: '系统分镜 · 产品特写',
    url: CREATIVE_CDN.pipeline.film,
    cover_url: CREATIVE_CDN.pipeline.film,
    mime: 'video/mp4',
    size_bytes: 0,
    width: 0,
    height: 0,
    duration: 8,
    owner_type: 'system',
    created_at: '2024-01-01T00:00:00Z',
  },
]

function formatSize(bytes: number) {
  if (!bytes) return ''
  if (bytes > 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB'
  if (bytes > 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
  if (bytes > 1 << 10) return (bytes >> 10) + ' KB'
  return bytes + ' B'
}

function timeAgo(iso: string) {
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60_000)
  if (m < 1) return '刚刚'
  if (m < 60) return `${m} 分钟前`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} 小时前`
  return `${Math.floor(h / 24)} 天前`
}

/**
 * 分镜素材：图片 / 视频上传与管理（列表布局）。
 * 数字人创建深链改走数字人库；音频生成仍可从工具入口打开。
 */
export default function AssetLibrary() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const queryClient = useQueryClient()

  const [source, setSource] = useState<SourceTab>('mine')
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all')
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [generateOpen, setGenerateOpen] = useState(false)
  const [generateType, setGenerateType] = useState<'image' | 'video' | 'audio' | 'voice'>('image')
  const [previewAsset, setPreviewAsset] = useState<MediaAsset | null>(null)
  const [uploading, setUploading] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const { data: assets = [], isLoading } = useMediaAssets(true, 'all')
  const { tasks: genTasks = [] } = useGenerationTasks()

  const myVoices = useMemo(() => {
    const ids = new Set<string>()
    for (const t of genTasks || []) {
      if (t.sub_type !== 'voice_clone' || t.state !== 'success') continue
      const vid = parseGenerationTaskParams(t).voice_id
      if (typeof vid === 'string' && vid) ids.add(vid)
    }
    return Array.from(ids)
  }, [genTasks])

  // 旧深链：创建数字人 → 数字人库
  useEffect(() => {
    if (searchParams.get('create') === 'subject') {
      navigate('/m/compose/avatar?create=1', { replace: true })
    }
  }, [searchParams, navigate, setSearchParams])

  const imageCount = useMemo(() => {
    const fromDisk = assets.filter((a) => isImageMedia(a.mime, a.url, a.type))
    const fromTasks = mediaAssetsFromGenerationTasks(genTasks, 'image')
    return mergeMediaByUrl(fromDisk, fromTasks).length
  }, [assets, genTasks])

  const videoCount = useMemo(() => {
    const fromDisk = assets.filter((a) => isVideoMedia(a.mime, a.url, a.type))
    const fromTasks = mediaAssetsFromGenerationTasks(genTasks, 'video')
    return mergeMediaByUrl(fromDisk, fromTasks).length
  }, [assets, genTasks])

  const mineList = useMemo(() => {
    const images = mergeMediaByUrl(
      assets.filter((a) => isImageMedia(a.mime, a.url, a.type)),
      mediaAssetsFromGenerationTasks(genTasks, 'image'),
    )
    const videos = mergeMediaByUrl(
      assets.filter((a) => isVideoMedia(a.mime, a.url, a.type)),
      mediaAssetsFromGenerationTasks(genTasks, 'video'),
    )
    return [...images, ...videos].sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    )
  }, [assets, genTasks])

  const filtered = useMemo(() => {
    const base = source === 'system' ? SYSTEM_SHOWCASE : mineList
    const needle = q.trim().toLowerCase()
    return base.filter((a) => {
      const kind = inferMediaKind(a.mime, a.url, a.type)
      if (typeFilter === 'image' && kind !== 'image') return false
      if (typeFilter === 'video' && kind !== 'video') return false
      if (!needle) return true
      return (
        (a.name || '').toLowerCase().includes(needle)
        || a.url.toLowerCase().includes(needle)
      )
    })
  }, [source, mineList, typeFilter, q])

  const toggleSelect = (id: string, checked: boolean) => {
    setSelected((prev) => {
      if (checked) return prev.includes(id) ? prev : [...prev, id]
      return prev.filter((x) => x !== id)
    })
  }

  const batchDelete = async () => {
    if (selected.length === 0) return
    const deletable = selected.filter((id) => !id.startsWith('gen-task:') && !id.startsWith('sys-'))
    if (deletable.length === 0) {
      message.info('选中项来自生成任务或系统素材，无法在此删除')
      return
    }
    setDeleting(true)
    try {
      await Promise.all(deletable.map((id) => businessApi.deleteAsset(id)))
      message.success(`已删除 ${deletable.length} 个素材`)
      setSelected([])
      queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY })
    } catch { /* 拦截器 */ } finally {
      setDeleting(false)
    }
  }

  const uploadFiles = async (files: File[]) => {
    setUploading(true)
    try {
      for (const file of files) {
        await businessApi.uploadAsset(file)
      }
      message.success(`已上传 ${files.length} 个素材`)
      queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY })
      setSource('mine')
    } catch { /* 拦截器 */ } finally {
      setUploading(false)
    }
  }

  return (
    <div className="sb-lib">
      <header className="sb-lib-head">
        <div className="sb-lib-titles">
          <h1 className="sb-lib-title">分镜素材</h1>
          <p className="sb-lib-lead">上传并管理您的分镜素材，支持图片和视频</p>
        </div>

        <div className="sb-lib-info-row">
          <div className="sb-lib-info-card">
            <span className="sb-lib-info-icon" aria-hidden>
              <InfoCircleOutlined />
            </span>
            <div>
              <strong>用量额度</strong>
              <p>
                视频：{videoCount}/{QUOTA_LIMIT}
                <span className="sb-lib-info-sep">|</span>
                图片：{imageCount}/{QUOTA_LIMIT}
              </p>
            </div>
          </div>
          <div className="sb-lib-info-card">
            <span className="sb-lib-info-icon" aria-hidden>
              <ClockCircleOutlined />
            </span>
            <div>
              <strong>清理规则</strong>
              <p>超过 7 天未使用的素材将自动清理，请及时用于创作</p>
            </div>
          </div>
        </div>

        <div className="sb-lib-toolbar">
          <div className="sb-lib-tabs" role="tablist">
            {(
              [
                { key: 'mine', label: '我的素材' },
                { key: 'system', label: '系统素材' },
              ] as const
            ).map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={source === t.key}
                className={`sb-lib-tab${source === t.key ? ' is-active' : ''}`}
                onClick={() => {
                  setSource(t.key)
                  setSelected([])
                }}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className="sb-lib-actions">
            <Input
              allowClear
              className="sb-lib-search"
              placeholder="搜索素材"
              prefix={<SearchOutlined />}
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
            {source === 'mine' && (
              <>
                <Upload
                  accept="image/*,video/*"
                  showUploadList={false}
                  multiple
                  beforeUpload={(file) => {
                    void uploadFiles([file])
                    return false
                  }}
                >
                  <Button
                    type="primary"
                    className="sb-lib-btn-primary"
                    icon={<PlusOutlined />}
                    loading={uploading}
                  >
                    上传素材
                  </Button>
                </Upload>
                <Button
                  className="sb-lib-btn-ghost"
                  icon={<CloudUploadOutlined />}
                  onClick={() => {
                    setGenerateType(typeFilter === 'video' ? 'video' : 'image')
                    setGenerateOpen(true)
                  }}
                >
                  AI 生成
                </Button>
                <Popconfirm
                  title={`删除选中的 ${selected.length} 个素材？`}
                  okText="删除"
                  okButtonProps={{ danger: true, loading: deleting }}
                  cancelText="取消"
                  disabled={selected.length === 0}
                  onConfirm={batchDelete}
                >
                  <Button
                    className="sb-lib-btn-ghost"
                    icon={<DeleteOutlined />}
                    disabled={selected.length === 0}
                  >
                    批量删除 ({selected.length})
                  </Button>
                </Popconfirm>
              </>
            )}
          </div>
        </div>

        <div className="sb-lib-filters">
          <div className="sb-lib-type-tabs" role="tablist">
            {(
              [
                { key: 'all', label: '全部' },
                { key: 'image', label: '图片' },
                { key: 'video', label: '视频' },
              ] as const
            ).map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={typeFilter === t.key}
                className={`sb-lib-type-tab${typeFilter === t.key ? ' is-active' : ''}`}
                onClick={() => setTypeFilter(t.key)}
              >
                {t.label}
              </button>
            ))}
          </div>
          <p className="sb-lib-filter-note">
            {source === 'system' ? '标准素材：不可用于商业' : '标签筛选：无可用标签'}
          </p>
        </div>
      </header>

      {isLoading && source === 'mine' && mineList.length === 0 ? (
        <div className="sb-lib-empty">
          <Spin size="large" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="sb-lib-empty">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={source === 'system' ? '暂无系统素材' : '暂无数据'}
          >
            {source === 'mine' && (
              <Upload
                accept="image/*,video/*"
                showUploadList={false}
                multiple
                beforeUpload={(file) => {
                  void uploadFiles([file])
                  return false
                }}
              >
                <Button type="primary" className="sb-lib-btn-primary" icon={<PlusOutlined />}>
                  上传素材
                </Button>
              </Upload>
            )}
          </Empty>
        </div>
      ) : (
        <ul className="sb-lib-list" role="list">
          {filtered.map((a) => {
            const kind = inferMediaKind(a.mime, a.url, a.type)
            const isSystem = a.id.startsWith('sys-') || a.owner_type === 'system'
            const selectable = source === 'mine' && !a.id.startsWith('gen-task:')
            const checked = selected.includes(a.id)
            const displayName = a.name || a.url.split('/').pop()?.split('?')[0] || '素材'
            const openPreview = () => {
              if (isSystem) {
                message.info('系统素材仅供参考，请上传或生成自有素材用于商业创作')
                return
              }
              setPreviewAsset(a)
            }
            return (
              <li
                key={a.id}
                className={`sb-lib-row${checked ? ' is-selected' : ''}${isSystem ? ' is-system' : ''}`}
              >
                {selectable ? (
                  <label className="sb-lib-row-check">
                    <Checkbox
                      checked={checked}
                      onChange={(e) => toggleSelect(a.id, e.target.checked)}
                    />
                  </label>
                ) : (
                  <span className="sb-lib-row-check is-spacer" aria-hidden />
                )}

                <button
                  type="button"
                  className="sb-lib-row-thumb"
                  onClick={openPreview}
                  aria-label={`预览 ${displayName}`}
                >
                  {kind === 'image' && <ImageCover url={a.url} />}
                  {kind === 'video' && <VideoFrameCover url={a.url} poster={a.cover_url} />}
                  {kind === 'audio' && (
                    <span className="sb-lib-row-placeholder">
                      <SoundOutlined />
                    </span>
                  )}
                </button>

                <button type="button" className="sb-lib-row-main" onClick={openPreview}>
                  <strong className="sb-lib-row-name" title={displayName}>{displayName}</strong>
                  <span className="sb-lib-row-meta">
                    {formatSize(a.size_bytes) && <span>{formatSize(a.size_bytes)}</span>}
                    {!isSystem && <span>{timeAgo(a.created_at)}</span>}
                    {isSystem && <span>系统</span>}
                    {a.owner_type === 'creation' && <span>AI 产物</span>}
                  </span>
                </button>

                <span className={`sb-lib-row-kind sb-lib-row-kind--${kind}`}>
                  {kind === 'video' ? <VideoCameraOutlined /> : kind === 'audio' ? <SoundOutlined /> : <PictureOutlined />}
                  {kind === 'video' ? '视频' : kind === 'audio' ? '音频' : '图片'}
                </span>

                <Button type="link" className="sb-lib-row-action" onClick={openPreview}>
                  {isSystem ? '仅供参考' : '预览'}
                </Button>
              </li>
            )
          })}
        </ul>
      )}

      <MediaPreviewModal
        open={!!previewAsset}
        asset={previewAsset}
        onClose={() => setPreviewAsset(null)}
      />

      <GenerateAssetModal
        open={generateOpen}
        type={generateType}
        myVoices={myVoices}
        onClose={() => setGenerateOpen(false)}
        onGenerated={() => {
          queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY })
          queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
          setSource('mine')
        }}
      />
    </div>
  )
}

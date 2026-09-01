import { useMemo, useState } from 'react'
import { Button, Empty, Modal, Popconfirm, Tag, Typography, Upload } from 'antd'
import { MODAL_W } from '../ui/modalFit'
import { UploadOutlined, SoundOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import { MEDIA_ASSETS_QUERY_KEY, normalizeMediaAssets, normalizeUploadedAsset, useMediaAssets } from '../hooks/useMediaAssets'
import { useGenerationTasks } from '../hooks/useGenerationTasks'
import type { MediaAsset } from '../types/api'
import {
  isAudioMedia,
  isImageMedia,
  isVideoMedia,
  mediaAssetsFromGenerationTasks,
  mergeMediaByUrl,
} from '../utils/generationTask'
import { VideoFrameCover } from './VideoFrameCover'
import { ImageCover } from './ImageCover'
import { toast } from '../utils/feedback'

const { Dragger } = Upload
const { Text } = Typography

// 素材库统一选择器（多媒体创作 / 社媒分发配图共用——此前两套独立实现）。
// 返回完整资产对象（引用需携带 url；id 用于上传素材，AI 产物以 url 为准）。

const ASSET_ACCEPT: Record<string, string> = {
  image: 'image/png,image/jpeg,image/webp',
  video: 'video/mp4,video/webm,video/quicktime',
  audio: 'audio/mpeg,audio/mp4,audio/wav',
  // 图片+视频（B-Roll 插入片段：图片按句子时长自动转视频）
  visual: 'image/png,image/jpeg,image/webp,video/mp4,video/webm,video/quicktime',
  any: 'image/png,image/jpeg,image/webp,video/mp4,video/webm,video/quicktime,audio/mpeg,audio/mp4,audio/wav',
}

export type AssetAccept = 'image' | 'video' | 'audio' | 'visual' | 'any'

function filterDiskAssets(list: MediaAsset[], accept: AssetAccept): MediaAsset[] {
  const normalized = normalizeMediaAssets(list)
  if (accept === 'any') return normalized
  if (accept === 'image') {
    return normalized.filter(a => isImageMedia(a.mime, a.url, a.type))
  }
  if (accept === 'audio') {
    return normalized.filter(a => isAudioMedia(a.mime, a.url, a.type))
  }
  if (accept === 'visual') {
    return normalized.filter(a => isImageMedia(a.mime, a.url, a.type) || isVideoMedia(a.mime, a.url, a.type))
  }
  return normalized.filter(a => isVideoMedia(a.mime, a.url, a.type))
}

export default function AssetPicker(props: {
  open: boolean
  mode: 'single' | 'multi'
  accept: AssetAccept
  title: string
  max?: number
  onClose: () => void
  onSelect: (assets: MediaAsset[]) => void
}) {
  const { open, mode, accept, title, max, onClose, onSelect } = props
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<MediaAsset[]>([])
  const [uploading, setUploading] = useState(false)

  const { data: assets = [] } = useMediaAssets(open, 'all')
  const { tasks: genTasks = [] } = useGenerationTasks({ enabled: open })

  const filtered = useMemo(() => {
    const fromDisk = filterDiskAssets(assets, accept)
    // visual（B-Roll 片段）合并图片+视频的生成产物；any 不并入任务产物（维持原行为）
    const taskKinds: Array<'image' | 'video' | 'audio'> = accept === 'visual'
      ? ['image', 'video']
      : accept === 'any' ? [] : [accept]
    if (taskKinds.length === 0) return fromDisk
    const fromTasks = taskKinds.flatMap((k) => mediaAssetsFromGenerationTasks(genTasks, k))
    return mergeMediaByUrl(fromDisk, fromTasks)
  }, [assets, accept, genTasks])

  const appendAsset = (asset: MediaAsset) => {
    if (mode === 'single') {
      onSelect([asset])
      onClose()
      return
    }
    setSelected(prev => {
      if (prev.some(x => x.url === asset.url)) return prev
      if (max && prev.length >= max) {
        toast.warn(`最多选择 ${max} 个`)
        return prev
      }
      return [...prev, asset]
    })
  }

  const uploadProps = {
    accept: ASSET_ACCEPT[accept],
    showUploadList: false,
    multiple: mode === 'multi',
    beforeUpload: async (file: File) => {
      setUploading(true)
      try {
        const asset = normalizeUploadedAsset(await businessApi.uploadAsset(file))
        toast.ok('上传成功', 'asset-pick')
        queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY })
        appendAsset(asset)
      } catch {
        // 错误已由拦截器提示
      } finally {
        setUploading(false)
      }
      return false
    },
  }

  const toggleSelect = (a: MediaAsset) => {
    if (mode === 'single') {
      appendAsset(a)
      return
    }
    setSelected(prev => {
      if (prev.some(x => x.id === a.id || x.url === a.url)) {
        return prev.filter(x => x.id !== a.id && x.url !== a.url)
      }
      if (max && prev.length >= max) {
        toast.warn(`最多选择 ${max} 个`)
        return prev
      }
      return [...prev, a]
    })
  }

  const confirm = () => {
    if (selected.length === 0) {
      toast.warn('请先选择素材')
      return
    }
    onSelect(selected)
    setSelected([])
    onClose()
  }

  return (
    <Modal
      title={title}
      open={open}
      onCancel={() => { setSelected([]); onClose() }}
      width={MODAL_W.xl}
      footer={mode === 'multi' ? [
        <Button key="cancel" onClick={() => { setSelected([]); onClose() }}>取消</Button>,
        <Button key="ok" type="primary" onClick={confirm} disabled={selected.length === 0}>
          确认选择（{selected.length}）
        </Button>,
      ] : null}
    >
      <Dragger {...uploadProps} disabled={uploading} style={{ marginBottom: 16 }}>
        <p className="ant-upload-drag-icon"><UploadOutlined /></p>
        <p className="ant-upload-text">本地上传 或 拖拽到此处</p>
        <p className="ant-upload-hint">下方列表含上传素材与 AI 生成产物 · 单文件 ≤ 20MB</p>
      </Dragger>

      {filtered.length === 0 ? (
        <Empty description="暂无素材，可先上传或去生成图片后再选" />
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, maxHeight: 320, overflow: 'auto' }}>
          {filtered.map(a => {
            const isImage = isImageMedia(a.mime, a.url, a.type)
            const isVideo = isVideoMedia(a.mime, a.url, a.type)
            const isAudio = isAudioMedia(a.mime, a.url, a.type)
            const active = selected.some(x => x.id === a.id || x.url === a.url)
            const isAi = a.owner_type === 'creation' || a.id.startsWith('gen-task:')
            return (
              <div
                key={a.id + a.url}
                onClick={() => toggleSelect(a)}
                style={{
                  border: active ? '2px solid var(--wr-primary)' : '1px solid var(--wr-border, #e5e7eb)',
                  borderRadius: 8, overflow: 'hidden', cursor: 'pointer', position: 'relative',
                  background: '#fff',
                }}
              >
                <div style={{ height: 90, position: 'relative', overflow: 'hidden', background: '#f5f5f5' }}>
                  {isImage ? (
                    <ImageCover url={a.url} />
                  ) : isVideo ? (
                    <VideoFrameCover url={a.url} poster={a.cover_url} />
                  ) : isAudio ? (
                    <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <SoundOutlined style={{ fontSize: 28, color: '#8c8c8c' }} />
                    </div>
                  ) : (
                    <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 12, color: '#8c8c8c' }}>文件</div>
                  )}
                </div>
                <div style={{ padding: '6px 8px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 4 }}>
                  <Text style={{ fontSize: 11 }} type="secondary" ellipsis>
                    {isAi ? <Tag color="cyan" style={{ margin: 0, fontSize: 10 }}>AI</Tag> : null}
                    {a.name || `${Math.round(a.size_bytes / 1024) || '?'}KB`}
                  </Text>
                  {!a.id.startsWith('gen-task:') && (
                    <Popconfirm title="删除该素材？" onConfirm={async () => {
                      await businessApi.deleteAsset(a.id)
                      queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY })
                    }}>
                      <DeleteOutlined style={{ color: '#999', fontSize: 12 }} onClick={e => e.stopPropagation()} />
                    </Popconfirm>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </Modal>
  )
}

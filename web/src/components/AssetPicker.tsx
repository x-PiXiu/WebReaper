import { useMemo, useState } from 'react'
import { Button, Empty, Modal, Popconfirm, Typography, Upload, message } from 'antd'
import { MODAL_W } from '../ui/modalFit'
import { UploadOutlined, SoundOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../api/business'
import { MEDIA_ASSETS_QUERY_KEY, normalizeMediaAssets, useMediaAssets } from '../hooks/useMediaAssets'
import type { MediaAsset } from '../types/api'

const { Dragger } = Upload
const { Text } = Typography

// 素材库统一选择器（多媒体创作 / 社媒分发配图共用——此前两套独立实现）。
// 返回完整资产对象（引用需携带 name/kind）；支持就地上传，无需先去创作页。

const ASSET_ACCEPT: Record<string, string> = {
  image: 'image/png,image/jpeg,image/webp',
  video: 'video/mp4,video/webm,video/quicktime',
  audio: 'audio/mpeg,audio/mp4,audio/wav',
  any: 'image/png,image/jpeg,image/webp,video/mp4,video/webm,video/quicktime,audio/mpeg,audio/mp4,audio/wav',
}

export default function AssetPicker(props: {
  open: boolean
  mode: 'single' | 'multi'
  accept: 'image' | 'video' | 'audio' | 'any'
  title: string
  max?: number
  onClose: () => void
  onSelect: (assets: MediaAsset[]) => void
}) {
  const { open, mode, accept, title, max, onClose, onSelect } = props
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<MediaAsset[]>([])
  const [uploading, setUploading] = useState(false)

  // 选视频/任意形态时拉素材+AI产物（成片视频主要落在 creation）——否则只拉上传素材
  const { data: assets = [] } = useMediaAssets(open, accept === 'video' || accept === 'any' ? 'all' : 'material')

  const filtered = useMemo(() => {
    const list = normalizeMediaAssets(assets)
    const kinds = accept === 'any' ? ['image', 'video', 'audio'] : [accept]
    return list.filter(a => kinds.some(k => a.mime.startsWith(k)))
  }, [assets, accept])

  const uploadProps = {
    accept: ASSET_ACCEPT[accept],
    showUploadList: false,
    beforeUpload: async (file: File) => {
      setUploading(true)
      try {
        await businessApi.uploadAsset(file)
        message.success('上传成功')
        queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY })
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
      setSelected([a])
      onSelect([a])
      onClose()
      return
    }
    setSelected(prev => {
      if (prev.some(x => x.id === a.id)) return prev.filter(x => x.id !== a.id)
      if (max && prev.length >= max) {
        message.warning(`最多选择 ${max} 个`)
        return prev
      }
      return [...prev, a]
    })
  }

  const confirm = () => {
    if (selected.length === 0) {
      message.warning('请先选择素材')
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
        <p className="ant-upload-text">点击或拖拽上传素材</p>
        <p className="ant-upload-hint">图片 png/jpg/webp · 音频 mp3/m4a/wav · 单文件 ≤ 20MB</p>
      </Dragger>

      {filtered.length === 0 ? (
        <Empty description="暂无素材，先上传一个吧" />
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, maxHeight: 320, overflow: 'auto' }}>
          {filtered.map(a => {
            const isImage = a.mime.startsWith('image')
            const active = selected.some(x => x.id === a.id)
            return (
              <div
                key={a.id}
                onClick={() => toggleSelect(a)}
                style={{
                  border: active ? '2px solid var(--wr-primary)' : '1px solid var(--wr-border, #e5e7eb)',
                  borderRadius: 8, overflow: 'hidden', cursor: 'pointer', position: 'relative',
                  background: '#fff',
                }}
              >
                <div style={{ height: 90, display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f5f5f5' }}>
                  {isImage ? (
                    <img src={a.url} alt={a.id} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
                  ) : (
                    <SoundOutlined style={{ fontSize: 28, color: '#8c8c8c' }} />
                  )}
                </div>
                <div style={{ padding: '6px 8px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Text style={{ fontSize: 11 }} type="secondary">
                    {Math.round(a.size_bytes / 1024)}KB · {isImage ? '图片' : '音频'}
                  </Text>
                  <Popconfirm title="删除该素材？" onConfirm={async () => { await businessApi.deleteAsset(a.id); queryClient.invalidateQueries({ queryKey: ['media-assets'] }) }}>
                    <DeleteOutlined style={{ color: '#999', fontSize: 12 }} onClick={e => e.stopPropagation()} />
                  </Popconfirm>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </Modal>
  )
}

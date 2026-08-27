import { Modal } from 'antd'
import type { MediaAsset } from '../types/api'
import { inferMediaKind, resolveMediaUrl } from '../utils/generationTask'

type Props = {
  open: boolean
  asset: MediaAsset | null
  onClose: () => void
}

/** 素材库内联预览（避免 window.open 触发 Content-Disposition 下载） */
export function MediaPreviewModal({ open, asset, onClose }: Props) {
  if (!asset) return null
  const kind = inferMediaKind(asset.mime, asset.url, asset.type)
  const title = asset.name || asset.url.split('/').pop()?.split('?')[0] || '预览'
  const src = resolveMediaUrl(asset.url)

  return (
    <Modal
      open={open}
      title={title}
      onCancel={onClose}
      footer={null}
      width={kind === 'audio' ? 480 : 720}
      destroyOnClose
      centered
    >
      {kind === 'image' && (
        <img src={src} alt={title} style={{ width: '100%', borderRadius: 8 }} />
      )}
      {kind === 'video' && (
        <video
          controls
          playsInline
          preload="metadata"
          src={src}
          style={{ width: '100%', maxHeight: '70vh', borderRadius: 8, background: '#000' }}
        />
      )}
      {kind === 'audio' && (
        <audio controls preload="metadata" src={src} style={{ width: '100%' }} />
      )}
    </Modal>
  )
}

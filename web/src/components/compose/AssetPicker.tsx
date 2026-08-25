import { useMemo } from 'react'
import { Empty, Modal, Spin } from 'antd'
import { MODAL_W } from '../../ui/modalFit'
import { useMediaAssets } from '../../hooks/useMediaAssets'

type Kind = 'image' | 'video' | 'audio' | 'all'

type Props = {
  open: boolean
  onClose: () => void
  onPick: (url: string, meta?: { id: string; mime: string }) => void
  kind?: Kind
  title?: string
}

/** 从素材库挑选已上传的媒体 URL */
export function AssetPicker({ open, onClose, onPick, kind = 'image', title = '从素材库选择' }: Props) {
  const { data: list = [], isLoading } = useMediaAssets(open)

  const assets = useMemo(() => {
    if (kind === 'all') return list
    const prefix = kind === 'image' ? 'image/' : kind === 'video' ? 'video/' : 'audio/'
    return list.filter((a) => a.mime.startsWith(prefix))
  }, [list, kind])

  return (
    <Modal open={open} title={title} onCancel={onClose} footer={null} width={MODAL_W.xl} destroyOnClose>
      {isLoading ? (
        <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>
      ) : assets.length === 0 ? (
        <Empty description="素材库暂无可用文件，可先上传" />
      ) : (
        <div className="cf-asset-pick-grid">
          {assets.map((a) => (
            <button
              key={a.id}
              type="button"
              className="cf-asset-pick-item"
              onClick={() => {
                onPick(a.url, { id: a.id, mime: a.mime })
                onClose()
              }}
            >
              {a.mime.startsWith('image/') ? (
                <span className="cf-asset-pick-thumb" style={{ backgroundImage: `url(${a.url})` }} />
              ) : (
                <span className="cf-asset-pick-thumb cf-asset-pick-file">{a.mime.split('/')[1] || 'file'}</span>
              )}
              <span className="cf-asset-pick-name">{a.url.split('/').pop()?.slice(0, 24) || a.id.slice(0, 8)}</span>
            </button>
          ))}
        </div>
      )}
    </Modal>
  )
}

import { useMemo } from 'react'
import { Empty, Modal, Spin, Tag } from 'antd'
import { MODAL_W } from '../../ui/modalFit'
import { useMediaAssets } from '../../hooks/useMediaAssets'
import { useGenerationTasks } from '../../hooks/useGenerationTasks'
import {
  isAudioMedia,
  isImageMedia,
  isVideoMedia,
  mediaAssetsFromGenerationTasks,
  mergeMediaByUrl,
} from '../../utils/generationTask'
import { VideoFrameCover } from '../VideoFrameCover'
import { ImageCover } from '../ImageCover'

type Kind = 'image' | 'video' | 'audio' | 'all'

type Props = {
  open: boolean
  onClose: () => void
  onPick: (url: string, meta?: { id: string; mime: string }) => void
  kind?: Kind
  title?: string
}

/** 从素材库挑选已上传 / AI 生成的媒体 URL */
export function AssetPicker({ open, onClose, onPick, kind = 'image', title = '从素材库选择' }: Props) {
  const { data: list = [], isLoading } = useMediaAssets(open, 'all')
  const { tasks: genTasks = [] } = useGenerationTasks({ enabled: open })

  const assets = useMemo(() => {
    const fromDisk = kind === 'all'
      ? list
      : kind === 'image'
        ? list.filter((a) => isImageMedia(a.mime, a.url, a.type))
        : kind === 'audio'
          ? list.filter((a) => isAudioMedia(a.mime, a.url, a.type))
          : list.filter((a) => isVideoMedia(a.mime, a.url, a.type))
    if (kind === 'all') return fromDisk
    const taskKind = kind === 'image' ? 'image' as const
      : kind === 'video' ? 'video' as const
      : 'audio' as const
    return mergeMediaByUrl(fromDisk, mediaAssetsFromGenerationTasks(genTasks, taskKind))
  }, [list, kind, genTasks])

  return (
    <Modal open={open} title={title} onCancel={onClose} footer={null} width={MODAL_W.xl} destroyOnHidden>
      {isLoading ? (
        <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>
      ) : assets.length === 0 ? (
        <Empty description="素材库暂无可用文件，可先上传或生成后再选" />
      ) : (
        <div className="cf-asset-pick-grid">
          {assets.map((a) => {
            const isAi = a.owner_type === 'creation' || a.id.startsWith('gen-task:')
            return (
              <button
                key={a.id + a.url}
                type="button"
                className="cf-asset-pick-item"
                onClick={() => {
                  onPick(a.url, { id: a.id, mime: a.mime })
                  onClose()
                }}
              >
                {isImageMedia(a.mime, a.url, a.type) ? (
                  <span className="cf-asset-pick-thumb" style={{ position: 'relative', overflow: 'hidden', padding: 0 }}>
                    <ImageCover url={a.url} />
                  </span>
                ) : isVideoMedia(a.mime, a.url, a.type) ? (
                  <span className="cf-asset-pick-thumb" style={{ position: 'relative', overflow: 'hidden', padding: 0 }}>
                    <VideoFrameCover url={a.url} poster={a.cover_url} />
                  </span>
                ) : isAudioMedia(a.mime, a.url, a.type) ? (
                  <span className="cf-asset-pick-thumb cf-asset-pick-file">音频</span>
                ) : (
                  <span className="cf-asset-pick-thumb cf-asset-pick-file">{a.mime.split('/')[1] || 'file'}</span>
                )}
                <span className="cf-asset-pick-name">
                  {isAi && <Tag color="cyan" style={{ marginRight: 4, fontSize: 10 }}>AI</Tag>}
                  {a.name || a.url.split('/').pop()?.slice(0, 24) || a.id.slice(0, 8)}
                </span>
              </button>
            )
          })}
        </div>
      )}
    </Modal>
  )
}

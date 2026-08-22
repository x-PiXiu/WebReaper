import { Button } from 'antd'
import { CloseOutlined } from '@ant-design/icons'

type Props = {
  kind: 'audio' | 'video' | 'image'
  url: string
  label?: string
  onClear?: () => void
}

/** 媒体结果卡片（替代裸露 URL 输入框） */
export function MediaResultCard({ kind, url, label, onClear }: Props) {
  return (
    <div className="cf-media-card">
      <div className="cf-media-card-preview">
        {kind === 'audio' && (
          <audio controls src={url} className="cf-media-audio" />
        )}
        {kind === 'video' && (
          <video controls playsInline src={url} className="cf-media-video" />
        )}
        {kind === 'image' && (
          <div className="cf-media-thumb" style={{ backgroundImage: `url(${url})` }} />
        )}
      </div>
      <div className="cf-media-card-meta">
        <strong>{label || (kind === 'audio' ? '配音' : kind === 'video' ? '成片' : '图片')}</strong>
        {onClear && (
          <Button type="text" size="small" icon={<CloseOutlined />} onClick={onClear} aria-label="移除" />
        )}
      </div>
    </div>
  )
}

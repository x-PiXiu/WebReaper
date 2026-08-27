import { Typography } from 'antd'
import { hasPauseMarkers, splitPauseMarkers } from '../../utils/pauseMarkers'

const { Text } = Typography

/** 口播文案停顿标记高亮预览 */
export function PauseMarkerPreview({ text }: { text: string }) {
  if (!hasPauseMarkers(text)) return null
  const parts = splitPauseMarkers(text)
  return (
    <div className="cf-pause-preview" aria-label="停顿标记预览">
      <Text type="secondary" className="cf-pause-preview-label">朗读预览</Text>
      <p className="cf-pause-preview-body">
        {parts.map((p, i) => (
          p.type === 'pause' ? (
            <mark key={`${i}-${p.sec}`} className="cf-pause-mark" title={p.value}>
              ⏸ {p.sec}s
            </mark>
          ) : (
            <span key={i}>{p.value}</span>
          )
        ))}
      </p>
    </div>
  )
}

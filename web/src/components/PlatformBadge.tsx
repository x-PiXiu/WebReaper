import type { CSSProperties } from 'react'
import { getPlatformMeta } from '../data/platforms'
import { PlatformIcon } from './PlatformIcon'

type Props = {
  platform?: string
  size?: number
  showLabel?: boolean
  className?: string
}

/** 平台徽章：Logo + 名称 */
export function PlatformBadge({ platform, size = 16, showLabel = true, className = '' }: Props) {
  const meta = getPlatformMeta(platform)
  return (
    <span
      className={`platform-badge ${className}`.trim()}
      style={{ '--platform-color': meta.color, '--platform-bg': meta.bg } as CSSProperties}
    >
      <PlatformIcon platform={platform} size={size} />
      {showLabel && <span className="platform-badge-label">{meta.label}</span>}
    </span>
  )
}

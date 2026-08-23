import { Link } from 'react-router-dom'
import { GROWTH_STAGES } from '../config/product'
import type { GrowthStageKey } from '../utils/growthStage'

type Props = {
  current?: GrowthStageKey
  className?: string
  style?: React.CSSProperties
  /** 创作台等页面：仅显示步骤标签，减少纵向占用 */
  compact?: boolean
}

/** 获客四步闭环导航（工作台 / 创作台共用） */
export function GrowthStagesNav({ current, className, style, compact }: Props) {
  const navClass = [
    className || 'ch-growth',
    compact ? 'ch-growth--compact' : '',
  ].filter(Boolean).join(' ')

  return (
    <nav className={navClass} aria-label="获客闭环" style={style}>
      {GROWTH_STAGES.map(s => (
        <Link
          key={s.key}
          to={s.path}
          className={`ch-growth-item${s.key === current ? ' is-current' : ''}`}
        >
          <span className="ch-growth-label">{s.label}</span>
          {!compact && <span className="ch-growth-desc">{s.desc}</span>}
        </Link>
      ))}
    </nav>
  )
}

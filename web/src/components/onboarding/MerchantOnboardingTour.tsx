import { useCallback, useEffect, useLayoutEffect, useState } from 'react'
import { createPortal } from 'react-dom'

type Placement = 'center' | 'right' | 'bottom' | 'top' | 'left'

type Step = {
  target: string | null
  placement: Placement
  kicker: string
  title: string
  lead: string
  cta?: string
}

const STEPS: Step[] = [
  {
    target: null,
    placement: 'center',
    kicker: 'Workspace',
    title: '工作台',
    lead: '人设、内容、发布与复盘，在同一界面持续推进。',
  },
  {
    target: 'merchant-nav',
    placement: 'right',
    kicker: 'Navigation',
    title: '模块入口',
    lead: '侧栏是日常动线。按需切换模块，不必一次走完。',
  },
  {
    target: 'growth-stages',
    placement: 'bottom',
    kicker: 'Flow',
    title: '四步闭环',
    lead: '建人设 · 出内容 · 发出去 · 看线索——顺序清晰，也可随时折返。',
  },
  {
    target: 'create-modes',
    placement: 'top',
    kicker: 'Create',
    title: '开写方式',
    lead: '口播、快速生成或图文，按当前任务选一条路径即可。',
  },
  {
    target: 'recent-panel',
    placement: 'top',
    kicker: 'Inbox',
    title: '进行中的事',
    lead: '草稿、待发作品与灵感入口集中在此，方便续写与发布。',
    cta: '进入工作台',
  },
]

type Rect = { top: number; left: number; width: number; height: number }

const PAD = 10
const GAP = 18
const PANEL_W = 360

function measureTarget(id: string | null): Rect | null {
  if (!id) return null
  const el = document.querySelector(`[data-tour="${id}"]`)
  if (!el) return null
  const r = el.getBoundingClientRect()
  return {
    top: r.top - PAD,
    left: r.left - PAD,
    width: r.width + PAD * 2,
    height: r.height + PAD * 2,
  }
}

function panelStyle(placement: Placement, spot: Rect | null): React.CSSProperties {
  if (!spot || placement === 'center') {
    return {
      top: '50%',
      left: '50%',
      transform: 'translate(-50%, -50%)',
      width: PANEL_W,
      maxWidth: 'min(92vw, 400px)',
    }
  }

  const vw = window.innerWidth
  const vh = window.innerHeight
  const base: React.CSSProperties = { width: PANEL_W, maxWidth: 'min(92vw, 400px)' }

  if (placement === 'right') {
    let left = spot.left + spot.width + GAP
    if (left + PANEL_W > vw - 16) left = Math.max(16, spot.left - PANEL_W - GAP)
    return { ...base, top: spot.top, left, transform: 'none' }
  }
  if (placement === 'left') {
    return { ...base, top: spot.top, left: Math.max(16, spot.left - PANEL_W - GAP), transform: 'none' }
  }
  if (placement === 'bottom') {
    let top = spot.top + spot.height + GAP
    let left = spot.left + spot.width / 2 - PANEL_W / 2
    left = Math.min(Math.max(16, left), vw - PANEL_W - 16)
    if (top + 220 > vh) top = Math.max(16, spot.top - 220 - GAP)
    return { ...base, top, left, transform: 'none' }
  }
  // top
  let top = spot.top - GAP
  let left = spot.left + spot.width / 2 - PANEL_W / 2
  left = Math.min(Math.max(16, left), vw - PANEL_W - 16)
  return { ...base, top, left, transform: 'translateY(-100%)' }
}

type Props = {
  open: boolean
  onClose: () => void
  onFinish: () => void
}

/** 商户工作台导览：自定义聚光灯 + 玻璃面板（替代默认 Tour 样式） */
export default function MerchantOnboardingTour({ open, onClose, onFinish }: Props) {
  const [index, setIndex] = useState(0)
  const [spot, setSpot] = useState<Rect | null>(null)

  const step = STEPS[index]
  const isLast = index >= STEPS.length - 1

  const refreshSpot = useCallback(() => {
    setSpot(measureTarget(step.target))
  }, [step.target])

  useLayoutEffect(() => {
    if (!open) return
    refreshSpot()
  }, [open, index, refreshSpot])

  useEffect(() => {
    if (!open) return
    const onResize = () => refreshSpot()
    window.addEventListener('resize', onResize)
    window.addEventListener('scroll', onResize, true)
    return () => {
      window.removeEventListener('resize', onResize)
      window.removeEventListener('scroll', onResize, true)
    }
  }, [open, refreshSpot])

  useEffect(() => {
    if (!open) setIndex(0)
  }, [open])

  useEffect(() => {
    if (!open) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [open])

  const goNext = () => {
    if (isLast) onFinish()
    else setIndex(i => i + 1)
  }

  const goPrev = () => {
    if (index > 0) setIndex(i => i - 1)
  }

  if (!open || typeof document === 'undefined') return null

  return createPortal(
    <div className="wr-onboard" role="dialog" aria-modal="true" aria-label="工作区导览">
      <button type="button" className="wr-onboard-mask" aria-label="关闭导览" onClick={onClose} />

      {spot && (
        <div
          className="wr-onboard-spot"
          style={{
            top: spot.top,
            left: spot.left,
            width: spot.width,
            height: spot.height,
          }}
        />
      )}

      <div
        key={index}
        className={`wr-onboard-panel${step.placement === 'center' ? ' is-center' : ''}`}
        style={panelStyle(step.placement, spot)}
      >
        <div className="wr-onboard-panel-inner">
          <p className="wr-onboard-kicker">{step.kicker}</p>
          <h2 className="wr-onboard-title">{step.title}</h2>
          <p className="wr-onboard-lead">{step.lead}</p>

          <div className="wr-onboard-foot">
            <div className="wr-onboard-dots" aria-hidden>
              {STEPS.map((_, i) => (
                <span key={i} className={`wr-onboard-dot${i === index ? ' is-active' : ''}${i < index ? ' is-done' : ''}`} />
              ))}
            </div>
            <div className="wr-onboard-actions">
              <button type="button" className="wr-onboard-skip" onClick={onClose}>
                跳过
              </button>
              {index > 0 && (
                <button type="button" className="wr-onboard-back" onClick={goPrev}>
                  上一步
                </button>
              )}
              <button type="button" className="wr-onboard-next" onClick={goNext}>
                {step.cta || '继续'}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  )
}

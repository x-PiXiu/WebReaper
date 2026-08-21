import { Link } from 'react-router-dom'
import type { FlowStepDef } from '../../../config/product'
import type { ComposeTrack } from '../../../store/composeDraft'

type Props = {
  track: ComposeTrack
  steps: FlowStepDef[]
  stepIndex: number
  onStepChange: (index: number) => void
  children: React.ReactNode
  preview: React.ReactNode
  onBack: () => void
  onNext: () => void
  nextDisabled?: boolean
  nextHint?: string
  nextLoading?: boolean
  backLabel?: string
}

/**
 * 步骤式创作壳：顶栏步骤条 + 左工作区 / 右预览 + 底栏导航（对标大厂内容台）
 */
export function ComposeFlowShell({
  track,
  steps,
  stepIndex,
  onStepChange,
  children,
  preview,
  onBack,
  onNext,
  nextDisabled,
  nextHint,
  nextLoading,
  backLabel,
}: Props) {
  const step = steps[stepIndex]
  const trackLabel = track === 'video' ? '发视频' : '发图文'
  const otherPath = track === 'video' ? '/m/compose/graphic' : '/m/compose/video'
  const otherLabel = track === 'video' ? '发图文' : '发视频'

  return (
    <div className="cf-root">
      <header className="cf-top">
        <div className="cf-top-left">
          <Link to="/m/compose" className="cf-crumb">爆款获客</Link>
          <span className="cf-crumb-sep">/</span>
          <span className="cf-crumb-cur">{trackLabel}</span>
        </div>
        <nav className="cf-steps" aria-label="创作步骤">
          {steps.map((s, i) => {
            const state = i === stepIndex ? 'active' : i < stepIndex ? 'done' : 'todo'
            return (
              <button
                key={s.key}
                type="button"
                className={`cf-step cf-step-${state}`}
                onClick={() => onStepChange(i)}
              >
                {s.label}
              </button>
            )
          })}
        </nav>
        <div className="cf-top-right">
          <Link to={otherPath} className="cf-switch">{otherLabel}</Link>
        </div>
      </header>

      <div className="cf-meta">
        <span className="cf-step-index">STEP {stepIndex + 1}/{steps.length}</span>
      </div>

      <div className="cf-body">
        <main className="cf-main">
          <div className="cf-main-head">
            <h1>{step.title}</h1>
            <p>{step.tip}</p>
          </div>
          <div className="cf-workspace">{children}</div>
        </main>
        <aside className="cf-preview" aria-label="预览区">
          <div className="cf-preview-head">
            <strong>实时预览</strong>
            <span>{trackLabel}</span>
          </div>
          <div className="cf-preview-body">{preview}</div>
        </aside>
      </div>

      <footer className="cf-foot">
        <button type="button" className="cf-btn-ghost" onClick={onBack}>
          ← {backLabel || (stepIndex === 0 ? '选轨道' : steps[stepIndex - 1].label)}
        </button>
        <div className="cf-foot-right">
          {nextHint && nextDisabled ? <span className="cf-hint">{nextHint}</span> : null}
          <button
            type="button"
            className="cf-btn-next"
            disabled={nextDisabled || nextLoading}
            onClick={onNext}
          >
            {nextLoading ? '处理中…' : step.nextLabel} →
          </button>
        </div>
      </footer>
    </div>
  )
}

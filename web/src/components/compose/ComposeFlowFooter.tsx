type StepLite = { key: string; label: string }

type Props = {
  onBack: () => void
  backLabel: string
  onNext: () => void
  nextLabel: string
  nextDisabled?: boolean
  nextHint?: string
  nextLoading?: boolean
  hideNext?: boolean
  progressIndex?: number
  progressTotal?: number
  /** 可点击的步骤名（已到达的可跳转） */
  steps?: StepLite[]
  maxReachableStep?: number
  onStepChange?: (index: number) => void
}

/** 创作台 / 向导共用底栏：进度可点 + 大号下一步 */
export function ComposeFlowFooter({
  onBack,
  backLabel,
  onNext,
  nextLabel,
  nextDisabled,
  nextHint,
  nextLoading,
  hideNext,
  progressIndex,
  progressTotal,
  steps,
  maxReachableStep = 0,
  onStepChange,
}: Props) {
  const showProgress = typeof progressIndex === 'number' && typeof progressTotal === 'number' && progressTotal > 0

  return (
    <footer className="cf-foot cf-foot-inset cf-foot--studio">
      <div className="cf-foot-left">
        <button type="button" className="cf-btn-ghost" onClick={onBack}>
          ← {backLabel}
        </button>
        {showProgress && (
          <div className="cf-progress" aria-label={`创作进度 ${progressIndex! + 1}/${progressTotal}`}>
            <span className="cf-progress-label">创作进度</span>
            <strong className="cf-progress-count">{progressIndex! + 1}/{progressTotal}</strong>
            <div className="cf-progress-steps" role="list">
              {(steps && steps.length === progressTotal
                ? steps
                : Array.from({ length: progressTotal! }, (_, i) => ({ key: String(i), label: String(i + 1) }))
              ).map((s, i) => {
                const reachable = i <= maxReachableStep
                const active = i === progressIndex
                const done = i < progressIndex!
                return (
                  <button
                    key={s.key}
                    type="button"
                    role="listitem"
                    className={`cf-progress-chip${active ? ' is-active' : ''}${done ? ' is-done' : ''}`}
                    disabled={!reachable}
                    onClick={() => reachable && onStepChange?.(i)}
                    title={s.label}
                  >
                    <i aria-hidden />
                    <span>{s.label}</span>
                  </button>
                )
              })}
            </div>
          </div>
        )}
      </div>
      <div className="cf-foot-right">
        {nextHint && nextDisabled && !hideNext ? <span className="cf-hint">{nextHint}</span> : null}
        {!hideNext && (
          <button
            type="button"
            className="cf-btn-next"
            disabled={nextDisabled || nextLoading}
            onClick={onNext}
          >
            {nextLoading ? '处理中…' : nextLabel}
          </button>
        )}
      </div>
    </footer>
  )
}

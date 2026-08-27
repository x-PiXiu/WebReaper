type Props = {
  onBack: () => void
  backLabel: string
  onNext: () => void
  nextLabel: string
  nextDisabled?: boolean
  nextHint?: string
  nextLoading?: boolean
}

/** 创作台 / 向导共用底栏（与 cf-foot-inset 样式配套） */
export function ComposeFlowFooter({
  onBack,
  backLabel,
  onNext,
  nextLabel,
  nextDisabled,
  nextHint,
  nextLoading,
}: Props) {
  return (
    <footer className="cf-foot cf-foot-inset">
      <button type="button" className="cf-btn-ghost" onClick={onBack}>
        ← {backLabel}
      </button>
      <div className="cf-foot-right">
        {nextHint && nextDisabled ? <span className="cf-hint">{nextHint}</span> : null}
        <button
          type="button"
          className="cf-btn-next"
          disabled={nextDisabled || nextLoading}
          onClick={onNext}
        >
          {nextLoading ? '处理中…' : nextLabel}
        </button>
      </div>
    </footer>
  )
}

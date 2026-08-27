import { Tooltip } from 'antd'
import { WIZARD_STEPS, type WizardStep } from './wizardModel'

type Props = {
  step: WizardStep
  onChange: (step: WizardStep) => void
}

/** 发布向导步骤条（窄屏横向滚动 + 短标签） */
export function PublishWizardSteps({ step, onChange }: Props) {
  return (
    <nav className="pub-wizard-steps" aria-label="发布步骤">
      {WIZARD_STEPS.map((s, i) => {
        const active = step === s.key
        const done = step > s.key
        return (
          <Tooltip key={s.key} title={s.title}>
            <button
              type="button"
              className={[
                'pub-wizard-step',
                active ? 'is-active' : '',
                done ? 'is-done' : '',
              ].filter(Boolean).join(' ')}
              onClick={() => onChange(s.key)}
              aria-current={active ? 'step' : undefined}
            >
              <span className="pub-wizard-step-num">{i + 1}</span>
              <span className="pub-wizard-step-label">{s.short}</span>
            </button>
          </Tooltip>
        )
      })}
    </nav>
  )
}

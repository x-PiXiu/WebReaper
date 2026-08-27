import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Modal } from 'antd'
import { MODAL_W } from '../../ui/modalFit'
import { EyeOutlined } from '@ant-design/icons'
import { ComposeFlowFooter } from '../compose/ComposeFlowFooter'
import type { WizardStepDef } from './types'

type Props = {
  breadcrumb?: string
  steps: WizardStepDef[]
  stepIndex: number
  maxReachableStep: number
  onStepChange: (index: number) => void
  children: React.ReactNode
  preview: React.ReactNode
  onBack: () => void
  onNext: () => void
  nextDisabled?: boolean
  nextHint?: string
  nextLoading?: boolean
  backLabel?: string
  nextLabel?: string
  alerts?: React.ReactNode
}

/**
 * 通用向导壳：顶步骤 + 左操作 / 右手机预览 + 底栏动作
 * 基于 ComposeFlowShell（cf-*）布局，口播向导专用扩展（wz-*）
 */
export function WizardShell({
  breadcrumb = '拍同款口播',
  steps,
  stepIndex,
  maxReachableStep,
  onStepChange,
  children,
  preview,
  onBack,
  onNext,
  nextDisabled,
  nextHint,
  nextLoading,
  backLabel,
  nextLabel,
  alerts,
}: Props) {
  const step = steps[stepIndex]
  const [previewOpen, setPreviewOpen] = useState(false)

  const previewHead = (
    <div className="cf-preview-head">
      <strong>成片预览</strong>
      <span className="cf-preview-step">{stepIndex + 1}/{steps.length}</span>
    </div>
  )

  return (
    <div className="wz-root cf-root cf-platform-douyin">
      <header className="cf-top">
        <div className="cf-top-left">
          <Link to="/m/compose" className="cf-crumb">工作台</Link>
          <span className="cf-crumb-sep">/</span>
          <span className="cf-crumb-cur">{breadcrumb}</span>
        </div>
        <nav className="cf-steps" aria-label="向导步骤">
          {steps.map((s, i) => {
            const state = i === stepIndex ? 'active' : i < stepIndex ? 'done' : 'todo'
            const reachable = i <= maxReachableStep
            return (
              <button
                key={s.key}
                type="button"
                className={`cf-step cf-step-${state}${!reachable ? ' cf-step-locked' : ''}`}
                disabled={!reachable}
                onClick={() => reachable && onStepChange(i)}
              >
                <span className="cf-step-num">{i + 1}</span>
                <span className="cf-step-label">{s.label}</span>
              </button>
            )
          })}
        </nav>
      </header>

      <div className="cf-body">
        <main className="cf-main">
          <div className="cf-main-head">
            <div className="cf-main-head-row">
              <h1>{step.title}</h1>
              <button
                type="button"
                className="cf-preview-toggle"
                onClick={() => setPreviewOpen(true)}
              >
                <EyeOutlined /> 预览
              </button>
            </div>
            <p className="cf-main-tip">{step.tip}</p>
          </div>

          {alerts}

          <div className="cf-workspace wz-workspace">
            <div key={step.key} className="wz-panel ip-wizard-panel">
              {children}
            </div>
          </div>

          <ComposeFlowFooter
            onBack={onBack}
            backLabel={backLabel || (stepIndex === 0 ? '返回工作台' : steps[stepIndex - 1].label)}
            onNext={onNext}
            nextLabel={nextLabel || step.nextLabel}
            nextDisabled={nextDisabled}
            nextHint={nextHint}
            nextLoading={nextLoading}
          />
        </main>

        <aside className="cf-preview" aria-label="预览区">
          {previewHead}
          <div className="cf-preview-body cf-preview-sticky">{preview}</div>
        </aside>
      </div>

      <Modal
        title="成片预览"
        open={previewOpen}
        onCancel={() => setPreviewOpen(false)}
        width={MODAL_W.md}
        footer={null}
        destroyOnHidden
        className="cf-preview-modal wr-modal-preview"
      >
        <div className="cf-preview-drawer-inner cf-platform-douyin">
          {previewHead}
          <div className="cf-preview-body">{preview}</div>
        </div>
      </Modal>
    </div>
  )
}

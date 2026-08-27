import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Modal, Segmented } from 'antd'
import { MODAL_W } from '../../../ui/modalFit'
import { EyeOutlined } from '@ant-design/icons'
import { ComposeFlowFooter } from '../../../components/compose/ComposeFlowFooter'
import type { FlowStepDef } from '../../../config/product'
import type { ComposeTrack } from '../../../store/composeDraft'

type PreviewPlatform = 'douyin' | 'xiaohongshu'

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
 * 步骤创作壳：顶栏步骤 + 左编辑 / 右预览 + 底栏动作（底栏与编辑区一体）
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
  const otherPath = track === 'graphic' ? '/m/compose/lipsync' : '/m/compose/graphic'
  const otherLabel = track === 'graphic' ? '拍口播' : '发图文'
  const [previewOpen, setPreviewOpen] = useState(false)
  const [previewPlatform, setPreviewPlatform] = useState<PreviewPlatform>(
    track === 'graphic' ? 'xiaohongshu' : 'douyin',
  )

  const previewHead = (
    <div className="cf-preview-head">
      <strong>预览</strong>
      <div className="cf-preview-head-right">
        <Segmented
          size="small"
          value={previewPlatform}
          onChange={(v) => setPreviewPlatform(v as PreviewPlatform)}
          options={[
            { label: '抖音', value: 'douyin' },
            { label: '小红书', value: 'xiaohongshu' },
          ]}
        />
        <span className="cf-preview-step">{stepIndex + 1}/{steps.length}</span>
      </div>
    </div>
  )

  return (
    <div className={`cf-root cf-platform-${previewPlatform}`}>
      <header className="cf-top">
        <div className="cf-top-left">
          <Link to="/m/compose" className="cf-crumb">工作台</Link>
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
                <span className="cf-step-num">{i + 1}</span>
                <span className="cf-step-label">{s.label}</span>
              </button>
            )
          })}
        </nav>
        <div className="cf-top-right">
          <Link to={otherPath} className="cf-switch">改做{otherLabel}</Link>
        </div>
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
          <div className="cf-workspace">{children}</div>
          <ComposeFlowFooter
            onBack={onBack}
            backLabel={backLabel || (stepIndex === 0 ? '返回工作台' : steps[stepIndex - 1].label)}
            onNext={onNext}
            nextLabel={step.nextLabel}
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
        title="发布预览"
        open={previewOpen}
        onCancel={() => setPreviewOpen(false)}
        width={MODAL_W.md}
        footer={null}
        destroyOnHidden
        className="cf-preview-modal wr-modal-preview"
      >
        <div className={`cf-preview-drawer-inner cf-platform-${previewPlatform}`}>
          {previewHead}
          <div className="cf-preview-body">{preview}</div>
        </div>
      </Modal>
    </div>
  )
}

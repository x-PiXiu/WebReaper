import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Drawer, Segmented } from 'antd'
import { EyeOutlined } from '@ant-design/icons'
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
  const otherPath = track === 'video' ? '/m/compose/graphic' : '/m/compose/video'
  const otherLabel = track === 'video' ? '发图文' : '发视频'
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
          <Link to="/m/compose" className="cf-crumb">创作台</Link>
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
          <footer className="cf-foot cf-foot-inset">
            <button type="button" className="cf-btn-ghost" onClick={onBack}>
              ← {backLabel || (stepIndex === 0 ? '返回创作台' : steps[stepIndex - 1].label)}
            </button>
            <div className="cf-foot-right">
              {nextHint && nextDisabled ? <span className="cf-hint">{nextHint}</span> : null}
              <button
                type="button"
                className="cf-btn-next"
                disabled={nextDisabled || nextLoading}
                onClick={onNext}
              >
                {nextLoading ? '处理中…' : step.nextLabel}
              </button>
            </div>
          </footer>
        </main>

        <aside className="cf-preview" aria-label="预览区">
          {previewHead}
          <div className="cf-preview-body cf-preview-sticky">{preview}</div>
        </aside>
      </div>

      <Drawer
        title="发布预览"
        placement="bottom"
        height="72vh"
        open={previewOpen}
        onClose={() => setPreviewOpen(false)}
        className="cf-preview-drawer"
        destroyOnClose
      >
        <div className={`cf-preview-drawer-inner cf-platform-${previewPlatform}`}>
          {previewHead}
          <div className="cf-preview-body">{preview}</div>
        </div>
      </Drawer>
    </div>
  )
}

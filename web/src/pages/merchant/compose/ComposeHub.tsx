import { useNavigate } from 'react-router-dom'
import { Button, Tooltip } from 'antd'
import {
  VideoCameraOutlined, RightOutlined, ArrowUpOutlined,
} from '@ant-design/icons'
import { useComposeDraft } from '../../../store/composeDraft'
import { useComposeTaskPoll } from '../../../hooks/useComposeTaskPoll'
import { NoBrandGuide } from '../../../components/NoBrandGuide'
import { composeProgressLabel, composeResumeHint, hasComposeDraft, composeResumePath, composeResumeLabel } from '../../../utils/composeProgress'
import MerchantOnboardingTour from '../../../components/onboarding/MerchantOnboardingTour'
import { useMerchantOnboarding } from '../../../hooks/useMerchantOnboarding'
import { useGenerationTypes } from '../../../hooks/useGenerationTypes'
import { CREATIVE_CDN } from '../../../config/creativeCdn'

const HERO_TAGS = [
  { label: '写文案', path: '/m/compose/copy' },
  { label: '配音 / 数字人', path: '/m/compose/avatar' },
  { label: '口播成片', path: '/m/compose/lipsync' },
] as const

type PipeTone = 'violet' | 'sky' | 'amber' | 'mint'

const PIPELINE: Array<{
  key: string
  index: string
  title: string
  desc: string
  path: string
  tone: PipeTone
  image: string
  overlay?: 'generating' | 'avatar-mic' | 'phone' | 'share'
  requires?: readonly string[]
}> = [
  {
    key: 'copy',
    index: '01',
    title: '文案创作',
    desc: '对标爆款，快速成稿',
    path: '/m/compose/copy',
    tone: 'violet',
    image: CREATIVE_CDN.pipeline.copy,
    overlay: 'generating',
  },
  {
    key: 'voice',
    index: '02',
    title: '配音 & 数字人',
    desc: '音色克隆与形象一站配齐',
    path: '/m/compose/avatar',
    tone: 'sky',
    image: CREATIVE_CDN.pipeline.voice,
    overlay: 'avatar-mic',
  },
  {
    key: 'lipsync',
    index: '03',
    title: '一键成片',
    desc: '口播对口型快速出片',
    path: '/m/compose/lipsync',
    tone: 'amber',
    image: CREATIVE_CDN.pipeline.film,
    overlay: 'phone',
    requires: ['tts', 'reference2video|lip_sync'],
  },
  {
    key: 'publish',
    index: '04',
    title: '自动发布',
    desc: '多平台一键分发获客',
    path: '/m/distribution',
    tone: 'mint',
    image: CREATIVE_CDN.pipeline.publish,
    overlay: 'share',
  },
]

function progressSteps(label: string) {
  return label.split(' · ').map(s => s.trim()).filter(Boolean)
}

function PipeCardVisual({
  image,
  overlay,
}: {
  image: string
  overlay?: (typeof PIPELINE)[number]['overlay']
}) {
  if (overlay === 'generating') {
    return (
      <div className="ch-embed ch-embed--copy">
        <div className="ch-device ch-device--doc">
          <div className="ch-device-screen ch-device-screen--doc">
            <img src={image} alt="" loading="lazy" decoding="async" onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
            <div className="ch-doc-lines" aria-hidden>
              <span /><span /><span /><span /><span />
            </div>
          </div>
        </div>
      </div>
    )
  }
  if (overlay === 'avatar-mic') {
    return (
      <div className="ch-embed ch-embed--duo">
        <div className="ch-device ch-device--back">
          <div className="ch-device-screen ch-device-screen--mic">
            <img src={CREATIVE_CDN.pipeline.mic} alt="" loading="lazy" decoding="async" onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
            <div className="ch-mic-core" aria-hidden />
            <div className="ch-wave" aria-hidden><i /><i /><i /><i /><i /></div>
          </div>
        </div>
        <div className="ch-device ch-device--front">
          <div className="ch-device-screen">
            <img src={image} alt="" loading="lazy" decoding="async" onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
            <div className="ch-face-fallback" aria-hidden />
          </div>
        </div>
      </div>
    )
  }
  if (overlay === 'phone') {
    return (
      <div className="ch-embed ch-embed--film">
        <div className="ch-device ch-device--tilt">
          <span className="ch-device-notch" />
          <div className="ch-device-screen">
            <img src={image} alt="" loading="lazy" decoding="async" onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
            <div className="ch-film-fallback" aria-hidden />
            <div className="ch-timeline" aria-hidden>
              <span className="ch-timeline-track">
                <em style={{ left: '10%', width: '20%' }} />
                <em style={{ left: '38%', width: '26%' }} />
                <em style={{ left: '72%', width: '16%' }} />
              </span>
            </div>
          </div>
        </div>
      </div>
    )
  }
  return (
    <div className="ch-embed ch-embed--share">
      <div className="ch-device ch-device--tilt-opp">
        <span className="ch-device-notch" />
        <div className="ch-device-screen">
          <img src={image} alt="" loading="lazy" decoding="async" onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none' }} />
          <div className="ch-share-fallback" aria-hidden />
          <div className="ch-share-btn" aria-hidden>
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none">
              <circle cx="18" cy="5" r="2.2" fill="currentColor" />
              <circle cx="6" cy="12" r="2.2" fill="currentColor" />
              <circle cx="18" cy="19" r="2.2" fill="currentColor" />
              <path d="M8 12.8l8 5.2M8 11.2l8-5.2" stroke="currentColor" strokeWidth="1.8" />
            </svg>
          </div>
        </div>
      </div>
    </div>
  )
}

/**
 * 创意首页：对齐参考卡片式布局——Hero + 四色序号白卡片 + CDN 分层视觉。
 */
export default function ComposeHub() {
  const navigate = useNavigate()
  const draft = useComposeDraft()
  useComposeTaskPoll()
  const { open: tourOpen, finish: finishTour, skip: skipTour } = useMerchantOnboarding(true)
  const hasDraft = hasComposeDraft(draft)
  const resumePath = composeResumePath(draft)
  const resumeLabel = composeResumeLabel(draft)
  const resumeHint = hasDraft ? composeResumeHint(draft) : ''
  const progressLabel = hasDraft && (draft.track === 'graphic' || draft.track === 'video')
    ? composeProgressLabel(draft, draft.track)
    : ''
  const draftTitle = draft.selectedTitle || '继续编辑草稿'

  const { isEnabled, isLoading: typesLoading } = useGenerationTypes()

  const pipeDisabled = (step: (typeof PIPELINE)[number]) => {
    if (typesLoading || !step.requires) return false
    for (const req of step.requires) {
      if (req.includes('|')) {
        if (!req.split('|').some((s) => isEnabled(s))) return true
      } else if (!isEnabled(req)) {
        return true
      }
    }
    return false
  }

  return (
    <div className="wr-page-content ch-hub ch-creative ch-creative--pro ch-studio ip-page">
      <NoBrandGuide style={{ marginBottom: 16 }} />
      <div className="ch-atmosphere" aria-hidden />
      <MerchantOnboardingTour open={tourOpen} onClose={skipTour} onFinish={finishTour} />

      <section className="ch-hero">
        <div className="ch-hero-copy">
          <p className="ch-hero-kicker">
            <span className="ch-hero-mark" aria-hidden />
            创作中心
          </p>
          <h1 className="ch-hero-title">从文案到成片，一气呵成。</h1>
          <p className="ch-hero-lead">
            分身准备 → 口播创作 → 作品插画面 → 多平台发布，同一条旅程。
          </p>
        </div>
        <div className="ch-hero-cta">
          <Button
            type="primary"
            size="large"
            className="ch-hero-btn"
            icon={<VideoCameraOutlined />}
            onClick={() => navigate('/m/compose/lipsync')}
          >
            开始创作
            <RightOutlined />
          </Button>
          <div className="ch-hero-tags">
            {HERO_TAGS.map((t) => (
              <button key={t.path} type="button" className="ch-hero-tag" onClick={() => navigate(t.path)}>
                {t.label}
              </button>
            ))}
          </div>
        </div>
      </section>

      <section className="ch-pipeline" aria-label="创作流水线">
        {PIPELINE.map((step) => {
          const disabled = pipeDisabled(step)
          const GoIcon = step.key === 'copy' ? ArrowUpOutlined : RightOutlined
          const card = (
            <button
              key={step.key}
              type="button"
              className={`ch-pipe-card ch-pipe-card--${step.tone}${disabled ? ' is-disabled' : ''}`}
              onClick={() => { if (!disabled) navigate(step.path) }}
              aria-disabled={disabled}
            >
              <div className="ch-pipe-head">
                <div className="ch-pipe-badge-row">
                  <span className="ch-pipe-badge">{step.index}</span>
                  <span className="ch-pipe-rule" aria-hidden />
                </div>
                <h2 className="ch-pipe-title">{step.title}</h2>
                <p className="ch-pipe-desc">{disabled ? '当前环境未启用相关能力' : step.desc}</p>
              </div>
              <PipeCardVisual image={step.image} overlay={step.overlay} />
              <span className={`ch-pipe-go${step.key === 'copy' ? ' is-diag' : ''}`} aria-hidden>
                <GoIcon />
              </span>
            </button>
          )
          return disabled ? (
            <Tooltip key={step.key} title="相关生成能力未在后台启用">
              <div className="ch-pipe-wrap">{card}</div>
            </Tooltip>
          ) : card
        })}
      </section>

      {hasDraft && (
        <button type="button" className="ch-draft-bar" onClick={() => navigate(resumePath)}>
          <div className="ch-draft-bar-main">
            <span className="ch-draft-bar-label">继续{resumeLabel}</span>
            <strong className="ch-draft-bar-title">{draftTitle}</strong>
            {resumeHint && <span className="ch-draft-bar-hint">{resumeHint}</span>}
            {progressLabel && (
              <div className="ch-draft-bar-steps">
                {progressSteps(progressLabel).map(step => (
                  <span
                    key={step}
                    className={[
                      'ch-draft-step',
                      step.includes('✓') ? 'is-done' : '',
                      step.includes('中') ? 'is-active' : '',
                    ].filter(Boolean).join(' ')}
                  >
                    {step.replace(' ✓', '')}
                  </span>
                ))}
              </div>
            )}
          </div>
          <span className="ch-draft-bar-go">
            继续 <RightOutlined />
          </span>
          <button
            type="button"
            className="ch-draft-bar-clear"
            title="清空当前草稿"
            onClick={(e) => {
              e.stopPropagation()
              if (window.confirm('清空当前创作草稿？')) draft.reset()
            }}
          >
            清空
          </button>
        </button>
      )}

    </div>
  )
}

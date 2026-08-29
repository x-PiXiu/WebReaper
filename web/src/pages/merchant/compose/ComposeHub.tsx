import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Button, Empty, Spin, Tooltip } from 'antd'
import { usePublishableWorks } from '../../../hooks/usePublishableWorks'
import {
  VideoCameraOutlined, ThunderboltOutlined, FileTextOutlined,
  RightOutlined, DownOutlined, UpOutlined, ArrowUpOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useComposeDraft } from '../../../store/composeDraft'
import { useComposeTaskPoll } from '../../../hooks/useComposeTaskPoll'
import { composeProgressLabel, composeResumeHint, hasComposeDraft, composeResumePath, composeResumeLabel } from '../../../utils/composeProgress'
import { GenerationFailedBar } from '../../../components/compose/GenerationFailedBar'
import MerchantOnboardingTour from '../../../components/onboarding/MerchantOnboardingTour'
import { useMerchantOnboarding } from '../../../hooks/useMerchantOnboarding'
import { useGenerationTypes } from '../../../hooks/useGenerationTypes'
import { CREATIVE_CDN } from '../../../config/creativeCdn'
import { retryFailureMessage } from '../../../components/RetryHint'

const WORK_STATUS: Record<string, string> = {
  draft: '草稿',
  generating: '生成中',
  ready: '待发布',
  published: '已发布',
}

const HERO_TAGS = [
  { label: '1分钟定剧本', path: '/m/compose/copy' },
  { label: '一键配音', path: '/m/compose/voice' },
  { label: '自动剪辑出片', path: '/m/compose/lipsync' },
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
    title: '搞定文案内容',
    desc: '轻松创作优质文案',
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

const CREATE_MODES = [
  {
    key: 'lipsync',
    icon: VideoCameraOutlined,
    title: '拍同款口播',
    desc: '推荐 · 文案到成片五步向导',
    path: '/m/compose/lipsync',
    featured: true,
    tone: 'teal',
    requires: ['tts', 'reference2video|lip_sync'] as const,
  },
  {
    key: 'quick',
    icon: ThunderboltOutlined,
    title: '快速生成',
    desc: '选模板，传素材即可',
    path: '/m/compose/quick',
    tone: 'violet',
  },
  {
    key: 'graphic',
    icon: FileTextOutlined,
    title: '发图文',
    desc: '种草文案与配图封面',
    path: '/m/compose/graphic',
    tone: 'amber',
    requires: ['text2image'] as const,
  },
] as const

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
        <div className="ch-toast" aria-hidden>
          <span className="ch-toast-dots"><i /><i /><i /></span>
          生成中…
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
            <span className="ch-ai-tag">AI DRIVING</span>
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
  const [moreOpen, setMoreOpen] = useState(false)
  const hasDraft = hasComposeDraft(draft)
  const resumePath = composeResumePath(draft)
  const resumeLabel = composeResumeLabel(draft)
  const resumeHint = hasDraft ? composeResumeHint(draft) : ''
  const progressLabel = hasDraft && (draft.track === 'graphic' || draft.track === 'video')
    ? composeProgressLabel(draft, draft.track)
    : ''
  const draftTitle = draft.selectedTitle || '继续编辑草稿'

  const { works = [], isLoading: worksLoading } = usePublishableWorks()
  const { data: failedTasks = [] } = useQuery({
    queryKey: ['generation-tasks'],
    queryFn: () => businessApi.listGenerationTasks()
      .then((r) => r.tasks.filter((t) => t.state === 'failed').slice(0, 5))
      .catch(() => []),
    staleTime: 30_000,
  })
  const pending = works
    .filter(w => w.status === 'draft' || w.status === 'ready' || w.status === 'generating')
    .slice(0, 5)

  const { isEnabled, isLoading: typesLoading } = useGenerationTypes()

  const modeDisabled = (mode: typeof CREATE_MODES[number]) => {
    if (typesLoading || !('requires' in mode) || !mode.requires) return false
    for (const req of mode.requires) {
      if (req.includes('|')) {
        const any = req.split('|').some((s) => isEnabled(s))
        if (!any) return true
      } else if (!isEnabled(req)) {
        return true
      }
    }
    return false
  }

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

  const modeDisabledReason = (mode: typeof CREATE_MODES[number]) => {
    if (mode.key === 'lipsync') {
      return '口播需要后台启用「语音合成」，以及「参考生视频」或「对口型」'
    }
    if (mode.key === 'graphic') {
      return '发图文配图需要后台启用「文生图」'
    }
    return '相关生成能力未在后台启用'
  }

  const openMode = (mode: typeof CREATE_MODES[number]) => {
    if (modeDisabled(mode)) return
    if (mode.key === 'graphic') {
      draft.setTrack('graphic')
      navigate('/m/compose/graphic')
    } else {
      navigate(mode.path)
    }
  }

  const openWork = (w: (typeof pending)[0]) => {
    if (w.status === 'ready' || w.status === 'published') {
      const q = new URLSearchParams()
      if (w.content_id) q.set('contentId', w.content_id)
      if (w.media_urls?.length) q.set('mediaUrls', w.media_urls.join(','))
      if (w.brand_id) q.set('brandId', w.brand_id)
      q.set('contentType', w.kind === 'image' ? 'image' : w.kind === 'article' ? 'article' : 'video')
      navigate(`/m/distribution?${q.toString()}`)
    } else {
      navigate(w.kind === 'article' || w.kind === 'image' ? '/m/compose/graphic' : '/m/compose/lipsync')
    }
  }

  return (
    <div className="wr-page-content ch-hub ch-creative ch-creative--pro ch-studio ip-page">
      <div className="ch-atmosphere" aria-hidden />
      <MerchantOnboardingTour open={tourOpen} onClose={skipTour} onFinish={finishTour} />

      <section className="ch-hero">
        <div className="ch-hero-copy">
          <p className="ch-hero-kicker">
            <span className="ch-hero-mark" aria-hidden />
            创作中心 / CREATIVE HUB
          </p>
          <h1 className="ch-hero-title">
            <span className="ch-hero-ai">AI</span>
            <span className="ch-hero-rest">，打造超级个体。</span>
          </h1>
          <p className="ch-hero-lead">
            顶尖 AI 矩阵协作，助您实现效率的<span className="ch-hero-em">降维打击</span>。
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
            立即开启创作
            <RightOutlined />
          </Button>
          <div className="ch-hero-tags">
            {HERO_TAGS.map((t) => (
              <button key={t.path} type="button" className="ch-hero-tag" onClick={() => navigate(t.path)}>
                {t.label}
              </button>
            ))}
          </div>
          <div className="ch-hero-social">
            <div className="ch-hero-avatars">
              {CREATIVE_CDN.avatars.map((src) => (
                <img key={src} src={src} alt="" width={32} height={32} loading="lazy" />
              ))}
            </div>
            <span><strong>10,000+</strong> 创作者</span>
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
        </button>
      )}

      <div className="ch-bottom-cta">
        <Button
          type="primary"
          size="large"
          className="ch-hero-btn ch-hero-btn--pill"
          icon={<VideoCameraOutlined />}
          onClick={() => navigate('/m/compose/lipsync')}
        >
          立即开启创作
          <RightOutlined />
        </Button>
        <p className="ch-bottom-cta-en">UNLEASH YOUR INFINITE CREATIVITY</p>
      </div>

      <div className="ch-more">
        <button
          type="button"
          className="ch-more-toggle"
          onClick={() => setMoreOpen((v) => !v)}
          aria-expanded={moreOpen}
        >
          <span>更多工具与作品</span>
          {moreOpen ? <UpOutlined /> : <DownOutlined />}
        </button>

        {moreOpen && (
          <div className="ch-more-body">
            <div className="ch-hub-divider">
              <span>选择创作方式</span>
            </div>
            <div className="ch-action-row" data-tour="create-modes">
              {CREATE_MODES.map(mode => {
                const Icon = mode.icon
                const featured = 'featured' in mode && mode.featured
                const disabled = modeDisabled(mode)
                const tile = (
                  <button
                    type="button"
                    className={`ch-action-tile ch-action-tile--${mode.tone}${featured ? ' is-featured' : ''}${disabled ? ' is-disabled' : ''}`}
                    onClick={() => openMode(mode)}
                    aria-disabled={disabled}
                  >
                    {featured && <span className="ch-action-badge">推荐</span>}
                    <span className="ch-action-icon" aria-hidden>
                      <Icon />
                    </span>
                    <span className="ch-action-title">{mode.title}</span>
                    <span className="ch-action-desc">
                      {disabled ? modeDisabledReason(mode) : mode.desc}
                    </span>
                  </button>
                )
                return disabled ? (
                  <Tooltip key={mode.key} title={modeDisabledReason(mode)}>
                    <div className="ch-action-tile-wrap">{tile}</div>
                  </Tooltip>
                ) : (
                  <div key={mode.key}>{tile}</div>
                )
              })}
            </div>

            {hasDraft && (
              <div className="ch-draft-foot">
                <span>草稿保存在本机</span>
                <button
                  type="button"
                  className="ch-hub-clear"
                  onClick={() => { if (window.confirm('清空当前创作草稿？')) draft.reset() }}
                >
                  清空草稿
                </button>
              </div>
            )}

            <div className="ch-hub-divider">
              <span>我的创作</span>
            </div>

            <div className="ch-panels ch-panels--hub" data-tour="recent-panel">
              <section className="ch-panel ch-panel-works">
                <div className="ch-panel-head">
                  <h3>待处理作品</h3>
                  <Link to="/m/works">查看全部</Link>
                </div>
                {worksLoading ? (
                  <div className="ch-panel-empty"><Spin /></div>
                ) : pending.length === 0 ? (
                  <div className="ch-panel-empty">
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无待发作品" />
                  </div>
                ) : (
                  <ul className="ch-list ch-list--clean">
                    {pending.map(w => (
                      <li key={w.id}>
                        <button
                          type="button"
                          className={`ch-list-row ch-list-row--clean ch-status-${w.status}`}
                          onClick={() => openWork(w)}
                        >
                          <span className="ch-list-status">{WORK_STATUS[w.status] || w.status}</span>
                          <span className="ch-list-title">{w.title || '未命名作品'}</span>
                          <span className="ch-list-meta">
                            {w.kind === 'video' ? '视频' : w.kind === 'article' || w.kind === 'image' ? '图文' : w.kind}
                          </span>
                          <span className="ch-list-action">
                            {w.status === 'ready' ? '去发布' : '打开'}
                            <RightOutlined />
                          </span>
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className="ch-panel ch-panel-inspire">
                <div className="ch-panel-head">
                  <h3>同赛道灵感</h3>
                  <Link to="/m/inspire">灵感广场</Link>
                </div>
                <div className="ch-panel-empty">
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="去灵感广场发现爆款视频">
                    <Link to="/m/inspire">
                      <Button type="primary">浏览灵感</Button>
                    </Link>
                  </Empty>
                </div>
              </section>

              {failedTasks.length > 0 && (
                <section className="ch-panel ch-panel-failed">
                  <div className="ch-panel-head">
                    <h3>最近生成失败</h3>
                    <Link to="/m/compose/tools?tab=media">任务中心</Link>
                  </div>
                  <ul className="ch-list ch-list--clean ch-failed-list">
                    {failedTasks.map((t) => (
                      <li key={t.id}>
                        <GenerationFailedBar
                          compact
                          message={`${t.sub_type} · ${t.model}：${retryFailureMessage(t, '生成失败')}`}
                          onRetry={() => navigate('/m/compose/tools?tab=media')}
                          retryLabel="去任务中心"
                        />
                      </li>
                    ))}
                  </ul>
                </section>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

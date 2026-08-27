import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Button, Empty, Spin, Tooltip } from 'antd'
import { usePublishableWorks } from '../../../hooks/usePublishableWorks'
import {
  VideoCameraOutlined, ThunderboltOutlined, FileTextOutlined,
  RightOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useComposeDraft } from '../../../store/composeDraft'
import { useComposeTaskPoll } from '../../../hooks/useComposeTaskPoll'
import { composeProgressLabel, composeResumeHint, hasComposeDraft, composeResumePath, composeResumeLabel } from '../../../utils/composeProgress'
import { PageHeader } from '../../../components/PageHeader'
import { GrowthStagesNav } from '../../../components/GrowthStagesNav'
import { retryFailureMessage } from '../../../components/RetryHint'
import { GenerationFailedBar } from '../../../components/compose/GenerationFailedBar'
import MerchantOnboardingTour from '../../../components/onboarding/MerchantOnboardingTour'
import { useMerchantOnboarding } from '../../../hooks/useMerchantOnboarding'
import { useGenerationTypes } from '../../../hooks/useGenerationTypes'

const WORK_STATUS: Record<string, string> = {
  draft: '草稿',
  generating: '生成中',
  ready: '待发布',
  published: '已发布',
}

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

/**
 * 工作台入口：选轨 + 续写草稿 + 待发作品 + 灵感快入口
 */
export default function ComposeHub() {
  const navigate = useNavigate()
  const draft = useComposeDraft()
  useComposeTaskPoll()
  const { open: tourOpen, finish: finishTour, skip: skipTour, replay: replayTour } = useMerchantOnboarding(true)
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
    <div className="wr-page-content ch-hub ip-page">
      <MerchantOnboardingTour open={tourOpen} onClose={skipTour} onFinish={finishTour} />
      <PageHeader
        kicker="Workbench"
        title="工作台"
        lead="选方式开写，续草稿或从灵感复刻。"
        className="ch-hub-header"
        actions={
          <div className="ch-hub-quick">
            <button type="button" className="ch-hub-quick-link" onClick={replayTour}>导览</button>
            <Link to="/m/inspire">灵感广场</Link>
            <Link to="/m/compose/benchmark">粘贴对标</Link>
            <Link to="/m/works">全部作品</Link>
          </div>
        }
      />

      <div data-tour="growth-stages">
        <GrowthStagesNav current="create" compact className="ch-growth ch-growth--hub" />
      </div>

      <div className="ch-hub-primary">
        {hasDraft && (
          <button type="button" className="ch-draft-bar" onClick={() => navigate(resumePath)}>
            <div className="ch-draft-bar-main">
              <span className="ch-draft-bar-label">继续{resumeLabel}</span>
              <strong className="ch-draft-bar-title">{draftTitle}</strong>
              {resumeHint && (
                <span className="ch-draft-bar-hint">{resumeHint}</span>
              )}
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
      </div>

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
  )
}

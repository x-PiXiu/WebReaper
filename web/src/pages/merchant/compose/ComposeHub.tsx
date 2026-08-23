import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Dropdown, Empty, Spin } from 'antd'
import {
  VideoCameraOutlined, ThunderboltOutlined, FileTextOutlined,
  RightOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useBrandContext } from '../../../hooks/useBrands'
import { useComposeDraft } from '../../../store/composeDraft'
import { useComposeTaskPoll } from '../../../hooks/useComposeTaskPoll'
import { composeProgressLabel, hasComposeDraft, composeResumePath } from '../../../utils/composeProgress'
import { PageHeader } from '../../../components/PageHeader'
import { GrowthStagesNav } from '../../../components/GrowthStagesNav'
import { PlatformBadge } from '../../../components/PlatformBadge'
import type { HotVideo } from '../../../types/api'

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
    desc: '文案 · 出镜 · 配音 · 成片',
    path: '/m/compose/lipsync',
    featured: true,
    tone: 'teal',
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
  },
] as const

function progressSteps(label: string) {
  return label.split(' · ').map(s => s.trim()).filter(Boolean)
}

/**
 * 创作台入口：选轨 + 续写草稿 + 待发作品 + 灵感快入口
 */
export default function ComposeHub() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  useComposeTaskPoll()
  const hasDraft = hasComposeDraft(draft)
  const resumePath = composeResumePath(draft)
  const resumeLabel = draft.track === 'graphic' ? '发图文' : '拍口播'
  const progressLabel = hasDraft ? composeProgressLabel(draft, draft.track) : ''
  const draftTitle = draft.selectedTitle || '继续编辑草稿'

  const { data: works = [], isLoading: worksLoading } = useQuery({
    queryKey: ['merchant-works'],
    queryFn: () => businessApi.listWorks().catch(() => []),
  })
  const pending = works
    .filter(w => w.status === 'draft' || w.status === 'ready' || w.status === 'generating')
    .slice(0, 5)

  const { data: hotData, isLoading: hotLoading } = useQuery({
    queryKey: ['hot-videos', brandId],
    queryFn: () => businessApi.listHotVideos(brandId!),
    enabled: !!brandId,
    staleTime: 24 * 3600_000,
  })
  const hotList = (hotData?.videos || []).slice(0, 5)

  const openMode = (mode: typeof CREATE_MODES[number]) => {
    if (mode.key === 'graphic') {
      draft.setTrack('graphic')
      navigate('/m/compose/graphic')
    } else {
      navigate(mode.path)
    }
  }

  const remake = (v: HotVideo, mode: 'video' | 'graphic') => {
    const topic = v.topic || (mode === 'graphic' ? `写一篇同款：${v.title}` : `拍一条同款：${v.title}`)
    draft.setTrack(mode)
    draft.patch({
      brandId,
      sourceUrl: v.url || undefined,
      refTitle: v.title,
      hotPoint: v.hot_point || undefined,
      script: topic,
      transcript: [
        v.hot_point ? `【为什么火】${v.hot_point}` : '',
        `【选题】${topic}`,
        v.url ? `【来源】${v.url}` : '',
      ].filter(Boolean).join('\n'),
      selectedTitle: v.title.slice(0, 40),
    })
    navigate(mode === 'graphic' ? '/m/compose/graphic' : '/m/compose/lipsync')
  }

  const openWork = (w: (typeof pending)[0]) => {
    if (w.status === 'ready' || w.status === 'published') {
      const q = new URLSearchParams()
      if (w.content_id) q.set('contentId', w.content_id)
      if (w.media_urls?.length) q.set('mediaUrls', w.media_urls.join(','))
      if (w.brand_id) q.set('brandId', w.brand_id)
      q.set('contentType', w.kind === 'article' || w.kind === 'image' ? 'article' : 'video')
      navigate(`/m/distribution?${q.toString()}`)
    } else {
      navigate(w.kind === 'article' || w.kind === 'image' ? '/m/compose/graphic' : '/m/compose/lipsync')
    }
  }

  return (
    <div className="wr-page-content ch-hub ip-page">
      <PageHeader
        kicker="Create"
        title="创作台"
        lead="选方式开写，续草稿或从灵感复刻。"
        className="ch-hub-header"
        actions={
          <div className="ch-hub-quick">
            <Link to="/m/inspire">灵感广场</Link>
            <Link to="/m/compose/benchmark">粘贴对标</Link>
            <Link to="/m/works">全部作品</Link>
          </div>
        }
      />

      <GrowthStagesNav current="create" compact className="ch-growth ch-growth--hub" />

      <div className="ch-hub-primary">
        {hasDraft && (
          <button type="button" className="ch-draft-bar" onClick={() => navigate(resumePath)}>
            <div className="ch-draft-bar-main">
              <span className="ch-draft-bar-label">继续{resumeLabel}</span>
              <strong className="ch-draft-bar-title">{draftTitle}</strong>
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

        <div className="ch-action-row">
          {CREATE_MODES.map(mode => {
            const Icon = mode.icon
            const featured = 'featured' in mode && mode.featured
            return (
              <button
                key={mode.key}
                type="button"
                className={`ch-action-tile ch-action-tile--${mode.tone}${featured ? ' is-featured' : ''}`}
                onClick={() => openMode(mode)}
              >
                {featured && <span className="ch-action-badge">推荐</span>}
                <span className="ch-action-icon" aria-hidden>
                  <Icon />
                </span>
                <span className="ch-action-title">{mode.title}</span>
                <span className="ch-action-desc">{mode.desc}</span>
              </button>
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

      <div className="ch-panels ch-panels--hub">
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
          {!brandId ? (
            <div className="ch-panel-empty">
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="选好人设后显示爆款灵感">
                <Link to="/m/brands">去账号人设</Link>
              </Empty>
            </div>
          ) : hotLoading ? (
            <div className="ch-panel-empty"><Spin tip="拉取爆款…" /></div>
          ) : hotList.length === 0 ? (
            <div className="ch-panel-empty">
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无灵感，可去灵感广场刷新" />
            </div>
          ) : (
            <ul className="ch-list ch-list--clean">
              {hotList.map((v, i) => (
                <li key={(v.url || v.title) + i}>
                  <div className="ch-list-row ch-list-row--clean ch-list-row-static">
                    <div className="ch-list-main">
                      <span className="ch-list-title">{v.title}</span>
                      <span className="ch-list-meta">
                        <PlatformBadge platform={v.platform} size={14} className="ch-list-platform" />
                        {v.hot_point ? ` · ${v.hot_point.slice(0, 28)}${v.hot_point.length > 28 ? '…' : ''}` : ''}
                      </span>
                    </div>
                    <Dropdown
                      menu={{
                        items: [
                          { key: 'video', label: '复刻为视频', onClick: () => remake(v, 'video') },
                          { key: 'graphic', label: '复刻为图文', onClick: () => remake(v, 'graphic') },
                        ],
                      }}
                      trigger={['click']}
                    >
                      <button type="button" className="ch-list-remake">复刻</button>
                    </Dropdown>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  )
}

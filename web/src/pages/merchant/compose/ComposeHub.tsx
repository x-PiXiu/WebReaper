import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Dropdown, Empty, Spin } from 'antd'
import { businessApi } from '../../../api/business'
import { useBrandContext } from '../../../hooks/useBrands'
import { useComposeDraft } from '../../../store/composeDraft'
import { useComposeTaskPoll } from '../../../hooks/useComposeTaskPoll'
import { composeProgressLabel, getComposeBody, hasComposeDraft } from '../../../utils/composeProgress'
import { GROWTH_STAGES } from '../../../config/product'
import { PageHeader } from '../../../components/PageHeader'
import { PlatformBadge } from '../../../components/PlatformBadge'
import type { HotVideo } from '../../../types/api'

const WORK_STATUS: Record<string, string> = {
  draft: '草稿',
  generating: '生成中',
  ready: '待发布',
  published: '已发布',
}

/**
 * 创作台入口：选轨 + 续写草稿 + 待发作品 + 灵感快入口
 */
export default function ComposeHub() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  useComposeTaskPoll()
  const body = getComposeBody(draft)
  const hasDraft = hasComposeDraft(draft)
  const resumePath = draft.track === 'graphic' ? '/m/compose/graphic' : '/m/compose/video'
  const resumeLabel = draft.track === 'graphic' ? '发图文' : '发视频'
  const progressLabel = hasDraft ? composeProgressLabel(draft, draft.track) : ''
  const snippet = body.replace(/\s+/g, ' ').slice(0, 80)

  const { data: works = [], isLoading: worksLoading } = useQuery({
    queryKey: ['merchant-works'],
    queryFn: () => businessApi.listWorks().catch(() => []),
  })
  const pending = works.filter((w) => w.status === 'draft' || w.status === 'ready' || w.status === 'generating').slice(0, 6)

  const { data: hotData, isLoading: hotLoading } = useQuery({
    queryKey: ['hot-videos', brandId],
    queryFn: () => businessApi.listHotVideos(brandId!),
    enabled: !!brandId,
    staleTime: 24 * 3600_000,
  })
  const hotList = (hotData?.videos || []).slice(0, 5)

  const go = (track: 'video' | 'graphic') => {
    draft.setTrack(track)
    navigate(track === 'video' ? '/m/compose/video' : '/m/compose/graphic')
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
    navigate(mode === 'graphic' ? '/m/compose/graphic' : '/m/compose/video')
  }

  return (
    <div className="wr-page-content ch-hub ip-page">
      <PageHeader
        kicker="Create"
        title="创作台"
        lead="选形态开写；有草稿就续写，有灵感就复刻。"
        actions={
          <div className="ch-hub-quick">
            <Link to="/m/inspire">灵感广场</Link>
            <Link to="/m/compose/benchmark">粘贴对标</Link>
            <Link to="/m/works">全部作品</Link>
          </div>
        }
      />

      <nav className="ch-growth" aria-label="获客闭环">
        {GROWTH_STAGES.map((s) => (
          <Link
            key={s.key}
            to={s.path}
            className={`ch-growth-item${s.key === 'create' ? ' is-current' : ''}`}
          >
            <span className="ch-growth-label">{s.label}</span>
            <span className="ch-growth-desc">{s.desc}</span>
          </Link>
        ))}
      </nav>

      {hasDraft && (
        <button type="button" className="ch-resume ch-resume-prominent" onClick={() => navigate(resumePath)}>
          <div className="ch-resume-main">
            <span className="ch-resume-kicker">继续创作 · {resumeLabel}</span>
            <strong>{draft.selectedTitle || '继续编辑草稿'}</strong>
            {progressLabel && <span className="ch-resume-progress">{progressLabel}</span>}
            <span className="ch-resume-snip">{snippet}{snippet.length >= 80 ? '…' : ''}</span>
          </div>
          <span className="ch-resume-go">继续 →</span>
        </button>
      )}

      <div className="ch-modes">
        <button type="button" className="ch-mode ch-mode-video" onClick={() => go('video')}>
          <div className="ch-mode-preview" aria-hidden>
            <div className="ch-mode-phone">
              <span>{draft.track === 'video' && draft.selectedTitle ? draft.selectedTitle : '竖屏口播成片'}</span>
            </div>
          </div>
          <div className="ch-mode-copy">
            <h2>发视频</h2>
            <p>口播稿 → 配音 / 数字人 → 成片发布</p>
            <span className="ch-mode-cta">开始制作</span>
          </div>
        </button>

        <button type="button" className="ch-mode ch-mode-graphic" onClick={() => go('graphic')}>
          <div className="ch-mode-preview" aria-hidden>
            <div className="ch-mode-note">
              <span className="ch-mode-note-bar" />
              <span className="ch-mode-note-bar short" />
              <span className="ch-mode-note-bar" />
            </div>
          </div>
          <div className="ch-mode-copy">
            <h2>发图文</h2>
            <p>种草文 → 配图封面 → 图文渠道</p>
            <span className="ch-mode-cta">开始制作</span>
          </div>
        </button>

        {/* 口播视频向导（08 计划 D2——傻瓜式主链路） */}
        <button type="button" className="ch-mode ch-mode-lipsync" onClick={() => navigate('/m/compose/lipsync')}>
          <div className="ch-mode-preview" aria-hidden>
            <div className="ch-mode-phone">
              <span>口播成片 · 五步向导</span>
            </div>
          </div>
          <div className="ch-mode-copy">
            <h2>拍同款口播</h2>
            <p>提取爆款文案 → 选谁出镜 → 配音色 → 一键成片</p>
            <span className="ch-mode-cta">开始向导</span>
          </div>
        </button>
      </div>

      <div className="ch-panels">
        <section className="ch-panel ch-panel-works">
          <div className="ch-panel-head">
            <h3>待处理作品</h3>
            <Link to="/m/works">查看全部</Link>
          </div>
          {worksLoading ? (
            <div className="ch-panel-empty"><Spin /></div>
          ) : pending.length === 0 ? (
            <div className="ch-panel-empty">
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无待发作品——做好一条会出现在这里" />
            </div>
          ) : (
            <ul className="ch-list">
              {pending.map((w) => (
                <li key={w.id}>
                  <button
                    type="button"
                    className={`ch-list-row ch-status-${w.status}`}
                    onClick={() => {
                      if (w.status === 'ready' || w.status === 'published') {
                        const q = new URLSearchParams()
                        if (w.content_id) q.set('contentId', w.content_id)
                        if (w.media_urls?.length) q.set('mediaUrls', w.media_urls.join(','))
                        if (w.brand_id) q.set('brandId', w.brand_id)
                        q.set('contentType', w.kind === 'article' || w.kind === 'image' ? 'article' : 'video')
                        navigate(`/m/distribution?${q.toString()}`)
                      } else {
                        navigate(w.kind === 'article' || w.kind === 'image' ? '/m/compose/graphic' : '/m/compose/video')
                      }
                    }}
                  >
                    <span className="ch-list-status">{WORK_STATUS[w.status] || w.status}</span>
                    <span className="ch-list-title">{w.title || '未命名作品'}</span>
                    <span className="ch-list-meta">
                      {w.kind === 'video' ? '视频' : w.kind === 'article' || w.kind === 'image' ? '图文' : w.kind}
                    </span>
                    <span className="ch-list-action">{w.status === 'ready' ? '去发布' : '打开'}</span>
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
            <ul className="ch-list">
              {hotList.map((v, i) => (
                <li key={(v.url || v.title) + i}>
                  <div className="ch-list-row ch-list-row-static">
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

      {hasDraft && (
        <footer className="ch-hub-foot">
          <span />
          <button
            type="button"
            className="ch-hub-clear"
            onClick={() => {
              if (window.confirm('清空当前创作草稿？')) draft.reset()
            }}
          >
            清空草稿
          </button>
        </footer>
      )}
    </div>
  )
}

import { Empty, Tag, Typography } from 'antd'
import { PlatformBadge } from '../../../components/PlatformBadge'
import type { PublishChannelView } from '../../../types/api'
import type { WizardDraft } from './wizardModel'

const { Text } = Typography

type AdaptPreview = {
  title?: string
  description?: string
  tags?: string[]
  title_truncated?: boolean
  error?: string
}

type Quota = {
  used_today: number
  max_per_day: number
  at_limit: boolean
}

type Props = {
  draft: WizardDraft
  targetPlatforms: string[]
  channelByPlatform: Map<string, PublishChannelView>
  adaptByPlatform: Map<string, AdaptPreview>
  quotaByPlatform: Map<string, Quota>
  platformMeta: Record<string, { name: string }>
}

const CONTENT_TYPE_LABEL: Record<WizardDraft['contentType'], string> = {
  video: '视频',
  image: '图文',
  article: '文章',
}

/** 发布向导 Step5：共用内容摘要 + 各平台差异行 */
export function PublishConfirmSummary({
  draft,
  targetPlatforms,
  channelByPlatform,
  adaptByPlatform,
  quotaByPlatform,
  platformMeta,
}: Props) {
  if (targetPlatforms.length === 0) {
    return <Empty description="还没有目标平台" />
  }

  const contentPreview = (draft.content || '').trim()
  const scheduleLabel = draft.isScheduled
    ? `定时 ${draft.scheduledAt?.replace('T', ' ') || '未设时间'}`
    : '立即发布'

  return (
    <div className="pub-confirm-summary">
      <section className="pub-summary-card" aria-label="发布内容摘要">
        <div className="pub-summary-head">
          <Text strong style={{ fontSize: 14 }}>发布内容</Text>
          <Tag style={{ margin: 0 }}>{CONTENT_TYPE_LABEL[draft.contentType]}</Tag>
        </div>
        <div className="pub-summary-row">
          <span className="pub-summary-label">标题</span>
          <strong className="pub-summary-value">{draft.title?.trim() || '(无标题)'}</strong>
        </div>
        {contentPreview && (
          <div className="pub-summary-row pub-summary-row--block">
            <span className="pub-summary-label">正文</span>
            <p className="pub-summary-body">{contentPreview.slice(0, 240)}{contentPreview.length > 240 ? '…' : ''}</p>
          </div>
        )}
        <div className="pub-summary-meta">
          {draft.mediaURLs.length > 0 && (
            <span>素材 ×{draft.mediaURLs.length}{draft.coverURL ? ' · 有封面' : ''}</span>
          )}
          {draft.tags.length > 0 && <span>标签 {draft.tags.length} 个</span>}
          <span>{draft.mode === 'auto' ? '全自动' : '半自动'}{draft.autoSelect && draft.mode === 'auto' ? ' · 自动选号' : ''}</span>
          <span>{scheduleLabel}</span>
          {draft.storeAddress?.trim() && <span>门店定位</span>}
        </div>
      </section>

      <section className="pub-platform-deltas" aria-label="各平台适配">
        <Text type="secondary" className="pub-platform-deltas-label">各平台适配</Text>
        <ul className="pub-platform-list">
          {targetPlatforms.map((p) => {
            const ch = channelByPlatform.get(p)
            const adapted = adaptByPlatform.get(p)
            const q = quotaByPlatform.get(p)
            const baseTitle = draft.title?.trim() || '(无标题)'
            const adaptedTitle = adapted?.title?.trim()
            const titleChanged = adaptedTitle && adaptedTitle !== baseTitle
            const descChanged = adapted?.description && adapted.description.trim() !== contentPreview
            const hasTags = (adapted?.tags?.length || 0) > 0
            const hasDelta = titleChanged || adapted?.title_truncated || descChanged || hasTags || adapted?.error

            return (
              <li
                key={p}
                className={`pub-platform-row${q?.at_limit ? ' is-limit' : ''}${adapted?.error ? ' is-error' : ''}`}
              >
                <div className="pub-platform-row-head">
                  <PlatformBadge platform={p} size={14} />
                  <Text strong style={{ fontSize: 13 }}>
                    {ch?.name || platformMeta[p]?.name || p}
                  </Text>
                  {q && q.max_per_day > 0 && (
                    <Tag color={q.at_limit ? 'error' : 'default'} style={{ margin: 0 }}>
                      今日 {q.used_today}/{q.max_per_day}
                    </Tag>
                  )}
                  {!hasDelta && !adapted?.error && (
                    <Text type="secondary" style={{ fontSize: 12 }}>与原文一致</Text>
                  )}
                </div>
                {adapted?.error ? (
                  <Text type="danger" style={{ fontSize: 12 }}>{adapted.error}</Text>
                ) : hasDelta ? (
                  <div className="pub-platform-delta-body">
                    {titleChanged && (
                      <div>
                        <span className="pub-delta-label">标题</span>
                        <span>{adaptedTitle}</span>
                        {adapted?.title_truncated && <Tag color="orange" style={{ marginLeft: 6 }}>已截断</Tag>}
                      </div>
                    )}
                    {adapted?.title_truncated && !titleChanged && (
                      <div><Tag color="orange" style={{ margin: 0 }}>标题已按平台限制截断</Tag></div>
                    )}
                    {descChanged && adapted?.description && (
                      <div>
                        <span className="pub-delta-label">描述</span>
                        <span>{adapted.description.slice(0, 120)}{adapted.description.length > 120 ? '…' : ''}</span>
                      </div>
                    )}
                    {hasTags && (
                      <div className="pub-delta-tags">
                        {adapted!.tags!.map((t) => <Tag key={t} style={{ margin: 0 }}>{t}</Tag>)}
                      </div>
                    )}
                  </div>
                ) : null}
              </li>
            )
          })}
        </ul>
      </section>
    </div>
  )
}

import { useMemo, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Button, Empty, Spin, Typography } from 'antd'
import { SendOutlined } from '@ant-design/icons'
import { usePublishableWorks } from '../../../hooks/usePublishableWorks'
import { brollLineage } from '../../../utils/publishableWorks'
import { BrollPanel } from '../../../components/compose/BrollPanel'
import { PageBackLink } from '../../../components/PageBackLink'
import { cleanWorkTitle } from '../../../utils/workTitle'
import { distributionPathFromWork } from '../../../utils/distributionPath'

const { Text } = Typography

const STATUS_CHIP: Record<string, { label: string; tone: string }> = {
  draft: { label: '草稿', tone: 'draft' },
  generating: { label: '生成中', tone: 'processing' },
  ready: { label: '待发布', tone: 'ready' },
  published: { label: '已发布', tone: 'published' },
}

const KIND_LABEL: Record<string, string> = {
  article: '文章', video: '视频', image: '图片', audio: '音频',
}

const PLATFORM_LABEL: Record<string, string> = {
  douyin: '抖音', kuaishou: '快手', zhihu: '知乎', xiaohongshu: '小红书', bilibili: 'B站', wechat: '微信',
}

function WorkMetaChip({ tone, children }: { tone: string; children: ReactNode }) {
  return <span className={`wr-chip wr-chip--${tone}`}>{children}</span>
}

/**
 * 作品详情页（23 号计划 §5 / §8#4）：
 * 成片播放器 + 台词时间轴（B-Roll）+ 发布入口；视频生成作品可插入画面。
 */
export default function WorkDetail() {
  const { workId = '' } = useParams<{ workId: string }>()
  const navigate = useNavigate()
  const { works, tasks, isLoading } = usePublishableWorks()

  const work = useMemo(() => works.find((w) => w.id === workId), [works, workId])
  const { composeWorkIds, brollSourceWorkIds } = useMemo(() => brollLineage(tasks), [tasks])

  const title = cleanWorkTitle(work?.title || '作品详情')
  const taskId = work?.id.startsWith('g-') ? work.id.slice(2) : ''
  const canBroll = work?.kind === 'video' && !!taskId
  const mediaUrl = work?.media_urls?.[0]
  const status = work ? (STATUS_CHIP[work.status] || { label: work.status, tone: 'draft' }) : null

  if (isLoading && !work) {
    return (
      <div className="wr-page-content wd-page">
        <div className="wd-loading"><Spin size="large" /></div>
      </div>
    )
  }

  if (!work) {
    return (
      <div className="wr-page-content wd-page">
        <PageBackLink to="/m/works" label="作品库" />
        <Empty description="未找到该作品" />
      </div>
    )
  }

  return (
    <div className="wr-page-content wd-page">
      <header className="wd-head">
        <div className="wd-head-main">
          <PageBackLink to="/m/works" label="作品库" />

          <h1 className="wd-title">{title}</h1>

          <div className="wd-tags">
            {status && <WorkMetaChip tone={status.tone}>{status.label}</WorkMetaChip>}
            <WorkMetaChip tone="kind">{KIND_LABEL[work.kind] || work.kind}</WorkMetaChip>
            {composeWorkIds.has(work.id) && <WorkMetaChip tone="broll">B-Roll</WorkMetaChip>}
            {brollSourceWorkIds.has(work.id) && <WorkMetaChip tone="broll">已插画面</WorkMetaChip>}
          </div>

          <Text type="secondary" className="wd-sub">
            {canBroll
              ? '按台词插入画面后再发布；合成后新成片与源片都会保留在作品库'
              : '查看成片并去发布'}
          </Text>

          {work.status === 'published' && work.platforms?.length ? (
            <div className="wd-platforms">
              {work.platforms.map((pf) => (
                <WorkMetaChip key={pf} tone="platform">{PLATFORM_LABEL[pf] || pf}</WorkMetaChip>
              ))}
            </div>
          ) : (
            <time className="wd-date">{new Date(work.created_at).toLocaleDateString('zh-CN')}</time>
          )}
        </div>

        <div className="wd-head-actions">
          {work.status !== 'published' && (
            <Button type="primary" size="large" className="ip-btn-primary" icon={<SendOutlined />} onClick={() => navigate(distributionPathFromWork(work))}>
              去发布
            </Button>
          )}
        </div>
      </header>

      {canBroll ? (
        <div className="wd-broll-shell">
          <BrollPanel
            source={{
              taskId,
              title: work.title,
              videoUrl: mediaUrl,
            }}
            variant="page"
            onClose={() => navigate('/m/works')}
            extraActions={
              work.status !== 'published' ? (
                <Button icon={<SendOutlined />} onClick={() => navigate(distributionPathFromWork(work))}>
                  去发布
                </Button>
              ) : undefined
            }
          />
        </div>
      ) : (
        <div className="wd-simple">
          {work.kind === 'video' && mediaUrl ? (
            <video src={mediaUrl} controls className="wd-simple-media" />
          ) : work.kind === 'image' && mediaUrl ? (
            <img src={mediaUrl} alt="" className="wd-simple-media" />
          ) : work.kind === 'audio' && mediaUrl ? (
            <audio src={mediaUrl} controls className="wd-simple-audio" />
          ) : (
            <Empty description="该作品暂无可预览媒体；可直接去发布">
              {work.status !== 'published' && (
                <Button type="primary" icon={<SendOutlined />} onClick={() => navigate(distributionPathFromWork(work))}>
                  去发布
                </Button>
              )}
            </Empty>
          )}
        </div>
      )}
    </div>
  )
}

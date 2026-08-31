import { useMemo } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Button, Empty, Spin, Typography } from 'antd'
import { ArrowLeftOutlined, SendOutlined } from '@ant-design/icons'
import { usePublishableWorks } from '../../../hooks/usePublishableWorks'
import { BrollPanel } from '../../../components/compose/BrollPanel'
import OralJourneyNav from '../../../components/compose/OralJourneyNav'
import { cleanWorkTitle } from '../../../utils/workTitle'
import type { WorkItem } from '../../../types/api'

const { Text, Title } = Typography

function distributionPath(w: WorkItem) {
  const q = new URLSearchParams()
  if (w.content_id) q.set('contentId', w.content_id)
  if (w.media_urls?.length) q.set('mediaUrls', w.media_urls.join(','))
  if (w.brand_id) q.set('brandId', w.brand_id)
  q.set('contentType', w.kind === 'article' ? 'article' : w.kind === 'image' ? 'image' : 'video')
  if (w.title) q.set('title', w.title)
  const s = q.toString()
  return s ? `/m/distribution?${s}` : '/m/distribution'
}

/**
 * 作品详情页（23 号计划 §5 / §8#4）：
 * 成片播放器 + 台词时间轴（B-Roll）+ 发布入口；视频生成作品可插入画面。
 */
export default function WorkDetail() {
  const { workId = '' } = useParams<{ workId: string }>()
  const navigate = useNavigate()
  const { works, isLoading } = usePublishableWorks()

  const work = useMemo(() => works.find((w) => w.id === workId), [works, workId])
  const title = cleanWorkTitle(work?.title || '作品详情')
  const taskId = work?.id.startsWith('g-') ? work.id.slice(2) : ''
  const canBroll = work?.kind === 'video' && !!taskId
  const mediaUrl = work?.media_urls?.[0]

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
        <OralJourneyNav />
        <Empty description="未找到该作品">
          <Button type="primary" onClick={() => navigate('/m/works')}>返回作品库</Button>
        </Empty>
      </div>
    )
  }

  return (
    <div className="wr-page-content wd-page">
      <OralJourneyNav />
      <header className="wd-head">
        <div className="wd-head-main">
          <Link to="/m/works" className="wd-back">
            <ArrowLeftOutlined /> 作品库
          </Link>
          <Title level={3} className="wd-title">{title}</Title>
          <Text type="secondary" className="wd-sub">
            {canBroll
              ? '可按台词插入画面后再发布；源片与合成成片都会留在作品库'
              : '查看成片并去发布'}
          </Text>
        </div>
        <div className="wd-head-actions">
          {work.status !== 'published' && (
            <Button type="primary" icon={<SendOutlined />} onClick={() => navigate(distributionPath(work))}>
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
                <Button icon={<SendOutlined />} onClick={() => navigate(distributionPath(work))}>
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
            <Empty description="该作品暂无可预览媒体；可直接去发布" />
          )}
        </div>
      )}
    </div>
  )
}

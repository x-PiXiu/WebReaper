import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Empty, Input, Modal, Select, Space, Spin, Tag, Typography, message } from 'antd'
import { PlayCircleOutlined, SyncOutlined, VideoCameraAddOutlined, SearchOutlined } from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useComposeDraft } from '../../../store/composeDraft'
import { PlatformBadge } from '../../../components/PlatformBadge'
import type { HotVideo } from '../../../types/api'

const { Text, Paragraph } = Typography

const SORT_OPTIONS = [
  { value: '', label: '最新入库' },
  { value: 'publish_time', label: '发布时间' },
  { value: 'digg_count', label: '点赞最多' },
  { value: 'play_count', label: '播放最多' },
  { value: 'comment_count', label: '评论最多' },
]

const PLATFORM_OPTIONS = [
  { value: '', label: '全部平台' },
  { value: 'douyin', label: '抖音' },
  { value: 'kuaishou', label: '快手' },
  { value: 'xiaohongshu', label: '小红书' },
  { value: 'bilibili', label: 'B站' },
  { value: 'web', label: '网页' },
]

/**
 * 热门同款（人设档案 Tab）：同赛道爆款短视频入口；完整广场见「灵感广场」。
 * 支持：关键词搜索 / 平台筛选 / 排序（DB 持久化数据，定时采集积累）。
 */
export default function HotVideosTab({ brandId }: { brandId: string }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const draft = useComposeDraft()
  const [shooting, setShooting] = useState<HotVideo | null>(null)
  const [topicDraft, setTopicDraft] = useState('')

  // 筛选/排序状态
  const [keyword, setKeyword] = useState('')
  const [platform, setPlatform] = useState('')
  const [sortBy, setSortBy] = useState('')

  const hasFilter = !!(keyword || platform || sortBy)

  const { data, isLoading } = useQuery({
    queryKey: ['hot-videos', brandId, { keyword, platform, sortBy }],
    queryFn: () => businessApi.listHotVideos(brandId, {
      ...(hasFilter ? { platform: platform || undefined, q: keyword || undefined, sort_by: sortBy || undefined } : {}),
    }),
    enabled: !!brandId,
    staleTime: hasFilter ? 0 : 24 * 3600_000,
  })
  const videos = data?.videos || []
  const total = data?.total

  const refresh = async () => {
    message.loading({ content: '正在搜索最新热门视频…', key: 'hv', duration: 0 })
    try {
      await businessApi.listHotVideos(brandId, { force: true })
      queryClient.invalidateQueries({ queryKey: ['hot-videos', brandId] })
      message.success({ content: '已更新（搜索结果自动入库积累）', key: 'hv' })
    } catch {
      message.error({ content: '搜索失败，稍后再试', key: 'hv' })
    }
  }

  const openShoot = (v: HotVideo) => {
    setShooting(v)
    setTopicDraft(v.topic || `拍一条同款：${v.title}`)
  }

  const confirmShoot = () => {
    if (!shooting) return
    const topic = topicDraft.trim()
    if (topic.length < 4) {
      message.warning('选题至少 4 个字')
      return
    }
    draft.setTrack('video')
    draft.patch({
      brandId,
      sourceUrl: shooting.url || undefined,
      refTitle: shooting.title,
      hotPoint: shooting.hot_point || undefined,
      script: topic,
      transcript: [
        shooting.hot_point ? `【为什么火】${shooting.hot_point}` : '',
        `【选题】${topic}`,
        shooting.url ? `【来源】${shooting.url}` : '',
      ].filter(Boolean).join('\n'),
      selectedTitle: shooting.title.slice(0, 40),
    })
    setShooting(null)
    navigate('/m/compose/lipsync')
  }

  // 提取文案并导航到向导（08 计划 D4——灵感广场「提取文案」入口）
  const extractAndGo = async (video: HotVideo) => {
    try {
      const result = await businessApi.extractTranscript({
        video_url: video.url, // 或 share_url，根据 HotVideo 结构
        title: video.title
      })
      navigate('/m/compose/lipsync', {
        state: {
          rawText: result.raw_text,
          title: result.title,
          method: result.method
        }
      })
    } catch (e: any) {
      message.error(e?.response?.data?.msg || e?.message || '提取失败')
    }
  }

  if (isLoading) {
    return (
      <div className="wr-glass-card" style={{ padding: 48, textAlign: 'center' }}>
        <Spin tip="正在发现你行业的爆款视频…" />
      </div>
    )
  }

  return (
    <div className="wr-glass-card" style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16, flexWrap: 'wrap', gap: 8 }}>
        <Space>
          <VideoCameraAddOutlined style={{ color: 'var(--wr-accent)' }} />
          <Text strong style={{ fontSize: 16 }}>热门同款</Text>
          <Tag style={{ margin: 0 }}>{videos.length} 个</Tag>
        </Space>
        <Space>
          <Button type="link" size="small" onClick={() => navigate('/m/inspire')}>灵感广场 →</Button>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {hasFilter ? `筛选结果：${videos.length}${total != null ? `/${total}` : ''} 个` : `同赛道爆款 · 每天更新 · ${videos.length} 个`}
          </Text>
          <Button size="small" icon={<SyncOutlined />} onClick={refresh}>换一批</Button>
        </Space>
      </div>

      {/* 筛选/排序工具栏 */}
      <div style={{ display: 'flex', gap: 10, marginBottom: 16, flexWrap: 'wrap', alignItems: 'center' }}>
        <Input
          size="small"
          placeholder="搜索标题/作者/火爆点"
          prefix={<SearchOutlined />}
          value={keyword}
          onChange={e => setKeyword(e.target.value)}
          style={{ width: 200 }}
          allowClear
        />
        <Select size="small" value={platform} onChange={setPlatform} options={PLATFORM_OPTIONS} style={{ width: 110 }} />
        <Select size="small" value={sortBy} onChange={setSortBy} options={SORT_OPTIONS} style={{ width: 120 }} />
        {hasFilter && (
          <Button size="small" type="link" onClick={() => { setKeyword(''); setPlatform(''); setSortBy('') }}>
            清除筛选
          </Button>
        )}
      </div>

      {videos.length === 0 ? (
        <Empty description="暂未发现热门视频——完善人设的行业与定位后再试" />
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 14 }}>
          {videos.map((v, i) => (
            <div
              key={v.url + i}
              className="wr-glass-card"
              style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10, background: 'var(--wr-bg-elevated)' }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 8 }}>
                <Text strong style={{ fontSize: 13.5, lineHeight: 1.5 }} ellipsis={{ tooltip: v.title }}>{v.title}</Text>
                <PlatformBadge platform={v.platform} size={14} />
              </div>
              {/* 互动数据 */}
              {(v.digg_count || v.play_count || v.comment_count) ? (
                <div style={{ display: 'flex', gap: 10, fontSize: 11, color: 'var(--wr-text-secondary)' }}>
                  {v.play_count ? <span>👁 {v.play_count > 10000 ? `${(v.play_count / 10000).toFixed(1)}万` : v.play_count}</span> : null}
                  {v.digg_count ? <span>❤ {v.digg_count > 10000 ? `${(v.digg_count / 10000).toFixed(1)}万` : v.digg_count}</span> : null}
                  {v.comment_count ? <span>💬 {v.comment_count}</span> : null}
                  {v.author ? <span style={{ marginLeft: 'auto' }}>@{v.author}</span> : null}
                </div>
              ) : null}
              {v.hot_point && (
                <Paragraph type="secondary" style={{ margin: 0, fontSize: 12.5, lineHeight: 1.6 }}>
                  🔥 {v.hot_point}
                </Paragraph>
              )}
              {v.topic && (
                <div style={{ padding: '8px 10px', borderRadius: 8, background: 'rgba(94,234,212,0.06)', border: '1px solid rgba(94,234,212,0.15)' }}>
                  <Text style={{ fontSize: 12, color: 'var(--wr-accent)' }}>同款选题：{v.topic}</Text>
                </div>
              )}
              <Space style={{ marginTop: 'auto' }}>
                <Button size="small" icon={<PlayCircleOutlined />} onClick={() => window.open(v.url, '_blank', 'noopener')}>播放</Button>
                <Button size="small" type="primary" onClick={() => openShoot(v)}>复刻视频</Button>
                <Button size="small" onClick={() => extractAndGo(v)}>提取文案</Button>
              </Space>
            </div>
          ))}
        </div>
      )}

      <Modal
        open={!!shooting}
        title="复刻为短视频"
        okText="进入发视频"
        cancelText="取消"
        onOk={confirmShoot}
        onCancel={() => setShooting(null)}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }} size={8}>
          <Text type="secondary" style={{ fontSize: 12 }}>参考爆款：{shooting?.title}</Text>
          <Text style={{ fontSize: 13 }}>确认或修改选题后进入发视频步骤：</Text>
          <Input.TextArea
            rows={3}
            value={topicDraft}
            onChange={(e) => setTopicDraft(e.target.value)}
            maxLength={120}
            showCount
            placeholder="拍摄同款的选题/想写什么"
          />
        </Space>
      </Modal>
    </div>
  )
}

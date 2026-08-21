import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Empty, Input, Modal, Space, Spin, Tag, Typography, message } from 'antd'
import { PlayCircleOutlined, SyncOutlined, VideoCameraAddOutlined } from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useComposeDraft } from '../../../store/composeDraft'
import type { HotVideo } from '../../../types/api'

const { Text, Paragraph } = Typography

const PLATFORM_LABEL: Record<string, string> = {
  douyin: '抖音',
  kuaishou: '快手',
  xiaohongshu: '小红书',
  bilibili: 'B站',
  weishi: '微视',
  web: '网页',
}

/**
 * 热门同款（人设档案 Tab）：同赛道爆款短视频入口；完整广场见「灵感广场」。
 */
export default function HotVideosTab({ brandId }: { brandId: string }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const draft = useComposeDraft()
  const [shooting, setShooting] = useState<HotVideo | null>(null)
  const [topicDraft, setTopicDraft] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['hot-videos', brandId],
    queryFn: () => businessApi.listHotVideos(brandId),
    enabled: !!brandId,
    staleTime: 24 * 3600_000,
  })
  const videos = data?.videos || []

  const refresh = async () => {
    message.loading({ content: '正在搜索最新热门视频…', key: 'hv', duration: 0 })
    try {
      await businessApi.listHotVideos(brandId, true)
      queryClient.invalidateQueries({ queryKey: ['hot-videos', brandId] })
      message.success({ content: '已更新', key: 'hv' })
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
    navigate('/m/compose/video')
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
          <Text type="secondary" style={{ fontSize: 12 }}>同赛道爆款 · 每天更新</Text>
          <Button size="small" icon={<SyncOutlined />} onClick={refresh}>换一批</Button>
        </Space>
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
                <Tag color="cyan" style={{ margin: 0, flexShrink: 0 }}>{PLATFORM_LABEL[v.platform] || v.platform}</Tag>
              </div>
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

import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Empty, Input, Segmented, Space, Tag, Typography } from 'antd'
import { PlusOutlined, SendOutlined } from '@ant-design/icons'
import { formatDuration } from '../../../mock/ipAssets'
import { useWorksStore } from '../../../store/works'

const { Text, Title } = Typography

type Filter = 'all' | 'draft' | 'ready' | 'published'

const STATUS_LABEL: Record<string, string> = {
  draft: '草稿',
  ready: '待发布',
  published: '已发布',
}

/**
 * 我的作品：合成向导产出 + 演示种子数据的统一作品库。
 */
export default function MyWorks() {
  const navigate = useNavigate()
  const works = useWorksStore((s) => s.works)
  const markPublished = useWorksStore((s) => s.markPublished)
  const [filter, setFilter] = useState<Filter>('all')
  const [q, setQ] = useState('')

  const list = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return works.filter((w) => {
      if (filter !== 'all' && w.status !== filter) return false
      if (needle && !w.title.toLowerCase().includes(needle)) return false
      return true
    })
  }, [works, filter, q])

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Library</p>
          <h1>我的作品</h1>
          <p className="ip-lead">成片、草稿与已发布内容——发布后自动沉淀于此</p>
        </div>
        <Button type="primary" size="large" className="ip-btn-primary" icon={<PlusOutlined />} onClick={() => navigate('/m/compose')}>
          去合成新作品
        </Button>
      </div>

      <div className="ip-toolbar">
        <Segmented
          value={filter}
          onChange={(v) => setFilter(v as Filter)}
          options={[
            { label: '全部', value: 'all' },
            { label: '草稿', value: 'draft' },
            { label: '待发布', value: 'ready' },
            { label: '已发布', value: 'published' },
          ]}
        />
        <Input.Search allowClear placeholder="搜索作品标题" style={{ maxWidth: 280 }} value={q} onChange={(e) => setQ(e.target.value)} />
      </div>

      {list.length === 0 ? (
        <Empty description="暂无作品" style={{ padding: 64 }}>
          <Button type="primary" className="ip-btn-primary" onClick={() => navigate('/m/compose')}>开始合成</Button>
        </Empty>
      ) : (
        <div className="ip-works-grid ip-stagger">
          {list.map((w) => (
            <article key={w.id} className="ip-work-card">
              <div className="ip-work-cover" style={{ background: `linear-gradient(155deg,#0b0b10 10%, ${w.coverAccent})` }}>
                <Tag className="ip-work-status">{STATUS_LABEL[w.status]}</Tag>
                {w.durationSec ? <span className="ip-work-dur">{formatDuration(w.durationSec)}</span> : null}
              </div>
              <div className="ip-work-body">
                <Title level={5} style={{ margin: 0 }}>{w.title}</Title>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {w.platform ? `${w.platform} · ` : ''}
                  {new Date(w.createdAt).toLocaleString('zh-CN', { hour12: false })}
                </Text>
                {w.status === 'published' && (
                  <div className="ip-work-metrics">
                    <span>播放 {(w.views || 0).toLocaleString()}</span>
                    <span>互动 {(w.likes || 0).toLocaleString()}</span>
                    <span>线索 {w.leads || 0}</span>
                  </div>
                )}
                <Space style={{ marginTop: 12 }}>
                  {w.status !== 'published' && (
                    <Button
                      size="small"
                      type="primary"
                      className="ip-btn-primary"
                      icon={<SendOutlined />}
                      onClick={() => {
                        markPublished(w.id, w.platform || '抖音')
                        navigate('/m/analytics')
                      }}
                    >
                      发布（演示）
                    </Button>
                  )}
                  {w.status === 'published' && (
                    <Button size="small" onClick={() => navigate('/m/analytics')}>查看数据</Button>
                  )}
                </Space>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}

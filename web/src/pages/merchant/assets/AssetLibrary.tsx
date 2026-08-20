import { useMemo, useState } from 'react'
import { Button, Empty, Input, Segmented, Space, Tag, Typography, message } from 'antd'
import { PlusOutlined, SoundOutlined, UserOutlined, PictureOutlined, LayoutOutlined } from '@ant-design/icons'
import {
  MOCK_AVATARS, MOCK_VOICES, MOCK_STORYBOARDS, MOCK_COVERS, formatDuration,
} from '../../../mock/ipAssets'

const { Text, Title } = Typography

type TabKey = 'avatar' | 'voice' | 'storyboard' | 'cover'

/**
 * 资产库：形象 / 音色 / 分镜 / 封面（演示假数据）。
 * 为内容合成分步向导提供可挑选的 IP 素材底座。
 */
export default function AssetLibrary() {
  const [tab, setTab] = useState<TabKey>('avatar')
  const [q, setQ] = useState('')

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    const match = (s: string) => !needle || s.toLowerCase().includes(needle)
    if (tab === 'avatar') return MOCK_AVATARS.filter((a) => match(a.name) || a.tags.some(match))
    if (tab === 'voice') return MOCK_VOICES.filter((v) => match(v.name) || match(v.tone))
    if (tab === 'storyboard') return MOCK_STORYBOARDS.filter((s) => match(s.title) || match(s.scene))
    return MOCK_COVERS.filter((c) => match(c.name) || match(c.mood))
  }, [tab, q])

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">IP Assets</p>
          <h1>资产库</h1>
          <p className="ip-lead">形象、音色、分镜与封面模板——合成向导会从这里取用素材</p>
        </div>
        <Button
          type="primary"
          size="large"
          className="ip-btn-primary"
          icon={<PlusOutlined />}
          onClick={() => message.info('演示版：上传接入将在后端就绪后开放')}
        >
          上传素材
        </Button>
      </div>

      <div className="ip-toolbar">
        <Segmented
          value={tab}
          onChange={(v) => setTab(v as TabKey)}
          options={[
            { label: <span><UserOutlined /> 形象库</span>, value: 'avatar' },
            { label: <span><SoundOutlined /> 音色库</span>, value: 'voice' },
            { label: <span><LayoutOutlined /> 分镜素材</span>, value: 'storyboard' },
            { label: <span><PictureOutlined /> 封面模板</span>, value: 'cover' },
          ]}
        />
        <Input.Search
          allowClear
          placeholder="搜索名称 / 标签"
          style={{ maxWidth: 280 }}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>

      {filtered.length === 0 ? (
        <Empty description="没有匹配素材" style={{ padding: 64 }} />
      ) : (
        <div className="ip-asset-grid ip-stagger">
          {tab === 'avatar' && (filtered as typeof MOCK_AVATARS).map((a) => (
            <article key={a.id} className="ip-asset-card">
              <div className="ip-asset-cover" style={{ background: a.cover }} />
              <div className="ip-asset-body">
                <Title level={5} style={{ margin: 0 }}>{a.name}</Title>
                <Text type="secondary">{a.style}</Text>
                <Space size={[6, 6]} wrap style={{ marginTop: 10 }}>
                  {a.tags.map((t) => <Tag key={t}>{t}</Tag>)}
                </Space>
                <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 10 }}>更新 {a.updatedAt}</Text>
              </div>
            </article>
          ))}

          {tab === 'voice' && (filtered as typeof MOCK_VOICES).map((v) => (
            <article key={v.id} className="ip-asset-card">
              <div className="ip-asset-cover ip-asset-cover--voice">
                <SoundOutlined style={{ fontSize: 28 }} />
                <span>{v.sampleLabel}</span>
              </div>
              <div className="ip-asset-body">
                <Title level={5} style={{ margin: 0 }}>{v.name}</Title>
                <Text type="secondary">{v.tone} · {v.lang}</Text>
                <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 10 }}>
                  时长 {formatDuration(v.durationSec)}
                </Text>
              </div>
            </article>
          ))}

          {tab === 'storyboard' && (filtered as typeof MOCK_STORYBOARDS).map((s) => (
            <article key={s.id} className="ip-asset-card">
              <div className="ip-asset-cover ip-asset-cover--board">
                <span className="ip-ratio">{s.ratio}</span>
                <strong>{formatDuration(s.durationSec)}</strong>
              </div>
              <div className="ip-asset-body">
                <Title level={5} style={{ margin: 0 }}>{s.title}</Title>
                <Text type="secondary">{s.scene}</Text>
              </div>
            </article>
          ))}

          {tab === 'cover' && (filtered as typeof MOCK_COVERS).map((c) => (
            <article key={c.id} className="ip-asset-card">
              <div
                className="ip-asset-cover"
                style={{ background: `linear-gradient(160deg, #0b0b10 20%, ${c.accent}55)` }}
              >
                <span className="ip-ratio">{c.ratio}</span>
              </div>
              <div className="ip-asset-body">
                <Title level={5} style={{ margin: 0 }}>{c.name}</Title>
                <Text type="secondary">{c.mood}</Text>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}

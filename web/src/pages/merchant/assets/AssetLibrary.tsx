import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button, Empty, Input, Segmented, Space, Tag, Typography, message } from 'antd'
import { SoundOutlined, PictureOutlined } from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import type { MediaAsset } from '../../../types/api'

const { Text } = Typography

function formatSize(bytes: number) {
  if (bytes > 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB'
  if (bytes > 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
  if (bytes > 1 << 10) return (bytes >> 10) + ' KB'
  return bytes + ' B'
}

function timeAgo(iso: string) {
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60_000)
  if (m < 1) return '刚刚'
  if (m < 60) return `${m} 分钟前`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} 小时前`
  return `${Math.floor(h / 24)} 天前`
}

/**
 * 资产库：真实媒体库（MediaAsset——素材上传 + AI 产物转存）。
 * 有则显示无则隐藏：音频/图片 tab 无数据时空态引导；
 * avatar/storyboard 等无后端概念的分类不出现（未来真做再恢复）。
 */
export default function AssetLibrary() {
  const [tab, setTab] = useState<'audio' | 'image'>('audio')
  const [q, setQ] = useState('')

  const { data: res, isLoading } = useQuery({
    queryKey: ['media-assets'],
    queryFn: () => businessApi.listAssets().catch(() => ({ assets: [] as MediaAsset[] })),
  })
  const assets = res?.assets || []

  const audioAssets = useMemo(() => assets.filter((a) => a.mime.startsWith('audio/')), [assets])
  const imageAssets = useMemo(() => assets.filter((a) => a.mime.startsWith('image/')), [assets])

  const filtered = useMemo(() => {
    const list = tab === 'audio' ? audioAssets : imageAssets
    const needle = q.trim().toLowerCase()
    if (!needle) return list
    return list.filter((a) => a.url.toLowerCase().includes(needle) || a.mime.toLowerCase().includes(needle))
  }, [tab, audioAssets, imageAssets, q])

  const hint = tab === 'audio' ? '配音/音效素材（视频创作的音频引用源）' : '图片素材（封面/图文创作的引用源）'

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Digital Twin</p>
          <h1>数字分身</h1>
          <p className="ip-lead">{hint}——形象、音色与封面素材，供口播数字人与成片取用</p>
        </div>
      </div>

      <div className="ip-toolbar">
        <Segmented
          value={tab}
          onChange={(v) => setTab(v as 'audio' | 'image')}
          options={[
            { value: 'audio', label: `音频 ${audioAssets.length}`, icon: <SoundOutlined /> },
            { value: 'image', label: `图片 ${imageAssets.length}`, icon: <PictureOutlined /> },
          ]}
        />
        <Input.Search allowClear placeholder="按文件名/类型搜索" style={{ maxWidth: 240 }} value={q} onChange={(e) => setQ(e.target.value)} />
      </div>

      {isLoading ? (
        <Empty description="加载中…" style={{ padding: 60 }} />
      ) : filtered.length === 0 ? (
        <Empty style={{ padding: 60 }} description={`暂无${tab === 'audio' ? '音频' : '图片'}资产`}>
          <Button type="primary" onClick={() => message.info('在内容合成中上传素材或生成作品，产物会自动入库存')}>
            了解如何入库
          </Button>
        </Empty>
      ) : (
        <div className="ip-asset-grid">
          {filtered.map((a) => (
            <div key={a.id} className="ip-asset-card">
              <div
                className={`ip-asset-cover ${a.mime.startsWith('audio/') ? 'ip-asset-cover--voice' : ''}`}
                style={a.mime.startsWith('image/') ? { background: `linear-gradient(180deg, rgba(0,0,0,0.05), rgba(0,0,0,0.45)), url(${a.url}) center/cover`, display: 'flex', alignItems: 'flex-end', padding: 12 } : undefined}
              >
                <Tag style={{ margin: 0 }} color={a.owner_type === 'creation' ? 'cyan' : 'blue'}>
                  {a.owner_type === 'creation' ? 'AI 产物' : '上传素材'}
                </Tag>
              </div>
              <div className="ip-asset-body">
                <Text strong style={{ fontSize: 13 }} ellipsis={{ tooltip: a.url }}>
                  {a.url.split('/').pop()?.split('?')[0] || '资产'}
                </Text>
                <div style={{ display: 'flex', gap: 10, marginTop: 6, fontSize: 12, color: 'var(--wr-text-secondary)' }}>
                  <span>{a.mime.split('/')[1]?.toUpperCase()}</span>
                  <span>{formatSize(a.size_bytes)}</span>
                  <span>{timeAgo(a.created_at)}</span>
                </div>
                <Space style={{ marginTop: 10 }}>
                  <Button size="small" onClick={() => window.open(a.url, '_blank', 'noopener')}>
                    {a.mime.startsWith('image/') ? '查看' : '播放'}
                  </Button>
                  <Button size="small" type="text" danger onClick={async () => {
                    try {
                      await businessApi.deleteAsset(a.id)
                      message.success('已删除')
                    } catch { /* 拦截器已提示 */ }
                  }}>删除</Button>
                </Space>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

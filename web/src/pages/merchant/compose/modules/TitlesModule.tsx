import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input, Space, Tag, Typography } from 'antd'
import { ThunderboltOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { message } from '../../../../utils/antdApp'

const { Text } = Typography

/** 标题话题：根据文案生成标题与话题（视频口播 / 图文种草共用） */
export default function TitlesModule() {
  const navigate = useNavigate()
  const { brandId, brands } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const body = draft.rewritten || draft.script || draft.transcript || ''
  const isGraphic = draft.track === 'graphic'

  const generate = async () => {
    if (!brandId) {
      message.warning('请先选择人设')
      return
    }
    if (!body.trim()) {
      message.warning('请先准备文案')
      return
    }
    setBusy(true)
    try {
      const brand = brands.find((b) => b.id === brandId)
      const res = await businessApi.generateContent(brandId, {
        topic: `根据以下口播稿，输出 5 个短视频标题（每行一个，≤20字）和 8 个话题标签（#开头，空格分隔）。不要正文。\n\n${body.slice(0, 1200)}`,
        brand_info: brand ? `${brand.name}：${brand.positioning || ''}` : '',
        format: 'xiaohongshu',
      })
      const raw = res.optimized_text || res.title || ''
      const lines = raw.split(/\n+/).map((l) => l.replace(/^[\d.、\-]+\s*/, '').trim()).filter(Boolean)
      const titles = lines.filter((l) => !l.startsWith('#')).slice(0, 5)
      const topicLine = lines.find((l) => l.includes('#')) || ''
      const topics = topicLine.split(/\s+/).filter((t) => t.startsWith('#')).slice(0, 8)
      const fallbackTopics = (raw.match(/#[\w\u4e00-\u9fa5]+/g) || []).slice(0, 8)
      draft.patch({
        titles: titles.length ? titles : [res.title || '未命名标题'].filter(Boolean),
        topics: topics.length ? topics : fallbackTopics,
        selectedTitle: titles[0] || res.title,
        brandId,
      })
      message.success('标题与话题已生成')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader
        title="标题话题"
        lead={isGraphic ? '为种草图文生成标题与话题标签' : '为口播成片生成标题与话题标签'}
        badge={isGraphic ? '发图文' : '发视频'}
      />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Button type="primary" className="ip-btn-primary" icon={<ThunderboltOutlined />} loading={busy} onClick={generate}>
          根据当前文案生成
        </Button>
        <div style={{ marginTop: 20 }}>
          <Text strong>标题候选</Text>
          <Space direction="vertical" style={{ width: '100%', marginTop: 8 }} size={8}>
            {(draft.titles || []).length === 0 && <Text type="secondary">暂无，点击上方生成</Text>}
            {(draft.titles || []).map((t) => (
              <Button
                key={t}
                type={draft.selectedTitle === t ? 'primary' : 'default'}
                block
                style={{ textAlign: 'left', height: 'auto', padding: '8px 12px', whiteSpace: 'normal' }}
                onClick={() => draft.patch({ selectedTitle: t })}
              >
                {t}
              </Button>
            ))}
          </Space>
          {draft.selectedTitle && (
            <Input
              style={{ marginTop: 12 }}
              value={draft.selectedTitle}
              onChange={(e) => draft.patch({ selectedTitle: e.target.value })}
              placeholder="可手改选中标题"
            />
          )}
        </div>
        <div style={{ marginTop: 20 }}>
          <Text strong>话题</Text>
          <div style={{ marginTop: 8 }}>
            {(draft.topics || []).map((t) => (
              <Tag key={t} color="cyan" style={{ marginBottom: 6 }}>{t}</Tag>
            ))}
            {(draft.topics || []).length === 0 && <Text type="secondary">暂无</Text>}
          </div>
        </div>
        <Space style={{ marginTop: 16 }} wrap>
          {draft.track === 'graphic' ? (
            <>
              <Button type="primary" className="ip-btn-primary" onClick={() => navigate('/m/compose/images')}>去图文配图</Button>
              <Button onClick={() => navigate('/m/compose/cover?track=graphic')}>去图文封面</Button>
            </>
          ) : (
            <>
              <Button type="primary" className="ip-btn-primary" onClick={() => navigate('/m/compose/voice')}>去配音</Button>
              <Button onClick={() => navigate('/m/compose/cover')}>去视频封面</Button>
            </>
          )}
        </Space>
      </div>
    </div>
  )
}

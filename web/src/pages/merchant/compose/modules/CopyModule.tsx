import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Input, Segmented, Space, Typography, message } from 'antd'
import { ThunderboltOutlined } from '@ant-design/icons'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'

const { Text } = Typography
const { TextArea } = Input

/**
 * 文案工作室：两轨共用，按轨道切换口播稿 / 图文种草格式。
 */
export default function CopyModule() {
  const navigate = useNavigate()
  const { brandId, brands } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const isGraphic = draft.track === 'graphic'
  const format = isGraphic ? 'xiaohongshu' : 'script'

  const text = draft.script || draft.rewritten || draft.transcript || ''
  const setText = (v: string) => draft.patch({ script: v, rewritten: v })

  const runRewrite = async () => {
    if (!brandId) {
      message.warning('请先选择人设档案')
      return
    }
    if (!text.trim()) {
      message.warning('请先填写或粘贴文案')
      return
    }
    setBusy(true)
    try {
      const brand = brands.find((b) => b.id === brandId)
      const keyword = draft.refTitle || brand?.name || (isGraphic ? '种草获客' : '口播获客')
      const res = await businessApi.optimizeContent({
        brand_id: brandId,
        keyword,
        original_text: text,
        format,
      })
      const out = res.optimized_text || ''
      draft.patch({ rewritten: out, script: out, brandId })
      message.success(isGraphic ? '已改写成差异化图文种草稿' : '已改写成差异化口播稿')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  const runFromTopic = async () => {
    if (!brandId) {
      message.warning('请先选择人设')
      return
    }
    setBusy(true)
    try {
      const brand = brands.find((b) => b.id === brandId)
      const topic = [
        draft.refTitle ? `对标爆款「${draft.refTitle}」` : '',
        draft.hotPoint ? `要点：${draft.hotPoint}` : '',
        text ? `参考原文：${text.slice(0, 400)}` : '',
        isGraphic
          ? '请写一篇差异化种草图文（小红书风格）：有标题感、分段、emoji 适度、结尾行动号召，突出我们的卖点'
          : '请写一条差异化品牌口播稿，保留爆款结构但突出我们的独特卖点与行动号召',
      ].filter(Boolean).join('\n')
      const res = await businessApi.generateContent(brandId, {
        topic,
        brand_info: brand ? `${brand.name}：${brand.positioning || ''}` : '',
        format,
      })
      const out = res.optimized_text || ''
      draft.patch({ rewritten: out, script: out, brandId })
      message.success(isGraphic ? '已生成图文种草稿' : '已生成差异化口播稿')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader
        title="文案工作室"
        lead={isGraphic ? '写种草图文 / 长文，并做差异化改写' : '写口播稿，并做差异化改写'}
        badge={isGraphic ? '发图文' : '发视频'}
      />
      <div style={{ marginBottom: 16 }}>
        <Segmented
          value={draft.track}
          onChange={(v) => draft.setTrack(v as 'video' | 'graphic')}
          options={[
            { value: 'video', label: '口播稿（发视频）' },
            { value: 'graphic', label: '种草图文（发图文）' },
          ]}
        />
      </div>
      {(draft.refTitle || draft.hotPoint) && (
        <Alert
          style={{ marginBottom: 16 }}
          type="info"
          showIcon
          message={`对标参考：${draft.refTitle || '—'}${draft.hotPoint ? ` · ${draft.hotPoint}` : ''}`}
        />
      )}
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 8 }}>
          <Text strong>{isGraphic ? '图文正文' : '口播正文'}</Text>
          <Space wrap>
            <Button
              type="primary"
              className="ip-btn-primary"
              icon={<ThunderboltOutlined />}
              loading={busy}
              disabled={!text.trim()}
              onClick={runRewrite}
            >
              AI 差异化改写
            </Button>
            <Button loading={busy} onClick={runFromTopic}>
              按对标要点重写
            </Button>
          </Space>
        </div>
        <TextArea
          rows={16}
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={isGraphic ? '种草笔记 / 图文正文，可从对标带入后改写' : '口播稿，可从对标带入后改写'}
          showCount
          maxLength={8000}
        />
        <Space style={{ marginTop: 16 }} wrap>
          <Button
            type="primary"
            className="ip-btn-primary"
            onClick={() => {
              if (!text.trim()) {
                message.warning('文案不能为空')
                return
              }
              draft.patch({ script: text, rewritten: text })
              message.success('已保存到共享草稿')
              navigate('/m/compose/titles')
            }}
          >
            保存并去标题话题
          </Button>
          {isGraphic ? (
            <Button onClick={() => navigate('/m/compose/images')}>去图文配图</Button>
          ) : (
            <Button onClick={() => navigate('/m/compose/voice')}>去配音</Button>
          )}
          <Button type="link" onClick={() => navigate(isGraphic ? '/m/compose/graphic' : '/m/compose/video')}>
            返回{isGraphic ? '发图文' : '发视频'}总览
          </Button>
        </Space>
      </div>
    </div>
  )
}

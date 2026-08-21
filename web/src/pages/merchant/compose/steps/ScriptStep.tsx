import { useState } from 'react'
import { Button, Input, Space, message } from 'antd'
import { ThunderboltOutlined, HighlightOutlined } from '@ant-design/icons'
import { useComposeDraft, type ComposeTrack } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'

const { TextArea } = Input

/** Step 1：写脚本 / 写图文 */
export function ScriptStep({ track }: { track: ComposeTrack }) {
  const { brandId, brands } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const isGraphic = track === 'graphic'
  const format = isGraphic ? 'xiaohongshu' : 'script'
  const text = draft.script || draft.rewritten || draft.transcript || ''
  const setText = (v: string) => draft.patch({ script: v, rewritten: v })

  const polish = async () => {
    if (!brandId) {
      message.warning('请先在顶栏选择人设档案')
      return
    }
    if (!text.trim()) {
      message.warning('请先填写文案')
      return
    }
    setBusy(true)
    try {
      const brand = brands.find((b) => b.id === brandId)
      const res = await businessApi.optimizeContent({
        brand_id: brandId,
        keyword: draft.refTitle || brand?.name || (isGraphic ? '种草获客' : '口播获客'),
        original_text: text,
        format,
      })
      const out = res.optimized_text || ''
      draft.patch({ rewritten: out, script: out, brandId })
      message.success('文案已润色')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  const genByTheme = async () => {
    if (!brandId) {
      message.warning('请先选择人设')
      return
    }
    setBusy(true)
    try {
      const brand = brands.find((b) => b.id === brandId)
      const topic = [
        draft.refTitle ? `对标「${draft.refTitle}」` : '',
        draft.hotPoint ? `要点：${draft.hotPoint}` : '',
        text ? `参考：${text.slice(0, 300)}` : '',
        isGraphic
          ? '写一篇差异化种草图文：分段清晰、适度 emoji、结尾行动号召'
          : '写一条差异化口播稿：结构清晰、卖点突出、结尾行动号召',
      ].filter(Boolean).join('\n')
      const res = await businessApi.generateContent(brandId, {
        topic: topic || (isGraphic ? '门店种草图文' : '品牌口播获客'),
        brand_info: brand ? `${brand.name}：${brand.positioning || ''}` : '',
        format,
      })
      const out = res.optimized_text || ''
      draft.patch({ rewritten: out, script: out, brandId })
      message.success('已按主题生成')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  const genTitles = async () => {
    if (!brandId || !text.trim()) {
      message.warning('需要人设与文案后才能生成标题')
      return
    }
    setBusy(true)
    try {
      const brand = brands.find((b) => b.id === brandId)
      const res = await businessApi.generateContent(brandId, {
        topic: `根据以下正文生成 5 条吸睛标题（每行一条），再给 5 个话题标签（#开头，逗号分隔）：\n${text.slice(0, 1200)}`,
        brand_info: brand ? `${brand.name}` : '',
        format: 'script',
      })
      const raw = res.optimized_text || ''
      const lines = raw.split(/\n+/).map((l) => l.replace(/^\d+[\.\)、]\s*/, '').trim()).filter(Boolean)
      const titles = lines.filter((l) => !l.startsWith('#')).slice(0, 5)
      const topics = lines.filter((l) => l.includes('#')).flatMap((l) => l.split(/[,，\s]+/)).filter((t) => t.startsWith('#')).slice(0, 8)
      draft.patch({
        titles,
        topics: topics.length ? topics : draft.topics,
        selectedTitle: titles[0] || draft.selectedTitle,
      })
      message.success('标题已生成')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="cf-panel">
      <div className="cf-editor-toolbar">
        <Space wrap size={8}>
          <Button size="small" icon={<ThunderboltOutlined />} loading={busy} onClick={genByTheme}>
            按主题生成
          </Button>
          <Button size="small" icon={<HighlightOutlined />} loading={busy} disabled={!text.trim()} onClick={polish}>
            文案润色
          </Button>
          <Button size="small" loading={busy} disabled={!text.trim()} onClick={genTitles}>
            生成标题
          </Button>
        </Space>
      </div>

      {(draft.refTitle || draft.hotPoint) && (
        <div className="cf-ref-chip">
          对标：{draft.refTitle || '—'}
          {draft.hotPoint ? ` · ${draft.hotPoint}` : ''}
        </div>
      )}

      <Input
        className="cf-title-input"
        placeholder={isGraphic ? '图文标题（可稍后生成）' : '短视频标题（可稍后生成）'}
        value={draft.selectedTitle || ''}
        onChange={(e) => draft.patch({ selectedTitle: e.target.value })}
      />

      <TextArea
        className="cf-editor"
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={
          isGraphic
            ? '在这里写或粘贴种草正文，也可用上方「按主题生成」…'
            : '在这里写或粘贴口播文案，也可用上方「按主题生成」…'
        }
        autoSize={{ minRows: 14, maxRows: 22 }}
      />

      <div className="cf-editor-foot">
        <span>{(draft.titles || []).length} 个标题候选</span>
        <span>
          {text.replace(/\s/g, '').length} 字
          {!isGraphic && ` · 约 ${Math.round(text.replace(/\s/g, '').length / 4)}s`}
        </span>
      </div>

      {(draft.titles || []).length > 0 && (
        <div className="cf-title-picks">
          {(draft.titles || []).map((t) => (
            <button
              key={t}
              type="button"
              className={`cf-title-pick${draft.selectedTitle === t ? ' is-on' : ''}`}
              onClick={() => draft.patch({ selectedTitle: t })}
            >
              {t}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

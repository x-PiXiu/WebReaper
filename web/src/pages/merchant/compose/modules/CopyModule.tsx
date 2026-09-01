import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Empty, Input, Space } from 'antd'
import {
  EditOutlined,
  FileTextOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import { COPY_TEMPLATES, type CopyTemplate } from '../../../../data/copyTemplates'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { toast } from '../../../../utils/feedback'
import { PageBackLink } from '../../../../components/PageBackLink'

const { TextArea } = Input

type Scope = 'all' | 'oral' | 'graphic' | 'mine'

/**
 * 文案工作室：模板库 + 编辑区（对齐库式页面视觉）。
 */
export default function CopyModule() {
  const navigate = useNavigate()
  const { brandId, brands } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [scope, setScope] = useState<Scope>('all')
  const [q, setQ] = useState('')
  const [activeTpl, setActiveTpl] = useState<string | null>(null)

  const isGraphic = draft.track === 'graphic'
  const format = isGraphic ? 'xiaohongshu' : 'script'
  const text = draft.script || draft.rewritten || draft.transcript || ''
  const setText = (v: string) => draft.patch({ script: v, rewritten: v })

  const templates = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return COPY_TEMPLATES.filter((t) => {
      if (scope === 'oral' && t.track === 'graphic') return false
      if (scope === 'graphic' && t.track === 'video') return false
      if (scope === 'mine') return false
      if (!needle) return true
      return (
        t.title.toLowerCase().includes(needle)
        || t.desc.toLowerCase().includes(needle)
        || t.tag.toLowerCase().includes(needle)
      )
    })
  }, [scope, q])

  const applyTemplate = (tpl: CopyTemplate) => {
    if (tpl.track === 'video' || tpl.track === 'graphic') {
      draft.setTrack(tpl.track)
    }
    setText(tpl.body)
    setActiveTpl(tpl.id)
    toast.ok(`已套用「${tpl.title}」`, 'copy-tpl')
  }

  const runRewrite = async () => {
    if (!brandId) {
      toast.warn('请先选择人设', 'copy-brand')
      return
    }
    if (!text.trim()) {
      toast.warn('请先填写或粘贴文案', 'copy-text')
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
      toast.ok(isGraphic ? '已改写成图文种草稿' : '已改写成口播稿', 'copy-rewrite')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  const runFromTopic = async () => {
    if (!brandId) {
      toast.warn('请先选择人设', 'copy-brand')
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
      toast.ok(isGraphic ? '图文种草稿已生成' : '口播稿已生成', 'copy-gen')
    } catch {
      /* 拦截器 */
    } finally {
      setBusy(false)
    }
  }

  const saveAndNext = () => {
    if (!text.trim()) {
      toast.warn('请先填写文案', 'copy-text')
      return
    }
    draft.patch({ script: text, rewritten: text })
    toast.ok('已保存到草稿', 'copy-save')
    navigate('/m/compose/titles')
  }

  return (
    <div className="cp-lib">
      <header className="cp-lib-head">
        <div className="cp-lib-titles">
          <PageBackLink to="/m/compose" label="工作台" />
          <h1 className="cp-lib-title" style={{ marginTop: 10 }}>文案工作室</h1>
          <p className="cp-lib-lead">模板起稿 · AI 差异化改写 · 口播与种草一站完成</p>
        </div>

        <div className="cp-lib-toolbar">
          <div className="cp-lib-tabs" role="tablist">
            {(
              [
                { key: 'all', label: '全部模板' },
                { key: 'oral', label: '口播' },
                { key: 'graphic', label: '图文' },
                { key: 'mine', label: '我的草稿' },
              ] as const
            ).map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={scope === t.key}
                className={`cp-lib-tab${scope === t.key ? ' is-active' : ''}`}
                onClick={() => setScope(t.key)}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className="cp-lib-actions">
            <Input
              allowClear
              className="cp-lib-search"
              placeholder="搜索模板"
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
            <div className="cp-lib-track">
              <button
                type="button"
                className={`cp-lib-track-btn${!isGraphic ? ' is-active' : ''}`}
                onClick={() => draft.setTrack('video')}
              >
                口播稿
              </button>
              <button
                type="button"
                className={`cp-lib-track-btn${isGraphic ? ' is-active' : ''}`}
                onClick={() => draft.setTrack('graphic')}
              >
                种草图文
              </button>
            </div>
          </div>
        </div>
      </header>

      <div className="cp-lib-layout">
        <section className="cp-lib-templates">
          {scope === 'mine' ? (
            text.trim() ? (
              <ul className="cp-lib-list" role="list">
                <li>
                  <div className="cp-lib-row is-active">
                    <span className="cp-lib-row-icon" aria-hidden>
                      <FileTextOutlined />
                    </span>
                    <div className="cp-lib-row-main">
                      <strong className="cp-lib-row-name">当前草稿</strong>
                      <span className="cp-lib-row-desc">
                        {text.slice(0, 80)}{text.length > 80 ? '…' : ''}
                      </span>
                    </div>
                    <span className="cp-lib-row-tag">我的</span>
                  </div>
                </li>
              </ul>
            ) : (
              <div className="cp-lib-empty">
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无草稿，先选模板或直接开写" />
              </div>
            )
          ) : templates.length === 0 ? (
            <div className="cp-lib-empty">
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有匹配的模板" />
            </div>
          ) : (
            <ul className="cp-lib-list" role="list">
              {templates.map((tpl) => (
                <li key={tpl.id}>
                  <button
                    type="button"
                    className={`cp-lib-row${activeTpl === tpl.id ? ' is-active' : ''}`}
                    onClick={() => applyTemplate(tpl)}
                  >
                    <span className="cp-lib-row-icon" aria-hidden>
                      <FileTextOutlined />
                    </span>
                    <div className="cp-lib-row-main">
                      <strong className="cp-lib-row-name" title={tpl.title}>{tpl.title}</strong>
                      <span className="cp-lib-row-desc">{tpl.desc}</span>
                    </div>
                    <span className="cp-lib-row-tag">{tpl.tag}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="cp-lib-editor">
          <div className="cp-lib-editor-bar">
            <div className="cp-lib-editor-title">
              <EditOutlined />
              <span>{isGraphic ? '图文正文' : '口播正文'}</span>
              {(draft.refTitle || draft.hotPoint) && (
                <em className="cp-lib-ref">
                  对标：{draft.refTitle || '—'}
                  {draft.hotPoint ? ` · ${draft.hotPoint}` : ''}
                </em>
              )}
            </div>
            <Space wrap size={8}>
              <Button
                type="primary"
                className="cp-lib-btn-primary"
                icon={<ThunderboltOutlined />}
                loading={busy}
                disabled={!text.trim()}
                onClick={runRewrite}
              >
                AI 差异化改写
              </Button>
              <Button className="cp-lib-btn-ghost" loading={busy} onClick={runFromTopic}>
                按对标重写
              </Button>
            </Space>
          </div>

          <TextArea
            className="cp-lib-textarea"
            value={text}
            onChange={(e) => {
              setText(e.target.value)
              setActiveTpl(null)
            }}
            placeholder={isGraphic ? '种草笔记 / 图文正文，可点左侧模板起稿' : '口播稿，可点左侧模板起稿'}
            showCount
            maxLength={8000}
          />

          <div className="cp-lib-editor-foot">
            <Button type="primary" className="cp-lib-btn-primary" onClick={saveAndNext}>
              保存并去标题话题
            </Button>
            {isGraphic ? (
              <Button className="cp-lib-btn-ghost" onClick={() => navigate('/m/compose/images')}>去图文配图</Button>
            ) : (
              <Button className="cp-lib-btn-ghost" onClick={() => navigate('/m/compose/voice')}>去音色库</Button>
            )}
            <Button type="link" onClick={() => navigate(isGraphic ? '/m/compose/graphic' : '/m/compose/lipsync')}>
              返回{isGraphic ? '发图文' : '发视频'}总览
            </Button>
          </div>
        </section>
      </div>
    </div>
  )
}

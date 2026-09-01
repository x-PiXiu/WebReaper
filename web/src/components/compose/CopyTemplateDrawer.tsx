import { useMemo, useState } from 'react'
import { Drawer, Empty, Input } from 'antd'
import { FileTextOutlined } from '@ant-design/icons'
import { COPY_TEMPLATES, type CopyTemplate } from '../../data/copyTemplates'

type Props = {
  open: boolean
  track: 'video' | 'graphic'
  onClose: () => void
  onApply: (tpl: CopyTemplate) => void
}

/**
 * 创作向导第一步的模板抽屉（原「文案工作室」模板库的向导内入口）。
 * 按当前轨道过滤（本轨 + 通用模板），支持搜索，点击即套用到正文。
 * 列表复用 copy-lib.css 的 cp-lib-* 样式，与全站模板视觉一致。
 */
export function CopyTemplateDrawer({ open, track, onClose, onApply }: Props) {
  const [q, setQ] = useState('')

  const templates = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return COPY_TEMPLATES.filter((t) => {
      if (t.track !== 'shared' && t.track !== track) return false
      if (!needle) return true
      return (
        t.title.toLowerCase().includes(needle)
        || t.desc.toLowerCase().includes(needle)
        || t.tag.toLowerCase().includes(needle)
      )
    })
  }, [track, q])

  return (
    <Drawer
      title="从模板起稿"
      placement="right"
      width={440}
      open={open}
      onClose={onClose}
      styles={{ body: { paddingTop: 8 } }}
    >
      <Input
        allowClear
        placeholder="搜索模板"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        style={{ marginBottom: 12 }}
      />
      {templates.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有匹配的模板" />
      ) : (
        <ul className="cp-lib-list" role="list">
          {templates.map((tpl) => (
            <li key={tpl.id}>
              <button type="button" className="cp-lib-row" onClick={() => onApply(tpl)}>
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
    </Drawer>
  )
}

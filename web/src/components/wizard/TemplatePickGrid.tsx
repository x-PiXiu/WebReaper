import { ThunderboltOutlined } from '@ant-design/icons'
import type { GenerationTemplate } from '../../types/api'

type Props = {
  templates: GenerationTemplate[]
  selectedId: string
  onSelect: (id: string) => void
  loading?: boolean
}

const MATERIAL_HINT: Record<string, string> = {
  image: '需图片',
  video: '需视频',
  audio: '需音频',
}

/** 模板选择墙（快速生成第一步） */
export function TemplatePickGrid({ templates, selectedId, onSelect, loading }: Props) {
  return (
    <div className={`wz-template-grid${loading ? ' is-loading' : ''}`}>
      <button
        type="button"
        className={`wz-template-card${selectedId === '' ? ' is-active' : ''}`}
        onClick={() => onSelect('')}
      >
        <span className="wz-template-icon"><ThunderboltOutlined /></span>
        <strong>自由生成</strong>
        <span>不传模板，系统根据素材自动判断</span>
      </button>
      {templates.filter(t => t.enabled).map((t) => (
        <button
          key={t.id}
          type="button"
          className={`wz-template-card${selectedId === t.id ? ' is-active' : ''}`}
          onClick={() => onSelect(t.id)}
        >
          <span className="wz-template-icon">{t.icon || '🎬'}</span>
          <strong>{t.name}</strong>
          <span>{t.description}</span>
          {(t.required_materials?.length > 0) && (
            <span className="wz-template-tags">
              {t.required_materials.map((m, i) => (
                <em key={`${t.id}-${m}-${i}`}>{MATERIAL_HINT[m] || m}</em>
              ))}
            </span>
          )}
        </button>
      ))}
    </div>
  )
}

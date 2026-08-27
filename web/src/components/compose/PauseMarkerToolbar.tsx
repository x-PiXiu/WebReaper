import { Button, Space } from 'antd'
import { PAUSE_PRESETS } from '../../utils/pauseMarkers'
import { PauseMarkerPreview } from './PauseMarkerPreview'

type Props = {
  text: string
  onChange: (next: string) => void
  /** 受控光标位置；不传则在文末插入 */
  caret?: number
  onCaretChange?: (pos: number) => void
}

/** 口播文案停顿工具栏：插入 Vidu <#x#> 标记 */
export function PauseMarkerToolbar({ text, onChange, caret, onCaretChange }: Props) {
  const insert = (sec: number) => {
    const pos = caret ?? text.length
    const marker = `<#${sec}#>`
    const next = text.slice(0, pos) + marker + text.slice(pos)
    onChange(next)
    onCaretChange?.(pos + marker.length)
  }

  return (
    <div className="cf-pause-toolbar">
      <Space wrap size={8}>
        {PAUSE_PRESETS.map(p => (
          <Button key={p.sec} size="small" onClick={() => insert(p.sec)}>
            {p.label}
          </Button>
        ))}
      </Space>
      <PauseMarkerPreview text={text} />
    </div>
  )
}

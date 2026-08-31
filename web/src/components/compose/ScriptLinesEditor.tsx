import { Button, Input } from 'antd'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'

type Props = {
  /** '\n' 连接的逐句文案（一行一句） */
  value: string
  onChange: (v: string) => void
  placeholder?: string
}

/**
 * 逐句文案编辑器（23 号计划 §3.1）：一行一句编辑/增删，
 * 为阶段3 B-Roll 的"句=插入点"做心智铺垫——任何处理后的文案都不破坏分行结构。
 * 行内粘贴多行文本会自然拆成多句（TextArea 换行即分句）。
 */
export function ScriptLinesEditor({ value, onChange, placeholder }: Props) {
  const lines = value.split('\n')

  const update = (i: number, text: string) => {
    const next = [...lines]
    next[i] = text
    onChange(next.join('\n'))
  }

  const remove = (i: number) => {
    if (lines.length <= 1) {
      onChange('')
      return
    }
    onChange(lines.filter((_, idx) => idx !== i).join('\n'))
  }

  const add = () => onChange([...lines, ''].join('\n'))

  return (
    <div className="wz-lines-editor">
      {lines.map((line, i) => (
        <div className="wz-lines-row" key={lines.length > 1 ? i : `only-${i}`}>
          <span className="wz-lines-idx" aria-hidden>{i + 1}</span>
          <Input.TextArea
            className="wz-lines-input"
            value={line}
            autoSize={{ minRows: 1, maxRows: 4 }}
            placeholder={placeholder}
            onChange={(e) => update(i, e.target.value)}
          />
          <Button
            type="text"
            size="small"
            className="wz-lines-del"
            icon={<DeleteOutlined />}
            title="删除该句"
            onClick={() => remove(i)}
            disabled={lines.length <= 1 && !line}
          />
        </div>
      ))}
      <Button type="dashed" size="small" className="wz-lines-add" icon={<PlusOutlined />} onClick={add}>
        添加一句
      </Button>
    </div>
  )
}

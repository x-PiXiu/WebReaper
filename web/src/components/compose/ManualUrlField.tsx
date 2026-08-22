import { useState } from 'react'
import { Button, Input } from 'antd'

type Props = {
  value: string
  placeholder?: string
  onChange: (v: string) => void
}

/** 折叠的手动 URL 回填（默认隐藏） */
export function ManualUrlField({ value, placeholder, onChange }: Props) {
  const [open, setOpen] = useState(false)
  if (!open) {
    return (
      <Button type="link" size="small" className="cf-manual-url-toggle" onClick={() => setOpen(true)}>
        手动填入链接
      </Button>
    )
  }
  return (
    <div className="cf-manual-url">
      <Input
        size="small"
        placeholder={placeholder || '粘贴资源 URL'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <Button type="link" size="small" onClick={() => setOpen(false)}>收起</Button>
    </div>
  )
}

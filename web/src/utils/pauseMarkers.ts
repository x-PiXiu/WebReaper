/** Vidu TTS 停顿标记（服务端 buildTTSParams 也会把标点转为 <#x#>） */
export const PAUSE_PRESETS = [
  { label: '短停 0.5s', sec: 0.5 },
  { label: '中停 1s', sec: 1 },
  { label: '长停 2s', sec: 2 },
] as const

export function insertPauseMarker(text: string, seconds: number, caret?: number): { text: string; caret: number } {
  const marker = `<#${seconds}#>`
  const pos = caret ?? text.length
  const next = text.slice(0, pos) + marker + text.slice(pos)
  return { text: next, caret: pos + marker.length }
}

/** 预览：将 <#n#> 转为可读标签（不修改提交内容） */
export function previewPauseMarkers(text: string): string {
  return text.replace(/<#([\d.]+)#>/g, ' ⏸$1s ')
}

export type PauseTextPart =
  | { type: 'text'; value: string }
  | { type: 'pause'; value: string; sec: string }

/** 将文案按停顿标记拆成片段（供高亮预览） */
export function splitPauseMarkers(text: string): PauseTextPart[] {
  if (!text) return []
  const parts: PauseTextPart[] = []
  const re = /<#([\d.]+)#>/g
  let last = 0
  let match: RegExpExecArray | null
  while ((match = re.exec(text)) !== null) {
    if (match.index > last) {
      parts.push({ type: 'text', value: text.slice(last, match.index) })
    }
    parts.push({ type: 'pause', value: match[0], sec: match[1] })
    last = match.index + match[0].length
  }
  if (last < text.length) {
    parts.push({ type: 'text', value: text.slice(last) })
  }
  return parts.length ? parts : [{ type: 'text', value: text }]
}

export function hasPauseMarkers(text: string): boolean {
  return /<#[\d.]+#>/.test(text)
}

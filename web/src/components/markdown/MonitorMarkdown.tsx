import { isValidElement, cloneElement, type ReactNode, type CSSProperties, type ComponentType, type ReactElement } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

// 监测详情 AI 回答：GFM Markdown + 品牌/竞品证据高亮（从 Keywords 页抽出，便于懒加载）。

const BRAND_S = '\uE000'
const BRAND_E = '\uE001'
const COMP_S = '\uE002'
const COMP_E = '\uE003'

function preMark(text: string, brand: string[], competitors: string[]): string {
  const brandWords = [...new Set(brand.filter((w) => w && w.length >= 2))]
  const compWords = [...new Set(competitors.filter((w) => w && w.length >= 2))]
  const words = [...brandWords, ...compWords].sort((a, b) => b.length - a.length)
  if (words.length === 0) return text
  let out = ''
  let rest = text
  while (rest.length > 0) {
    let matched: { word: string; index: number } | null = null
    for (const w of words) {
      const idx = rest.indexOf(w)
      if (idx >= 0 && (matched === null || idx < matched.index)) {
        matched = { word: w, index: idx }
      }
    }
    if (!matched) {
      out += rest
      break
    }
    if (matched.index > 0) out += rest.slice(0, matched.index)
    const isBrand = brandWords.includes(matched.word)
    out += (isBrand ? BRAND_S : COMP_S) + matched.word + (isBrand ? BRAND_E : COMP_E)
    rest = rest.slice(matched.index + matched.word.length)
  }
  return out
}

function splitMarked(text: string, keyBase: number): ReactNode[] {
  const nodes: ReactNode[] = []
  let rest = text
  let key = keyBase
  while (rest.length > 0) {
    let match: { start: number; end: number; mine: boolean } | null = null
    for (const [s, e, mine] of [[BRAND_S, BRAND_E, true], [COMP_S, COMP_E, false]] as const) {
      const i = rest.indexOf(s)
      if (i >= 0 && (match === null || i < match.start)) {
        const j = rest.indexOf(e, i + 1)
        if (j > i) match = { start: i, end: j + 1, mine }
      }
    }
    if (!match) {
      nodes.push(rest)
      break
    }
    if (match.start > 0) nodes.push(rest.slice(0, match.start))
    nodes.push(
      <mark key={key++} className={match.mine ? 'hl-brand' : 'hl-comp'}>
        {rest.slice(match.start + 1, match.end - 1)}
      </mark>
    )
    rest = rest.slice(match.end)
  }
  return nodes
}

function scanHighlight(children: ReactNode, keyBase: number): ReactNode {
  if (typeof children === 'string') return splitMarked(children, keyBase)
  if (Array.isArray(children)) return children.map((c, i) => scanHighlight(c, keyBase + i))
  if (isValidElement(children)) {
    const el = children as ReactElement<{ children?: ReactNode }>
    return cloneElement(el, { children: scanHighlight(el.props.children, keyBase) })
  }
  return children
}

function wrapScan({ children }: { children?: ReactNode }) {
  return <>{scanHighlight(children, 0)}</>
}

const mdComponents: Record<string, ComponentType<{ children?: ReactNode }>> = {
  p: wrapScan, li: wrapScan, h1: wrapScan, h2: wrapScan, h3: wrapScan, h4: wrapScan, h5: wrapScan, h6: wrapScan,
  strong: wrapScan, em: wrapScan, del: wrapScan, blockquote: wrapScan, td: wrapScan, th: wrapScan,
}

export default function MonitorMarkdown({
  text,
  brand,
  competitors,
  style,
}: {
  text: string
  brand: string[]
  competitors: string[]
  style?: CSSProperties
}) {
  return (
    <div className="wr-md" style={style}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents as never}>
        {preMark(text, brand, competitors)}
      </ReactMarkdown>
    </div>
  )
}

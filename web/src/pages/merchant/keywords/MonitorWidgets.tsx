import { Typography } from 'antd'

const { Text } = Typography

/** 情感元信息（emoji + 语义色）。 */
export function sentimentMeta(s: string): { label: string; emoji: string; color: string } {
  if (s === 'positive') return { label: '正面', emoji: '😊', color: 'var(--wr-success)' }
  if (s === 'negative') return { label: '负面', emoji: '😞', color: 'var(--wr-danger)' }
  return { label: '中性', emoji: '😐', color: 'var(--wr-text-muted)' }
}

/** 统计小格：label + 大数字 + 可选 sub。 */
export function StatBlock({ label, value, color, sub }: { label: string; value: string; color?: string; sub?: string }) {
  return (
    <div style={{ padding: '8px 12px', borderRadius: 10, background: 'var(--wr-bg-elevated)', border: '1px solid rgba(124,108,255,0.08)' }}>
      <Text type="secondary" style={{ fontSize: 10, display: 'block', marginBottom: 2 }}>{label}</Text>
      <Text style={{ fontSize: 20, fontWeight: 700, lineHeight: 1.2, color: color || 'var(--wr-text-primary)' }}>{value}</Text>
      {sub && <Text type="secondary" style={{ fontSize: 10, display: 'block', marginTop: 2 }}>{sub}</Text>}
    </div>
  )
}

/** 对比条：品牌/竞品一行（名称 + 进度条 + 百分比）。 */
export function CompareBar({ name, rate, barColor, textColor, nameColor, isMine }: {
  name: string; rate: number; barColor: string; textColor: string; nameColor?: string; isMine?: boolean
}) {
  const pct = Math.min(100, Math.round(rate * 100))
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <span style={{ fontSize: 12, minWidth: 76, fontWeight: isMine ? 700 : 500, color: nameColor || 'var(--wr-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={name}>
        {isMine && <span style={{ marginRight: 4 }}>🛡️</span>}{name}
      </span>
      <div style={{ flex: 1, height: 8, borderRadius: 4, background: 'var(--wr-bg-hover)', overflow: 'hidden' }}>
        <div style={{ width: `${pct}%`, height: '100%', background: barColor, borderRadius: 4, opacity: isMine ? 1 : 0.85, transition: 'width 500ms ease' }} />
      </div>
      <Text style={{ fontSize: 12, minWidth: 42, textAlign: 'right', fontWeight: 700, color: textColor }}>{pct}%</Text>
    </div>
  )
}

/** 把 raw_sample 按【搜索：xxx】问法标记拆成采样气泡。 */
export function splitSamples(raw: string): { header: string; body: string }[] {
  const parts = raw.split(/(?=【搜索：|【问：)/).map((s) => s.trim()).filter(Boolean)
  if (parts.length <= 1) return [{ header: '', body: raw }]
  return parts.map((p) => {
    const nl = p.indexOf('\n')
    const header = nl >= 0 ? p.slice(0, nl).trim() : ''
    const body = nl >= 0 ? p.slice(nl).trim() : p
    return { header, body }
  })
}

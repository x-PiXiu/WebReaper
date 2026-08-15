// GEO 展示层纯函数工具：变化对比（delta）计算。
//
// 设计动机：付费说服力关键在"变化可见"——绝对值让用户看到现状，
// delta 让用户看到"工作在生效"。所有对比逻辑收拢在此，纯函数、可单测、多页面复用。
// 数据契约：传入监测结果列表（按 keyword_id 分组后的原始记录）。

import type { MonitoringResult } from '../types/api'

/** 按探测时间升序排序的监测记录。 */
export function sortByTime(results: MonitoringResult[]): MonitoringResult[] {
  return [...results].sort((a, b) =>
    new Date(a.probed_at).getTime() - new Date(b.probed_at).getTime()
  )
}

/** 最近一次监测结果（无记录返回 null）。 */
export function latestMonitor(results: MonitoringResult[]): MonitoringResult | null {
  const sorted = sortByTime(results)
  return sorted.length > 0 ? sorted[sorted.length - 1] : null
}

/** 最近两次监测的提及率差（百分点，最新 - 上一次；不足两次返回 null）。 */
export function mentionDelta(results: MonitoringResult[]): number | null {
  const sorted = sortByTime(results)
  if (sorted.length < 2) return null
  const latest = sorted[sorted.length - 1].mention_rate
  const prev = sorted[sorted.length - 2].mention_rate
  return Math.round((latest - prev) * 1000) / 10 // 保留 1 位小数
}

export interface DeltaView {
  text: string   // "+12.0%"
  color: string  // 语义色 CSS 变量
  arrow: string  // "↑" / "↓" / "→"
  improved: boolean // 是否向好（上升）
}

/** delta（百分点）→ 展示视图：颜色/箭头语义化。 */
export function deltaView(delta: number | null): DeltaView {
  if (delta === null) {
    return { text: '—', color: 'var(--wr-text-muted)', arrow: '→', improved: false }
  }
  if (delta > 0.05) return { text: `+${delta.toFixed(1)}%`, color: 'var(--wr-success)', arrow: '↑', improved: true }
  if (delta < -0.05) return { text: `${delta.toFixed(1)}%`, color: 'var(--wr-danger)', arrow: '↓', improved: false }
  return { text: '0%', color: 'var(--wr-text-muted)', arrow: '→', improved: false }
}

/** 趋势序列中标记最后一个点（变化点突出用）。返回带 is_last 标记的数组。 */
export function markLastPoint<T>(points: T[]): (T & { is_last: boolean })[] {
  return points.map((p, i) => ({ ...p, is_last: i === points.length - 1 }))
}

/** 提及率 → 语义色（多页共用）。 */
export function rateColor(rate: number): string {
  if (rate >= 0.8) return 'var(--wr-success)'
  if (rate >= 0.5) return 'var(--wr-accent)'
  if (rate >= 0.2) return 'var(--wr-warning)'
  return 'var(--wr-danger)'
}

/** 提及率 → 等级文案。 */
export function rateLabel(rate: number): string {
  if (rate >= 0.8) return '强势'
  if (rate >= 0.5) return '稳定'
  if (rate >= 0.2) return '偶尔'
  return '缺席'
}

/** GEO 内容评分 → 颜色（沿用项目 token，双主题自适应）。 */
export function scoreColor(s: number): string {
  if (s >= 80) return 'var(--wr-success)'
  if (s >= 65) return 'var(--wr-accent)'
  if (s >= 50) return 'var(--wr-warning)'
  return 'var(--wr-danger)'
}

/** GEO 内容评分 → 等级文案。 */
export function scoreLevel(s: number): string {
  if (s >= 80) return 'A 优秀'
  if (s >= 65) return 'B 良好'
  if (s >= 50) return 'C 及格'
  return 'D 待优化'
}

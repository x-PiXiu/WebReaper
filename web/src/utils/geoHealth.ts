import type { MonitoringResult } from '../types/api'

// GEO 健康分本地合成（**降级兜底**——v3 归位后单一事实源是后端 GET /geo/health-report，
// hooks/useHealthReport 优先消费后端报告；仅当接口不可用（旧后端）时走本文件）。
//
// 对齐行业驾驶舱口径（万象镜五指数 × 摘星语义份额驾驶舱）：
//   提及覆盖：品牌被 AI 提到的广度（提及率）
//   情感指数：AI 回答中的正面/负面倾向
//   首选提及：品牌在回答中排第 1 位的比例（位次）
//   内容资产：已发布内容规模（可被 AI 引用的弹药）
//   信源完整度：AI 回答实际引用自营公开站的比例（归因）
// 总分 = 加权合成，0-100；无监测数据时相关项为 0（如实反映"还没开始"）。

export interface HealthIndicators {
  mentionCoverage: number // 0-100
  sentimentScore: number  // 0-100（正面占比加权，负面扣分）
  firstPickRate: number   // 0-100
  contentAsset: number    // 0-100
  sourceIntegrity: number // 0-100
  total: number           // 0-100
}

const W = { coverage: 0.4, sentiment: 0.2, firstPick: 0.2, asset: 0.1, source: 0.1 }

/** 全部品牌最近一次监测结果（跨引擎取最新一条）。 */
export function latestResultsPerBrand(trends: MonitoringResult[][]): MonitoringResult[] {
  return trends
    .filter((list) => list.length > 0)
    .map((list) =>
      [...list].sort((a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime())[0],
    )
}

/**
 * 合成健康分。
 * @param overviews 品牌总览（含 trend）
 * @param contents 全品牌内容列表
 * @param publishedCount 已发布内容数（可从 contents 推导，单独传避免重复计算）
 */
export function computeHealth(
  overviews: Array<{ avg_mention_rate?: number; trend?: MonitoringResult[] }>,
  contents: unknown[],
  publishedCount: number,
): HealthIndicators {
  const ovCount = overviews.length
  const latest = latestResultsPerBrand(overviews.map((o) => o.trend || []))

  // 提及覆盖：平均提及率（无品牌的品牌按 0）
  const avgRate = ovCount > 0
    ? overviews.reduce((s, o) => s + (o.avg_mention_rate || 0), 0) / ovCount
    : 0
  const mentionCoverage = Math.round(avgRate * 100)

  // 情感指数：所有最新结果的 sentiment 聚合（positive +1 / neutral 0 / negative -1 → 0-100）
  let sentSum = 0
  for (const r of latest) {
    if (r.sentiment === 'positive') sentSum += 1
    else if (r.sentiment === 'negative') sentSum -= 1
  }
  const sentimentScore = latest.length > 0
    ? Math.round(((sentSum / latest.length) + 1) * 50)
    : 0

  // 首选提及率（F1-2 可信度门槛，与后端同口径）：真实计数需 ≥3 次采样；旧数据近似需 ≥3 条
  // 有位次结果；不足返回 -1（面板显示"积累中"）——修复 1 条命中即 100% 的矛盾展示
  const sampleSum = latest.reduce((s, r) => s + (r.sample_count || 0), 0)
  const pickSum = latest.reduce((s, r) => s + (r.first_pick_count || 0), 0)
  const ranked = latest.filter((r) => (r.avg_position || 0) > 0)
  let firstPickRate = -1
  if (pickSum > 0 && sampleSum >= 3) {
    firstPickRate = Math.round((pickSum / sampleSum) * 100)
  } else if (pickSum === 0 && ranked.length >= 3) {
    firstPickRate = Math.round((ranked.filter((r) => r.avg_position === 1).length / ranked.length) * 100)
  }

  // 内容资产：已发布内容 + 全部内容的规模（封顶 100）
  const totalCount = contents.length
  const contentAsset = Math.min(100, publishedCount * 15 + (totalCount - publishedCount) * 5)

  // 信源完整度：被 AI 实际引用（自营站来源）的结果占比
  const cited = latest.filter((r) => (r.self_source_count || 0) > 0).length
  const sourceIntegrity = latest.length > 0
    ? Math.round((cited / latest.length) * 100)
    : 0

  const total = Math.round(
    mentionCoverage * W.coverage
    + sentimentScore * W.sentiment
    + Math.max(0, firstPickRate) * W.firstPick // 积累中(-1)按 0 计入——与后端口径一致
    + contentAsset * W.asset
    + sourceIntegrity * W.source,
  )
  return { mentionCoverage, sentimentScore, firstPickRate, contentAsset, sourceIntegrity, total }
}

/**
 * 上一期健康分（"较上期 +3.2" 的数据源——驾驶舱变化叙事）。
 * 用每个品牌 trend 中"7 天前窗口内最新一条"的语义数据重算；无历史返回 null。
 * 说明：健康分本身为前端合成（无服务端持久化），用监测历史序列推导上一期，
 * 保证"变化可见"（付费说服力核心）不依赖新增后端。
 */
export function computeHealthPrev(
  overviews: Array<{ trend?: MonitoringResult[] }>,
  contents: unknown[],
  publishedCount: number,
): number | null {
  const cutoff = Date.now() - 7 * 24 * 3600 * 1000
  const prevOverviews: Array<{ avg_mention_rate?: number; trend?: MonitoringResult[] }> = []
  let hasHistory = false

  for (const o of overviews) {
    const trend = (o.trend || [])
      .filter((t) => t.probed_at && new Date(t.probed_at).getTime() <= cutoff)
      .sort((a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime())
    if (trend.length === 0) {
      prevOverviews.push({ trend: [] })
      continue
    }
    hasHistory = true
    const last = trend[0]
    // 上一期窗口内的平均提及率：取窗口内最后一条
    prevOverviews.push({ avg_mention_rate: last.mention_rate || 0, trend: [last] })
  }

  if (!hasHistory) return null
  return computeHealth(prevOverviews, contents, publishedCount).total
}

/** 总分 → 等级文案与颜色语义（驾驶舱展示用）。 */
export function healthLevel(total: number): { label: string; color: string } {
  if (total >= 80) return { label: '优秀', color: 'var(--wr-success)' }
  if (total >= 60) return { label: '良好', color: 'var(--wr-accent)' }
  if (total >= 40) return { label: '起步', color: 'var(--wr-warning)' }
  return { label: '待建设', color: 'var(--wr-danger)' }
}

/** 每个关键词取最近一次监测结果（跨引擎）。 */
export function latestByKeyword(results: MonitoringResult[]): MonitoringResult[] {
  const sorted = [...results].sort(
    (a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime(),
  )
  const map = new Map<string, MonitoringResult>()
  for (const r of sorted) {
    if (!map.has(r.keyword_id)) map.set(r.keyword_id, r)
  }
  return Array.from(map.values())
}

export interface CompetitorStats {
  selfAvg: number        // 自家平均提及率（0-1）
  compAvg: number        // 竞品平均提及率（0-1）
  gapPct: number         // 差距百分点（+领先/-落后）
  size: number           // 参与聚合的竞品数
  threats: { name: string; avgRate: number; sentiment?: string }[] // 竞品威胁榜（按提及率降序；情感=多数采样观点）
}

/** 竞品对标聚合：最近一次监测中的自家均值 vs 各竞品均值（对齐"付费说服力"坐标系）。 */
export function competitorStats(results: MonitoringResult[]): CompetitorStats {
  const latest = latestByKeyword(results)
  let selfRateSum = 0
  const compMap = new Map<string, { total: number; count: number; sentCounts: Record<string, number> }>()
  for (const r of latest) {
    selfRateSum += r.mention_rate || 0
    Object.entries(r.competitor_rates || {}).forEach(([c, rate]) => {
      const cur = compMap.get(c) || { total: 0, count: 0, sentCounts: {} }
      cur.total += rate
      cur.count += 1
      // 竞品情感（该次回答中的评价）：多数采样观点
      const sent = r.competitor_sentiments?.[c]
      if (sent) cur.sentCounts[sent] = (cur.sentCounts[sent] || 0) + 1
      compMap.set(c, cur)
    })
  }
  const selfAvg = latest.length > 0 ? selfRateSum / latest.length : 0
  const threats = Array.from(compMap.entries())
    .map(([name, c]) => {
      let sentiment: string | undefined
      const sents = Object.entries(c.sentCounts).sort((a, b) => b[1] - a[1])
      if (sents.length > 0 && sents[0][0] !== 'neutral') sentiment = sents[0][0]
      return { name, avgRate: c.count > 0 ? (c.total / c.count) * 100 : 0, sentiment }
    })
    .sort((a, b) => b.avgRate - a.avgRate)
  const compAvg = threats.reduce((s, c) => s + c.avgRate / 100, 0) / Math.max(1, threats.length)
  const gapPct = Math.round((selfAvg - compAvg) * 1000) / 10
  return { selfAvg, compAvg, gapPct, size: threats.length, threats }
}


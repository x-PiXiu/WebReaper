import { useMemo, useState } from 'react'
import type { NavigateFunction } from 'react-router-dom'
import { Card, Row, Col, Table, Tag, Space, Empty, Tooltip, List, Typography, Segmented, Progress } from 'antd'
import {
  TrophyOutlined, RiseOutlined, FallOutlined, BulbOutlined, LinkOutlined, RadarChartOutlined,
} from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { LazyLine as Line, LazyColumn } from '../../../components/charts/LazyCharts'
import { businessApi } from '../../../api/business'
import { deltaView, rateColor, rateLabel, mentionDelta } from '../../../utils/geo'
import { computeHealth, computeHealthPrev, competitorStats, latestByKeyword } from '../../../utils/geoHealth'
import { useHealthReport } from '../../../hooks/useHealthReport'
import HealthScorePanel from '../../../components/HealthScorePanel'
import { useBrandOverviews } from '../../../hooks/useBrandOverviews'
import type { Brand, MonitoringResult, Advice, OptimizedContent } from '../../../types/api'

const { Text, Title } = Typography

// 移动平均降噪：平滑 AI 采样的随机波动
function movingAverage(points: { date: string; rate: number }[], window = 3) {
  return points.map((p, i) => {
    const start = Math.max(0, i - window + 1)
    const slice = points.slice(start, i + 1)
    const avg = slice.reduce((s, x) => s + x.rate, 0) / slice.length
    return { ...p, rate: avg }
  })
}

/** 总览 Tab：GEO 健康驾驶舱 + 行动建议 + 引用归因 + 趋势（时间维度）+ 排行 + 竞品威胁/对标。 */
export default function OverviewTab({
  brands,
  monitorResults,
  navigate,
}: {
  brands: Brand[]
  monitorResults: MonitoringResult[]
  navigate: NavigateFunction
}) {
  const { data: brandsData = brands, isLoading } = useBrandOverviews(brands)
  const overviews = (brandsData as Array<{ brand_id: string; avg_mention_rate?: number; trend?: MonitoringResult[] }>) || []

  // 行动建议（P5-05：针对最需要帮助的品牌）
  const { data: adviceRes } = useQuery({
    queryKey: ['geo-advice-last', overviews.map((o) => o.brand_id).join(',')],
    queryFn: () => {
      if (overviews.length === 0) return { advices: [] as Advice[] }
      const worst = [...overviews].sort((a, b) => (a.avg_mention_rate || 0) - (b.avg_mention_rate || 0))[0]
      if (!worst?.brand_id) return { advices: [] as Advice[] }
      return businessApi.getAdvice(worst.brand_id).catch(() => ({ advices: [] as Advice[] }))
    },
    enabled: overviews.length > 0,
  })
  const advices: Advice[] = adviceRes?.advices || []

  // 时间维度：全部（明细）/ 按天 / 按周（窗口内取最新一次监测值——降采样而非首条）。
  // 修复（H1-②）：分组 key 加入品牌维度——原 key 只有日期，多品牌同日互相覆盖，
  // 折线图每个日期只剩最后遍历品牌的点；"全部"与"按天"原 key 相同属假切换，现真实差异化。
  const [range, setRange] = useState<'' | 'day' | 'week'>('')
  const trendData = useMemo(() => {
    const pad = (n: number) => String(n).padStart(2, '0')
    const dayWindow = (d: Date) => `${d.getFullYear()}-${d.getMonth() + 1}-${pad(d.getDate())}`
    const weekWindow = (d: Date) => `${d.getFullYear()}-W${Math.ceil((d.getDate() + (new Date(d.getFullYear(), d.getMonth(), 1).getDay() || 7) - 1) / 7)}`
    const brandNameById = new Map(brands.map((b) => [b.id, b.name]))
    const byKey = new Map<string, { date: string; ts: number; rate: number; brand: string }>()
    overviews.forEach((o) => {
      const brandName = brandNameById.get(o.brand_id) || '品牌'
      ;(o.trend || []).forEach((t: MonitoringResult) => {
        if (t.mention_rate === undefined || !t.probed_at) return
        const d = new Date(t.probed_at)
        const ts = d.getTime()
        // 全部=每探测点一个点；按天/按周=窗口降采样（key 含品牌，同窗口保留最新一条）
        const key = `${o.brand_id}·${range === 'week' ? weekWindow(d) : range === 'day' ? dayWindow(d) : ts}`
        const prev = byKey.get(key)
        if (prev && prev.ts >= ts) return
        byKey.set(key, {
          date: range === 'week' ? weekWindow(d)
            : range === 'day' ? `${d.getMonth() + 1}-${pad(d.getDate())}`
            : `${d.getMonth() + 1}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`,
          ts,
          rate: Math.round((t.mention_rate || 0) * 1000) / 10,
          brand: brandName,
        })
      })
    })
    return Array.from(byKey.values()).sort((a, b) => a.ts - b.ts)
  }, [overviews, brands, range])

  // 排行（含情感/位次列）。品牌按 brand_id join（修复：原按下标对齐 overviews，
  // 任一品牌 overview 请求失败（hook 会 filter 掉 null）后即品牌名/趋势张冠李戴）
  const ranking = overviews.map((o) => {
    const trend = (o.trend || []) as MonitoringResult[]
    const latest = [...trend].sort((a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime())[0]
    return {
      brand: brands.find((b) => b.id === o.brand_id) || { id: o.brand_id, name: '品牌' },
      overview: o,
      rate: (o.avg_mention_rate || 0) * 100,
      trend,
      latest,
    }
  }).sort((a, b) => b.rate - a.rate).slice(0, 8)

  // 引用归因聚合
  const allResults = monitorResults
  const selfCiteCount = allResults.reduce((s, r) => s + (r.self_source_count || 0), 0)
  const lowConfidenceSamples = allResults.filter((r) => (r.confidence || 0) < 0.4).length
  const sourceSet = new Set<string>()
  allResults.forEach((r) => (r.sources || []).forEach((s) => sourceSet.add(s)))
  const allSources = Array.from(sourceSet).slice(0, 8)

  // 竞品对标：后端健康报告（单一事实源）——报告不可用时降级本地合成（共享纯函数）
  const { report } = useHealthReport()
  const compStats = competitorStats(monitorResults)
  const competitorGap = report
    ? (report.competitor.size > 0
        ? `${report.competitor.gap_pct >= 0 ? '领先' : '落后'} ${Math.abs(report.competitor.gap_pct).toFixed(1)}%`
        : undefined)
    : (compStats.size > 0
        ? `${compStats.gapPct >= 0 ? '领先' : '落后'} ${Math.abs(compStats.gapPct).toFixed(1)}%`
        : undefined)
  const competitorThreats = report
    ? report.competitor.threats.slice(0, 8).map((t) => ({ name: t.name, avgRate: t.avg_rate, sentiment: t.sentiment }))
    : compStats.threats.slice(0, 8)
  const selfAvg = report ? report.competitor.self_avg : compStats.selfAvg

  // 情感分布：全部最新结果的情感占比（正面/中性/负面）
  const sentimentDist = useMemo(() => {
    const latest = latestByKeyword(monitorResults)
    const d = { positive: 0, neutral: 0, negative: 0 }
    latest.forEach((r) => {
      if (r.sentiment === 'positive') d.positive += 1
      else if (r.sentiment === 'negative') d.negative += 1
      else d.neutral += 1
    })
    const total = Math.max(1, latest.length)
    return {
      total: latest.length,
      positive: Math.round((d.positive / total) * 100),
      neutral: Math.round((d.neutral / total) * 100),
      negative: Math.round((d.negative / total) * 100),
    }
  }, [monitorResults])

  // 健康分（前端合成；内容资产维度复用工作台同款扇出，共享缓存）
  const { data: allContents = [] } = useQuery<OptimizedContent[]>({
    queryKey: ['geo-contents-all', brands.map((b) => b.id).join(',')],
    queryFn: async () => {
      const lists = await Promise.all(
        brands.map((b) => businessApi.listContents(b.id).catch(() => [] as OptimizedContent[])),
      )
      return lists.flat()
    },
    enabled: brands.length > 0,
    staleTime: 60_000,
  })
  const publishedCount = allContents.filter((c) => c.status === 'published').length
  // 健康分：后端健康报告（单一事实源）；接口不可用时降级本地合成（geoHealth 兜底）
  const health = report
    ? {
        total: report.total,
        mentionCoverage: report.indicators.mention_coverage,
        sentimentScore: report.indicators.sentiment_score,
        firstPickRate: report.indicators.first_pick_rate,
        contentAsset: report.indicators.content_asset,
        sourceIntegrity: report.indicators.source_integrity,
      }
    : computeHealth(overviews, allContents, publishedCount)
  const prevTotal = report
    ? (report.has_prev ? report.prev_total : null)
    : computeHealthPrev(overviews, allContents, publishedCount)
  const healthDelta = prevTotal === null ? undefined
    : `${health.total - prevTotal >= 0 ? '+' : ''}${(health.total - prevTotal).toFixed(1)}`

  const chartTheme = {
    color: 'var(--wr-text-primary)',
    axis: { common: { labelFill: 'var(--wr-text-muted)', lineStroke: 'var(--wr-border)' } },
  }

  if (isLoading) return <div style={{ minHeight: 200 }} />

  return (
    <div>
      {/* 健康驾驶舱 */}
      <HealthScorePanel
        total={health.total}
        indicators={[
          { label: '提及覆盖', key: 'coverage', value: health.mentionCoverage, hint: '品牌被 AI 提到的广度（平均提及率）', path: '/m/keywords' },
          { label: '情感指数', key: 'sentiment', value: health.sentimentScore, hint: 'AI 回答中的正面倾向（正/负采样聚合）', path: '/m/indexing-report' },
          { label: '首选提及', key: 'firstPick', value: health.firstPickRate, hint: '品牌在 AI 回答里排第 1 位被推荐的比例（需 ≥3 次采样，不足显示"—"积累中）', path: '/m/indexing-report' },
          { label: '内容资产', key: 'asset', value: health.contentAsset, hint: '已发布内容规模（可被 AI 引用的弹药）', path: '/m/content' },
          { label: '信源完整', key: 'source', value: health.sourceIntegrity, hint: 'AI 实际引用你公开站的比例（归因）', path: '/m/content' },
        ]}
        competitorGap={competitorGap}
        deltaText={healthDelta}
        onNavigate={navigate}
      />

      {/* P1-7-1：四个窄卡两列网格（进阶/情感/行动/归因）——压缩纵向密度 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
      <Col xs={24} lg={12}>
      {/* 两阶段叙事：从被收录到被首选引用（进阶路径——效果叙事框架） */}
      <Card className="wr-glass-card" style={{ height: '100%' }}>
        <Space wrap style={{ width: '100%', alignItems: 'center' }}>
          <Text strong style={{ fontSize: 14 }}>进阶路径</Text>
          <div style={{ flex: 1, minWidth: 260, display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                <Text style={{ fontSize: 12, color: 'var(--wr-text-secondary)' }}>入门 · 被收录</Text>
                <Text style={{ fontSize: 12, fontWeight: 600 }}>{health.mentionCoverage}%</Text>
              </div>
              <Progress percent={health.mentionCoverage} size="small" showInfo={false} strokeColor="var(--wr-accent)" />
            </div>
            <Text type="secondary" style={{ fontSize: 14 }}>→</Text>
            <div style={{ flex: 1 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                <Text style={{ fontSize: 12, color: 'var(--wr-text-secondary)' }}>进阶 · 被首选引用</Text>
                <Text style={{ fontSize: 12, fontWeight: 600 }}>{health.firstPickRate < 0 ? '积累中' : `${health.firstPickRate}%`}</Text>
              </div>
              <Progress percent={Math.max(0, health.firstPickRate)} size="small" showInfo={false} strokeColor="var(--wr-success)" />
            </div>
          </div>
          <Tag style={{ margin: 0, fontSize: 11 }}>首选引用 = AI 回答里第 1 位推荐你</Tag>
        </Space>
      </Card>

      </Col>
      <Col xs={24} lg={12}>
      {/* 情感分布：AI 怎么说你（正面/中性/负面占比） */}
      <Card className="wr-glass-card" style={{ height: '100%' }}>
        <Space wrap style={{ width: '100%' }}>
          <Text strong style={{ fontSize: 14 }}>情感分布</Text>
          {sentimentDist.total === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>暂无采样——发起监测后这里会显示 AI 回答的态度倾向</Text>
          ) : (
            <>
              <div style={{ flex: 1, minWidth: 220, display: 'flex', gap: 16, alignItems: 'center' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                    <Text style={{ fontSize: 12, color: 'var(--wr-success)' }}>正面</Text>
                    <Text style={{ fontSize: 12, color: 'var(--wr-success)' }}>{sentimentDist.positive}%</Text>
                  </div>
                  <Progress percent={sentimentDist.positive} size="small" showInfo={false} strokeColor="var(--wr-success)" />
                </div>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                    <Text style={{ fontSize: 12, color: 'var(--wr-text-muted)' }}>中性</Text>
                    <Text style={{ fontSize: 12, color: 'var(--wr-text-muted)' }}>{sentimentDist.neutral}%</Text>
                  </div>
                  <Progress percent={sentimentDist.neutral} size="small" showInfo={false} strokeColor="var(--wr-text-muted)" />
                </div>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                    <Text style={{ fontSize: 12, color: 'var(--wr-danger)' }}>负面</Text>
                    <Text style={{ fontSize: 12, color: 'var(--wr-danger)' }}>{sentimentDist.negative}%</Text>
                  </div>
                  <Progress percent={sentimentDist.negative} size="small" showInfo={false} strokeColor="var(--wr-danger)" />
                </div>
              </div>
              <Tag style={{ margin: 0, fontSize: 11 }}>
                基于 {sentimentDist.total} 条采样（AI 回答中提到你的部分）
              </Tag>
            </>
          )}
        </Space>
      </Card>

      {/* 行动建议：给老板"下一步做什么" */}
      {advices.length > 0 && (
        <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
          <Space style={{ marginBottom: 8 }}>
            <BulbOutlined style={{ color: 'var(--wr-warning)' }} />
            <Text strong style={{ fontSize: 14 }}>行动建议 · 下一步做什么</Text>
            <Text type="secondary" style={{ fontSize: 12 }}>针对最需要帮助的品牌</Text>
          </Space>
          <List
            size="small"
            dataSource={advices}
            renderItem={(a: Advice) => (
              <List.Item actions={a.page ? [<a key="go" onClick={() => navigate(a.page)}>去做 →</a>] : undefined}>
                <Space>
                  <Tag color={a.level === 'high' ? 'error' : a.level === 'medium' ? 'warning' : 'default'} style={{ margin: 0, fontSize: 11 }}>
                    {a.level === 'high' ? '优先' : a.level === 'medium' ? '建议' : '保持'}
                  </Tag>
                  <Text style={{ fontSize: 13 }}>{a.message}</Text>
                </Space>
              </List.Item>
            )}
          />
        </Card>
      )}
      </Col>
      <Col xs={24} lg={24}>
      {/* 引用归因：AI 提到你 ≠ 引用你的内容 */}
      <Card className="wr-glass-card" style={{ height: '100%' }}>
        <Space wrap>
          <LinkOutlined style={{ color: 'var(--wr-accent)' }} />
          <Text strong style={{ fontSize: 14 }}>引用归因</Text>
          <Tag color={selfCiteCount > 0 ? 'success' : 'default'} style={{ margin: 0 }}>
            你的内容被 AI 引用 {selfCiteCount} 次
          </Tag>
          {lowConfidenceSamples > 0 && (
            <Text type="secondary" style={{ fontSize: 12 }}>（{lowConfidenceSamples} 条结果采样不足，置信度低）</Text>
          )}
          <Tooltip title="AI 提到品牌 ≠ 引用了你的内容。此数字 = AI 回答中列出的来源里包含你公开站域名的次数——内容 GEO 的直接效果证据。">
            <span className="wr-help-tip">?</span>
          </Tooltip>
        </Space>
        {allSources.length > 0 && (
          <div style={{ marginTop: 10 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>AI 回答中出现的来源：</Text>
            <div style={{ marginTop: 6 }}>
              <Space wrap size={[6, 6]}>
                {allSources.map((s) => <Tag key={s} style={{ fontSize: 11 }}>{s}</Tag>)}
              </Space>
            </div>
          </div>
        )}
      </Card>
      </Col>
      </Row>

      {/* 提及率趋势（时间维度切换：明细 / 按天） */}
      <Card className="wr-glass-card" styles={{ body: { padding: 20 } }} style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 0, fontSize: 14 }}>
            提及率趋势
          </Title>
          <Segmented
            size="small"
            value={range}
            onChange={(v) => setRange(v as '' | 'day' | 'week')}
            options={[{ label: '全部', value: '' }, { label: '按天', value: 'day' }, { label: '按周', value: 'week' }]}
          />
        </div>
        {trendData.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无监测数据——去「监测矩阵」发起任务" />
        ) : (
          <Line
            data={movingAverage(trendData)}
            xField="date" yField="rate"
            seriesField="brand"
            smooth
            height={280}
            theme={chartTheme as any}
            color={['#6366f1', '#ec4899', '#10b981', '#f59e0b', '#06b6d4']}
            point={{ size: 3, shape: 'circle' }}
            yAxis={{ min: 0, max: 100, label: { formatter: (v: string) => v + '%' } }}
            tooltip={{ name: '提及率', formatter: (d: any) => ({ name: d.brand, value: d.rate + '%' }) }}
          />
        )}
      </Card>

      <Row gutter={[16, 16]}>
        {/* 品牌可见度排行（含情感/位次） */}
        <Col xs={24} lg={14}>
          <Card className="wr-glass-card" styles={{ body: { padding: 0 } }}>
            <div style={{ padding: '16px 20px 0' }}>
              <Space>
                <TrophyOutlined style={{ color: 'var(--wr-warning)' }} />
                <Text strong style={{ fontSize: 14 }}>品牌可见度排行</Text>
              </Space>
            </div>
            <Table
              dataSource={ranking}
              rowKey={(r) => r.brand.id}
              size="small"
              pagination={false}
              style={{ marginTop: 8 }}
              columns={[
                {
                  title: '#', width: 50, align: 'center',
                  render: (_: unknown, __: unknown, idx: number) => (
                    <Text strong style={{ color: idx === 0 ? 'var(--wr-warning)' : idx < 3 ? 'var(--wr-accent)' : 'var(--wr-text-muted)' }}>{idx + 1}</Text>
                  ),
                },
                {
                  title: '品牌', dataIndex: ['brand', 'name'], key: 'name',
                  render: (name: string, r: any) => (
                    <Space direction="vertical" size={0}>
                      <Text strong>{name}</Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>{r.overview?.keyword_count ?? 0} 个关键词</Text>
                    </Space>
                  ),
                },
                {
                  title: '提及率', key: 'rate', width: 110, align: 'center',
                  render: (_: unknown, r: any) => (
                    <div style={{ textAlign: 'center' }}>
                      <Text strong style={{ fontSize: 18, color: rateColor(r.rate / 100) }}>{(r.rate).toFixed(0)}%</Text>
                      <div><Tag style={{ fontSize: 10, margin: 0 }}>{rateLabel(r.rate / 100)}</Tag></div>
                    </div>
                  ),
                },
                {
                  title: '情感', key: 'sentiment', width: 80, align: 'center',
                  render: (_: unknown, r: any) => {
                    const s = r.latest?.sentiment
                    if (!s || s === 'neutral') return <Text type="secondary" style={{ fontSize: 12 }}>中性</Text>
                    return <Tag color={s === 'positive' ? 'success' : 'error'} style={{ margin: 0 }}>{s === 'positive' ? '正面' : '负面'}</Tag>
                  },
                },
                {
                  title: '变化', key: 'delta', width: 90, align: 'center',
                  render: (_: unknown, r: any) => {
                    // 复用共享纯函数（内部先按 probed_at 排序）——不再依赖 trend 返回顺序
                    const delta = mentionDelta(r.trend)
                    if (delta === null) return <Text type="secondary">—</Text>
                    const dv = deltaView(delta)
                    return (
                      <Tooltip title="最新 vs 上一次监测">
                        <Text style={{ color: dv.color, fontSize: 13 }}>
                          {delta > 0 ? <RiseOutlined /> : delta < 0 ? <FallOutlined /> : null} {dv.text}
                        </Text>
                      </Tooltip>
                    )
                  },
                },
                {
                  title: '操作', key: 'action', width: 90,
                  render: () => (
                    <a onClick={() => navigate('/m/indexing-report')} style={{ fontSize: 12 }}>查看详情 →</a>
                  ),
                },
              ]}
            />
          </Card>
        </Col>

        {/* 竞品威胁榜（含自家 vs 竞品并排对标） */}
        <Col xs={24} lg={10}>
          <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
            <Space style={{ marginBottom: 12 }}>
              <RadarChartOutlined style={{ color: 'var(--wr-danger)' }} />
              <Text strong style={{ fontSize: 14 }}>竞品对标</Text>
            </Space>
            {competitorThreats.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无竞品数据（监测时会自动识别）" />
            ) : (
              <>
                {/* 你 vs Top 竞品并排柱图（提及率%）——差距一眼可见 */}
                <LazyColumn
                  data={[
                    { platform: '你', rate: Math.round(selfAvg * 1000) / 10 },
                    ...competitorThreats.slice(0, 5).map((c) => ({ platform: c.name.length > 6 ? c.name.slice(0, 6) + '…' : c.name, rate: Math.round(c.avgRate * 10) / 10 })),
                  ]}
                  xField="platform"
                  yField="rate"
                  height={180}
                  style={{
                    fill: 'l(270) 0:#7c6cff 1:#22d3ee',
                    radiusTopLeft: 4,
                    radiusTopRight: 4,
                  }}
                  label={{
                    text: (d: { rate: number }) => `${d.rate}%`,
                    position: 'top',
                    style: { fill: 'var(--wr-text-secondary)', fontSize: 10 },
                  }}
                  axis={{ y: { title: false }, x: { title: false } }}
                />
                <Space direction="vertical" size={10} style={{ width: '100%', marginTop: 12 }}>
                  {competitorThreats.map((c) => (
                    <div key={c.name}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                        <Space size={6}>
                          <Text style={{ fontSize: 13 }}>{c.name}</Text>
                          {c.sentiment === 'positive' && <Tag color="success" style={{ margin: 0, fontSize: 10 }}>被推荐</Tag>}
                          {c.sentiment === 'negative' && <Tag color="error" style={{ margin: 0, fontSize: 10 }}>被批评</Tag>}
                        </Space>
                        <Text strong style={{ fontSize: 13, color: c.avgRate > selfAvg * 100 ? 'var(--wr-danger)' : 'var(--wr-text-secondary)' }}>
                          {c.avgRate.toFixed(0)}%
                        </Text>
                      </div>
                      <div style={{ height: 6, background: 'var(--wr-bg-elevated)', borderRadius: 3, overflow: 'hidden' }}>
                        <div style={{ height: '100%', width: `${Math.min(100, c.avgRate)}%`, background: c.avgRate > selfAvg * 100 ? 'var(--wr-danger)' : 'var(--wr-accent)', borderRadius: 3 }} />
                      </div>
                    </div>
                  ))}
                  <Text type="secondary" style={{ fontSize: 11, display: 'block', paddingTop: 4 }}>
                    你的平均提及率 {Math.round(selfAvg * 100)}%（柱图中的「你」）· 情感标签 = AI 回答中的评价倾向
                  </Text>
                </Space>
              </>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}

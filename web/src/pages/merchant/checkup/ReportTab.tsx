import { useMemo, useState } from 'react'
import { Card, Empty, List, Segmented, Space, Tag, Typography } from 'antd'
import { BulbOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { LazyLine as Line } from '../../../components/charts/LazyCharts'
import { businessApi } from '../../../api/business'
import { computeHealth, computeHealthPrev } from '../../../utils/geoHealth'
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

/**
 * 体检报告 Tab（傻瓜化：从 7 块砍到 3 块——健康分 / 行动建议 / 趋势）。
 * 进阶路径、情感分布、引用归因、品牌排行、竞品对标降级：
 * 引用明细 → 体检记录 Tab；竞品/情感 → 品牌档案·竞品对比；排行 → 工作台品牌卡。
 * 每个指标带"怎么算的"溯源说明（数据口径 Popover 范式）。
 */
export default function ReportTab({
  brands,
  navigate,
  goAsk,
}: {
  brands: Brand[]
  navigate: (path: string) => void
  goAsk: () => void
}) {
  const { data: brandsData = brands, isLoading } = useBrandOverviews(brands)
  const overviews = (brandsData as Array<{ brand_id: string; avg_mention_rate?: number; trend?: MonitoringResult[] }>) || []

  // 行动建议（针对最需要帮助的品牌）
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

  // 趋势（时间维度降采样：窗口内取最新一次）
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

  // 健康分：后端健康报告（单一事实源）；接口不可用时降级本地合成
  const { report } = useHealthReport()
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
  const competitorGap = report
    ? (report.competitor.size > 0
        ? `${report.competitor.gap_pct >= 0 ? '领先' : '落后'} ${Math.abs(report.competitor.gap_pct).toFixed(1)}%`
        : undefined)
    : undefined

  const chartTheme = {
    color: 'var(--wr-text-primary)',
    axis: { common: { labelFill: 'var(--wr-text-muted)', lineStroke: 'var(--wr-border)' } },
  }

  if (isLoading) return <div style={{ minHeight: 200 }} />

  return (
    <div>
      {/* ① 健康分（总分 + 五指数 + 环比 + 竞品差距——口径说明全在 Popover） */}
      <HealthScorePanel
        total={health.total}
        indicators={[
          { label: 'AI 提到你', key: 'coverage', value: health.mentionCoverage, hint: 'AI 有多常提到你——近期所有监测的平均提及率', path: '/m/checkup?tab=records' },
          { label: '态度', key: 'sentiment', value: health.sentimentScore, hint: 'AI 提到你时是夸你（正面）还是中性/批评', path: '/m/checkup?tab=records' },
          { label: '首选推荐', key: 'firstPick', value: health.firstPickRate, hint: 'AI 第一个推荐你的比例（数据不足时显示"—"）', path: '/m/checkup?tab=records' },
          { label: '内容资产', key: 'asset', value: health.contentAsset, hint: '已发布内容的规模——AI 可引用的素材（发布 1 篇顶 3 篇草稿）', path: '/m/studio' },
          { label: '信源完整', key: 'source', value: health.sourceIntegrity, hint: 'AI 回答列出的来源里，你自己网站占多少', path: '/m/studio' },
        ]}
        competitorGap={competitorGap}
        deltaText={healthDelta}
        onNavigate={navigate}
      />

      {/* ② 行动建议（傻瓜化骨架"下一步"——空态也给明确动作，不再条件消失） */}
      <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
        <Space style={{ marginBottom: 8 }}>
          <BulbOutlined style={{ color: 'var(--wr-warning)' }} />
          <Text strong style={{ fontSize: 14 }}>下一步做什么</Text>
          {advices.length > 0 && <Text type="secondary" style={{ fontSize: 12 }}>针对你最需要提升的地方</Text>}
        </Space>
        {advices.length > 0 ? (
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
        ) : (
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', padding: '8px 0' }}>
            <Text type="secondary" style={{ fontSize: 13 }}>还没有足够数据生成建议——先问 AI 几个问题，测出基线</Text>
            <ThunderboltOutlined style={{ color: 'var(--wr-primary)' }} />
            <a onClick={goAsk} style={{ fontSize: 13 }}>去问问 AI →</a>
          </div>
        )}
      </Card>

      {/* ③ 趋势（"AI 提到你"的变化——移动平均平滑，口径说明） */}
      <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <Space>
            <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 0, fontSize: 14 }}>
              AI 提到你的趋势
            </Title>
            <Tag style={{ fontSize: 10, margin: 0 }}>数据怎么来的：每次体检的提及率，3 点移动平均</Tag>
          </Space>
          <Segmented
            size="small"
            value={range}
            onChange={(v) => setRange(v as '' | 'day' | 'week')}
            options={[{ label: '全部', value: '' }, { label: '按天', value: 'day' }, { label: '按周', value: 'week' }]}
          />
        </div>
        {trendData.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无体检数据——去「问问 AI」测第一题" style={{ padding: 32 }}>
            <a onClick={goAsk}>去问问 AI →</a>
          </Empty>
        ) : (
          <Line
            data={movingAverage(trendData)}
            xField="date" yField="rate"
            seriesField="brand"
            smooth
            height={260}
            theme={chartTheme as any}
            color={['#6366f1', '#ec4899', '#10b981', '#f59e0b', '#06b6d4']}
            point={{ size: 3, shape: 'circle' }}
            yAxis={{ min: 0, max: 100, label: { formatter: (v: string) => v + '%' } }}
            tooltip={{ name: 'AI 提到你', formatter: (d: any) => ({ name: d.brand, value: d.rate + '%' }) }}
          />
        )}
      </Card>
    </div>
  )
}

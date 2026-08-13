import { Typography, Card, Row, Col, Table, Tag, Space, Empty, Spin, Tooltip, List } from 'antd'
import { RadarChartOutlined, TrophyOutlined, RiseOutlined, FallOutlined, BulbOutlined, LinkOutlined } from '@ant-design/icons'
import { LazyLine as Line } from '../../components/charts/LazyCharts'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { deltaView, rateColor, rateLabel } from '../../utils/geo'
import type { Brand, MonitoringResult, Advice } from '../../types/api'

const { Title, Text } = Typography

// P5-04 趋势降噪：对提及率时间序列做移动平均（窗口 3），
// 平滑 AI 采样的随机波动，让老板看到"趋势"而非"噪声"。
function movingAverage(points: { date: string; rate: number }[], window = 3) {
  return points.map((p, i) => {
    const start = Math.max(0, i - window + 1)
    const slice = points.slice(start, i + 1)
    const avg = slice.reduce((s, x) => s + x.rate, 0) / slice.length
    return { ...p, rate: avg }
  })
}

// AI 可见度总览（商户端一级入口）：
// 跨品牌提及率排行 + 趋势对比 + 竞品威胁。
// 数据源：brands + overviews（与首页同源接口，无新后端依赖）。
export default function Visibility() {
  const navigate = useNavigate()

  const { data: brands = [], isLoading } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const overviews = useQuery({
    queryKey: ['geo-overviews', brands.map((b: Brand) => b.id).join(',')],
    queryFn: async () => {
      const results = await Promise.all(
        brands.map((b: Brand) => businessApi.getBrandOverview(b.id, b.name).catch(() => null))
      )
      return results.filter(Boolean) as any[]
    },
    enabled: brands.length > 0,
  })

  // 全部监测结果（用于竞品聚合）
  const { data: allResults = [] } = useQuery({
    queryKey: ['all-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch(() => []),
    enabled: brands.length > 0,
  })

  // 行动建议（P5-05）：针对排名最末（最需要帮助的）品牌生成"下一步做什么"
  // 注意：必须在条件 return 之前声明（React Hooks 规则——与 Home.tsx 同款修复）
  const { data: adviceRes } = useQuery({
    queryKey: ['geo-advice-last', overviews.data?.map((o: any) => o.brand_id).join(',')],
    queryFn: () => {
      const ov = (overviews.data || []) as any[]
      if (ov.length === 0) return { advices: [] as Advice[] }
      const worst = [...ov].sort((a, b) => (a.avg_mention_rate || 0) - (b.avg_mention_rate || 0))[0]
      if (!worst?.brand_id) return { advices: [] as Advice[] }
      return businessApi.getAdvice(worst.brand_id).catch(() => ({ advices: [] as Advice[] }))
    },
    enabled: (overviews.data?.length || 0) > 0,
  })
  const advices: Advice[] = adviceRes?.advices || []

  if (isLoading) {
    return <div style={{ display: 'flex', justifyContent: 'center', padding: 120 }}><Spin size="large" /></div>
  }

  if (brands.length === 0) {
    return (
      <div className="wr-page-content">
        <Empty description="还没有品牌——创建品牌后即可查看 AI 可见度">
          <a onClick={() => navigate('/m/brands')}>去创建品牌 →</a>
        </Empty>
      </div>
    )
  }

  const ovData = overviews.data || []

  // 排行榜数据：按提及率降序
  const ranking = ovData.map((o: any, i: number) => ({
    brand: brands[i],
    overview: o,
    rate: o.avg_mention_rate || 0,
    trend: (o.trend || []) as MonitoringResult[],
  })).sort((a, b) => b.rate - a.rate)

  // P5-04 趋势降噪：按品牌分组做移动平均（平滑 AI 采样随机波动）
  const trendData: any[] = []
  ranking.forEach(({ brand, trend }) => {
    const points = trend
      .filter((t) => t.mention_rate !== undefined && t.probed_at)
      .map((t) => ({
        date: new Date(t.probed_at).toLocaleDateString(),
        rate: Math.round((t.mention_rate || 0) * 1000) / 10,
      }))
      .sort((a, b) => (a.date < b.date ? -1 : 1))
    movingAverage(points, 3).forEach((p) => trendData.push({ ...p, brand: brand.name }))
  })

  // P5-01 归因统计：AI 回答引用的来源（去重）+ 自营内容被引用总次数
  const sourceSet = new Set<string>()
  let selfCiteCount = 0
  let lowConfidenceSamples = 0
  allResults.forEach((r: MonitoringResult) => {
    r.sources?.forEach((s) => sourceSet.add(s))
    selfCiteCount += r.self_source_count || 0
    if (r.sample_count > 0 && r.sample_count < 3) lowConfidenceSamples++
  })
  const allSources = Array.from(sourceSet).slice(0, 8)

  // 竞品威胁聚合：所有监测结果里出现的竞品及其平均提及率
  const competitorMap = new Map<string, { total: number; count: number }>()
  allResults.forEach((r: MonitoringResult) => {
    r.competitors?.forEach((c: string) => {
      const rate = r.competitor_rates?.[c] || 0
      const cur = competitorMap.get(c) || { total: 0, count: 0 }
      cur.total += rate
      cur.count += 1
      competitorMap.set(c, cur)
    })
  })
  const competitorThreats = Array.from(competitorMap.entries())
    .map(([name, v]) => ({ name, avgRate: v.total / v.count }))
    .sort((a, b) => b.avgRate - a.avgRate)
    .slice(0, 8)

  const chartTheme = {
    color: 'var(--wr-text-primary)',
    axis: { common: { labelFill: 'var(--wr-text-muted)', lineStroke: 'var(--wr-border)' } },
  }

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1><RadarChartOutlined style={{ marginRight: 8 }} />可见度报表</h1>
        <p>
          你的品牌在 AI 搜索引擎中被提及的情况 · 跨品牌排行 · 竞品对比
          <Tooltip title="提及率 = AI 回答中提到你品牌的次数 ÷ 总采样次数。如 10 次提问里 AI 3 次提到你 = 30%。越高说明你在 AI 时代的曝光越强。">
            <span className="wr-help-tip">?</span>
          </Tooltip>
        </p>
      </div>

      {/* P5-05 行动建议：给老板"下一步做什么"（针对最需要帮助的品牌） */}
      {advices.length > 0 && (
        <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
          <Space style={{ marginBottom: 8 }}>
            <BulbOutlined style={{ color: 'var(--wr-warning)' }} />
            <Text strong style={{ fontSize: 14 }}>行动建议 · 下一步做什么</Text>
            {ranking.length > 0 && (
              <Text type="secondary" style={{ fontSize: 12 }}>基于「{ranking[ranking.length - 1]?.brand?.name}」的现状</Text>
            )}
          </Space>
          <List
            size="small"
            dataSource={advices}
            renderItem={(a: Advice) => (
              <List.Item
                actions={a.page ? [<a key="go" onClick={() => navigate(a.page)}>去做 →</a>] : undefined}
              >
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

      {/* P5-01 归因卡片：AI 提到你 ≠ 引用你的内容 */}
      <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
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

      {/* 提及率趋势对比（全品牌）*/}
      <Card className="wr-glass-card" styles={{ body: { padding: 20 } }} style={{ marginBottom: 16 }}>
        <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 16, fontSize: 14 }}>
          提及率趋势对比
        </Title>
        {trendData.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无监测数据——前往平台收录报表发起任务" />
        ) : (
          <Line
            data={trendData}
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
        {/* 品牌可见度排行榜 */}
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
                  title: '提及率', key: 'rate', width: 120, align: 'center',
                  sorter: (a: any, b: any) => b.rate - a.rate,
                  defaultSortOrder: 'ascend',
                  render: (_: unknown, r: any) => (
                    <div style={{ textAlign: 'center' }}>
                      <Text strong style={{ fontSize: 18, color: rateColor(r.rate) }}>{(r.rate * 100).toFixed(0)}%</Text>
                      <div><Tag style={{ fontSize: 10, margin: 0 }}>{rateLabel(r.rate)}</Tag></div>
                    </div>
                  ),
                },
                {
                  title: '变化', key: 'delta', width: 90, align: 'center',
                  render: (_: unknown, r: any) => {
                    const trend = r.trend.filter((t: MonitoringResult) => t.mention_rate !== undefined)
                    if (trend.length < 2) return <Text type="secondary">—</Text>
                    const delta = (trend[trend.length - 1].mention_rate - trend[trend.length - 2].mention_rate) * 100
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

        {/* 竞品威胁 */}
        <Col xs={24} lg={10}>
          <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
            <Space style={{ marginBottom: 16 }}>
              <TrophyOutlined style={{ color: 'var(--wr-danger)' }} />
              <Text strong style={{ fontSize: 14 }}>竞品威胁榜</Text>
            </Space>
            {competitorThreats.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无竞品数据（监测时会自动识别）" />
            ) : (
              <Space direction="vertical" size={10} style={{ width: '100%' }}>
                {competitorThreats.map((c, i) => (
                  <div key={c.name} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <Text style={{ color: i < 3 ? 'var(--wr-danger)' : 'var(--wr-text-muted)', fontWeight: 600, width: 20 }}>{i + 1}</Text>
                    <Text ellipsis style={{ flex: 1, fontSize: 13 }}>{c.name}</Text>
                    <div style={{ width: 100, height: 6, borderRadius: 3, background: 'var(--wr-bg-base)', overflow: 'hidden' }}>
                      <div style={{ width: `${Math.min(100, c.avgRate * 100)}%`, height: '100%', background: rateColor(c.avgRate), borderRadius: 3 }} />
                    </div>
                    <Text strong style={{ color: rateColor(c.avgRate), width: 44, textAlign: 'right', fontSize: 13 }}>{(c.avgRate * 100).toFixed(0)}%</Text>
                  </div>
                ))}
              </Space>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}

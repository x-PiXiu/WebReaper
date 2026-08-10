import { Typography, Card, Row, Col, Table, Tag, Space, Empty, Spin, Tooltip } from 'antd'
import { RadarChartOutlined, TrophyOutlined, RiseOutlined, FallOutlined } from '@ant-design/icons'
import { Line } from '@ant-design/charts'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { deltaView } from '../../utils/geo'
import type { Brand, MonitoringResult } from '../../types/api'

const { Title, Text } = Typography

function rateColor(rate: number): string {
  if (rate >= 0.8) return 'var(--wr-success)'
  if (rate >= 0.5) return 'var(--wr-accent)'
  if (rate >= 0.2) return 'var(--wr-warning)'
  return 'var(--wr-danger)'
}

function rateLabel(rate: number): string {
  if (rate >= 0.8) return '强势'
  if (rate >= 0.5) return '稳定'
  if (rate >= 0.2) return '偶尔'
  return '缺席'
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
    queryKey: ['geo-overviews-vis', brands.map((b: Brand) => b.id).join(',')],
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

  // 趋势对比图数据（所有品牌的提及率时间序列）
  const trendData: any[] = []
  ranking.forEach(({ brand, trend }) => {
    trend.forEach((t: MonitoringResult) => {
      if (t.mention_rate !== undefined && t.probed_at) {
        trendData.push({
          date: new Date(t.probed_at).toLocaleDateString(),
          rate: Math.round((t.mention_rate || 0) * 1000) / 10,
          brand: brand.name,
        })
      }
    })
  })

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
        <h1><RadarChartOutlined style={{ marginRight: 8 }} />AI 可见度</h1>
        <p>
          你的品牌在 AI 搜索引擎中被提及的情况 · 跨品牌排行 · 竞品对比
          <Tooltip title="提及率 = AI 回答中提到你品牌的次数 ÷ 总采样次数。如 10 次提问里 AI 3 次提到你 = 30%。越高说明你在 AI 时代的曝光越强。">
            <span className="wr-help-tip">?</span>
          </Tooltip>
        </p>
      </div>

      {/* 提及率趋势对比（全品牌）*/}
      <Card className="wr-glass-card" styles={{ body: { padding: 20 } }} style={{ marginBottom: 16 }}>
        <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 16, fontSize: 14 }}>
          提及率趋势对比
        </Title>
        {trendData.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无监测数据——前往关键词管理发起监测" />
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
                      <Text type="secondary" style={{ fontSize: 11 }}>{r.keywordCount} 个关键词</Text>
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
                    <a onClick={() => navigate('/m/keywords')} style={{ fontSize: 12 }}>查看详情 →</a>
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

import { Card, Typography, Row, Col, Spin, Tag, Button, Switch, Space, message } from 'antd'
import { RocketOutlined, ArrowRightOutlined, AppstoreAddOutlined, SearchOutlined, RadarChartOutlined, FileTextOutlined } from '@ant-design/icons'
import { Line, Pie } from '@ant-design/charts'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { deltaView, latestMonitor } from '../../utils/geo'
import type { Brand } from '../../types/api'

const { Title, Text } = Typography

// 空状态「10 分钟快速见效」引导步骤
const QUICK_START_STEPS = [
  { key: 'brand', title: '创建品牌', desc: '填写定位/卖点/竞品', path: '/m/brands', icon: <AppstoreAddOutlined /> },
  { key: 'keyword', title: '添加关键词', desc: 'AI 生成或蒸馏获取', path: '/m/keywords', icon: <SearchOutlined /> },
  { key: 'monitor', title: '立即监测', desc: '看 AI 怎么评价你', path: '/m/keywords', icon: <RadarChartOutlined /> },
  { key: 'content', title: '生成内容', desc: '优化 AI 可见度', path: '/m/content', icon: <FileTextOutlined /> },
]

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

// 数据驾驶舱：品牌可见度总览（Linear 风大屏感）。
// 数据源：brands + 各品牌 overview（租户级已有接口组合，无新后端依赖）。
export default function MerchantHome() {
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
      return results.filter(Boolean)
    },
    enabled: brands.length > 0,
  })

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
        <Spin size="large" />
      </div>
    )
  }

  if (brands.length === 0) {
    return (
      <div className="wr-page-content" style={{ paddingTop: 80 }}>
        <div className="wr-glass-card" style={{ padding: 48, maxWidth: 860, margin: '0 auto' }}>
          <div style={{ textAlign: 'center', marginBottom: 40 }}>
            <div style={{
              width: 72, height: 72, borderRadius: 20, margin: '0 auto 20px',
              background: 'var(--wr-gradient)', display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 32, color: '#fff', boxShadow: 'var(--wr-shadow-glow)',
            }}>
              <RocketOutlined />
            </div>
            <h1 style={{ fontSize: 24, fontWeight: 700, margin: '0 0 8px', letterSpacing: '-0.02em' }}>
              10 分钟，看到你的品牌在 AI 里的样子
            </h1>
            <Text type="secondary" style={{ fontSize: 14, maxWidth: 480, display: 'block', margin: '0 auto' }}>
              现在用户问"XX哪家好"都问 AI 了——10 次回答里提到你几次？按下面四步走完，你的第一份 AI 可见度报告就出来了。
            </Text>
          </div>

          {/* 快速见效步骤 */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12, marginBottom: 32 }}>
            {QUICK_START_STEPS.map((s, i) => (
              <div key={s.key} style={{
                padding: 16, borderRadius: 12,
                border: '1px solid var(--wr-border)', background: 'var(--wr-bg-elevated)',
                display: 'flex', flexDirection: 'column', gap: 6, position: 'relative',
              }}>
                <div style={{
                  width: 28, height: 28, borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center',
                  background: 'var(--wr-gradient)', color: '#fff', fontSize: 13, fontWeight: 700, marginBottom: 4,
                }}>{i + 1}</div>
                <Text strong style={{ fontSize: 14 }}>{s.title}</Text>
                <Text type="secondary" style={{ fontSize: 12 }}>{s.desc}</Text>
                <Button size="small" type="link" style={{ padding: 0, fontSize: 12, alignSelf: 'flex-start' }}
                  onClick={() => navigate(s.path)}>
                  前往 <ArrowRightOutlined style={{ fontSize: 10 }} />
                </Button>
              </div>
            ))}
          </div>

          <div style={{ textAlign: 'center' }}>
            <Button type="primary" size="large" onClick={() => navigate('/m/brands')}>
              创建第一个品牌，开始
            </Button>
          </div>
        </div>
      </div>
    )
  }

  const ovData = (overviews.data || []) as any[]
  const totalAvg = ovData.length > 0
    ? ovData.reduce((s: number, o: any) => s + (o.avg_mention_rate || 0), 0) / ovData.length
    : 0
  const totalKeywords = ovData.reduce((s: number, o: any) => s + (o.keyword_count || 0), 0)
  const totalCompetitors = brands.reduce((s: number, b: Brand) => s + (b.competitors?.length || 0), 0)
  const strongCount = ovData.filter((o: any) => (o.avg_mention_rate || 0) >= 0.5).length

  // 整体变化对比：各品牌最新 vs 上一次提及率的平均变化（delta）
  const brandDeltas = ovData.map((o: any) => {
    const trend = (o.trend || []).filter((t: any) => t.mention_rate !== undefined)
    if (trend.length < 2) return null
    const latest = trend[trend.length - 1].mention_rate
    const prev = trend[trend.length - 2].mention_rate
    return Math.round((latest - prev) * 1000) / 10
  }).filter((d: number | null) => d !== null) as number[]
  const overallDelta = brandDeltas.length > 0
    ? brandDeltas.reduce((s: number, d: number) => s + d, 0) / brandDeltas.length
    : null
  const overallDeltaView = deltaView(overallDelta)

  // 提及率分布（环形图）
  const distData = [
    { type: '强势 (≥80%)', value: ovData.filter((o: any) => (o.avg_mention_rate || 0) >= 0.8).length },
    { type: '稳定 (50-80%)', value: ovData.filter((o: any) => (o.avg_mention_rate || 0) >= 0.5 && (o.avg_mention_rate || 0) < 0.8).length },
    { type: '偶尔 (20-50%)', value: ovData.filter((o: any) => (o.avg_mention_rate || 0) >= 0.2 && (o.avg_mention_rate || 0) < 0.5).length },
    { type: '缺席 (<20%)', value: ovData.filter((o: any) => (o.avg_mention_rate || 0) < 0.2).length },
  ].filter((d) => d.value > 0)

  const chartTheme = {
    color: 'var(--wr-text-primary)',
    axis: { common: { labelFill: 'var(--wr-text-muted)', lineStroke: 'var(--wr-border)' } },
  }

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* 页面标题 */}
        <div className="wr-page-header">
          <h1>数据驾驶舱</h1>
          <p>你的品牌在 AI 搜索引擎中的可见度 · {brands.length} 个品牌 · {totalKeywords} 个监测关键词</p>
        </div>

        {/* 核心指标卡（6 张）*/}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }} className="wr-stagger">
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/brands')} style={{ cursor: 'pointer' }}>
              <div className="wr-metric-value wr-gradient-text">{brands.length}</div>
              <div className="wr-metric-label">品牌数量</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/keywords')} style={{ cursor: 'pointer' }}>
              <div className="wr-metric-value">{totalKeywords}</div>
              <div className="wr-metric-label">监测关键词</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card">
              <div className="wr-metric-value" style={{ color: rateColor(totalAvg) }}>
                {(totalAvg * 100).toFixed(1)}<span style={{ fontSize: 16, fontWeight: 600 }}>%</span>
              </div>
              <div className="wr-metric-label">平均提及率</div>
              <div style={{ fontSize: 11, marginTop: 4, fontWeight: 600, color: overallDeltaView.color }}>
                {overallDeltaView.arrow} {overallDeltaView.text} 较上期
              </div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card">
              <div className="wr-metric-value" style={{ color: 'var(--wr-success)' }}>{strongCount}</div>
              <div className="wr-metric-label">强势品牌 (≥50%)</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/brands')} style={{ cursor: 'pointer' }}>
              <div className="wr-metric-value">{totalCompetitors}</div>
              <div className="wr-metric-label">竞品追踪</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/content')} style={{ cursor: 'pointer' }}>
              <div className="wr-metric-value wr-shimmer">→</div>
              <div className="wr-metric-label">去内容工作台</div>
            </div>
          </Col>
        </Row>

        {/* 提及率趋势 + 分布 */}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} lg={16}>
            <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 0, fontSize: 14 }}>
                  提及率趋势
                </Title>
                {/* 自动盯盘状态（商户端感知：趋势自动生长）*/}
                <AutoMonitorBadge />
              </div>
              {(() => {
                const trendData: any[] = []
                ovData.forEach((o: any, i: number) => {
                  const brandName = brands[i]?.name || `品牌${i + 1}`
                  ;(o.trend || []).forEach((t: any) => {
                    if (t.mention_rate !== undefined && t.probed_at) {
                      trendData.push({
                        date: new Date(t.probed_at).toLocaleDateString(),
                        rate: Math.round((t.mention_rate || 0) * 1000) / 10,
                        brand: brandName,
                      })
                    }
                  })
                })
                if (trendData.length === 0) {
                  return <div style={{ textAlign: 'center', padding: '60px 0' }}><Text type="secondary">暂无监测数据——前往「关键词管理」发起监测</Text></div>
                }
                return (
                  <Line
                    data={trendData}
                    xField="date" yField="rate"
                    seriesField="brand"
                    smooth
                    height={260}
                    color={['#7c6cff', '#22d3ee', '#4ade80', '#fbbf24', '#fb7185']}
                    point={{ size: 3, shape: 'circle' }}
                    yAxis={{ label: { formatter: (v: string) => v + '%' } }}
                    tooltip={{ formatter: (d: any) => ({ name: d.brand, value: d.rate + '%' }) }}
                  />
                )
              })()}
            </Card>
          </Col>
          <Col xs={24} lg={8}>
            <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
              <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 16, fontSize: 14 }}>
                提及率分布
              </Title>
              {distData.length > 0 ? (
                <Pie
                  data={distData}
                  angleField="value" colorField="type"
                  height={260}
                  radius={0.85} innerRadius={0.6}
                  color={['#4ade80', '#22d3ee', '#fbbf24', '#fb7185']}
                  label={{ text: 'type', position: 'outside', fill: 'var(--wr-text-secondary)', fontSize: 11 }}
                  theme={chartTheme as any}
                />
              ) : <div style={{ textAlign: 'center', paddingTop: 80 }}><Text type="secondary">暂无数据</Text></div>}
            </Card>
          </Col>
        </Row>

        {/* 品牌可见度卡片 */}
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 0, fontSize: 14 }}>
            品牌 AI 可见度
          </Title>
          <Button type="text" size="small" onClick={() => navigate('/m/brands')}>
            管理品牌 <ArrowRightOutlined style={{ fontSize: 11 }} />
          </Button>
        </div>
        <Row gutter={[16, 16]} className="wr-stagger">
          {brands.map((b: Brand) => {
            const ov = ovData.find((o: any) => o.brand_id === b.id)
            const rate = ov?.avg_mention_rate || 0
            const color = rateColor(rate)
            // 该品牌最新 vs 上一次提及率变化
            const trend = (ov?.trend || []).filter((t: any) => t.mention_rate !== undefined)
            const delta = deltaView(trend.length >= 2
              ? Math.round((trend[trend.length - 1].mention_rate - trend[trend.length - 2].mention_rate) * 1000) / 10
              : null)
            // 最近一次监测的采样次数（置信度传达）
            const lastSample = latestMonitor(trend as any)?.sample_count || 0
            return (
              <Col xs={24} sm={12} lg={8} key={b.id}>
                <div className="wr-glass-card" style={{ padding: 22, height: '100%', cursor: 'pointer' }} onClick={() => navigate('/m/brands')}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 18 }}>
                    <div>
                      <Text strong style={{ fontSize: 16, letterSpacing: '-0.01em' }}>{b.name}</Text>
                      {b.positioning && (
                        <Text type="secondary" style={{ display: 'block', marginTop: 4, fontSize: 12.5, lineHeight: 1.5 }}>
                          {b.positioning.length > 46 ? b.positioning.slice(0, 46) + '...' : b.positioning}
                        </Text>
                      )}
                    </div>
                    <span className="wr-rate-badge" style={{ background: `${color}1a`, color, borderColor: `${color}33` }}>
                      {rateLabel(rate)}
                    </span>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginBottom: 16 }}>
                    <span style={{ fontSize: 40, fontWeight: 700, color, letterSpacing: '-0.03em', lineHeight: 1 }}>
                      {(rate * 100).toFixed(0)}
                    </span>
                    <span style={{ fontSize: 18, color: 'var(--wr-text-muted)', fontWeight: 500 }}>%</span>
                    <span style={{ fontSize: 12, color: 'var(--wr-text-muted)', marginLeft: 8 }}>提及率</span>
                    {/* 变化对比 */}
                    <span style={{ fontSize: 12, fontWeight: 700, color: delta.color, marginLeft: 6 }}>
                      {delta.arrow} {delta.text}
                    </span>
                  </div>

                  <div style={{ display: 'flex', gap: 16, paddingTop: 14, borderTop: '1px solid var(--wr-border)' }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>{ov?.keyword_count || 0} 个关键词</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>{b.competitors?.length || 0} 个竞品</Text>
                    {lastSample > 0 && <Text type="secondary" style={{ fontSize: 12 }}>采样 {lastSample} 次</Text>}
                  </div>

                  {b.core_selling && b.core_selling.length > 0 && (
                    <div style={{ marginTop: 12, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                      {b.core_selling.slice(0, 3).map((s, i) => (
                        <Tag key={i} style={{ margin: 0, fontSize: 11, borderRadius: 6 }}>{s}</Tag>
                      ))}
                    </div>
                  )}

                  <div style={{ marginTop: 14, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>GEO 可见度</Text>
                    <Button
                      size="small" type="text"
                      style={{ fontSize: 12, color: 'var(--wr-primary)' }}
                      onClick={(e) => { e.stopPropagation(); navigate('/m/keywords') }}
                    >
                      查看关键词与监测 →
                    </Button>
                  </div>
                </div>
              </Col>
            )
          })}
        </Row>
      </div>
    </div>
  )
}

// AutoMonitorBadge 自动盯盘状态徽标（商户端感知：趋势自动生长）。
// 两级开关：平台总闸（管理员控制，只读）+ 租户开关（商户可自控）。
function AutoMonitorBadge() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['tenant-auto-monitor'],
    queryFn: () => businessApi.getTenantAutoMonitor(),
  })

  const toggleMutation = useMutation({
    mutationFn: (enabled: boolean) => businessApi.setTenantAutoMonitor(enabled),
    onSuccess: () => {
      message.success('自动盯盘已' + (data?.tenant_enabled ? '关闭' : '开启') + '（每日自动监测，趋势自动生长）')
      queryClient.invalidateQueries({ queryKey: ['tenant-auto-monitor'] })
    },
  })

  const active = data?.platform_enabled && data?.tenant_enabled
  return (
    <Space size={6}>
      <span style={{
        display: 'inline-flex', alignItems: 'center', gap: 6,
        padding: '3px 10px', borderRadius: 20, fontSize: 11.5, fontWeight: 600,
        background: active ? 'rgba(74,222,128,0.12)' : 'var(--wr-bg-elevated)',
        color: active ? 'var(--wr-success)' : 'var(--wr-text-muted)',
        border: `1px solid ${active ? 'rgba(74,222,128,0.3)' : 'var(--wr-border)'}`,
      }}>
        <span style={{
          width: 6, height: 6, borderRadius: '50%',
          background: active ? 'var(--wr-success)' : 'var(--wr-text-muted)',
          animation: active ? 'wr-pulse 2s infinite' : 'none',
        }} />
        {active ? '自动盯盘已开启 · 趋势自动生长' : data?.platform_enabled ? '自动盯盘已暂停' : '平台自动盯盘未开启'}
      </span>
      {data?.platform_enabled && (
        <Switch
          size="small"
          checked={data?.tenant_enabled}
          loading={isLoading || toggleMutation.isPending}
          onChange={(v) => toggleMutation.mutate(v)}
        />
      )}
    </Space>
  )
}

import { useState } from 'react'
import { Card, Typography, Row, Col, Spin, Tag, Button, Switch, Space, message, Progress, Tooltip, List, Select, InputNumber } from 'antd'
import { RocketOutlined, ArrowRightOutlined, RadarChartOutlined, BellOutlined, ThunderboltOutlined, CheckCircleOutlined } from '@ant-design/icons'
import { Line, Pie } from '@ant-design/charts'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { deltaView, latestMonitor } from '../../utils/geo'
import type { Brand } from '../../types/api'

const { Title, Text } = Typography

// 渐进式 Onboarding 步骤（基于实际数据判断 done/pending，非硬编码）
// 从 overviews + brands 推导，无需额外查询
function useOnboardingSteps(brands: any[], ovData: any[]) {
  const hasBrands = brands.length > 0
  const hasCompetitors = brands.some((b: any) => (b.competitors?.length || 0) > 0)
  const hasKeywords = ovData.some((o: any) => (o.keyword_count || 0) > 0)
  const hasMonitor = ovData.some((o: any) => (o.trend?.length || 0) > 0)
  const allDone = hasBrands && hasCompetitors && hasKeywords && hasMonitor
  const doneCount = [hasBrands, hasCompetitors, hasKeywords, hasMonitor].filter(Boolean).length
  return { hasBrands, hasCompetitors, hasKeywords, hasMonitor, allDone, doneCount }
}

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
  // Onboarding dismiss 状态必须在所有条件 return 之前（React Hooks 规则）
  const [onboardingDismissed, setOnboardingDismissed] = useState(
    typeof window !== 'undefined' && localStorage.getItem('wr-onboarding-dismissed') === '1'
  )
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

  // 待办：未读通知（提及率变化/自动复测/排期发布等主动唤醒信号）
  const { data: notifRes } = useQuery({
    queryKey: ['merchant-notifications'],
    queryFn: () => businessApi.listNotifications(),
    staleTime: 30_000,
  })
  const unreadNotifs = (notifRes || []).filter((n: any) => !n.read).slice(0, 3)

  // 配额用量（套餐余量——让商户每次进来感知"我还剩多少额度"）
  const { data: usage } = useQuery({
    queryKey: ['my-usage'],
    queryFn: () => businessApi.getMyUsage().catch(() => null),
    staleTime: 60_000,
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

          {/* 快速见效步骤（内联，不再用常量——渐进式 Onboarding 已移到有品牌时的引导条）*/}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12, marginBottom: 32 }}>
            {[
              { title: '创建品牌', desc: '填写定位/卖点/竞品', path: '/m/brands' },
              { title: '添加关键词', desc: 'AI 生成或蒸馏获取', path: '/m/keywords' },
              { title: '立即监测', desc: '看 AI 怎么评价你', path: '/m/keywords' },
              { title: '生成内容', desc: '优化 AI 可见度', path: '/m/content' },
            ].map((s, i) => (
              <div key={i} style={{
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

  // 渐进式 Onboarding：基于数据判断步骤完成度（有品牌但未完成全流程时显示引导条）
  const steps = useOnboardingSteps(brands, ovData)
  const showOnboarding = !steps.allDone && !onboardingDismissed && brands.length > 0

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

        {/* 渐进式 Onboarding 引导条（有品牌但未完成全流程时显示）*/}
        {showOnboarding && (
          <Card className="wr-glass-card" style={{ marginBottom: 16, borderColor: 'rgba(124,108,255,0.2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
              <div style={{ flexShrink: 0 }}>
                <Progress type="circle" percent={(steps.doneCount / 4) * 100} size={48} strokeColor="var(--wr-primary)" />
              </div>
              <div style={{ flex: 1 }}>
                <Text strong style={{ fontSize: 14, display: 'block', marginBottom: 6 }}>
                  🚀 快速配置向导 · 已完成 {steps.doneCount}/4 步
                </Text>
                <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                  {[
                    { done: steps.hasBrands, label: '创建品牌', path: '/m/brands' },
                    { done: steps.hasCompetitors, label: '配置竞品', path: '/m/brands' },
                    { done: steps.hasKeywords, label: '添加关键词', path: '/m/keywords' },
                    { done: steps.hasMonitor, label: '发起监测', path: '/m/keywords' },
                  ].map((s, i) => (
                    <Button
                      key={i}
                      size="small"
                      type={s.done ? 'default' : 'dashed'}
                      style={{ fontSize: 12 }}
                      icon={s.done ? <CheckCircleOutlined style={{ color: 'var(--wr-success)' }} /> : undefined}
                      onClick={() => navigate(s.path)}
                    >
                      {s.done ? '' : `${i + 1}. `}{s.label}
                    </Button>
                  ))}
                </div>
              </div>
              <Button
                size="small" type="text"
                onClick={() => {
                  setOnboardingDismissed(true)
                  localStorage.setItem('wr-onboarding-dismissed', '1')
                }}
              >关闭引导</Button>
            </div>
          </Card>
        )}

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

        {/* 自动盯盘控制（商户端显眼入口：状态/开关/说明/套餐提示）*/}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col span={24}>
            <AutoMonitorCard />
          </Col>
        </Row>

        {/* 待办 + 配额用量横条（每次进来第一眼看到的运营信号）*/}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          {/* 待办：未读通知 */}
          <Col xs={24} lg={14}>
            <Card className="wr-glass-card" styles={{ body: { padding: 16 } }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <Space size={6}>
                  <BellOutlined style={{ color: 'var(--wr-accent)' }} />
                  <Text strong style={{ fontSize: 14 }}>待办提醒</Text>
                  {unreadNotifs.length > 0 && <Tag color="processing" style={{ fontSize: 11 }}>{unreadNotifs.length} 条未读</Tag>}
                </Space>
                {unreadNotifs.length === 0 && <Text type="secondary" style={{ fontSize: 12 }}><CheckCircleOutlined /> 全部已处理</Text>}
              </div>
              {unreadNotifs.length > 0 ? (
                <List
                  size="small"
                  dataSource={unreadNotifs}
                  renderItem={(n: any) => (
                    <List.Item style={{ padding: '6px 0', border: 'none', cursor: n.link ? 'pointer' : 'default' }}
                      onClick={() => n.link && navigate(n.link)}>
                      <Space size={8} style={{ width: '100%' }}>
                        <Tag color={n.type?.includes('drop') ? 'error' : n.type?.includes('overtake') ? 'warning' : 'default'} style={{ fontSize: 11, margin: 0 }}>{n.type || '通知'}</Tag>
                        <Text ellipsis style={{ flex: 1, fontSize: 13, color: 'var(--wr-text-secondary)' }}>{n.title}</Text>
                        <Text type="secondary" style={{ fontSize: 11, flexShrink: 0 }}>{(n.created_at || '').slice(5, 16).replace('T', ' ')}</Text>
                      </Space>
                    </List.Item>
                  )}
                />
              ) : (
                <Text type="secondary" style={{ fontSize: 13 }}>暂无待办——监测/复测/排期发布的结果会出现在这里</Text>
              )}
            </Card>
          </Col>
          {/* 配额用量 */}
          <Col xs={24} lg={10}>
            <Card className="wr-glass-card" styles={{ body: { padding: 16 } }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <Space size={6}>
                  <ThunderboltOutlined style={{ color: 'var(--wr-warning)' }} />
                  <Text strong style={{ fontSize: 14 }}>本月用量</Text>
                  <Tag color={usage?.plan?.level === 'team' ? 'gold' : usage?.plan?.level === 'pro' ? 'purple' : 'default'} style={{ fontSize: 11 }}>{usage?.plan?.name || '免费版'}</Tag>
                </Space>
                <Button size="small" type="link" onClick={() => navigate('/m/my-plan')}>详情</Button>
              </div>
              <Row gutter={[12, 8]}>
                {usage && Object.entries(usage.usages || {}).slice(0, 4).map(([scene, u]: [string, any]) => {
                  const unlimited = u.limit === -1
                  const pct = unlimited ? 0 : u.limit > 0 ? Math.min(100, (u.used / u.limit) * 100) : 0
                  const labels: Record<string, string> = { monitor: '监测', 'content-gen': '生成', 'content-opt': '优化', chat: '对话' }
                  return (
                    <Col span={12} key={scene}>
                      <Tooltip title={`${labels[scene] || scene}：${unlimited ? '无限' : u.used + '/' + u.limit}`}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                          <Text type="secondary" style={{ fontSize: 11 }}>{labels[scene] || scene}</Text>
                          <Text style={{ fontSize: 11, color: pct >= 100 ? 'var(--wr-danger)' : 'var(--wr-text-muted)' }}>{unlimited ? '∞' : `${u.used}/${u.limit}`}</Text>
                        </div>
                        {!unlimited && <Progress percent={pct} size="small" showInfo={false} strokeColor={pct >= 100 ? 'var(--wr-danger)' : pct >= 80 ? 'var(--wr-warning)' : 'var(--wr-accent)'} />}
                      </Tooltip>
                    </Col>
                  )
                })}
              </Row>
            </Card>
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
                      const d = new Date(t.probed_at)
                      // x 轴用"月-日 时:分"而非日期：同一天多次监测（手动+自动盯盘）点不重叠
                      trendData.push({
                        date: `${d.getMonth() + 1}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`,
                        ts: d.getTime(),
                        rate: Math.round((t.mention_rate || 0) * 1000) / 10,
                        brand: brandName,
                      })
                    }
                  })
                })
                if (trendData.length === 0) {
                  return <div style={{ textAlign: 'center', padding: '60px 0' }}><Text type="secondary">暂无监测数据——前往「关键词管理」发起监测</Text></div>
                }
                // 按时间排序（Trend 已升序，双保险）
                trendData.sort((a: any, b: any) => a.ts - b.ts)
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
                    xAxis={{ label: { autoRotate: true, style: { fontSize: 10 } } }}
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

// AutoMonitorCard 自动盯盘控制卡片（商户端显眼入口）：
// 状态 + 开关 + 高级设置（频率/采样/通知阈值——用户清楚"开启后发生什么"）。
// 数据同 AutoMonitorBadge（tenant-auto-monitor）——开关与配置一次保存。
function AutoMonitorCard() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [showAdvanced, setShowAdvanced] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['tenant-auto-monitor'],
    queryFn: () => businessApi.getTenantAutoMonitor(),
  })
  const { data: usage } = useQuery({
    queryKey: ['my-usage'],
    queryFn: () => businessApi.getMyUsage().catch(() => null),
    staleTime: 60_000,
  })
  // 本地表单状态（默认值来自服务端配置；未加载完成用默认）
  const cfg = data?.config || { frequency: 'daily', sample_size: 5, notify_drop_threshold: 20, notify_overtake: true }
  const [frequency, setFrequency] = useState<string>('daily')
  const [sampleSize, setSampleSize] = useState<number>(5)
  const [dropThreshold, setDropThreshold] = useState<number>(20)
  const [notifyOvertake, setNotifyOvertake] = useState<boolean>(true)
  const [loadedCfg, setLoadedCfg] = useState(false)
  // 配置加载完成后同步到表单（只同步一次，避免每次渲染重置用户编辑）
  if (data?.config && !loadedCfg) {
    setLoadedCfg(true)
    setFrequency(data.config.frequency || 'daily')
    setSampleSize(data.config.sample_size || 5)
    setDropThreshold(data.config.notify_drop_threshold || 20)
    setNotifyOvertake(data.config.notify_overtake !== false)
  }
  const saveMutation = useMutation({
    mutationFn: ({ enabled, cfg }: { enabled: boolean; cfg?: any }) =>
      businessApi.setTenantAutoMonitor({ enabled, config: cfg }),
    onSuccess: () => {
      message.success('自动盯盘设置已保存（按配置每日自动监测，趋势自动生长）')
      queryClient.invalidateQueries({ queryKey: ['tenant-auto-monitor'] })
    },
  })

  const frequencyLabel: Record<string, string> = { daily: '每天 1 次', half_day: '每 12 小时', weekly: '每周 1 次' }
  const hasFeature = (usage?.plan?.features || []).includes('auto-monitor')
  const active = data?.platform_enabled && data?.tenant_enabled
  return (
    <Card className="wr-glass-card" styles={{ body: { padding: 16 } }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
        <Space size={6}>
          <RadarChartOutlined style={{ color: 'var(--wr-primary)' }} />
          <Text strong style={{ fontSize: 14 }}>自动盯盘</Text>
          <Tag color={active ? 'success' : data?.platform_enabled ? 'warning' : 'default'} style={{ fontSize: 11, margin: 0 }}>
            {active ? '运行中' : data?.platform_enabled ? '已暂停' : '平台未开启'}
          </Tag>
        </Space>
        <Space size={8}>
          <Button size="small" type="link" onClick={() => setShowAdvanced(!showAdvanced)}>
            {showAdvanced ? '收起设置 ↑' : '高级设置 ⚙'}
          </Button>
          {hasFeature ? (
            <Switch
              checked={!!data?.tenant_enabled}
              disabled={!data?.platform_enabled}
              loading={isLoading || saveMutation.isPending}
              onChange={(v) => saveMutation.mutate({ enabled: v })}
              checkedChildren="开启" unCheckedChildren="关闭"
            />
          ) : (
            <Button size="small" type="link" onClick={() => navigate('/m/my-plan')}>升级解锁 →</Button>
          )}
        </Space>
      </div>
      <Text type="secondary" style={{ fontSize: 12, display: 'block', lineHeight: 1.7 }}>
        开启后系统按你设置的节奏自动监测全部关键词——趋势自动生长，无需手动点监测；
        提及率下降或竞品反超时按阈值自动通知（见待办提醒）。
        {showAdvanced && data?.platform_enabled && ` 当前：${frequencyLabel[cfg.frequency]} · 每关键词 ${cfg.sample_size} 次采样。`}
      </Text>
      {!data?.platform_enabled && (
        <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 8, color: 'var(--wr-text-muted)' }}>
          平台总开关未开启（管理员在平台设置中控制）
        </Text>
      )}

      {/* 高级设置（用户清楚"开启后发生什么"）*/}
      {showAdvanced && hasFeature && (
        <div style={{ marginTop: 12, padding: '12px 14px', borderRadius: 10, background: 'var(--wr-bg-elevated)' }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12 }}>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>监测频率</Text>
              <Select
                size="small" style={{ width: '100%' }} value={frequency}
                onChange={setFrequency}
                options={[
                  { value: 'daily', label: '每天 1 次（省额度）' },
                  { value: 'half_day', label: '每 12 小时（更灵敏）' },
                  { value: 'weekly', label: '每周 1 次（最省）' },
                ]}
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>每关键词采样次数</Text>
              <Select
                size="small" style={{ width: '100%' }} value={sampleSize}
                onChange={setSampleSize}
                options={[
                  { value: 3, label: '3 次（快测，省 token）' },
                  { value: 5, label: '5 次（推荐，更准）' },
                  { value: 10, label: '10 次（最准，烧 token）' },
                ]}
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>提及率下降通知阈值</Text>
              <InputNumber
                size="small" min={5} max={80} style={{ width: '100%' }} value={dropThreshold}
                onChange={(v) => setDropThreshold(v || 20)} addonAfter="%"
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>竞品反超通知</Text>
              <Switch size="small" checked={notifyOvertake} onChange={setNotifyOvertake} checkedChildren="开" unCheckedChildren="关" />
              <Text type="secondary" style={{ fontSize: 10, display: 'block', marginTop: 2 }}>竞品提及率超过你时提醒</Text>
            </div>
          </div>
          <div style={{ marginTop: 10, display: 'flex', justifyContent: 'flex-end' }}>
            <Button size="small" type="primary" loading={saveMutation.isPending}
              onClick={() => saveMutation.mutate({
                enabled: !!data?.tenant_enabled,
                cfg: { frequency, sample_size: sampleSize, engine_name: '', notify_drop_threshold: dropThreshold, notify_overtake: notifyOvertake },
              })}>
              保存盯盘设置
            </Button>
          </div>
        </div>
      )}
    </Card>
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
    mutationFn: (enabled: boolean) => businessApi.setTenantAutoMonitor({ enabled }),
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

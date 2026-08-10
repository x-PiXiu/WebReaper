import { Card, Typography, Row, Col, Spin, Tag } from 'antd'
import { RocketOutlined } from '@ant-design/icons'
import { Line } from '@ant-design/charts'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand } from '../../types/api'

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

export default function MerchantHome() {
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
        <Card className="wr-glass-card" style={{ textAlign: 'center', padding: '60px 40px' }}>
          <RocketOutlined style={{ fontSize: 48, color: 'var(--wr-primary)', marginBottom: 16 }} />
          <Title level={3} style={{ marginBottom: 8 }}>开启你的 GEO 之旅</Title>
          <Text type="secondary" style={{ fontSize: 15 }}>
            创建第一个品牌，开始优化你在 AI 搜索引擎中的可见度
          </Text>
          <div style={{ marginTop: 24 }}>
            <Text type="secondary" style={{ fontSize: 13 }}>
              前往「品牌管理」→ 创建品牌 → AI 生成关键词 → 监测排名
            </Text>
          </div>
        </Card>
      </div>
    )
  }

  const ovData = (overviews.data || []) as any[]
  const totalAvg = ovData.length > 0
    ? ovData.reduce((s: number, o: any) => s + (o.avg_mention_rate || 0), 0) / ovData.length
    : 0
  const totalKeywords = ovData.reduce((s: number, o: any) => s + (o.keyword_count || 0), 0)

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* 页面标题 */}
        <div style={{ marginBottom: 32 }}>
          <Title level={3} style={{ margin: 0, fontSize: 28, letterSpacing: '-0.03em' }}>
            数据驾驶舱
          </Title>
          <Text type="secondary" style={{ fontSize: 14 }}>
            你的品牌在 AI 搜索引擎中的可见度概览
          </Text>
        </div>

        {/* 核心指标卡 */}
        <Row gutter={[20, 20]} style={{ marginBottom: 32 }} className="wr-stagger">
          <Col xs={24} sm={8}>
            <div className="wr-metric-card">
              <div className="wr-metric-label">品牌数量</div>
              <div className="wr-metric-value wr-gradient-text">{brands.length}</div>
            </div>
          </Col>
          <Col xs={24} sm={8}>
            <div className="wr-metric-card">
              <div className="wr-metric-label">平均提及率</div>
              <div className="wr-metric-value" style={{ color: rateColor(totalAvg) }}>
                {(totalAvg * 100).toFixed(1)}<span style={{ fontSize: 18, fontWeight: 500 }}>%</span>
              </div>
            </div>
          </Col>
          <Col xs={24} sm={8}>
            <div className="wr-metric-card">
              <div className="wr-metric-label">监测关键词</div>
              <div className="wr-metric-value" style={{ color: 'var(--wr-text-primary)' }}>{totalKeywords}</div>
            </div>
          </Col>
        </Row>

        {/* 提及率趋势图 */}
        {ovData.length > 0 && (() => {
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
          if (trendData.length === 0) return null
          return (
            <Card className="wr-glass-card" style={{ marginBottom: 20 }} styles={{ body: { padding: 24 } }}>
              <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 500, marginBottom: 16 }}>
                提及率趋势
              </Title>
              <Line
                data={trendData}
                xField="date" yField="rate"
                seriesField="brand"
                smooth
                height={260}
                color={['#6366f1', '#0891b2', '#10b981', '#f59e0b', '#ef4444']}
                point={{ size: 3, shape: 'circle' }}
                yAxis={{ label: { formatter: (v: string) => v + '%' } }}
                tooltip={{ formatter: (d: any) => ({ name: d.brand, value: d.rate + '%' }) }}
              />
            </Card>
          )
        })()}

        {/* 品牌可见度卡片 */}
        <div style={{ marginBottom: 16 }}>
          <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 500, marginBottom: 16 }}>
            品牌 AI 可见度
          </Title>
        </div>
        <Row gutter={[20, 20]} className="wr-stagger">
          {brands.map((b: Brand) => {
            const ov = ovData.find((o: any) => o.brand_id === b.id)
            const rate = ov?.avg_mention_rate || 0
            const color = rateColor(rate)
            return (
              <Col xs={24} sm={12} lg={8} key={b.id}>
                <div className="wr-glass-card" style={{ padding: 24, height: '100%' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
                    <div>
                      <Text strong style={{ fontSize: 18, letterSpacing: '-0.01em' }}>{b.name}</Text>
                      {b.positioning && (
                        <Text type="secondary" style={{ display: 'block', marginTop: 4, fontSize: 13, lineHeight: 1.5 }}>
                          {b.positioning.length > 40 ? b.positioning.slice(0, 40) + '...' : b.positioning}
                        </Text>
                      )}
                    </div>
                    <span className="wr-rate-badge" style={{ background: `${color}20`, color }}>
                      {rateLabel(rate)}
                    </span>
                  </div>

                  {/* 大数字指标 */}
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginBottom: 16 }}>
                    <span style={{ fontSize: 40, fontWeight: 700, color, letterSpacing: '-0.03em', lineHeight: 1 }}>
                      {(rate * 100).toFixed(0)}
                    </span>
                    <span style={{ fontSize: 18, color: 'var(--wr-text-muted)', fontWeight: 500 }}>%</span>
                    <span style={{ fontSize: 13, color: 'var(--wr-text-muted)', marginLeft: 8 }}>提及率</span>
                  </div>

                  {/* 底部统计 */}
                  <div style={{ display: 'flex', gap: 16, paddingTop: 16, borderTop: '1px solid var(--wr-border)' }}>
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>{ov?.keyword_count || 0} 个关键词</Text>
                    </div>
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>{b.competitors?.length || 0} 个竞品</Text>
                    </div>
                  </div>

                  {/* 核心卖点标签 */}
                  {b.core_selling && b.core_selling.length > 0 && (
                    <div style={{ marginTop: 12, display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                      {b.core_selling.slice(0, 3).map((s, i) => (
                        <Tag key={i} style={{ margin: 0, fontSize: 11, borderRadius: 6 }}>{s}</Tag>
                      ))}
                    </div>
                  )}
                </div>
              </Col>
            )
          })}
        </Row>
      </div>
    </div>
  )
}

import { Row, Col, Typography, Spin, Card, Tag, Space, Empty, Progress } from 'antd'
import { DollarOutlined, CrownOutlined, RiseOutlined, AppstoreOutlined, FileTextOutlined, GlobalOutlined, TrophyOutlined, SmileOutlined, LinkOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '../../store/auth'
import { businessApi } from '../../api/business'
import type { IndustryOverviewView } from '../../types/api'

const { Text } = Typography

// 核心指标卡（SaaS 运营三件套：MRR / 活跃商户 / 有效订阅——比规模数字更值得关注）
function HeroCard({ label, value, sublabel, icon, onClick }: {
  label: string; value: string; sublabel?: string; icon: React.ReactNode; onClick?: () => void
}) {
  return (
    <div onClick={onClick} style={{
      position: 'relative', padding: '20px 24px', background: 'var(--wr-card-bg)',
      border: '1px solid var(--wr-border)', borderRadius: 16, overflow: 'hidden',
      cursor: onClick ? 'pointer' : 'default', transition: 'all 200ms',
    }}
      onMouseEnter={e => { e.currentTarget.style.borderColor = 'var(--wr-border-hover)'; e.currentTarget.style.transform = 'translateY(-2px)' }}
      onMouseLeave={e => { e.currentTarget.style.borderColor = 'var(--wr-border)'; e.currentTarget.style.transform = 'translateY(0)' }}>
      <div style={{ position: 'absolute', right: -12, top: -12, fontSize: 64, opacity: 0.08 }}>{icon}</div>
      <Text style={{ color: 'var(--wr-text-muted)', fontSize: 12, display: 'block', marginBottom: 6 }}>{label}</Text>
      <div style={{ fontSize: 30, fontWeight: 800, color: 'var(--wr-text-primary)', letterSpacing: '-0.03em', lineHeight: 1.1 }}>{value}</div>
      {sublabel && <Text style={{ color: 'var(--wr-text-secondary)', fontSize: 11 }}>{sublabel}</Text>}
    </div>
  )
}

// 统计卡片
function StatCard({ label, value, sublabel, gradient, onClick }: {
  label: string; value: string | number; sublabel?: string; gradient: string; onClick?: () => void
}) {
  return (
    <div onClick={onClick} style={{
      position: 'relative', padding: 24, background: 'var(--wr-card-bg)',
      border: '1px solid var(--wr-border)', borderRadius: 14,
      cursor: onClick ? 'pointer' : 'default', transition: 'all 200ms', overflow: 'hidden',
    }}
      onMouseEnter={e => { e.currentTarget.style.borderColor = 'var(--wr-border-hover)'; e.currentTarget.style.transform = 'translateY(-2px)' }}
      onMouseLeave={e => { e.currentTarget.style.borderColor = 'var(--wr-border)'; e.currentTarget.style.transform = 'translateY(0)' }}>
      <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 3, background: gradient }} />
      <Text style={{ color: 'var(--wr-text-muted)', fontSize: 13, display: 'block', marginBottom: 8 }}>{label}</Text>
      <div style={{ fontSize: 32, fontWeight: 700, color: 'var(--wr-text-primary)', letterSpacing: '-0.03em' }}>{value}</div>
      {sublabel && <Text style={{ color: 'var(--wr-text-secondary)', fontSize: 11 }}>{sublabel}</Text>}
    </div>
  )
}

// 平台总览：SaaS 运营核心指标 + GEO 业务规模。
export default function Dashboard() {
  const username = useAuthStore(s => s.username)
  const navigate = useNavigate()

  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: () => businessApi.getStats(),
  })

  // 经济系统核心指标（MRR / 活跃订阅——SaaS 运营最关心的）
  const { data: revenue } = useQuery({
    queryKey: ['billing-revenue'],
    queryFn: () => businessApi.adminRevenueReport().catch(() => null),
  })
  const yuan = (cents: number) => `¥${(cents / 100).toFixed(0)}`

  // 平台 GEO 健康：行业分布 + 内容状态分布（admin 旁路聚合）
  const { data: brands = [] } = useQuery({
    queryKey: ['admin-brands'],
    queryFn: () => businessApi.adminListBrands().catch(() => []),
  })
  const { data: contents = [] } = useQuery({
    queryKey: ['admin-contents'],
    queryFn: () => businessApi.adminListContents().catch(() => []),
  })

  const industryDist = useMemo(() => {
    const map = new Map<string, number>()
    brands.forEach((b: { industry?: string; biz_type?: string }) => {
      // F1-4 口径修正：本图按"业务类型"分（与下方行业榜的 industry 字段是两个维度，避免同页两套"行业"口径互相矛盾）
      const key = b.biz_type === 'online' ? '线上业务' : '本地生意'
      map.set(key, (map.get(key) || 0) + 1)
    })
    return Array.from(map.entries()).sort((a, b) => b[1] - a[1]).slice(0, 8)
  }, [brands])

  // F1-4 治理卡数据：行业字段未填的品牌数（影响行业能见度榜——全落"未分类"）
  const industryMissing = useMemo(
    () => brands.filter((b: { industry?: string }) => !(b.industry || '').trim()).length,
    [brands],
  )

  // F4 告警面：沉睡商户（从未监测或 >30 天未活跃）+ 草稿积压——运营最该跟进的信号
  const { data: allUsers = [] } = useQuery({
    queryKey: ['admin-users'],
    queryFn: () => businessApi.listUsers().catch(() => []),
    staleTime: 60_000,
  })
  const sleepyMerchants = useMemo(
    () => allUsers.filter((u: { role: string; last_active?: string }) => {
      if (u.role !== 'merchant') return false
      if (!u.last_active) return true
      return Date.now() - new Date(u.last_active).getTime() > 30 * 86400000
    }).length,
    [allUsers],
  )
  const draftBacklog = useMemo(
    () => contents.filter((c: { status?: string }) => c.status === 'draft').length,
    [contents],
  )

  const statusDist = useMemo(() => {
    const map = new Map<string, number>()
    contents.forEach((c: { status?: string }) => {
      const key = c.status === 'published' ? '已发布' : c.status === 'draft' ? '草稿' : (c.status || '其他')
      map.set(key, (map.get(key) || 0) + 1)
    })
    const total = Math.max(1, contents.length)
    const published = map.get('已发布') || 0
    return { dist: Array.from(map.entries()), publishRate: Math.round((published / total) * 100) }
  }, [contents])

  // 行业全景看板（v3 P2：跨商户聚合——行业能见度/品牌美誉度/信源域名榜；
  // 数据源：监测结果情感字段 + 品牌行业字段，后端一次聚合）
  const { data: industry = null } = useQuery<IndustryOverviewView | null>({
    queryKey: ['admin-industry-overview'],
    queryFn: () => businessApi.getIndustryOverview().catch(() => null),
    staleTime: 5 * 60_000,
  })

  return (
    <div className="wr-page-content">
      {/* 标题（统一 wr-page-header 规范——与商户端/其余 admin 页一致） */}
      <div className="wr-page-header">
        <h1>平台总览{username ? ` · ${username}` : ''}</h1>
        <p>SaaS 运营核心指标 · GEO 内容引擎运行概况</p>
      </div>

      {statsLoading && <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>}

      {/* 核心运营指标（SaaS 三件套：MRR / 活跃订阅 / 当月收入）*/}
      <Row gutter={[16, 16]} style={{ marginBottom: 20 }}>
        <Col xs={24} md={8}>
          <HeroCard label="当月经常性收入 (MRR)" value={yuan(revenue?.month_revenue_cents || 0)} sublabel="本月已支付订单"
            icon={<DollarOutlined />} onClick={() => navigate('/admin/billing')} />
        </Col>
        <Col xs={24} md={8}>
          <HeroCard label="有效订阅" value={String(revenue?.active_subscriptions || 0)} sublabel="当前计费周期内活跃"
            icon={<CrownOutlined />} onClick={() => navigate('/admin/billing')} />
        </Col>
        <Col xs={24} md={8}>
          <HeroCard label="累计收入" value={yuan(revenue?.total_revenue_cents || 0)} sublabel={`${revenue?.paid_orders || 0} 笔已支付订单`}
            icon={<RiseOutlined />} onClick={() => navigate('/admin/billing')} />
        </Col>
      </Row>

      {/* 平台规模（次要指标——8 个小卡片，运营参考用）*/}
      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8, paddingLeft: 4 }}>平台规模</Text>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} md={6}>
          <StatCard label="商户数" value={stats?.users ?? 0} sublabel="平台注册商户" gradient="linear-gradient(180deg,#7c6cff,#5b48e8)" onClick={() => navigate('/admin/users')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="品牌资产" value={stats?.brands ?? 0} sublabel="GEO 监测品牌" gradient="linear-gradient(180deg,#f59e0b,#d97706)" onClick={() => navigate('/admin/brands')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="关键词" value={stats?.keywords ?? 0} sublabel="投放监测关键词" gradient="linear-gradient(180deg,#22d3ee,#0891b2)" onClick={() => navigate('/admin/brands')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="优化内容" value={stats?.optimized_contents ?? 0} sublabel="GEO 生成/优化" gradient="linear-gradient(180deg,#10b981,#059669)" onClick={() => navigate('/admin/contents')} />
        </Col>
      </Row>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} md={8}>
          <StatCard label="已发布公开页" value={stats?.published_contents ?? 0} sublabel="AI 引擎可爬取" gradient="linear-gradient(180deg,#8b5cf6,#7c3aed)" onClick={() => navigate('/admin/contents')} />
        </Col>
        <Col xs={12} md={8}>
          <StatCard label="监测结果" value={stats?.monitor_results ?? 0} sublabel="累计引擎探测" gradient="linear-gradient(180deg,#ec4899,#db2777)" onClick={() => navigate('/admin/brands')} />
        </Col>
        <Col xs={12} md={8}>
          <StatCard label="发布任务" value={stats?.publish_jobs ?? 0} sublabel="多平台分发" gradient="linear-gradient(180deg,#f97316,#ea580c)" onClick={() => navigate('/admin/brands')} />
        </Col>
      </Row>

      {/* 平台 GEO 健康看板：行业分布 + 内容资产状态（跨商户聚合） */}
      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8, paddingLeft: 4 }}>平台 GEO 健康</Text>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
            <Space style={{ marginBottom: 12 }}>
              <AppstoreOutlined style={{ color: 'var(--wr-primary)' }} />
              <Text strong style={{ fontSize: 14 }}>品牌业务类型分布</Text>
              <Tag style={{ margin: 0 }}>{brands.length} 个品牌</Tag>
            </Space>
            {industryDist.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无品牌" />
            ) : (
              <Space direction="vertical" size={10} style={{ width: '100%' }}>
                {industryDist.map(([name, n]) => (
                  <div key={name}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                      <Text style={{ fontSize: 13 }}>{name}</Text>
                      <Text strong style={{ fontSize: 13 }}>{n}</Text>
                    </div>
                    <Progress percent={Math.round((n / Math.max(1, brands.length)) * 100)} size="small" showInfo={false} strokeColor="var(--wr-accent)" />
                  </div>
                ))}
              </Space>
            )}
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
            <Space style={{ marginBottom: 12 }}>
              <FileTextOutlined style={{ color: 'var(--wr-success)' }} />
              <Text strong style={{ fontSize: 14 }}>内容资产状态</Text>
              <Tag color="success" style={{ margin: 0 }}>发布率 {statusDist.publishRate}%</Tag>
            </Space>
            {contents.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无内容" />
            ) : (
              <Space direction="vertical" size={10} style={{ width: '100%' }}>
                {statusDist.dist.map(([name, n]) => (
                  <div key={name}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                      <Text style={{ fontSize: 13 }}>{name}</Text>
                      <Text strong style={{ fontSize: 13 }}>{n}</Text>
                    </div>
                    <Progress percent={Math.round((n / contents.length) * 100)} size="small" showInfo={false}
                      strokeColor={name === '已发布' ? 'var(--wr-success)' : name === '草稿' ? 'var(--wr-warning)' : 'var(--wr-text-muted)'} />
                  </div>
                ))}
                <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
                  <GlobalOutlined style={{ marginRight: 4 }} />发布率 = 已发布内容 ÷ 全部内容——发布率越高，AI 可引用的信源越充足
                </Text>
              </Space>
            )}
          </Card>
        </Col>
      </Row>

      {/* F4 需要关注（运营告警面：数据治理 + 沉睡商户 + 草稿积压——只读信号，点击直达对应页） */}
      {(industryMissing > 0 || sleepyMerchants > 0 || draftBacklog > 0) && (
        <Card className="wr-glass-card" styles={{ body: { padding: 16 } }} style={{ marginBottom: 16, borderColor: 'rgba(245,158,11,0.35)' }}>
          <Space wrap size={16}>
            <Text strong style={{ fontSize: 14 }}>需要关注</Text>
            {industryMissing > 0 && (
              <Text style={{ fontSize: 13 }}>{industryMissing} 个品牌未填行业
                <Text type="secondary" style={{ fontSize: 11 }}>（行业看板全落"未分类"）</Text>
              </Text>
            )}
            {sleepyMerchants > 0 && (
              <a style={{ fontSize: 13 }} onClick={() => navigate('/admin/users')}>
                {sleepyMerchants} 个商户 30 天未活跃<Text type="secondary" style={{ fontSize: 11 }}>（流失风险，去跟进 →）</Text>
              </a>
            )}
            {draftBacklog > 0 && (
              <a style={{ fontSize: 13 }} onClick={() => navigate('/admin/contents')}>
                {draftBacklog} 篇内容停留草稿<Text type="secondary" style={{ fontSize: 11 }}>（未进公开站=不可被引用 →）</Text>
              </a>
            )}
          </Space>
        </Card>
      )}

      {/* 行业全景看板（v3 P2：跨商户聚合——对齐 Geowise 行业全景，行业报告/销售素材的数据源） */}
      {industry && (industry.industries.length > 0 || industry.reputation.length > 0 || industry.top_sources.length > 0) && (
        <>
          <Text type="secondary" style={{ fontSize: 12, display: 'block', margin: '24px 0 8px', paddingLeft: 4 }}>行业全景（跨商户聚合）</Text>
          <Row gutter={[16, 16]}>
            <Col xs={24} lg={8}>
              <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
                <Space style={{ marginBottom: 12 }}>
                  <TrophyOutlined style={{ color: 'var(--wr-warning)' }} />
                  <Text strong style={{ fontSize: 14 }}>行业能见度榜</Text>
                  <Tag style={{ margin: 0 }}>按平均提及率</Tag>
                </Space>
                {industry.industries.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无监测数据" />
                ) : (
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    {industry.industries.slice(0, 8).map((i) => (
                      <div key={i.industry}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 3 }}>
                          <Text style={{ fontSize: 13 }}>
                            {i.industry} <Text type="secondary" style={{ fontSize: 11 }}>{i.brand_count} 品牌</Text>
                            {i.industry === '未分类' && industryMissing > 0 && (
                              <Tag color="warning" style={{ margin: '0 0 0 6px', fontSize: 10 }}>品牌未填行业——去引导商户补填</Tag>
                            )}
                          </Text>
                          <Text strong style={{ fontSize: 13 }}>{i.avg_rate.toFixed(0)}%</Text>
                        </div>
                        <Progress percent={Math.round(i.avg_rate)} size="small" showInfo={false} strokeColor="var(--wr-accent)" />
                      </div>
                    ))}
                  </Space>
                )}
              </Card>
            </Col>
            <Col xs={24} lg={8}>
              <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
                <Space style={{ marginBottom: 12 }}>
                  <SmileOutlined style={{ color: 'var(--wr-success)' }} />
                  <Text strong style={{ fontSize: 14 }}>品牌美誉度榜</Text>
                  <Tag style={{ margin: 0 }}>AI 口碑正面占比</Tag>
                </Space>
                {industry.reputation.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无情感数据（需 ≥2 条采样）" />
                ) : (
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    {industry.reputation.map((r, idx) => (
                      <div key={r.brand_name} style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Text style={{ fontSize: 13 }}>
                          <Text strong style={{ color: idx < 3 ? 'var(--wr-warning)' : 'inherit', marginRight: 6 }}>{idx + 1}</Text>
                          {r.brand_name} <Text type="secondary" style={{ fontSize: 11 }}>{r.industry}</Text>
                        </Text>
                        <Text strong style={{ fontSize: 13, color: r.positive_rate >= 60 ? 'var(--wr-success)' : 'var(--wr-text-secondary)' }}>
                          {r.positive_rate}%
                          <Text type="secondary" style={{ fontSize: 11, fontWeight: 400, marginLeft: 4 }}>{r.sample_count} 采样</Text>
                        </Text>
                      </div>
                    ))}
                  </Space>
                )}
              </Card>
            </Col>
            <Col xs={24} lg={8}>
              <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
                <Space style={{ marginBottom: 12 }}>
                  <LinkOutlined style={{ color: 'var(--wr-primary)' }} />
                  <Text strong style={{ fontSize: 14 }}>信源域名榜</Text>
                  <Tag style={{ margin: 0 }}>AI 引用的来源</Tag>
                </Space>
                {industry.top_sources.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无引用来源" />
                ) : (
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    {industry.top_sources.map((s, idx) => (
                      <div key={s.domain} style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Text style={{ fontSize: 13 }} ellipsis>
                          <Text type="secondary" style={{ marginRight: 6 }}>{idx + 1}</Text>
                          {s.domain}
                        </Text>
                        <Text strong style={{ fontSize: 13 }}>{s.count} 次</Text>
                      </div>
                    ))}
                  </Space>
                )}
              </Card>
            </Col>
          </Row>
        </>
      )}
    </div>
  )
}

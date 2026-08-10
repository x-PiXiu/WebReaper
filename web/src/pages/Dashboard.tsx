import { Card, Row, Col, Typography, Spin } from 'antd'
import { Line, Pie, Bar } from '@ant-design/charts'
import { DollarOutlined, CrownOutlined, RiseOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '../store/auth'
import { businessApi } from '../api/business'

const { Text } = Typography

const statusLabel: Record<string, string> = {
  pending_review: '待审核', approved: '已通过', rejected: '已拒绝',
}

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

// 图表卡片容器（统一标题+容器）
function ChartCard({ title, children, height = 280 }: { title: string; children: React.ReactNode; height?: number }) {
  return (
    <Card title={title} styles={{ body: { height, padding: 16 } }}>
      <div style={{ height: height - 40 }}>
        {children}
      </div>
    </Card>
  )
}

// 平台总览：SaaS 平台维度统计（商户/GEO 资产/发布/采集）。
// 每个数字卡片点击跳转到对应管理页；无对应页面的指标（如已发布内容）纯展示。
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

  // 图表数据转换（数据资产明细）
  const trendData = (stats?.daily_trend || []).map(d => ({ date: d.date, value: d.count }))
  const statusData = Object.entries(stats?.status_breakdown || {}).map(([k, v]) => ({
    type: statusLabel[k] || k, value: v,
  }))
  const sourceData = (stats?.source_distribution || []).map(s => ({ type: s.name, value: s.count }))
  const tagData = (stats?.top_tags || []).map(t => ({ name: t.name, value: t.count }))

  // 图表通用配置（用 CSS 变量适配双主题）
  const chartTheme = {
    color: 'var(--wr-text-primary)',
    axis: { common: { labelFill: 'var(--wr-text-muted)', lineStroke: 'var(--wr-border)' } },
  }

  return (
    <div className="wr-page-content">
      {/* 标题 */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 28, fontWeight: 700, margin: 0, letterSpacing: '-0.02em', color: 'var(--wr-text-primary)' }}>
          平台总览{username ? ` · ${username}` : ''}
        </h1>
        <Text type="secondary" style={{ fontSize: 14 }}>
          SaaS 平台核心指标 · GEO 内容引擎 + 数据采集双域运行概况
        </Text>
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
          <StatCard label="商户数" value={stats?.users ?? 0} sublabel="平台注册商户" gradient="linear-gradient(180deg,#6366f1,#4f46e5)" onClick={() => navigate('/admin/users')} />
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
        <Col xs={12} md={6}>
          <StatCard label="已发布公开页" value={stats?.published_contents ?? 0} sublabel="AI 引擎可爬取" gradient="linear-gradient(180deg,#8b5cf6,#7c3aed)" onClick={() => navigate('/admin/contents')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="监测结果" value={stats?.monitor_results ?? 0} sublabel="累计引擎探测" gradient="linear-gradient(180deg,#ec4899,#db2777)" onClick={() => navigate('/admin/brands')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="发布任务" value={stats?.publish_jobs ?? 0} sublabel="多平台分发" gradient="linear-gradient(180deg,#f97316,#ea580c)" onClick={() => navigate('/admin/brands')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="采集数据项" value={stats?.data_items ?? 0} sublabel="数据资产总量" gradient="linear-gradient(180deg,#14b8a6,#0d9488)" onClick={() => navigate('/admin/data')} />
        </Col>
      </Row>

      {/* 数据资产明细：趋势 + 状态 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={14}>
          <ChartCard title="数据项采集趋势（近 14 天）">
            <Line
              data={trendData}
              xField="date" yField="value"
              smooth
              height={240}
              theme={chartTheme as any}
              color="#6366f1"
              areaStyle={{ fill: 'l(270) 0:#6366f180 1:#6366f105' }}
              point={{ size: 3, shape: 'circle' }}
              tooltip={{ name: '采集量' }}
            />
          </ChartCard>
        </Col>
        <Col xs={24} lg={10}>
          <ChartCard title="数据项审核状态分布">
            {statusData.length > 0 ? (
              <Pie
                data={statusData}
                angleField="value" colorField="type"
                height={240}
                radius={0.8} innerRadius={0.5}
                color={['#f59e0b', '#10b981', '#ef4444']}
                label={{ text: 'type', position: 'outside', fill: 'var(--wr-text-secondary)' }}
                theme={chartTheme as any}
              />
            ) : <div style={{ textAlign: 'center', paddingTop: 80 }}><Text type="secondary">暂无数据</Text></div>}
          </ChartCard>
        </Col>
      </Row>

      {/* 数据资产明细：数据源 + 标签 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={10}>
          <ChartCard title="数据源分布">
            {sourceData.length > 0 ? (
              <Pie
                data={sourceData}
                angleField="value" colorField="type"
                height={240}
                radius={0.8}
                label={{ text: 'type', position: 'outside', fill: 'var(--wr-text-secondary)' }}
                theme={chartTheme as any}
              />
            ) : <div style={{ textAlign: 'center', paddingTop: 80 }}><Text type="secondary">暂无数据</Text></div>}
          </ChartCard>
        </Col>
        <Col xs={24} lg={14}>
          <ChartCard title="热门标签 Top 8">
            {tagData.length > 0 ? (
              <Bar
                data={tagData}
                xField="value" yField="name"
                height={240}
                color="#22d3ee"
                theme={chartTheme as any}
                axis={{ y: { labelFill: 'var(--wr-text-secondary)' }, x: { labelFill: 'var(--wr-text-muted)' } } as any}
              />
            ) : <div style={{ textAlign: 'center', paddingTop: 80 }}><Text type="secondary">暂无标签</Text></div>}
          </ChartCard>
        </Col>
      </Row>
    </div>
  )
}

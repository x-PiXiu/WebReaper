import { Card, Row, Col, Typography, Tag, Table, Button, Spin } from 'antd'
import { Line, Pie, Bar } from '@ant-design/charts'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '../store/auth'
import { businessApi } from '../api/business'
import type { DataItem, StatsView } from '../types/api'

const { Text } = Typography

const statusColor: Record<string, string> = {
  pending_review: 'orange', approved: 'green', rejected: 'red',
}
const statusLabel: Record<string, string> = {
  pending_review: '待审核', approved: '已通过', rejected: '已拒绝',
}

// 统计卡片
function StatCard({ label, value, sublabel, gradient, onClick }: {
  label: string; value: string | number; sublabel?: string; gradient: string; onClick?: () => void
}) {
  return (
    <div onClick={onClick} style={{
      position: 'relative', padding: 24, background: 'var(--wr-bg-surface, #121218)',
      border: '1px solid rgba(255,255,255,0.06)', borderRadius: 14,
      cursor: onClick ? 'pointer' : 'default', transition: 'all 200ms', overflow: 'hidden',
    }}
      onMouseEnter={e => { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.12)'; e.currentTarget.style.transform = 'translateY(-2px)' }}
      onMouseLeave={e => { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.06)'; e.currentTarget.style.transform = 'translateY(0)' }}>
      <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 3, background: gradient }} />
      <Text style={{ color: '#71717a', fontSize: 13, display: 'block', marginBottom: 8 }}>{label}</Text>
      <div style={{ fontSize: 32, fontWeight: 700, color: '#e4e4e7', letterSpacing: '-0.03em' }}>{value}</div>
      {sublabel && <Text style={{ color: '#52525b', fontSize: 11 }}>{sublabel}</Text>}
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

export default function Dashboard() {
  const username = useAuthStore(s => s.username)
  const navigate = useNavigate()

  // 统计聚合（一次请求拿全）
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: () => businessApi.getStats(),
  })
  // 最近数据（表格用，独立请求）
  const { data: items = [] } = useQuery({ queryKey: ['data-items'], queryFn: () => businessApi.listDataItems() })

  const totals = stats?.totals || {}
  const pending = totals['pending_review'] || 0
  const approved = totals['approved'] || 0

  // 图表数据转换
  const trendData = (stats?.daily_trend || []).map(d => ({ date: d.date, value: d.count }))
  const statusData = Object.entries(stats?.status_breakdown || {}).map(([k, v]) => ({
    type: statusLabel[k] || k, value: v,
  }))
  const sourceData = (stats?.source_distribution || []).map(s => ({ type: s.name, value: s.count }))
  const tagData = (stats?.top_tags || []).map(t => ({ name: t.name, value: t.count }))

  // 暗色主题图表通用配置
  const darkTheme = {
    color: '#e4e4e7',
    axis: { common: { labelFill: '#71717a', lineStroke: 'rgba(255,255,255,0.08)' } },
  }

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto' }}>
      {/* 标题 */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 28, fontWeight: 700, margin: 0, letterSpacing: '-0.02em' }}>
          数据中心{username ? ` · ${username}` : ''}
        </h1>
        <Text type="secondary" style={{ fontSize: 14 }}>
          {totals['data_items'] || 0} 条数据 · {sourceData.length} 个数据源 · {tagData.length} 个标签
        </Text>
      </div>

      {statsLoading && <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>}

      {/* 统计卡片 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} md={6}>
          <StatCard label="数据总量" value={totals['data_items'] || 0} sublabel={`已通过 ${approved}`} gradient="linear-gradient(180deg,#f59e0b,#d97706)" onClick={() => navigate('/data')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="待审核" value={pending} sublabel={pending > 0 ? '需处理' : '全部已审'} gradient="linear-gradient(180deg,#ec4899,#db2777)" onClick={() => navigate('/data')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="数据源" value={sourceData.length} sublabel="采集来源数" gradient="linear-gradient(180deg,#22d3ee,#0891b2)" onClick={() => navigate('/tools')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="标签类型" value={tagData.length} sublabel="分类维度" gradient="linear-gradient(180deg,#6366f1,#4f46e5)" onClick={() => navigate('/data')} />
        </Col>
      </Row>

      {/* 图表区：趋势 + 状态 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={14}>
          <ChartCard title="采集趋势（近 14 天）">
            <Line
              data={trendData}
              xField="date" yField="value"
              smooth
              height={240}
              theme={darkTheme as any}
              color="#6366f1"
              areaStyle={{ fill: 'l(270) 0:#6366f180 1:#6366f105' }}
              point={{ size: 3, shape: 'circle' }}
              tooltip={{ name: '采集量' }}
            />
          </ChartCard>
        </Col>
        <Col xs={24} lg={10}>
          <ChartCard title="审核状态分布">
            {statusData.length > 0 ? (
              <Pie
                data={statusData}
                angleField="value" colorField="type"
                height={240}
                radius={0.8} innerRadius={0.5}
                color={['#f59e0b', '#10b981', '#ef4444']}
                label={{ text: 'type', position: 'outside', fill: '#a1a1aa' }}
                theme={darkTheme as any}
              />
            ) : <div style={{ textAlign: 'center', paddingTop: 80 }}><Text type="secondary">暂无数据</Text></div>}
          </ChartCard>
        </Col>
      </Row>

      {/* 图表区：数据源 + 标签 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={10}>
          <ChartCard title="数据源分布">
            {sourceData.length > 0 ? (
              <Pie
                data={sourceData}
                angleField="value" colorField="type"
                height={240}
                radius={0.8}
                label={{ text: 'type', position: 'outside', fill: '#a1a1aa' }}
                theme={darkTheme as any}
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
                theme={darkTheme as any}
                axis={{ y: { labelFill: '#a1a1aa' }, x: { labelFill: '#71717a' } } as any}
              />
            ) : <div style={{ textAlign: 'center', paddingTop: 80 }}><Text type="secondary">暂无标签</Text></div>}
          </ChartCard>
        </Col>
      </Row>

      {/* 最近采集 */}
      <Card title="最近采集" extra={<Button type="link" onClick={() => navigate('/data')}>查看全部</Button>}>
        <Table
          dataSource={items.slice(0, 8)} rowKey="id" size="small" pagination={false}
          columns={[
            { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
            { title: '标签', dataIndex: 'tags', key: 'tags', width: 200, render: (tags: string[]) => tags?.slice(0, 3).map(t => <Tag key={t}>{t}</Tag>) },
            { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (s: string) => <Tag color={statusColor[s]}>{statusLabel[s] || s}</Tag> },
          ]}
        />
      </Card>
    </div>
  )
}

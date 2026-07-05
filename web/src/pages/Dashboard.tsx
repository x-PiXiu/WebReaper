import { Card, Row, Col, Typography, Tag, Table, Button } from 'antd'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '../store/auth'
import { businessApi } from '../api/business'
import type { DataItem } from '../types/api'

const { Text } = Typography

const statusColor: Record<string, string> = {
  pending_review: 'orange', approved: 'green', rejected: 'red',
}

// 统计卡片组件
function StatCard({ label, value, sublabel, gradient, onClick }: {
  label: string; value: string | number; sublabel?: string; gradient: string; onClick?: () => void
}) {
  return (
    <div
      onClick={onClick}
      style={{
        position: 'relative', padding: 24, background: 'var(--wr-bg-surface, #121218)',
        border: '1px solid rgba(255,255,255,0.06)', borderRadius: 14,
        cursor: onClick ? 'pointer' : 'default',
        transition: 'all 200ms cubic-bezier(0.2,0,0,1)', overflow: 'hidden',
      }}
      onMouseEnter={e => { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.12)'; e.currentTarget.style.transform = 'translateY(-2px)' }}
      onMouseLeave={e => { e.currentTarget.style.borderColor = 'rgba(255,255,255,0.06)'; e.currentTarget.style.transform = 'translateY(0)' }}
    >
      <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 3, background: gradient }} />
      <Text style={{ color: '#71717a', fontSize: 13, display: 'block', marginBottom: 8 }}>{label}</Text>
      <div style={{ fontSize: 32, fontWeight: 700, color: '#e4e4e7', letterSpacing: '-0.03em' }}>{value}</div>
      {sublabel && <Text style={{ color: '#52525b', fontSize: 11 }}>{sublabel}</Text>}
    </div>
  )
}

export default function Dashboard() {
  const username = useAuthStore(s => s.username)
  const navigate = useNavigate()

  const { data: items = [] } = useQuery({ queryKey: ['data-items'], queryFn: () => businessApi.listDataItems() })
  const { data: agents = [] } = useQuery({ queryKey: ['agent-configs'], queryFn: () => businessApi.listAgentConfigs() })
  const { data: collections = [] } = useQuery({ queryKey: ['collections'], queryFn: () => businessApi.listCollections() })

  const pending = items.filter((i: DataItem) => i.status === 'pending_review').length
  const approved = items.filter((i: DataItem) => i.status === 'approved').length

  return (
    <div>
      <div style={{ marginBottom: 32 }}>
        <h1 style={{ fontSize: 28, fontWeight: 700, margin: 0, letterSpacing: '-0.02em' }}>
          欢迎回来{username ? `, ${username}` : ''}
        </h1>
        <Text type="secondary" style={{ fontSize: 14 }}>通用智能数据采集平台 · Agent 驱动 · 7种爬虫工具</Text>
      </div>

      {/* 统计卡片 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={12} md={6}>
          <StatCard label="Agent 配置" value={agents.length} sublabel="可配置的 Agent" gradient="linear-gradient(180deg,#6366f1,#4f46e5)" onClick={() => navigate('/agent-configs')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="采集集合" value={collections.length} sublabel="采集任务" gradient="linear-gradient(180deg,#22d3ee,#0891b2)" onClick={() => navigate('/tasks')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="数据项" value={items.length} sublabel={`已通过 ${approved}`} gradient="linear-gradient(180deg,#f59e0b,#d97706)" onClick={() => navigate('/data')} />
        </Col>
        <Col xs={12} md={6}>
          <StatCard label="待审核" value={pending} sublabel={pending > 0 ? '需处理' : '全部已审'} gradient="linear-gradient(180deg,#ec4899,#db2777)" onClick={() => navigate('/data')} />
        </Col>
      </Row>

      {/* 最近数据 */}
      <Card title="最近采集" extra={<Button type="link" onClick={() => navigate('/data')}>查看全部</Button>}>
        <Table
          dataSource={items.slice(0, 8)} rowKey="id" size="small" pagination={false}
          columns={[
            { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
            { title: '标签', dataIndex: 'tags', key: 'tags', width: 200, render: (tags: string[]) => tags?.slice(0, 3).map(t => <Tag key={t}>{t}</Tag>) },
            { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (s: string) => <Tag color={statusColor[s]}>{s}</Tag> },
          ]}
        />
      </Card>
    </div>
  )
}

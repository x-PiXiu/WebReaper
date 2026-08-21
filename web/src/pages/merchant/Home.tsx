import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Col, Progress, Row, Space, Typography } from 'antd'
import {
  ArrowRightOutlined, DatabaseOutlined, FundOutlined, SendOutlined,
  UserOutlined, VideoCameraOutlined, AppstoreOutlined,
} from '@ant-design/icons'
import { useBrands } from '../../hooks/useBrands'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import PageLoading from '../../components/PageLoading'

const { Text, Title } = Typography

const JOURNEY = [
  { key: 'persona', label: '人设档案', desc: '定义账号人设与知识', path: '/m/brands', icon: <UserOutlined /> },
  { key: 'assets', label: '资产库', desc: '形象 · 音色 · 分镜 · 封面', path: '/m/assets', icon: <DatabaseOutlined /> },
  { key: 'compose', label: '内容合成', desc: '写文章 / 做视频', path: '/m/compose', icon: <VideoCameraOutlined /> },
  { key: 'works', label: '我的作品', desc: '草稿与成片统一管理', path: '/m/works', icon: <AppstoreOutlined /> },
  { key: 'publish', label: '发布中心', desc: '账号绑定与发布', path: '/m/distribution', icon: <SendOutlined /> },
  { key: 'analytics', label: '作品数据', desc: '播放 · 互动 · 线索', path: '/m/analytics', icon: <FundOutlined /> },
]

/** 工作台：账号 IP 智能体指挥中心 */
export default function MerchantHome() {
  const navigate = useNavigate()
  const { data: brands = [], isLoading } = useBrands()
  // 真实作品库（三源聚合）+ 作品数据汇总——无数据则相关区块自然隐藏
  const { data: works = [] } = useQuery({ queryKey: ['merchant-works'], queryFn: () => businessApi.listWorks().catch(() => []) })
  const { data: summary } = useQuery({ queryKey: ['analytics-summary'], queryFn: () => businessApi.getAnalyticsSummary().catch(() => null), staleTime: 60_000 })

  const published = works.filter((w) => w.status === 'published')
  const ready = works.filter((w) => w.status === 'ready')
  const hasPersona = brands.length > 0
  const hasWork = works.length > 0
  const hasPublished = published.length > 0

  const doneCount = [hasPersona, hasWork, hasPublished].filter(Boolean).length

  const next = useMemo(() => {
    if (!hasPersona) return JOURNEY[0]
    if (!hasWork) return JOURNEY[2]
    if (ready.length > 0) return JOURNEY[3]
    if (!hasPublished) return JOURNEY[4]
    return JOURNEY[5]
  }, [hasPersona, hasWork, ready.length, hasPublished])

  const weekViews = summary?.totals?.views ?? 0

  if (isLoading) return <PageLoading />

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Account IP Agent</p>
          <h1>工作台</h1>
          <p className="ip-lead">围绕账号 IP 的获客闭环——人设、合成、发布与数据一站推进</p>
        </div>
      </div>

      <div className="ip-next-card ip-rise">
        <div style={{ position: 'relative', zIndex: 1 }}>
          <Text type="secondary" style={{ fontSize: 12, letterSpacing: '0.06em' }}>下一步建议</Text>
          <Title level={3} style={{ margin: '8px 0 10px' }}>{next.label}</Title>
          <Text type="secondary">{next.desc}</Text>
        </div>
        <Button
          type="primary"
          size="large"
          className="ip-btn-primary"
          icon={<ArrowRightOutlined />}
          onClick={() => navigate(next.path)}
          style={{ position: 'relative', zIndex: 1 }}
        >
          立即前往
        </Button>
      </div>

      <Row gutter={[16, 16]} className="ip-stagger" style={{ marginTop: 22 }}>
        <Col xs={24} md={8}>
          <div className="ip-metric-card">
            <span className="ip-metric-label">IP 旅程完成度</span>
            <Progress percent={Math.round((doneCount / 3) * 100)} strokeColor={{ '0%': '#2dd4bf', '100%': '#d4a574' }} />
            <Text type="secondary" style={{ fontSize: 12 }}>人设 / 作品 / 发布</Text>
          </div>
        </Col>
        <Col xs={12} md={8}>
          <div className="ip-metric-card">
            <span className="ip-metric-label">作品库</span>
            <strong className="ip-metric-value">{works.length}</strong>
            <Text type="secondary" style={{ fontSize: 12 }}>待发布 {ready.length} · 已发布 {published.length}</Text>
          </div>
        </Col>
        <Col xs={12} md={8}>
          <div className="ip-metric-card">
            <span className="ip-metric-label">近 7 日增长（演示）</span>
            <strong className="ip-metric-value">{weekViews.toLocaleString()}</strong>
            <Text type="secondary" style={{ fontSize: 12 }}>累计播放</Text>
          </div>
        </Col>
      </Row>

      <Title level={5} style={{ marginTop: 32, marginBottom: 14 }}>打造旅程</Title>
      <div className="ip-journey-grid ip-stagger">
        {JOURNEY.map((j, i) => (
          <button key={j.key} type="button" className="ip-journey-card" onClick={() => navigate(j.path)}>
            <span className="ip-journey-index">{String(i + 1).padStart(2, '0')}</span>
            <span className="ip-journey-icon">{j.icon}</span>
            <strong>{j.label}</strong>
            <span>{j.desc}</span>
          </button>
        ))}
      </div>

      <div className="ip-panel ip-rise" style={{ marginTop: 28 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Title level={5} style={{ margin: 0 }}>人设档案</Title>
          <Button type="link" onClick={() => navigate('/m/brands')}>管理</Button>
        </div>
        {brands.length === 0 ? (
          <Text type="secondary">还没有人设——先创建档案，合成时才能贴合账号调性</Text>
        ) : (
          <Space wrap style={{ marginTop: 14 }}>
            {brands.slice(0, 6).map((b) => (
              <button key={b.id} type="button" className="ip-chip" onClick={() => navigate('/m/brands')}>
                {b.name}
              </button>
            ))}
          </Space>
        )}
      </div>
    </div>
  )
}

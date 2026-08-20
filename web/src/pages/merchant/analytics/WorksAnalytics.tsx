import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Col, Row, Table, Tag, Typography } from 'antd'
import { ArrowUpOutlined, FundOutlined } from '@ant-design/icons'
import { LazyColumn, LazyLine } from '../../../components/charts/LazyCharts'
import { MOCK_METRICS } from '../../../mock/ipAssets'
import { useWorksStore } from '../../../store/works'

const { Text, Title } = Typography

/**
 * 作品数据：通用播放 / 互动 / 线索指标（演示假数据 + 本地作品库汇总）。
 * 原「AI 效果」入口并入本页。
 */
export default function WorksAnalytics() {
  const navigate = useNavigate()
  const works = useWorksStore((s) => s.works)
  const published = works.filter((w) => w.status === 'published')

  const totals = useMemo(() => {
    const views = published.reduce((s, w) => s + (w.views || 0), 0)
      + MOCK_METRICS.reduce((s, m) => s + m.views, 0) / 2
    const leads = published.reduce((s, w) => s + (w.leads || 0), 0)
      + MOCK_METRICS.reduce((s, m) => s + m.leads, 0) / 2
    const likes = published.reduce((s, w) => s + (w.likes || 0), 0)
    const engage = MOCK_METRICS[MOCK_METRICS.length - 1]?.engage || 0
    return {
      views: Math.round(views),
      leads: Math.round(leads),
      likes,
      engage,
    }
  }, [published])

  const trend = MOCK_METRICS.flatMap((m) => [
    { day: m.day, type: '播放', value: m.views },
    { day: m.day, type: '线索', value: m.leads * 80 },
  ])

  const engageBars = MOCK_METRICS.map((m) => ({ day: m.day, rate: m.engage }))

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Growth</p>
          <h1>作品数据</h1>
          <p className="ip-lead">播放、互动与线索转化——先看通用指标，再下钻单作品</p>
        </div>
        <Button icon={<FundOutlined />} onClick={() => navigate('/m/works')}>回到作品库</Button>
      </div>

      <Row gutter={[16, 16]} className="ip-metric-row ip-stagger">
        {[
          { label: '近 7 日播放', value: totals.views.toLocaleString(), delta: '+18%' },
          { label: '互动总量', value: totals.likes.toLocaleString(), delta: '+9%' },
          { label: '线索数', value: totals.leads.toLocaleString(), delta: '+22%' },
          { label: '互动率', value: `${totals.engage.toFixed(1)}%`, delta: '+0.6pt' },
        ].map((m) => (
          <Col xs={12} md={6} key={m.label}>
            <div className="ip-metric-card">
              <span className="ip-metric-label">{m.label}</span>
              <strong className="ip-metric-value">{m.value}</strong>
              <span className="ip-metric-delta"><ArrowUpOutlined /> {m.delta}</span>
            </div>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 8 }}>
        <Col xs={24} lg={14}>
          <div className="ip-panel">
            <Title level={5}>播放与线索趋势</Title>
            <LazyLine
              data={trend}
              xField="day"
              yField="value"
              seriesField="type"
              smooth
              height={260}
              color={['#5eead4', '#d4a574']}
            />
          </div>
        </Col>
        <Col xs={24} lg={10}>
          <div className="ip-panel">
            <Title level={5}>互动率（%）</Title>
            <LazyColumn
              data={engageBars}
              xField="day"
              yField="rate"
              height={260}
              style={{ fill: 'l(270) 0:#5eead488 1:#5eead4', radiusTopLeft: 6, radiusTopRight: 6 }}
            />
          </div>
        </Col>
      </Row>

      <div className="ip-panel" style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <Title level={5} style={{ margin: 0 }}>已发布作品</Title>
          <Text type="secondary" style={{ fontSize: 12 }}>演示数据 · 发布后自动汇总</Text>
        </div>
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          dataSource={published}
          locale={{ emptyText: '暂无已发布作品——去合成并向导末步发布' }}
          columns={[
            {
              title: '作品',
              dataIndex: 'title',
              render: (t: string, r: { platform?: string }) => (
                <div>
                  <Text strong>{t}</Text>
                  <div><Tag>{r.platform || '—'}</Tag></div>
                </div>
              ),
            },
            { title: '播放', dataIndex: 'views', render: (v: number) => (v || 0).toLocaleString() },
            { title: '点赞', dataIndex: 'likes', render: (v: number) => (v || 0).toLocaleString() },
            { title: '评论', dataIndex: 'comments', render: (v: number) => (v || 0).toLocaleString() },
            { title: '线索', dataIndex: 'leads', render: (v: number) => <Text style={{ color: 'var(--wr-accent)' }}>{v || 0}</Text> },
            {
              title: '发布时间',
              dataIndex: 'publishedAt',
              render: (t?: string) => (t ? new Date(t).toLocaleString('zh-CN', { hour12: false }) : '—'),
            },
          ]}
        />
      </div>
    </div>
  )
}

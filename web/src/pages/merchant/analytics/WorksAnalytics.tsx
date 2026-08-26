import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Alert, Button, Col, Empty, Modal, Row, Select, Space, Table, Tabs, Tag, Typography } from 'antd'
import { ArrowDownOutlined, ArrowUpOutlined, ExperimentOutlined, FundOutlined, SearchOutlined } from '@ant-design/icons'
import { LazyColumn, LazyLine } from '../../../components/charts/LazyCharts'
import WorkDetailDrawer, { type WorkDetailData } from '../../../components/WorkDetailDrawer'
import { businessApi } from '../../../api/business'
import { useBrandContext } from '../../../hooks/useBrands'
import { MODAL_W, modalBodyScroll } from '../../../ui/modalFit'
import AskTab from '../checkup/AskTab'
import ReportTab from '../checkup/ReportTab'
import RecordsTab from '../checkup/RecordsTab'
import { engineLabel } from '../../../utils/geoTerms'
import type { EngineOption, Keyword, MonitoringResult } from '../../../types/api'

const { Text, Title } = Typography

const PLATFORM_LABEL: Record<string, string> = { douyin: '抖音', kuaishou: '快手', zhihu: '知乎', xiaohongshu: '小红书' }

/** 各引擎最新提及率（品牌维度聚合）。 */
interface EngineStat {
  name: string
  rate: number      // 最新提及率（0~1）
  delta: number     // 较上一条的涨跌（百分点）
  history: { day: string; rate: number }[] // 迷你趋势（按时间升序）
}

function engineStats(results: MonitoringResult[]): EngineStat[] {
  const byEngine = new Map<string, MonitoringResult[]>()
  for (const r of results) {
    if (!r.engine_name) continue
    const list = byEngine.get(r.engine_name) || []
    list.push(r)
    byEngine.set(r.engine_name, list)
  }
  const stats: EngineStat[] = []
  for (const [name, list] of byEngine) {
    list.sort((a, b) => new Date(a.probed_at).getTime() - new Date(b.probed_at).getTime())
    const latest = list[list.length - 1]
    const prev = list.length > 1 ? list[list.length - 2] : null
    stats.push({
      name,
      rate: latest.mention_rate,
      delta: prev ? (latest.mention_rate - prev.mention_rate) * 100 : 0,
      history: list.map((r) => ({ day: new Date(r.probed_at).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }), rate: +(r.mention_rate * 100).toFixed(1) })),
    })
  }
  return stats.sort((a, b) => b.rate - a.rate)
}

/**
 * 作品数据：平台数据（真实发布记录聚合）+ AI 提及（引擎级品牌提及率）+ 作品明细。
 * 滚动叙事：指标卡 → 趋势图 → AI 提及面板 → 已发布作品表。
 * 互动数据有回读则展示真实数值；尚无则显示「平台回读接入后更新」。
 */
export default function WorksAnalytics() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { brands, brandId, setCurrentBrand } = useBrandContext()
  const urlTab = searchParams.get('tab')
  const [aiDrawerOpen, setAiDrawerOpen] = useState(false)
  const [aiTabKey, setAiTabKey] = useState('report')
  const [detail, setDetail] = useState<WorkDetailData | null>(null)

  useEffect(() => {
    if (urlTab === 'ask' || urlTab === 'report' || urlTab === 'records') {
      setAiTabKey(urlTab)
      setAiDrawerOpen(true)
    }
  }, [urlTab])

  const openAiDrawer = (tab = 'report') => {
    setAiTabKey(tab)
    setAiDrawerOpen(true)
    setSearchParams((prev) => {
      const p = new URLSearchParams(prev)
      p.set('tab', tab)
      return p
    }, { replace: true })
  }
  const closeAiDrawer = () => {
    setAiDrawerOpen(false)
    setSearchParams((prev) => {
      const p = new URLSearchParams(prev)
      p.delete('tab')
      return p
    }, { replace: true })
  }
  const setAiTab = (tab: string) => {
    setAiTabKey(tab)
    setSearchParams((prev) => {
      const p = new URLSearchParams(prev)
      p.set('tab', tab)
      return p
    }, { replace: true })
  }

  // 平台数据（真实发布记录聚合）
  const { data: summary, isLoading: summaryLoading } = useQuery({
    queryKey: ['analytics-summary'],
    queryFn: () => businessApi.getAnalyticsSummary(),
  })
  // AI 提及（引擎级监测数据，按当前品牌过滤）
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch((): MonitoringResult[] => []),
  })
  const { data: keywords = [] } = useQuery({
    queryKey: ['geo-all-keywords'],
    queryFn: () => businessApi.listAllKeywords().catch(() => [] as Keyword[]),
  })
  const { data: engineOptions = [] } = useQuery({
    queryKey: ['geo-engines'],
    queryFn: () => businessApi.listEngines().catch(() => [] as EngineOption[]),
  })

  const engines = useMemo(
    () => engineStats(monitorResults.filter((r) => r.brand_id === brandId)),
    [monitorResults, brandId],
  )
  const avgMention = engines.length ? engines.reduce((s, e) => s + e.rate, 0) / engines.length : null

  const works = summary?.works || []
  const trend = (summary?.trend || []).map((p) => ({ day: p.day, 发布数: p.published }))
  const totals = summary?.totals

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Growth</p>
          <h1>作品数据</h1>
          <p className="ip-lead">平台数据与 AI 提及一屏看完——发布效果与获客影响同步追踪</p>
        </div>
        <Space>
          <Select
            style={{ minWidth: 180 }}
            placeholder="选择人设"
            value={brandId}
            onChange={(v) => setCurrentBrand(v)}
            options={brands.map((b) => ({ value: b.id, label: b.name }))}
          />
          <Button icon={<FundOutlined />} onClick={() => navigate('/m/works')}>回到作品库</Button>
        </Space>
      </div>

      <Row gutter={[16, 16]} className="ip-metric-row ip-stagger">
        {[
          { label: '已发布作品', value: (totals?.published ?? 0).toLocaleString(), delta: '' },
          { label: '近 7 日发布', value: (trend.slice(-7).reduce((s, p) => s + p.发布数, 0)).toLocaleString(), delta: '' },
          { label: 'AI 提及率', value: avgMention !== null ? `${(avgMention * 100).toFixed(1)}%` : '—', delta: '' },
          {
            label: '互动总量',
            value: ((totals?.likes ?? 0) + (totals?.comments ?? 0)).toLocaleString(),
            delta: (totals?.likes || totals?.comments) ? '' : '平台回读接入后更新',
          },
        ].map((m) => (
          <Col xs={12} md={6} key={m.label}>
            <div className="ip-metric-card">
              <span className="ip-metric-label">{m.label}</span>
              <strong className="ip-metric-value">{m.value}</strong>
              {m.delta && <span className="ip-metric-delta" style={{ color: 'var(--wr-text-secondary)' }}>{m.delta}</span>}
            </div>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 8 }}>
        <Col xs={24} lg={14}>
          <div className="ip-panel">
            <Title level={5}>发布节奏（近 14 天）</Title>
            {summaryLoading ? null : trend.every((p) => p.发布数 === 0) ? (
              <Empty description="还没有发布记录——去发布中心发出第一条作品" style={{ padding: '40px 0' }} />
            ) : (
              <LazyLine data={trend} xField="day" yField="发布数" smooth height={260} color={['#5eead4']} />
            )}
          </div>
        </Col>
        <Col xs={24} lg={10}>
          <div className="ip-panel">
            <Title level={5}>每日发布（条）</Title>
            <LazyColumn data={trend} xField="day" yField="发布数" height={260} style={{ fill: 'l(270) 0:#5eead488 1:#5eead4', radiusTopLeft: 6, radiusTopRight: 6 }} />
          </div>
        </Col>
      </Row>

      {/* ===== AI 提及面板（同构插入：引擎小卡 + 均值卡，14/10 分栏） ===== */}
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={14}>
          <div className="ip-panel">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <Space>
                <ExperimentOutlined style={{ color: 'var(--wr-accent)' }} />
                <Title level={5} style={{ margin: 0 }}>AI 提及——各大模型怎么推荐你</Title>
              </Space>
              <Button size="small" type="link" onClick={() => openAiDrawer('report')}>查看完整 AI 报告 →</Button>
            </div>
            {engines.length === 0 ? (
              <Empty description="暂无监测数据——AI 效果需要先发起监测" style={{ padding: '32px 0' }} />
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))', gap: 10 }}>
                {engines.slice(0, 6).map((e) => (
                  <div key={e.name} className="ip-metric-card" style={{ padding: 14, minHeight: 96 }}>
                    <span className="ip-metric-label" style={{ fontSize: 12 }}>{engineLabel(e.name)}</span>
                    <strong className="ip-metric-value" style={{ fontSize: 20 }}>{(e.rate * 100).toFixed(0)}%</strong>
                    {e.delta !== 0 && (
                      <span className="ip-metric-delta" style={{ color: e.delta > 0 ? 'var(--wr-success)' : 'var(--wr-danger)' }}>
                        {e.delta > 0 ? <ArrowUpOutlined /> : <ArrowDownOutlined />} {Math.abs(e.delta).toFixed(1)}pt
                      </span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </Col>
        <Col xs={24} lg={10}>
          <div className="ip-panel" style={{ height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>全模型平均提及率</Text>
            <strong style={{ fontSize: 34, fontFamily: '"Noto Serif SC", serif', color: 'var(--wr-text-primary)' }}>
              {avgMention !== null ? `${(avgMention * 100).toFixed(1)}%` : '—'}
            </strong>
            <Text type="secondary" style={{ fontSize: 12, marginTop: 8, lineHeight: 1.6 }}>
              商户问 AI「{brands.find((b) => b.id === brandId)?.industry || '你的行业'}哪家好」时，AI 推荐你的比例
            </Text>
            <Button size="small" style={{ marginTop: 12, alignSelf: 'flex-start' }} onClick={() => openAiDrawer('report')}>
              完整报告与体检记录
            </Button>
            <Button size="small" type="link" icon={<SearchOutlined />} style={{ marginTop: 4, alignSelf: 'flex-start', paddingLeft: 0 }} onClick={() => openAiDrawer('ask')}>
              测一测 AI 推不推荐你 →
            </Button>
          </div>
        </Col>
      </Row>

      {/* ===== 已发布作品（真实发布记录 + 详情入口） ===== */}
      <div className="ip-panel" style={{ marginTop: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <Title level={5} style={{ margin: 0 }}>已发布作品</Title>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {(totals?.likes || totals?.comments) ? '含平台回读互动' : '互动数据随平台回读自动填充'}
          </Text>
        </div>
        <Table
          rowKey="job_id"
          size="middle"
          loading={summaryLoading}
          pagination={false}
          dataSource={works}
          locale={{ emptyText: '暂无已发布作品——去发布中心发出第一条' }}
          columns={[
            {
              title: '作品',
              dataIndex: 'title',
              render: (t: string, r) => (
                <div>
                  <Text strong>{t}</Text>
                  <div>
                    <Tag style={{ margin: 0 }}>{PLATFORM_LABEL[r.platform] || r.platform}</Tag>
                    {r.content_type === 'video' && <Tag style={{ margin: 0 }}>视频</Tag>}
                  </div>
                </div>
              ),
            },
            { title: '播放', dataIndex: 'views', render: (v: number) => (v || 0).toLocaleString() },
            { title: '点赞', dataIndex: 'likes', render: (v: number) => (v || 0).toLocaleString() },
            { title: '评论', dataIndex: 'comments', render: (v: number) => (v || 0).toLocaleString() },
            {
              title: '发布时间',
              dataIndex: 'published_at',
              render: (t?: string) => (t ? new Date(t).toLocaleString('zh-CN', { hour12: false }) : '—'),
            },
            {
              title: '',
              key: 'detail',
              width: 80,
              render: (_, r) => (
                <Button size="small" onClick={() => setDetail({ jobId: r.job_id, title: r.title, platform: r.platform, content_type: r.content_type, external_url: r.external_url, published_at: r.published_at, status: r.status, views: r.views, likes: r.likes, comments: r.comments, shares: r.shares })}>
                  详情
                </Button>
              ),
            },
          ]}
        />
      </div>

      {/* AI 效果完整报告（居中弹窗；checkup 报告/记录/测一测，深链 ?tab=ask|report|records） */}
      <Modal
        open={aiDrawerOpen}
        onCancel={closeAiDrawer}
        width={MODAL_W.xxl}
        title="AI 效果"
        footer={null}
        destroyOnHidden
        className="wr-modal-preview"
        styles={{ body: { ...modalBodyScroll.body, background: 'var(--wr-bg)', paddingTop: 8 } }}
      >
        <Tabs
          activeKey={aiTabKey}
          onChange={setAiTab}
          items={[
            {
              key: 'ask',
              label: '测一测',
              children: <AskTab keywords={keywords} engines={engineOptions} monitorResults={monitorResults} />,
            },
            {
              key: 'report',
              label: '效果报告',
              children: <ReportTab brands={brands} navigate={(p) => { closeAiDrawer(); navigate(p) }} goAsk={() => setAiTab('ask')} />,
            },
            {
              key: 'records',
              label: '效果记录',
              children: <RecordsTab brands={brands} keywords={keywords} monitorResults={monitorResults} />,
            },
          ]}
        />
      </Modal>

      <WorkDetailDrawer open={!!detail} onClose={() => setDetail(null)} work={detail} />

      {(totals?.views ?? 0) === 0 && works.length > 0 && (
        <Alert
          style={{ marginTop: 16 }}
          type="info"
          showIcon
          message="互动数据回读待激活"
          description="发布记录已实时汇总。互动数据（播放/点赞/评论）需对应平台的浏览器通道账号：账号池中给抖音补充「浏览器通道」绑定后，每日自动回读 + 详情页可手动刷新。"
        />
      )}
    </div>
  )
}

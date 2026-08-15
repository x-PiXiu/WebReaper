import { useState } from 'react'
import { Card, Typography, Row, Col, Tag, Button, Space, Progress, Tooltip, List } from 'antd'
import { RocketOutlined, ArrowRightOutlined, BellOutlined, ThunderboltOutlined, CheckCircleOutlined } from '@ant-design/icons'
import PageLoading from '../../components/PageLoading'
import AutoMonitorControl from '../../components/AutoMonitorControl'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { deltaView, latestMonitor, rateColor, rateLabel, mentionDelta } from '../../utils/geo'
import { computeHealth, computeHealthPrev, competitorStats, healthLevel } from '../../utils/geoHealth'
import { useHealthReport } from '../../hooks/useHealthReport'
import HealthScorePanel from '../../components/HealthScorePanel'
import { useBrandOverviews } from '../../hooks/useBrandOverviews'
import { useNotificationList } from '../../hooks/useNotifications'
import { useBrandStore } from '../../store/brand'
import type { Brand } from '../../types/api'

const { Title, Text } = Typography

// 渐进式 Onboarding 步骤（基于实际数据判断 done/pending，非硬编码）。
// 口径与零品牌引导卡完全一致：创建品牌 → 添加关键词 → 发起监测 → 生成内容。
// （"配置竞品"已并入品牌创建流程，不再单列一步）
function useOnboardingSteps(brands: any[], ovData: any[], contentCount: number) {
  const hasBrands = brands.length > 0
  const hasKeywords = ovData.some((o: any) => (o.keyword_count || 0) > 0)
  const hasMonitor = ovData.some((o: any) => (o.trend?.length || 0) > 0)
  const hasContent = contentCount > 0
  const allDone = hasBrands && hasKeywords && hasMonitor && hasContent
  const doneCount = [hasBrands, hasKeywords, hasMonitor, hasContent].filter(Boolean).length
  return { hasBrands, hasKeywords, hasMonitor, hasContent, allDone, doneCount }
}

// 数据驾驶舱：品牌可见度总览（Linear 风大屏感）。
// 数据源：brands + 各品牌 overview（租户级已有接口组合，无新后端依赖）。
export default function MerchantHome() {
  const navigate = useNavigate()
  const setCurrentBrand = useBrandStore((s) => s.setCurrentBrand)
  // Onboarding dismiss 状态必须在所有条件 return 之前（React Hooks 规则）
  const [onboardingDismissed, setOnboardingDismissed] = useState(
    typeof window !== 'undefined' && localStorage.getItem('wr-onboarding-dismissed') === '1'
  )
  const { data: brands = [], isLoading } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const overviews = useBrandOverviews(brands)

  // 待办：未读通知（提及率变化/自动复测/排期发布等主动唤醒信号）
  // 与铃铛/通知中心共享缓存（hooks/useNotifications）
  const { data: notifRes } = useNotificationList()
  const unreadNotifs = (notifRes || []).filter((n: any) => !n.read).slice(0, 3)

  // 配额用量（套餐余量——让商户每次进来感知"我还剩多少额度"）
  const { data: usage } = useQuery({
    queryKey: ['my-usage'],
    queryFn: () => businessApi.getMyUsage().catch(() => null),
    staleTime: 60_000,
  })

  // 工作台汇总：全品牌内容 + 发布任务（与侧栏模块对齐）
  const { data: allContents = [] } = useQuery({
    queryKey: ['geo-contents-all', brands.map((b: Brand) => b.id).join(',')],
    queryFn: async () => {
      const lists = await Promise.all(
        brands.map((b: Brand) => businessApi.listContents(b.id).catch(() => []))
      )
      return lists.flat()
    },
    enabled: brands.length > 0,
    staleTime: 60_000,
  })
  const { data: publishJobs = [] } = useQuery({
    queryKey: ['geo-publish-jobs'],
    queryFn: () => businessApi.listPublishJobs().catch(() => []),
    staleTime: 60_000,
  })
  // 监测结果（驾驶舱竞品对标/情感位次——与 AI 可见度页共享缓存）
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch(() => []),
    staleTime: 60_000,
  })

  const articleCount = allContents.length
  const draftContentCount = allContents.filter((c: { status?: string }) => c.status === 'draft').length
  const publishJobCount = publishJobs.length
  const pendingPublishCount = publishJobs.filter((j: { status?: string }) =>
    j.status === 'pending' || j.status === 'running'
  ).length + draftContentCount

  const ovData = (overviews.data || []) as any[]
  const steps = useOnboardingSteps(brands, ovData, articleCount)

  // 驾驶舱：后端健康报告（单一事实源：总分/五指数/环比/竞品差距一次出全量口径）；
  // 接口不可用时降级本地合成（geoHealth 兜底——灰度兼容旧后端）
  const { report } = useHealthReport()
  const health = report
    ? {
        total: report.total,
        mentionCoverage: report.indicators.mention_coverage,
        sentimentScore: report.indicators.sentiment_score,
        firstPickRate: report.indicators.first_pick_rate,
        contentAsset: report.indicators.content_asset,
        sourceIntegrity: report.indicators.source_integrity,
      }
    : computeHealth(ovData, allContents, articleCount - draftContentCount)
  const prevTotal = report
    ? (report.has_prev ? report.prev_total : null)
    : computeHealthPrev(ovData, allContents, articleCount - draftContentCount)
  const healthDelta = prevTotal === null ? undefined
    : `${health.total - prevTotal >= 0 ? '+' : ''}${(health.total - prevTotal).toFixed(1)}`
  const compStats = competitorStats(monitorResults) // 兜底口径（报告不可用时）
  const competitorGap = report
    ? (report.competitor.size > 0
        ? `${report.competitor.gap_pct >= 0 ? '领先' : '落后'} ${Math.abs(report.competitor.gap_pct).toFixed(1)}%`
        : undefined)
    : (compStats.size > 0
        ? `${compStats.gapPct >= 0 ? '领先' : '落后'} ${Math.abs(compStats.gapPct).toFixed(1)}%`
        : undefined)

  if (isLoading) {
    return <PageLoading tip="工作台加载中..." />
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
              { title: '立即监测', desc: '看 AI 怎么评价你', path: '/m/indexing-report' },
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
            <Button type="primary" size="large" onClick={() => navigate('/m/brands', { state: { openCreate: true } })}>
              创建第一个品牌，开始
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // 渐进式 Onboarding：基于数据判断步骤完成度（有品牌但未完成全流程时显示引导条）
  const showOnboarding = !steps.allDone && !onboardingDismissed && brands.length > 0

  const totalAvg = ovData.length > 0
    ? ovData.reduce((s: number, o: any) => s + (o.avg_mention_rate || 0), 0) / ovData.length
    : 0
  const totalKeywords = ovData.reduce((s: number, o: any) => s + (o.keyword_count || 0), 0)

  // 整体变化对比：各品牌最新 vs 上一次提及率的平均变化（delta）
  // 口径统一：复用 mentionDelta（内部按 probed_at 排序）——不依赖 trend 返回顺序
  const brandDeltas = ovData
    .map((o: any) => mentionDelta((o.trend || []).filter((t: any) => t.mention_rate !== undefined)))
    .filter((d: number | null) => d !== null) as number[]
  const overallDelta = brandDeltas.length > 0
    ? brandDeltas.reduce((s: number, d: number) => s + d, 0) / brandDeltas.length
    : null
  const overallDeltaView = deltaView(overallDelta)

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* 页面标题 */}
        <div className="wr-page-header">
          <h1>工作台</h1>
          <p>业务总览 · {brands.length} 个品牌 · {totalKeywords} 个关键词 · AI 可见度一屏掌握</p>
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
                    { done: steps.hasKeywords, label: '添加关键词', path: '/m/keywords' },
                    { done: steps.hasMonitor, label: '发起监测', path: '/m/indexing-report' },
                    { done: steps.hasContent, label: '生成内容', path: '/m/content' },
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

        {/* 驾驶舱：GEO 健康总分 + 五指数 + 竞品差距（老板 10 秒看懂） */}
        <HealthScorePanel
          total={health.total}
          indicators={[
            { label: '提及覆盖', key: 'coverage', value: health.mentionCoverage, hint: '品牌被 AI 提到的广度（平均提及率）', path: '/m/keywords' },
            { label: '情感指数', key: 'sentiment', value: health.sentimentScore, hint: 'AI 回答中的正面倾向（正/负采样聚合）', path: '/m/indexing-report' },
            { label: '首选提及', key: 'firstPick', value: health.firstPickRate, hint: '品牌在 AI 回答里排第 1 位被推荐的比例（需 ≥3 次采样，不足显示"—"积累中）', path: '/m/indexing-report' },
            { label: '内容资产', key: 'asset', value: health.contentAsset, hint: '已发布内容规模（可被 AI 引用的弹药）', path: '/m/content' },
            { label: '信源完整', key: 'source', value: health.sourceIntegrity, hint: 'AI 实际引用你公开站的比例（归因）', path: '/m/content' },
          ]}
          competitorGap={competitorGap}
          deltaText={healthDelta}
          onNavigate={navigate}
        />

        {/* 工作台汇总卡：对齐侧栏内容模块 */}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }} className="wr-stagger">
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/keywords')} style={{ cursor: 'pointer' }} role="button" tabIndex={0} onKeyDown={(e) => e.key === 'Enter' && navigate('/m/keywords')}>
              <div className="wr-metric-value wr-gradient-text">{totalKeywords}</div>
              <div className="wr-metric-label">关键词</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/brands')} style={{ cursor: 'pointer' }} role="button" tabIndex={0} onKeyDown={(e) => e.key === 'Enter' && navigate('/m/brands')}>
              <div className="wr-metric-value">{brands.length}</div>
              <div className="wr-metric-label">品牌</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/content')} style={{ cursor: 'pointer' }} role="button" tabIndex={0} onKeyDown={(e) => e.key === 'Enter' && navigate('/m/content')}>
              <div className="wr-metric-value">{articleCount}</div>
              <div className="wr-metric-label">内容</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/distribution')} style={{ cursor: 'pointer' }} role="button" tabIndex={0} onKeyDown={(e) => e.key === 'Enter' && navigate('/m/distribution')}>
              <div className="wr-metric-value">{publishJobCount}</div>
              <div className="wr-metric-label">发布任务</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/distribution')} style={{ cursor: 'pointer' }} role="button" tabIndex={0} onKeyDown={(e) => e.key === 'Enter' && navigate('/m/distribution')}>
              <div className="wr-metric-value" style={{ color: pendingPublishCount > 0 ? 'var(--wr-warning)' : undefined }}>{pendingPublishCount}</div>
              <div className="wr-metric-label">待发布</div>
            </div>
          </Col>
          <Col xs={12} sm={8} lg={4}>
            <div className="wr-metric-card" onClick={() => navigate('/m/indexing-report')} style={{ cursor: 'pointer' }} role="button" tabIndex={0} onKeyDown={(e) => e.key === 'Enter' && navigate('/m/indexing-report')}>
              <div className="wr-metric-value" style={{ color: rateColor(totalAvg) }}>
                {(totalAvg * 100).toFixed(1)}<span style={{ fontSize: 16, fontWeight: 600 }}>%</span>
              </div>
              <div className="wr-metric-label">平均提及率</div>
              <div style={{ fontSize: 11, marginTop: 4, fontWeight: 600, color: overallDeltaView.color }}>
                {overallDeltaView.arrow} {overallDeltaView.text} 较上期
              </div>
            </div>
          </Col>
        </Row>

        {/* 自动盯盘状态行（极简感知；完整配置在 AI 可见度矩阵页） */}
        <AutoMonitorControl compact />

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

        {/* 品牌可见度卡片（趋势/分布的深度分析在「AI 可见度」页——含按天/按周时间维度） */}
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
            // 该品牌最新 vs 上一次提及率变化（复用共享纯函数，不依赖 trend 返回顺序）
            const trend = (ov?.trend || []).filter((t: any) => t.mention_rate !== undefined)
            const delta = deltaView(mentionDelta(trend))
            // 最近一次监测的采样次数（置信度传达）
            const lastSample = latestMonitor(trend as any)?.sample_count || 0
            // 单品牌健康分徽章（驾驶舱思想下沉到品牌卡——统一走后端报告口径，
            // 报告不可用时降级本地合成）
            const brandContents = allContents.filter((c: { brand_id: string }) => c.brand_id === b.id)
            const brandPub = brandContents.filter((c: { status?: string }) => c.status === 'published').length
            const bh = report?.brands.find((rb) => rb.brand_id === b.id)?.total
              ?? computeHealth(ov ? [ov] : [], brandContents, brandPub).total
            const bhLv = healthLevel(bh)
            return (
              <Col xs={24} sm={12} lg={8} key={b.id}>
                {/* 品牌卡深链：写入全局品牌上下文再跳转——品牌 Hub 自动预选该品牌（驾驶舱直达思想） */}
                <div
                  className="wr-glass-card"
                  style={{ padding: 22, height: '100%', cursor: 'pointer' }}
                  onClick={() => { setCurrentBrand(b.id); navigate('/m/brands') }}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => e.key === 'Enter' && (setCurrentBrand(b.id), navigate('/m/brands'))}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 18 }}>
                    <div>
                      <Text strong style={{ fontSize: 16, letterSpacing: '-0.01em' }}>{b.name}</Text>
                      {b.positioning && (
                        <Text type="secondary" style={{ display: 'block', marginTop: 4, fontSize: 12.5, lineHeight: 1.5 }}>
                          {b.positioning.length > 46 ? b.positioning.slice(0, 46) + '...' : b.positioning}
                        </Text>
                      )}
                    </div>
                    <Space size={4} align="start">
                      <span className="wr-rate-badge" style={{ background: `${color}1a`, color, borderColor: `${color}33` }}>
                        {rateLabel(rate)}
                      </span>
                      <Tooltip title={`GEO 健康分 ${bh}（${bhLv.label}）`}>
                        <span style={{
                          padding: '2px 8px', borderRadius: 8, fontSize: 11, fontWeight: 700,
                          background: `${bhLv.color}1a`, color: bhLv.color, border: `1px solid ${bhLv.color}33`,
                          whiteSpace: 'nowrap',
                        }}>
                          {bh}
                        </span>
                      </Tooltip>
                    </Space>
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
                      onClick={(e) => { e.stopPropagation(); navigate('/m/indexing-report') }}
                    >
                      查看 AI 提及 →
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

import { useMemo, useState } from 'react'
import { Card, Typography, Row, Col, Button, Space, Progress, Tag } from 'antd'
import { RocketOutlined, ArrowRightOutlined, BellOutlined, ThunderboltOutlined, CheckCircleOutlined, SearchOutlined } from '@ant-design/icons'
import PageLoading from '../../components/PageLoading'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { rateColor, rateLabel, deltaView, mentionDelta } from '../../utils/geo'
import { computeHealth, computeHealthPrev, healthLevel } from '../../utils/geoHealth'
import { useHealthReport } from '../../hooks/useHealthReport'
import { useBrandOverviews } from '../../hooks/useBrandOverviews'
import { useNotificationList } from '../../hooks/useNotifications'
import { timeAgo } from '../../utils/geoTerms'
import { useBrandStore } from '../../store/brand'
import type { Brand } from '../../types/api'

const { Title, Text } = Typography

// 迷你趋势线（sparkline：一条线，无坐标轴——文字管结论、曲线管体感）
function Sparkline({ points }: { points: { d: string; v: number }[] }) {
  if (points.length < 2) return null
  const W = 100, H = 28, PAD = 2
  const max = Math.max(...points.map((p) => p.v), 1)
  const step = (W - PAD * 2) / (points.length - 1)
  const y = (v: number) => H - PAD - (v / max) * (H - PAD * 2)
  const path = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${(PAD + i * step).toFixed(1)},${y(p.v).toFixed(1)}`).join(' ')
  const last = points[points.length - 1]
  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', gap: 10, marginTop: 8 }}>
      <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" style={{ width: 180, height: 44, flexShrink: 0 }}>
        <path d={path} fill="none" stroke="var(--wr-primary)" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" />
        <circle cx={PAD + (points.length - 1) * step} cy={y(last.v)} r={2.2} fill="var(--wr-primary)" />
      </svg>
      <Text type="secondary" style={{ fontSize: 11, lineHeight: 1.5, paddingBottom: 2 }}>
        最近 {points.length} 天走势<br />（每天所有问答的平均提及率）
      </Text>
    </div>
  )
}

// 渐进式 Onboarding（四步主线：建档案 → 做体检 → 造内容 → 发出去）。
// 基于实际数据判断 done/pending，非硬编码；口径与零品牌引导卡一致。
function useOnboardingSteps(brands: any[], ovData: any[], contentCount: number, publishedCount: number, publishJobCount: number) {
  const hasBrands = brands.length > 0
  const hasMonitor = ovData.some((o: any) => (o.trend?.length || 0) > 0)
  const hasContent = contentCount > 0
  const hasPublished = publishedCount > 0 || publishJobCount > 0
  const allDone = hasBrands && hasMonitor && hasContent && hasPublished
  const doneCount = [hasBrands, hasMonitor, hasContent, hasPublished].filter(Boolean).length
  return { hasBrands, hasMonitor, hasContent, hasPublished, allDone, doneCount }
}

// 最近一次体检摘要（服务端监测数据实时计算——换设备也在，不依赖本地存储）：
// 取最新一条监测的时间戳，30 分钟窗口内的结果视为同一批体检。
function useLastCheck(monitorResults: any[]) {
  return (() => {
    if (!monitorResults || monitorResults.length === 0) return null
    const t = (r: any) => new Date(r.probed_at).getTime()
    let latestT = 0
    for (const r of monitorResults) {
      const ts = t(r)
      if (ts > latestT) latestT = ts
    }
    const batch = monitorResults.filter((r: any) => latestT - t(r) <= 30 * 60 * 1000)
    const questions = new Set(batch.map((r: any) => r.keyword_id)).size
    const engines = new Set(batch.map((r: any) => r.engine_name || 'default')).size
    const mentioned = batch.filter((r: any) => (r.mention_rate || 0) > 0).length
    return { questions, engines, mentioned, total: batch.length, at: new Date(latestT) }
  })()
}

// 工作台 = 任务指挥中心（傻瓜化 v3）：只回答老板的三个问题——
// "我现在该干嘛？最近有没有变好？谁在等我处理？"
// 数字的家在各业务页（体检报告/内容中心），这里只留结论与行动：
// ① 下一步任务卡 ② 最近体检摘要+健康分一行 ③ 待办与用量 ④ 品牌一句话结论卡。
// （原健康分五指数面板只在体检报告保留；6 指标卡行删除——各页自有完整口径。）
export default function MerchantHome() {
  const navigate = useNavigate()
  const setCurrentBrand = useBrandStore((s) => s.setCurrentBrand)
  const [onboardingDismissed, setOnboardingDismissed] = useState(
    typeof window !== 'undefined' && localStorage.getItem('wr-onboarding-dismissed') === '1'
  )
  const { data: brands = [], isLoading } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const overviews = useBrandOverviews(brands)

  // 待办：未读通知（今日清单的事件来源——数据推导建议见 todayTodos）
  const { data: notifRes } = useNotificationList()

  // 配额用量（套餐余量）
  const { data: usage } = useQuery({
    queryKey: ['my-usage'],
    queryFn: () => businessApi.getMyUsage().catch(() => null),
    staleTime: 60_000,
  })

  // 全品牌内容 + 发布任务（Onboarding 完成度判定）
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
  // 监测结果（最近体检摘要 + 品牌结论的数据源——与 AI 体检页共享缓存）
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch(() => []),
    staleTime: 60_000,
  })

  const articleCount = allContents.length
  const publishedCount = allContents.filter((c: { status?: string }) => c.status === 'published').length
  const publishJobCount = publishJobs.length

  const ovData = (overviews.data || []) as any[]
  const steps = useOnboardingSteps(brands, ovData, articleCount, publishedCount, publishJobCount)
  const lastCheck = useLastCheck(monitorResults)

  // 7 天走势（按天聚合全量问答的平均提及率——一条线，克制在 sparkline 以内）
  const spark = useMemo(() => {
    const days = new Map<string, { sum: number; n: number }>()
    for (const r of monitorResults) {
      const d = (r.probed_at || '').slice(0, 10)
      if (!d) continue
      const cur = days.get(d) || { sum: 0, n: 0 }
      cur.sum += (r.mention_rate || 0)
      cur.n++
      days.set(d, cur)
    }
    return Array.from(days.entries())
      .sort((a, b) => (a[0] < b[0] ? -1 : 1))
      .slice(-7)
      .map(([d, v]) => ({ d, v: Math.round((v.sum / v.n) * 1000) / 10 }))
  }, [monitorResults])

  // 今日清单（傻瓜化：事件置顶 + 数据推导——清单永远有内容，死盒消失）
  const todayTodos = useMemo(() => {
    const items: { text: string; path?: string; action?: string; event?: boolean }[] = []
    // 事件通知置顶（提及率下降/竞品反超等）
    ;(notifRes || []).filter((n: any) => !n.read).slice(0, 2).forEach((n: any) => {
      items.push({ text: n.title, path: n.link, action: '去处理', event: true })
    })
    // 数据推导（数据自己会说话）
    let lastT = 0
    for (const r of monitorResults) {
      const ts = new Date(r.probed_at).getTime() || 0
      if (ts > lastT) lastT = ts
    }
    const daysSince = lastT ? Math.floor((Date.now() - lastT) / 86400000) : -1
    if (daysSince < 0) {
      items.push({ text: '还没做过体检——问一句，看 AI 会不会推荐你', path: '/m/checkup?tab=ask', action: '去问问 AI' })
    } else if (daysSince >= 7) {
      items.push({ text: `距上次体检 ${daysSince} 天了，该复测看看变化`, path: '/m/checkup?tab=ask', action: '去复测' })
    }
    const drafts = allContents.filter((c: any) => c.status === 'draft')
    if (drafts.length > 0) {
      items.push({ text: `有 ${drafts.length} 篇草稿还没发布，AI 找不到它们`, path: '/m/studio', action: '去发布' })
    }
    const totalKeywords = ovData.reduce((s: number, o: any) => s + (o.keyword_count || 0), 0)
    if (brands.length > 0 && totalKeywords < 3) {
      items.push({ text: `关键词只有 ${totalKeywords} 个，再添几个体检更全面`, path: '/m/checkup?tab=records&sub=questions', action: '去添加' })
    }
    const lowScore = allContents.find((c: any) =>
      c.status === 'published' && (c.score?.total ?? 0) > 0 && (c.score?.total ?? 0) < 50)
    if (lowScore) {
      items.push({ text: `「${(lowScore.title || '未命名内容').slice(0, 14)}」评分 ${Math.round(lowScore.score.total)}，优化后更易被引用`, path: '/m/studio', action: '去优化' })
    }
    return items
  }, [monitorResults, allContents, ovData, brands.length, notifRes])

  // 健康分：工作台只留一行结论（"72 分，比上周 +3"）——完整五指数在体检报告
  const { report } = useHealthReport()
  const health = report
    ? report.total
    : computeHealth(ovData, allContents, publishedCount).total
  const prevTotal = report
    ? (report.has_prev ? report.prev_total : null)
    : computeHealthPrev(ovData, allContents, publishedCount)
  const healthDeltaText = prevTotal === null ? undefined
    : `${health - prevTotal >= 0 ? '+' : ''}${(health - prevTotal).toFixed(1)}`

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
              现在用户问"XX哪家好"都问 AI 了——10 次回答里提到你几次？按下面四步走完，你的第一份 AI 体检报告就出来了。
            </Text>
          </div>

          {/* 快速见效步骤（四步主线：菜单同序——走完即见效） */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12, marginBottom: 32 }}>
            {[
              { title: '① 建档案', desc: '品牌名+定位，2 分钟', path: '/m/brands' },
              { title: '② 做体检', desc: '问一句，看 AI 怎么回答', path: '/m/checkup' },
              { title: '③ 造内容', desc: '让 AI 写容易被引用的内容', path: '/m/studio' },
              { title: '④ 发出去', desc: '发布到公开站和社媒', path: '/m/distribution' },
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

  // 渐进式 Onboarding：基于数据判断步骤完成度
  const showOnboarding = !steps.allDone && !onboardingDismissed

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* 页面标题 */}
        <div className="wr-page-header">
          <h1>工作台</h1>
          <p>今天该做什么，一眼看清</p>
        </div>

        {/* ① 下一步任务卡（四步主线：只告诉用户"现在做这一件事"） */}
        {showOnboarding && (() => {
          const onboardingSteps = [
            { done: steps.hasBrands, label: '建档案', path: '/m/brands', cta: '告诉 AI 你是谁——品牌名和定位就行' },
            { done: steps.hasMonitor, label: '做体检', path: '/m/checkup', cta: '问一句，看看 AI 现在会不会推荐你' },
            { done: steps.hasContent, label: '造内容', path: '/m/studio', cta: '让 AI 写一篇容易被引用的内容' },
            { done: steps.hasPublished, label: '发出去', path: '/m/distribution', cta: '发布到公开站和知乎/小红书，AI 才能引用' },
          ]
          const nextStep = onboardingSteps.find((s) => !s.done)
          if (!nextStep) return null
          return (
            <Card className="wr-glass-card" style={{ marginBottom: 16, borderColor: 'rgba(124,108,255,0.2)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
                <div style={{ flexShrink: 0 }}>
                  <Progress type="circle" percent={(steps.doneCount / 4) * 100} size={48} strokeColor="var(--wr-primary)" />
                </div>
                <div style={{ flex: 1, minWidth: 240 }}>
                  <Text strong style={{ fontSize: 14, display: 'block', marginBottom: 4 }}>
                    你的下一步（第 {steps.doneCount + 1}/4 步）：{nextStep.label}
                  </Text>
                  <Text type="secondary" style={{ fontSize: 12.5, display: 'block', marginBottom: 8 }}>
                    {nextStep.cta}——四步走完，AI 就有理由推荐你了
                  </Text>
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                    {onboardingSteps.map((s, i) => (
                      <Tag
                        key={i}
                        style={{ margin: 0, fontSize: 11 }}
                        color={s.done ? 'success' : s === nextStep ? 'processing' : 'default'}
                      >
                        {s.done ? '✓' : `${i + 1}.`}{s.label}
                      </Tag>
                    ))}
                  </div>
                </div>
                <Button type="primary" onClick={() => navigate(nextStep.path)}>
                  去完成 <ArrowRightOutlined style={{ fontSize: 11 }} />
                </Button>
                <Button
                  size="small" type="text"
                  onClick={() => {
                    setOnboardingDismissed(true)
                    localStorage.setItem('wr-onboarding-dismissed', '1')
                  }}
                >关闭引导</Button>
              </div>
            </Card>
          )
        })()}

        {/* ② 最近体检摘要（服务端数据实时算：最近有没有变好） */}
        <Card className="wr-glass-card" styles={{ body: { padding: 16 } }} style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 10 }}>
            <Space size={8}>
              <SearchOutlined style={{ color: 'var(--wr-primary)' }} />
              <Text strong style={{ fontSize: 14 }}>最近一次体检</Text>
              {healthDeltaText && (
                <Tag color="processing" style={{ margin: 0, fontSize: 11 }}>
                  健康分 {health.toFixed(0)}，比上周 {healthDeltaText}
                </Tag>
              )}
            </Space>
            <Button size="small" type="link" onClick={() => navigate('/m/checkup?tab=report')}>
              看体检报告 <ArrowRightOutlined style={{ fontSize: 10 }} />
            </Button>
          </div>
          {lastCheck ? (
            <>
              <Text style={{ fontSize: 13.5, display: 'block', marginTop: 6, color: 'var(--wr-text-secondary)' }}>
                {timeAgo(lastCheck.at.toISOString())}问了 <b>{lastCheck.questions}</b> 个问题、动用了 <b>{lastCheck.engines}</b> 个 AI，其中 <b style={{ color: lastCheck.mentioned > 0 ? 'var(--wr-success)' : 'var(--wr-danger)' }}>{lastCheck.mentioned}</b> 次提到了你（共 {lastCheck.total} 次问答）
              </Text>
              <Sparkline points={spark} />
            </>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 6, flexWrap: 'wrap' }}>
              <Text type="secondary" style={{ fontSize: 13.5 }}>还没体检过——问一句，看 AI 会不会推荐你</Text>
              <Button size="small" type="primary" onClick={() => navigate('/m/checkup?tab=ask')}>去问问 AI</Button>
            </div>
          )}
        </Card>

        {/* ③ 今日清单（事件置顶 + 数据推导——清单永远有内容，死盒消失） */}
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} lg={14}>
            <Card className="wr-glass-card" styles={{ body: { padding: 16 } }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <Space size={6}>
                  <BellOutlined style={{ color: 'var(--wr-accent)' }} />
                  <Text strong style={{ fontSize: 14 }}>今日该做的</Text>
                </Space>
                <Text type="secondary" style={{ fontSize: 11 }}>系统根据你的数据状态整理</Text>
              </div>
              {todayTodos.length > 0 ? (
                <Space direction="vertical" size={0} style={{ width: '100%' }}>
                  {todayTodos.slice(0, 5).map((t, i) => (
                    <div
                      key={i}
                      style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '7px 0', borderBottom: i < Math.min(todayTodos.length, 5) - 1 ? '1px dashed var(--wr-border)' : 'none', cursor: t.path ? 'pointer' : 'default' }}
                      onClick={() => t.path && navigate(t.path)}
                    >
                      {t.event ? (
                        <Tag color={t.text?.includes('下降') ? 'error' : t.text?.includes('反超') ? 'warning' : 'processing'} style={{ margin: 0, fontSize: 11, flexShrink: 0 }}>事件</Tag>
                      ) : (
                        <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--wr-accent)', flexShrink: 0 }} />
                      )}
                      <Text ellipsis style={{ flex: 1, fontSize: 13, color: 'var(--wr-text-secondary)' }}>{t.text}</Text>
                      {t.path && <a style={{ fontSize: 12, flexShrink: 0 }} onClick={(e) => { e.stopPropagation(); navigate(t.path!) }}>{t.action || '前往'} <ArrowRightOutlined style={{ fontSize: 10 }} /></a>}
                    </div>
                  ))}
                </Space>
              ) : (
                <Text type="secondary" style={{ fontSize: 13 }}>
                  <CheckCircleOutlined style={{ color: 'var(--wr-success)', marginRight: 6 }} />
                  状态很好——保持每周体检一次的习惯，有变化会第一时间告诉你
                </Text>
              )}
            </Card>
          </Col>
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
                  const labels: Record<string, string> = { monitor: '体检', 'content-gen': '写内容', 'content-opt': '改内容', chat: '对话' }
                  return (
                    <Col span={12} key={scene}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                        <Text type="secondary" style={{ fontSize: 11 }}>{labels[scene] || scene}</Text>
                        <Text style={{ fontSize: 11, color: pct >= 100 ? 'var(--wr-danger)' : 'var(--wr-text-muted)' }}>{unlimited ? '无限' : `${u.used}/${u.limit}`}</Text>
                      </div>
                      {!unlimited && <Progress percent={pct} size="small" showInfo={false} strokeColor={pct >= 100 ? 'var(--wr-danger)' : pct >= 80 ? 'var(--wr-warning)' : 'var(--wr-accent)'} />}
                    </Col>
                  )
                })}
              </Row>
            </Card>
          </Col>
        </Row>

        {/* ④ 品牌一句话结论卡（详细数据在品牌档案/AI 体检） */}
        <div style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Title level={5} style={{ color: 'var(--wr-text-secondary)', fontWeight: 600, marginBottom: 0, fontSize: 14 }}>
            我的品牌
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
            const trend = (ov?.trend || []).filter((t: any) => t.mention_rate !== undefined)
            const delta = deltaView(mentionDelta(trend))
            const brandContents = allContents.filter((c: { brand_id: string }) => c.brand_id === b.id)
            const brandPub = brandContents.filter((c: { status?: string }) => c.status === 'published').length
            const bh = report?.brands.find((rb) => rb.brand_id === b.id)?.total
              ?? computeHealth(ov ? [ov] : [], brandContents, brandPub).total
            const bhLv = healthLevel(bh)
            return (
              <Col xs={24} sm={12} lg={8} key={b.id}>
                <div
                  className="wr-glass-card"
                  style={{ padding: 18, height: '100%', cursor: 'pointer', display: 'flex', flexDirection: 'column', gap: 10 }}
                  onClick={() => { setCurrentBrand(b.id); navigate('/m/brands') }}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => e.key === 'Enter' && (setCurrentBrand(b.id), navigate('/m/brands'))}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Text strong style={{ fontSize: 16 }}>{b.name}</Text>
                    <span style={{
                      padding: '2px 9px', borderRadius: 8, fontSize: 12, fontWeight: 700,
                      background: `${bhLv.color}1a`, color: bhLv.color, border: `1px solid ${bhLv.color}33`,
                    }}>
                      {bh} 分
                    </span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, flexWrap: 'wrap' }}>
                    <span className="wr-rate-badge" style={{ background: `${color}1a`, color, borderColor: `${color}33` }}>
                      {rateLabel(rate)}
                    </span>
                    <Text strong style={{ fontSize: 18, color }}>{(rate * 100).toFixed(0)}%</Text>
                    <Text type="secondary" style={{ fontSize: 11 }}>AI 提到你</Text>
                    <span style={{ fontSize: 12, fontWeight: 700, color: delta.color }}>
                      {delta.arrow} {delta.text}
                    </span>
                  </div>
                  <Text type="secondary" style={{ fontSize: 12, marginTop: 'auto' }}>
                    点击进入品牌档案 <ArrowRightOutlined style={{ fontSize: 10 }} />
                  </Text>
                </div>
              </Col>
            )
          })}
        </Row>
      </div>
    </div>
  )
}

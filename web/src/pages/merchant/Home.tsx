import { lazy, Suspense, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Button, Col, Row, Segmented, Space, Spin, Tag, Typography } from 'antd'
import {
  ArrowRightOutlined, FireOutlined, LinkOutlined, RobotOutlined, SendOutlined,
  VideoCameraOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import { useBrands } from '../../hooks/useBrands'
import { usePublishableWorks } from '../../hooks/usePublishableWorks'
import { businessApi } from '../../api/business'
import { useComposeDraft } from '../../store/composeDraft'
import { composeProgressLabel, hasComposeDraft, composeResumePath } from '../../utils/composeProgress'
import QueryBoundary from '../../components/QueryBoundary'
import { GrowthStagesNav } from '../../components/GrowthStagesNav'
import { useGenerationTasks } from '../../hooks/useGenerationTasks'
import { inferGrowthStage } from '../../utils/growthStage'
import { PRODUCT } from '../../config/product'
import {
  CITY_HOTSPOTS, PROVINCE_HEAT, provinceByName, type CityHotspot,
} from '../../data/hotspots'

// 地图组件懒加载：echarts 体积大（约 1MB），不阻塞首页首屏
const ChinaHotMap = lazy(() => import('../../components/ChinaHotMap'))

const { Text, Title } = Typography

type MetricKey = 'heat' | 'leads' | 'posts'

/** 工作台：中国地图获客热力 + 城市热点参数（热力数据空则隐藏演示指标，作品走真实 API） */
export default function MerchantHome() {
  const navigate = useNavigate()
  const { data: brands = [], isLoading, isError, refetch: refetchBrands } = useBrands()
  const { works = [] } = usePublishableWorks({ staleTime: 60_000 })
  const { data: summary } = useQuery({
    queryKey: ['analytics-summary'],
    queryFn: () => businessApi.getAnalyticsSummary().catch(() => null),
    staleTime: 60_000,
  })
  const { tasks: genTasks = [] } = useGenerationTasks({ refetchInterval: false })
  const draft = useComposeDraft()

  const [metric, setMetric] = useState<MetricKey>('heat')
  const [hotspot, setHotspot] = useState<CityHotspot | null>(CITY_HOTSPOTS[0] ?? null)
  const [provinceName, setProvinceName] = useState<string | null>(null)

  const published = works.filter((w) => w.status === 'published')
  const ready = works.filter((w) => w.status === 'ready')
  const hasDraft = hasComposeDraft(draft)
  const hasLipsyncVideo = genTasks.some(
    t => t.sub_type === 'lip_sync' && t.state === 'success' && (t.creations?.length ?? 0) > 0,
  )
  const draftProgress = hasDraft ? composeProgressLabel(draft, draft.track) : ''
  const draftResumePath = composeResumePath(draft)
  const draftResumeLabel = draft.track === 'lipsync' ? '拍口播' : draft.track === 'graphic' ? '发图文' : '发视频'
  const weekViews = summary?.totals?.views ?? 0
  const hasHeatData = PROVINCE_HEAT.length > 0 || CITY_HOTSPOTS.length > 0
  const growthCurrent = inferGrowthStage({
    brandCount: brands.length,
    hasContent: hasLipsyncVideo || works.length > 0,
    readyCount: ready.length,
    publishedCount: published.length,
  })

  const national = useMemo(() => {
    const leads = PROVINCE_HEAT.reduce((s, p) => s + p.leads, 0)
    const posts = PROVINCE_HEAT.reduce((s, p) => s + p.posts, 0)
    const avgHeat = PROVINCE_HEAT.length
      ? Math.round(PROVINCE_HEAT.reduce((s, p) => s + p.heat, 0) / PROVINCE_HEAT.length)
      : 0
    return { leads, posts, avgHeat, hotCities: CITY_HOTSPOTS.length }
  }, [])

  const topCities = useMemo(() => {
    const key = metric
    return [...CITY_HOTSPOTS].sort((a, b) => b[key] - a[key]).slice(0, 5)
  }, [metric])

  const province = provinceName ? provinceByName(provinceName) : null

  return (
    <QueryBoundary loading={isLoading} error={isError} onRetry={() => refetchBrands()}>
      <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">{PRODUCT.nameEn}</p>
          <h1>工作台</h1>
          <p className="ip-lead">
            {hasHeatData
              ? '全国获客热力与城市热点——点选地图查看线索、发布量与对标话题'
              : '从灵感到成片、发布与数据回看——先拍口播或快速生成，打通获客主链路'}
          </p>
        </div>
        <Space wrap>
          <Button icon={<LinkOutlined />} onClick={() => navigate('/m/inspire')}>灵感广场</Button>
          <Button type="primary" className="ip-btn-primary" icon={<RobotOutlined />} onClick={() => navigate('/m/compose/lipsync')}>
            拍口播视频
          </Button>
        </Space>
      </div>

      <GrowthStagesNav current={growthCurrent} className="ch-growth ip-stagger" style={{ marginBottom: 16 }} />

      <Row gutter={[16, 16]} className="ip-stagger" style={{ marginBottom: 16 }}>
        {hasHeatData ? (
          <>
            <Col xs={12} md={6}>
              <div className="ip-metric-card">
                <span className="ip-metric-label">全国均热度</span>
                <strong className="ip-metric-value">{national.avgHeat}</strong>
                <Text type="secondary" style={{ fontSize: 12 }}>省份获客热力均值</Text>
              </div>
            </Col>
            <Col xs={12} md={6}>
              <div className="ip-metric-card">
                <span className="ip-metric-label">近 7 日线索</span>
                <strong className="ip-metric-value">{national.leads}</strong>
                <Text type="secondary" style={{ fontSize: 12 }}>热力线索参数</Text>
              </div>
            </Col>
          </>
        ) : (
          <>
            <Col xs={12} md={6}>
              <div className="ip-metric-card">
                <span className="ip-metric-label">累计播放</span>
                <strong className="ip-metric-value">{weekViews.toLocaleString()}</strong>
                <Text type="secondary" style={{ fontSize: 12 }}>作品数据汇总</Text>
              </div>
            </Col>
            <Col xs={12} md={6}>
              <div className="ip-metric-card">
                <span className="ip-metric-label">待发作品</span>
                <strong className="ip-metric-value">{ready.length}</strong>
                <Text type="secondary" style={{ fontSize: 12 }}>草稿 / 成片就绪</Text>
              </div>
            </Col>
          </>
        )}
        <Col xs={12} md={6}>
          <div className="ip-metric-card">
            <span className="ip-metric-label">已发作品</span>
            <strong className="ip-metric-value">{published.length}</strong>
            <Text type="secondary" style={{ fontSize: 12 }}>作品库已发布</Text>
          </div>
        </Col>
        <Col xs={12} md={6}>
          <div className="ip-metric-card">
            <span className="ip-metric-label">人设档案</span>
            <strong className="ip-metric-value">{brands.length}</strong>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {CITY_HOTSPOTS.length > 0 ? `热点城市 ${CITY_HOTSPOTS.length}` : '账号 IP'}
            </Text>
          </div>
        </Col>
      </Row>

      {brands.length > 0 && !hasLipsyncVideo && !hasDraft && (
        <div className="ip-onboard-card ip-stagger" style={{ marginBottom: 16 }}>
          <h2 style={{ margin: '0 0 8px', fontSize: 18 }}>还没拍过口播视频？</h2>
          <p style={{ margin: '0 0 16px', color: 'var(--wr-text-secondary)' }}>
            三步搞定：提取爆款文案 → 选出镜 → 一键成片，系统自动完成对口型与合成。
          </p>
          <Space wrap>
            <Button type="primary" size="large" className="ip-btn-primary" icon={<VideoCameraOutlined />} onClick={() => navigate('/m/compose/lipsync')}>
              开始拍口播
            </Button>
            <Button size="large" icon={<ThunderboltOutlined />} onClick={() => navigate('/m/compose/quick')}>
              快速生成
            </Button>
          </Space>
        </div>
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          {hasHeatData ? (
            <div className="ip-panel ip-rise" style={{ padding: '12px 12px 4px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap', padding: '4px 8px 8px' }}>
                <Space size={8}>
                  <FireOutlined style={{ color: 'var(--wr-accent)' }} />
                  <Text strong>获客热力图</Text>
                  <Tag style={{ margin: 0 }}>可缩放拖拽</Tag>
                </Space>
                <Segmented
                  size="small"
                  value={metric}
                  onChange={(v) => setMetric(v as MetricKey)}
                  options={[
                    { label: '按热度榜', value: 'heat' },
                    { label: '按线索榜', value: 'leads' },
                    { label: '按发布榜', value: 'posts' },
                  ]}
                />
              </div>
              <Suspense
                fallback={
                  <div style={{ height: 520, display: 'grid', placeItems: 'center' }}>
                    <Spin tip="地图加载中…" />
                  </div>
                }
              >
                <ChinaHotMap
                  height={520}
                  selectedId={hotspot?.id}
                  onSelectHotspot={(h) => {
                    setHotspot(h)
                    if (h) setProvinceName(h.province)
                  }}
                  onSelectProvince={(name) => {
                    setProvinceName(name)
                    const city = CITY_HOTSPOTS.find((c) => c.province === name || name.startsWith(c.province))
                    if (city) setHotspot(city)
                  }}
                />
              </Suspense>
            </div>
          ) : (
            <div className="ip-panel ip-rise" style={{ padding: 24 }}>
              <Space size={8} style={{ marginBottom: 12 }}>
                <ThunderboltOutlined style={{ color: 'var(--wr-accent)' }} />
                <Text strong>今日动作</Text>
              </Space>
              <Title level={4} style={{ margin: '0 0 8px' }}>用真实作品与发布推进获客</Title>
              <Text type="secondary" style={{ display: 'block', marginBottom: 20 }}>
                热力地图数据尚未接入——先完成「创作 → 作品 → 发布」，数据会汇总到播放与互动指标。
              </Text>
              <Space wrap>
                <Button type="primary" className="ip-btn-primary" icon={<VideoCameraOutlined />} onClick={() => navigate('/m/compose/lipsync')}>
                  拍口播
                </Button>
                <Button icon={<ThunderboltOutlined />} onClick={() => navigate('/m/compose/quick')}>快速生成</Button>
                <Button icon={<LinkOutlined />} onClick={() => navigate('/m/inspire')}>灵感广场</Button>
                <Button icon={<SendOutlined />} onClick={() => navigate('/m/distribution?contentType=video')}>去发布</Button>
                <Button type="link" onClick={() => navigate('/m/analytics')}>查看作品数据 →</Button>
              </Space>
              <div className="ip-hotspot-params" style={{ marginTop: 24 }}>
                <div><span>已发</span><strong>{published.length}</strong></div>
                <div><span>待发</span><strong>{ready.length}</strong></div>
                <div><span>播放</span><strong>{weekViews.toLocaleString()}</strong></div>
                <div><span>点赞</span><strong>{(summary?.totals?.likes ?? 0).toLocaleString()}</strong></div>
              </div>
            </div>
          )}
        </Col>

        <Col xs={24} lg={8}>
          <div className="ip-panel ip-rise" style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>{hasHeatData ? '当前热点' : '快捷入口'}</Text>
            {hotspot ? (
              <>
                <Title level={4} style={{ margin: '6px 0 8px' }}>
                  {hotspot.name}
                  <Text type="secondary" style={{ fontSize: 13, fontWeight: 400 }}> · {hotspot.province}</Text>
                </Title>
                <Space wrap style={{ marginBottom: 12 }}>
                  <Tag color="cyan">{hotspot.industry}</Tag>
                  <Tag color={hotspot.growth >= 15 ? 'gold' : 'default'}>
                    环比 {hotspot.growth > 0 ? '+' : ''}{hotspot.growth}%
                  </Tag>
                </Space>
                <div className="ip-hotspot-params">
                  <div><span>热度</span><strong>{hotspot.heat}</strong></div>
                  <div><span>线索</span><strong>{hotspot.leads}</strong></div>
                  <div><span>发布</span><strong>{hotspot.posts}</strong></div>
                </div>
                <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 12 }}>对标话题</Text>
                <Text strong style={{ display: 'block', marginTop: 4 }}>{hotspot.topic}</Text>
                <Space style={{ marginTop: 16 }} wrap>
                  <Button
                    type="primary"
                    className="ip-btn-primary"
                    icon={<ArrowRightOutlined />}
                    onClick={() => navigate('/m/compose/lipsync')}
                  >
                    拍口播
                  </Button>
                  <Button icon={<SendOutlined />} onClick={() => navigate('/m/distribution?contentType=video')}>去发布</Button>
                </Space>
              </>
            ) : (
              <>
                <Title level={4} style={{ margin: '6px 0 8px' }}>创作与分发</Title>
                <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
                  从对标、口播到发布，一气呵成。
                </Text>
                <Space direction="vertical" style={{ width: '100%' }} size={8}>
                  <Button block type="primary" className="ip-btn-primary" onClick={() => navigate('/m/compose/lipsync')}>拍口播</Button>
                  <Button block onClick={() => navigate('/m/compose/benchmark')}>爆款对标</Button>
                  <Button block onClick={() => navigate('/m/compose/graphic')}>发图文</Button>
                  <Button block onClick={() => navigate('/m/works')}>我的作品</Button>
                </Space>
              </>
            )}
          </div>

          {province && (
            <div className="ip-panel" style={{ marginBottom: 16 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>省份概览 · {province.name}</Text>
              <div className="ip-hotspot-params" style={{ marginTop: 10 }}>
                <div><span>热度</span><strong>{province.heat}</strong></div>
                <div><span>线索</span><strong>{province.leads}</strong></div>
                <div><span>发布</span><strong>{province.posts}</strong></div>
              </div>
            </div>
          )}

          {topCities.length > 0 && (
            <div className="ip-panel">
              <Text strong style={{ display: 'block', marginBottom: 10 }}>
                {metric === 'heat' ? '热度' : metric === 'leads' ? '线索' : '发布'} Top 城市
              </Text>
              <Space direction="vertical" style={{ width: '100%' }} size={8}>
                {topCities.map((c, i) => (
                  <button
                    key={c.id}
                    type="button"
                    className={`ip-hotspot-row${hotspot?.id === c.id ? ' is-active' : ''}`}
                    onClick={() => { setHotspot(c); setProvinceName(c.province) }}
                  >
                    <span className="ip-hotspot-rank">{i + 1}</span>
                    <span style={{ flex: 1, textAlign: 'left' }}>
                      <strong>{c.name}</strong>
                      <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>{c.topic}</Text>
                    </span>
                    <strong>{c[metric]}</strong>
                  </button>
                ))}
              </Space>
            </div>
          )}

          {(brands.length === 0 || hasDraft) && (
            <div className="ip-panel" style={{ marginTop: 16 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>本地进度</Text>
              <div style={{ marginTop: 8 }}>
                {brands.length === 0 ? (
                  <Button type="link" style={{ padding: 0 }} onClick={() => navigate('/m/brands')}>还没有人设，先去建档案 →</Button>
                ) : hasDraft ? (
                  <>
                    <Text style={{ display: 'block', marginBottom: 4 }}>{draftProgress}</Text>
                    <Button type="link" style={{ padding: 0 }} onClick={() => navigate(draftResumePath)}>
                      草稿未完成，继续{draftResumeLabel} →
                    </Button>
                  </>
                ) : null}
              </div>
            </div>
          )}
        </Col>
      </Row>
      </div>
    </QueryBoundary>
  )
}

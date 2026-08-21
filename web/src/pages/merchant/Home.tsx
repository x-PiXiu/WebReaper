import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Button, Col, Row, Segmented, Space, Tag, Typography } from 'antd'
import {
  ArrowRightOutlined, FireOutlined, LinkOutlined, RobotOutlined, SendOutlined,
} from '@ant-design/icons'
import { useBrands } from '../../hooks/useBrands'
import { businessApi } from '../../api/business'
import { useComposeDraft } from '../../store/composeDraft'
import PageLoading from '../../components/PageLoading'
import ChinaHotMap from '../../components/ChinaHotMap'
import { PRODUCT } from '../../config/product'
import {
  CITY_HOTSPOTS, PROVINCE_HEAT, provinceByName, type CityHotspot,
} from '../../data/hotspots'

const { Text, Title } = Typography

type MetricKey = 'heat' | 'leads' | 'posts'

/** 工作台：中国地图获客热力 + 城市热点参数（热力数据空则隐藏演示指标，作品走真实 API） */
export default function MerchantHome() {
  const navigate = useNavigate()
  const { data: brands = [], isLoading } = useBrands()
  const { data: works = [] } = useQuery({
    queryKey: ['merchant-works'],
    queryFn: () => businessApi.listWorks().catch(() => []),
  })
  const { data: summary } = useQuery({
    queryKey: ['analytics-summary'],
    queryFn: () => businessApi.getAnalyticsSummary().catch(() => null),
    staleTime: 60_000,
  })
  const draft = useComposeDraft()

  const [metric, setMetric] = useState<MetricKey>('heat')
  const [hotspot, setHotspot] = useState<CityHotspot | null>(CITY_HOTSPOTS[0] ?? null)
  const [provinceName, setProvinceName] = useState<string | null>(null)

  const published = works.filter((w) => w.status === 'published')
  const ready = works.filter((w) => w.status === 'ready')
  const hasDraft = !!(draft.script || draft.transcript)
  const weekViews = summary?.totals?.views ?? 0
  const hasHeatData = PROVINCE_HEAT.length > 0 || CITY_HOTSPOTS.length > 0

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

  if (isLoading) return <PageLoading />

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">{PRODUCT.nameEn}</p>
          <h1>工作台</h1>
          <p className="ip-lead">全国获客热力与城市热点——点选地图查看线索、发布量与对标话题</p>
        </div>
        <Space wrap>
          <Button icon={<LinkOutlined />} onClick={() => navigate('/m/inspire')}>灵感广场</Button>
          <Button type="primary" className="ip-btn-primary" icon={<RobotOutlined />} onClick={() => navigate('/m/compose/video')}>
            发视频
          </Button>
        </Space>
      </div>

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

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <div className="ip-panel ip-rise" style={{ padding: '12px 12px 4px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap', padding: '4px 8px 8px' }}>
              <Space size={8}>
                <FireOutlined style={{ color: 'var(--wr-accent)' }} />
                <Text strong>中国获客热力图</Text>
                <Tag style={{ margin: 0 }}>可缩放拖拽</Tag>
              </Space>
              {CITY_HOTSPOTS.length > 0 && (
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
              )}
            </div>
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
          </div>
        </Col>

        <Col xs={24} lg={8}>
          <div className="ip-panel ip-rise" style={{ marginBottom: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>当前热点</Text>
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
                    onClick={() => navigate(`/m/compose/video`)}
                  >
                    去发视频
                  </Button>
                  <Button icon={<SendOutlined />} onClick={() => navigate('/m/distribution')}>去发布</Button>
                </Space>
              </>
            ) : (
              <Text type="secondary">
                {CITY_HOTSPOTS.length === 0
                  ? '暂无热点数据，可先走爆款获客双轨创作'
                  : '点击地图上的金色热点查看参数'}
              </Text>
            )}
            {CITY_HOTSPOTS.length === 0 && (
              <Space style={{ marginTop: 16 }} wrap>
                <Button type="primary" className="ip-btn-primary" onClick={() => navigate('/m/compose/video')}>发视频</Button>
                <Button onClick={() => navigate('/m/compose/graphic')}>发图文</Button>
              </Space>
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
                  <Button type="link" style={{ padding: 0 }} onClick={() => navigate('/m/compose/video')}>草稿未完成，继续发视频 →</Button>
                ) : null}
              </div>
            </div>
          )}
        </Col>
      </Row>
    </div>
  )
}

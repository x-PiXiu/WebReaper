import { useState } from 'react'
import { Button, Card, Col, Empty, Progress, Row, Select, Space, Tag, Tooltip, Typography, Collapse } from 'antd'
import { EnvironmentOutlined, PlusOutlined, ReloadOutlined, TrophyOutlined, CompassOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { useBrandContext } from '../../hooks/useBrands'
import { useNavigate } from 'react-router-dom'
import type { Brand } from '../../types/api'
import QueryBoundary from '../../components/QueryBoundary'
import { message } from '../../utils/antdApp'

const { Text } = Typography

// POI 类型扫描（收进"高级"折叠——高德类目编码是工程概念，默认不打扰）
const POI_TYPE_OPTIONS = [
  { value: '', label: '默认（按品牌名+竞品+卖点搜索）' },
  { value: '050000', label: '餐饮（全部餐厅/咖啡馆/快餐）' },
  { value: '060000', label: '购物（商超/零售）' },
  { value: '070000', label: '生活服务（美容/维修/洗衣等）' },
  { value: '080000', label: '体育休闲（健身/娱乐）' },
  { value: '100000', label: '住宿（酒店/民宿）' },
  { value: '110000', label: '风景名胜（景点/公园）' },
]

/**
 * 附近对比（AI 体检 · 体检记录子层——纯双榜数据视图）：
 * 现实世界地图榜（距离/评分）vs AI 世界竞品榜（提及率）对照。
 * 门店档案管理在「品牌档案 · 门店档案」（输入归档案、输出归体检——职责拆干净）。
 */
export default function Nearby({ embedded }: { embedded?: boolean }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { brands, brandId, setCurrentBrand } = useBrandContext()
  const [types, setTypes] = useState<string>('')

  const selectedBrand = brands.find((b: Brand) => b.id === brandId)

  // 门店（仅用于"无门店"引导判断——管理在品牌档案）
  const { data: stores = [] } = useQuery({
    queryKey: ['geo-stores', brandId],
    queryFn: () => businessApi.listStoreLocations(brandId!).catch(() => []),
    enabled: !!brandId && brandId !== '' && (selectedBrand?.biz_type !== 'online'),
  })

  const { data: ranking, isLoading: rankingLoading, isError: rankingError, refetch: refetchRanking } = useQuery({
    queryKey: ['geo-nearby', brandId, types],
    queryFn: () => businessApi.getNearbyCompetitors(brandId!, types || undefined),
    enabled: !!brandId,
    retry: false,
  })

  // AI 榜探查（让 AI 真实搜索附近同行——约 1-2 分钟，缓存 24 小时）
  const airProbeMut = useMutation({
    mutationFn: (t: string) => businessApi.runAIRankProbe(brandId!, t),
    onSuccess: () => {
      queryClient.setQueryData(['geo-nearby', brandId, types], undefined)
      refetchRanking()
      message.success('AI 榜已更新：AI 真实搜索了附近同行')
    },
    onError: () => { /* 拦截器已提示 */ },
  })

  return (
    // 嵌入体检记录子层时去掉页面级外壳（padding 由父层统一提供）
    <div className={embedded ? '' : 'wr-page-content'}>
      {!embedded && (
        <div className="wr-page-header">
          <h1><EnvironmentOutlined style={{ marginRight: 8 }} />附近对比</h1>
          <p>
            附近同行双榜（地图真实排名 vs AI 提及率）
            <Tooltip title="地图榜：以你的门店为中心搜索周边同行（距离/评分，来自高德地图）；AI 榜：监测结果中竞品被 AI 提及的比例。双榜对照——物理距离上的对手 + AI 声量上的对手。">
              <span className="wr-help-tip">?</span>
            </Tooltip>
          </p>
        </div>
      )}

      {brands.length === 0 ? (
        <Empty description="还没有品牌——先创建品牌" />
      ) : (
        <>
          <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
            <Space wrap>
              <Text strong>选择品牌：</Text>
              <Select
                style={{ width: 240 }}
                placeholder="选择品牌"
                value={brandId}
                onChange={setCurrentBrand}
                options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
              />
              {selectedBrand?.biz_type === 'online' && (
                <Text type="warning" style={{ fontSize: 12 }}>
                  线上业务品牌——附近对比不适用（竞品请用「品牌档案 · 竞品」的监测结果推荐）
                </Text>
              )}
            </Space>
          </Card>

          {selectedBrand?.biz_type === 'online' ? null : brandId && (
            <>
              {/* 无门店引导（门店管理在品牌档案·门店档案） */}
              {stores.length === 0 && (
                <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                    <EnvironmentOutlined style={{ color: 'var(--wr-warning)' }} />
                    <Text style={{ fontSize: 13 }}>还没有门店——双榜以你的门店为中心，先去建个档</Text>
                    <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => navigate('/m/brands')}>
                      去「品牌档案 · 门店档案」添加
                    </Button>
                  </div>
                </Card>
              )}

              <Card
                className="wr-glass-card"
                title={<Space><CompassOutlined />附近同行双榜</Space>}
                extra={
                  <Tooltip title="重新搜索地图同行 + 让 AI 真实搜索一次附近同行（约 1-2 分钟，结果缓存 24 小时，消耗少量额度）">
                    <Button
                      type="primary"
                      onClick={() => airProbeMut.mutate(types || '')}
                      icon={<ReloadOutlined />}
                      loading={airProbeMut.isPending}
                    >
                      {airProbeMut.isPending ? 'AI 搜索中…' : '更新双榜'}
                    </Button>
                  </Tooltip>
                }
              >
                {/* POI 类型收进高级折叠 */}
                <Collapse
                  ghost size="small" style={{ marginBottom: 12 }}
                  items={[{
                    key: 'poi',
                    label: <span style={{ fontSize: 12 }}>高级：按类目扫描同行{types ? '（已选择类目）' : '（默认按品牌名+竞品搜索）'}</span>,
                    children: (
                      <Select
                        placeholder="按类目扫描（可选，如餐饮/购物/生活服务）"
                        value={types || undefined}
                        onChange={(v) => setTypes(v || '')}
                        options={POI_TYPE_OPTIONS}
                        style={{ width: 280 }}
                        allowClear
                      />
                    ),
                  }]}
                />
                <QueryBoundary loading={rankingLoading} error={rankingError} onRetry={() => refetchRanking()}>
                  {!ranking ? (
                    <Empty description="暂无附近同行数据——请先创建门店并完成定位（或先发起 AI 体检）" />
                  ) : (
                  <>
                    {!ranking.map_available && (
                      <div style={{ marginBottom: 16, padding: '10px 16px', background: '#fff7e6', borderRadius: 8, fontSize: 13 }}>
                        地图榜暂不可用（未配置高德地图服务或门店未定位）——当前仅展示 AI 竞品榜
                      </div>
                    )}
                    <Row gutter={[16, 16]}>
                      <Col xs={24} lg={14}>
                        <div style={{ fontWeight: 600, marginBottom: 12, fontSize: 14 }}>
                          <TrophyOutlined style={{ color: 'var(--wr-warning)', marginRight: 6 }} />
                          地图榜 · 周边同行（距离/评分）
                          {ranking.search_keyword && <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>搜索词：{ranking.search_keyword}</Text>}
                        </div>
                        {ranking.map_ranking.length === 0 ? (
                          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="周边未搜到同行门店" />
                        ) : (
                          <Space direction="vertical" size={8} style={{ width: '100%' }}>
                            {ranking.map_ranking.map((p, i) => (
                              <div key={p.name + i} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '8px 12px', background: 'var(--wr-bg-base)', borderRadius: 8 }}>
                                <Text strong style={{ color: i < 3 ? 'var(--wr-danger)' : 'var(--wr-text-muted)', width: 20 }}>{i + 1}</Text>
                                <div style={{ flex: 1, minWidth: 0 }}>
                                  <Text strong ellipsis style={{ fontSize: 13 }}>{p.name}</Text>
                                  <div style={{ fontSize: 11, color: 'var(--wr-text-muted)' }}>
                                    {p.distance_m < 1000 ? `${p.distance_m} 米` : `${(p.distance_m / 1000).toFixed(1)} 公里`}
                                    {p.rating > 0 && <span style={{ marginLeft: 8, color: p.rating >= 4.5 ? 'var(--wr-success)' : 'var(--wr-warning)' }}>评分 {p.rating}</span>}
                                    {p.cost && <span style={{ marginLeft: 8 }}>人均 {p.cost}</span>}
                                    {p.business_area && <span style={{ marginLeft: 8, color: 'var(--wr-accent)' }}>商圈 {p.business_area}</span>}
                                    {p.drive_duration_sec ? (
                                      <span style={{ marginLeft: 8 }} title={`驾车 ${((p.drive_distance_m || 0) / 1000).toFixed(1)} 公里`}>
                                        驾车约 {Math.round(p.drive_duration_sec / 60)} 分钟
                                      </span>
                                    ) : null}
                                  </div>
                                  {(p.tag || p.open_time_today) && (
                                    <div style={{ fontSize: 11, color: 'var(--wr-text-muted)', marginTop: 2 }}>
                                      {p.tag && <span>{p.tag}</span>}
                                      {p.open_time_today && <span style={{ marginLeft: 8 }}>今日 {p.open_time_today} 营业</span>}
                                    </div>
                                  )}
                                </div>
                                <div style={{ textAlign: 'right', maxWidth: 200 }}>
                                  <Text type="secondary" ellipsis style={{ fontSize: 11, display: 'block' }}>{p.address}</Text>
                                  {p.tel && <Text type="secondary" style={{ fontSize: 11 }}>{p.tel}</Text>}
                                </div>
                              </div>
                            ))}
                          </Space>
                        )}
                      </Col>
                      <Col xs={24} lg={10}>
                        <div style={{ fontWeight: 600, marginBottom: 12, fontSize: 14 }}>
                          <TrophyOutlined style={{ color: 'var(--wr-accent)', marginRight: 6 }} />
                          AI 榜 · 谁被 AI 提及
                          {ranking.own_rate >= 0 && (
                            <Text style={{ fontSize: 12, marginLeft: 8 }}>
                              我的提及率 <b style={{ color: 'var(--wr-accent)' }}>{(ranking.own_rate * 100).toFixed(0)}%</b>
                              {!ranking.ai_ranking.some((c: { is_own?: boolean }) => c.is_own) && ranking.ai_ranking.length > 0 && (
                                <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
                                  · 榜首 <b style={{ color: 'var(--wr-danger)' }}>{ranking.ai_ranking[0].name}</b>（{(ranking.ai_ranking[0].rate * 100).toFixed(0)}%）
                                </Text>
                              )}
                            </Text>
                          )}
                        </div>
                        {ranking.ai_rank_from_probe && (
                          <div style={{ marginBottom: 10, fontSize: 12, color: 'var(--wr-text-muted)' }}>
                            {(ranking.ai_rank_mentioned ?? 0) > 0 ? (
                              <Text type="warning">
                                上榜率 <b>{ranking.ai_rank_mentioned}/{ranking.ai_rank_total}</b>——附近同行里只有 {ranking.ai_rank_mentioned} 家被 AI 提及
                                {(ranking.ai_rank_mentioned ?? 0) > 0 && (ranking.ai_rank_mentioned ?? 0) < (ranking.ai_rank_total || 1) ? '，你还没上榜' : ''}
                              </Text>
                            ) : (
                              <Text type="secondary">
                                上榜率 0/{ranking.ai_rank_total}——本地 AI 竞争度低，正是抢先发布内容让 AI 认识你的机会
                              </Text>
                            )}
                            <div>更新于 {ranking.ai_rank_probed_at} · {ranking.ai_rank_sample} 次问法探查</div>
                          </div>
                        )}
                        {ranking.ai_ranking.length === 0 ? (
                          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无 AI 榜数据——点「更新双榜」让 AI 真实搜索一次附近同行" />
                        ) : (
                          <Space direction="vertical" size={8} style={{ width: '100%' }}>
                            {ranking.ai_ranking.map((c, i) => (
                              <div
                                key={c.name + (c.is_own ? '-own' : '')}
                                style={{
                                  display: 'flex', alignItems: 'center', gap: 12,
                                  background: c.is_own ? 'rgba(250, 173, 20, 0.10)' : undefined,
                                  border: c.is_own ? '1px solid rgba(250, 173, 20, 0.35)' : undefined,
                                  borderRadius: 8, padding: c.is_own ? '4px 8px' : undefined,
                                }}
                              >
                                <Text style={{ color: i < 3 && c.mentioned ? 'var(--wr-danger)' : 'var(--wr-text-muted)', fontWeight: 600, width: 20 }}>
                                  {c.mentioned ? i + 1 : '—'}
                                </Text>
                                <Text ellipsis style={{ flex: 1, fontSize: 13, color: c.mentioned ? undefined : 'var(--wr-text-muted)', fontWeight: c.is_own ? 700 : undefined }}>
                                  {c.name}
                                </Text>
                                {c.is_own && (
                                  <Tag color="gold" style={{ margin: 0, fontSize: 10, lineHeight: '16px' }}>我的品牌</Tag>
                                )}
                                {c.mentioned ? (
                                  <>
                                    <Progress
                                      percent={Math.round(c.rate * 100)}
                                      size="small"
                                      strokeColor={c.is_own ? '#faad14' : (c.rate >= 0.5 ? 'var(--wr-danger)' : 'var(--wr-warning)')}
                                      style={{ width: 110, margin: 0 }}
                                    />
                                    <Text strong style={{ color: 'var(--wr-text-secondary)', width: 40, textAlign: 'right', fontSize: 13 }}>{(c.rate * 100).toFixed(0)}%</Text>
                                  </>
                                ) : (
                                  <Text type="secondary" style={{ fontSize: 12 }}>未上榜</Text>
                                )}
                              </div>
                            ))}
                          </Space>
                        )}
                      </Col>
                    </Row>
                  </>
                  )}
                </QueryBoundary>
              </Card>
            </>
          )}
        </>
      )}
    </div>
  )
}

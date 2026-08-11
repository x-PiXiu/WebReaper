import { useState } from 'react'
import { Typography, Button, Modal, Form, Input, Select, Space, message, Popconfirm, Empty, Spin, Tag, Table, Card, Row, Col, Tooltip, Progress, AutoComplete } from 'antd'
import { PlusOutlined, EnvironmentOutlined, ReloadOutlined, DeleteOutlined, TrophyOutlined, CompassOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand, StoreLocation } from '../../types/api'

const { Text } = Typography

const BIZ_TYPES = [
  { value: 'Restaurant', label: '餐饮（Restaurant）' },
  { value: 'LocalBusiness', label: '通用门店（LocalBusiness）' },
  { value: 'Cafe', label: '咖啡馆（Cafe）' },
  { value: 'Bar', label: '酒吧（Bar）' },
  { value: 'Store', label: '商店（Store）' },
]

// P1 POI 类型扫描下拉：高德分类编码（大类自动含中/小类）——
// 按类目扫描全量竞品，不依赖品牌/竞品名称命中。
const POI_TYPE_OPTIONS = [
  { value: '', label: '默认（品牌+竞品+卖点词搜索）' },
  { value: '050000', label: '餐饮服务（全部餐厅/咖啡馆/快餐）' },
  { value: '060000', label: '购物服务（商超/零售）' },
  { value: '070000', label: '生活服务（美容/维修/洗衣等）' },
  { value: '080000', label: '体育休闲（健身/娱乐）' },
  { value: '100000', label: '住宿服务（酒店/民宿）' },
  { value: '110000', label: '风景名胜（景点/公园）' },
]

function geoStatusTag(s: StoreLocation) {
  if (s.geo_status === 'ok') return <Tag color="success">已定位</Tag>
  if (s.geo_status === 'failed') return <Tag color="error">定位失败</Tag>
  return <Tag color="warning">待定位</Tag>
}

// 附近同行（本地生活 GEO 主战场）：
//   门店档案管理（地址/电话/营业时间/人均——NAP 信号，地理编码自动定位）
//   + 附近同行双榜：现实世界地图榜（距离/评分）与 AI 世界竞品榜（提及率）对照。
export default function Nearby() {
  const queryClient = useQueryClient()
  const [brandId, setBrandId] = useState<string>()
  const [types, setTypes] = useState<string>('') // P1 POI 类型扫描（如 050000 餐饮）
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<StoreLocation | null>(null)
  const [suggestOptions, setSuggestOptions] = useState<{ value: string; address?: string }[]>([]) // P1 地址联想

  // P1 地址联想：输入 2 字以上防抖查询（轻量实现：直接请求，联想失败静默）
  let suggestTimer: ReturnType<typeof setTimeout> | null = null
  const handleSuggest = (q: string) => {
    if (suggestTimer) clearTimeout(suggestTimer)
    if (q.trim().length < 2) {
      setSuggestOptions([])
      return
    }
    suggestTimer = setTimeout(async () => {
      try {
        const tips = await businessApi.suggestLocations(q.trim())
        setSuggestOptions(tips.map((t) => ({
          value: t.name + (t.address ? '（' + t.address + '）' : ''),
          address: t.address || t.name,
        })))
      } catch {
        setSuggestOptions([]) // 未配置地图服务 → 纯手输
      }
    }, 300)
  }
  const [form] = Form.useForm()

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const selectedBrand = brands.find((b: Brand) => b.id === brandId)

  const { data: stores = [], isLoading: storesLoading } = useQuery({
    queryKey: ['geo-stores', brandId],
    queryFn: () => businessApi.listStoreLocations(brandId!),
    enabled: !!brandId,
  })

  const { data: ranking, isLoading: rankingLoading, refetch: refetchRanking } = useQuery({
    queryKey: ['geo-nearby', brandId, types],
    queryFn: () => businessApi.getNearbyCompetitors(brandId!, types || undefined),
    enabled: !!brandId,
    retry: false,
  })

  const createMut = useMutation({
    mutationFn: (v: any) => businessApi.createStoreLocation(brandId!, v),
    onSuccess: () => {
      message.success('门店创建成功，已尝试地理编码定位')
      setModalOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['geo-stores', brandId] })
    },
    onError: (e: Error) => message.error('创建失败：' + e.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: { id: string; data: Record<string, any> }) => businessApi.updateStoreLocation(brandId!, v.id, v.data),
    onSuccess: () => {
      message.success('门店已更新（地址变更已重新定位）')
      setModalOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['geo-stores', brandId] })
      refetchRanking()
    },
    onError: (e: Error) => message.error('更新失败：' + e.message),
  })

  const deleteMut = useMutation({
    mutationFn: (storeId: string) => businessApi.deleteStoreLocation(brandId!, storeId),
    onSuccess: () => {
      message.success('门店已删除')
      queryClient.invalidateQueries({ queryKey: ['geo-stores', brandId] })
    },
  })

  const reGeoMut = useMutation({
    mutationFn: (storeId: string) => businessApi.reGeocodeStoreLocation(brandId!, storeId),
    onSuccess: (loc: StoreLocation) => {
      message.success(loc.geo_status === 'ok' ? '定位成功' : '定位失败（可修改地址后重试）')
      queryClient.invalidateQueries({ queryKey: ['geo-stores', brandId] })
    },
  })

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (s: StoreLocation) => {
    setEditing(s)
    form.setFieldsValue({ name: s.name, address: s.address, phone: s.phone, hours: s.hours, price_level: s.price_level, biz_type: s.biz_type || 'Restaurant' })
    setModalOpen(true)
  }

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      if (editing) {
        updateMut.mutate({ id: editing.id, data: values })
      } else {
        createMut.mutate(values)
      }
    })
  }

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1><EnvironmentOutlined style={{ marginRight: 8 }} />附近同行</h1>
        <p>
          门店档案（地址/电话/营业时间——本地曝光的地基）· 附近同行双榜（地图真实排名 vs AI 提及率）
          <Tooltip title="地图榜：以你的门店为中心搜索周边同行（距离/评分，来自高德地图）；AI 榜：监测结果中竞品被 AI 提及的比例。双榜对照——物理距离上的对手 + AI 声量上的对手。">
            <span className="wr-help-tip">?</span>
          </Tooltip>
        </p>
      </div>

      {brands.length === 0 ? (
        <Empty description="还没有品牌——先创建品牌即可管理门店" />
      ) : (
        <>
          <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
            <Space wrap>
              <Text strong>选择品牌：</Text>
              <Select
                style={{ width: 240 }}
                placeholder="选择品牌"
                value={brandId}
                onChange={setBrandId}
                options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
              />
              {selectedBrand && (
                <Space direction="vertical" size={0}>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    品牌定位：{selectedBrand.positioning || '未填写'} · 竞品：{selectedBrand.competitors?.join('、') || '未填写'}
                  </Text>
                  {selectedBrand.biz_type === 'online' && (
                    <Text type="warning" style={{ fontSize: 12 }}>
                      ⚠️ 线上业务品牌——门店/附近同行功能不适用（线上业务无地理约束，竞品请用「品牌管理 → 从监测结果推荐」）
                    </Text>
                  )}
                </Space>
              )}
            </Space>
          </Card>

          {/* BizType 门控：online 品牌显示提示，不渲染门店/双榜（P0-3）*/}
          {selectedBrand?.biz_type === 'online' ? (
            <Card className="wr-glass-card" style={{ textAlign: 'center', padding: '40px 0' }}>
              <Empty description="线上业务品牌无需门店与附近同行管理">
                <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 13 }}>
                  线上业务的 GEO 核心是行业关键词监测 + 内容优化——<br />
                  请前往「关键词管理」发起监测，在「品牌管理」用「从监测结果推荐」获取竞品。
                </Text>
              </Empty>
            </Card>
          ) : brandId && (
            <Card
              className="wr-glass-card"
              style={{ marginBottom: 16 }}
              title={<Space><EnvironmentOutlined />门店档案</Space>}
              extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建门店</Button>}
            >
              {storesLoading ? <Spin /> : stores.length === 0 ? (
                <Empty description="还没有门店——添加门店后即可自动定位、查看附近同行排名" />
              ) : (
                <Table
                  dataSource={stores}
                  rowKey="id"
                  size="small"
                  pagination={false}
                  columns={[
                    { title: '门店', dataIndex: 'name', render: (n, r) => <Text strong>{n || r.address}</Text> },
                    { title: '地址', dataIndex: 'address', ellipsis: true },
                    {
                      title: '定位', dataIndex: 'geo_status', width: 100,
                      render: (_, s) => geoStatusTag(s),
                    },
                    {
                      title: '坐标', key: 'geo', width: 130,
                      render: (_, s) => s.has_geo
                        ? <Text style={{ fontSize: 12 }}>{s.lat.toFixed(4)}, {s.lng.toFixed(4)}</Text>
                        : <Text type="secondary" style={{ fontSize: 12 }}>—</Text>,
                    },
                    { title: '营业时间', dataIndex: 'hours', width: 120, render: (v) => v || '—' },
                    { title: '电话', dataIndex: 'phone', width: 130, render: (v) => v || '—' },
                    {
                      title: '操作', key: 'op', width: 170,
                      render: (_, s) => (
                        <Space size={4}>
                          <Button size="small" onClick={() => openEdit(s)}>编辑</Button>
                          <Button size="small" icon={<ReloadOutlined />} onClick={() => reGeoMut.mutate(s.id)}>重定位</Button>
                          <Popconfirm title="删除该门店？" onConfirm={() => deleteMut.mutate(s.id)}>
                            <Button size="small" danger icon={<DeleteOutlined />} />
                          </Popconfirm>
                        </Space>
                      ),
                    },
                  ]}
                />
              )}
            </Card>
          )}

          {brandId && (
            <Card
              className="wr-glass-card"
              title={<Space><CompassOutlined />附近同行双榜</Space>}
              extra={
                <Space>
                  {/* P1 POI 类型扫描：按类目编码搜全量竞品（下拉选择，无需记编码） */}
                  <Select
                    placeholder="竞品类型（可选）"
                    value={types || undefined}
                    onChange={(v) => setTypes(v || '')}
                    options={POI_TYPE_OPTIONS}
                    style={{ width: 240 }}
                    allowClear
                  />
                  <Button onClick={() => refetchRanking()} icon={<ReloadOutlined />}>刷新</Button>
                </Space>
              }
            >
              {rankingLoading ? <Spin /> : !ranking ? (
                <Empty description="暂无附近同行数据——请先创建门店并完成定位（或先发起 AI 监测）" />
              ) : (
                <>
                  {!ranking.map_available && (
                    <div style={{ marginBottom: 16, padding: '10px 16px', background: '#fff7e6', borderRadius: 8, fontSize: 13 }}>
                      ⚠️ 地图榜暂不可用（未配置高德 AMAP_API_KEY 或门店未定位）——当前仅展示 AI 竞品榜
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
                                  {p.business_area && <span style={{ marginLeft: 8, color: 'var(--wr-accent)' }}>📍{p.business_area}</span>}
                                  {p.drive_duration_sec ? (
                                    <span style={{ marginLeft: 8 }} title={`驾车 ${((p.drive_distance_m || 0) / 1000).toFixed(1)} 公里`}>
                                      🚗 约 {Math.round(p.drive_duration_sec / 60)} 分钟
                                    </span>
                                  ) : null}
                                </div>
                                {(p.tag || p.open_time_today) && (
                                  <div style={{ fontSize: 11, color: 'var(--wr-text-muted)', marginTop: 2 }}>
                                    {p.tag && <span>{p.tag}</span>}
                                    {p.open_time_today && <span style={{ marginLeft: 8 }}>⏰ {p.open_time_today}</span>}
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
                        AI 榜 · 竞品提及率
                        {ranking.own_rate >= 0 && (
                          <Text style={{ fontSize: 12, marginLeft: 8 }}>
                            我的提及率 <b style={{ color: 'var(--wr-accent)' }}>{(ranking.own_rate * 100).toFixed(0)}%</b>
                          </Text>
                        )}
                      </div>
                      {ranking.ai_ranking.length === 0 ? (
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无 AI 竞品数据（监测时自动识别）" />
                      ) : (
                        <Space direction="vertical" size={8} style={{ width: '100%' }}>
                          {ranking.ai_ranking.map((c, i) => (
                            <div key={c.name} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                              <Text style={{ color: i < 3 ? 'var(--wr-danger)' : 'var(--wr-text-muted)', fontWeight: 600, width: 20 }}>{i + 1}</Text>
                              <Text ellipsis style={{ flex: 1, fontSize: 13 }}>{c.name}</Text>
                              <Progress
                                percent={Math.round(c.rate * 100)}
                                size="small"
                                strokeColor={c.rate >= 0.5 ? 'var(--wr-danger)' : 'var(--wr-warning)'}
                                style={{ width: 110, margin: 0 }}
                              />
                              <Text strong style={{ color: 'var(--wr-text-secondary)', width: 40, textAlign: 'right', fontSize: 13 }}>{(c.rate * 100).toFixed(0)}%</Text>
                            </div>
                          ))}
                        </Space>
                      )}
                    </Col>
                  </Row>
                </>
              )}
            </Card>
          )}
        </>
      )}

      <Modal
        title={editing ? '编辑门店' : '新建门店'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        confirmLoading={createMut.isPending || updateMut.isPending}
        okText="保存"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="门店名称" extra="留空则使用品牌名">
            <Input placeholder="如：望京店" />
          </Form.Item>
          <Form.Item name="address" label="详细地址" rules={[{ required: !editing, message: '请输入门店详细地址（用于地图定位）' }]}>
            {/* P1 地址联想：输入自动提示高德地址，选中后回填完整地址（免手输） */}
            <AutoComplete
              placeholder="如：北京市朝阳区望京街10号"
              options={suggestOptions}
              onSearch={handleSuggest}
              onSelect={(_, opt) => {
                form.setFieldValue('address', opt.address || opt.value)
                form.setFieldValue('hours', form.getFieldValue('hours') || '') // 保持用户已填项
              }}
              filterOption={false}
            />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="hours" label="营业时间">
                <Input placeholder="如：10:00-22:00" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="phone" label="联系电话">
                <Input placeholder="如：010-12345678" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="price_level" label="人均消费档位">
                <Input placeholder="如：¥80/人" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="biz_type" label="业态" initialValue="Restaurant">
                <Select options={BIZ_TYPES} />
              </Form.Item>
            </Col>
          </Row>
          <Text type="secondary" style={{ fontSize: 12 }}>
            📍 保存后将自动调用高德地图定位（未配置 AMAP_API_KEY 时标记"待定位"，可稍后重试）。地址/电话/营业时间会注入文章生成与公开页展示。
          </Text>
        </Form>
      </Modal>
    </div>
  )
}

import { useState, useEffect } from 'react'
import { Typography, Button, Modal, Form, Input, Select, Space, message, Popconfirm, Empty, Checkbox, Spin, Tag, Input as AntInput } from 'antd'
import { PlusOutlined, DeleteOutlined, TagOutlined, EnvironmentOutlined, BulbOutlined, SearchOutlined, RadarChartOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand, CompetitorSuggestion } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

export default function Brands() {
  const queryClient = useQueryClient()
  const [brandModalOpen, setBrandModalOpen] = useState(false)
  const [selectedBrand, setSelectedBrand] = useState<Brand | null>(null)
  const [brandForm] = Form.useForm()
  const [editForm] = Form.useForm()
  const [searchText, setSearchText] = useState('')
  // 竞品推荐
  const [compSuggestOpen, setCompSuggestOpen] = useState(false)
  const [suggestions, setSuggestions] = useState<CompetitorSuggestion[]>([])
  const [checkedComps, setCheckedComps] = useState<string[]>([])
  const [loadingSuggest, setLoadingSuggest] = useState(false)
  const [savingBrand, setSavingBrand] = useState(false)

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['geo-brands'] })
    queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
  }

  // 选中品牌时同步编辑表单
  useEffect(() => {
    if (selectedBrand) {
      editForm.setFieldsValue({
        name: selectedBrand.name,
        biz_type: selectedBrand.biz_type || 'local',
        positioning: selectedBrand.positioning,
        core_selling: (selectedBrand.core_selling || []).join('、'),
        competitors: (selectedBrand.competitors || []).join('、'),
      })
    }
  }, [selectedBrand]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleCreateBrand = async (values: { name: string; positioning: string; core_selling: string; competitors: string; biz_type?: string }) => {
    try {
      await businessApi.createBrand({
        name: values.name,
        positioning: values.positioning,
        core_selling: values.core_selling ? values.core_selling.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean) : [],
        competitors: values.competitors ? values.competitors.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean) : [],
        biz_type: values.biz_type || 'local',
      })
      message.success(`品牌「${values.name}」创建成功`)
      setBrandModalOpen(false)
      brandForm.resetFields()
      invalidate()
    } catch {}
  }

  const handleDeleteBrand = async (id: string) => {
    try {
      await businessApi.deleteBrand(id)
      message.success('已删除')
      if (selectedBrand?.id === id) setSelectedBrand(null)
      invalidate()
    } catch {}
  }

  // 保存品牌编辑（名称/定位/卖点/竞品/业务类型）
  const handleSaveBrand = async () => {
    if (!selectedBrand) return
    try {
      const values = await editForm.validateFields()
      setSavingBrand(true)
      const updated = await businessApi.updateBrand(selectedBrand.id, {
        name: values.name,
        biz_type: values.biz_type || 'local',
        positioning: values.positioning,
        core_selling: values.core_selling ? values.core_selling.split(/[,，、\n]/).map((s: string) => s.trim()).filter(Boolean) : [],
        competitors: values.competitors ? values.competitors.split(/[,，、\n]/).map((s: string) => s.trim()).filter(Boolean) : [],
      })
      message.success('品牌信息已保存')
      setSelectedBrand(updated)
      invalidate()
    } catch {} finally {
      setSavingBrand(false)
    }
  }

  // 推荐竞品（source=poi 附近同行 / source=monitoring 监测结果蒸馏）
  const [suggestSource, setSuggestSource] = useState<string>('poi')
  const handleSuggestCompetitors = async (source: string = 'poi') => {
    if (!selectedBrand) return
    setSuggestSource(source)
    setCompSuggestOpen(true)
    setCheckedComps([])
    setSuggestions([])
    setLoadingSuggest(true)
    try {
      const res = await businessApi.suggestCompetitors(selectedBrand.id, source, 8)
      setSuggestions(res || [])
    } catch (e) {
      message.error('推荐失败：' + ((e as Error)?.message || ''))
    } finally {
      setLoadingSuggest(false)
    }
  }

  // 采纳勾选的竞品（合并到品牌竞品列表，去重）
  const handleAdoptCompetitors = async () => {
    if (!selectedBrand || checkedComps.length === 0) {
      message.warning('请至少勾选一个竞品')
      return
    }
    const existing = new Set(selectedBrand.competitors || [])
    const merged = [...(selectedBrand.competitors || []), ...checkedComps.filter((c) => !existing.has(c))]
    try {
      const updated = await businessApi.updateBrand(selectedBrand.id, { competitors: merged })
      message.success(`已采纳 ${checkedComps.length} 个竞品`)
      setSelectedBrand(updated)
      editForm.setFieldsValue({ competitors: merged.join('、') })
      setCompSuggestOpen(false)
      invalidate()
    } catch {}
  }

  const filteredBrands = brands.filter((b: Brand) =>
    !searchText || b.name.toLowerCase().includes(searchText.toLowerCase()) ||
    (b.positioning || '').toLowerCase().includes(searchText.toLowerCase())
  )

  return (
    <div className="wr-page-content" style={{ paddingTop: 0 }}>
      <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h1>品牌管理</h1>
          <p>管理品牌资产与竞品——关键词管理请前往「关键词管理」页</p>
        </div>
        <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setBrandModalOpen(true)}>
          创建品牌
        </Button>
      </div>

      <div style={{ display: 'flex', gap: 16, minHeight: 'calc(100vh - 200px)' }}>
        {/* 左：品牌列表 */}
        <div className="wr-glass-card" style={{ width: 340, flexShrink: 0, display: 'flex', flexDirection: 'column', gap: 8, padding: 16 }}>
          <AntInput
            prefix={<SearchOutlined style={{ color: 'var(--wr-text-muted)' }} />}
            placeholder="搜索品牌名或定位"
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            allowClear
            style={{ marginBottom: 4 }}
          />
          <div style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column', gap: 6 }}>
            {filteredBrands.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={searchText ? '未找到匹配品牌' : '暂无品牌'} style={{ padding: 40 }}>
                {!searchText && (
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => setBrandModalOpen(true)}>创建第一个品牌</Button>
                )}
              </Empty>
            ) : (
              filteredBrands.map((brand: Brand) => {
                const isSelected = selectedBrand?.id === brand.id
                return (
                  <div
                    key={brand.id}
                    onClick={() => setSelectedBrand(brand)}
                    style={{
                      padding: '14px 16px', borderRadius: 10, cursor: 'pointer',
                      background: isSelected ? 'var(--wr-primary-bg)' : 'var(--wr-bg-surface)',
                      border: `1px solid ${isSelected ? 'var(--wr-primary)' : 'var(--wr-border)'}`,
                      transition: 'all 200ms cubic-bezier(0.2, 0, 0, 1)',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
                      <Text strong style={{ fontSize: 15, color: isSelected ? 'var(--wr-primary-hover)' : 'var(--wr-text-primary)' }}>{brand.name}</Text>
                      <Popconfirm title="删除品牌及其关键词？" onConfirm={() => handleDeleteBrand(brand.id)}>
                        <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={(e) => e.stopPropagation()} style={{ opacity: 0.5 }} />
                      </Popconfirm>
                    </div>
                    {brand.positioning && (
                      <Text type="secondary" style={{ fontSize: 12, lineHeight: 1.5, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{brand.positioning}</Text>
                    )}
                    <div style={{ display: 'flex', gap: 12, marginTop: 6, alignItems: 'center' }}>
                      <Tag color={brand.biz_type === 'online' ? 'blue' : 'green'} style={{ margin: 0, fontSize: 10 }}>
                        {brand.biz_type === 'online' ? '💻 线上' : '🏪 本地'}
                      </Tag>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        <TagOutlined style={{ marginRight: 3 }} />{brand.core_selling?.length || 0} 卖点
                      </Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        <EnvironmentOutlined style={{ marginRight: 3 }} />{brand.competitors?.length || 0} 竞品
                      </Text>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* 右：品牌详情编辑 + 竞品管理 */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {selectedBrand ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* 品牌信息编辑卡片 */}
              <div className="wr-glass-card" style={{ padding: 24 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                  <Space>
                    <BulbOutlined style={{ color: 'var(--wr-primary)' }} />
                    <Text strong style={{ fontSize: 16 }}>品牌信息</Text>
                    <Tag color={selectedBrand.biz_type === 'online' ? 'blue' : 'green'} style={{ fontSize: 10 }}>
                      {selectedBrand.biz_type === 'online' ? '💻 线上业务' : '🏪 本地生意'}
                    </Tag>
                  </Space>
                  <Button size="small" type="text" onClick={() => setSelectedBrand(null)}>关闭</Button>
                </div>
                <Form form={editForm} layout="vertical" requiredMark={false}>
                  <Form.Item label="品牌名" name="name" rules={[{ required: true, message: '请输入品牌名' }]}>
                    <Input placeholder="品牌名" />
                  </Form.Item>
                  <Form.Item label="业务类型" name="biz_type" tooltip="本地生意：有门店+附近同行+本地搜索词；线上业务：无地理约束+品类搜索词">
                    <Select options={[
                      { value: 'local', label: '🏪 本地生意（有门店，做附近同行对比）' },
                      { value: 'online', label: '💻 线上业务（无门店，做行业竞品对比）' },
                    ]} />
                  </Form.Item>
                  <Form.Item label="品牌定位" name="positioning">
                    <TextArea placeholder="描述品牌的核心价值" autoSize={{ minRows: 2, maxRows: 4 }} />
                  </Form.Item>
                  <Form.Item label="核心卖点" name="core_selling" tooltip="用顿号或逗号分隔">
                    <TextArea placeholder="10年经验、环保材料、终身保修" autoSize={{ minRows: 2 }} />
                  </Form.Item>
                  <Form.Item label="竞品" name="competitors" tooltip="用顿号或逗号分隔。可点击下方「从附近同行推荐」自动补充">
                    <TextArea placeholder="竞品A、竞品B、竞品C" autoSize={{ minRows: 1 }} />
                  </Form.Item>
                  <Button type="primary" loading={savingBrand} onClick={handleSaveBrand}>保存品牌信息</Button>
                </Form>
              </div>

              {/* 竞品管理卡片 */}
              <div className="wr-glass-card" style={{ padding: 24 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                  <Space>
                    <RadarChartOutlined style={{ color: 'var(--wr-accent)' }} />
                    <Text strong style={{ fontSize: 16 }}>竞品管理</Text>
                    <Tag color="purple">{selectedBrand.competitors?.length || 0}</Tag>
                  </Space>
                  {selectedBrand.biz_type !== 'online' && (
                    <Button size="small" type="primary" ghost icon={<RadarChartOutlined />} onClick={() => handleSuggestCompetitors('poi')}>
                      从附近同行推荐
                    </Button>
                  )}
                  <Button size="small" type="primary" ghost icon={<SearchOutlined />} onClick={() => handleSuggestCompetitors('monitoring')}>
                    从监测结果推荐
                  </Button>
                </div>
                <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 12, lineHeight: 1.6 }}>
                  竞品是监测时的对比坐标系——「你的 AI 提及率 vs 竞品」让商户知道差距。
                  {selectedBrand.biz_type !== 'online' && '本地品牌可一键从附近同行 POI（按评分/距离）推荐竞品候选。'}
                </Text>
                {selectedBrand.competitors && selectedBrand.competitors.length > 0 ? (
                  <Space size={6} wrap>
                    {selectedBrand.competitors.map((c, i) => (
                      <Tag key={i} closable color="orange" onClose={async () => {
                        const remaining = selectedBrand.competitors!.filter((_, idx) => idx !== i)
                        const updated = await businessApi.updateBrand(selectedBrand.id, { competitors: remaining })
                        setSelectedBrand(updated)
                        editForm.setFieldsValue({ competitors: remaining.join('、') })
                        invalidate()
                      }}>{c}</Tag>
                    ))}
                  </Space>
                ) : (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无竞品——手动添加或从附近同行推荐" style={{ padding: 20 }} />
                )}
              </div>
            </div>
          ) : (
            <div className="wr-glass-card" style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="选择左侧品牌查看详情和编辑" />
            </div>
          )}
        </div>
      </div>

      {/* 创建品牌弹窗 */}
      <Modal title="创建品牌" open={brandModalOpen} onCancel={() => setBrandModalOpen(false)} footer={null} width={560}>
        <Form form={brandForm} layout="vertical" onFinish={handleCreateBrand} requiredMark={false} initialValues={{ biz_type: 'local' }}>
          <Form.Item label="品牌名" name="name" rules={[{ required: true, message: '请输入品牌名' }]}>
            <Input placeholder="如 某装修公司" />
          </Form.Item>
          <Form.Item label="业务类型" name="biz_type" tooltip="本地生意（餐厅/装修/理发）：有门店+附近同行对比+本地搜索词；线上业务（SaaS/工具/网络公司）：无地理约束+品类搜索词+行业竞品">
            <Select options={[
              { value: 'local', label: '🏪 本地生意（有门店，做附近同行对比）' },
              { value: 'online', label: '💻 线上业务（无门店，做行业竞品对比）' },
            ]} />
          </Form.Item>
          <Form.Item label="品牌定位" name="positioning" tooltip="描述品牌的核心价值，AI 生成内容时会参考">
            <TextArea placeholder="如 专注北京地区中高端家装，提供设计-施工-软装一站式服务" autoSize={{ minRows: 2, maxRows: 4 }} />
          </Form.Item>
          <Form.Item label="核心卖点" name="core_selling" tooltip="用逗号分隔，如：10年经验,环保材料,终身保修">
            <TextArea placeholder="10年经验, 环保材料, 终身保修" autoSize={{ minRows: 2 }} />
          </Form.Item>
          <Form.Item label="竞品" name="competitors" tooltip="用逗号分隔，监测时会对比这些竞品的 AI 可见度。创建后可在品牌详情用「从附近同行推荐」自动补充">
            <Input placeholder="竞品A, 竞品B, 竞品C（可留空，后续自动推荐）" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">创建</Button>
              <Button onClick={() => setBrandModalOpen(false)}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 竞品推荐弹窗（附近同行 POI 按评分/距离排序） */}
      <Modal
        title={`竞品推荐 · ${suggestSource === 'monitoring' ? '从监测结果' : '从附近同行'} · ${selectedBrand?.name || ''}`}
        open={compSuggestOpen}
        onCancel={() => setCompSuggestOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setCompSuggestOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleAdoptCompetitors} disabled={checkedComps.length === 0}>
              采纳勾选的 {checkedComps.length} 个
            </Button>
          </Space>
        }
        width={560}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 12 }}>
          {suggestSource === 'monitoring'
            ? '监测时 AI 回答中提到的对手（按提及率降序，已排除品牌自身和已有竞品）。勾选要采纳的竞品。'
            : '附近同行 POI 按评分降序+距离升序推荐（已排除品牌自身和已有竞品）。勾选要采纳的竞品。'}
        </Text>
        {loadingSuggest ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin tip={suggestSource === 'monitoring' ? '正在从监测结果蒸馏...' : '正在搜索附近同行...'} /></div>
        ) : suggestions.length === 0 ? (
          <Empty description={suggestSource === 'monitoring' ? '暂无推荐——需先发起监测' : '暂无推荐——需先创建门店并完成地理编码'} style={{ padding: 24 }} />
        ) : (
          <Checkbox.Group value={checkedComps} onChange={(values) => setCheckedComps(values as string[])} style={{ width: '100%' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {suggestions.map((s) => (
                <Checkbox key={s.name} value={s.name} style={{ marginLeft: 0 }}>
                  <Space size={8}>
                    <Text strong style={{ fontSize: 13 }}>{s.name}</Text>
                    {suggestSource === 'monitoring' ? (
                      <Tag color="purple" style={{ fontSize: 10, margin: 0 }}>提及率 {s.address}</Tag>
                    ) : (
                      <>
                        {s.rating > 0 && <Tag color="gold" style={{ fontSize: 10, margin: 0 }}>⭐ {s.rating}</Tag>}
                        {s.distance_m > 0 && <Text type="secondary" style={{ fontSize: 11 }}>📍 {s.distance_m < 1000 ? s.distance_m + '米' : (s.distance_m / 1000).toFixed(1) + '公里'}</Text>}
                      </>
                    )}
                    {s.category && suggestSource !== 'monitoring' && <Text type="secondary" style={{ fontSize: 10 }}>{s.category.split(';')[0]}</Text>}
                  </Space>
                </Checkbox>
              ))}
            </div>
          </Checkbox.Group>
        )}
        {suggestions.length > 0 && (
          <div style={{ marginTop: 12, paddingTop: 12, borderTop: '1px solid var(--wr-border)' }}>
            <Button size="small" type="link" onClick={() => setCheckedComps(suggestions.map(s => s.name))}>全选</Button>
            <Button size="small" type="link" onClick={() => setCheckedComps([])}>清空</Button>
          </div>
        )}
      </Modal>
    </div>
  )
}

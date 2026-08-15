import { useState, useEffect, useMemo } from 'react'
import { Typography, Button, Modal, Form, Input, Select, AutoComplete, Space, message, Popconfirm, Empty, Checkbox, Spin, Tag, Tooltip, Tabs, Input as AntInput } from 'antd'
import { PlusOutlined, DeleteOutlined, TagOutlined, EnvironmentOutlined, BulbOutlined, SearchOutlined, RadarChartOutlined, RobotOutlined, FileTextOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { useNavigate, useLocation } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { useBrandStore } from '../../store/brand'
import { competitorStats, computeHealth, healthLevel, latestByKeyword } from '../../utils/geoHealth'
import { useHealthReport } from '../../hooks/useHealthReport'
import { INDUSTRY_OPTIONS, CLEAN_TEXT_VALIDATOR, websiteRules } from '../../constants/formRules'
import { useBrandOverviews } from '../../hooks/useBrandOverviews'
import { rateColor } from '../../utils/geo'
import StoreTab from './brands/StoreTab'
import type { Brand, CompetitorSuggestion } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

export default function Brands() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const setCurrentBrand = useBrandStore((s) => s.setCurrentBrand)
  const currentBrandId = useBrandStore((s) => s.currentBrandId)
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
  // 编辑表单校验联动（P0-2-2）：online 品牌官网必填
  const watchedBizType = Form.useWatch('biz_type', editForm)
  // 创建弹窗的业务类型联动（F1-1 修复：此前创建弹窗的官网必填引用了 editForm 的值——
  // 弹窗里选"线上业务"规则仍读编辑表单的 local，必填永不生效，空官网照样创建成功）
  const watchedCreateBizType = Form.useWatch('biz_type', brandForm)

  // F1-3：引导页 CTA 携带 location.state.openCreate 跳入——自动打开创建弹窗（消除冷启动断一步）
  const location = useLocation()
  useEffect(() => {
    if ((location.state as { openCreate?: boolean } | null)?.openCreate) {
      setBrandModalOpen(true)
      navigate('/m/brands', { replace: true, state: {} }) // 清 state 防刷新重复弹窗
    }
  }, [location.state, navigate]) // eslint-disable-line react-hooks/exhaustive-deps

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })
  // 列表健康分徽章：后端健康报告（单一事实源，含内容资产——与工作区头部/工作台卡片统一口径，
  // 修复"同一品牌三个位置三个健康分"）；报告不可用时降级本地合成（不含内容资产的旧口径）
  const { report } = useHealthReport()
  const { data: listOverviews = [] } = useBrandOverviews(brands)
  const brandHealthMap = useMemo(() => {
    const m = new Map<string, number>()
    if (report) {
      report.brands.forEach((b) => m.set(b.brand_id, b.total))
      return m
    }
    ;(listOverviews as Array<{ brand_id: string; avg_mention_rate?: number; trend?: unknown[] }>).forEach((o) => {
      m.set(o.brand_id, computeHealth([{ avg_mention_rate: o.avg_mention_rate, trend: o.trend as never[] }], [], 0).total)
    })
    return m
  }, [report, listOverviews])
  // GEO 体检数据源（与关键词/内容页共享缓存）
  const { data: allKeywords = [] } = useQuery({
    queryKey: ['geo-all-keywords'],
    queryFn: () => businessApi.listAllKeywords().catch(() => []),
  })
  const { data: brandContents = [] } = useQuery({
    queryKey: ['geo-contents', selectedBrand?.id],
    queryFn: () => businessApi.listContents(selectedBrand!.id).catch(() => []),
    enabled: !!selectedBrand,
  })
  // 内容引用（每篇被 AI 引用次数）
  const { data: citations = {} } = useQuery({
    queryKey: ['geo-citations', selectedBrand?.id],
    queryFn: () => businessApi.getContentCitations(selectedBrand!.id).catch(() => ({}) as Record<string, number>),
    enabled: !!selectedBrand,
  })
  // NAP 摘要（本地品牌的地基事实——地址/电话/营业时间）
  const { data: stores = [] } = useQuery({
    queryKey: ['geo-stores', selectedBrand?.id],
    queryFn: () => businessApi.listStoreLocations(selectedBrand!.id).catch(() => []),
    enabled: !!selectedBrand && (selectedBrand?.biz_type !== 'online'),
  })
  // 监测结果（竞品 AI 提及率聚合）
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch(() => []),
  })

  // 单品牌总览（工作区头部健康分/提及率）——报告口径与列表徽章统一
  const { data: overview } = useQuery({
    queryKey: ['geo-overview', selectedBrand?.id],
    queryFn: () => businessApi.getBrandOverview(selectedBrand!.id, selectedBrand?.name).catch(() => null),
    enabled: !!selectedBrand,
  })
  const brandHealth = report?.brands.find((b) => b.brand_id === selectedBrand?.id)?.total
    ?? computeHealth(
      overview ? [{ avg_mention_rate: (overview as { avg_mention_rate?: number })?.avg_mention_rate, trend: (overview as { trend?: unknown[] })?.trend as never[] }] : [],
      brandContents,
      brandContents.filter((c: { status?: string }) => c.status === 'published').length,
    ).total
  const bhLv = healthLevel(brandHealth)

  // 竞品 AI 数据：该品牌最近监测中的竞品提及率（对标思想——竞品不只是名字，还有 AI 表现）
  const compAiStats = useMemo(() => {
    const mine = monitorResults.filter((r: { brand_id: string }) => r.brand_id === selectedBrand?.id)
    if (mine.length === 0) return null
    return competitorStats(mine)
  }, [monitorResults, selectedBrand?.id]) // eslint-disable-line react-hooks/exhaustive-deps
  const compAi = compAiStats?.threats.slice(0, 6) || []
  const selfRate = compAiStats?.selfAvg || 0

  // 自家情感分布（AI 提到你时的态度——竞品 Tab 对标卡用）。
  // 口径统一：每关键词取最新一条（与总览 Tab 同口径——修复"全部历史不去重"的口径漂移）
  const sentDist = useMemo(() => {
    const mine = latestByKeyword(monitorResults.filter((r: { brand_id: string }) => r.brand_id === selectedBrand?.id))
    const d = { positive: 0, neutral: 0, negative: 0 }
    mine.forEach((r: { sentiment?: string }) => {
      if (r.sentiment === 'positive') d.positive += 1
      else if (r.sentiment === 'negative') d.negative += 1
      else d.neutral += 1
    })
    const total = Math.max(1, mine.length)
    return {
      total: mine.length,
      positive: Math.round((d.positive / total) * 100),
      neutral: Math.round((d.neutral / total) * 100),
      negative: Math.round((d.negative / total) * 100),
    }
  }, [monitorResults, selectedBrand?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  // 竞品正面占比（v3 P2：对标的第二维度——该竞品在最新结果中被评价为正面的比例，
  // 与自家 sentDist.positive 并排构成"提及率+情感"双维对比）
  const compPositiveMap = useMemo(() => {
    const m = new Map<string, { pos: number; total: number }>()
    latestByKeyword(monitorResults.filter((r: { brand_id: string }) => r.brand_id === selectedBrand?.id)).forEach((r: { competitor_sentiments?: Record<string, string> }) => {
      Object.entries(r.competitor_sentiments || {}).forEach(([name, s]) => {
        if (!s) return
        const cur = m.get(name) || { pos: 0, total: 0 }
        cur.total++
        if (s === 'positive') cur.pos++
        m.set(name, cur)
      })
    })
    return m
  }, [monitorResults, selectedBrand?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  // "AI 眼中的你"（品牌语义场预览——可解释性思想：AI 回答里将使用的事实就是这些）
  const aiView = (() => {
    if (!selectedBrand) return ''
    const lines: string[] = []
    lines.push(`【${selectedBrand.name}】${selectedBrand.industry?.trim() || '（行业未填写）'}`)
    lines.push(`定位：${selectedBrand.positioning?.trim() || '（未填写——AI 无法准确描述你）'}`)
    lines.push(`核心卖点：${selectedBrand.core_selling?.length ? selectedBrand.core_selling.join('、') : '（未填写——AI 不知道你强在哪）'}`)
    lines.push(`竞品：${selectedBrand.competitors?.length ? selectedBrand.competitors.join('、') : '（未配置——AI 回答里没有对比参照）'}`)
    if (selectedBrand.website_url?.trim()) lines.push(`官网：${selectedBrand.website_url.trim()}`)
    if (selectedBrand.biz_type !== 'online') {
      if (stores.length > 0) {
        const s = stores[0] as { address?: string; phone?: string }
        lines.push(`门店：${stores.length} 家（${s.address || ''}${s.phone ? ' · ' + s.phone : ''}）`)
      } else {
        lines.push('门店：未建档——本地品牌没有 NAP 事实，AI 本地回答将无据可引')
      }
    }
    return lines.join('\n')
  })()
  const citeTotal = Object.values(citations as Record<string, number>).reduce((s, n) => s + n, 0)

  // GEO 体检（VisiGEO 思想轻量版）：静态检查"信源资产是否完备"，逐项给建议
  const auditItems = (() => {
    if (!selectedBrand) return []
    const kwCount = allKeywords.filter((k: { brand_id: string }) => k.brand_id === selectedBrand.id).length
    const published = brandContents.filter((c: { status?: string }) => c.status === 'published').length
    return [
      { label: '品牌定位与卖点已填写', field: 'positioning', ok: !!selectedBrand.positioning?.trim() && (selectedBrand.core_selling?.length || 0) > 0, hint: '定位与卖点是 AI 生成内容时引用的核心事实' },
      { label: '竞品已配置', field: 'competitors', ok: (selectedBrand.competitors?.length || 0) > 0, hint: '竞品是监测对比坐标系——没有竞品就没有"差距"叙事' },
      { label: '官网地址已填写', field: 'website_url', ok: !!selectedBrand.website_url?.trim(), hint: selectedBrand.biz_type === 'online' ? '线上品牌必填——NAP 会注入内容与结构化数据' : '本地品牌可选，填写可增强权威性' },
      { label: '行业已填写', field: 'industry', ok: !!selectedBrand.industry?.trim(), hint: '行业决定知识库素材检索范围与平台行业看板——留空则全行业检索' },
      { label: '关键词已添加（≥3）', ok: kwCount >= 3, hint: '关键词是监测与内容生成的输入——至少 3 个才能形成可见度基线' },
      { label: '内容已发布（≥1）', ok: published >= 1, hint: '发布内容到公开站后 AI 引擎才能爬取引用——信源完整度的前提' },
    ]
  })()
  const auditScore = auditItems.filter((i) => i.ok).length

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['geo-brands'] })
    queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
  }

  // 深链预选（P0-1-1）：从工作台品牌卡/其他页面带 currentBrand 进入时自动选中
  useEffect(() => {
    if (!selectedBrand && currentBrandId && brands.length > 0) {
      const hit = brands.find((b: Brand) => b.id === currentBrandId)
      if (hit) setSelectedBrand(hit)
    }
  }, [brands, currentBrandId, selectedBrand])

  // 选中品牌时同步编辑表单
  useEffect(() => {
    if (selectedBrand) {
      editForm.setFieldsValue({
        name: selectedBrand.name,
        biz_type: selectedBrand.biz_type || 'local',
        industry: selectedBrand.industry || '',
        website_url: selectedBrand.website_url || '',
        positioning: selectedBrand.positioning,
        core_selling: (selectedBrand.core_selling || []).join('、'),
        competitors: (selectedBrand.competitors || []).join('、'),
      })
    }
  }, [selectedBrand]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleCreateBrand = async (values: { name: string; positioning: string; core_selling: string; competitors: string; biz_type?: string; industry?: string; website_url?: string }) => {
    try {
      const created = await businessApi.createBrand({
        name: values.name,
        positioning: values.positioning,
        core_selling: values.core_selling ? values.core_selling.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean) : [],
        competitors: values.competitors ? values.competitors.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean) : [],
        biz_type: values.biz_type || 'local',
        industry: values.industry || '',
        website_url: values.website_url || '',
      })
      setBrandModalOpen(false)
      brandForm.resetFields()
      invalidate()
      if (created?.id) {
        // 写入全局品牌上下文：后续页面（关键词/内容/监测）自动预选该品牌
        setCurrentBrand(created.id)
        setSelectedBrand(created)
      }
      // 成功即下一步：GEO 旅程是 品牌→关键词→监测→内容，创建完成直接引导去添加关键词
      Modal.success({
        title: `品牌「${values.name}」创建成功`,
        content: '下一步：为这个品牌添加关键词，AI 才知道要为哪些搜索词优化可见度。',
        okText: '去添加关键词',
        cancelText: '留在本页',
        onOk: () => navigate('/m/keywords'),
      })
    } catch { /* 拦截器已提示 */ }
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
        industry: values.industry || '',
        website_url: values.website_url || '',
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
    } catch { /* 拦截器已提示 */ } finally {
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
          <p>品牌定位、卖点与竞品——关键词请前往「关键词工程」</p>
        </div>
        <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setBrandModalOpen(true)}>
          创建品牌
        </Button>
      </div>

      <div className="wr-brands-layout" style={{ display: 'flex', gap: 16, minHeight: 'calc(100vh - 200px)' }}>
        {/* 左：品牌列表（窄屏断点见 .wr-brands-layout 媒体查询——上下堆叠） */}
        <div className="wr-brands-list wr-glass-card" style={{ width: 340, flexShrink: 0, display: 'flex', flexDirection: 'column', gap: 8, padding: 16 }}>
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
                      {/* 列表健康分概览（P1-2-1）：切换品牌前先见强弱 */}
                      {(() => {
                        const h = brandHealthMap.get(brand.id)
                        if (h === undefined) return null
                        const lv = healthLevel(h)
                        return (
                          <Tooltip title={`GEO 健康分 ${h}（${lv.label}）`}>
                            <span style={{
                              marginLeft: 'auto', padding: '1px 7px', borderRadius: 7, fontSize: 10.5, fontWeight: 700,
                              background: `${lv.color}1a`, color: lv.color, border: `1px solid ${lv.color}33`,
                            }}>{h}</span>
                          </Tooltip>
                        )
                      })()}
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* 右：品牌工作区（Hub——品牌名+健康分+关键指标行 + 三 Tab） */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {selectedBrand ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* 工作区头部：品牌名 + 健康分徽章 + 关键指标行 */}
              <div className="wr-glass-card" style={{ padding: '16px 20px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
                  <Space size={10}>
                    <Text strong style={{ fontSize: 18, letterSpacing: '-0.01em' }}>{selectedBrand.name}</Text>
                    <Tag color={selectedBrand.biz_type === 'online' ? 'blue' : 'green'} style={{ margin: 0, fontSize: 10 }}>
                      {selectedBrand.biz_type === 'online' ? '💻 线上业务' : '🏪 本地生意'}
                    </Tag>
                    <Tooltip title={`GEO 健康分 ${brandHealth}（${bhLv.label}）——立身份与盯数据的综合表现`}>
                      <span style={{ padding: '2px 8px', borderRadius: 8, fontSize: 12, fontWeight: 700, background: `${bhLv.color}1a`, color: bhLv.color, border: `1px solid ${bhLv.color}33` }}>
                        {brandHealth} · {bhLv.label}
                      </span>
                    </Tooltip>
                  </Space>
                  <Space size={20}>
                    <span>
                      <Text type="secondary" style={{ fontSize: 11 }}>提及率 </Text>
                      <Text strong style={{ fontSize: 16, color: rateColor((overview as { avg_mention_rate?: number })?.avg_mention_rate || 0) }}>
                        {Math.round(((overview as { avg_mention_rate?: number })?.avg_mention_rate || 0) * 100)}%
                      </Text>
                    </span>
                    <span>
                      <Text type="secondary" style={{ fontSize: 11 }}>关键词 </Text>
                      <Text strong style={{ fontSize: 16 }}>{allKeywords.filter((k: { brand_id: string }) => k.brand_id === selectedBrand.id).length}</Text>
                    </span>
                    <span>
                      <Text type="secondary" style={{ fontSize: 11 }}>内容 </Text>
                      <Text strong style={{ fontSize: 16 }}>{brandContents.length}</Text>
                    </span>
                    <span>
                      <Text type="secondary" style={{ fontSize: 11 }}>被引用 </Text>
                      <Text strong style={{ fontSize: 16, color: citeTotal > 0 ? 'var(--wr-accent)' : undefined }}>{citeTotal}</Text>
                    </span>
                    <Button size="small" type="text" onClick={() => setSelectedBrand(null)}>关闭</Button>
                  </Space>
                </div>
              </div>

              <Tabs
                items={[
                  { key: 'profile', label: '品牌档案', children: (<>
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
                  <Form.Item label="行业" name="industry" tooltip="如 餐饮/SaaS 工具——知识库素材检索与平台行业看板的过滤维度（留空则全行业检索）">
                    <AutoComplete options={INDUSTRY_OPTIONS.map((v) => ({ value: v }))} placeholder="如 餐饮、SaaS/软件工具">
                      <Input maxLength={20} />
                    </AutoComplete>
                  </Form.Item>
                  <Form.Item label="官网地址" name="website_url"
                    tooltip="online 品牌的 NAP——内容生成时注入'了解更多：https://...'"
                    dependencies={['biz_type']}
                    rules={websiteRules(watchedBizType === 'online')}>
                    <Input placeholder="https://example.com（线上品牌必填，本地品牌可选）" />
                  </Form.Item>
                  <Form.Item label="品牌定位" name="positioning" rules={[{ max: 200, message: '品牌定位 ≤200 字' }, CLEAN_TEXT_VALIDATOR]}>
                    <TextArea placeholder="描述品牌的核心价值" autoSize={{ minRows: 2, maxRows: 4 }} />
                  </Form.Item>
                  <Form.Item label="核心卖点" name="core_selling" tooltip="用顿号或逗号分隔（单项 ≤30 字，最多 8 项）" rules={[CLEAN_TEXT_VALIDATOR]}>
                    <TextArea placeholder="10年经验、环保材料、终身保修" autoSize={{ minRows: 2 }} />
                  </Form.Item>
                  <Form.Item label="竞品" name="competitors" tooltip="用顿号或逗号分隔。可点击下方「从附近同行推荐」自动补充" rules={[CLEAN_TEXT_VALIDATOR]}>
                    <TextArea placeholder="竞品A、竞品B、竞品C" autoSize={{ minRows: 1 }} />
                  </Form.Item>
                  <Button type="primary" loading={savingBrand} onClick={handleSaveBrand}>保存品牌信息</Button>
                </Form>
              </div>

              {/* GEO 体检卡（VisiGEO 思想轻量版：信源资产完备度检查——老板知道还差什么） */}
              <div className="wr-glass-card" style={{ padding: 24 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                  <Space>
                    <EnvironmentOutlined style={{ color: 'var(--wr-success)' }} />
                    <Text strong style={{ fontSize: 16 }}>GEO 体检</Text>
                    <Tag color={auditScore === 6 ? 'success' : auditScore >= 4 ? 'warning' : 'error'} style={{ fontSize: 11 }}>
                      {auditScore}/6
                    </Tag>
                  </Space>
                  <Tooltip title="对齐行业站点审计思想：GEO 效果的前提是信源资产完备——逐项补齐后 AI 才有东西可引用">
                    <Text type="secondary" style={{ fontSize: 11 }}>信源资产完备度</Text>
                  </Tooltip>
                </div>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  {auditItems.map((item) => (
                    <div key={item.label} style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                      <span style={{ color: item.ok ? 'var(--wr-success)' : 'var(--wr-danger)', fontSize: 14, lineHeight: '20px' }}>
                        {item.ok ? '✓' : '✗'}
                      </span>
                      <div style={{ flex: 1 }}>
                        <Text style={{ fontSize: 13, color: item.ok ? 'var(--wr-text-primary)' : 'var(--wr-text-secondary)' }}>{item.label}</Text>
                        {!item.ok && (
                          <>
                            <Text type="secondary" style={{ fontSize: 11, display: 'block', lineHeight: 1.5 }}>{item.hint}</Text>
                            {/* F1-4：未达标项直达修复——点击滚动定位到对应编辑框，消除"知道缺什么但不知道去哪补" */}
                            <a style={{ fontSize: 11 }} onClick={() => editForm.scrollToField(item.field, { behavior: 'smooth', block: 'center' })}>
                              去补填 →
                            </a>
                          </>
                        )}
                      </div>
                    </div>
                  ))}
                </Space>
              </div>

            {/* AI 眼中的你（品牌语义场——可解释性：让用户看到 AI 将如何使用这些事实） */}
            <div className="wr-glass-card" style={{ padding: 24 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <Space>
                  <RobotOutlined style={{ color: 'var(--wr-primary)' }} />
                  <Text strong style={{ fontSize: 16 }}>AI 眼中的你</Text>
                </Space>
                <Tooltip title="品牌语义场思想：GEO 的本质是让 AI 正确理解你——这里预览的每一行都会被注入内容生成与结构化数据，AI 回答里提到你时用的就是这些事实">
                  <Text type="secondary" style={{ fontSize: 11 }}>可解释性预览</Text>
                </Tooltip>
              </div>
              <div style={{
                padding: 14, borderRadius: 10, background: 'var(--wr-bg-elevated)',
                border: '1px solid var(--wr-border)', fontSize: 13, lineHeight: 1.9,
                whiteSpace: 'pre-wrap', color: 'var(--wr-text-secondary)',
              }}>
                {aiView}
              </div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 8 }}>
                灰色（未填写）项 = AI 理解你的事实缺口——补齐后 AI 才能准确描述、引用你（见上方 GEO 体检）
              </Text>
            </div>

            {/* 内容资产 + NAP 摘要（立身份与建资产打通） */}
            <div className="wr-glass-card" style={{ padding: 24 }}>
              <Space style={{ marginBottom: 12 }}>
                <FileTextOutlined style={{ color: 'var(--wr-success)' }} />
                <Text strong style={{ fontSize: 16 }}>内容资产</Text>
              </Space>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 12 }}>
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>内容总数</Text>
                  <Text strong style={{ fontSize: 22 }}>{brandContents.length}</Text>
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>已发布（AI 可爬）</Text>
                  <Text strong style={{ fontSize: 22, color: 'var(--wr-success)' }}>
                    {brandContents.filter((c: { status?: string }) => c.status === 'published').length}
                  </Text>
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>被 AI 引用</Text>
                  <Text strong style={{ fontSize: 22, color: citeTotal > 0 ? 'var(--wr-accent)' : undefined }}>{citeTotal}</Text>
                </div>
                {selectedBrand.biz_type !== 'online' && (
                  <div>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>门店档案（NAP）</Text>
                    <Text strong style={{ fontSize: 22 }}>{stores.length}</Text>
                    <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
                      {stores.length > 0
                        ? (stores[0] as { address?: string }).address?.slice(0, 18) || '已建档'
                        : '未建档'}
                    </Text>
                  </div>
                )}
              </div>
              <Space style={{ marginTop: 12 }}>
                <Button size="small" type="link" onClick={() => navigate('/m/content')}>去生成内容 →</Button>
                {selectedBrand.biz_type !== 'online' && (
                  <Button size="small" type="link" onClick={() => navigate('/m/nearby')}>管理门店档案 →</Button>
                )}
              </Space>
            </div>

            </>)},
                  { key: 'stores', label: '门店与附近', children: <StoreTab brand={selectedBrand} /> },
                  { key: 'competitors', label: '竞品', children: (<>
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
                    {selectedBrand.competitors.map((c, i) => {
                      const ai = compAi.find((x) => x.name === c)
                      return (
                        <Tag key={i} closable color="orange" onClose={async () => {
                          const remaining = selectedBrand.competitors!.filter((_, idx) => idx !== i)
                          const updated = await businessApi.updateBrand(selectedBrand.id, { competitors: remaining })
                          setSelectedBrand(updated)
                          editForm.setFieldsValue({ competitors: remaining.join('、') })
                          invalidate()
                        }}>
                          {c}
                          {ai && (
                            <Tooltip title={`AI 回答中提及率 ${ai.avgRate.toFixed(0)}%${ai.sentiment === 'positive' ? '· 被推荐' : ai.sentiment === 'negative' ? '· 被批评' : ''}${ai.avgRate > 0 ? '（高于你时标红提醒）' : ''}`}>
                              <span style={{ marginLeft: 6, color: ai.avgRate > 0 ? 'var(--wr-danger)' : 'var(--wr-text-muted)' }}>
                                {ai.avgRate.toFixed(0)}%
                                {ai.sentiment === 'positive' && <CheckCircleOutlined style={{ color: 'var(--wr-success)', fontSize: 11, marginLeft: 3 }} />}
                                {ai.sentiment === 'negative' && <CloseCircleOutlined style={{ color: 'var(--wr-danger)', fontSize: 11, marginLeft: 3 }} />}
                              </span>
                            </Tooltip>
                          )}
                        </Tag>
                      )
                    })}
                  </Space>
                ) : (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无竞品——手动添加或从附近同行推荐" style={{ padding: 20 }} />
                )}
              </div>

              {/* 竞品 AI 对标：自家 vs 竞品提及率 + 自家情感分布（规划 4.3 竞品 Tab 增强） */}
              <div className="wr-glass-card" style={{ padding: 24 }}>
                <Space style={{ marginBottom: 12 }}>
                  <RadarChartOutlined style={{ color: 'var(--wr-danger)' }} />
                  <Text strong style={{ fontSize: 16 }}>AI 对标</Text>
                </Space>
                {compAi.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无竞品监测数据——在「AI 可见度」发起监测后自动生成" />
                ) : (
                  <>
                    <Space direction="vertical" size={10} style={{ width: '100%', marginBottom: 16 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                        <Text style={{ fontSize: 13, fontWeight: 600, color: 'var(--wr-primary)' }}>
                          你（{Math.round(selfRate * 100)}%{sentDist.total > 0 ? ` · 正面 ${sentDist.positive}%` : ''}）
                        </Text>
                        <Text style={{ fontSize: 12, color: 'var(--wr-text-muted)' }}>基准线</Text>
                      </div>
                      <div style={{ height: 8, background: 'var(--wr-bg-elevated)', borderRadius: 4, overflow: 'hidden' }}>
                        <div style={{ height: '100%', width: `${Math.min(100, selfRate * 100)}%`, background: 'var(--wr-primary)', borderRadius: 4 }} />
                      </div>
                      {compAi.map((c) => {
                        const pr = compPositiveMap.get(c.name)
                        return (
                        <div key={c.name}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                            <Space size={6}>
                              <Text style={{ fontSize: 13 }}>{c.name}</Text>
                              {pr && pr.total > 0 && (
                                <Text type="secondary" style={{ fontSize: 11 }}>正面 {Math.round((pr.pos / pr.total) * 100)}%</Text>
                              )}
                            </Space>
                            <Text strong style={{ fontSize: 13, color: c.avgRate > selfRate * 100 ? 'var(--wr-danger)' : 'var(--wr-text-secondary)' }}>
                              {c.avgRate.toFixed(0)}%
                            </Text>
                          </div>
                          <div style={{ height: 6, background: 'var(--wr-bg-elevated)', borderRadius: 3, overflow: 'hidden' }}>
                            <div style={{ height: '100%', width: `${Math.min(100, c.avgRate)}%`, background: c.avgRate > selfRate * 100 ? 'var(--wr-danger)' : 'var(--wr-accent)', borderRadius: 3 }} />
                          </div>
                        </div>
                        )
                      })}
                    </Space>
                    <div style={{ paddingTop: 12, borderTop: '1px solid var(--wr-border)' }}>
                      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>你的情感分布（AI 提到你时的态度）</Text>
                      <Space size={8} wrap>
                        <Tag color="success" style={{ margin: 0 }}>正面 {sentDist.positive}%</Tag>
                        <Tag style={{ margin: 0 }}>中性 {sentDist.neutral}%</Tag>
                        <Tag color="error" style={{ margin: 0 }}>负面 {sentDist.negative}%</Tag>
                        <Text type="secondary" style={{ fontSize: 11 }}>基于 {sentDist.total} 条采样</Text>
                      </Space>
                    </div>
                  </>
                )}
              </div>
            </>)},
                ]}
              />
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
          <Form.Item label="品牌名" name="name" rules={[{ required: true, message: '请输入品牌名' }, { max: 50, message: '品牌名 ≤50 字' }, CLEAN_TEXT_VALIDATOR]}>
            <Input placeholder={watchedCreateBizType === 'online' ? '如 NoteFlow、某 SaaS 工具' : '如 某装修公司、某餐厅'} />
          </Form.Item>
          <Form.Item label="业务类型" name="biz_type" tooltip="本地生意（餐厅/装修/理发）：有门店+附近同行对比+本地搜索词；线上业务（SaaS/工具/网络公司）：无地理约束+品类搜索词+行业竞品">
            <Select options={[
              { value: 'local', label: '🏪 本地生意（有门店，做附近同行对比）' },
              { value: 'online', label: '💻 线上业务（无门店，做行业竞品对比）' },
            ]} />
          </Form.Item>
          <Form.Item label="官网地址" name="website_url" dependencies={['biz_type']} tooltip="online 品牌的 NAP——内容生成时注入'了解更多：https://...'" rules={websiteRules(watchedCreateBizType === 'online')}>
            <Input placeholder="https://example.com（线上品牌必填，本地品牌可选）" />
          </Form.Item>
          <Form.Item label="行业" name="industry" tooltip="如 餐饮/SaaS 工具——平台按行业采集知识库素材，生成时优先检索同行业素材；也用于平台行业看板（留空则全行业检索）">
            <AutoComplete options={INDUSTRY_OPTIONS.map((v) => ({ value: v }))} placeholder={watchedCreateBizType === 'online' ? '如 SaaS/软件工具、电商/零售' : '如 餐饮、美业/美容美发'}>
              <Input maxLength={20} />
            </AutoComplete>
          </Form.Item>
          <Form.Item label="品牌定位" name="positioning" rules={[{ max: 200, message: '品牌定位 ≤200 字' }, CLEAN_TEXT_VALIDATOR]} tooltip="描述品牌的核心价值，AI 生成内容时会参考">
            <TextArea placeholder={watchedCreateBizType === 'online' ? '如 面向个人与团队的智能云笔记，AI 检索与多端同步' : '如 专注北京地区中高端家装，提供设计-施工-软装一站式服务'} autoSize={{ minRows: 2, maxRows: 4 }} />
          </Form.Item>
          <Form.Item label="核心卖点" name="core_selling" tooltip="用逗号分隔，如：10年经验,环保材料,终身保修（单项 ≤30 字，最多 8 项）" rules={[CLEAN_TEXT_VALIDATOR, {
            validator: (_: unknown, v: string) => {
              const items = (v || '').split(/[、,，]/).map((s) => s.trim()).filter(Boolean)
              if (items.length > 8) return Promise.reject(new Error('最多 8 个卖点'))
              if (items.some((it) => it.length > 30)) return Promise.reject(new Error('单项卖点 ≤30 字'))
              return Promise.resolve()
            },
          }]}>
            <TextArea placeholder="10年经验, 环保材料, 终身保修" autoSize={{ minRows: 2 }} />
          </Form.Item>
          <Form.Item label="竞品" name="competitors" tooltip="用逗号分隔，监测时会对比这些竞品的 AI 可见度。创建后可在品牌详情用「从附近同行推荐」自动补充" rules={[CLEAN_TEXT_VALIDATOR]}>
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

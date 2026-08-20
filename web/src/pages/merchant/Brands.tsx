import { useEffect, useMemo, useState } from 'react'
import { Typography, Button, Modal, Form, Input, Select, AutoComplete, Space, message, Popconfirm, Empty, Checkbox, Spin, Tag, Tooltip, Tabs, Collapse, Progress } from 'antd'
import { PlusOutlined, DeleteOutlined, BulbOutlined, EnvironmentOutlined, RadarChartOutlined, RobotOutlined, FileTextOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
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
import KnowledgeTab from './brands/KnowledgeTab'
import type { Brand, CompetitorSuggestion } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

// 品牌档案（输入之家 v3，傻瓜化）：
//   品牌切换 = 顶部小 Tab 栏（名称 + 健康分徽章 + ＋ 新品牌）；
//   三个内容 Tab——品牌资料（完善度嵌卡头 + 效果卡智能展开）/ 门店档案（本地品牌，
//   AI 本地回答的地基）/ 竞品（三条获取路 + 名单 + AI 对标闭环同屏）。
//   原则：品牌档案告诉 AI 你是谁（输入）；AI 体检看 AI 眼里的你（输出）。
export default function Brands() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const setCurrentBrand = useBrandStore((s) => s.setCurrentBrand)
  const currentBrandId = useBrandStore((s) => s.currentBrandId)
  const [brandModalOpen, setBrandModalOpen] = useState(false)
  const [selectedBrand, setSelectedBrand] = useState<Brand | null>(null)
  const [brandForm] = Form.useForm()
  const [editForm] = Form.useForm()
  // 竞品推荐
  const [compSuggestOpen, setCompSuggestOpen] = useState(false)
  const [suggestions, setSuggestions] = useState<CompetitorSuggestion[]>([])
  const [checkedComps, setCheckedComps] = useState<string[]>([])
  const [loadingSuggest, setLoadingSuggest] = useState(false)
  // 手动添加竞品（空态三条路之一）
  const [manualCompInput, setManualCompInput] = useState('')
  const [savingBrand, setSavingBrand] = useState(false)
  // 编辑表单校验联动（P0-2-2）：online 品牌官网必填
  const watchedBizType = Form.useWatch('biz_type', editForm)
  // 创建弹窗的业务类型联动（线上品牌才出现官网必填——渐进披露）
  const watchedCreateBizType = Form.useWatch('biz_type', brandForm)

  // F1-3：引导页 CTA 携带 location.state.openCreate 跳入——自动打开创建弹窗（消除冷启动断一步）
  const location = useLocation()
  useEffect(() => {
    if ((location.state as { openCreate?: boolean } | null)?.openCreate) {
      setBrandModalOpen(true)
      navigate('/m/brands', { replace: true, state: {} }) // 清 state 防刷新重复弹窗
    }
  }, [location.state, navigate])  

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })
  // 健康分徽章：后端健康报告（单一事实源，与工作台卡片统一口径）；报告不可用时降级本地合成
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
  // 资料完善度数据源（关键词/内容计数）
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
  // 门店档案（本地品牌"无门店"提示用——完整管理在「附近同行」页）
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
  // 单品牌总览（效果卡提及率）
  const { data: overview } = useQuery({
    queryKey: ['geo-overview', selectedBrand?.id],
    queryFn: () => businessApi.getBrandOverview(selectedBrand!.id, selectedBrand?.name).catch(() => null),
    enabled: !!selectedBrand,
  })

  // 品牌小 Tab 栏：自动选中（全局上下文优先，否则第一个品牌）——单品牌无空状态，
  // 删除品牌后自动落到剩余品牌
  useEffect(() => {
    if (brands.length === 0) {
      if (selectedBrand) setSelectedBrand(null)
      return
    }
    if (!selectedBrand || !brands.some((b: Brand) => b.id === selectedBrand.id)) {
      setSelectedBrand(brands.find((b: Brand) => b.id === currentBrandId) || brands[0])
    }
  }, [brands, currentBrandId, selectedBrand])

  const selectBrand = (b: Brand) => {
    setSelectedBrand(b)
    setCurrentBrand(b.id) // 同步全局品牌上下文：关键词/内容/监测页跟随切换
  }

  // 竞品 AI 数据：该品牌最近监测中的竞品提及率（对标思想——竞品不只是名字，还有 AI 表现）
  const compAiStats = useMemo(() => {
    const mine = monitorResults.filter((r: { brand_id: string }) => r.brand_id === selectedBrand?.id)
    if (mine.length === 0) return null
    return competitorStats(mine)
  }, [monitorResults, selectedBrand?.id])  
  const compAi = compAiStats?.threats.slice(0, 6) || []
  const selfRate = compAiStats?.selfAvg || 0

  // 自家情感分布（AI 提到你时的态度——竞品对比卡用）。
  // 口径统一：每关键词取最新一条
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
  }, [monitorResults, selectedBrand?.id])  

  // 竞品正面占比（对标的第二维度——与自家 sentDist.positive 并排构成"提及率+情感"双维对比）
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
  }, [monitorResults, selectedBrand?.id])  

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
        lines.push('门店：未建档——AI 介绍附近商家时将没有你的地址电话可引用')
      }
    }
    return lines.join('\n')
  })()
  const citeTotal = Object.values(citations as Record<string, number>).reduce((s, n) => s + n, 0)

  // 资料完善度（原 GEO 体检）：静态检查"AI 认识你的事实是否完备"。
  // 傻瓜化：不再列 6 项清单——进度嵌表单卡头，未完成项压缩为卡尾一行可点击提示
  const auditItems: { label: string; short: string; field?: string; link?: string; ok: boolean }[] = (() => {
    if (!selectedBrand) return []
    const kwCount = allKeywords.filter((k: { brand_id: string }) => k.brand_id === selectedBrand.id).length
    const published = brandContents.filter((c: { status?: string }) => c.status === 'published').length
    return [
      { label: '品牌定位与卖点已填写', short: '定位卖点', field: 'positioning', ok: !!selectedBrand.positioning?.trim() && (selectedBrand.core_selling?.length || 0) > 0 },
      { label: '竞品已配置', short: '竞品', field: 'competitors', ok: (selectedBrand.competitors?.length || 0) > 0 },
      { label: '官网地址已填写', short: '官网', field: 'website_url', ok: !!selectedBrand.website_url?.trim() },
      { label: '行业已填写', short: '行业', field: 'industry', ok: !!selectedBrand.industry?.trim() },
      { label: '关键词已有 3 个以上', short: '关键词（去添加）', link: '/m/checkup?tab=records', ok: kwCount >= 3 },
      { label: '已发布第 1 篇内容', short: '发布内容（去生成）', link: '/m/studio', ok: published >= 1 },
    ]
  })()
  const auditScore = auditItems.filter((i) => i.ok).length
  const auditPct = Math.round((auditScore / 6) * 100)
  const missingItems = auditItems.filter((i) => !i.ok)

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['geo-brands'] })
    queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
  }

  // 内容 Tab 受控（无门店提示等可跳转到门店档案 Tab）；切换品牌时回到资料页
  const [activeInnerTab, setActiveInnerTab] = useState('profile')
  useEffect(() => { setActiveInnerTab('profile') }, [selectedBrand?.id])

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
        content: '资料可以随时回来补齐。下一步：添加几个关键词（顾客会搜的词），AI 才知道为哪些搜索优化你。',
        okText: '去添加关键词',
        cancelText: '留在本页',
        onOk: () => navigate('/m/checkup?tab=records'),
      })
    } catch { /* 拦截器已提示 */ }
  }

  const handleDeleteBrand = async (id: string) => {
    try {
      await businessApi.deleteBrand(id)
      message.success('已删除')
      if (selectedBrand?.id === id) setSelectedBrand(null) // 自动选中效果会落到剩余品牌
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

  // 手动添加竞品（合并去重）
  const handleAddManualCompetitor = async () => {
    if (!selectedBrand) return
    const name = manualCompInput.trim()
    if (!name) return
    const existing = new Set(selectedBrand.competitors || [])
    if (existing.has(name)) {
      message.warning('该竞品已存在')
      return
    }
    try {
      const merged = [...(selectedBrand.competitors || []), name]
      const updated = await businessApi.updateBrand(selectedBrand.id, { competitors: merged })
      message.success(`已添加「${name}」`)
      setManualCompInput('')
      setSelectedBrand(updated)
      editForm.setFieldsValue({ competitors: merged.join('、') })
      invalidate()
    } catch { /* 拦截器已提示 */ }
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

  return (
    <div className="wr-page-content" style={{ paddingTop: 0 }}>
      <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h1>品牌管理</h1>
          <p>品牌资料 · 竞品对比——先把"你是谁"讲清楚，AI 才会推荐你</p>
        </div>
        <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setBrandModalOpen(true)}>
          创建品牌
        </Button>
      </div>

      {brands.length === 0 ? (
        <div className="wr-glass-card" style={{ padding: 64, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有品牌——创建第一个品牌，告诉 AI 你是谁">
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setBrandModalOpen(true)}>创建第一个品牌</Button>
          </Empty>
        </div>
      ) : (
        <>
          {/* 品牌小 Tab 栏（名称 + 健康分徽章；尾挂 ＋ 新品牌；多品牌横向滚动） */}
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', overflowX: 'auto', paddingBottom: 6, marginBottom: 10 }}>
            {brands.map((brand: Brand) => {
              const isActive = selectedBrand?.id === brand.id
              const h = brandHealthMap.get(brand.id)
              const lv = h !== undefined ? healthLevel(h) : null
              return (
                <Tooltip key={brand.id} title={lv ? `健康分 ${h}（${lv.label}）——立身份与盯数据的综合表现` : '还没有健康分（发起监测后产生）'}>
                  <div
                    onClick={() => selectBrand(brand)}
                    style={{
                      display: 'inline-flex', alignItems: 'center', gap: 7, padding: '5px 14px', borderRadius: 16,
                      border: `1px solid ${isActive ? 'var(--wr-primary)' : 'var(--wr-border)'}`,
                      background: isActive ? 'var(--wr-primary)' : 'var(--wr-bg-surface)',
                      cursor: 'pointer', whiteSpace: 'nowrap', flexShrink: 0,
                      transition: 'all 200ms cubic-bezier(0.2, 0, 0, 1)',
                    }}
                  >
                    <span style={{ fontSize: 13, fontWeight: isActive ? 600 : 400, color: isActive ? '#fff' : 'var(--wr-text-primary)' }}>
                      {brand.name}
                    </span>
                    {lv ? (
                      <span style={{
                        fontSize: 10.5, fontWeight: 700, padding: '0 6px', borderRadius: 8,
                        background: isActive ? 'rgba(255,255,255,0.2)' : `${lv.color}1a`,
                        color: isActive ? '#fff' : lv.color,
                      }}>{h}</span>
                    ) : (
                      <span style={{ fontSize: 10.5, color: isActive ? 'rgba(255,255,255,0.55)' : 'var(--wr-text-muted)' }}>—</span>
                    )}
                  </div>
                </Tooltip>
              )
            })}
            <div
              onClick={() => setBrandModalOpen(true)}
              style={{
                display: 'inline-flex', alignItems: 'center', gap: 5, padding: '5px 14px', borderRadius: 16,
                border: '1px dashed var(--wr-border)', cursor: 'pointer', whiteSpace: 'nowrap', flexShrink: 0,
                color: 'var(--wr-text-secondary)', fontSize: 13,
              }}
            >
              <PlusOutlined style={{ fontSize: 11 }} /> 新品牌
            </div>
          </div>

          {selectedBrand && (
            <Tabs
              activeKey={activeInnerTab}
              onChange={setActiveInnerTab}
              items={[
                {
                  key: 'profile',
                  label: '品牌资料',
                  children: (<>
                    {/* ① 品牌资料卡（完善度嵌卡头——填表单与看进度是同一件事） */}
                    <div className="wr-glass-card" style={{ padding: 24, marginBottom: 16 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16, flexWrap: 'wrap', gap: 10 }}>
                        <Space>
                          <BulbOutlined style={{ color: 'var(--wr-primary)' }} />
                          <Text strong style={{ fontSize: 16 }}>品牌资料</Text>
                          <Tag color={selectedBrand.biz_type === 'online' ? 'blue' : 'green'} style={{ margin: 0, fontSize: 10 }}>
                            {selectedBrand.biz_type === 'online' ? '线上业务' : '本地生意'}
                          </Tag>
                        </Space>
                        <Space size={14}>
                          <Space size={8}>
                            <Text type="secondary" style={{ fontSize: 12 }}>完善度</Text>
                            <Progress
                              percent={auditPct}
                              showInfo={false}
                              size="small"
                              style={{ width: 110, marginBottom: 0 }}
                              strokeColor={auditScore === 6 ? 'var(--wr-success)' : 'var(--wr-primary)'}
                            />
                            <Text strong style={{ fontSize: 13 }}>{auditPct}%（{auditScore}/6）</Text>
                          </Space>
                          <Popconfirm title="删除品牌及其关键词？" onConfirm={() => handleDeleteBrand(selectedBrand.id)}>
                            <Button size="small" type="text" danger icon={<DeleteOutlined />} style={{ opacity: 0.55 }}>删除</Button>
                          </Popconfirm>
                        </Space>
                      </div>

                      <Form form={editForm} layout="vertical" requiredMark={false}>
                        <Form.Item label="品牌名" name="name" rules={[{ required: true, message: '请输入品牌名' }]}>
                          <Input placeholder="品牌名" />
                        </Form.Item>
                        <Form.Item label="业务类型" name="biz_type" tooltip="有实体门店选本地生意（可以做附近同行对比）；只在线上经营选线上业务（可以做行业竞品对比）">
                          <Select options={[
                            { value: 'local', label: '本地生意（有门店，做附近同行对比）' },
                            { value: 'online', label: '线上业务（无门店，做行业竞品对比）' },
                          ]} />
                        </Form.Item>
                        <Form.Item label="行业" name="industry" tooltip="如 餐饮/SaaS 工具——AI 会优先参考同行业的资料；留空则全行业参考">
                          <AutoComplete options={INDUSTRY_OPTIONS.map((v) => ({ value: v }))} placeholder="如 餐饮、美业/美容美发">
                            <Input maxLength={20} />
                          </AutoComplete>
                        </Form.Item>
                        <Form.Item label="官网地址" name="website_url"
                          tooltip="发布的内容会自动附上官网链接，AI 引用你的内容时读者可直达官网（线上品牌必填）"
                          dependencies={['biz_type']}
                          rules={websiteRules(watchedBizType === 'online')}>
                          <Input placeholder="https://example.com（线上品牌必填，本地品牌可选）" />
                        </Form.Item>
                        <Form.Item label="品牌定位" name="positioning" tooltip="一句话说清你是干嘛的、给谁服务——AI 介绍你时会参考" rules={[{ max: 200, message: '品牌定位 ≤200 字' }, CLEAN_TEXT_VALIDATOR]}>
                          <TextArea placeholder="如 专注北京地区中高端家装，提供设计-施工-软装一站式服务" autoSize={{ minRows: 2, maxRows: 4 }} />
                        </Form.Item>
                        <Form.Item label="核心卖点" name="core_selling" tooltip="你最想让顾客记住的 3-8 个点，用顿号或逗号分隔（单项 ≤30 字）" rules={[CLEAN_TEXT_VALIDATOR]}>
                          <TextArea placeholder="10年经验、环保材料、终身保修" autoSize={{ minRows: 2 }} />
                        </Form.Item>
                        <Form.Item label="竞品" name="competitors" tooltip="你想对比的同行/对手，用逗号分隔。可留空——可用「竞品对比」页的一键推荐自动补充" rules={[CLEAN_TEXT_VALIDATOR]}>
                          <TextArea placeholder="竞品A、竞品B、竞品C（可留空，可自动推荐）" autoSize={{ minRows: 1 }} />
                        </Form.Item>
                        <Button type="primary" loading={savingBrand} onClick={handleSaveBrand}>保存品牌信息</Button>
                      </Form>

                      {/* 未完成项一行提示（傻瓜化：不列 6 项清单，只告诉还差什么、点了就去） */}
                      {missingItems.length > 0 && (
                        <div style={{ marginTop: 14, padding: '9px 12px', borderRadius: 8, background: 'var(--wr-primary-bg)', fontSize: 13, lineHeight: 1.9 }}>
                          <Text style={{ fontSize: 13 }}>还差 {missingItems.length} 项：</Text>
                          {missingItems.map((m, i) => (
                            <span key={m.label}>
                              <a style={{ fontSize: 13 }} onClick={() => {
                                if (m.field) editForm.scrollToField(m.field, { behavior: 'smooth', block: 'center' })
                                else if (m.link) navigate(m.link)
                              }}>{m.short}</a>
                              {i < missingItems.length - 1 && <Text type="secondary" style={{ fontSize: 12 }}> · </Text>}
                            </span>
                          ))}
                          <Text type="secondary" style={{ fontSize: 12 }}>——补齐后 AI 才能更准确地推荐你</Text>
                        </div>
                      )}

                      {/* 本地品牌无门店提示（门店档案就在隔壁 Tab——点击直达） */}
                      {selectedBrand.biz_type !== 'online' && stores.length === 0 && (
                        <div style={{ marginTop: 8, padding: '9px 12px', borderRadius: 8, background: 'var(--wr-bg-elevated)', fontSize: 13, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                          <EnvironmentOutlined style={{ color: 'var(--wr-warning)' }} />
                          <span>本地生意还没添加门店——补上门店地址，AI 推荐附近商家时才不会漏掉你</span>
                          <a style={{ fontSize: 13 }} onClick={() => setActiveInnerTab('stores')}>去「门店档案」添加 →</a>
                        </div>
                      )}
                    </div>

                    {/* ② 效果卡（智能展开：完善度不满时默认展开——它是补资料的最强动机；
                        6/6 满分时折叠（资料齐了，效果数据去体检中心看）。key 随品牌重置展开态） */}
                    <div className="wr-glass-card" style={{ padding: '6px 20px' }} key={selectedBrand.id}>
                      <Collapse
                        ghost
                        defaultActiveKey={auditScore < 6 ? ['effect'] : []}
                        items={[{
                          key: 'effect',
                          label: (
                            <Space size={8}>
                              <FileTextOutlined style={{ color: 'var(--wr-accent)' }} />
                              <Text strong style={{ fontSize: 14 }}>效果 · AI 眼中的你</Text>
                              <Text type="secondary" style={{ fontSize: 12 }}>提及率 / 被引用等结果信号，点开查看</Text>
                            </Space>
                          ),
                          children: (<>
                            {/* 结果指标（原头部指标行收敛于此） */}
                            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(130px, 1fr))', gap: 12, marginBottom: 16 }}>
                              <div>
                                <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>提及率</Text>
                                <Text strong style={{ fontSize: 20, color: rateColor((overview as { avg_mention_rate?: number })?.avg_mention_rate || 0) }}>
                                  {Math.round(((overview as { avg_mention_rate?: number })?.avg_mention_rate || 0) * 100)}%
                                </Text>
                              </div>
                              <div>
                                <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>关键词</Text>
                                <Text strong style={{ fontSize: 20 }}>{allKeywords.filter((k: { brand_id: string }) => k.brand_id === selectedBrand.id).length}</Text>
                              </div>
                              <div>
                                <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>内容（已发布）</Text>
                                <Text strong style={{ fontSize: 20 }}>
                                  {brandContents.length}
                                  <Text type="secondary" style={{ fontSize: 13 }}>（{brandContents.filter((c: { status?: string }) => c.status === 'published').length}）</Text>
                                </Text>
                              </div>
                              <div>
                                <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>被 AI 引用</Text>
                                <Text strong style={{ fontSize: 20, color: citeTotal > 0 ? 'var(--wr-accent)' : undefined }}>{citeTotal}</Text>
                              </div>
                            </div>

                            {/* AI 眼中的你（品牌语义场——可解释性：AI 回答里用的就是这些事实） */}
                            <div style={{
                              padding: 14, borderRadius: 10, background: 'var(--wr-bg-elevated)',
                              border: '1px solid var(--wr-border)', fontSize: 13, lineHeight: 1.9,
                              whiteSpace: 'pre-wrap', color: 'var(--wr-text-secondary)',
                            }}>
                              {aiView}
                            </div>
                            <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 8 }}>
                              灰色（未填写）项 = AI 理解你的事实缺口——补齐后 AI 才能准确描述、引用你
                            </Text>
                            <Space style={{ marginTop: 10 }}>
                              <Button size="small" type="link" onClick={() => navigate('/m/studio')}>去生成内容 →</Button>
                              <Button size="small" type="link" onClick={() => navigate('/m/checkup?tab=report')}>查看 AI 可见度 →</Button>
                            </Space>
                          </>),
                        }]}
                      />
                    </div>
                  </>),
                },
                ...(selectedBrand.biz_type !== 'online'
                  ? [{ key: 'stores', label: '门店档案', children: <StoreTab brand={selectedBrand} /> }]
                  : []),
                {
                  key: 'knowledge',
                  label: '知识库',
                  children: <KnowledgeTab brandId={selectedBrand.id} />,
                },
                {
                  key: 'competitors',
                  label: '竞品对比',
                  children: (
                    /* 竞品对比（管理 + AI 对标合并为一张卡——竞品是同一件事的两个面） */
                    <div className="wr-glass-card" style={{ padding: 24 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8, flexWrap: 'wrap', gap: 8 }}>
                        <Space>
                          <RadarChartOutlined style={{ color: 'var(--wr-accent)' }} />
                          <Text strong style={{ fontSize: 16 }}>竞品对比</Text>
                          <Tag color="purple" style={{ margin: 0 }}>{selectedBrand.competitors?.length || 0}</Tag>
                        </Space>
                        <Space>
                          {selectedBrand.biz_type !== 'online' && (
                            <Button size="small" type="primary" ghost icon={<EnvironmentOutlined />} onClick={() => handleSuggestCompetitors('poi')}>
                              从附近同行推荐
                            </Button>
                          )}
                          <Button size="small" type="primary" ghost icon={<RadarChartOutlined />} onClick={() => handleSuggestCompetitors('monitoring')}>
                            从监测结果推荐
                          </Button>
                        </Space>
                      </div>
                      <Text type="secondary" style={{ display: 'block', marginBottom: 14, fontSize: 13, lineHeight: 1.6 }}>
                        竞品是你观察差距的参照——发起监测后，这里能看到"你 vs 竞品"在 AI 回答里的提及率对比。
                        {selectedBrand.biz_type !== 'online' && '本地品牌可一键从附近同行（按评分/距离）推荐竞品候选。'}
                      </Text>

                      {selectedBrand.competitors && selectedBrand.competitors.length > 0 ? (
                        <Space size={6} wrap style={{ marginBottom: 4 }}>
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
                        /* 空态 = 三条获取路（傻瓜化 3b：不是一句 Empty，而是"怎么得到竞品"的入口卡） */
                        <div style={{ padding: '16px 0 8px' }}>
                          <Text type="secondary" style={{ display: 'block', fontSize: 13, marginBottom: 12 }}>
                            竞品是你观察差距的参照——三种方式获得：
                          </Text>
                          <Space direction="vertical" size={10} style={{ width: '100%' }}>
                            {selectedBrand.biz_type !== 'online' && (
                              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                                <Button size="small" type="primary" ghost icon={<EnvironmentOutlined />} onClick={() => handleSuggestCompetitors('poi')}>
                                  从附近同行推荐
                                </Button>
                                <Text type="secondary" style={{ fontSize: 12 }}>按评分和距离，搜你门店周边的对手</Text>
                              </div>
                            )}
                            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                              <Button size="small" type="primary" ghost icon={<RadarChartOutlined />} onClick={() => handleSuggestCompetitors('monitoring')}>
                                从监测结果推荐
                              </Button>
                              <Text type="secondary" style={{ fontSize: 12 }}>AI 回答里出现过的对手（需先测过一题）</Text>
                            </div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                              <Input
                                size="small"
                                style={{ width: 200 }}
                                placeholder="手动输入竞品名"
                                maxLength={30}
                                value={manualCompInput}
                                onChange={(e) => setManualCompInput(e.target.value)}
                                onPressEnter={handleAddManualCompetitor}
                              />
                              <Button size="small" icon={<PlusOutlined />} onClick={handleAddManualCompetitor} disabled={!manualCompInput.trim()}>添加</Button>
                              <Text type="secondary" style={{ fontSize: 12 }}>知道对手是谁，直接填</Text>
                            </div>
                          </Space>
                        </div>
                      )}

                      {/* AI 对标：自家 vs 竞品提及率 + 自家情感分布 */}
                      <div style={{ borderTop: '1px solid var(--wr-border)', margin: '18px 0 14px' }} />
                      <Space size={8} style={{ marginBottom: 12 }}>
                        <RobotOutlined style={{ color: 'var(--wr-primary)' }} />
                        <Text strong style={{ fontSize: 14 }}>AI 眼里的你 vs 竞品</Text>
                      </Space>
                      {compAi.length === 0 ? (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', padding: '8px 0' }}>
                          <Text type="secondary" style={{ fontSize: 13 }}>还没有和竞品的对比数据</Text>
                          <Button size="small" type="primary" onClick={() => navigate('/m/checkup?tab=ask')}>去问一题，测出差距</Button>
                        </div>
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
                            <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>AI 提到你时的态度</Text>
                            <Space size={8} wrap>
                              <Tag color="success" style={{ margin: 0 }}>正面 {sentDist.positive}%</Tag>
                              <Tag style={{ margin: 0 }}>中性 {sentDist.neutral}%</Tag>
                              <Tag color="error" style={{ margin: 0 }}>负面 {sentDist.negative}%</Tag>
                              <Text type="secondary" style={{ fontSize: 11 }}>基于 {sentDist.total} 次监测</Text>
                            </Space>
                            {/* 行动暗示（傻瓜化 3c：数据变行动——竞品领先时该干什么） */}
                            <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 10, paddingTop: 10, borderTop: '1px dashed var(--wr-border)' }}>
                              竞品领先你时：去「AI 体检 · 引用归因」看看它被 AI 引用了什么内容，针对性写一篇——
                              <a style={{ fontSize: 12 }} onClick={() => navigate('/m/checkup?tab=records&sub=citations')}>去看引用归因</a>
                            </Text>
                          </div>
                        </>
                      )}
                    </div>
                  ),
                },
              ]}
            />
          )}
        </>
      )}

      {/* 创建品牌弹窗——傻瓜化：首屏只有 2 个必填（品牌名+业务类型），其余收进选填折叠区。
          线上品牌才出现官网必填（渐进披露）；资料可创建后在"品牌档案"渐进补齐 */}
      <Modal title="创建品牌" open={brandModalOpen} onCancel={() => setBrandModalOpen(false)} footer={null} width={560}>
        <Text type="secondary" style={{ display: 'block', marginBottom: 16, fontSize: 13 }}>
          只需两步就能创建——资料越完整，AI 越懂你；选填部分随时可以回来补。
        </Text>
        <Form form={brandForm} layout="vertical" onFinish={handleCreateBrand} requiredMark={false} initialValues={{ biz_type: 'local' }}>
          <Form.Item label="品牌名" name="name" rules={[{ required: true, message: '请输入品牌名' }, { max: 50, message: '品牌名 ≤50 字' }, CLEAN_TEXT_VALIDATOR]}>
            <Input placeholder={watchedCreateBizType === 'online' ? '如 NoteFlow、某 SaaS 工具' : '如 某装修公司、某餐厅'} />
          </Form.Item>
          <Form.Item label="业务类型" name="biz_type" tooltip="有实体门店选本地生意（可以做附近同行对比）；只在线上经营选线上业务（可以做行业竞品对比）">
            <Select options={[
              { value: 'local', label: '本地生意（有门店，做附近同行对比）' },
              { value: 'online', label: '线上业务（无门店，做行业竞品对比）' },
            ]} />
          </Form.Item>
          {watchedCreateBizType === 'online' && (
            <Form.Item label="官网地址" name="website_url" tooltip="发布的内容会自动附上官网链接，AI 引用你的内容时读者可直达官网" rules={websiteRules(true)}>
              <Input placeholder="https://example.com" />
            </Form.Item>
          )}
          <Collapse
            ghost
            size="small"
            style={{ marginBottom: 8 }}
            items={[{
              key: 'optional',
              label: <span style={{ fontSize: 13 }}>选填：完善资料，让 AI 更懂你（推荐填写）</span>,
              children: (<>
                <Form.Item label="行业" name="industry" style={{ marginBottom: 12 }} tooltip="如 餐饮/SaaS 工具——AI 会优先参考同行业的资料；留空则全行业参考">
                  <AutoComplete options={INDUSTRY_OPTIONS.map((v) => ({ value: v }))} placeholder={watchedCreateBizType === 'online' ? '如 SaaS/软件工具、电商/零售' : '如 餐饮、美业/美容美发'}>
                    <Input maxLength={20} />
                  </AutoComplete>
                </Form.Item>
                <Form.Item label="品牌定位" name="positioning" style={{ marginBottom: 12 }} tooltip="一句话说清你是干嘛的、给谁服务——AI 介绍你时会参考" rules={[{ max: 200, message: '品牌定位 ≤200 字' }, CLEAN_TEXT_VALIDATOR]}>
                  <TextArea placeholder={watchedCreateBizType === 'online' ? '如 面向个人与团队的智能云笔记，AI 检索与多端同步' : '如 专注北京地区中高端家装，提供设计-施工-软装一站式服务'} autoSize={{ minRows: 2, maxRows: 4 }} />
                </Form.Item>
                <Form.Item label="核心卖点" name="core_selling" style={{ marginBottom: 12 }} tooltip="你最想让顾客记住的 3-8 个点，用顿号或逗号分隔（单项 ≤30 字）" rules={[CLEAN_TEXT_VALIDATOR, {
                  validator: (_: unknown, v: string) => {
                    const items = (v || '').split(/[、,，]/).map((s) => s.trim()).filter(Boolean)
                    if (items.length > 8) return Promise.reject(new Error('最多 8 个卖点'))
                    if (items.some((it) => it.length > 30)) return Promise.reject(new Error('单项卖点 ≤30 字'))
                    return Promise.resolve()
                  },
                }]}>
                  <TextArea placeholder="10年经验, 环保材料, 终身保修" autoSize={{ minRows: 2 }} />
                </Form.Item>
                <Form.Item label="竞品" name="competitors" style={{ marginBottom: 0 }} tooltip="你想对比的同行/对手，用逗号分隔。可留空——创建后可一键从附近同行自动推荐">
                  <Input placeholder="竞品A, 竞品B, 竞品C（可留空，后续自动推荐）" />
                </Form.Item>
              </>),
            }]}
          />
          <Form.Item style={{ marginTop: 16 }}>
            <Space>
              <Button type="primary" htmlType="submit">创建品牌</Button>
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
                        {s.rating > 0 && <Tag color="gold" style={{ fontSize: 10, margin: 0 }}>评分 {s.rating}</Tag>}
                        {s.distance_m > 0 && <Text type="secondary" style={{ fontSize: 11 }}>{s.distance_m < 1000 ? s.distance_m + '米' : (s.distance_m / 1000).toFixed(1) + '公里'}</Text>}
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

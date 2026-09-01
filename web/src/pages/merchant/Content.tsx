import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Typography, Button, Input, Select, Space, Empty, Tag, Row, Col, Spin, Tooltip, Switch, Collapse, Segmented } from 'antd'
import { FileTextOutlined, DatabaseOutlined, ClearOutlined, EditOutlined, ThunderboltOutlined, ExportOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { scoreColor, scoreLevel } from '../../utils/geo'
import { citabilityOf } from '../../utils/citability'
import { useBrandContext } from '../../hooks/useBrands'
import PublishToSiteButton from '../../components/PublishToSiteButton'
import ContentPreviewDrawer from '../../components/ContentPreviewDrawer'
import type { Brand, Keyword, OptimizedContent } from '../../types/api'
import { toast } from '../../utils/feedback'

const { Text, Paragraph } = Typography
const { TextArea } = Input

// 内容质量 5 维度（标签 + 取值 key + 溯源说明）
const DIMENSIONS: { label: string; key: keyof OptimizedContent['score']; tip: string }[] = [
  { label: '权威性', key: 'authority', tip: '有没有数据、来源、专业依据——AI 更信任有据可查的内容' },
  { label: '具体性', key: 'specificity', tip: '有没有具体数字和事实细节，而不是空话套话' },
  { label: '结构化', key: 'structure', tip: '小标题分段清不清晰——结构清楚的内容 AI 更容易摘录引用' },
  { label: '独特性', key: 'uniqueness', tip: '有没有别处没有的观点和信息——搬运内容不会被优先引用' },
  { label: '时效性', key: 'recency', tip: '信息新不新鲜——提到近期时间和新事件的加分' },
]

// 目标 AI 引擎偏好（按引擎偏好调整内容格式）
const ENGINE_OPTIONS = [
  { value: '', label: '通用（不指定）' },
  { value: 'chatgpt', label: 'ChatGPT' },
  { value: 'perplexity', label: 'Perplexity' },
  { value: 'kimi', label: 'Kimi' },
  { value: 'doubao', label: '豆包' },
]

// 想写什么内容（傻瓜化：场景化命名——让用户选"要干嘛"，不是选"格式参数"）
const FORMAT_OPTIONS = [
  { value: '', label: '写一篇网站文章（默认）' },
  { value: 'review', label: '写顾客好评风格的文案' },
  { value: 'xiaohongshu', label: '写种草笔记（适合发小红书）' },
  { value: 'script', label: '写短视频口播稿' },
  { value: 'faq', label: '写常见问题解答' },
  { value: 'comparison', label: '写和同行的对比评测' },
  { value: 'citation', label: '写深度长文（最容易被 AI 引用）' },
]

export default function Content({ embedded }: { embedded?: boolean }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  // 全局品牌上下文：与分发/监测页共享，跨页不丢
  const { brands, brandId: selectedBrand, setCurrentBrand } = useBrandContext()
  const [originalText, setOriginalText] = useState('')
  const [topic, setTopic] = useState('') // 想写什么（可选——获客智能体转型：替代关键词选择）
  const [optimizing, setOptimizing] = useState(false)
  // 傻瓜化：显式"帮我写 / 帮我改"切换（此前靠输入框空不空隐式决定——用户无感知）
  const [mode, setMode] = useState<'write' | 'improve'>('write')
  const [result, setResult] = useState<OptimizedContent | null>(null)
  const [genKeywords, setGenKeywords] = useState<string[]>([])
  const [targetEngine, setTargetEngine] = useState<string>('') // 目标 AI 引擎偏好
  const [format, setFormat] = useState<string>('') // 内容格式（P1-5：文章/点评/小红书/脚本/FAQ/评测）
  const [citationToggles, setCitationToggles] = useState<string[]>([]) // 可引用结构开关（v3 P2：与格式正交，可组合）
  const [useDiagnose, setUseDiagnose] = useState(false) // P5-03 诊断→优化闭环开关
  const [generating, setGenerating] = useState(false)
  const [drafting, setDrafting] = useState(false)
  const [resubmitting, setResubmitting] = useState<string | null>(null) // 收录补提交 loading
  const [preview, setPreview] = useState<OptimizedContent | null>(null) // 全文预览（历史卡片点击）
  const [fullWidth, setFullWidth] = useState(false) // 结果面板全宽模式（长文阅读）

  const { data: keywords = [] } = useQuery({    queryKey: ['geo-keywords', selectedBrand],
    queryFn: () => businessApi.listKeywords(selectedBrand!),
    enabled: !!selectedBrand,
  })
  // 监测数据（共享缓存——仅用于关键词预选排序：最近体检过的词优先）
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch(() => []),
    staleTime: 60_000,
  })
  const orderedKeywords = useMemo(() => {
    const lastByKw = new Map<string, number>()
    for (const r of monitorResults) {
      const t = new Date(r.probed_at).getTime()
      if (t > (lastByKw.get(r.keyword_id) || 0)) lastByKw.set(r.keyword_id, t)
    }
    return [...keywords].sort((a, b) => (lastByKw.get(b.id) || 0) - (lastByKw.get(a.id) || 0))
  }, [keywords, monitorResults])

  // 智能预选：高级选项里的可选关键词种子（留空也可生成）
  const lastPreselectBrand = useRef<string | null>(null)
  useEffect(() => {
    if (!selectedBrand || orderedKeywords.length === 0) return
    if (lastPreselectBrand.current === selectedBrand) return
    lastPreselectBrand.current = selectedBrand
    if (genKeywords.length === 0) {
      setGenKeywords(orderedKeywords.slice(0, 3).map((k: Keyword) => k.term))
    }
  }, [selectedBrand, orderedKeywords, genKeywords.length])

  const { data: contents = [] } = useQuery({
    queryKey: ['geo-contents', selectedBrand],
    queryFn: () => businessApi.listContents(selectedBrand!),
    enabled: !!selectedBrand,
  })

  const { data: knowledgePack } = useQuery({
    queryKey: ['brand-knowledge', selectedBrand],
    queryFn: () => businessApi.listBrandKnowledge(selectedBrand!),
    enabled: !!selectedBrand,
    staleTime: 60_000,
  })
  const knowledgeCount = knowledgePack?.total ?? knowledgePack?.materials?.length ?? 0

  // P5-02 内容引用统计：每篇被 AI 回答引用几次（归因细化到篇）
  const { data: citations = {} } = useQuery({
    queryKey: ['geo-citations', selectedBrand],
    queryFn: () => businessApi.getContentCitations(selectedBrand!).catch(() => ({}) as Record<string, number>),
    enabled: !!selectedBrand,
  })

  const seedKeyword = () =>
    genKeywords[0] || topic.trim() || brands.find((b: Brand) => b.id === selectedBrand)?.name || ''

  // 内容优化（有原始内容时调用）
  const handleOptimize = async () => {
    const kw = seedKeyword()
    if (!selectedBrand || !originalText.trim() || !kw) {
      toast.warn('请选择品牌并填写选题或关键词，再贴入原始内容')
      return
    }
    setOptimizing(true)
    setResult(null)
    try {
      const res = await businessApi.optimizeContent({
        brand_id: selectedBrand,
        keyword: kw,
        original_text: originalText,
        target_engine: targetEngine || undefined,
        format: format || undefined,
      })
      setResult(res)
      toast.ok('优化完成')
      queryClient.invalidateQueries({ queryKey: ['geo-contents', selectedBrand] })
    } catch {
    } finally {
      setOptimizing(false)
    }
  }

  // 从零生成内容（非流式：走结构化 JSON 输出，标题/正文零解析成本）
  const handleGenerate = async () => {
    if (!selectedBrand) {
      toast.warn('请选择品牌')
      return
    }
    setGenerating(true)
    setResult(null)

    try {
      const brandInfo = brands.find((b: Brand) => b.id === selectedBrand)
      const res = await businessApi.generateContent(selectedBrand, {
        topic: topic || undefined,
        keywords: genKeywords.length > 0 ? genKeywords : undefined,
        brand_info: brandInfo ? `${brandInfo.name}：${brandInfo.positioning || ''}` : '',
        target_engine: targetEngine || undefined,
        format: format || undefined,
        citation_toggles: citationToggles.length > 0 ? citationToggles : undefined,
        use_diagnose: useDiagnose,
      })
      setResult(res)
      const modeLabel = topic ? `（围绕“${topic.slice(0, 10)}”）` : ''
      const scoreLabel = res.score?.total ? `，AI 推荐度 ${res.score.total.toFixed(0)}` : ''
      toast.ok(`内容已生成${modeLabel}${scoreLabel}${useDiagnose ? '（已按诊断优化）' : ''}`, 'content-gen')
      const dups = (res as OptimizedContent & { duplicate_warnings?: string[] }).duplicate_warnings
      if (dups?.length) {
        toast.warn(dups[0], 6)
      }
      queryClient.invalidateQueries({ queryKey: ['geo-contents', selectedBrand] })
    } catch { /* 拦截器已提示 */ } finally {
      setGenerating(false)
    }
  }

  // 内容状态流转：发布到公开站 / 下线
  const handleSetStatus = async (c: OptimizedContent, status: 'draft' | 'published') => {
    try {
      // 服务端在内容视图上附加 index_submitted / publish_warnings——
      // 发布是否提交了收录、低分未提交等原因随响应下发（黑洞修复）
      const res = await businessApi.setContentStatus(selectedBrand!, c.id, status) as
        OptimizedContent & { index_submitted?: boolean; publish_warnings?: string[] }
      if (status === 'published') {
        const warnings = res?.publish_warnings || []
        if (warnings.length > 0) {
          toast.warn(`「${c.title || c.id}」${warnings[0]}`, 7)
        } else {
          toast.ok(`「${c.title || '内容'}」已发布到公开站`, 'content-pub', 4)
        }
      } else {
        toast.ok('已下线')
      }
      queryClient.invalidateQueries({ queryKey: ['geo-contents', selectedBrand] })
      if (result?.id === c.id) setResult({ ...result, status })
    } catch { /* 拦截器已提示 */ }
  }

  const score = result?.score

  // AI 生成原始素材（填入编辑区，用户可编辑后再优化）
  const handleGenerateDraft = async () => {
    if (!selectedBrand) {
      toast.warn('请先选择品牌')
      return
    }
    setDrafting(true)
    try {
      const brandInfo = brands.find((b: Brand) => b.id === selectedBrand)
      const res = await businessApi.generateContent(selectedBrand, {
        topic: topic || undefined,
        keywords: genKeywords.length > 0 ? genKeywords.slice(0, 1) : undefined,
        brand_info: brandInfo ? `${brandInfo.name}：${brandInfo.positioning || ''}` : '',
        target_engine: targetEngine || undefined,
        format: format || undefined,
      })
      setOriginalText(res.optimized_text || '')
      toast.ok('素材已生成，你可以编辑后点击优化')
    } catch { /* 拦截器已提示 */ } finally {
      setDrafting(false)
    }
  }

  const busy = optimizing || generating

  // 统一操作：按显式模式决定（写=AI 原创；改=优化贴入的内容）
  const handleAction = () => {
    if (mode === 'improve') {
      handleOptimize()
    } else {
      handleGenerate()
    }
  }

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* ① Hero 区（嵌入内容中心时隐藏——父层已有标题） */}
        {!embedded && (
          <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap' }}>
            <div>
              <h1>内容生成</h1>
              <p>AI 帮你写文章</p>
            </div>
            <Button type="default" icon={<ThunderboltOutlined />} onClick={() => navigate('/m/compose/tools?tab=media')}>
              多媒体创作
            </Button>
          </div>
        )}

        {/* ② 品牌上下文条 */}
        <Card className="wr-glass-card" styles={{ body: { padding: '16px 20px' } }} style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, flex: '1 1 280px' }}>
              <Text strong style={{ whiteSpace: 'nowrap' }}>当前品牌</Text>
              <Select
                style={{ maxWidth: 320, minWidth: 200, flex: 1 }}
                placeholder="选择要创作/优化的品牌"
                value={selectedBrand}
                onChange={(v) => { setCurrentBrand(v); setGenKeywords([]) }}
                options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
              />
            </div>
            {selectedBrand && (
              <Space size={24}>
                <ContextStat icon={<DatabaseOutlined />} value={knowledgeCount} label="知识库" />
                <ContextStat icon={<FileTextOutlined />} value={contents.length} label="历史内容" />
              </Space>
            )}
          </div>
        </Card>

        {/* ③ 主工作区：左输入 / 右结果；全宽模式=上下分栏（输入在上保留编辑，长文阅读不拥挤） */}
        <Row gutter={16}>
          {/* 输入面板（全宽时占整行，置顶保留编辑能力——反模式修复 P0-4-1） */}
          <Col xs={24} lg={fullWidth ? 24 : 12}>
            <Card styles={{ body: { padding: 24 } }}>
              <div className="wr-stagger">
                {/* 想写什么（可选——留空 = AI 从知识库+品牌资料全自动提炼；获客智能体转型：关键词管理界面已移除） */}
                <div style={{ marginBottom: 20 }}>
                  <Text strong style={{ display: 'block', marginBottom: 8 }}>想写什么？（可选）</Text>
                  <Input
                    placeholder="如：介绍一下我们的招牌菜和开业优惠"
                    value={topic}
                    onChange={(e) => setTopic(e.target.value)}
                    maxLength={200}
                  />
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                    留空 = AI 根据品牌资料和知识库自动提炼主题写一篇
                  </Text>
                </div>

                {/* 傻瓜化：显式"帮我写 / 帮我改"（此前靠输入框空不空隐式决定，用户无感知） */}
                <div style={{ marginBottom: 20 }}>
                  <Segmented
                    block
                    value={mode}
                    onChange={(v) => setMode(v as 'write' | 'improve')}
                    options={[
                      { value: 'write', label: '帮我写（AI 原创一篇）' },
                      { value: 'improve', label: '帮我改（优化已有内容）' },
                    ]}
                  />
                </div>

                {/* 想写什么类型（场景化命名——让用户选"要干嘛"，不是选"格式参数"） */}
                <div style={{ marginBottom: 20 }}>
                  <Text strong style={{ display: 'block', marginBottom: 8 }}>想写什么内容？</Text>
                  <Select
                    style={{ width: '100%' }}
                    value={format || undefined}
                    onChange={(v) => setFormat(v || '')}
                    options={FORMAT_OPTIONS}
                    allowClear
                    placeholder="智能推荐（默认写一篇网站文章）"
                  />
                </div>

                {/* 高级选项（默认折叠——90% 用户不需要动，推荐默认值已给足） */}
                <Collapse
                  ghost
                  size="small"
                  style={{ marginBottom: 20 }}
                  items={[{
                    key: 'adv',
                    label: <span style={{ fontSize: 13 }}>高级选项（一般不用改）</span>,
                    children: (<>
                      <div style={{ marginBottom: 16 }}>
                        <Text strong style={{ display: 'block', marginBottom: 8 }}>选题种子词（可选）</Text>
                        <Select
                          mode="tags"
                          style={{ width: '100%' }}
                          value={genKeywords}
                          onChange={setGenKeywords}
                          options={orderedKeywords.map((k: Keyword) => ({ value: k.term, label: k.term }))}
                          placeholder="可从历史词选，也可自行输入；留空则按选题/人设写"
                          maxTagCount={3}
                        />
                        <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 4 }}>
                          不再强制依赖关键词库；这里只是可选强化信号
                        </Text>
                      </div>
                      <div style={{ marginBottom: 16 }}>
                        <Text strong style={{ display: 'block', marginBottom: 8 }}>主要想让哪个 AI 引用？</Text>
                        <Select
                          style={{ width: '100%' }}
                          value={targetEngine || undefined}
                          onChange={(v) => setTargetEngine(v || '')}
                          options={ENGINE_OPTIONS}
                          allowClear
                          placeholder="不指定（对所有 AI 通用优化）"
                        />
                      </div>
                      <div style={{ marginBottom: 16 }}>
                        <Text strong style={{ display: 'block', marginBottom: 8 }}>让 AI 更容易摘录引用的写法</Text>
                        <Select
                          mode="multiple"
                          style={{ width: '100%' }}
                          value={citationToggles}
                          onChange={setCitationToggles}
                          options={[
                            { value: 'conclusion-first', label: '结论放开头' },
                            { value: 'standalone-paragraphs', label: '观点分段清晰' },
                            { value: 'data-cited', label: '数据带来源' },
                            { value: 'subheadings', label: '加小标题' },
                          ]}
                          placeholder="选填：让 AI 一眼能摘到重点"
                          maxTagCount={2}
                        />
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Space size={6}>
                          <Text strong>先做一次诊断，按建议写</Text>
                          <Tag color={useDiagnose ? 'gold' : 'default'} style={{ margin: 0, fontSize: 11 }}>
                            {useDiagnose ? '已开启' : '效果更好，消耗更多额度'}
                          </Tag>
                        </Space>
                        <Switch checked={useDiagnose} onChange={setUseDiagnose} />
                      </div>
                    </>),
                  }]}
                />

                {/* 帮我改：贴入原文（仅 improve 模式显示） */}
                {mode === 'improve' && (
                <div style={{ marginBottom: 20 }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                    <Space size={6}>
                      <Text strong>贴入你想优化的内容</Text>
                    </Space>
                    <Space size={8}>
                      <Button
                        size="small"
                        type="link"
                        loading={drafting}
                        disabled={!selectedBrand}
                        onClick={handleGenerateDraft}
                      >
                        让 AI 先写一版
                      </Button>
                      {originalText.trim().length > 0 && (
                        <Button
                          size="small"
                          type="text"
                          icon={<ClearOutlined />}
                          onClick={() => setOriginalText('')}
                        >
                          清空
                        </Button>
                      )}
                    </Space>
                  </div>

                  {/* 模式提示条 */}
                  <div style={{
                    marginBottom: 8, padding: '6px 12px', borderRadius: 8,
                    background: 'var(--wr-primary-bg)',
                    display: 'flex', alignItems: 'center', gap: 6,
                  }}>
                    <EditOutlined style={{ fontSize: 13, color: 'var(--wr-primary)' }} />
                    <Text style={{ fontSize: 12, color: 'var(--wr-primary)' }}>
                      AI 会围绕选题与人设资料改写，让内容更适合发布获客
                    </Text>
                  </div>

                  <TextArea
                    rows={8}
                    placeholder={drafting
                      ? 'AI 正在生成素材...'
                      : '把已有的文章/文案贴到这里（也可以点右上角让 AI 先写一版，再在此基础上改）'}
                    value={originalText}
                    onChange={(e) => setOriginalText(e.target.value)}
                    style={{ fontSize: 14, lineHeight: 1.8, resize: 'vertical' }}
                    showCount
                    maxLength={20000}
                  />
                </div>
                )}

                {/* AI 将参考（获客智能体：透明度提示——让用户知道 AI 不是睛写） */}
                <div style={{ marginBottom: 12, padding: '8px 12px', borderRadius: 8, background: 'var(--wr-bg-elevated)', fontSize: 12 }}>
                  <Text type="secondary">AI 将参考：</Text>
                  <Text strong>{brands.find((b: Brand) => b.id === selectedBrand)?.name || '人设资料'}</Text>
                  <Text type="secondary"> + 知识库 {knowledgeCount} 份素材（自动注入）</Text>
                </div>

                {/* 统一操作按钮 */}
                <Button
                  type="primary"
                  size="large"
                  block
                  loading={busy}
                  disabled={!selectedBrand || (mode === 'improve' && originalText.trim().length === 0)}
                  onClick={handleAction}
                  icon={mode === 'improve' ? <EditOutlined /> : <ThunderboltOutlined />}
                >
                  {busy
                    ? 'AI 创作中...'
                    : mode === 'improve'
                    ? '开始优化'
                    : '帮我写'}
                </Button>
              </div>
            </Card>
          </Col>

          {/* 右：结果面板（AI 推荐度仪表盘；全宽模式占满整行在输入下方）*/}
          <Col xs={24} lg={fullWidth ? 24 : 12}>
            <Card
              title="优化结果"
              extra={
                <Button size="small" type="text" onClick={() => setFullWidth(!fullWidth)}>
                  {fullWidth ? '分栏模式' : '全宽阅读'}
                </Button>
              }
              styles={{ body: { padding: 24 } }}
              style={{ minHeight: '100%' }}
            >              {busy ? (
                <div style={{ textAlign: 'center', padding: '60px 20px' }}>
                  <Spin size="large" />
                  <Paragraph type="secondary" style={{ marginTop: 16, marginBottom: 0 }}>
                    {generating ? 'AI 正在创作内容，文章会逐字显示...' : 'AI 正在优化内容并评分...'}
                  </Paragraph>
                </div>
              ) : result ? (
                <>
                  {/* AI 推荐度仪表盘 */}
                  {score && score.total > 0 && (
                    <div style={{ marginBottom: 20 }}>
                      <Row gutter={16} align="middle">
                        <Col flex="auto">
                          <ScoreRing total={score.total} />
                        </Col>
                        <Col flex="auto">
                          <Space direction="vertical" size={10} style={{ width: '100%' }}>
                            {DIMENSIONS.map((d) => (
                              <ScoreBar key={d.key} label={d.label} value={score[d.key]} tip={d.tip} />
                            ))}
                          </Space>
                        </Col>
                      </Row>
                    </div>
                  )}

                  {/* 可引用度提示（v3 P2：结构信号检测——引导开启引用友好开关） */}
                  {(() => {
                    const cit = citabilityOf(result.optimized_text || result.original_text || '')
                    return (
                      <div style={{ marginBottom: 20, padding: '12px 16px', borderRadius: 12, background: 'var(--wr-bg-elevated)' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                          <Tooltip title="AI 引擎摘录友好的素材结构：结论前置 / 小标题分段 / 数据标注来源——三个可机读信号的启发式检测（非精确评分）">
                            <Text strong style={{ fontSize: 13 }}>可引用度</Text>
                          </Tooltip>
                          <Tag color={cit.score >= 67 ? 'success' : cit.score >= 34 ? 'warning' : 'default'} style={{ margin: 0 }}>
                            {cit.score}%（{3 - cit.hints.length}/3 结构信号）
                          </Tag>
                        </div>
                        <Space size={6} wrap style={{ marginBottom: cit.hints.length > 0 ? 8 : 0 }}>
                          <Tag style={{ margin: 0, fontSize: 11 }} color={cit.conclusionFirst ? 'success' : 'default'}>{cit.conclusionFirst ? '✓' : '✗'} 结论前置</Tag>
                          <Tag style={{ margin: 0, fontSize: 11 }} color={cit.hasSubheadings ? 'success' : 'default'}>{cit.hasSubheadings ? '✓' : '✗'} 小标题分段</Tag>
                          <Tag style={{ margin: 0, fontSize: 11 }} color={cit.dataCited ? 'success' : 'default'}>{cit.dataCited ? '✓' : '✗'} 数据标注来源</Tag>
                        </Space>
                        {cit.hints.length > 0 && (
                          <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
                            提升提示：{cit.hints.join('；')}——可在左侧开启「可引用结构」开关重新生成
                          </Text>
                        )}
                      </div>
                    )
                  })()}

                  {/* 前后对比反馈（仅优化模式：后端返回 score_before + recommendations） */}
                  {result.score_before && (
                    <div style={{ marginBottom: 20 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
                        <Text strong style={{ fontSize: 14 }}>优化前后对比</Text>
                        <Tag color="blue" style={{ margin: 0, fontSize: 11 }}>
                          {result.score_before.total?.toFixed(0)} → {score?.total?.toFixed(0)}
                        </Tag>
                        <Tooltip title="优化前是免费规则快筛分（正则/关键词统计），优化后是 AI 深度评审分——两套口径不同，看提升方向即可，不必比较绝对值">
                          <Text type="secondary" style={{ fontSize: 11 }}>数据口径说明</Text>
                        </Tooltip>
                      </div>
                      <Space direction="vertical" size={8} style={{ width: '100%' }}>
                        {DIMENSIONS.map((d) => {
                          const before = result.score_before?.[d.key] ?? 0
                          const after = score?.[d.key] ?? 0
                          const diff = after - before
                          const diffColor = diff > 5 ? 'var(--wr-success)' : diff < -5 ? 'var(--wr-danger)' : 'var(--wr-text-muted)'
                          return (
                            <div key={d.key}>
                              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, marginBottom: 4 }}>
                                <Tooltip title={d.tip}><Text type="secondary">{d.label}</Text></Tooltip>
                                <Text style={{ fontSize: 12 }}>
                                  <Text type="secondary" style={{ fontSize: 12 }}>{before.toFixed(0)}</Text>
                                  <Text strong style={{ color: diffColor, fontSize: 12 }}> → {after.toFixed(0)}</Text>
                                </Text>
                              </div>
                              {/* 双条对比：before 灰条 + after 彩条 */}
                              <div style={{ position: 'relative', height: 6, background: 'var(--wr-bg-elevated)', borderRadius: 3, overflow: 'hidden' }}>
                                <div style={{
                                  position: 'absolute', top: 0, left: 0, bottom: 0,
                                  width: `${Math.max(0, Math.min(100, before))}%`,
                                  background: 'var(--wr-text-muted)',
                                  opacity: 0.35,
                                }} />
                                <div style={{
                                  position: 'absolute', top: 0, left: 0, bottom: 0,
                                  width: `${Math.max(0, Math.min(100, after))}%`,
                                  background: scoreColor(after),
                                  borderRadius: 3,
                                  transition: 'width 600ms cubic-bezier(0.2, 0, 0, 1)',
                                }} />
                              </div>
                            </div>
                          )
                        })}
                      </Space>

                      {/* 改进建议 */}
                      {result.recommendations && result.recommendations.length > 0 && (
                        <div style={{ marginTop: 16, padding: '12px 16px', background: 'var(--wr-primary-bg)', borderRadius: 10 }}>
                          <Text strong style={{ fontSize: 13, color: 'var(--wr-primary)' }}>改进建议</Text>
                          <ul style={{ margin: '8px 0 0', paddingLeft: 18, fontSize: 13, lineHeight: 1.8, color: 'var(--wr-text-secondary)' }}>
                            {result.recommendations.map((r, i) => (
                              <li key={i}>{r}</li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </div>
                  )}

                  <Paragraph style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8, margin: 0 }}>
                    {result.optimized_text}
                  </Paragraph>

                  {/* B: 去「社媒分发」发布到社交平台（预选该内容+品牌） */}
                  <div style={{ marginTop: 16 }}>
                    <Button
                      type="primary"
                      icon={<ExportOutlined />}
                      onClick={() => navigate(`/m/distribution?contentId=${result.id}${selectedBrand ? `&brandId=${selectedBrand}` : ''}`)}
                    >
                      去发布中心分发
                    </Button>
                  </div>

                  {/* 公开链接（AI 引擎可爬取的公开文章页——发布为 published 后生效） */}
                  {result.id && (
                    <div style={{ marginTop: 16, padding: '10px 14px', background: 'var(--wr-bg-elevated)', borderRadius: 8, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                      {result.status === 'published' ? (
                        <>
                          <Tag color="success" style={{ margin: 0 }}>已发布</Tag>
                          <Text code style={{ fontSize: 12 }}>/public/articles/{result.id}</Text>
                          <Button size="small" type="link" icon={<ExportOutlined />} href={`/public/articles/${result.id}`} target="_blank" style={{ fontSize: 12 }}>
                            查看公开页
                          </Button>
                          <Button size="small" type="text" danger style={{ fontSize: 12 }} onClick={() => handleSetStatus(result, 'draft')}>
                            下线
                          </Button>
                        </>
                      ) : (
                        <>
                          <Text type="secondary" style={{ fontSize: 12 }}>草稿——发布后 AI 引擎可爬取此内容</Text>
                          <PublishToSiteButton
                            score={result.score?.total}
                            onPublish={() => handleSetStatus(result, 'published')}
                          />
                        </>
                      )}
                    </div>
                  )}
                </>
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="选好人设后点「帮我写」，结果会显示在这里"
                  style={{ padding: 60 }}
                />
              )}
            </Card>
          </Col>
        </Row>

        {/* ④ 历史内容卡片网格 */}
        {selectedBrand && contents.length > 0 && (
          <Card title="历史内容" style={{ marginTop: 16 }}>
            <Row gutter={[16, 16]} className="wr-stagger">
              {contents.map((c: OptimizedContent) => {
                const total = c.score?.total || 0
                return (
                  <Col xs={24} sm={12} lg={8} key={c.id}>
                    <Card
                      size="small"
                      hoverable
                      style={{ height: '100%' }}
                      styles={{ body: { padding: 16, height: '100%', display: 'flex', flexDirection: 'column', gap: 10 } }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Space size={8}>
                          <Tag color={scoreColor(total)} style={{ margin: 0, fontWeight: 600 }}>
                            {total.toFixed(0)}
                          </Tag>
                          <Text type="secondary" style={{ fontSize: 12 }}>{scoreLevel(total)}</Text>
                          {c.status === 'published' ? (
                            <Tag color="success" style={{ margin: 0, fontSize: 11 }}>已发布</Tag>
                          ) : (
                            <Tag style={{ margin: 0, fontSize: 11 }}>草稿</Tag>
                          )}
                          {c.status === 'published' && c.index_status === 'indexed' && (
                            <Tag color="green" style={{ margin: 0, fontSize: 11 }}>已收录</Tag>
                          )}
                          {c.status === 'published' && c.index_status === 'pending' && (
                            <Tag color="warning" style={{ margin: 0, fontSize: 11 }}>待收录</Tag>
                          )}
                          {citations[c.id] > 0 && (
                            <Tooltip title={`AI 回答引用了这篇内容 ${citations[c.id]} 次——内容被 AI 引用的直接效果证据`}>
                              <Tag color="purple" style={{ margin: 0, fontSize: 11 }}>被引用 {citations[c.id]} 次</Tag>
                            </Tooltip>
                          )}
                        </Space>
                        <Text type="secondary" style={{ fontSize: 12 }}>v{c.version}</Text>
                      </div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {new Date(c.created_at).toLocaleString()}
                      </Text>
                      {/* 点击标题看全文（抽屉）——2 行摘要读不了长文章 */}
                      <Text
                        strong
                        ellipsis
                        style={{ fontSize: 13, cursor: 'pointer', color: 'var(--wr-primary)' }}
                        onClick={() => setPreview(c)}
                      >
                        {c.title || '(无标题)'}
                      </Text>
                      <Paragraph
                        ellipsis={{ rows: 2 }}
                        style={{ margin: 0, color: 'var(--wr-text-secondary)', fontSize: 13, lineHeight: 1.6 }}
                      >
                        {c.optimized_text}
                      </Paragraph>
                      <div style={{ display: 'flex', gap: 4, marginTop: 'auto', paddingTop: 4, flexWrap: 'wrap' }}>
                        <Button
                          size="small"
                          type="link"
                          icon={<ExportOutlined />}
                          style={{ fontSize: 12 }}
                          onClick={() => navigate(`/m/distribution?contentId=${c.id}&brandId=${selectedBrand}`)}
                        >
                          去发布中心
                        </Button>
                        {c.status === 'published' ? (
                          <>
                            <Button size="small" type="link" icon={<ExportOutlined />} href={`/public/articles/${c.id}`} target="_blank" style={{ fontSize: 12 }}>
                              公开页
                            </Button>
                            <Button size="small" type="link" style={{ fontSize: 12 }} loading={resubmitting === c.id}
                              onClick={async () => {
                                setResubmitting(c.id)
                                try {
                                  await businessApi.resubmitIndex(selectedBrand!, c.id)
                                  toast.ok('已重新提交收录通知', 'content-index')
                                  queryClient.invalidateQueries({ queryKey: ['geo-contents'] })
                                } catch { /* 拦截器已提示 */ } finally { setResubmitting(null) }
                              }}>
                              补提交收录
                            </Button>
                            <Button size="small" type="text" danger style={{ fontSize: 12 }} onClick={() => handleSetStatus(c, 'draft')}>
                              下线
                            </Button>
                          </>
                        ) : (
                          <PublishToSiteButton
                            score={c.score?.total}
                            onPublish={() => handleSetStatus(c, 'published')}
                          />
                        )}
                      </div>
                    </Card>
                  </Col>
                )
              })}
            </Row>
          </Card>
        )}

        {/* 历史内容全文预览（与 admin 内容管理共用组件） */}
        <ContentPreviewDrawer content={preview} onClose={() => setPreview(null)} />
      </div>
    </div>
  )
}

/* ===== 纯展示子组件（谦卑对象：只负责渲染，无业务逻辑）===== */

// 上下文条小统计
function ContextStat({ icon, value, label }: { icon: React.ReactNode; value: number; label: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <span style={{ color: 'var(--wr-text-muted)', fontSize: 16 }}>{icon}</span>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
        <Text strong style={{ fontSize: 18 }}>{value}</Text>
        <Text type="secondary" style={{ fontSize: 12 }}>{label}</Text>
      </div>
    </div>
  )
}

// AI 推荐度进度环（自绘 SVG，颜色随分数变化）
function ScoreRing({ total }: { total: number }) {
  const color = scoreColor(total)
  const radius = 52
  const circ = 2 * Math.PI * radius
  const pct = Math.max(0, Math.min(100, total)) / 100
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
      <div style={{ position: 'relative', width: 128, height: 128 }}>
        <svg width="128" height="128" viewBox="0 0 128 128" style={{ transform: 'rotate(-90deg)' }}>
          <circle cx="64" cy="64" r={radius} fill="none" stroke="var(--wr-bg-elevated)" strokeWidth="10" />
          <circle
            cx="64" cy="64" r={radius} fill="none" stroke={color} strokeWidth="10" strokeLinecap="round"
            strokeDasharray={circ}
            strokeDashoffset={circ * (1 - pct)}
            style={{ transition: 'stroke-dashoffset 600ms cubic-bezier(0.2, 0, 0, 1), stroke 300ms ease' }}
          />
        </svg>
        <div style={{
          position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center',
        }}>
                          <span className="wr-metric-value" style={{ fontSize: 28, color }}>{total.toFixed(0)}</span>
          <Text type="secondary" style={{ fontSize: 12 }}>AI 推荐度</Text>
        </div>
      </div>
      <Tooltip title="AI 深度评审打分（0-100，五维平均）——衡量这篇内容被 AI 引用的难易程度">
        <Tag color={color} style={{ fontWeight: 600 }}>{scoreLevel(total)}</Tag>
      </Tooltip>
      <Text type="secondary" style={{ fontSize: 11 }}>AI 深度评审</Text>
    </div>
  )
}

// 单维度评分横条（label 带"怎么算的"溯源 tooltip）
function ScoreBar({ label, value, tip }: { label: string; value: number; tip?: string }) {
  const color = scoreColor(value)
  const labelEl = <Text type="secondary">{label}</Text>
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, marginBottom: 4 }}>
        {tip ? <Tooltip title={tip}>{labelEl}</Tooltip> : labelEl}
        <Text strong style={{ color }}>{value.toFixed(0)}</Text>
      </div>
      <div style={{ height: 6, background: 'var(--wr-bg-elevated)', borderRadius: 3, overflow: 'hidden' }}>
        <div style={{
          height: '100%',
          width: `${Math.max(0, Math.min(100, value))}%`,
          background: color,
          borderRadius: 3,
          transition: 'width 600ms cubic-bezier(0.2, 0, 0, 1), background 300ms ease',
        }} />
      </div>
    </div>
  )
}

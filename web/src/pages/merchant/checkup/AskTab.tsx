import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Card, Checkbox, Divider, Empty, Input, Segmented, Select, Space, Spin, Tag, Tooltip, Typography } from 'antd'
import { ThunderboltOutlined, CheckCircleOutlined, CloseCircleOutlined, RobotOutlined, ReloadOutlined } from '@ant-design/icons'
import { useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import { useBrandContext } from '../../../hooks/useBrands'
import { engineLabel, MENTION_RATE_TIP } from '../../../utils/geoTerms'
import type { Keyword, MonitoringResult, EngineOption } from '../../../types/api'
import { message } from '../../../utils/antdApp'

const { Text, Paragraph } = Typography

const SAMPLE_SIZE = 3 // 每个引擎问 3 次取平均（服务端固定口径）
const COST_PER_CHECK = SAMPLE_SIZE * 2 // 每次 = 提问 + 解析两次 LLM 调用（配额计数口径）
const BATCH_WINDOW_MS = 30 * 60 * 1000 // 同一批体检的时间窗口（30 分钟内视为一次）

function sentimentLabel(s?: string) {
  if (s === 'positive') return { text: '夸你', color: 'success' as const }
  if (s === 'negative') return { text: '批评你', color: 'error' as const }
  return { text: '中性', color: 'default' as const }
}

/** 单引擎结果行（一问多答卡内的一行：✓/✗ + 结论徽章 + AI 原话折叠）——问答历史共用 */
export function EngineRow({ r }: { r: MonitoringResult }) {
  const [openRaw, setOpenRaw] = useState(false)
  const mentioned = (r.mention_rate || 0) > 0
  const sent = sentimentLabel(r.sentiment)
  const topComps = Object.entries(r.competitor_rates || {})
    .sort((a, b) => (b[1] as number) - (a[1] as number))
    .slice(0, 2)
    .map(([n]) => n)
  return (
    <div style={{ padding: '10px 14px', borderRadius: 10, background: 'var(--wr-bg-elevated)', marginBottom: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
        {mentioned ? (
          <CheckCircleOutlined style={{ color: 'var(--wr-success)', fontSize: 17 }} />
        ) : (
          <CloseCircleOutlined style={{ color: 'var(--wr-text-muted)', fontSize: 17 }} />
        )}
        <Text strong style={{ fontSize: 14, minWidth: 76 }}>{engineLabel(r.engine_name)}</Text>
        {mentioned ? (
          <>
            <Tooltip title={MENTION_RATE_TIP}>
              <Tag color="success" style={{ margin: 0 }}>提到你 {Math.round((r.mention_rate || 0) * 100)}%</Tag>
            </Tooltip>
            {(r.avg_position || 0) > 0 && (
              <Tooltip title={`AI 列出的推荐名单里你排第 ${r.avg_position} 位——数字越小越靠前`}>
                <Tag color="purple" style={{ margin: 0 }}>第 {r.avg_position} 位推荐</Tag>
              </Tooltip>
            )}
            <Tag color={sent.color} style={{ margin: 0 }}>{sent.text}</Tag>
          </>
        ) : (
          <Text type="secondary" style={{ fontSize: 13 }}>
            没提你{topComps.length > 0 ? `——AI 推荐了「${topComps.join('、')}」` : ''}
          </Text>
        )}
        {(r.self_source_count || 0) > 0 && (
          <Tag color="green" style={{ margin: 0, fontSize: 11 }}>引用了你的内容 {r.self_source_count} 次</Tag>
        )}
        {r.semantic_degraded && (
          <Tooltip title="部分回答解析降级（结果可能偏差，建议过几天再测一次）">
            <Tag color="warning" style={{ margin: 0, fontSize: 11 }}>数据积累中</Tag>
          </Tooltip>
        )}
        <a style={{ marginLeft: 'auto', fontSize: 12 }} onClick={() => setOpenRaw(!openRaw)}>
          {openRaw ? '收起原话' : '看 AI 原话'}
        </a>
      </div>
      {openRaw && (
        <div style={{ marginTop: 10, maxHeight: 260, overflow: 'auto', padding: 12, background: 'var(--wr-bg-surface)', borderRadius: 8 }}>
          <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12, lineHeight: 1.7, color: 'var(--wr-text-secondary)', margin: 0 }}>
            {r.raw_sample || '本次回答无原文记录'}
          </pre>
        </div>
      )}
    </div>
  )
}

/** 一问多答卡：一个问题 + 各引擎结果并排 + 人话汇总 + 结果驱动的行动条 */
function AskResultCard({ question, results, onRetest, retestText }: { question: string; results: MonitoringResult[]; onRetest?: () => void; retestText?: string }) {
  const navigate = useNavigate()
  const mentioned = results.filter((r) => (r.mention_rate || 0) > 0).length
  // 行动条跟着结论走（傻瓜化 Q4：测完没去路 → 结果就是最好的行动依据）
  const hasFirst = results.some((r) => (r.mention_rate || 0) > 0 && (r.avg_position || 9) === 1)
  const act = mentioned === 0
    ? { text: 'AI 还不认识你——发布内容让它有东西可引', label: '去造内容' }
    : (mentioned === results.length && hasFirst)
      ? { text: '表现很好——把这个优势写进更多内容，稳固排名', label: '去写一篇' }
      : { text: '让更多 AI 推荐你：针对这个问题写一篇内容', label: '去写一篇' }
  return (
    <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
        <Space size={10} wrap>
          <Text strong style={{ fontSize: 15 }}>“{question}”</Text>
          <Tag color={mentioned > 0 ? 'success' : 'warning'} style={{ margin: 0 }}>
            {results.length} 个 AI 问了，{mentioned} 个提到了你
          </Tag>
        </Space>
        {onRetest && (
          <Button size="small" icon={<ReloadOutlined />} onClick={onRetest}>{retestText || '过几天再测这个问题'}</Button>
        )}
      </div>
      {results.map((r, i) => <EngineRow key={r.id || i} r={r} />)}
      <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
        每个引擎真实提问 {SAMPLE_SIZE} 次取平均 · 结果已存入效果记录，报告页自动汇总
      </Text>
      {/* 行动条：写文章页的关键词预选按最近体检排序——刚测的这个问题会自动排在第一位被选中 */}
      <div style={{ marginTop: 12, padding: '10px 14px', borderRadius: 10, background: 'var(--wr-primary-bg)', display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
        <ThunderboltOutlined style={{ color: 'var(--wr-primary)' }} />
        <Text style={{ fontSize: 13, flex: 1, minWidth: 200 }}>{act.text}</Text>
        <Button size="small" type="primary" onClick={() => navigate('/m/compose')}>{act.label}</Button>
      </div>
    </Card>
  )
}

/**
 * 测一测（AI 效果中心主交互）：
 * 我来问（默认）/ AI 帮我出题 二选一 → 勾选引擎（默认全选）→ 一问多答统一结果。
 * 品牌跟随全局上下文；支持 ?q= 预填（问答历史的"再测一次"入口）；
 * 结果服务端留痕——回显上次结果也从服务端取（换设备也在）。
 */
export default function AskTab({
  keywords,
  engines,
  monitorResults,
}: {
  keywords: Keyword[]
  engines: EngineOption[]
  monitorResults: MonitoringResult[]
}) {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  // 品牌跟随全局上下文（工作台品牌卡/品牌档案选中的品牌——全站不错位）
  const { brands, brandId, setCurrentBrand } = useBrandContext()
  const [mode, setMode] = useState<'manual' | 'auto'>('manual')
  const [questionText, setQuestionText] = useState('')
  const [pickedKwId, setPickedKwId] = useState<string>()
  const [selectedEngines, setSelectedEngines] = useState<string[]>([])
  const [running, setRunning] = useState(false)
  // AI 出题
  const [generating, setGenerating] = useState(false)
  const [candidates, setCandidates] = useState<string[]>([])
  const [checkedQs, setCheckedQs] = useState<string[]>([])
  // 结果（按问题分组）
  const [runs, setRuns] = useState<{ question: string; results: MonitoringResult[] }[]>([])

  const brandKeywords = useMemo(() => keywords.filter((k) => k.brand_id === brandId), [keywords, brandId])

  // 引擎 chips 默认全选（引擎名单异步到达时初始化一次）
  useEffect(() => {
    if (engines.length > 0 && selectedEngines.length === 0) {
      setSelectedEngines(engines.map((e) => e.name))
    }
  }, [engines]) // eslint-disable-line react-hooks/exhaustive-deps

  // ?q= 预填（问答历史"再测一次"入口）——消费后清除防重复触发
  useEffect(() => {
    const q = searchParams.get('q')
    if (q) {
      setMode('manual')
      setQuestionText(q)
      setPickedKwId(undefined)
      setRuns([])
      setSearchParams({ tab: 'ask' }, { replace: true })
    }
  }, [searchParams]) // eslint-disable-line react-hooks/exhaustive-deps

  // 服务端回显：本次会话未查询时，展示该品牌上次体检的结果（留痕在服务端，换设备也在）
  const lastRunView = useMemo(() => {
    if (runs.length > 0 || !brandId) return null
    const mine = monitorResults.filter((r) => r.brand_id === brandId)
    if (mine.length === 0) return null
    const t = (r: MonitoringResult) => new Date(r.probed_at).getTime()
    let latest = mine[0]
    for (const r of mine) {
      if (t(r) > t(latest)) latest = r
    }
    const batch = mine.filter((r) => r.keyword_id === latest.keyword_id && Math.abs(t(r) - t(latest)) <= BATCH_WINDOW_MS)
    const byEngine = new Map<string, MonitoringResult>()
    for (const r of [...batch].sort((a, b) => t(b) - t(a))) {
      const k = r.engine_name || 'default'
      if (!byEngine.has(k)) byEngine.set(k, r)
    }
    const term = keywords.find((k) => k.id === latest.keyword_id)?.term
    if (!term || byEngine.size === 0) return null
    return { question: term, results: Array.from(byEngine.values()) }
  }, [monitorResults, brandId, keywords, runs.length])

  // 问题库匹配（输入的问题已存在时不重复建）
  const matchedKw = useMemo(() => {
    const q = questionText.trim().toLowerCase()
    if (!q) return undefined
    return brandKeywords.find((k) => k.term.toLowerCase().includes(q) || q.includes(k.term.toLowerCase()))
  }, [brandKeywords, questionText])

  // 目标问题数 → 预计额度消耗
  const targetQCount = mode === 'manual' ? 1 : checkedQs.length
  const estCost = targetQCount * Math.max(1, selectedEngines.length) * COST_PER_CHECK

  // 解析/创建问题关键词（问题即关键词——入库后进效果记录与报告口径）
  const resolveKwId = async (term: string): Promise<[string | undefined, boolean]> => {
    if (pickedKwId && term === brandKeywords.find((k) => k.id === pickedKwId)?.term) return [pickedKwId, false]
    const existed = brandKeywords.find((k) => k.term === term)
    if (existed) return [existed.id, false]
    if (!brandId) return [undefined, false]
    try {
      const created = await businessApi.addKeyword(brandId, { term, intent: 'informational' })
      return [created?.id, true]
    } catch { return [undefined, false] }
  }

  const invalidate = (keywordCreated: boolean) => {
    void queryClient.invalidateQueries({ queryKey: ['geo-monitor-results'] })
    void queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
    void queryClient.invalidateQueries({ queryKey: ['geo-advice-last'] })
    void queryClient.invalidateQueries({ queryKey: ['geo-health-report'] })
    if (keywordCreated) void queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] })
  }

  // 对一个问题发起多引擎体检
  const askOne = async (kwId: string): Promise<MonitoringResult[]> => {
    const res = await businessApi.monitorMulti({
      keyword_id: kwId,
      engine_names: selectedEngines,
      sample_size: SAMPLE_SIZE,
    })
    return Array.isArray(res) ? res : [res]
  }

  const run = async (overrideQuestion?: string) => {
    if (!brandId) { message.warning('请先选择品牌'); return }
    if (selectedEngines.length === 0) { message.warning('请至少选择一个 AI'); return }

    // 组装本轮要问的问题列表
    let questions: string[]
    if (overrideQuestion) {
      questions = [overrideQuestion]
    } else if (mode === 'manual') {
      const q = questionText.trim() || brandKeywords.find((k) => k.id === pickedKwId)?.term || ''
      if (!q) { message.warning('输入一个问题，或从问题库选一个'); return }
      questions = [q]
    } else {
      if (checkedQs.length === 0) { message.warning('先让 AI 出题并勾选要测的问题'); return }
      questions = checkedQs
    }

    setRunning(true)
    setRuns([])
    let keywordCreated = false
    try {
      const nextRuns: { question: string; results: MonitoringResult[] }[] = []
      for (const q of questions) {
        const [kwId, created] = await resolveKwId(q)
        if (created) keywordCreated = true
        if (!kwId) continue
        const results = await askOne(kwId)
        nextRuns.push({ question: q, results })
        setRuns([...nextRuns]) // 逐问渐进展示
      }
      invalidate(keywordCreated)
    } catch { /* 拦截器已提示 */ } finally {
      setRunning(false)
    }
  }

  // AI 帮我出题（问题词蒸馏——先勾选确认再测，不浪费额度、不问歪）
  const generateQuestions = async () => {
    if (!brandId) { message.warning('请先选择品牌'); return }
    setGenerating(true)
    try {
      const res = await businessApi.distillKeywords({ source: 'questions', brand_id: brandId } as never)
      const kws = (res.keywords || []).slice(0, 10)
      setCandidates(kws)
      setCheckedQs(kws)
    } catch { /* 拦截器已提示 */ } finally {
      setGenerating(false)
    }
  }

  return (
    <div>
      {/* 出题卡（傻瓜化主操作：一个输入区 + 引擎多选 + 预计消耗 → 测一测） */}
      <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
        <Segmented
          block
          value={mode}
          onChange={(v) => { setMode(v as 'manual' | 'auto'); setRuns([]) }}
          options={[
            { value: 'manual', label: '我来问' },
            { value: 'auto', label: 'AI 帮我出题' },
          ]}
          style={{ marginBottom: 16 }}
        />

        <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: 12, flexWrap: 'wrap' }}>
          <Text strong>品牌</Text>
          <Select
            style={{ minWidth: 180 }}
            placeholder="选择品牌"
            value={brandId}
            onChange={(v) => { setCurrentBrand(v); setPickedKwId(undefined); setRuns([]); setCandidates([]) }}
            options={brands.map((b) => ({ value: b.id, label: b.name }))}
          />
        </div>

        {mode === 'manual' ? (
          <>
            <Input.TextArea
              rows={2}
              placeholder={'像顾客一样问一句，如：成都哪家装修公司口碑好？'}
              value={questionText}
              onChange={(e) => { setQuestionText(e.target.value); setPickedKwId(undefined) }}
              maxLength={60}
            />
            {questionText.trim() && matchedKw && (
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                问题库里已有这个问题——测完会和历史结果对比
              </Text>
            )}
            {questionText.trim() && !matchedKw && brandId && (
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                新问题——测完会自动存入问题库，以后可以随时复测看变化
              </Text>
            )}

          </>
        ) : (
          <>
            <Paragraph type="secondary" style={{ fontSize: 13, marginBottom: 10 }}>
              AI 根据你的品牌资料生成顾客真实会问的问题——<Text strong>先勾选确认，再开始测</Text>（不浪费额度、不问歪）。
            </Paragraph>
            <Button icon={<RobotOutlined />} loading={generating} onClick={generateQuestions} style={{ marginBottom: 12 }}>
              {candidates.length > 0 ? '重新出题' : '让 AI 出 10 个问题'}
            </Button>
            {generating && <div style={{ textAlign: 'center', padding: 24 }}><Spin /></div>}
            {!generating && candidates.length > 0 && (
              <>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
                  <Text strong style={{ fontSize: 13 }}>勾选要测的问题（已选 {checkedQs.length}/{candidates.length}）</Text>
                  <Space size={4}>
                    <Button size="small" type="link" onClick={() => setCheckedQs(candidates)}>全选</Button>
                    <Button size="small" type="link" onClick={() => setCheckedQs([])}>清空</Button>
                  </Space>
                </div>
                <Checkbox.Group value={checkedQs} onChange={(v) => setCheckedQs(v as string[])} style={{ width: '100%' }}>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: 6, maxHeight: 220, overflowY: 'auto' }}>
                    {candidates.map((q) => (
                      <Checkbox key={q} value={q} style={{ marginLeft: 0 }}>
                        <Text style={{ fontSize: 13 }}>{q}</Text>
                      </Checkbox>
                    ))}
                  </div>
                </Checkbox.Group>
              </>
            )}
          </>
        )}

        {/* 引擎多选（默认全选） + 预计消耗 */}
        <div style={{ marginTop: 16, paddingTop: 12, borderTop: '1px solid var(--wr-border)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <Text strong style={{ fontSize: 13 }}>问哪些 AI：</Text>
            {engines.length === 0 ? (
              <Text type="secondary" style={{ fontSize: 12 }}>暂无可用 AI 引擎——请联系管理员配置</Text>
            ) : engines.map((e) => {
              const checked = selectedEngines.includes(e.name)
              return (
                <Tag.CheckableTag
                  key={e.name}
                  checked={checked}
                  onChange={() => setSelectedEngines(checked ? selectedEngines.filter((n) => n !== e.name) : [...selectedEngines, e.name])}
                  style={{ fontSize: 13, padding: '3px 12px', borderRadius: 16, border: checked ? '1px solid var(--wr-primary)' : '1px solid var(--wr-border)' }}
                >
                  {engineLabel(e.name)}
                </Tag.CheckableTag>
              )
            })}
            {engines.length > 1 && (
              <Button size="small" type="link" onClick={() => setSelectedEngines(engines.map((e) => e.name))}>全选</Button>
            )}
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 14, flexWrap: 'wrap', gap: 10 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {targetQCount > 0
                ? <>将问 {targetQCount} 个问题 × {Math.max(1, selectedEngines.length)} 个 AI，预计消耗约 <Text strong>{estCost}</Text> 次AI 查询额度（每个 AI 问 {SAMPLE_SIZE} 次取平均）</>
                : '选择问题后开始'}
            </Text>
            <Button
              type="primary" size="large" icon={<ThunderboltOutlined />}
              loading={running}
              disabled={!brandId || selectedEngines.length === 0 || (mode === 'auto' ? checkedQs.length === 0 : (!questionText.trim() && !pickedKwId))}
              onClick={() => run()}
              style={{ minWidth: 160 }}
            >
              {running ? 'AI 回答中（约 10-30 秒）...' : '测一测'}
            </Button>
          </div>
        </div>
      </Card>

      {/* 结果：一问多答（每问题一张统一卡） */}
      {running && runs.length === 0 && (
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
          <Paragraph type="secondary" style={{ marginTop: 12 }}>
            正在向 {selectedEngines.length} 个 AI 真实提问“{mode === 'manual' ? (questionText.trim() || '所选问题') : '勾选的问题'}”...
          </Paragraph>
        </div>
      )}
      {runs.map((runItem, i) => (
        <AskResultCard
          key={i}
          question={runItem.question}
          results={runItem.results}
          onRetest={() => { setQuestionText(runItem.question); setMode('manual'); void run(runItem.question) }}
        />
      ))}

      {/* 服务端回显：上次的结果（本次会话未查询时——换设备/换浏览器也在） */}
      {!running && runs.length === 0 && lastRunView && (
        <div>
          <Divider style={{ fontSize: 12, margin: '8px 0 16px' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>上次的结果</Text>
          </Divider>
          <AskResultCard
            question={lastRunView.question}
            results={lastRunView.results}
            retestText="再测一次这个问题"
            onRetest={() => { setQuestionText(lastRunView.question); setMode('manual'); void run(lastRunView.question) }}
          />
        </div>
      )}

      {!running && runs.length === 0 && !lastRunView && (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={null}
          style={{ padding: 48 }}
        >
          <div style={{ textAlign: 'center', maxWidth: 380, margin: '0 auto' }}>
            <Text strong style={{ fontSize: 15 }}>问一句，看 AI 会不会推荐你</Text>
            <div style={{ marginTop: 6 }}>
              <Text type="secondary" style={{ fontSize: 13 }}>
                输入顾客会问的问题（或让 AI 出题），选几个 AI 引擎点「测一测」——结果立刻可见，并自动存入效果报告
              </Text>
            </div>
          </div>
        </Empty>
      )}
    </div>
  )
}

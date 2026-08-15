import { useMemo, useState, useEffect } from 'react'
import { Card, Select, Space, Tag, Typography, Button, Empty, Spin, Alert, Input, message } from 'antd'
import { ThunderboltOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../../api/business'
import { latestMonitor } from '../../../utils/geo'
import type { Brand, Keyword, MonitoringResult, EngineOption } from '../../../types/api'

const { Text, Paragraph } = Typography

// 情感展示
function sentimentTag(s?: string) {
  if (s === 'positive') return <Tag color="success" style={{ margin: 0 }}>正面</Tag>
  if (s === 'negative') return <Tag color="error" style={{ margin: 0 }}>负面</Tag>
  return <Tag style={{ margin: 0 }}>中性</Tag>
}

// 引擎显示名
function engineLabelOf(name?: string) {
  const map: Record<string, string> = {
    default: '默认引擎', chatgpt: 'ChatGPT', kimi: 'Kimi', doubao: '豆包',
    deepseek: 'DeepSeek', qwen: '通义千问', ernie: '文心一言', yuanbao: '腾讯元宝', perplexity: 'Perplexity',
  }
  const key = (name || 'default').toLowerCase()
  return map[key] || name || '默认引擎'
}

// 单条结果卡（速查展示：提及率/情感/位次/竞品/摘录）
function QuickResultCard({ r, fresh }: { r: MonitoringResult; fresh?: boolean }) {
  return (
    <div>
      <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', alignItems: 'center', marginBottom: 12 }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: 40, fontWeight: 800, color: (r.mention_rate || 0) > 0 ? 'var(--wr-success)' : 'var(--wr-danger)', lineHeight: 1.1 }}>
            {Math.round((r.mention_rate || 0) * 100)}%
          </div>
          <Text type="secondary" style={{ fontSize: 11 }}>提及率（{r.sample_count} 次采样）</Text>
        </div>
        <Space size={8} wrap>
          <Text style={{ fontSize: 13 }}>情感：{sentimentTag(r.sentiment)}</Text>
          {(r.avg_position || 0) > 0 && (
            <Text style={{ fontSize: 13 }}>回答位次：<Tag color="purple" style={{ margin: 0 }}>#{r.avg_position}</Tag></Text>
          )}
          {(r.self_source_count || 0) > 0 && (
            <Tag color="success" style={{ margin: 0 }}>引用了你的内容 {r.self_source_count} 次</Tag>
          )}
          {r.semantic_degraded && (
            <Tag color="warning" style={{ margin: 0 }} title="部分采样解析降级（字符串匹配兜底）——情感/位次可能不准确">解析降级</Tag>
          )}
          {fresh && <Tag color="processing" style={{ margin: 0 }}>刚刚查询</Tag>}
        </Space>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {r.probed_at ? new Date(r.probed_at).toLocaleString() : ''}
        </Text>
      </div>

      {Object.keys(r.competitor_rates || {}).length > 0 && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="AI 回答中还提到了这些品牌"
          description={
            <Space wrap size={[8, 4]}>
              {Object.entries(r.competitor_rates || {}).map(([name, rate]) => {
                const sent = r.competitor_sentiments?.[name]
                return (
                  <Tag key={name} style={{ fontSize: 11 }}>
                    {name} · {Math.round(rate * 100)}%
                    {sent === 'positive' && <CheckCircleOutlined style={{ color: 'var(--wr-success)', fontSize: 10, marginLeft: 4 }} />}
                    {sent === 'negative' && <CloseCircleOutlined style={{ color: 'var(--wr-danger)', fontSize: 10, marginLeft: 4 }} />}
                  </Tag>
                )
              })}
            </Space>
          }
        />
      )}

      {r.raw_sample ? (
        <div>
          <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>AI 回答摘录</Text>
          <div style={{ maxHeight: 280, overflow: 'auto', padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 10 }}>
            <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12, lineHeight: 1.7, color: 'var(--wr-text-secondary)', margin: 0 }}>{r.raw_sample}</pre>
          </div>
        </div>
      ) : (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="本次回答无原文记录" />
      )}
    </div>
  )
}

/**
 * 速查 Tab：轻量即时监测（对齐 Geowise 速查工具）。
 * 选品牌 → 选关键词（或直接输入一个疑问句）→ 选择引擎 → 立即查一次。
 * 复用现有 monitorKeyword 通道，数据与矩阵共享。
 */
export default function QuickCheckTab({
  brands,
  keywords,
  engines,
  monitorResults,
}: {
  brands: Brand[]
  keywords: Keyword[]
  engines: EngineOption[]
  monitorResults: MonitoringResult[]
}) {
  const queryClient = useQueryClient()
  const [brandId, setBrandId] = useState<string>()
  const [keywordId, setKeywordId] = useState<string>()
  const [engine, setEngine] = useState<string>('')
  const [running, setRunning] = useState(false)
  const [results, setResults] = useState<MonitoringResult[]>([])
  // 速查与问题库联动：输入一个真实问题，无匹配关键词时自动入库再监测
  const [questionText, setQuestionText] = useState('')

  const brandKeywords = useMemo(
    () => keywords.filter((k) => k.brand_id === brandId),
    [keywords, brandId],
  )

  // 匹配问题 → 已有关键词（问题词即关键词，支持模糊包含）
  const matchedKw = useMemo(() => {
    const q = questionText.trim().toLowerCase()
    if (!q) return undefined
    return brandKeywords.find((k) => k.term.toLowerCase().includes(q) || q.includes(k.term.toLowerCase()))
  }, [brandKeywords, questionText])

  // 历史最近一条（未查询时展示）
  const history = useMemo(() => {
    const kwId = keywordId || matchedKw?.id
    if (!kwId) return null
    const list = monitorResults.filter((r) => r.keyword_id === kwId && (!engine || r.engine_name === engine))
    return latestMonitor(list)
  }, [monitorResults, keywordId, matchedKw, engine])

  // 解析目标关键词（关键词选择 / 问题匹配 / 问题自动入库）
  const resolveKwId = async (): Promise<string | undefined> => {
    let kwId = keywordId
    if (!kwId && matchedKw) kwId = matchedKw.id
    if (!kwId && questionText.trim() && brandId) {
      try {
        const created = await businessApi.addKeyword(brandId, { term: questionText.trim(), intent: 'informational' })
        kwId = created?.id
      } catch { /* 拦截器已提示 */ }
    }
    return kwId
  }

  // 速查历史（P2-7-3）：最近 5 条结果持久化，刷新回显。
  // 修复（H1-③）：原实现"读入 effect 写 results + 监听 results 写回"互相触发，
  // 每次刷新历史翻倍直至上限全是重复项。现拆分为：restored 只在挂载读一次（不进 results），
  // 持久化只在查询成功时做一次（新结果在前、按 id 去重、截断 5 条）。
  const HISTORY_KEY = 'wr-quickcheck-history'
  const [restored, setRestored] = useState<MonitoringResult[] | null>(null)
  const [isFresh, setIsFresh] = useState(false)
  useEffect(() => {
    try {
      const h = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]')
      if (Array.isArray(h)) setRestored(h.slice(0, 5))
      // F4：恢复上次速查的品牌/引擎选择——"随手查"不该每次都重新选
      const last = JSON.parse(localStorage.getItem('wr-quickcheck-last') || 'null')
      if (last && typeof last === 'object') {
        if (last.brandId) setBrandId(last.brandId)
        if (typeof last.engine === 'string') setEngine(last.engine)
      }
    } catch { /* 损坏数据忽略 */ }
  }, [])

  const persistHistory = (items: MonitoringResult[]) => {
    try {
      const prev = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]')
      const ids = new Set(items.map((r) => r.id))
      const merged = [...items, ...(Array.isArray(prev) ? prev.filter((x: MonitoringResult) => !ids.has(x.id)) : [])]
      localStorage.setItem(HISTORY_KEY, JSON.stringify(merged.slice(0, 5)))
    } catch { /* 忽略 */ }
  }

  // 速查写入共享缓存后，矩阵/总览/引用/建议/健康分各 Tab 立即可见（30s staleTime 内不再滞留旧数据）
  const invalidateAfterCheck = (keywordCreated: boolean) => {
    void queryClient.invalidateQueries({ queryKey: ['geo-monitor-results'] })
    void queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
    void queryClient.invalidateQueries({ queryKey: ['geo-advice-last'] })
    void queryClient.invalidateQueries({ queryKey: ['geo-health-report'] })
    if (keywordCreated) void queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] })
  }

  // 解析目标关键词 + 是否新建入库（新建时需失效词库缓存）
  const resolveKwWithFlag = async (): Promise<[string | undefined, boolean]> => {
    const hadKw = !!keywordId || !!matchedKw
    const kwId = await resolveKwId()
    return [kwId, !!kwId && !hadKw]
  }

  // 单引擎查询
  const runCheck = async () => {
    const [kwId, createdKw] = await resolveKwWithFlag()
    if (!kwId) return
    setRunning(true)
    setResults([])
    try {
      const res = await businessApi.monitorKeyword({
        keyword_id: kwId,
        engine_name: engine || '',
        sample_size: 3,
      })
      setResults([res])
      setIsFresh(true)
      persistHistory([res])
      invalidateAfterCheck(createdKw)
      // F4：记忆本次品牌+引擎——下次进入即预填，动线从 4 步降为 1 步
      try {
        localStorage.setItem('wr-quickcheck-last', JSON.stringify({ brandId, engine, keywordId: kwId }))
      } catch { /* 忽略 */ }
    } catch { /* 拦截器已提示 */ } finally {
      setRunning(false)
    }
  }

  // 一键并发全部引擎（Geowise 速查思想：六平台并排结果卡）
  const runCheckAll = async () => {
    const [kwId, createdKw] = await resolveKwWithFlag()
    if (!kwId) return
    const engineNames = engines.map((e) => e.name)
    if (engineNames.length === 0) {
      message.warning('暂无可用引擎配置——请联系管理员')
      return
    }
    setRunning(true)
    setResults([])
    try {
      const res = await businessApi.monitorMulti({
        keyword_id: kwId,
        engine_names: engineNames,
        sample_size: 3,
      })
      const list = Array.isArray(res) ? res : [res]
      setResults(list)
      setIsFresh(true)
      persistHistory(list)
      invalidateAfterCheck(createdKw)
    } catch { /* 拦截器已提示 */ } finally {
      setRunning(false)
    }
  }

  // 展示优先级：本次查询结果 > 当前选中关键词的最近监测 > 上次速查回显
  const show = results.length > 0 ? results[0] : (history ?? restored?.[0] ?? null)

  return (
    <div>
      <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
        <Space wrap size={12}>
          <Text strong style={{ fontSize: 14 }}><ThunderboltOutlined style={{ marginRight: 6, color: 'var(--wr-warning)' }} />随手查一次</Text>
          <Select
            style={{ minWidth: 160 }}
            placeholder="选择品牌"
            value={brandId}
            onChange={(v) => { setBrandId(v); setKeywordId(undefined); setResults([]) }}
            options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
          />
          <Select
            style={{ minWidth: 200 }}
            placeholder="选择关键词（或直接在下方输入问题）"
            value={keywordId}
            onChange={(v) => { setKeywordId(v); setResults([]) }}
            options={brandKeywords.map((k: Keyword) => ({ value: k.id, label: k.term }))}
            disabled={!brandId}
            showSearch
            optionFilterProp="label"
            allowClear
          />
          <Select
            style={{ minWidth: 140 }}
            placeholder="引擎（默认）"
            allowClear
            value={engine || undefined}
            onChange={(v) => { setEngine(v || ''); setResults([]) }}
            options={engines.map((e: EngineOption) => ({ value: e.name, label: e.name }))}
          />
          <Button type="primary" loading={running} disabled={!brandId || (!keywordId && !questionText.trim())} onClick={runCheck}>
            {running ? 'AI 查询中...' : '立即查询'}
          </Button>
          <Button
            loading={running}
            disabled={!brandId || (!keywordId && !questionText.trim())}
            onClick={runCheckAll}
            icon={<ThunderboltOutlined />}
          >
            全部引擎并发查
          </Button>
        </Space>
        <div style={{ marginTop: 12 }}>
          <Input.TextArea
            rows={2}
            placeholder={'或输入一个真实问题直接查（如：成都哪家装修公司口碑好？）——系统会把它加入词库并立即监测'}
            value={questionText}
            onChange={(e) => setQuestionText(e.target.value)}
            maxLength={60}
          />
          {questionText.trim() && !matchedKw && brandId && (
            <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
              新问题：将自动加入「{brands.find((b) => b.id === brandId)?.name}」词库（可作为内容选题）再监测
            </Text>
          )}
        </div>
        <Paragraph type="secondary" style={{ fontSize: 12, margin: '10px 0 0' }}>
          速查 = 向 AI 引擎真实提问 {3} 次，看它是否提到你的品牌、态度如何、排在第几位——「全部引擎并发查」一键对比各平台（Geowise 速查思想）。
        </Paragraph>
      </Card>

      {running ? (
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin size="large" />
          <Paragraph type="secondary" style={{ marginTop: 12 }}>AI 正在回答中（约 10-30 秒）...</Paragraph>
        </div>
      ) : results.length > 1 ? (
        /* 多引擎并发：每引擎一张结果卡并排 */
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 12 }}>
          {results.map((r) => (
            <Card key={r.engine_name || 'default'} className="wr-glass-card" title={engineLabelOf(r.engine_name)}>
              <QuickResultCard r={r} fresh={isFresh} />
            </Card>
          ))}
        </div>
      ) : show ? (
        <Card className="wr-glass-card">
          <QuickResultCard r={show} fresh={isFresh} />
        </Card>
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="选择品牌与关键词，点「立即查询」——看 AI 现在怎么回答"
          style={{ padding: 60 }}
        />
      )}

      {/* 结果图例 */}
      <div style={{ marginTop: 12, display: 'flex', gap: 16, flexWrap: 'wrap', fontSize: 12, color: 'var(--wr-text-muted)' }}>
        <Space size={4}><CheckCircleOutlined style={{ color: 'var(--wr-success)' }} /> 提及</Space>
        <Space size={4}><CloseCircleOutlined style={{ color: 'var(--wr-text-muted)' }} /> 未提及</Space>
        <span>位次 #1 = AI 最先推荐你</span>
      </div>
    </div>
  )
}

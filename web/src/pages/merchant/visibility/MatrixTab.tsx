import { useMemo, useState, type Key } from 'react'
import {
  Button, Card, Col, Input, Progress, Row, Space, Table, Tag, Typography, message, Empty, Tooltip, Select, Modal, Form,
} from 'antd'
import {
  CheckCircleOutlined, CloseCircleOutlined, CloudSyncOutlined, ReloadOutlined,
  SearchOutlined, ExportOutlined, BarChartOutlined,
} from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import AutoMonitorControl from '../../../components/AutoMonitorControl'
import { LazyPie, LazyColumn } from '../../../components/charts/LazyCharts'
import { businessApi } from '../../../api/business'
import { mapWithConcurrency, settleSummary } from '../../../utils/async'
import { csvRow } from '../../../utils/csv'
import type { Keyword, MonitoringResult, EngineOption } from '../../../types/api'
import MonitorDetailPanel from '../keywords/MonitorDetailPanel'

const { Text, Title } = Typography
const { TextArea } = Input

/** 引擎展示名（监测 engine_name → 报表平台名） */
const ENGINE_LABEL: Record<string, string> = {
  default: '默认引擎',
  chatgpt: 'ChatGPT',
  kimi: 'Kimi',
  doubao: '豆包',
  deepseek: 'DeepSeek',
  qwen: '通义千问',
  ernie: '文心一言',
  yuanbao: '腾讯元宝',
  perplexity: 'Perplexity',
}

function engineLabel(name: string) {
  const key = (name || 'default').toLowerCase()
  return ENGINE_LABEL[key] || name || '默认引擎'
}

/** 情感 → 展示（矩阵单元格色点） */
function sentimentMeta(s?: string) {
  if (s === 'positive') return { color: 'var(--wr-success)', label: '正面' }
  if (s === 'negative') return { color: 'var(--wr-danger)', label: '负面' }
  return { color: 'var(--wr-text-muted)', label: '中性' }
}

type KwEngineCell = {
  rate: number
  mentioned: boolean
  sentiment?: string
  position?: number
  sampleCount?: number
  probed_at: string
}

type KwRow = {
  key: string
  keyword: Keyword
  brandName: string
  weight: number
  collected: number
  engines: Record<string, KwEngineCell>
  lastProbed: string
}

/**
 * 监测矩阵 Tab：关键词×引擎矩阵（提及✓/情感色点/位次）+ 监测执行 + 图表 + 导出。
 * 口径：提及率 > 0 视为该平台「已提及」；情感与位次来自最近一次采样。
 */
export default function MatrixTab({
  keywords,
  monitorResults,
  engines,
  brandMap,
  loading,
}: {
  keywords: Keyword[]
  monitorResults: MonitoringResult[]
  engines: EngineOption[]
  brandMap: Map<string, string>
  loading: boolean
}) {
  const queryClient = useQueryClient()
  const [keywordQuery, setKeywordQuery] = useState('')
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([])
  const [selectedEngines, setSelectedEngines] = useState<string[]>([])
  const [monitoringKwId, setMonitoringKwId] = useState<string | null>(null)
  const [monitoringBatch, setMonitoringBatch] = useState(false)
  const [batchProgress, setBatchProgress] = useState<{ done: number; total: number } | null>(null)
  const [addKwOpen, setAddKwOpen] = useState(false)
  const [addingKw, setAddingKw] = useState(false)
  const [addKwForm] = Form.useForm()
  const [detailRow, setDetailRow] = useState<KwRow | null>(null)

  const { data: autoMon } = useQuery({
    queryKey: ['tenant-auto-monitor'],
    queryFn: () => businessApi.getTenantAutoMonitor().catch(() => null),
  })

  const monitorByKeyword = useMemo(() => {
    const map = new Map<string, MonitoringResult[]>()
    monitorResults.forEach((r: MonitoringResult) => {
      const arr = map.get(r.keyword_id) || []
      arr.push(r)
      map.set(r.keyword_id, arr)
    })
    return map
  }, [monitorResults])

  const latestByKwEngine = useMemo(() => {
    const map = new Map<string, MonitoringResult>()
    const sorted = [...monitorResults].sort(
      (a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime(),
    )
    for (const r of sorted) {
      const eng = r.engine_name || 'default'
      const key = `${r.keyword_id}::${eng}`
      if (!map.has(key)) map.set(key, r)
    }
    return map
  }, [monitorResults])

  const engineNames = useMemo(() => {
    const set = new Set<string>()
    for (const r of monitorResults) set.add(r.engine_name || 'default')
    if (set.size === 0) set.add('default')
    return [...set].sort()
  }, [monitorResults])

  const rows: KwRow[] = useMemo(() => {
    return keywords.map((kw: Keyword) => {
      const engines: Record<string, KwEngineCell> = {}
      let collected = 0
      let lastProbed = ''
      let maxRate = 0
      for (const eng of engineNames) {
        const hit = latestByKwEngine.get(`${kw.id}::${eng}`)
        if (hit) {
          const mentioned = (hit.mention_rate || 0) > 0
          engines[eng] = {
            rate: hit.mention_rate || 0,
            mentioned,
            sentiment: hit.sentiment,
            position: hit.avg_position || 0,
            sampleCount: hit.sample_count || 0,
            probed_at: hit.probed_at,
          }
          if (mentioned) collected++
          if (!lastProbed || new Date(hit.probed_at) > new Date(lastProbed)) {
            lastProbed = hit.probed_at
          }
          maxRate = Math.max(maxRate, hit.mention_rate || 0)
        } else {
          engines[eng] = { rate: 0, mentioned: false, probed_at: '' }
        }
      }
      const weight = Math.round(maxRate * 100) || (collected > 0 ? 10 : 1)
      return {
        key: kw.id,
        keyword: kw,
        brandName: brandMap.get(kw.brand_id) || '—',
        weight,
        collected,
        engines,
        lastProbed,
      }
    })
  }, [keywords, engineNames, latestByKwEngine, brandMap])

  const latestProbedAt = useMemo(() => {
    let t = ''
    for (const r of monitorResults) {
      if (!t || new Date(r.probed_at) > new Date(t)) t = r.probed_at
    }
    return t ? new Date(t).toLocaleString('zh-CN', { hour12: false }) : '—'
  }, [monitorResults])

  const filteredRows = useMemo(() => {
    const q = keywordQuery.trim().toLowerCase()
    if (!q) return rows
    return rows.filter((r) =>
      r.keyword.term.toLowerCase().includes(q)
      || r.brandName.toLowerCase().includes(q),
    )
  }, [rows, keywordQuery])

  const totalCells = Math.max(1, keywords.length * engineNames.length)
  const collectedCells = rows.reduce((s, r) => s + r.collected, 0)
  const overallRate = collectedCells / totalCells
  const keywordsWithAny = rows.filter((r) => r.collected > 0).length

  // F2-1 引擎列瘦身：只展示"当前关键词有监测数据"的引擎——纯空列（"—"）折叠，
  // 新租户不再面对一屏空格子；被折叠的引擎数以提示 Tag 露出。
  const { displayEngines, hiddenEngineCount } = useMemo(() => {
    const withData = engineNames.filter((eng) =>
      rows.some((r) => r.engines[eng] && (r.engines[eng] as KwEngineCell).probed_at),
    )
    return {
      displayEngines: withData.length > 0 ? withData : engineNames.slice(0, 1),
      hiddenEngineCount: engineNames.length - (withData.length > 0 ? withData.length : 1),
    }
  }, [engineNames, rows])

  // F2-1 上手卡：数据覆盖率低时给明确下一步（"先监测 1 个词看效果"），
  // 不让老板面对空矩阵自己找按钮。取权重最高且无数据的关键词一键监测。
  const firstTargetRow = useMemo(
    () => [...rows].sort((a, b) => b.weight - a.weight).find((r) => !r.lastProbed) || null,
    [rows],
  )
  const showOnboarding = rows.length > 0 && overallRate < 0.3 && !!firstTargetRow

  const pieData = useMemo(() => {
    return engineNames.map((eng) => {
      let value = 0
      for (const r of rows) {
        if (r.engines[eng]?.mentioned) value++
      }
      return { type: engineLabel(eng), value }
    }).filter((d) => d.value > 0)
  }, [engineNames, rows])

  const barData = useMemo(() => {
    const kwN = Math.max(1, keywords.length)
    return engineNames.map((eng) => {
      let hit = 0
      for (const r of rows) {
        if (r.engines[eng]?.mentioned) hit++
      }
      return {
        platform: engineLabel(eng),
        rate: Math.round((hit / kwN) * 1000) / 10,
      }
    })
  }, [engineNames, rows, keywords.length])

  const invalidateMonitor = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['geo-monitor-results'] }),
      queryClient.invalidateQueries({ queryKey: ['geo-overviews'] }),
      queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] }),
      queryClient.invalidateQueries({ queryKey: ['geo-health-report'] }), // 健康分随监测结果联动
    ])
  }

  const runMonitorOne = async (keywordId: string) => {
    const engines = selectedEngines.filter(Boolean)
    if (engines.length > 1) {
      await businessApi.monitorMulti({ keyword_id: keywordId, engine_names: engines, sample_size: 3 })
    } else {
      await businessApi.monitorKeyword({ keyword_id: keywordId, sample_size: 3, engine_name: engines[0] || '' })
    }
  }

  const handleMonitorKeyword = async (keywordId: string) => {
    setMonitoringKwId(keywordId)
    try {
      await runMonitorOne(keywordId)
      message.success('监测完成，提及状态已更新')
      await invalidateMonitor()
    } catch { /* 拦截器已提示 */ } finally {
      setMonitoringKwId(null)
    }
  }

  const handleMonitorBatch = async (ids?: Key[]) => {
    const targets = (ids && ids.length > 0
      ? filteredRows.filter((r) => ids.includes(r.key))
      : filteredRows
    ).map((r) => r.key)

    if (targets.length === 0) {
      message.warning(keywords.length === 0 ? '暂无关键词，请先去关键词工程添词' : '没有可监测的关键词')
      return
    }

    setMonitoringBatch(true)
    // 受控并发（v3 P2：并发度 3——此前逐词串行几十分钟级等待；全并发会瞬间打满 LLM 配额）
    const settled = await mapWithConcurrency(
      targets.map(String),
      (id) => runMonitorOne(id),
      3,
      (done, total) => setBatchProgress({ done, total }),
    )
    const { ok, failed } = settleSummary(settled)
    setMonitoringKwId(null)
    setBatchProgress(null)
    setMonitoringBatch(false)
    message.success(`批量监测完成：${ok}/${targets.length} 成功${failed > 0 ? `（${failed} 个失败，可单独重测）` : ''}`)
    await invalidateMonitor()
  }

  const handleRefresh = async () => {
    await invalidateMonitor()
    message.success('报表已刷新')
  }

  const handleAddKeywords = async () => {
    try {
      const values = await addKwForm.validateFields()
      const terms = String(values.terms || '')
        .split(/[,，\n]/)
        .map((s: string) => s.trim())
        .filter(Boolean)
      if (terms.length === 0) {
        message.warning('请至少输入一个关键词')
        return
      }
      setAddingKw(true)
      for (const term of terms) {
        await businessApi.addKeyword(values.brand_id, {
          term,
          intent: values.intent || 'informational',
        })
      }
      message.success(`已添加 ${terms.length} 个关键词`)
      setAddKwOpen(false)
      addKwForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] })
    } catch (e) {
      if ((e as { errorFields?: unknown })?.errorFields) return
      /* 业务错误拦截器已提示 */
    } finally {
      setAddingKw(false)
    }
  }

  const handleExport = () => {
    const source = selectedRowKeys.length
      ? filteredRows.filter((r) => selectedRowKeys.includes(r.key))
      : filteredRows
    if (source.length === 0) {
      message.warning('没有可导出的数据')
      return
    }
    const header = ['关键词', '品牌', '权重', '已提及以上', ...engineNames.map(engineLabel), '最近探测']
    const lines = source.map((r) => csvRow([
      r.keyword.term,
      r.brandName,
      String(r.weight),
      String(r.collected),
      ...engineNames.map((eng) => (r.engines[eng]?.mentioned ? '是' : '否')),
      r.lastProbed ? new Date(r.lastProbed).toLocaleString('zh-CN') : '',
    ]))
    const bom = '\uFEFF'
    const blob = new Blob([bom + [csvRow(header), ...lines].join('\n')], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `AI提及监测_${Date.now()}.csv`
    a.click()
    URL.revokeObjectURL(url)
    message.success(`已导出 ${source.length} 条`)
  }

  // 单元格：提及 ✓ + 情感色点 + 位次数字；低采样（<3 次）灰显警示
  const renderCell = (cell?: KwEngineCell) => {
    if (!cell?.probed_at) return <Text type="secondary" style={{ fontSize: 12 }}>—</Text>
    const lowSample = (cell.sampleCount || 0) > 0 && (cell.sampleCount || 0) < 3
    if (!cell.mentioned) {
      return (
        <Tooltip title={lowSample ? `已监测但未提及（仅采样 ${cell.sampleCount} 次，置信度低）` : '已监测但未提及'}>
          <CloseCircleOutlined style={{ color: 'var(--wr-text-muted)', fontSize: 15, opacity: lowSample ? 0.45 : 1 }} />
        </Tooltip>
      )
    }
    const sm = sentimentMeta(cell.sentiment)
    return (
      <Space size={6} style={{ opacity: lowSample ? 0.55 : 1 }}>
        <Tooltip title={`提及率 ${(cell.rate * 100).toFixed(0)}% · ${sm.label}${lowSample ? ` · 仅采样 ${cell.sampleCount} 次（低置信度，建议复测）` : ''}`}>
          <CheckCircleOutlined style={{ color: 'var(--wr-success)', fontSize: 15 }} />
        </Tooltip>
        <Tooltip title={`情感：${sm.label}`}>
          <span style={{ width: 8, height: 8, borderRadius: '50%', background: sm.color, display: 'inline-block' }} />
        </Tooltip>
        {(cell.position || 0) > 0 && (
          <Tooltip title={`回答位次 #${cell.position}（1=最先推荐）`}>
            <Tag style={{ margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 5px' }}>#{cell.position}</Tag>
          </Tooltip>
        )}
      </Space>
    )
  }

  const columns = [
    {
      title: '关键词',
      dataIndex: ['keyword', 'term'],
      key: 'term',
      fixed: 'left' as const,
      width: 180,
      render: (term: string, row: KwRow) => (
        <div>
          <Text strong style={{ color: 'var(--wr-text-primary)' }}>{term}</Text>
          <div style={{ fontSize: 11, color: 'var(--wr-text-muted)' }}>{row.brandName}</div>
        </div>
      ),
    },
    {
      title: '权重',
      dataIndex: 'weight',
      key: 'weight',
      width: 72,
      sorter: (a: KwRow, b: KwRow) => a.weight - b.weight,
    },
    {
      title: '已提及',
      dataIndex: 'collected',
      key: 'collected',
      width: 88,
      render: (n: number) => (
        <Tag color={n > 0 ? 'success' : 'default'}>{n}/{engineNames.length}</Tag>
      ),
      sorter: (a: KwRow, b: KwRow) => a.collected - b.collected,
    },
    ...displayEngines.map((eng) => ({
      title: engineLabel(eng),
      key: eng,
      width: 130,
      align: 'center' as const,
      render: (_: unknown, row: KwRow) => renderCell(row.engines[eng]),
    })),
    {
      title: '最近探测',
      dataIndex: 'lastProbed',
      key: 'lastProbed',
      width: 160,
      render: (t: string) => (t ? new Date(t).toLocaleString('zh-CN', { hour12: false }) : '—'),
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      fixed: 'right' as const,
      render: (_: unknown, row: KwRow) => (
        <Space size={4}>
          <Button type="link" size="small" style={{ padding: 0 }} onClick={() => setDetailRow(row)}>
            详情
          </Button>
          <Button
            type="link"
            size="small"
            style={{ padding: 0 }}
            loading={monitoringKwId === row.key}
            disabled={monitoringBatch}
            onClick={() => handleMonitorKeyword(row.key)}
          >
            {monitoringKwId === row.key ? '监测中' : '监测'}
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      {/* 横幅大屏 */}
      <div className="wr-index-banner">
        <div className="wr-index-banner-inner">
          <div className="wr-index-banner-copy">
            <Title level={4} style={{ margin: 0, color: '#1a1a2e' }}>AI 提及监测报表</Title>
            <Text style={{ color: '#5a5a72', fontSize: 13 }}>
              按 AI 平台统计关键词提及占比 · 情感与位次来自最近一次采样
            </Text>
          </div>
          <div className="wr-index-banner-gauge">
            <Progress
              type="circle"
              percent={Math.round(overallRate * 100)}
              size={72}
              strokeColor={{ '0%': '#7c6cff', '100%': '#22d3ee' }}
              format={(p) => <span style={{ color: '#1a1a2e', fontWeight: 700 }}>{p}%</span>}
            />
          </div>
        </div>

        <Row gutter={[12, 12]} className="wr-index-stat-row">
          <Col xs={12} md={6}>
            <div className="wr-index-stat-chip">
              <span className="wr-index-stat-label">运营方式</span>
              <span className="wr-index-stat-value">
                {autoMon?.tenant_enabled && autoMon?.platform_enabled ? '托管盯盘' : '自助监测'}
              </span>
            </div>
          </Col>
          <Col xs={12} md={6}>
            <div className="wr-index-stat-chip">
              <span className="wr-index-stat-label">关键词资产</span>
              <span className="wr-index-stat-value">{keywords.length}</span>
            </div>
          </Col>
          <Col xs={12} md={6}>
            <div className="wr-index-stat-chip">
              <span className="wr-index-stat-label">综合提及率</span>
              <span className="wr-index-stat-value" style={{ color: '#5b4fe0' }}>
                {(overallRate * 100).toFixed(1)}%
              </span>
              <span className="wr-index-stat-sub">{collectedCells}/{totalCells} 格 · {keywordsWithAny} 词被提及</span>
            </div>
          </Col>
          <Col xs={12} md={6}>
            <div className="wr-index-stat-chip">
              <span className="wr-index-stat-label">更新时间</span>
              <span className="wr-index-stat-value" style={{ fontSize: 14 }}>{latestProbedAt}</span>
            </div>
          </Col>
        </Row>

        <div className="wr-index-banner-actions">
          <Space wrap>
            <Select
              style={{ minWidth: 260 }}
              mode="multiple"
              maxTagCount={2}
              placeholder="监测引擎（空=default；多选对比）"
              allowClear
              value={selectedEngines}
              onChange={(v) => setSelectedEngines(v || [])}
              options={engines.map((e: EngineOption) => ({ value: e.name, label: `${e.name}（${e.model}）` }))}
            />
            <Button
              type="primary"
              icon={<CloudSyncOutlined />}
              loading={monitoringBatch}
              onClick={() => handleMonitorBatch()}
              disabled={keywords.length === 0}
            >
              {monitoringBatch
                ? (batchProgress ? `监测中 ${batchProgress.done}/${batchProgress.total}...` : '监测中...')
                : '立即更新（监测全部）'}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={handleRefresh} loading={loading}>
              刷新报表
            </Button>
            <Text type="secondary" style={{ fontSize: 12 }}>
              图例：<Tooltip title="✓ = AI 在回答里提到了你（灰 ✓ = 采样不足 3 次，结果不稳）"><span>✓ AI 提到了你</span></Tooltip>
              {' · '}
              <Tooltip title="绿点 = AI 夸你 / 灰点 = 中性提及 / 红点 = AI 批评你"><span>色点 = AI 对你的态度</span></Tooltip>
              {' · '}
              <Tooltip title="#1 = AI 第一个推荐你；#N 数字越小越靠前"><span>#N = AI 第几个推荐你</span></Tooltip>
              {hiddenEngineCount > 0 && (
                <Tooltip title="这些引擎对当前关键词还没有监测数据，列已折叠——发起监测后自动展开">
                  <Tag style={{ margin: '0 0 0 8px', fontSize: 11 }}>＋{hiddenEngineCount} 个引擎待监测</Tag>
                </Tooltip>
              )}
            </Text>
          </Space>
        </div>
      </div>

      {/* F2-1 上手卡：覆盖率低时给明确下一步——不让老板面对空矩阵自己找按钮 */}
      {showOnboarding && firstTargetRow && (
        <Card className="wr-glass-card" styles={{ body: { padding: 16 } }} style={{ marginBottom: 12, borderColor: 'rgba(124,108,255,0.3)' }}>
          <Space wrap>
            <Text strong style={{ fontSize: 14 }}>🚀 先监测 1 个词，看 AI 怎么评价你</Text>
            <Text type="secondary" style={{ fontSize: 12 }}>
              一次监测约 30 秒——立刻能看到「{firstTargetRow.keyword.term}」在各 AI 的提及与态度
            </Text>
            <Button
              type="primary" size="small" icon={<CloudSyncOutlined />}
              loading={monitoringKwId === firstTargetRow.key}
              onClick={() => handleMonitorKeyword(firstTargetRow.key)}
            >
              立即监测「{firstTargetRow.keyword.term.length > 12 ? firstTargetRow.keyword.term.slice(0, 12) + '…' : firstTargetRow.keyword.term}」
            </Button>
          </Space>
        </Card>
      )}

      {/* 自动盯盘配置（监测执行方式归监测页——此前在工作台占据大块表单） */}
      <AutoMonitorControl />

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={12}>
          <Card
            className="wr-glass-card"
            title={<Space><BarChartOutlined />各大 AI 平台提及占比</Space>}
            styles={{ body: { padding: 20, minHeight: 300 } }}
          >
            {pieData.length === 0 ? (
              <Empty description="暂无提及数据——请在上方发起监测" style={{ padding: 48 }} />
            ) : (
              <LazyPie
                data={pieData}
                angleField="value"
                colorField="type"
                height={260}
                radius={0.85}
                innerRadius={0.62}
                color={['#7c6cff', '#22d3ee', '#4ade80', '#fbbf24', '#fb7185', '#60a5fa']}
                label={{ text: 'type', position: 'outside', fontSize: 11 }}
                legend={{ position: 'bottom' }}
              />
            )}
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card
            className="wr-glass-card"
            title={<Space><BarChartOutlined />各平台提及率对比</Space>}
            styles={{ body: { padding: 20, minHeight: 300 } }}
          >
            {barData.every((d) => d.rate === 0) ? (
              <Empty description="暂无对比数据" style={{ padding: 48 }} />
            ) : (
              <LazyColumn
                data={barData}
                xField="platform"
                yField="rate"
                height={260}
                style={{
                  fill: 'l(270) 0:#a99cff 1:#7c6cff',
                  radiusTopLeft: 6,
                  radiusTopRight: 6,
                }}
                label={{
                  text: (d: { rate: number }) => `${d.rate}%`,
                  position: 'top',
                  style: { fill: 'var(--wr-text-secondary)', fontSize: 11 },
                }}
                axis={{
                  y: { title: false, labelFormatter: (v: string) => `${v}%` },
                  x: { title: false },
                }}
              />
            )}
          </Card>
        </Col>
      </Row>

      <Card
        className="wr-glass-card"
        title="关键词提及资产列表"
        styles={{ body: { padding: 16 } }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', marginBottom: 14 }}>
          <Space.Compact style={{ maxWidth: 360, width: '100%' }}>
            <Input
              allowClear
              prefix={<SearchOutlined style={{ color: 'var(--wr-text-muted)' }} />}
              placeholder="搜索关键词 / 品牌"
              value={keywordQuery}
              onChange={(e) => setKeywordQuery(e.target.value)}
            />
            <Button type="primary">搜索</Button>
          </Space.Compact>
          <Space wrap>
            <Button
              loading={monitoringBatch}
              disabled={filteredRows.length === 0}
              onClick={() => handleMonitorBatch(selectedRowKeys.length ? selectedRowKeys : undefined)}
            >
              {monitoringBatch
                ? (batchProgress ? `监测中 ${batchProgress.done}/${batchProgress.total}` : '监测中...')
                : selectedRowKeys.length
                  ? `监测选中(${selectedRowKeys.length})`
                  : '批量监测'}
            </Button>
            <Button icon={<ExportOutlined />} onClick={handleExport}>
              {selectedRowKeys.length ? `导出选中(${selectedRowKeys.length})` : '导出全部'}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={handleRefresh}>刷新</Button>
          </Space>
        </div>

        <Table
          rowKey="key"
          loading={loading}
          size="middle"
          scroll={{ x: 900 + engineNames.length * 130 }}
          rowSelection={{
            selectedRowKeys,
            onChange: setSelectedRowKeys,
          }}
          columns={columns}
          dataSource={filteredRows}
          pagination={{ pageSize: 10, showSizeChanger: true }}
          locale={{ emptyText: <Empty description="暂无关键词资产" /> }}
        />
      </Card>

      {/* 添加关键词弹窗 */}
      <Modal
        title="添加关键词"
        open={addKwOpen}
        onCancel={() => { setAddKwOpen(false); addKwForm.resetFields() }}
        onOk={handleAddKeywords}
        confirmLoading={addingKw}
        okText="添加"
        destroyOnClose
        width={480}
      >
        <Form form={addKwForm} layout="vertical" requiredMark={false} style={{ marginTop: 8 }}>
          <Form.Item name="brand_id" label="所属品牌" rules={[{ required: true, message: '请选择品牌' }]}>
            <Select
              placeholder="选择品牌"
              options={Array.from(brandMap.entries()).map(([id, name]) => ({ value: id, label: name }))}
            />
          </Form.Item>
          <Form.Item name="intent" label="意图" initialValue="informational">
            <Select
              options={[
                { value: 'informational', label: '信息型' },
                { value: 'transactional', label: '交易型' },
                { value: 'local', label: '本地型' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="terms"
            label="关键词"
            rules={[{ required: true, message: '请输入关键词' }]}
            extra="多个词用逗号或换行分隔"
          >
            <TextArea rows={4} placeholder={'例如：\nagent 开发框架\n源码解析'} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 监测详情抽屉（趋势/引擎对比/采样原文） */}
      <Modal
        title={detailRow ? `监测详情 · ${detailRow.keyword.term}` : '监测详情'}
        open={!!detailRow}
        onCancel={() => setDetailRow(null)}
        footer={
          <Space>
            <Button onClick={() => setDetailRow(null)}>关闭</Button>
            {detailRow && (
              <Button
                type="primary"
                loading={monitoringKwId === detailRow.key}
                onClick={() => handleMonitorKeyword(detailRow.key)}
              >
                重新监测
              </Button>
            )}
          </Space>
        }
        width={820}
        destroyOnClose
        styles={{ body: { maxHeight: '70vh', overflowY: 'auto' } }}
      >
        {detailRow && (
          <MonitorDetailPanel
            results={monitorByKeyword.get(detailRow.key) || []}
            brandName={detailRow.brandName === '—' ? '' : detailRow.brandName}
          />
        )}
      </Modal>
    </div>
  )
}

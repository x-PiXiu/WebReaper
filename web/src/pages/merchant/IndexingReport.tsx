import { useMemo, useState, type Key } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Button, Card, Col, Input, Progress, Row, Space, Table, Tag, Typography, message, Empty, Tooltip, Select, Modal, Form,
} from 'antd'
import {
  CheckCircleOutlined, CloseCircleOutlined, CloudSyncOutlined, ReloadOutlined,
  SearchOutlined, ExportOutlined, BarChartOutlined, PlusOutlined,
} from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { LazyPie, LazyColumn } from '../../components/charts/LazyCharts'
import { businessApi } from '../../api/business'
import type { Brand, Keyword, MonitoringResult, LLMConfig } from '../../types/api'
import MonitorDetailPanel from './keywords/MonitorDetailPanel'

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

type KwEngineCell = {
  rate: number
  mentioned: boolean
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
 * 平台收录报表（含监测执行）
 * 报表矩阵 + 在此发起单词/批量监测，展开可看监测明细。
 * 口径：提及率 > 0 视为该平台「已收录」。
 */
export default function IndexingReport() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [keywordQuery, setKeywordQuery] = useState('')
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([])
  const [selectedEngines, setSelectedEngines] = useState<string[]>([])
  const [monitoringKwId, setMonitoringKwId] = useState<string | null>(null)
  const [monitoringBatch, setMonitoringBatch] = useState(false)
  const [addKwOpen, setAddKwOpen] = useState(false)
  const [addingKw, setAddingKw] = useState(false)
  const [addKwForm] = Form.useForm()
  const [detailRow, setDetailRow] = useState<KwRow | null>(null)

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })
  const brandMap = useMemo(
    () => new Map(brands.map((b: Brand) => [b.id, b.name])),
    [brands],
  )

  const { data: llmConfigs = [] } = useQuery({
    queryKey: ['llm-configs'],
    queryFn: () => businessApi.listLLMConfigs(),
  })

  const { data: keywords = [], isLoading: kwLoading } = useQuery({
    queryKey: ['geo-all-keywords'],
    queryFn: () => businessApi.listAllKeywords(),
  })

  const { data: monitorResults = [], isLoading: monLoading, dataUpdatedAt } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults(),
  })

  const { data: autoMon } = useQuery({
    queryKey: ['tenant-auto-monitor'],
    queryFn: () => businessApi.getTenantAutoMonitor().catch(() => null),
  })

  const loading = kwLoading || monLoading

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

  const updatedAtText = dataUpdatedAt
    ? new Date(dataUpdatedAt).toLocaleString('zh-CN', { hour12: false })
    : '—'

  const invalidateMonitor = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['geo-monitor-results'] }),
      queryClient.invalidateQueries({ queryKey: ['geo-overviews'] }),
      queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] }),
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
      message.success('监测完成，收录状态已更新')
      await invalidateMonitor()
    } catch (e) {
      message.error('监测失败：' + ((e as Error)?.message || ''))
    } finally {
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
    let success = 0
    for (const id of targets) {
      setMonitoringKwId(String(id))
      try {
        await runMonitorOne(String(id))
        success++
      } catch { /* continue */ }
    }
    setMonitoringKwId(null)
    setMonitoringBatch(false)
    message.success(`批量监测完成：${success}/${targets.length} 成功`)
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
      message.error('添加失败：' + ((e as Error)?.message || ''))
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
    const header = ['关键词', '品牌', '权重', '已收录数', ...engineNames.map(engineLabel), '最近探测']
    const lines = source.map((r) => [
      r.keyword.term,
      r.brandName,
      String(r.weight),
      String(r.collected),
      ...engineNames.map((eng) => (r.engines[eng]?.mentioned ? '是' : '否')),
      r.lastProbed ? new Date(r.lastProbed).toLocaleString('zh-CN') : '',
    ].join(','))
    const bom = '\uFEFF'
    const blob = new Blob([bom + [header.join(','), ...lines].join('\n')], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `平台收录报表_${Date.now()}.csv`
    a.click()
    URL.revokeObjectURL(url)
    message.success(`已导出 ${source.length} 条`)
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
      title: '已收录',
      dataIndex: 'collected',
      key: 'collected',
      width: 88,
      render: (n: number) => (
        <Tag color={n > 0 ? 'success' : 'default'}>{n}/{engineNames.length}</Tag>
      ),
      sorter: (a: KwRow, b: KwRow) => a.collected - b.collected,
    },
    ...engineNames.map((eng) => ({
      title: engineLabel(eng),
      key: eng,
      width: 100,
      align: 'center' as const,
      render: (_: unknown, row: KwRow) => {
        const cell = row.engines[eng]
        if (!cell?.probed_at) {
          return <Text type="secondary" style={{ fontSize: 12 }}>—</Text>
        }
        return cell.mentioned ? (
          <Tooltip title={`提及率 ${(cell.rate * 100).toFixed(0)}%`}>
            <CheckCircleOutlined style={{ color: 'var(--wr-success)', fontSize: 16 }} />
          </Tooltip>
        ) : (
          <Tooltip title="已监测但未提及">
            <CloseCircleOutlined style={{ color: 'var(--wr-text-muted)', fontSize: 16 }} />
          </Tooltip>
        )
      },
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
    <div className="wr-page-content wr-index-report" style={{ paddingTop: 4 }}>
      <div className="wr-page-header" style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1>平台收录报表</h1>
          <p>统计各 AI 平台收录覆盖 · 在此发起监测并查看明细</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddKwOpen(true)}>
          添加关键词
        </Button>
      </div>

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
              options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
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
        <Text type="secondary" style={{ fontSize: 12 }}>
          需要 AI 蒸馏生成？
          <Button type="link" size="small" style={{ padding: '0 4px' }} onClick={() => { setAddKwOpen(false); navigate('/m/keywords') }}>
            打开关键词工程
          </Button>
        </Text>
      </Modal>

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

      <div className="wr-index-banner">
        <div className="wr-index-banner-inner">
          <div className="wr-index-banner-copy">
            <Title level={4} style={{ margin: 0, color: '#1a1a2e' }}>平台收录数据报表</Title>
            <Text style={{ color: '#5a5a72', fontSize: 13 }}>
              按 AI 平台统计关键词收录占比，沉淀可运营的关键词收录资产
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
              <span className="wr-index-stat-label">综合收录率</span>
              <span className="wr-index-stat-value" style={{ color: '#5b4fe0' }}>
                {(overallRate * 100).toFixed(1)}%
              </span>
              <span className="wr-index-stat-sub">{collectedCells}/{totalCells} 格 · {keywordsWithAny} 词有收录</span>
            </div>
          </Col>
          <Col xs={12} md={6}>
            <div className="wr-index-stat-chip">
              <span className="wr-index-stat-label">更新时间</span>
              <span className="wr-index-stat-value" style={{ fontSize: 14 }}>{updatedAtText}</span>
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
              options={llmConfigs.map((l: LLMConfig) => ({ value: l.name, label: `${l.name}（${l.model}）` }))}
            />
            <Button
              type="primary"
              icon={<CloudSyncOutlined />}
              loading={monitoringBatch}
              onClick={() => handleMonitorBatch()}
              disabled={keywords.length === 0}
            >
              {monitoringBatch ? '监测中...' : '立即更新（监测全部）'}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={handleRefresh} loading={loading}>
              刷新报表
            </Button>
            <Text type="secondary" style={{ fontSize: 12 }}>
              口径：监测提及率 &gt; 0 计为该 AI 平台已收录
            </Text>
          </Space>
        </div>
      </div>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={12}>
          <Card
            className="wr-glass-card"
            title={<Space><BarChartOutlined />各大 AI 平台收录占比</Space>}
            styles={{ body: { padding: 20, minHeight: 300 } }}
          >
            {pieData.length === 0 ? (
              <Empty description="暂无收录数据——请在上方发起监测" style={{ padding: 48 }} />
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
            title={<Space><BarChartOutlined />各平台收录率对比</Space>}
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
        title="关键词收录资产列表"
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
              {selectedRowKeys.length
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
          scroll={{ x: 900 + engineNames.length * 100 }}
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
    </div>
  )
}

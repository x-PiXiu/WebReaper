import { useState } from 'react'
import { Card, Typography, Button, Table, Tag, Space, message, Input, Select, Tabs, Upload, Checkbox, Empty, Spin, Popconfirm, Progress } from 'antd'
import { UploadOutlined } from '@ant-design/icons'
import { Line } from '@ant-design/charts'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { mentionDelta, deltaView, markLastPoint } from '../../utils/geo'
import type { Brand, Keyword, MonitoringResult, LLMConfig } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

type SourceType = 'mine' | 'brand' | 'text' | 'seed' | 'file' | 'web'

function rateColor(rate: number): string {
  if (rate >= 0.8) return 'var(--wr-success)'
  if (rate >= 0.5) return 'var(--wr-accent)'
  if (rate >= 0.2) return 'var(--wr-warning)'
  return 'var(--wr-danger)'
}

export default function Keywords() {
  const queryClient = useQueryClient()
  const [activeSource, setActiveSource] = useState<SourceType>('mine')
  const [distilling, setDistilling] = useState(false)
  const [resultKeywords, setResultKeywords] = useState<string[]>([])
  const [checkedKeywords, setCheckedKeywords] = useState<string[]>([])
  const [targetBrand, setTargetBrand] = useState<string | undefined>()
  const [selectedBrandForMonitor, setSelectedBrandForMonitor] = useState<string | undefined>()

  // 蒸馏输入
  const [brandForSource, setBrandForSource] = useState<string | undefined>()
  const [textInput, setTextInput] = useState('')
  const [seedsInput, setSeedsInput] = useState('')
  const [topicInput, setTopicInput] = useState('')
  const [fileText, setFileText] = useState('')
  const [fileName, setFileName] = useState('')
  const [monitoringKwId, setMonitoringKwId] = useState<string | null>(null) // 正在监测的关键词
  const [monitoringAll, setMonitoringAll] = useState(false) // 一键监测全部
  const [selectedEngine, setSelectedEngine] = useState<string>('') // 监测用的 LLM 引擎（空=default）
  const [intentFilter, setIntentFilter] = useState<string>('') // 意图筛选（空=全部）

  const { data: brands = [] } = useQuery({ queryKey: ['geo-brands'], queryFn: () => businessApi.listBrands() })
  const { data: llmConfigs = [] } = useQuery({ queryKey: ['llm-configs'], queryFn: () => businessApi.listLLMConfigs() })
  const { data: allKeywords = [] } = useQuery({ queryKey: ['geo-all-keywords'], queryFn: () => businessApi.listAllKeywords() })
  const brandMap = new Map(brands.map((b: Brand) => [b.id, b.name]))

  // 监测结果（租户级查询，不依赖品牌筛选——这样无论有没有选品牌，监测后都能刷新）
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults(),
  })
  // 按 keyword_id 分组监测结果
  const monitorByKeyword = new Map<string, MonitoringResult[]>()
  monitorResults.forEach((r: MonitoringResult) => {
    const arr = monitorByKeyword.get(r.keyword_id) || []
    arr.push(r)
    monitorByKeyword.set(r.keyword_id, arr)
  })

  // 过滤关键词（按选中的品牌 + 意图）
  const displayedKeywords = allKeywords.filter((k: Keyword) =>
    (!selectedBrandForMonitor || k.brand_id === selectedBrandForMonitor) &&
    (!intentFilter || k.intent === intentFilter)
  )

  // 蒸馏
  const handleDistill = async () => {
    setDistilling(true)
    setResultKeywords([])
    setCheckedKeywords([])
    try {
      const params: Record<string, unknown> = { source: activeSource }
      if (activeSource === 'brand') {
        if (!brandForSource) { message.warning('请选择品牌'); setDistilling(false); return }
        params.brand_id = brandForSource
      } else if (activeSource === 'text') {
        if (!textInput.trim()) { message.warning('请输入文本'); setDistilling(false); return }
        params.text = textInput
      } else if (activeSource === 'seed') {
        const seeds = seedsInput.split(/[,，\n]/).map(s => s.trim()).filter(Boolean)
        if (seeds.length === 0) { message.warning('请输入种子词'); setDistilling(false); return }
        params.seeds = seeds
      } else if (activeSource === 'web') {
        if (!topicInput.trim()) { message.warning('请输入主题'); setDistilling(false); return }
        params.topic = topicInput
      } else if (activeSource === 'file') {
        if (!fileText.trim()) { message.warning('请先上传文件'); setDistilling(false); return }
        params.text = fileText
      }
      const res = await businessApi.distillKeywords(params as any)
      const kws = res.keywords || []
      setResultKeywords(kws)
      if (kws.length === 0) message.warning('未蒸馏出关键词')
    } catch (e) {
      message.error('蒸馏失败：' + ((e as Error)?.message || ''))
    } finally {
      setDistilling(false)
    }
  }

  const handleAddKeywords = async () => {
    if (!targetBrand) { message.warning('请先选择目标品牌'); return }
    if (checkedKeywords.length === 0) { message.warning('请至少勾选一个关键词'); return }
    try {
      for (const term of checkedKeywords) {
        await businessApi.addKeyword(targetBrand, { term, intent: 'informational' })
      }
      message.success(`已添加 ${checkedKeywords.length} 个关键词`)
      queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] })
      setCheckedKeywords([])
    } catch {}
  }

  const handleFileUpload = (file: File) => {
    setFileName(file.name)
    const reader = new FileReader()
    reader.onload = (e) => {
      const text = e.target?.result as string
      setFileText(text || '')
      message.success(`已读取 ${file.name}`)
    }
    reader.readAsText(file)
    return false
  }

  const handleDelete = async (id: string) => {
    try {
      await businessApi.deleteKeyword(id)
      message.success('已删除')
      queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] })
    } catch {}
  }

  // 单关键词即时监测（用选中的引擎）
  const handleMonitorKeyword = async (keywordId: string) => {
    setMonitoringKwId(keywordId)
    try {
      await businessApi.monitorKeyword({ keyword_id: keywordId, sample_size: 1, engine_name: selectedEngine })
      message.success(`监测完成（引擎：${selectedEngine || 'default'}）`)
      queryClient.invalidateQueries({ queryKey: ['geo-monitor-results'] })
      queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
    } catch (e) {
      message.error('监测失败：' + ((e as Error)?.message || ''))
    } finally {
      setMonitoringKwId(null)
    }
  }

  // 一键监测全部（批量：逐个调单关键词监测）
  const handleMonitorAll = async () => {
    if (displayedKeywords.length === 0) {
      message.warning('暂无关键词可监测')
      return
    }
    setMonitoringAll(true)
    let success = 0
    for (const kw of displayedKeywords) {
      setMonitoringKwId(kw.id)
      try {
        await businessApi.monitorKeyword({ keyword_id: kw.id, sample_size: 1, engine_name: selectedEngine })
        success++
      } catch {}
    }
    setMonitoringKwId(null)
    setMonitoringAll(false)
    message.success(`批量监测完成：${success}/${displayedKeywords.length} 成功（引擎：${selectedEngine || 'default'}）`)
    queryClient.invalidateQueries({ queryKey: ['geo-monitor-results'] })
    queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
  }

  // 蒸馏来源面板
  const sourcePanels: Record<string, React.ReactNode> = {
    brand: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>
          根据品牌的定位、核心卖点和竞品信息，结合全网内容蒸馏关键词
        </Text>
        <Select style={{ width: '100%' }} placeholder="选择品牌" value={brandForSource} onChange={setBrandForSource}
          options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))} />
      </div>
    ),
    text: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>
          粘贴任意文本，AI 从中蒸馏核心关键词
        </Text>
        <TextArea rows={5} placeholder="粘贴文本内容..." value={textInput} onChange={(e) => setTextInput(e.target.value)} />
      </div>
    ),
    seed: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>
          输入种子词，AI 拓展出长尾关键词和问题词
        </Text>
        <TextArea rows={3} placeholder="用逗号分隔，如：agent开发, 源码解析" value={seedsInput} onChange={(e) => setSeedsInput(e.target.value)} />
      </div>
    ),
    file: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>
          上传 txt/md 文件，AI 读取内容后蒸馏关键词
        </Text>
        <Upload beforeUpload={handleFileUpload} accept=".txt,.md,.markdown" maxCount={1} showUploadList={false}>
          <Button icon={<UploadOutlined />}>选择文件</Button>
        </Upload>
        {fileName && <div style={{ marginTop: 8 }}><Tag color="orange">{fileName}</Tag><Text type="secondary" style={{ fontSize: 12 }}>{fileText.length} 字符</Text></div>}
      </div>
    ),
    web: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>
          输入主题，AI 爬取全网相关内容后蒸馏关键词
        </Text>
        <Input placeholder="如：agent 开发框架对比" value={topicInput} onChange={(e) => setTopicInput(e.target.value)} />
      </div>
    ),
  }

  // 关键词列表列（折叠态显示全部5维度，展开看各引擎详情+AI回答）
  const keywordColumns = [
    {
      title: '关键词', dataIndex: 'term', key: 'term', width: 180,
      render: (t: string) => <Text strong>{t}</Text>,
    },
    {
      title: '品牌', key: 'brand', width: 100,
      render: (_: unknown, r: Keyword) => {
        const name = brandMap.get(r.brand_id)
        return name ? <Tag color="blue">{name}</Tag> : <Text type="secondary">-</Text>
      },
    },
    {
      title: 'AI 可见度', key: 'monitor',
      render: (_: unknown, r: Keyword) => {
        const results = monitorByKeyword.get(r.id) || []
        if (results.length === 0) {
          return <Text type="secondary" style={{ fontSize: 12 }}>未监测</Text>
        }
        // 聚合各引擎的结果，取每个维度的最优值
        const bestMention = results.reduce((a: MonitoringResult, b: MonitoringResult) => a.mention_rate > b.mention_rate ? a : b)
        const bestPosition = results.reduce((a: MonitoringResult, b: MonitoringResult) =>
          (a.avg_position > 0 && (b.avg_position === 0 || a.avg_position < b.avg_position)) ? a : b)
        const allCompetitors = new Set<string>()
        results.forEach((rr: MonitoringResult) => rr.competitors?.forEach((c: string) => allCompetitors.add(c)))
        const avgConfidence = results.reduce((s: number, rr: MonitoringResult) => s + rr.confidence, 0) / results.length

        // 情感统计（取多数）
        const sentCount: Record<string, number> = {}
        results.forEach((rr: MonitoringResult) => { sentCount[rr.sentiment] = (sentCount[rr.sentiment] || 0) + 1 })
        const dominantSentiment = Object.entries(sentCount).sort((a, b) => b[1] - a[1])[0]?.[0] || 'neutral'

        const sentColor = dominantSentiment === 'positive' ? 'var(--wr-success)' : dominantSentiment === 'negative' ? 'var(--wr-danger)' : 'var(--wr-text-muted)'
        const sentLabel = dominantSentiment === 'positive' ? '正面' : dominantSentiment === 'negative' ? '负面' : '中性'

        // 变化对比（最新 vs 上一次监测的提及率差）
        const delta = deltaView(mentionDelta(results))

        return (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {/* 维度1：提及率（含变化对比 delta——付费说服力：让用户看到"在生效"）*/}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 72, padding: '4px 8px', borderRadius: 8, background: `${rateColor(bestMention.mention_rate)}15` }}>
              <Text style={{ fontSize: 16, fontWeight: 700, color: rateColor(bestMention.mention_rate), lineHeight: 1.2 }}>
                {(bestMention.mention_rate * 100).toFixed(0)}%
              </Text>
              <Text type="secondary" style={{ fontSize: 10 }}>提及率</Text>
              <Text style={{ fontSize: 10, color: delta.color, fontWeight: 600 }}>
                {delta.arrow} {delta.text}
              </Text>
            </div>
            {/* 维度2：排名 */}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 56, padding: '4px 8px', borderRadius: 8, background: 'var(--wr-bg-elevated)' }}>
              <Text style={{ fontSize: 16, fontWeight: 700, color: bestPosition.avg_position > 0 ? 'var(--wr-accent)' : 'var(--wr-text-muted)', lineHeight: 1.2 }}>
                {bestPosition.avg_position > 0 ? `#${bestPosition.avg_position}` : '-'}
              </Text>
              <Text type="secondary" style={{ fontSize: 10 }}>排名</Text>
            </div>
            {/* 维度3：情感 */}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 48, padding: '4px 8px', borderRadius: 8, background: `${sentColor}15` }}>
              <Text style={{ fontSize: 13, fontWeight: 700, color: sentColor, lineHeight: 1.6 }}>{sentLabel}</Text>
              <Text type="secondary" style={{ fontSize: 10 }}>情感</Text>
            </div>
            {/* 维度4：竞品 */}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 48, padding: '4px 8px', borderRadius: 8, background: 'var(--wr-bg-elevated)' }}>
              <Text style={{ fontSize: 16, fontWeight: 700, color: 'var(--wr-text-primary)', lineHeight: 1.2 }}>{allCompetitors.size}</Text>
              <Text type="secondary" style={{ fontSize: 10 }}>竞品提及</Text>
            </div>
            {/* 维度5：置信度（含采样次数——可信度传达：监测是采样，样本越多越可信）*/}
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 64, padding: '4px 8px', borderRadius: 8, background: avgConfidence >= 0.6 ? 'var(--wr-primary-bg)' : 'var(--wr-bg-elevated)' }}>
              <Text style={{ fontSize: 14, fontWeight: 700, color: avgConfidence >= 0.6 ? 'var(--wr-primary)' : 'var(--wr-warning)', lineHeight: 1.5 }}>
                {(avgConfidence * 100).toFixed(0)}%
              </Text>
              <Text type="secondary" style={{ fontSize: 10 }}>置信度</Text>
              <Text type="secondary" style={{ fontSize: 9, opacity: 0.8 }}>采样 {bestMention.sample_count || 1} 次</Text>
            </div>
          </div>
        )
      },
    },
    {
      title: '操作', key: 'action', width: 120,
      render: (_: unknown, r: Keyword) => (
        <Space size="small">
          <Button size="small" type="link" loading={monitoringKwId === r.id} onClick={() => handleMonitorKeyword(r.id)}>
            {monitoringKwId === r.id ? '监测中' : '监测'}
          </Button>
          <Popconfirm title="删除此关键词？" onConfirm={() => handleDelete(r.id)}>
            <Button size="small" type="text" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
      <div className="wr-page-header">
        <h1>关键词管理</h1>
        <p>多来源关键词蒸馏 · 结构化生成 · 监测结果一览</p>
      </div>

      <Card className="wr-glass-card" styles={{ body: { padding: 24 } }}>
        <Tabs
          activeKey={activeSource}
          onChange={(k) => setActiveSource(k as SourceType)}
          items={[
            {
              key: 'mine',
              label: <Space><span>我的关键词</span><Tag>{allKeywords.length}</Tag></Space>,
              children: (
                <div>
                  {/* 工具栏：品牌筛选 + 引擎选择 + 一键监测 */}
                  <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
                    <Space>
                      <Select
                        style={{ width: 200 }}
                        placeholder="按品牌筛选（空=全部）"
                        allowClear
                        value={selectedBrandForMonitor}
                        onChange={setSelectedBrandForMonitor}
                        options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
                      />
                      <Select
                        style={{ width: 180 }}
                        placeholder="监测引擎（空=default）"
                        allowClear
                        value={selectedEngine || undefined}
                        onChange={(v) => setSelectedEngine(v || '')}
                        options={llmConfigs.map((l: LLMConfig) => ({ value: l.name, label: `${l.name}（${l.model}）` }))}
                      />
                      <Select
                        style={{ width: 140 }}
                        placeholder="意图筛选"
                        allowClear
                        value={intentFilter || undefined}
                        onChange={(v) => setIntentFilter(v || '')}
                        options={[
                          { value: 'informational', label: '信息型' },
                          { value: 'transactional', label: '交易型' },
                          { value: 'local', label: '本地型' },
                        ]}
                      />
                    </Space>
                    <Button
                      type="primary"
                      loading={monitoringAll}
                      onClick={handleMonitorAll}
                      disabled={displayedKeywords.length === 0}
                    >
                      {monitoringAll ? '监测中...' : '一键监测全部'}
                    </Button>
                  </div>

                  {displayedKeywords.length === 0 ? (
                    <Empty description="暂无关键词，用其他 Tab 的蒸馏引擎生成" style={{ padding: 40 }} />
                  ) : (
                    <Table
                      dataSource={displayedKeywords}
                      columns={keywordColumns}
                      rowKey="id"
                      pagination={{ pageSize: 20, size: 'small' }}
                      size="small"
                      expandable={{
                        expandedRowRender: (record: Keyword) => {
                          const results = monitorByKeyword.get(record.id) || []
                          if (results.length === 0) {
                            return <Text type="secondary" style={{ fontSize: 13, padding: '8px 0', display: 'block' }}>暂无监测数据，点击「监测」按钮执行 AI 可见度检测</Text>
                          }
                          return (
                            <div style={{ padding: '4px 0' }}>
                              {/* 提及率趋势图（最后一点突出 = 变化点，展示"最新监测结果"）*/}
                              {results.length > 1 && (() => {
                                const trendData = markLastPoint(results.map((r: MonitoringResult) => ({
                                  date: new Date(r.probed_at).toLocaleDateString(),
                                  rate: Math.round((r.mention_rate || 0) * 1000) / 10,
                                  engine: r.engine_name || 'default',
                                })).sort((a, b) => a.date.localeCompare(b.date)))
                                return (
                                  <div style={{ marginBottom: 16, padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 10 }}>
                                    <Text strong style={{ fontSize: 13, marginBottom: 8, display: 'block' }}>提及率趋势</Text>
                                    <Line
                                      data={trendData}
                                      xField="date" yField="rate"
                                      seriesField="engine"
                                      smooth
                                      height={180}
                                      color={['#6366f1', '#0891b2', '#10b981', '#f59e0b']}
                                      point={{
                                        size: 3,
                                        shape: 'circle',
                                        style: {
                                          fill: (d: any) => d.is_last ? 'var(--wr-primary)' : 'transparent',
                                          stroke: (d: any) => d.is_last ? 'var(--wr-primary)' : '#6366f1',
                                          lineWidth: 2,
                                        },
                                      }}
                                      yAxis={{ label: { formatter: (v: string) => v + '%' } }}
                                    />
                                  </div>
                                )
                              })()}
                              <Text strong style={{ fontSize: 13, marginBottom: 8, display: 'block' }}>各 AI 引擎检测详情</Text>
                              {results.map((r: MonitoringResult) => (
                                <div key={r.id} style={{ marginBottom: 12, padding: 12, background: 'var(--wr-bg-elevated)', borderRadius: 10 }}>
                                  {/* 引擎标识 + 五维度 */}
                                  <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 8, flexWrap: 'wrap' }}>
                                    <Tag color="purple" style={{ margin: 0, minWidth: 70, textAlign: 'center' }}>{r.engine_name || 'default'}</Tag>
                                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                                      <Text type="secondary" style={{ fontSize: 11 }}>提及率</Text>
                                      <div style={{ width: 100 }}>
                                        <Progress percent={Math.round(r.mention_rate * 100)} size="small" strokeColor={rateColor(r.mention_rate)} format={() => `${(r.mention_rate * 100).toFixed(0)}%`} />
                                      </div>
                                    </div>
                                    <Text style={{ fontSize: 12, fontWeight: 600 }}>{r.avg_position > 0 ? `排名 #${r.avg_position}` : '未被提及'}</Text>
                                    <Tag style={{ margin: 0 }} color={r.sentiment === 'positive' ? 'success' : r.sentiment === 'negative' ? 'error' : 'default'}>
                                      {r.sentiment === 'positive' ? '正面' : r.sentiment === 'negative' ? '负面' : '中性'}
                                    </Tag>
                                    <Text type="secondary" style={{ fontSize: 11 }}>置信 {(r.confidence * 100).toFixed(0)}%</Text>
                                    {r.competitors && r.competitors.length > 0 && (
                                      <Text type="secondary" style={{ fontSize: 11 }}>竞品: {r.competitors.join('、')}</Text>
                                    )}
                                  </div>
                                  {/* AI 生成的回答内容 */}
                                  {r.raw_sample && (
                                    <div style={{ marginTop: 8, padding: 10, background: 'rgba(0,0,0,0.15)', borderRadius: 8 }}>
                                      <Text type="secondary" style={{ fontSize: 10, display: 'block', marginBottom: 4 }}>
                                        AI 回答内容（{r.mention_count}/{r.sample_count} 次采样中提到品牌）
                                      </Text>
                                      <Text style={{ whiteSpace: 'pre-wrap', fontSize: 12, color: 'var(--wr-text-secondary)', lineHeight: 1.6, display: 'block' }}>
                                        {r.raw_sample}
                                      </Text>
                                    </div>
                                  )}
                                </div>
                              ))}
                            </div>
                          )
                        },
                        rowExpandable: () => true,
                      }}
                    />
                  )}
                </div>
              ),
            },
            { key: 'brand', label: '品牌生成', children: sourcePanels.brand },
            { key: 'text', label: '文本蒸馏', children: sourcePanels.text },
            { key: 'seed', label: '种子拓展', children: sourcePanels.seed },
            { key: 'file', label: '文件读取', children: sourcePanels.file },
            { key: 'web', label: '网络获取', children: sourcePanels.web },
          ]}
        />

        {/* 蒸馏输入区域（非"我的关键词"Tab 才显示）*/}
        {activeSource !== 'mine' && (
          <>
            <Button type="primary" block size="large" loading={distilling} onClick={handleDistill} style={{ marginTop: 16 }}>
              {distilling ? 'AI 蒸馏中...' : '开始蒸馏关键词'}
            </Button>

            {distilling && (
              <div style={{ textAlign: 'center', padding: 40 }}>
                <Spin size="large" />
              </div>
            )}

            {!distilling && resultKeywords.length > 0 && (
              <div style={{ marginTop: 20, paddingTop: 20, borderTop: '1px solid var(--wr-border)' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                  <Text strong>蒸馏出 {resultKeywords.length} 个关键词</Text>
                  <Space>
                    <Button size="small" type="link" onClick={() => setCheckedKeywords(resultKeywords)}>全选</Button>
                    <Button size="small" type="link" onClick={() => setCheckedKeywords([])}>清空</Button>
                  </Space>
                </div>
                <Select style={{ width: '100%', marginBottom: 12 }} placeholder="选择要添加到哪个品牌" value={targetBrand} onChange={setTargetBrand}
                  options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))} />
                <Checkbox.Group value={checkedKeywords} onChange={(values) => setCheckedKeywords(values as string[])} style={{ width: '100%' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 280, overflowY: 'auto' }}>
                    {resultKeywords.map((kw) => (
                      <Checkbox key={kw} value={kw} style={{ marginLeft: 0 }}><Text style={{ fontSize: 14 }}>{kw}</Text></Checkbox>
                    ))}
                  </div>
                </Checkbox.Group>
                <Button type="primary" block style={{ marginTop: 12 }} disabled={checkedKeywords.length === 0 || !targetBrand} onClick={handleAddKeywords}>
                  添加 {checkedKeywords.length} 个关键词到品牌
                </Button>
              </div>
            )}
          </>
        )}
      </Card>
      </div>
    </div>
  )
}

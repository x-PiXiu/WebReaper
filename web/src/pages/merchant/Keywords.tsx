import { useState, useMemo, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Typography, Button, Table, Tag, Space, message, Input, Select, Upload, Checkbox, Empty, Spin, Popconfirm, Modal, Collapse } from 'antd'
import { UploadOutlined, RadarChartOutlined, ThunderboltOutlined, SettingOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import { mapWithConcurrency, settleSummary } from '../../utils/async'
import { intentLabel, questionIntentLabel } from '../../utils/geoTerms'
import { useBrandStore } from '../../store/brand'
import type { Brand, Keyword } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

type SourceType = 'text' | 'seed' | 'file' | 'web'

/**
 * 问题库（体检记录子层——去 Tab 化）：
 * 一张表 + 两个入口——「AI 推荐问题」（弹窗：出题勾选入库，覆盖 90% 商户）
 * 和「高级导入」（折叠：文本/种子/文件/网络，面向懂行用户）。
 * 问题 = 体检的题面 = 内容生成的选题，同一份词库三个用途。
 */
export default function Keywords({ embedded }: { embedded?: boolean }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  // AI 推荐弹窗
  const [recoOpen, setRecoOpen] = useState(false)
  const [recoLoading, setRecoLoading] = useState(false)
  const [recoAdding, setRecoAdding] = useState(false)
  const [recoList, setRecoList] = useState<string[]>([])
  const [recoChecked, setRecoChecked] = useState<string[]>([])
  const [recoBrand, setRecoBrand] = useState<string | undefined>(
    useBrandStore.getState().currentBrandId ?? undefined,
  )
  // 高级导入
  const [advSource, setAdvSource] = useState<SourceType>('text')
  const [distilling, setDistilling] = useState(false)
  const [resultKeywords, setResultKeywords] = useState<string[]>([])
  const [intentMap, setIntentMap] = useState<Record<string, string>>({})
  const [checkedKeywords, setCheckedKeywords] = useState<string[]>([])
  const [targetBrand, setTargetBrand] = useState<string | undefined>(
    useBrandStore.getState().currentBrandId ?? undefined,
  )
  // 表格筛选
  const [selectedBrand, setSelectedBrand] = useState<string | undefined>()
  const [textInput, setTextInput] = useState('')
  const [seedsInput, setSeedsInput] = useState('')
  const [topicInput, setTopicInput] = useState('')
  const [fileText, setFileText] = useState('')
  const [fileName, setFileName] = useState('')
  const [intentFilter, setIntentFilter] = useState('')

  const { data: brands = [] } = useQuery({ queryKey: ['geo-brands'], queryFn: () => businessApi.listBrands() })
  const { data: allKeywords = [], isLoading } = useQuery({ queryKey: ['geo-all-keywords'], queryFn: () => businessApi.listAllKeywords() })
  // 监测结果（"最近监测"列数据源——与 AI 体检共享缓存）
  const { data: monitorResults = [] } = useQuery({
    queryKey: ['geo-monitor-results'],
    queryFn: () => businessApi.getAllMonitorResults().catch(() => []),
  })
  const kwLatest = useMemo(() => {
    const m = new Map<string, { mention_rate?: number; probed_at?: string }>()
    ;[...monitorResults].sort((a, b) => new Date(b.probed_at).getTime() - new Date(a.probed_at).getTime())
      .forEach((r) => { if (!m.has(r.keyword_id)) m.set(r.keyword_id, r) })
    return m
  }, [monitorResults])
  const brandMap = new Map(brands.map((b: Brand) => [b.id, b.name]))

  const displayedKeywords = allKeywords.filter((k: Keyword) =>
    (!selectedBrand || k.brand_id === selectedBrand)
    && (!intentFilter || k.intent === intentFilter),
  )

  // ---- AI 推荐问题（弹窗流程：出题 → 勾选 → 入库） ----
  const genReco = async () => {
    if (!recoBrand) { message.warning('请先选择品牌'); return }
    setRecoLoading(true)
    try {
      const res = await businessApi.distillKeywords({ source: 'questions', brand_id: recoBrand } as never)
      const kws = res.keywords || []
      setRecoList(kws)
      setRecoChecked(kws)
      if (kws.length === 0) message.warning('没有生成出问题——试试先完善品牌资料')
    } catch { /* 拦截器已提示 */ } finally {
      setRecoLoading(false)
    }
  }

  const addReco = async () => {
    if (!recoBrand || recoChecked.length === 0) return
    setRecoAdding(true)
    try {
      const settled = await mapWithConcurrency(
        recoChecked,
        (term) => businessApi.addKeyword(recoBrand, { term, intent: 'informational' }),
        3,
      )
      const { ok, failed } = settleSummary(settled)
      if (ok > 0) {
        queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] })
        setRecoOpen(false)
        setRecoList([])
        setRecoChecked([])
        if (failed > 0) message.warning(`已添加 ${ok} 个，${failed} 个失败`)
        Modal.success({
          title: `已添加 ${ok} 个问题`,
          content: '问题已入库——去问问 AI，看看这些问题上 AI 怎么回答。',
          okText: '去问问 AI',
          cancelText: '留在本页',
          onOk: () => navigate('/m/checkup?tab=ask'),
        })
      } else {
        message.error('添加失败——请检查网络后重试')
      }
    } finally {
      setRecoAdding(false)
    }
  }

  // ---- 高级导入（折叠区：文本/种子/文件/网络） ----
  const handleDistill = async () => {
    setDistilling(true)
    setResultKeywords([])
    setCheckedKeywords([])
    try {
      const params: Record<string, unknown> = { source: advSource }
      if (advSource === 'text') {
        if (!textInput.trim()) { message.warning('请输入文本'); setDistilling(false); return }
        params.text = textInput
      } else if (advSource === 'seed') {
        const seeds = seedsInput.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean)
        if (seeds.length === 0) { message.warning('请输入种子词'); setDistilling(false); return }
        params.seeds = seeds
      } else if (advSource === 'web') {
        if (!topicInput.trim()) { message.warning('请输入主题'); setDistilling(false); return }
        params.topic = topicInput
      } else if (advSource === 'file') {
        if (!fileText.trim()) { message.warning('请先上传文件'); setDistilling(false); return }
        params.text = fileText
      }
      const res = await businessApi.distillKeywords(params as never)
      const kws = res.keywords || []
      setResultKeywords(kws)
      setIntentMap((res as { keyword_intents?: Record<string, string> }).keyword_intents || {})
      if (kws.length === 0) message.warning('未蒸馏出关键词')
    } catch { /* 拦截器已提示 */ } finally {
      setDistilling(false)
    }
  }

  const handleAddKeywords = async () => {
    if (!targetBrand) { message.warning('请先选择目标品牌'); return }
    if (checkedKeywords.length === 0) { message.warning('请至少勾选一个关键词'); return }
    // 受控并发（失败数显式汇总，已入库的不回滚但可重试）
    const settled = await mapWithConcurrency(
      checkedKeywords,
      (term) => businessApi.addKeyword(targetBrand, { term, intent: intentMap[term] || 'informational' }),
      3,
    )
    const { ok, failed } = settleSummary(settled)
    if (ok === 0) {
      message.error('添加失败——请检查网络后重试（已勾选词未入库）')
      return
    }
    queryClient.invalidateQueries({ queryKey: ['geo-all-keywords'] })
    setCheckedKeywords([])
    setResultKeywords([])
    if (failed > 0) {
      message.warning(`已添加 ${ok} 个，${failed} 个失败（可重新勾选后重试）`)
    } else {
      Modal.success({
        title: `已添加 ${ok} 个关键词`,
        content: '已入库——去问问 AI，看看这些词上 AI 会不会提到你。',
        okText: '去问问 AI',
        cancelText: '留在本页',
        onOk: () => navigate('/m/checkup?tab=ask'),
      })
    }
  }

  const handleFileUpload = (file: File) => {
    setFileName(file.name)
    const reader = new FileReader()
    reader.onload = (e) => {
      setFileText((e.target?.result as string) || '')
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
    } catch { /* interceptor */ }
  }

  const sourcePanels: Record<string, ReactNode> = {
    text: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>粘贴任意文本，AI 从中蒸馏核心关键词</Text>
        <TextArea rows={5} placeholder="粘贴文本内容..." value={textInput} onChange={(e) => setTextInput(e.target.value)} />
      </div>
    ),
    seed: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>输入种子词，AI 拓展出长尾关键词和问题词</Text>
        <TextArea rows={3} placeholder="用逗号分隔，如：agent开发, 源码解析" value={seedsInput} onChange={(e) => setSeedsInput(e.target.value)} />
      </div>
    ),
    file: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>上传 txt/md 文件，AI 读取内容后蒸馏关键词</Text>
        <Upload beforeUpload={handleFileUpload} accept=".txt,.md,.markdown" maxCount={1} showUploadList={false}>
          <Button icon={<UploadOutlined />}>选择文件</Button>
        </Upload>
        {fileName && (
          <div style={{ marginTop: 8 }}>
            <Tag color="orange">{fileName}</Tag>
            <Text type="secondary" style={{ fontSize: 12 }}>{fileText.length} 字符</Text>
          </div>
        )}
      </div>
    ),
    web: (
      <div>
        <Text type="secondary" style={{ display: 'block', marginBottom: 8, fontSize: 13 }}>输入主题，AI 爬取全网相关内容后蒸馏关键词</Text>
        <Input placeholder="如：agent 开发框架对比" value={topicInput} onChange={(e) => setTopicInput(e.target.value)} />
      </div>
    ),
  }

  const columns = [
    {
      title: '关键词', dataIndex: 'term', key: 'term',
      render: (t: string) => <Text strong>{t}</Text>,
    },
    {
      title: '品牌', key: 'brand', width: 140,
      render: (_: unknown, r: Keyword) => {
        const name = brandMap.get(r.brand_id)
        return name ? <Tag color="blue">{name}</Tag> : <Text type="secondary">-</Text>
      },
    },
    {
      title: '搜索意图', dataIndex: 'intent', key: 'intent', width: 110,
      render: (intent: string) => <Tag>{intentLabel(intent)}</Tag>,
    },
    {
      title: '最近体检', key: 'monitor', width: 140,
      render: (_: unknown, r: Keyword) => {
        const latest = kwLatest.get(r.id)
        if (!latest) return <Text type="secondary" style={{ fontSize: 12 }}>未测过</Text>
        const rate = Math.round((latest.mention_rate || 0) * 100)
        return (
          <Space size={6}>
            <Tag color={rate > 0 ? 'success' : 'default'} style={{ margin: 0, fontSize: 11 }}>
              {rate > 0 ? `提到 ${rate}%` : '未提及'}
            </Tag>
            <Text type="secondary" style={{ fontSize: 10 }}>{latest.probed_at?.slice(5, 10)}</Text>
          </Space>
        )
      },
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 170,
      render: (t: string) => (t ? new Date(t).toLocaleString('zh-CN', { hour12: false }) : '—'),
    },
    {
      title: '操作', key: 'action', width: 160,
      render: (_: unknown, r: Keyword) => (
        <Space size="small">
          <Button size="small" type="link" onClick={() => navigate(`/m/checkup?tab=ask&q=${encodeURIComponent(r.term)}`)}>去测一测</Button>
          <Popconfirm title="删除此关键词？" onConfirm={() => handleDelete(r.id)}>
            <Button size="small" type="text" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    // 嵌入体检记录子层时去掉页面级外壳（padding/极光光晕由父层统一提供，避免双层叠加）
    <div className={embedded ? '' : 'wr-page-content wr-aurora-bg'} style={{ position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* 嵌入体检记录子层时隐藏页头（父层已有标题） */}
        {!embedded && (
          <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap' }}>
            <div>
              <h1>问题库</h1>
              <p>顾客会搜什么，AI 就该在什么时候提到你——先让 AI 帮你想一批</p>
            </div>
            <Button type="primary" icon={<RadarChartOutlined />} onClick={() => navigate('/m/checkup?tab=ask')}>
              去问问 AI
            </Button>
          </div>
        )}

        <Card className="wr-glass-card" styles={{ body: { padding: 20 } }}>
          {/* 工具栏：AI 推荐入口 + 筛选 */}
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center', marginBottom: 16 }}>
            <Button type="primary" icon={<ThunderboltOutlined />} onClick={() => { setRecoOpen(true); if (recoList.length === 0) void genReco() }}>
              AI 推荐问题
            </Button>
            <Select
              style={{ width: 180 }}
              placeholder="按品牌筛选"
              allowClear
              value={selectedBrand}
              onChange={setSelectedBrand}
              options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
            />
            <Select
              style={{ width: 130 }}
              placeholder="意图筛选"
              allowClear
              value={intentFilter || undefined}
              onChange={(v) => setIntentFilter(v || '')}
              options={[
                { value: 'informational', label: intentLabel('informational') },
                { value: 'transactional', label: intentLabel('transactional') },
                { value: 'local', label: intentLabel('local') },
              ]}
            />
            <Text type="secondary" style={{ fontSize: 12, marginLeft: 'auto' }}>
              共 {allKeywords.length} 个问题/关键词
            </Text>
          </div>

          {displayedKeywords.length === 0 ? (
            <Empty description="问题库还是空的——让 AI 根据品牌资料推荐一批" style={{ padding: 40 }}>
              <Button type="primary" icon={<ThunderboltOutlined />} onClick={() => { setRecoOpen(true); void genReco() }}>让 AI 推荐问题</Button>
            </Empty>
          ) : (
            <Table
              loading={isLoading}
              dataSource={displayedKeywords}
              columns={columns}
              rowKey="id"
              pagination={{ pageSize: 20, size: 'small' }}
              size="small"
            />
          )}

          {/* 高级导入（折叠：面向懂行用户——手里有素材或想全网挖词） */}
          <Collapse
            ghost
            style={{ marginTop: 16 }}
            items={[{
              key: 'adv',
              label: <span style={{ fontSize: 13 }}><SettingOutlined style={{ marginRight: 6 }} />高级导入（文本 / 种子 / 文件 / 全网）</span>,
              children: (<>
                <Select
                  style={{ width: 240, marginBottom: 12 }}
                  value={advSource}
                  onChange={(v) => setAdvSource(v as SourceType)}
                  options={[
                    { value: 'text', label: '从粘贴的文本提取' },
                    { value: 'seed', label: '从几个种子词拓展' },
                    { value: 'file', label: '从 txt/md 文件读取' },
                    { value: 'web', label: '按主题爬取全网提取' },
                  ]}
                />
                {sourcePanels[advSource]}
                <Button type="primary" loading={distilling} onClick={handleDistill} style={{ marginTop: 12 }}>
                  {distilling ? 'AI 提取中...' : '开始提取'}
                </Button>
                {distilling && (
                  <div style={{ textAlign: 'center', padding: 32 }}><Spin size="large" /></div>
                )}
                {!distilling && resultKeywords.length > 0 && (
                  <div style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--wr-border)' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                      <Text strong>提取出 {resultKeywords.length} 个关键词</Text>
                      <Space>
                        <Button size="small" type="link" onClick={() => setCheckedKeywords(resultKeywords)}>全选</Button>
                        <Button size="small" type="link" onClick={() => setCheckedKeywords([])}>清空</Button>
                      </Space>
                    </div>
                    <Select
                      style={{ width: '100%', marginBottom: 12 }}
                      placeholder="选择要添加到哪个品牌"
                      value={targetBrand}
                      onChange={setTargetBrand}
                      options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
                    />
                    <Checkbox.Group value={checkedKeywords} onChange={(values) => setCheckedKeywords(values as string[])} style={{ width: '100%' }}>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 240, overflowY: 'auto' }}>
                        {resultKeywords.map((kw) => {
                          const it = intentMap[kw]
                          return (
                            <Checkbox key={kw} value={kw} style={{ marginLeft: 0 }}>
                              <Text style={{ fontSize: 14 }}>{kw}</Text>
                              {it && (
                                <Tag style={{ margin: '0 0 0 8px', fontSize: 11 }}
                                  color={it === 'recommendational' ? 'gold' : it === 'comparative' ? 'blue' : 'default'}>
                                  {questionIntentLabel(it)}
                                </Tag>
                              )}
                            </Checkbox>
                          )
                        })}
                      </div>
                    </Checkbox.Group>
                    <Button type="primary" block style={{ marginTop: 12 }} disabled={checkedKeywords.length === 0 || !targetBrand} onClick={handleAddKeywords}>
                      添加 {checkedKeywords.length} 个到问题库
                    </Button>
                  </div>
                )}
              </>),
            }]}
          />
        </Card>
      </div>

      {/* AI 推荐问题弹窗（出题 → 勾选 → 入库） */}
      <Modal
        title="AI 推荐问题"
        open={recoOpen}
        onCancel={() => setRecoOpen(false)}
        width={560}
        footer={
          <Space>
            <Button onClick={() => setRecoOpen(false)}>取消</Button>
            <Button onClick={genReco} loading={recoLoading}>重新出题</Button>
            <Button type="primary" loading={recoAdding} disabled={recoChecked.length === 0 || !recoBrand} onClick={addReco}>
              添加 {recoChecked.length} 个到问题库
            </Button>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 10, fontSize: 12.5 }}>
          AI 根据品牌资料生成顾客真实会问的问题——勾选后入库，可用于体检和内容选题。
        </Text>
        <Select
          style={{ width: '100%', marginBottom: 12 }}
          placeholder="基于哪个品牌出题"
          value={recoBrand}
          onChange={(v) => setRecoBrand(v)}
          options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
        />
        {recoLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin tip="AI 出题中..." /></div>
        ) : recoList.length === 0 ? (
          <Empty description="还没有出题——点「重新出题」" style={{ padding: 24 }} />
        ) : (
          <Checkbox.Group value={recoChecked} onChange={(values) => setRecoChecked(values as string[])} style={{ width: '100%' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 300, overflowY: 'auto' }}>
              {recoList.map((q) => (
                <Checkbox key={q} value={q} style={{ marginLeft: 0 }}>
                  <Text style={{ fontSize: 13.5 }}>{q}</Text>
                </Checkbox>
              ))}
            </div>
          </Checkbox.Group>
        )}
      </Modal>
    </div>
  )
}

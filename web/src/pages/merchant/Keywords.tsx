import { useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Typography, Button, Table, Tag, Space, message, Input, Select, Tabs, Upload, Checkbox, Empty, Spin, Popconfirm } from 'antd'
import { UploadOutlined, RadarChartOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand, Keyword } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input

type SourceType = 'mine' | 'brand' | 'text' | 'seed' | 'file' | 'web'

/**
 * 关键词工程：只管词库生产（蒸馏 / 导入 / 维护）。
 * 监测执行已并入「平台收录报表」。
 */
export default function Keywords() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [activeSource, setActiveSource] = useState<SourceType>('mine')
  const [distilling, setDistilling] = useState(false)
  const [resultKeywords, setResultKeywords] = useState<string[]>([])
  const [checkedKeywords, setCheckedKeywords] = useState<string[]>([])
  const [targetBrand, setTargetBrand] = useState<string | undefined>()
  const [selectedBrand, setSelectedBrand] = useState<string | undefined>()
  const [brandForSource, setBrandForSource] = useState<string | undefined>()
  const [textInput, setTextInput] = useState('')
  const [seedsInput, setSeedsInput] = useState('')
  const [topicInput, setTopicInput] = useState('')
  const [fileText, setFileText] = useState('')
  const [fileName, setFileName] = useState('')
  const [intentFilter, setIntentFilter] = useState('')

  const { data: brands = [] } = useQuery({ queryKey: ['geo-brands'], queryFn: () => businessApi.listBrands() })
  const { data: allKeywords = [], isLoading } = useQuery({ queryKey: ['geo-all-keywords'], queryFn: () => businessApi.listAllKeywords() })
  const brandMap = new Map(brands.map((b: Brand) => [b.id, b.name]))

  const displayedKeywords = allKeywords.filter((k: Keyword) =>
    (!selectedBrand || k.brand_id === selectedBrand)
    && (!intentFilter || k.intent === intentFilter),
  )

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
        const seeds = seedsInput.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean)
        if (seeds.length === 0) { message.warning('请输入种子词'); setDistilling(false); return }
        params.seeds = seeds
      } else if (activeSource === 'web') {
        if (!topicInput.trim()) { message.warning('请输入主题'); setDistilling(false); return }
        params.topic = topicInput
      } else if (activeSource === 'file') {
        if (!fileText.trim()) { message.warning('请先上传文件'); setDistilling(false); return }
        params.text = fileText
      }
      const res = await businessApi.distillKeywords(params as never)
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
      setActiveSource('mine')
    } catch { /* interceptor */ }
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
      title: '意图', dataIndex: 'intent', key: 'intent', width: 110,
      render: (intent: string) => {
        const map: Record<string, string> = { informational: '信息型', transactional: '交易型', local: '本地型' }
        return <Tag>{map[intent] || intent || '—'}</Tag>
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
          <Button size="small" type="link" onClick={() => navigate('/m/indexing-report')}>去收录监测</Button>
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
        <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap' }}>
          <div>
            <h1>关键词工程</h1>
            <p>多来源蒸馏与词库管理 · 监测请前往「平台收录报表」</p>
          </div>
          <Button type="primary" icon={<RadarChartOutlined />} onClick={() => navigate('/m/indexing-report')}>
            打开平台收录
          </Button>
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
                    <div style={{ marginBottom: 16, display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                      <Select
                        style={{ width: 200 }}
                        placeholder="按品牌筛选"
                        allowClear
                        value={selectedBrand}
                        onChange={setSelectedBrand}
                        options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
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
                    </div>
                    {displayedKeywords.length === 0 ? (
                      <Empty description="暂无关键词，用其他 Tab 蒸馏生成" style={{ padding: 40 }} />
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

          {activeSource !== 'mine' && (
            <>
              <Button type="primary" block size="large" loading={distilling} onClick={handleDistill} style={{ marginTop: 16 }}>
                {distilling ? 'AI 蒸馏中...' : '开始蒸馏关键词'}
              </Button>
              {distilling && (
                <div style={{ textAlign: 'center', padding: 40 }}><Spin size="large" /></div>
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
                  <Select
                    style={{ width: '100%', marginBottom: 12 }}
                    placeholder="选择要添加到哪个品牌"
                    value={targetBrand}
                    onChange={setTargetBrand}
                    options={brands.map((b: Brand) => ({ value: b.id, label: b.name }))}
                  />
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

import { useState } from 'react'
import { Typography, Button, Modal, Form, Input, Space, message, Popconfirm, Empty, Checkbox, Spin, Tag, Table, Input as AntInput } from 'antd'
import { PlusOutlined, DeleteOutlined, ThunderboltOutlined, TagOutlined, EnvironmentOutlined, BulbOutlined, SearchOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { Brand } from '../../types/api'

const { Title, Text } = Typography
const { TextArea } = Input

export default function Brands() {
  const queryClient = useQueryClient()
  const [brandModalOpen, setBrandModalOpen] = useState(false)
  const [selectedBrand, setSelectedBrand] = useState<Brand | null>(null)
  const [brandForm] = Form.useForm()
  const [kwForm] = Form.useForm()
  const [kwModalOpen, setKwModalOpen] = useState(false)
  const [genKwModalOpen, setGenKwModalOpen] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [generatedKeywords, setGeneratedKeywords] = useState<string[]>([])
  const [checkedKeywords, setCheckedKeywords] = useState<string[]>([])
  const [searchText, setSearchText] = useState('')

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const { data: keywords = [] } = useQuery({
    queryKey: ['geo-keywords', selectedBrand?.id],
    queryFn: () => businessApi.listKeywords(selectedBrand!.id),
    enabled: !!selectedBrand,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['geo-brands'] })
    queryClient.invalidateQueries({ queryKey: ['geo-overviews'] })
  }

  const handleCreateBrand = async (values: { name: string; positioning: string; core_selling: string; competitors: string }) => {
    try {
      await businessApi.createBrand({
        name: values.name,
        positioning: values.positioning,
        core_selling: values.core_selling ? values.core_selling.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean) : [],
        competitors: values.competitors ? values.competitors.split(/[,，\n]/).map((s) => s.trim()).filter(Boolean) : [],
      })
      message.success(`品牌「${values.name}」创建成功`)
      setBrandModalOpen(false)
      brandForm.resetFields()
      invalidate()
    } catch {}
  }

  const handleDeleteBrand = async (id: string) => {
    try {
      await businessApi.deleteBrand(id)
      message.success('已删除')
      if (selectedBrand?.id === id) setSelectedBrand(null)
      invalidate()
    } catch {}
  }

  const handleAddKeyword = async (values: { term: string; intent: string }) => {
    if (!selectedBrand) return
    try {
      await businessApi.addKeyword(selectedBrand.id, { term: values.term, intent: values.intent || 'informational' })
      message.success('关键词已添加')
      setKwModalOpen(false)
      kwForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['geo-keywords', selectedBrand.id] })
    } catch {}
  }

  const handleGenerateKeywords = async () => {
    if (!selectedBrand) return
    setGenerating(true)
    setGeneratedKeywords([])
    setCheckedKeywords([])
    setGenKwModalOpen(true)
    try {
      const res = await businessApi.generateKeywords(selectedBrand.id)
      const kws = Array.isArray(res) ? res : (res?.keywords || [])
      setGeneratedKeywords(kws)
      if (kws.length === 0) {
        message.warning('AI 未生成关键词，可能是 LLM 配置问题')
      }
    } catch (e) {
      message.error('生成关键词失败：' + ((e as Error)?.message || ''))
    } finally {
      setGenerating(false)
    }
  }

  const handleAddGeneratedKeywords = async () => {
    if (!selectedBrand || checkedKeywords.length === 0) {
      message.warning('请至少勾选一个关键词')
      return
    }
    try {
      for (const term of checkedKeywords) {
        await businessApi.addKeyword(selectedBrand.id, { term, intent: 'informational' })
      }
      message.success(`已添加 ${checkedKeywords.length} 个关键词`)
      setGenKwModalOpen(false)
      queryClient.invalidateQueries({ queryKey: ['geo-keywords', selectedBrand.id] })
    } catch {}
  }

  const filteredBrands = brands.filter((b: Brand) =>
    !searchText || b.name.toLowerCase().includes(searchText.toLowerCase()) ||
    (b.positioning || '').toLowerCase().includes(searchText.toLowerCase())
  )

  const kwColumns = [
    { title: '关键词', dataIndex: 'term', key: 'term', render: (t: string) => <Text strong>{t}</Text> },
    { title: '意图', dataIndex: 'intent', key: 'intent', width: 120, render: (i: string) => <Tag color="cyan">{i || '-'}</Tag> },
  ]

  return (
    <div className="wr-page-content" style={{ paddingTop: 0 }}>
      {/* 页面标题区（Linear 式）*/}
      <div className="wr-page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h1>品牌管理</h1>
          <p>管理品牌资产与关键词，AI 据此监测可见度并生成优化内容</p>
        </div>
        <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setBrandModalOpen(true)}>
          创建品牌
        </Button>
      </div>

      {/* 左右分栏：品牌列表 + 详情面板（玻璃卡片）*/}
      <div style={{ display: 'flex', gap: 16, minHeight: 'calc(100vh - 200px)' }}>
        {/* 左：品牌列表 */}
        <div className="wr-glass-card" style={{ width: 340, flexShrink: 0, display: 'flex', flexDirection: 'column', gap: 8, padding: 16 }}>
          {/* 搜索框 */}
          <AntInput
            prefix={<SearchOutlined style={{ color: 'var(--wr-text-muted)' }} />}
            placeholder="搜索品牌名或定位"
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            allowClear
            style={{ marginBottom: 4 }}
          />

          {/* 品牌列表 */}
          <div style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column', gap: 6 }}>
            {filteredBrands.length === 0 ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={searchText ? '未找到匹配品牌' : '暂无品牌'}
                style={{ padding: 40 }}
              >
                {!searchText && (
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => setBrandModalOpen(true)}>
                    创建第一个品牌
                  </Button>
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
                      padding: '14px 16px',
                      borderRadius: 10,
                      cursor: 'pointer',
                      background: isSelected ? 'var(--wr-primary-bg)' : 'var(--wr-bg-surface)',
                      border: `1px solid ${isSelected ? 'var(--wr-primary)' : 'var(--wr-border)'}`,
                      transition: 'all 200ms cubic-bezier(0.2, 0, 0, 1)',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
                      <Text strong style={{ fontSize: 15, color: isSelected ? 'var(--wr-primary-hover)' : 'var(--wr-text-primary)' }}>
                        {brand.name}
                      </Text>
                      <Popconfirm title="删除品牌及其关键词？" onConfirm={() => handleDeleteBrand(brand.id)}>
                        <Button
                          size="small"
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          onClick={(e) => e.stopPropagation()}
                          style={{ opacity: 0.5 }}
                        />
                      </Popconfirm>
                    </div>
                    {brand.positioning && (
                      <Text type="secondary" style={{ fontSize: 12, lineHeight: 1.5, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {brand.positioning}
                      </Text>
                    )}
                    <div style={{ display: 'flex', gap: 12, marginTop: 6 }}>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        <TagOutlined style={{ marginRight: 3 }} />
                        {brand.core_selling?.length || 0} 卖点
                      </Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        <EnvironmentOutlined style={{ marginRight: 3 }} />
                        {brand.competitors?.length || 0} 竞品
                      </Text>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>

        {/* 右：品牌详情 + 关键词管理 */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {selectedBrand ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* 品牌信息区（玻璃卡片）*/}
              <div className="wr-glass-card" style={{ padding: 24 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
                  <Title level={4} style={{ margin: 0, fontSize: 22 }}>{selectedBrand.name}</Title>
                  <Button
                    size="small"
                    type="text"
                    onClick={() => setSelectedBrand(null)}
                  >
                    关闭
                  </Button>
                </div>

                {selectedBrand.positioning && (
                  <div style={{ marginBottom: 16 }}>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
                      <BulbOutlined /> 品牌定位
                    </Text>
                    <Text style={{ fontSize: 14, lineHeight: 1.6 }}>{selectedBrand.positioning}</Text>
                  </div>
                )}

                {selectedBrand.core_selling && selectedBrand.core_selling.length > 0 && (
                  <div style={{ marginBottom: 16 }}>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>
                      <TagOutlined /> 核心卖点
                    </Text>
                    <Space size={6} wrap>
                      {selectedBrand.core_selling.map((s, i) => (
                        <Tag key={i} color="blue">{s}</Tag>
                      ))}
                    </Space>
                  </div>
                )}

                {selectedBrand.competitors && selectedBrand.competitors.length > 0 && (
                  <div>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>
                      <EnvironmentOutlined /> 竞品
                    </Text>
                    <Space size={6} wrap>
                      {selectedBrand.competitors.map((c, i) => (
                        <Tag key={i}>{c}</Tag>
                      ))}
                    </Space>
                  </div>
                )}
              </div>

              {/* 关键词管理区（玻璃卡片）*/}
              <div className="wr-glass-card" style={{ padding: 24, flex: 1 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                  <Space>
                    <Text strong style={{ fontSize: 16 }}>关键词</Text>
                    <Tag color="blue">{keywords.length}</Tag>
                  </Space>
                  <Space>
                    <Button size="small" icon={<PlusOutlined />} onClick={() => setKwModalOpen(true)}>手动添加</Button>
                    <Button size="small" type="primary" icon={<ThunderboltOutlined />} onClick={handleGenerateKeywords}>AI 生成</Button>
                  </Space>
                </div>

                <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 13 }}>
                  添加用户可能搜索的关键词，AI 会据此监测品牌可见度并生成优化内容
                </Text>

                {keywords.length === 0 ? (
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description="暂无关键词"
                    style={{ padding: 24 }}
                  >
                    <Button size="small" type="primary" icon={<ThunderboltOutlined />} onClick={handleGenerateKeywords}>
                      AI 生成关键词
                    </Button>
                  </Empty>
                ) : (
                  <Table dataSource={keywords} columns={kwColumns} rowKey="id" pagination={{ pageSize: 10, size: 'small' }} size="small" />
                )}
              </div>
            </div>
          ) : (
            <div className="wr-glass-card" style={{
              height: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}>
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="选择左侧品牌查看详情和管理关键词"
              />
            </div>
          )}
        </div>
      </div>

      {/* 创建品牌弹窗 */}
      <Modal title="创建品牌" open={brandModalOpen} onCancel={() => setBrandModalOpen(false)} footer={null} width={560}>
        <Form form={brandForm} layout="vertical" onFinish={handleCreateBrand} requiredMark={false}>
          <Form.Item label="品牌名" name="name" rules={[{ required: true, message: '请输入品牌名' }]}>
            <Input placeholder="如 某装修公司" />
          </Form.Item>
          <Form.Item label="品牌定位" name="positioning" tooltip="描述品牌的核心价值，AI 生成内容时会参考">
            <TextArea placeholder="如 专注北京地区中高端家装，提供设计-施工-软装一站式服务" autoSize={{ minRows: 2, maxRows: 4 }} />
          </Form.Item>
          <Form.Item label="核心卖点" name="core_selling" tooltip="用逗号分隔，如：10年经验,环保材料,终身保修">
            <TextArea placeholder="10年经验, 环保材料, 终身保修" autoSize={{ minRows: 2 }} />
          </Form.Item>
          <Form.Item label="竞品" name="competitors" tooltip="用逗号分隔，监测时会对比这些竞品的 AI 可见度">
            <Input placeholder="竞品A, 竞品B, 竞品C" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">创建</Button>
              <Button onClick={() => setBrandModalOpen(false)}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 添加关键词弹窗 */}
      <Modal title="添加关键词" open={kwModalOpen} onCancel={() => setKwModalOpen(false)} footer={null} width={480}>
        <Form form={kwForm} layout="vertical" onFinish={handleAddKeyword} requiredMark={false}>
          <Form.Item label="关键词/搜索词" name="term" rules={[{ required: true, message: '请输入关键词' }]}
            tooltip="用户可能搜的词，如「北京装修公司哪家好」">
            <Input placeholder="北京装修公司哪家好" />
          </Form.Item>
          <Form.Item label="搜索意图" name="intent" tooltip="informational=了解信息, transactional=想交易, local=找本地服务">
            <Input placeholder="informational / transactional / local" defaultValue="informational" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">添加</Button>
              <Button onClick={() => setKwModalOpen(false)}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* AI 生成关键词弹窗 */}
      <Modal
        title="AI 生成关键词"
        open={genKwModalOpen}
        onCancel={() => setGenKwModalOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setGenKwModalOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleAddGeneratedKeywords} disabled={checkedKeywords.length === 0}>
              添加勾选的 {checkedKeywords.length} 个
            </Button>
          </Space>
        }
        width={520}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 13 }}>
          根据品牌「{selectedBrand?.name}」的定位和核心卖点，AI 生成的候选关键词。勾选要添加的。
        </Text>
        {generating ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin tip="AI 正在生成候选关键词..." />
          </div>
        ) : generatedKeywords.length === 0 ? (
          <Empty description="未能生成关键词，请手动添加" />
        ) : (
          <Checkbox.Group
            value={checkedKeywords}
            onChange={(values) => setCheckedKeywords(values as string[])}
            style={{ width: '100%' }}
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {generatedKeywords.map((kw) => (
                <Checkbox key={kw} value={kw} style={{ marginLeft: 0 }}>
                  <Text>{kw}</Text>
                </Checkbox>
              ))}
            </div>
          </Checkbox.Group>
        )}
        {generatedKeywords.length > 0 && (
          <div style={{ marginTop: 12, paddingTop: 12, borderTop: '1px solid var(--wr-border)' }}>
            <Button size="small" type="link" onClick={() => setCheckedKeywords(generatedKeywords)}>全选</Button>
            <Button size="small" type="link" onClick={() => setCheckedKeywords([])}>清空</Button>
          </div>
        )}
      </Modal>
    </div>
  )
}

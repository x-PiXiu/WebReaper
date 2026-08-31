import { useEffect, useState } from 'react'
import { Card, Typography, Button, Input, Form, Table, Tag, Space, Popconfirm, Alert, Select, InputNumber, Row, Col, Tooltip } from 'antd'
import { SaveOutlined, ReloadOutlined, DeleteOutlined, DatabaseOutlined, LinkOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { IndustryCrawlConfig, KnowledgeMaterialView } from '../../types/api'
import { message } from '../../utils/antdApp'

const { Text } = Typography

// 平台知识库：向量嵌入/向量库配置（30s 生效免重启）· 行业采集配置 · 素材管理与向量重建。
// 背景（Docs/Plans/04）：平台按行业持续采集网页素材（保留来源 URL+原文）→ 生成前按
// "品牌行业+关键词"向量检索素材（带来源）→ 规格化 prompt → 上游 LLM 生成。
export default function Knowledge({ embedded: _embedded = false }: { embedded?: boolean }) {
  void _embedded
  const queryClient = useQueryClient()
  const [embForm] = Form.useForm()
  const [crawlForm] = Form.useForm()
  const [savingEmb, setSavingEmb] = useState(false)
  const [savingCrawl, setSavingCrawl] = useState(false)
  const [reindexing, setReindexing] = useState(false)
  const [reindexResult, setReindexResult] = useState<{ processed: number; updated: number; failed: number } | null>(null)
  const [materialPage, setMaterialPage] = useState(1)
  const [industryFilter, setIndustryFilter] = useState('')
  const PAGE_SIZE = 20

  // 向量配置
  const { data: embConfig, isLoading: embLoading } = useQuery({
    queryKey: ['knowledge-emb-config'],
    queryFn: () => businessApi.getKnowledgeEmbeddingConfig(),
  })

  // 行业采集配置（后端未配置时可能返回 null——统一兜底为空数组）
  const { data: crawlConfigs } = useQuery({
    queryKey: ['knowledge-crawl-config'],
    queryFn: () => businessApi.getKnowledgeCrawlConfig(),
  })
  const cfgList = crawlConfigs ?? []

  // 采集间隔（分钟，30-1440；管理后台可改，下个周期生效免重启）
  const { data: intervalCfg } = useQuery({
    queryKey: ['knowledge-crawl-interval'],
    queryFn: () => businessApi.getKnowledgeCrawlInterval(),
  })
  useEffect(() => {
    if (intervalCfg) {
      crawlForm.setFieldsValue({ crawl_interval: intervalCfg.interval_minutes })
    }
  }, [intervalCfg, crawlForm])

  // 素材统计 + 列表
  const { data: stats } = useQuery({
    queryKey: ['knowledge-stats'],
    queryFn: () => businessApi.getKnowledgeStats(),
    refetchInterval: 30000,
  })
  const { data: materials = [], isLoading: materialsLoading } = useQuery({
    queryKey: ['knowledge-materials', materialPage, industryFilter],
    queryFn: () =>
      businessApi.listKnowledgeMaterials({
        industry: industryFilter || undefined,
        limit: PAGE_SIZE,
        offset: (materialPage - 1) * PAGE_SIZE,
      }),
  })

  // 配置加载后回填表单（首次进入编辑态显示当前值）
  useEffect(() => {
    if (embConfig) {
      embForm.setFieldsValue({
        model: embConfig.model,
        base_url: embConfig.base_url,
        api_key: embConfig.api_key,
        dimensions: embConfig.dimensions || 0,
        vector_db: embConfig.vector_db || 'mysql',
        milvus_host: embConfig.milvus_host,
        milvus_port: embConfig.milvus_port,
        milvus_collection: embConfig.milvus_collection,
      })
    }
  }, [embConfig, embForm])
  useEffect(() => {
    if (cfgList.length > 0) {
      crawlForm.setFieldsValue({
        industries: cfgList.map((c) => ({
          industry: c.industry,
          keywords: c.keywords.join(', '),
          per_round: c.per_round,
        })),
      })
    }
  }, [cfgList, crawlForm])

  // 保存向量配置
  const handleSaveEmbedding = async () => {
    setSavingEmb(true)
    try {
      const v = await embForm.validateFields()
      const r = await businessApi.updateKnowledgeEmbeddingConfig({
        model: v.model || '',
        base_url: v.base_url || '',
        api_key: v.api_key || '',
        dimensions: v.dimensions || 0,
        vector_db: v.vector_db || 'mysql',
        milvus_host: v.milvus_host || '',
        milvus_port: v.milvus_port || '',
        milvus_collection: v.milvus_collection || '',
      })
      message.success(r.note || '向量配置已保存，30 秒内生效（无需重启）')
      queryClient.invalidateQueries({ queryKey: ['knowledge-emb-config'] })
    } catch (e) {
      message.error('保存失败：' + ((e as Error)?.message || '请检查配置格式'))
    } finally {
      setSavingEmb(false)
    }
  }

  // 保存行业采集配置
  const handleSaveCrawl = async () => {
    setSavingCrawl(true)
    try {
      const v = await crawlForm.validateFields()
      const cfgs: IndustryCrawlConfig[] = (v.industries || []).map((row: { industry: string; keywords: string; per_round: number }) => ({
        industry: row.industry,
        keywords: (row.keywords || '').split(/[,，]/).map((s: string) => s.trim()).filter(Boolean),
        per_round: row.per_round || 10,
      }))
      await businessApi.updateKnowledgeCrawlConfig(cfgs)
      if (v.crawl_interval) {
        await businessApi.updateKnowledgeCrawlInterval(v.crawl_interval)
      }
      message.success('行业采集配置已保存，下一轮采集任务生效')
      queryClient.invalidateQueries({ queryKey: ['knowledge-crawl-config'] })
      queryClient.invalidateQueries({ queryKey: ['knowledge-crawl-interval'] })
    } catch (e) {
      message.error('保存失败：' + ((e as Error)?.message || '请检查配置格式'))
    } finally {
      setSavingCrawl(false)
    }
  }

  // 重建向量（换 embedding 模型后存量向量失效——重建恢复检索正确性）
  const handleReindex = async (onlyMissing: boolean) => {
    setReindexing(true)
    try {
      const r = await businessApi.reindexKnowledgeMaterials({ only_missing: onlyMissing })
      setReindexResult({ processed: r.processed, updated: r.updated, failed: r.failed })
      message.success(`向量重建完成：处理 ${r.processed} 条，更新 ${r.updated} 条${r.failed ? `，失败 ${r.failed} 条` : ''}`)
    } catch (e) {
      message.error('重建失败：' + ((e as Error)?.message || ''))
    } finally {
      setReindexing(false)
    }
  }

  // 删除素材
  const handleDelete = async (id: string) => {
    try {
      await businessApi.deleteKnowledgeMaterial(id)
      message.success('素材已删除（含向量）')
      queryClient.invalidateQueries({ queryKey: ['knowledge-materials'] })
      queryClient.invalidateQueries({ queryKey: ['knowledge-stats'] })
    } catch (e) {
      message.error('删除失败：' + ((e as Error)?.message || ''))
    }
  }

  const materialColumns = [
    {
      title: '标题', dataIndex: 'title', key: 'title', ellipsis: true,
      render: (t: string, r: KnowledgeMaterialView) => (
        <Tooltip title={t || r.source_url}>
          <a href={r.source_url} target="_blank" rel="noreferrer">
            {t || r.source_url} <LinkOutlined style={{ fontSize: 11 }} />
          </a>
        </Tooltip>
      ),
    },
    { title: '行业', dataIndex: 'industry', key: 'industry', width: 90, render: (i: string) => <Tag color="geekblue">{i}</Tag> },
    { title: '采集关键词', dataIndex: 'crawl_keyword', key: 'crawl_keyword', width: 140, ellipsis: true },
    {
      title: '向量', dataIndex: 'has_vector', key: 'has_vector', width: 80,
      render: (v: boolean) => (v ? <Tag color="green">已索引</Tag> : <Tag color="orange">待向量</Tag>),
    },
    {
      title: '入库时间', dataIndex: 'created_at', key: 'created_at', width: 160,
      render: (t: string) => (t ? new Date(t).toLocaleString() : '-'),
    },
    {
      title: '操作', key: 'action', width: 80,
      render: (_: unknown, r: KnowledgeMaterialView) => (
        <Popconfirm title="确认删除该素材？" onConfirm={() => handleDelete(r.id)}>
          <Button type="text" danger size="small" icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 标题区（统一 wr-page-header 规范） */}
      <div className="wr-page-header">
        <h1>知识库</h1>
        <p>向量嵌入与向量库配置 · 行业采集 · 素材管理——生成内容时的参考资料来源</p>
      </div>

      {/* ① 向量嵌入/向量库配置 */}
      <Card
        title={
          <Space><DatabaseOutlined />向量嵌入与向量库（管理后台动态修改，30 秒生效免重启）</Space>
        }
        loading={embLoading}
        extra={<Text type="secondary" style={{ fontSize: 12 }}>缺省复用 LLM 配置（EMBEDDING_* env 兜底）</Text>}
      >
        <Form
          form={embForm}
          layout="vertical"
        >
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="嵌入模型" name="model" tooltip="如 embedding-3（智谱）；更换后需重建存量向量">
                <Input placeholder="embedding-3" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="API 端点（OpenAI 兼容）" name="base_url">
                <Input placeholder="https://open.bigmodel.cn/api/paas/v4" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="API Key" name="api_key">
                <Input.Password placeholder="嵌入 API Key（留空沿用现有）" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={6}>
              <Form.Item label="向量维度" name="dimensions" tooltip="0=模型默认（智谱 embedding-3 默认 2048，可设 256-2048）；修改后需重建向量">
                <InputNumber min={0} max={4096} style={{ width: '100%' }} placeholder="0（模型默认）" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item label="向量库" name="vector_db" tooltip="mysql=内置（默认）；milvus=已接入驱动">
                <Select
                  options={[
                    { value: 'mysql', label: 'MySQL（内置，零依赖）' },
                    { value: 'milvus', label: 'Milvus（已接入）' },
                  ]}
                />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item label="Milvus Host" name="milvus_host">
                <Input placeholder="如 10.0.0.1（milvus 模式必填）" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item label="Milvus Port" name="milvus_port">
                <Input placeholder="19530" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item label="Milvus Collection" name="milvus_collection">
                <Input placeholder="kb_materials" />
              </Form.Item>
            </Col>
          </Row>
          <Space>
            <Button type="primary" icon={<SaveOutlined />} loading={savingEmb} onClick={handleSaveEmbedding}>
              保存向量配置
            </Button>
            <Button icon={<ReloadOutlined />} loading={reindexing} onClick={() => handleReindex(false)}>
              全量重建向量
            </Button>
            <Button icon={<ReloadOutlined />} loading={reindexing} onClick={() => handleReindex(true)}>
              增量补向量（仅无向量）
            </Button>
          </Space>
          {reindexResult && (
            <Alert
              style={{ marginTop: 12 }}
              type={reindexResult.failed > 0 ? 'warning' : 'success'}
              showIcon
              message={`重建完成：处理 ${reindexResult.processed} 条，更新 ${reindexResult.updated} 条，失败 ${reindexResult.failed} 条`}
            />
          )}
          <Alert
            style={{ marginTop: 12 }}
            type="info"
            showIcon
            message="换 embedding 模型后，存量素材的向量是旧模型产物（检索会错乱）——请点击「全量重建向量」恢复；「增量补向量」用于补齐采集时向量化失败的素材。"
          />
        </Form>
      </Card>

      {/* ② 行业采集配置 */}
      <Card title="行业采集配置（平台按行业持续采集素材入库）">
        <Form form={crawlForm} layout="vertical">
          <Form.Item
            label="采集间隔（分钟）"
            name="crawl_interval"
            tooltip="自动采集周期：30-1440 分钟（0.5h-24h）。冷启动可调短密集采集，稳定后调长；修改后下个周期生效（免重启）"
            rules={[{ required: true, message: '请输入采集间隔' }]}
          >
            <InputNumber min={30} max={1440} style={{ width: 240 }} addonAfter="分钟（默认 360 = 6 小时）" />
          </Form.Item>
          <Form.List name="industries">
            {(fields, { add, remove }) => (
              <>
                {fields.map((field, idx) => (
                  <Row key={field.key} gutter={12} align="top">
                    <Col span={5}>
                      <Form.Item
                        {...field}
                        label={idx === 0 ? '行业' : ' '}
                        name={[field.name, 'industry']}
                        rules={[{ required: true, message: '行业名' }]}
                      >
                        <Input placeholder="如 餐饮" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item
                        {...field}
                        label={idx === 0 ? '采集关键词（逗号分隔）' : ' '}
                        name={[field.name, 'keywords']}
                        rules={[{ required: true, message: '至少一个关键词' }]}
                      >
                        <Input placeholder="餐饮营销, 餐厅获客, 餐饮行业趋势" />
                      </Form.Item>
                    </Col>
                    <Col span={4}>
                      <Form.Item
                        {...field}
                        label={idx === 0 ? '每轮上限' : ' '}
                        name={[field.name, 'per_round']}
                      >
                        <InputNumber min={1} max={50} style={{ width: '100%' }} placeholder="10" />
                      </Form.Item>
                    </Col>
                    <Col span={3}>
                      <Form.Item label={idx === 0 ? ' ' : ' '}>
                        <Button type="text" danger onClick={() => remove(field.name)}>移除</Button>
                      </Form.Item>
                    </Col>
                  </Row>
                ))}
                <Button type="dashed" onClick={() => add({ industry: '', keywords: '', per_round: 10 })} block>
                  + 添加行业采集配置
                </Button>
              </>
            )}
          </Form.List>
          <Space style={{ marginTop: 12 }}>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={savingCrawl}
              onClick={handleSaveCrawl}
            >
              保存行业采集配置
            </Button>
            <Text type="secondary" style={{ fontSize: 12 }}>
              保存后下一轮采集任务（每 6 小时）按新配置执行；素材总数：{stats?.total_materials ?? '-'}
            </Text>
          </Space>
        </Form>
      </Card>

      {/* ③ 素材管理 */}
      <Card
        title={
          <Space>
            知识库素材
            <Select
              allowClear
              placeholder="按行业过滤"
              style={{ width: 160 }}
              value={industryFilter || undefined}
              onChange={(v) => { setIndustryFilter(v || ''); setMaterialPage(1) }}
              options={Array.from(new Set(materials.map((m) => m.industry).concat(industryFilter ? [industryFilter] : []))).map((i) => ({ value: i, label: i }))}
            />
          </Space>
        }
        extra={<Text type="secondary" style={{ fontSize: 12 }}>来源可溯源：生成内容时素材带来源 URL 注入</Text>}
      >
        <Table
          rowKey="id"
          size="small"
          loading={materialsLoading}
          columns={materialColumns}
          dataSource={materials}
          pagination={{
            current: materialPage,
            pageSize: PAGE_SIZE,
            total: stats?.total_materials ?? 0,
            showSizeChanger: false,
            onChange: setMaterialPage,
          }}
        />
      </Card>
    </div>
  )
}

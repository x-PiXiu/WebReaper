import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, Typography, Tag, Space, Button, Descriptions, Switch, Input, Select, Alert, Empty, Tabs, Collapse, Table, Form, InputNumber, Modal, Popconfirm, Tooltip } from 'antd'
import { message } from '../../utils/antdApp'
import QueryBoundary from '../../components/QueryBoundary'
import {
  CloudServerOutlined, RobotOutlined, AudioOutlined, SearchOutlined, WalletOutlined,
  ReloadOutlined, SaveOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../api/business'
import type { IntegrationVendor, IntegrationCapability } from '../../types/api'

const { Text } = Typography

// 能力 ID → 中文名
const CAP_LABELS: Record<string, string> = {
  'asr': '语音识别', 'llm-chat': 'AI 对话', 'llm-vision': '视觉模型',
  'tts': '语音合成', 'tts-design': '音色设计', 'voice-clone': '声音克隆',
  'video-gen': '视频生成', 'search': '搜索', 'payment': '支付',
}

// 能力类型选项（新增能力下拉）
const CAP_OPTIONS = [
  { value: 'asr', label: '语音识别（ASR）' },
  { value: 'llm-chat', label: 'AI 对话（LLM）' },
  { value: 'llm-vision', label: '视觉模型（LLM Vision）' },
  { value: 'tts', label: '语音合成（TTS）' },
  { value: 'tts-design', label: '音色设计（TTS Design）' },
  { value: 'voice-clone', label: '声音克隆（Voice Clone）' },
  { value: 'search', label: '搜索' },
  { value: 'payment', label: '支付' },
]

// Vidu 模式中文名（模型管理面板的模式开关区）
const MODE_LABELS: Record<string, string> = {
  text2video: '文生视频', img2video: '图生视频', start_end2video: '首尾帧',
  reference2video: '参考生视频', multiframe: '智能多帧', digital_human: '数字人口播',
  text2image: '文生图', text2audio: '音乐生成', sound_effect: '音效生成',
  tts: '语音合成', voice_clone: '声音克隆', lip_sync: '对口型',
  subject: '主体创建', template: '模板成片',
}

/**
 * 第三方集成中心（08 计划 D7——统一视图）：
 * 厂商卡片网格 + 点击展开详情（Key/启停/能力路由/首选模型/音色库）。
 * 一个页面，无 tab 切换——按厂商组织，能力路由内联展示。
 */
export default function IntegrationsPage({ embedded = false }: { embedded?: boolean }) {
  const { id } = useParams<{ id: string }>()
  if (id) return <IntegrationDetail id={id} />
  return <IntegrationCenter embedded={embedded} />
}

// ---- 主页面：厂商卡片 + 内联详情 ----
function IntegrationCenter({ embedded = false }: { embedded?: boolean }) {
  const [expandedVendor, setExpandedVendor] = useState<string | null>(null)

  const { data: vendorsData, isLoading: vendorsLoading, isError: vendorsError, refetch: refetchVendors } = useQuery({
    queryKey: ['admin-integration-vendors'],
    queryFn: () => businessApi.listIntegrationVendors(),
  })

  const { data: capsData } = useQuery({
    queryKey: ['admin-integration-capabilities'],
    queryFn: () => businessApi.listIntegrationCapabilities(),
  })

  const vendors = vendorsData?.vendors || []
  const caps = capsData?.capabilities || []

  // 按 vendor 分组能力
  const capsByVendor = new Map<string, IntegrationCapability[]>()
  for (const c of caps) {
    if (!capsByVendor.has(c.vendor_id)) capsByVendor.set(c.vendor_id, [])
    capsByVendor.get(c.vendor_id)!.push(c)
  }

  // 按 cap_id 分组（能力路由视图）
  const capsByType = new Map<string, IntegrationCapability[]>()
  for (const c of caps) {
    if (!capsByType.has(c.cap_id)) capsByType.set(c.cap_id, [])
    capsByType.get(c.cap_id)!.push(c)
  }

  return (
    <div className={embedded ? '' : 'wr-page-content'} style={embedded ? undefined : { paddingTop: 8 }}>
      <div className="wr-page-header" style={{ marginBottom: 16 }}>
        <h1>第三方集成</h1>
        <p>厂商配置 + 能力路由——点击厂商卡片展开详情，切换默认 ≤10s 生效</p>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="商户口播向导依赖项"
        description="商户「拍口播」与「快速生成」需要 Vidu 对口型、语音合成、参考生视频等模式已启用且积分充足。未配置时商户入口会显示能力不可用提示。"
      />

      <QueryBoundary loading={vendorsLoading} error={vendorsError} onRetry={() => refetchVendors()}>
        <>
          {/* 厂商卡片网格 */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 14, marginBottom: 24 }}>
            {vendors.map(v => {
              const vendorCaps = capsByVendor.get(v.id) || []
              const hasKey = !!v.api_key
              const isExpanded = expandedVendor === v.id
              return (
                <Card
                  key={v.id}
                  hoverable
                  onClick={() => setExpandedVendor(isExpanded ? null : v.id)}
                  style={{ cursor: 'pointer', borderColor: isExpanded ? 'var(--wr-primary)' : undefined }}
                  styles={{ body: { padding: '16px 20px' } }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 10 }}>
                    <span style={{ fontSize: 22, color: 'var(--wr-primary)' }}>
                      {v.protocol === 'vidu' ? <CloudServerOutlined /> :
                       v.protocol === 'openai-chat' ? <RobotOutlined /> :
                       v.id.includes('asr') ? <AudioOutlined /> :
                       v.id.includes('search') ? <SearchOutlined /> :
                       v.id.includes('pay') ? <WalletOutlined /> : <CloudServerOutlined />}
                    </span>
                    <div style={{ flex: 1 }}>
                      <Text strong>{v.name || v.id}</Text>
                      <Tag color={v.protocol === 'vidu' ? 'geekblue' : 'blue'} style={{ marginLeft: 8, fontSize: 11 }}>{v.protocol}</Tag>
                    </div>
                    <Tag color={v.enabled && hasKey ? 'success' : v.enabled ? 'warning' : 'default'}>
                      {v.enabled && hasKey ? '正常' : v.enabled ? '缺 Key' : '未启用'}
                    </Tag>
                  </div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>{v.base_url || '—'}</Text>
                  <Space size={4} wrap>
                    {vendorCaps.map(c => (
                      <Tag key={c.id} style={{ margin: 0, fontSize: 11 }} color={c.is_default ? 'green' : 'default'}>
                        {CAP_LABELS[c.cap_id] || c.cap_id}{c.is_default ? ' ✓' : ''}
                      </Tag>
                    ))}
                    {vendorCaps.length === 0 && <Tag style={{ margin: 0, fontSize: 11, color: '#999' }}>无能力路由</Tag>}
                  </Space>
                </Card>
              )
            })}
          </div>

          {/* 展开的厂商详情 */}
          {expandedVendor && (
            <VendorDetailInline
              vendor={vendors.find(v => v.id === expandedVendor)!}
              vendorCaps={capsByVendor.get(expandedVendor) || []}
              onClose={() => setExpandedVendor(null)}
            />
          )}

          {/* 能力路由全局视图 */}
          {capsByType.size > 0 && (
            <Collapse
              ghost
              style={{ marginTop: 16 }}
              items={[{
                key: 'routing',
                label: <Text strong>能力路由全局视图（点击展开）</Text>,
                children: <CapabilityRoutingTable capsByType={capsByType} vendors={vendors} />,
              }]}
            />
          )}
        </>
      </QueryBoundary>
    </div>
  )
}

// ---- 厂商内联详情 ----
function VendorDetailInline({ vendor, vendorCaps, onClose }: {
  vendor: IntegrationVendor & { capabilities?: IntegrationCapability[] }
  vendorCaps: IntegrationCapability[]
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [key, setKey] = useState('')
  const [enabled, setEnabled] = useState(vendor.enabled)
  const [addingCap, setAddingCap] = useState(false)
  const [capForm] = Form.useForm()

  const saveMutation = useMutation({
    mutationFn: () => businessApi.saveIntegrationVendor(vendor.id, {
      api_key: key || undefined,
      enabled,
    }),
    onSuccess: () => {
      message.success('已保存')
      queryClient.invalidateQueries({ queryKey: ['admin-integration-vendors'] })
      queryClient.invalidateQueries({ queryKey: ['admin-integrations'] })
    },
  })

  const setDefaultMutation = useMutation({
    mutationFn: ({ capId, vendorId }: { capId: string; vendorId: string }) =>
      businessApi.setCapabilityDefault(capId, vendorId),
    onSuccess: () => {
      message.success('默认已切换（≤10s 生效）')
      queryClient.invalidateQueries({ queryKey: ['admin-integration-capabilities'] })
    },
  })

  const saveCapMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => businessApi.saveIntegrationCapability(id, data),
    onSuccess: () => {
      message.success('能力配置已保存')
      queryClient.invalidateQueries({ queryKey: ['admin-integration-capabilities'] })
    },
  })

  // 新增能力条目（所有厂商通用）
  const addCapMutation = useMutation({
    mutationFn: (data: { id: string; data: any }) => businessApi.saveIntegrationCapability(data.id, data.data),
    onSuccess: () => {
      message.success('能力已添加')
      setAddingCap(false)
      queryClient.invalidateQueries({ queryKey: ['admin-integration-capabilities'] })
      queryClient.invalidateQueries({ queryKey: ['admin-integration-vendors'] })
    },
  })

  // 删除能力条目
  const deleteCapMutation = useMutation({
    mutationFn: (id: string) => businessApi.deleteIntegrationCapability(id),
    onSuccess: () => {
      message.success('能力已删除')
      queryClient.invalidateQueries({ queryKey: ['admin-integration-capabilities'] })
      queryClient.invalidateQueries({ queryKey: ['admin-integration-vendors'] })
    },
  })

  // 编辑能力条目
  const [editingCap, setEditingCap] = useState<any>(null)
  const [editCapForm] = Form.useForm()

  const handleEditCapability = async () => {
    const v = await editCapForm.validateFields()
    saveCapMutation.mutate({
      id: editingCap.id,
      data: { model: v.model, endpoint: v.endpoint, extra_json: v.extra_json },
    })
    setEditingCap(null)
  }

  const handleAddCapability = async (vendorId: string) => {
    const v = await capForm.validateFields()
    const id = `${v.cap_id}#${vendorId}`
    addCapMutation.mutate({
      id,
      data: {
        cap_id: v.cap_id,
        vendor_id: vendorId,
        model: v.model,
        endpoint: v.endpoint || '',
        extra_json: v.extra_json || '',
        enabled: true,
        is_default: false,
      },
    })
  }

  return (
    <Card style={{ marginBottom: 20, borderColor: 'var(--wr-primary)' }} styles={{ body: { padding: '20px 24px' } }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 16 }}>{vendor.name || vendor.id}</Text>
        <Button size="small" onClick={onClose}>收起</Button>
      </div>

      {/* Key / 启停 */}
      <div style={{ marginBottom: 16 }}>
        <Text strong style={{ fontSize: 13, marginBottom: 8, display: 'block' }}>连接配置</Text>
        <Space direction="vertical" size={8} style={{ maxWidth: 500 }}>
          <div>
            <Text>当前 Key：</Text>
            <Text code>{vendor.api_key ? vendor.api_key.slice(0, 8) + '***' + vendor.api_key.slice(-4) : '未配置'}</Text>
          </div>
          <Input.Password
            placeholder={vendor.api_key ? '留空不修改' : '输入 API Key'}
            value={key} onChange={e => setKey(e.target.value)}
            style={{ maxWidth: 400 }}
          />
          <Space>
            <Text>启用</Text>
            <Switch checked={enabled} onChange={setEnabled} />
            <Button type="primary" size="small" icon={<SaveOutlined />}
              loading={saveMutation.isPending} onClick={() => saveMutation.mutate()}>
              保存
            </Button>
          </Space>
        </Space>
      </div>

      {/* 能力路由（所有厂商通用）+ 新增能力条目 */}
      <div style={{ marginBottom: vendor.id === 'vidu' ? 16 : 0 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          <Text strong style={{ fontSize: 13 }}>能力路由</Text>
          <Button size="small" onClick={() => { setAddingCap(true); capForm.resetFields() }}>+ 新增能力</Button>
        </div>
        {vendorCaps.length === 0 ? (
          <Text type="secondary" style={{ fontSize: 12 }}>暂无能力路由——点击「新增能力」添加</Text>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 10 }}>
            {vendorCaps.map(c => (
              <Card key={c.id} size="small" style={c.is_default ? { borderColor: 'var(--wr-primary)' } : undefined}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Space>
                    <Text>{CAP_LABELS[c.cap_id] || c.cap_id}</Text>
                    {c.is_default && <Tag color="green">默认</Tag>}
                    <Tag color={c.enabled ? 'green' : 'default'}>{c.enabled ? '启用' : '停用'}</Tag>
                  </Space>
                  <Space>
                    {!c.is_default && c.enabled && (
                      <Button size="small"
                        onClick={() => setDefaultMutation.mutate({ capId: c.cap_id, vendorId: c.vendor_id })}>
                        设为默认
                      </Button>
                    )}
                    <Switch size="small" checked={c.enabled}
                      onChange={enabled => saveCapMutation.mutate({ id: c.id, data: { enabled } })} />
                  </Space>
                </div>
                <Descriptions column={1} size="small" style={{ marginTop: 6 }}>
                  <Descriptions.Item label="模型">{c.model || '—'}</Descriptions.Item>
                  <Descriptions.Item label="端点">{c.endpoint || '—'}</Descriptions.Item>
                  {c.extra_json && <Descriptions.Item label="扩展参数"><Text code style={{ fontSize: 11 }}>{c.extra_json}</Text></Descriptions.Item>}
                </Descriptions>
                <Space style={{ marginTop: 8 }}>
                  <Button size="small" type="link" onClick={() => {
                    editCapForm.setFieldsValue({ model: c.model, endpoint: c.endpoint, extra_json: c.extra_json || '' })
                    setEditingCap(c)
                  }}>编辑</Button>
                  <Popconfirm
                    title={`删除 ${CAP_LABELS[c.cap_id] || c.cap_id}（${c.vendor_id}）？`}
                    description="删除后该能力路由条目消失，可重新添加"
                    okText="删除" okButtonProps={{ danger: true }} cancelText="取消"
                    onConfirm={() => deleteCapMutation.mutate(c.id)}
                  >
                    <Button size="small" type="link" danger>删除</Button>
                  </Popconfirm>
                </Space>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* 新增能力弹窗（所有厂商通用） */}
      <Modal
        title="新增能力"
        open={addingCap}
        onOk={() => handleAddCapability(vendor.id)}
        onCancel={() => setAddingCap(false)}
        confirmLoading={addCapMutation.isPending}
        width={500}
        okText="添加"
      >
        <Form form={capForm} layout="vertical">
          <Form.Item name="cap_id" label="能力类型" rules={[{ required: true }]}>
            <Select placeholder="选择能力类型" options={CAP_OPTIONS} />
          </Form.Item>
          <Form.Item name="model" label="模型名" rules={[{ required: true }]}>
            <Input placeholder="如 mimo-v2.5-pro / MiniMax-M2.5 / whisper-1" />
          </Form.Item>
          <Form.Item name="endpoint" label="端点路径（通常留空）" tooltip="LLM/ASR 等标准端点留空即可，系统自动拼接标准路径。仅在厂商端点非标准时填写相对路径（如 /v1/generate）。">
            <Input placeholder="留空 = 自动用厂商 base_url + 标准路径" />
          </Form.Item>
          <Form.Item name="extra_json" label="扩展参数（可选，JSON）">
            <Input.TextArea placeholder='{"response_style":"chat","asr_options_language":"auto"}' rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 编辑能力弹窗 */}
      <Modal
        title={`编辑能力 · ${CAP_LABELS[editingCap?.cap_id] || editingCap?.cap_id || ''}（${editingCap?.vendor_id || ''}）`}
        open={!!editingCap}
        onOk={handleEditCapability}
        onCancel={() => setEditingCap(null)}
        confirmLoading={saveCapMutation.isPending}
        width={500}
        okText="保存"
      >
        <Form form={editCapForm} layout="vertical">
          <Form.Item name="model" label="模型名" rules={[{ required: true }]}>
            <Input placeholder="如 mimo-v2.5-pro" />
          </Form.Item>
          <Form.Item name="endpoint" label="端点路径" tooltip="留空 = 自动用厂商 base_url + 标准路径。仅在非标准端点时填写相对路径。">
            <Input placeholder="留空 = 自动拼接（推荐）" />
          </Form.Item>
          <Form.Item name="extra_json" label="扩展参数（JSON）">
            <Input.TextArea placeholder='{"response_style":"chat"}' rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Vidu 特有：端点×模型矩阵（时长/分辨率/比例等复杂能力参数） */}
      {vendor.id === 'vidu' && <ViduModelPanel />}
    </Card>
  )
}

// ---- Vidu 模型管理面板（从 GenerationSpecs 迁入——一个入口，详情页内管理）----
function ViduModelPanel() {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<any>(null)
  const [adding, setAdding] = useState(false)
  const [form] = Form.useForm()

  const { data: specs = [] } = useQuery({
    queryKey: ['admin-gen-specs'],
    queryFn: () => businessApi.adminListGenerationSpecs(),
  })

  const { data: modes = [] } = useQuery({
    queryKey: ['admin-gen-modes'],
    queryFn: () => businessApi.adminListGenerationModes().then(r => r.modes),
  })

  const saveMut = useMutation({
    mutationFn: ({ subType, model, body }: any) => businessApi.adminSaveGenerationSpec(subType, model, body),
    onSuccess: () => {
      message.success('已保存（30s 热生效）')
      setEditing(null); setAdding(false)
      queryClient.invalidateQueries({ queryKey: ['admin-gen-specs'] })
    },
  })

  const deleteMut = useMutation({
    mutationFn: ({ subType, model }: any) => businessApi.adminDeleteGenerationSpec(subType, model),
    onSuccess: () => { message.success('已恢复出厂默认'); queryClient.invalidateQueries({ queryKey: ['admin-gen-specs'] }) },
  })

  const setModeMut = useMutation({
    mutationFn: ({ subType, enabled }: any) => businessApi.adminSetGenerationMode(subType, enabled),
    onSuccess: () => {
      message.success('模式开关已更新')
      queryClient.invalidateQueries({ queryKey: ['admin-gen-modes'] })
      queryClient.invalidateQueries({ queryKey: ['admin-gen-specs'] })
    },
  })

  const openEdit = (spec: any) => {
    let cap: any = null
    try { cap = spec.capabilities_json ? JSON.parse(spec.capabilities_json) : null } catch {}
    form.setFieldsValue({
      model: spec.model, sub_type: spec.sub_type, enabled: spec.enabled,
      family: cap?.family, durations: cap?.durations?.join('-') || '',
      resolutions: cap?.resolutions?.join(',') || '', aspect_ratios: cap?.aspect_ratios?.join(',') || '',
      image_slots: cap?.image_slots ?? 0, video_slots: cap?.video_slots ?? 0,
      max_prompt_len: cap?.max_prompt_len || 0, supports_subjects: !!cap?.supports_subjects,
    })
    setEditing(spec)
  }

  const handleSave = async () => {
    const v = await form.validateFields()
    const cap = {
      model: v.model, family: v.family || '',
      durations: (v.durations ? v.durations.split('-').map(Number) : [0, 0]),
      resolutions: v.resolutions ? v.resolutions.split(',').map((s: string) => s.trim()).filter(Boolean) : [],
      aspect_ratios: v.aspect_ratios ? v.aspect_ratios.split(',').map((s: string) => s.trim()).filter(Boolean) : [],
      image_slots: v.image_slots ?? 0, video_slots: v.video_slots ?? 0,
      supports_subjects: !!v.supports_subjects, max_prompt_len: v.max_prompt_len || 0,
    }
    saveMut.mutate({ subType: v.sub_type, model: v.model, body: { capability: cap, enabled: v.enabled } })
  }

  const types = [...new Set(specs.map((s: any) => s.sub_type))].sort()

  return (
    <div style={{ marginTop: 16 }}>
      <Text strong style={{ fontSize: 13, marginBottom: 8, display: 'block' }}>模型管理（端点×模型矩阵）</Text>
      <Text type="secondary" style={{ fontSize: 12, marginBottom: 12, display: 'block' }}>
        新增模型 = 直接插入（端点组装逻辑与模型名无关），删除行 = 恢复出厂默认，30s 热生效
      </Text>

      {/* 模式开关 */}
      <div style={{ marginBottom: 14 }}>
        <Text strong style={{ fontSize: 12, marginBottom: 6, display: 'block' }}>商户端模式开关</Text>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {(modes as any[]).map((m: any) => (
            <Tooltip key={m.sub_type} title={`${m.model_count} 个模型 · ${m.enabled ? '全部启用' : '全部停用'}`}>
              <Tag.CheckableTag
                checked={m.enabled}
                onChange={v => setModeMut.mutate({ subType: m.sub_type, enabled: v })}
                style={{ fontSize: 12, padding: '2px 10px' }}
              >
                {MODE_LABELS[m.sub_type] || m.sub_type}
              </Tag.CheckableTag>
            </Tooltip>
          ))}
        </div>
      </div>

      {/* 新增模型 */}
      <Button size="small" type="primary" style={{ marginBottom: 10 }} onClick={() => { setAdding(true); form.resetFields(); form.setFieldsValue({ sub_type: types[0], enabled: true }) }}>
        + 新增模型
      </Button>

      {/* 模型矩阵表格 */}
      <Table
        rowKey={r => r.sub_type + '|' + r.model}
        dataSource={specs as any[]}
        size="small"
        pagination={{ pageSize: 15 }}
        columns={[
          { title: '端点', dataIndex: 'sub_type', width: 150, render: (t: string) => <Tag color="purple">{t}</Tag> },
          { title: '模型', dataIndex: 'model', width: 140, render: (m: string) => <Text strong>{m}</Text> },
          {
            title: '能力', key: 'cap',
            render: (_: any, r: any) => {
              let cap: any = null
              try { cap = r.capabilities_json ? JSON.parse(r.capabilities_json) : null } catch {}
              const parts: string[] = []
              if (cap?.durations?.[1]) parts.push(`${cap.durations[0]}-${cap.durations[1]}s`)
              if (cap?.image_slots) parts.push(`图${cap.image_slots === -1 ? '1-7' : cap.image_slots}`)
              if (cap?.resolutions?.length) parts.push(cap.resolutions.join('/'))
              return <Text type="secondary" style={{ fontSize: 11 }}>{parts.join(' · ') || '—'}</Text>
            },
          },
          { title: '状态', dataIndex: 'enabled', width: 80, render: (e: boolean) => <Tag color={e ? 'success' : 'error'}>{e ? '启用' : '停用'}</Tag> },
          {
            title: '操作', key: 'action', width: 140,
            render: (_: any, r: any) => (
              <Space size={4}>
                <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
                {r.has_override && <Button size="small" type="link" danger onClick={() => deleteMut.mutate({ subType: r.sub_type, model: r.model })}>恢复默认</Button>}
              </Space>
            ),
          },
        ]}
      />

      {/* 编辑/新增弹窗 */}
      <Modal
        title={adding ? '新增模型' : `编辑 · ${editing?.sub_type}/${editing?.model}`}
        open={!!editing || adding}
        onOk={handleSave}
        onCancel={() => { setEditing(null); setAdding(false) }}
        confirmLoading={saveMut.isPending}
        width={640}
        okText="保存"
      >
        <Form form={form} layout="vertical">
          <Space size={16}>
            <Form.Item name="sub_type" label="端点" rules={[{ required: true }]} style={{ width: 200 }}>
              <Select options={types.map(t => ({ value: t, label: t }))} disabled={!adding} />
            </Form.Item>
            <Form.Item name="model" label="模型名" rules={[{ required: true }]} style={{ width: 220 }}>
              <Input placeholder="如 viduq4-pro" disabled={!adding} />
            </Form.Item>
            <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
          </Space>
          <Space size={16} style={{ flexWrap: 'wrap' }}>
            <Form.Item name="family" label="系列"><Input placeholder="q3" style={{ width: 120 }} /></Form.Item>
            <Form.Item name="durations" label="时长(min-max)"><Input placeholder="1-16" style={{ width: 140 }} /></Form.Item>
            <Form.Item name="image_slots" label="图片槽位"><InputNumber placeholder="0/1/2/-1" style={{ width: 120 }} /></Form.Item>
            <Form.Item name="video_slots" label="视频槽位"><InputNumber style={{ width: 100 }} /></Form.Item>
            <Form.Item name="max_prompt_len" label="prompt上限"><InputNumber style={{ width: 120 }} /></Form.Item>
          </Space>
          <Space size={16}>
            <Form.Item name="resolutions" label="分辨率"><Input placeholder="540p,720p,1080p" style={{ width: 200 }} /></Form.Item>
            <Form.Item name="aspect_ratios" label="比例"><Input placeholder="16:9,9:16,1:1" style={{ width: 200 }} /></Form.Item>
            <Form.Item name="supports_subjects" label="支持主体" valuePropName="checked"><Switch /></Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}

// ---- 能力路由全局表（折叠面板内）----
function CapabilityRoutingTable({ capsByType, vendors }: {
  capsByType: Map<string, IntegrationCapability[]>
  vendors: (IntegrationVendor & { capabilities?: IntegrationCapability[] })[]
}) {
  const queryClient = useQueryClient()
  const vendorMap = new Map(vendors.map(v => [v.id, v]))

  const setDefaultMutation = useMutation({
    mutationFn: ({ capId, vendorId }: { capId: string; vendorId: string }) =>
      businessApi.setCapabilityDefault(capId, vendorId),
    onSuccess: () => {
      message.success('默认已切换')
      queryClient.invalidateQueries({ queryKey: ['admin-integration-capabilities'] })
    },
  })

  return (
    <div>
      {Array.from(capsByType.entries()).map(([capId, capList]) => (
        <div key={capId} style={{ marginBottom: 14 }}>
          <Text strong style={{ fontSize: 13 }}>{CAP_LABELS[capId] || capId}</Text>
          <div style={{ display: 'flex', gap: 8, marginTop: 6, flexWrap: 'wrap' }}>
            {capList.map(c => {
              const vendor = vendorMap.get(c.vendor_id)
              return (
                <Tag
                  key={c.id}
                  color={c.is_default ? 'green' : c.enabled ? 'blue' : 'default'}
                  style={{ cursor: !c.is_default && c.enabled ? 'pointer' : undefined, fontSize: 12, padding: '2px 8px' }}
                  onClick={() => {
                    if (!c.is_default && c.enabled) {
                      setDefaultMutation.mutate({ capId: c.cap_id, vendorId: c.vendor_id })
                    }
                  }}
                >
                  {vendor?.name || c.vendor_id}{c.is_default ? ' ✓ 默认' : c.enabled ? '' : ' (停用)'}
                </Tag>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}

// ---- 旧版详情页保留（Vidu 特有：模式/矩阵/首选模型/音色/回调/用量）----
function IntegrationDetail({ id }: { id: string }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['admin-integration', id],
    queryFn: () => businessApi.getIntegrationDetail(id),
    retry: false,
  })

  const { data: health, isLoading: healthLoading, refetch: refetchHealth } = useQuery({
    queryKey: ['admin-integration-health', id],
    queryFn: () => businessApi.getIntegrationHealth(id),
    staleTime: 30_000,
  })

  // data 兜底：内容分支仅在 data 就绪时渲染（QueryBoundary 先挡 loading/error）
  const meta = data?.meta as any
  const sections = (data?.sections ?? {}) as Record<string, any>

  return (
    <QueryBoundary loading={isLoading} error={isError} onRetry={() => refetch()}>
      {!data ? (
        <Empty description="集成不存在" />
      ) : (
    <div className="wr-page-content" style={{ paddingTop: 8 }}>
      <Button type="link" onClick={() => navigate('/admin/integrations')} style={{ marginBottom: 8, padding: 0 }}>
        ← 返回集成列表
      </Button>
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 20 }}>
        <span style={{ fontSize: 28, color: 'var(--wr-primary)' }}><CloudServerOutlined /></span>
        <div>
          <Typography.Title level={3} style={{ margin: 0 }}>{meta.name}</Typography.Title>
          <Text type="secondary">{meta.desc}</Text>
        </div>
        <div style={{ marginLeft: 'auto' }}>
          <Space>
            <Tag color={health?.status === 'ok' ? 'success' : 'warning'} style={{ fontSize: 13 }}>
              {health?.detail || '检查中…'}
            </Tag>
            <Button size="small" icon={<ReloadOutlined />} loading={healthLoading} onClick={() => refetchHealth()}>刷新健康</Button>
          </Space>
        </div>
      </div>
      <Tabs
        defaultActiveKey={meta.sections?.[0]}
        items={(meta.sections || []).map((s: string) => ({
          key: s,
          label: SECTION_LABELS[s] || s,
          children: <LegacySectionRenderer sectionId={s} data={sections[s]} meta={meta} queryClient={queryClient} />,
        }))}
      />
    </div>
      )}
    </QueryBoundary>
  )
}

const SECTION_LABELS: Record<string, string> = {
  'overview': '概览', 'api-key': 'API Key', 'modes': '生成模式', 'models': '模型矩阵',
  'preferred-model': '首选模型', 'voices': '音色库', 'callback-health': '回调健康',
  'usage': '用量', 'llm-configs': 'LLM 配置', 'asr-config': 'ASR 配置',
  'tavily-config': 'Tavily 配置', 'zpay-config': 'ZPAY 配置',
}

function LegacySectionRenderer({ sectionId, data, meta, queryClient }: {
  sectionId: string; data: any; meta: any; queryClient: any
}) {
  if (!data) return <Text type="secondary">暂无数据</Text>
  switch (sectionId) {
    case 'overview':
      return (
        <Descriptions column={2} size="small" style={{ maxWidth: 600 }}>
          <Descriptions.Item label="状态">{data.enabled ? <Tag color="success">启用</Tag> : <Tag color="default">停用</Tag>}</Descriptions.Item>
          <Descriptions.Item label="Key">{data.has_key ? data.key_masked : '未配置'}</Descriptions.Item>
          <Descriptions.Item label="最后更新">{data.updated_at?.replace('T', ' ').slice(0, 19) || '—'}</Descriptions.Item>
        </Descriptions>
      )
    case 'api-key':
      return <APIKeySection data={data} meta={meta} queryClient={queryClient} />
    case 'modes':
      return <ModesSection data={data} queryClient={queryClient} />
    case 'preferred-model':
      return <PreferredModelSection data={data} queryClient={queryClient} />
    case 'llm-configs':
      return <LLMConfigsSection data={data} />
    case 'asr-config':
      return <ASRConfigSection data={data} queryClient={queryClient} />
    default:
      return <Text type="secondary">区块 {sectionId} 待实现</Text>
  }
}

function APIKeySection({ data, meta, queryClient }: { data: any; meta: any; queryClient: any }) {
  const [key, setKey] = useState('')
  const [enabled, setEnabled] = useState(data.enabled ?? true)
  const saveMutation = useMutation({
    mutationFn: () => businessApi.saveProviderConfig(meta.id, { api_key: key || undefined, enabled }),
    onSuccess: () => { message.success('已保存'); queryClient.invalidateQueries({ queryKey: ['admin-integration', meta.id] }) },
  })
  return (
    <Space direction="vertical" size={10} style={{ maxWidth: 500 }}>
      <div><Text strong>当前 Key：</Text><Text code>{data.has_key ? data.key_masked : '未配置'}</Text></div>
      <Input.Password placeholder={data.has_key ? '留空不修改' : '输入 API Key'} value={key} onChange={e => setKey(e.target.value)} style={{ maxWidth: 400 }} />
      <Space><Text>启用</Text><Switch checked={enabled} onChange={setEnabled} /></Space>
      <Button type="primary" icon={<SaveOutlined />} loading={saveMutation.isPending} onClick={() => saveMutation.mutate()}>保存</Button>
    </Space>
  )
}

function ModesSection({ data, queryClient }: { data: any; queryClient: any }) {
  const toggleMutation = useMutation({
    mutationFn: ({ subType, enabled }: { subType: string; enabled: boolean }) => businessApi.adminSetGenerationMode(subType, enabled),
    onSuccess: () => { message.success('已保存'); queryClient.invalidateQueries({ queryKey: ['admin-integration'] }) },
  })
  return (
    <div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 10 }}>
        {(data || []).map((m: any) => (
          <div key={m.sub_type} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', border: '1px solid #f0f0f0', borderRadius: 8 }}>
            <div>
              <Text style={{ fontSize: 13 }}>{m.sub_type}</Text>
              <Tag style={{ marginLeft: 6, fontSize: 10 }} color={m.tier === 'default' ? 'green' : 'default'}>{m.tier}</Tag>
            </div>
            <Switch size="small" checked={m.enabled} onChange={v => toggleMutation.mutate({ subType: m.sub_type, enabled: v })} />
          </div>
        ))}
      </div>
    </div>
  )
}

function PreferredModelSection({ data, queryClient }: { data: any; queryClient: any }) {
  const [imageModel, setImageModel] = useState(data?.image_subject || '')
  const [videoModel, setVideoModel] = useState(data?.video_subject || '')
  const saveMutation = useMutation({
    mutationFn: () => businessApi.setViduPreferredModel({ image_subject: imageModel || undefined, video_subject: videoModel || undefined }),
    onSuccess: () => { message.success('首选模型已保存'); queryClient.invalidateQueries({ queryKey: ['admin-integration'] }) },
  })
  return (
    <Space direction="vertical" size={10} style={{ maxWidth: 500 }}>
      <Alert type="info" showIcon message="D3 模型自动切换：参考生视频按主体类型选模型。用户不传 model 时自动使用此处配置。" />
      <div><Text strong>图片主体首选</Text><Select style={{ width: 260, marginLeft: 10 }} value={imageModel || undefined} onChange={setImageModel} placeholder="默认 viduq3-turbo" allowClear options={[{ value: 'viduq3-turbo', label: 'viduq3-turbo（推荐）' }, { value: 'viduq3', label: 'viduq3' }, { value: 'viduq3-mix', label: 'viduq3-mix' }]} /></div>
      <div><Text strong>视频主体首选</Text><Select style={{ width: 260, marginLeft: 10 }} value={videoModel || undefined} onChange={setVideoModel} placeholder="默认 viduq2-pro" allowClear options={[{ value: 'viduq2-pro', label: 'viduq2-pro（唯一支持视频主体）' }]} /></div>
      <Button type="primary" icon={<SaveOutlined />} loading={saveMutation.isPending} onClick={() => saveMutation.mutate()}>保存首选模型</Button>
    </Space>
  )
}

function LLMConfigsSection({ data }: { data: any[] }) {
  const queryClient = useQueryClient()
  const setDefaultMutation = useMutation({
    mutationFn: (name: string) => businessApi.setLLMDefault(name),
    onSuccess: () => { message.success('默认模型已切换'); queryClient.invalidateQueries({ queryKey: ['admin-integration', 'llm'] }) },
  })
  if (!data || data.length === 0) return <Empty description="暂无 LLM 配置" />
  return (
    <div>
      <Alert type="info" showIcon style={{ marginBottom: 12 }} message="默认模型：未指定模型的 AI 任务自动使用此处标记的默认配置。切换后 ≤10s 生效。" />
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 12 }}>
        {data.map((c: any) => (
          <Card key={c.name} size="small" style={c.is_default ? { borderColor: 'var(--wr-primary)' } : undefined}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Space><Text strong>{c.name}</Text>{c.is_default && <Tag color="green">默认</Tag>}</Space>
              {!c.is_default && <Button size="small" onClick={() => setDefaultMutation.mutate(c.name)}>设为默认</Button>}
            </div>
            <Descriptions column={1} size="small" style={{ marginTop: 8 }}>
              <Descriptions.Item label="模型">{c.model}</Descriptions.Item>
              <Descriptions.Item label="端点">{c.base_url || '—'}</Descriptions.Item>
              <Descriptions.Item label="Key">{c.api_key ? '已配置' : '未配置'}</Descriptions.Item>
            </Descriptions>
          </Card>
        ))}
      </div>
    </div>
  )
}

function ASRConfigSection({ data, queryClient }: { data: any; queryClient: any }) {
  const [key, setKey] = useState('')
  const [enabled, setEnabled] = useState(data.enabled ?? true)
  const [endpoint, setEndpoint] = useState(data.endpoint || '')
  const [model, setModel] = useState(data.model || '')

  const onProviderChange = (providerId: string) => {
    const p = (data.providers || []).find((x: any) => x.model === providerId)
    if (p) { setModel(p.model); if (p.endpoint) setEndpoint(p.endpoint) }
  }

  const saveMutation = useMutation({
    mutationFn: () => businessApi.saveProviderConfig('asr', { api_key: key || undefined, base_url: endpoint || undefined, enabled, extra_json: JSON.stringify({ model }) }),
    onSuccess: () => { message.success('ASR 配置已保存（≤10s 生效）'); queryClient.invalidateQueries({ queryKey: ['admin-integration', 'asr'] }) },
  })

  return (
    <Space direction="vertical" size={10} style={{ maxWidth: 600 }}>
      <Alert type="info" showIcon message="协议：OpenAI 兼容 /v1/chat/completions（小米 MiMo）或 /audio/transcriptions（硅基流动/OpenAI）。选择厂商自动填充端点。" />
      <div><Text strong>当前 Key：</Text><Text code>{data.has_key ? data.key_masked : '未配置'}</Text></div>
      <Input.Password placeholder={data.has_key ? '留空不修改' : '输入 ASR API Key'} value={key} onChange={e => setKey(e.target.value)} style={{ maxWidth: 400 }} />
      <div>
        <Text strong>厂商/模型</Text>
        <Select style={{ width: 400, marginLeft: 10 }} value={model || undefined} onChange={onProviderChange} allowClear showSearch placeholder="选择厂商自动填充端点" optionFilterProp="label"
          options={(data.providers || []).map((p: any) => ({ value: p.model, label: `${p.name}（${p.model}）${p.free ? ' 免费' : ''}` }))} />
      </div>
      <div>
        <Text strong>端点</Text>
        <Input placeholder="https://api.siliconflow.cn/v1/audio/transcriptions" value={endpoint} onChange={e => setEndpoint(e.target.value)} style={{ maxWidth: 450, marginLeft: 10 }} />
        <Text type="secondary" style={{ fontSize: 11, display: 'block', marginLeft: 10, marginTop: 2 }}>选择厂商后自动填充，也可手动修改</Text>
      </div>
      <Space><Text>启用</Text><Switch checked={enabled} onChange={setEnabled} /></Space>
      <Button type="primary" icon={<SaveOutlined />} loading={saveMutation.isPending} onClick={() => saveMutation.mutate()}>保存 ASR 配置</Button>
      {data.providers && (
        <div style={{ marginTop: 8 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>可选服务商：</Text>
          {data.providers.map((p: any) => (
            <div key={p.id} style={{ fontSize: 12, marginTop: 4, cursor: 'pointer' }} onClick={() => onProviderChange(p.model)}>
              <Text>{p.name}</Text>
              {p.free && <Tag color="green" style={{ marginLeft: 4, fontSize: 10 }}>免费</Tag>}
              {p.note && <Text type="secondary" style={{ marginLeft: 4 }}>（{p.note}）</Text>}
            </div>
          ))}
        </div>
      )}
    </Space>
  )
}

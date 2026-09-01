import { useState } from 'react'
import { Typography, Table, Tag, Button, Space, Modal, Form, Input, InputNumber, Switch, Select, Tooltip, Card } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessApi } from '../../api/business'
import type { GenerationSpec, GenerationModeView } from '../../types/api'
import { toast } from '../../utils/feedback'

const { Text } = Typography

// 模式中文名（模式开关区展示——与商户端 SUBTYPE_META 同口径）
const MODE_LABEL: Record<string, string> = {
  text2image: '文生图', text2video: '文生视频', img2video: '图生视频',
  tts: '语音合成', digital_human: '数字人', reference2video: '参考生视频',
  voice_clone: '声音克隆', subject: '主体创建',
  start_end2video: '首尾帧', multiframe: '智能多帧', text2audio: '音乐生成',
  sound_effect: '音效生成', template: '模板成片',
}
const TIER_META: Record<string, { label: string; hint: string }> = {
  default: { label: '默认集', hint: '商户端快捷卡片区直接可见' },
  advanced: { label: '进阶折叠', hint: '收进"更多模式"折叠区' },
  closed: { label: '默认关闭', hint: '商户端不显示（开启后进折叠区）' },
}

interface GenerationCapability {
  model?: string
  family?: string
  durations?: [number, number]
  resolutions?: string[]
  aspect_ratios?: string[]
  audio_default?: boolean
  audio_types?: string[]
  image_slots?: number
  video_slots?: number
  supports_bgm?: boolean
  supports_subjects?: boolean
  supports_movement?: boolean
  max_prompt_len?: number
}

function capSummary(cap: GenerationCapability | null): string {
  if (!cap) return '-'
  const parts: string[] = []
  if (cap.durations && (cap.durations[0] || cap.durations[1])) parts.push(`时长${cap.durations[0]}-${cap.durations[1]}s`)
  if (cap.image_slots !== undefined && cap.image_slots !== 0) parts.push(`图${cap.image_slots === -1 ? '1-7' : cap.image_slots}张`)
  if (cap.resolutions?.length) parts.push(cap.resolutions.join('/'))
  if (cap.max_prompt_len) parts.push(`prompt≤${cap.max_prompt_len}`)
  return parts.join(' · ') || '-'
}

export default function GenerationSpecs() {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<GenerationSpec | null>(null)
  const [adding, setAdding] = useState(false)
  const [form] = Form.useForm()
  const [types, setTypes] = useState<string[]>([])

  const { data: specs = [], isLoading } = useQuery({
    queryKey: ['admin-gen-specs'],
    queryFn: async () => {
      const r = await businessApi.adminListGenerationSpecs()
      // 端点类型枚举（用于"新增模型"下拉）
      const t = await businessApi.listGenerationTypes()
      setTypes(t.types.map((x) => x.sub_type))
      return r
    },
  })

  // 模式开关（sub_type 批量启停——傻瓜化：三档分组，切一个开关=批量改该模式全部模型）
  const { data: modes = [] } = useQuery<GenerationModeView[]>({
    queryKey: ['admin-gen-modes'],
    queryFn: () => businessApi.adminListGenerationModes().then((r) => r.modes),
  })
  const setModeMut = useMutation({
    mutationFn: ({ subType, enabled }: { subType: string; enabled: boolean }) =>
      businessApi.adminSetGenerationMode(subType, enabled),
    onSuccess: (_d, v) => {
      toast.ok(`${MODE_LABEL[v.subType] || v.subType} 已${v.enabled ? '开启' : '关闭'}`)
      queryClient.invalidateQueries({ queryKey: ['admin-gen-modes'] })
      queryClient.invalidateQueries({ queryKey: ['admin-gen-specs'] })
      queryClient.invalidateQueries({ queryKey: ['generation-types'] })
    },
    onError: (e: Error) => toast.fail('切换失败：' + e.message),
  })
  const tierOrder: Record<string, number> = { default: 0, advanced: 1, closed: 2 }
  const sortedModes = [...modes].sort((a, b) => (tierOrder[a.tier] ?? 9) - (tierOrder[b.tier] ?? 9))

  const saveMut = useMutation({
    mutationFn: ({ subType, model, body }: { subType: string; model: string; body: { capability: GenerationCapability; enabled: boolean } }) =>
      businessApi.adminSaveGenerationSpec(subType, model, body),
    onSuccess: () => {
      toast.ok('已保存')
      setEditing(null)
      setAdding(false)
      queryClient.invalidateQueries({ queryKey: ['admin-gen-specs'] })
    },
    onError: (e: Error) => toast.fail('保存失败：' + e.message),
  })

  const deleteMut = useMutation({
    mutationFn: ({ subType, model }: { subType: string; model: string }) =>
      businessApi.adminDeleteGenerationSpec(subType, model),
    onSuccess: () => {
      toast.ok('已删除，恢复默认', 'admin-spec-del')
      queryClient.invalidateQueries({ queryKey: ['admin-gen-specs'] })
    },
    onError: (e: Error) => toast.fail('删除失败：' + e.message),
  })

  const openEdit = (spec: GenerationSpec) => {
    let cap: GenerationCapability | null = null
    try { cap = spec.capabilities_json ? JSON.parse(spec.capabilities_json) : null } catch { /* ignore */ }
    form.setFieldsValue({
      model: spec.model, sub_type: spec.sub_type, enabled: spec.enabled,
      family: cap?.family, durations: cap?.durations?.join('-') || '', resolutions: cap?.resolutions?.join(',') || '',
      aspect_ratios: cap?.aspect_ratios?.join(',') || '', image_slots: cap?.image_slots, video_slots: cap?.video_slots,
      audio_default: cap?.audio_default, supports_subjects: cap?.supports_subjects,
      supports_movement: cap?.supports_movement, supports_bgm: cap?.supports_bgm, max_prompt_len: cap?.max_prompt_len,
    })
    setEditing(spec)
  }

  const handleSave = async () => {
    const v = await form.validateFields()
    const cap: GenerationCapability = {
      model: v.model, family: v.family || '', durations: (v.durations ? v.durations.split('-').map(Number) : [0, 0]) as [number, number],
      resolutions: v.resolutions ? v.resolutions.split(',').map((s: string) => s.trim()).filter(Boolean) : [],
      aspect_ratios: v.aspect_ratios ? v.aspect_ratios.split(',').map((s: string) => s.trim()).filter(Boolean) : [],
      image_slots: v.image_slots ?? 0, video_slots: v.video_slots ?? 0,
      audio_default: !!v.audio_default, supports_subjects: !!v.supports_subjects,
      supports_movement: !!v.supports_movement, supports_bgm: !!v.supports_bgm,
      max_prompt_len: v.max_prompt_len || 0,
    }
    saveMut.mutate({ subType: v.sub_type, model: v.model, body: { capability: cap, enabled: v.enabled } })
  }

  const columns = [
    {
      title: '端点', dataIndex: 'sub_type', width: 160,
      render: (t: string) => <Tag color="purple" style={{ fontFamily: 'monospace' }}>{t}</Tag>,
    },
    { title: '模型', dataIndex: 'model', width: 140, render: (m: string) => <Text strong>{m}</Text> },
    {
      title: '能力摘要', key: 'cap',
      render: (_: unknown, r: GenerationSpec) => {
        let cap: GenerationCapability | null = null
        try { cap = r.capabilities_json ? JSON.parse(r.capabilities_json) : null } catch { /* ignore */ }
        return <Text type="secondary" style={{ fontSize: 12 }}>{capSummary(cap)}</Text>
      },
    },
    {
      title: '状态', dataIndex: 'enabled', width: 100,
      render: (e: boolean) => <Tag color={e ? 'success' : 'error'}>{e ? '启用' : '停用'}</Tag>,
    },
    {
      title: '来源', key: 'src', width: 90,
      render: (_: unknown, r: GenerationSpec) => (
        <Tooltip title={r.has_override ? '数据库覆盖行——删除可恢复出厂默认' : '出厂默认值（未覆盖）'}>
          <Tag color={r.has_override ? 'blue' : 'default'} style={{ fontSize: 10 }}>{r.has_override ? 'DB 覆盖' : '出厂'}</Tag>
        </Tooltip>
      ),
    },
    {
      title: '操作', key: 'action', width: 180,
      render: (_: unknown, r: GenerationSpec) => (
        <Space size={4}>
          <Button size="small" type="link" onClick={() => openEdit(r)}>编辑</Button>
          {r.has_override && (
            <Button size="small" type="link" danger onClick={() => deleteMut.mutate({ subType: r.sub_type, model: r.model })}>
              恢复默认
            </Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div className="wr-page-content">
      <div className="wr-page-header">
        <h1>生成规格（Vidu 端点×模型矩阵）</h1>
        <p>数据库驱动·全局掌控——所有端点/模型/参数可在后台动态调整，30 秒热生效（无需重启）。删除行 = 恢复出厂默认。</p>
      </div>

      {/* 模式开关区（傻瓜化：商户端生成模式三档控制——切一个开关=批量启停该模式全部模型） */}
      <Card className="wr-glass-card" style={{ marginBottom: 16 }} title="商户端模式开关">
        <Text type="secondary" style={{ display: 'block', fontSize: 12, marginBottom: 12 }}>
          控制商户端「内容中心 · 做视频图片」展示哪些创作模式——关闭的模式（含快捷卡片、折叠区、模型下拉）自动隐藏，提交被拒。30 秒热生效。
        </Text>
        {(['default', 'advanced', 'closed'] as const).map((tier) => {
          const group = sortedModes.filter((m) => m.tier === tier)
          if (group.length === 0) return null
          return (
            <div key={tier} style={{ marginBottom: 12 }}>
              <Space size={8} style={{ marginBottom: 8 }}>
                <Tag color={tier === 'default' ? 'success' : tier === 'advanced' ? 'processing' : 'default'} style={{ margin: 0 }}>
                  {TIER_META[tier].label}
                </Tag>
                <Text type="secondary" style={{ fontSize: 11 }}>{TIER_META[tier].hint}</Text>
              </Space>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                {group.map((m) => (
                  <Tooltip key={m.sub_type} title={
                    m.partial
                      ? `该模式 ${m.model_count} 个模型中有部分被单独停用（见下方矩阵）——开关打开时以矩阵为准微调`
                      : `${m.model_count} 个模型 · ${m.enabled ? '全部启用' : '全部停用'}`
                  }>
                    <div style={{
                      display: 'flex', alignItems: 'center', gap: 8, padding: '6px 12px',
                      border: `1px solid ${m.enabled ? 'var(--wr-primary-border)' : 'var(--wr-border)'}`,
                      borderRadius: 10, background: m.enabled ? 'var(--wr-primary-bg)' : 'var(--wr-bg-elevated)',
                    }}>
                      <Text style={{ fontSize: 13 }}>{MODE_LABEL[m.sub_type] || m.sub_type}</Text>
                      {m.partial && <Tag style={{ margin: 0, fontSize: 10 }}>部分</Tag>}
                      <Switch
                        size="small"
                        checked={m.enabled}
                        loading={setModeMut.isPending && setModeMut.variables?.subType === m.sub_type}
                        onChange={(v) => setModeMut.mutate({ subType: m.sub_type, enabled: v })}
                      />
                    </div>
                  </Tooltip>
                ))}
              </div>
            </div>
          )
        })}
      </Card>

      <Card className="wr-glass-card" style={{ marginBottom: 16 }}>
        <Space>
          <Text type="secondary" style={{ fontSize: 12 }}>
            共 {specs.length} 条规格 · 端点策略（提交/组装逻辑）在代码注册，模型与参数约束全部由本表驱动
          </Text>
          <Button type="primary" size="small" onClick={() => {
            setAdding(true)
            form.resetFields()
            form.setFieldsValue({ sub_type: types[0], enabled: true })
          }}>
            + 新增模型
          </Button>
        </Space>
      </Card>

      <Table
        className="wr-glass-card"
        rowKey={(r) => r.sub_type + '|' + r.model}
        dataSource={specs}
        columns={columns}
        loading={isLoading}
        size="small"
        pagination={{ pageSize: 20 }}
      />

      <Modal
        title={adding ? '新增模型' : `编辑规格 · ${editing?.sub_type}/${editing?.model}`}
        open={!!editing || adding}
        onOk={handleSave}
        onCancel={() => { setEditing(null); setAdding(false) }}
        confirmLoading={saveMut.isPending}
        width={640}
        okText="保存"
      >
        <Form form={form} layout="vertical">
          <Space size={16} style={{ display: 'flex' }}>
            <Form.Item name="sub_type" label="端点" rules={[{ required: true }]} style={{ width: 220 }}>
              <Select options={types.map((t) => ({ value: t, label: t }))} disabled={!adding} />
            </Form.Item>
            <Form.Item name="model" label="模型名" rules={[{ required: true, message: '如 viduq4-pro（新增模型 = 直接插入，端点组装逻辑通用）' }]} style={{ width: 260 }}>
              <Input placeholder="viduq4-pro" disabled={!adding} />
            </Form.Item>
            <Form.Item name="enabled" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
          <Space size={16} style={{ display: 'flex', flexWrap: 'wrap' }}>
            <Form.Item name="family" label="系列"><Input placeholder="q3" style={{ width: 140 }} /></Form.Item>
            <Form.Item name="durations" label="时长范围(min-max)"><Input placeholder="1-16（0-0=不支持自定义）" style={{ width: 220 }} /></Form.Item>
            <Form.Item name="image_slots" label="图片槽位"><InputNumber placeholder="0=无 1=单图 2=双图 -1=1-7" style={{ width: 220 }} /></Form.Item>
            <Form.Item name="video_slots" label="视频槽位"><InputNumber style={{ width: 120 }} /></Form.Item>
            <Form.Item name="max_prompt_len" label="提示词上限"><InputNumber style={{ width: 140 }} /></Form.Item>
          </Space>
          <Space size={16} style={{ display: 'flex', flexWrap: 'wrap' }}>
            <Form.Item name="resolutions" label="分辨率(逗号分隔)"><Input placeholder="540p,720p,1080p" style={{ width: 240 }} /></Form.Item>
            <Form.Item name="aspect_ratios" label="比例(逗号分隔)"><Input placeholder="16:9,9:16,1:1" style={{ width: 220 }} /></Form.Item>
          </Space>
          <Space size={24}>
            <Form.Item name="audio_default" label="音频默认" valuePropName="checked"><Switch /></Form.Item>
            <Form.Item name="supports_subjects" label="主体模式" valuePropName="checked"><Switch /></Form.Item>
            <Form.Item name="supports_movement" label="运动幅度" valuePropName="checked"><Switch /></Form.Item>
            <Form.Item name="supports_bgm" label="BGM" valuePropName="checked"><Switch /></Form.Item>
          </Space>
          {adding && (
            <Text type="warning" style={{ fontSize: 12 }}>
              新增模型后前端「多媒体创作」模型下拉与后端参数校验自动生效——端点的提交/组装逻辑无需改动（与模型名无关）。
            </Text>
          )}
        </Form>
      </Modal>
    </div>
  )
}

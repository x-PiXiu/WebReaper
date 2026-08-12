import { useState, useMemo } from 'react'
import type { ReactNode } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Tabs, Typography, Select, Input, InputNumber, Switch, Slider, Button, Space, Tag, message,
  Table, Modal, Upload, Empty, Popconfirm, Tooltip, Alert, Badge, Segmented, Collapse,
} from 'antd'
import {
  VideoCameraOutlined, PictureOutlined, AudioOutlined, RobotOutlined, AppstoreOutlined,
  UploadOutlined, DeleteOutlined, ReloadOutlined, PlayCircleOutlined, SoundOutlined,
  PlusOutlined, MinusCircleOutlined, FileImageOutlined, CloseCircleOutlined,
  ThunderboltOutlined, ApartmentOutlined, SettingOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../api/business'
import type { GenerationTask, GenerationType, ModelCapability, MediaAsset, PromptRef } from '../../types/api'

const { Text } = Typography
const { TextArea } = Input
const { Dragger } = Upload

// ---- 端点元数据（sub_type → 分类/中文名/说明）----
const SUBTYPE_META: Record<string, { category: string; label: string; desc: string }> = {
  text2video: { category: 'video', label: '文生视频', desc: '一句话生成视频（支持音频/风格/比例）' },
  img2video: { category: 'video', label: '图生视频', desc: '参考图 + 提示词生成视频' },
  start_end2video: { category: 'video', label: '首尾帧', desc: '首帧 + 尾帧生成过渡视频' },
  reference2video: { category: 'video', label: '参考生视频', desc: '主体/参考图模式（q2-pro 支持视频参考）' },
  multiframe: { category: 'video', label: '智能多帧', desc: '首帧 + 2-9 个关键帧生成长视频' },
  text2image: { category: 'image', label: '文生图', desc: '提示词生成图片（可附参考图）' },
  text2audio: { category: 'audio', label: '音乐生成', desc: '提示词生成 2-10 秒音乐' },
  sound_effect: { category: 'audio', label: '音效生成', desc: '时间轴事件驱动生成音效' },
  tts: { category: 'audio', label: '语音合成', desc: '文本合成语音（语速/音量/情绪可控）' },
  voice_clone: { category: 'audio', label: '声音克隆', desc: '引用音频复刻音色（voice_id 永久复用）' },
  digital_human: { category: 'digital_human', label: '数字人', desc: '人像图 + 文本/音频生成口播视频' },
  subject: { category: 'other', label: '主体创建', desc: '创建主体素材，供参考生视频复用' },
}

const CATEGORIES = [
  { key: 'video', label: '视频', icon: <VideoCameraOutlined /> },
  { key: 'image', label: '图片', icon: <PictureOutlined /> },
  { key: 'audio', label: '音频', icon: <AudioOutlined /> },
  { key: 'digital_human', label: '数字人', icon: <RobotOutlined /> },
  { key: 'other', label: '其他', icon: <AppstoreOutlined /> },
]

const STATE_META: Record<string, { color: string; label: string }> = {
  created: { color: 'default', label: '排队中' },
  queueing: { color: 'processing', label: '队列中' },
  processing: { color: 'processing', label: '生成中' },
  success: { color: 'success', label: '已完成' },
  failed: { color: 'error', label: '失败' },
  cancelled: { color: 'warning', label: '已取消' },
}
const ACTIVE_STATES = ['created', 'queueing', 'processing']

// 失败分类 → 处理建议（后端 ClassifyError 三分类）
const ERR_CODE_HINT: Record<string, string> = {
  RetryAuto: '临时错误，系统将自动重试',
  RetryManual: '可稍后重新生成',
  RetryTerminal: '参数或配额问题，请调整后重试',
}

const STYLE_OPTIONS = [
  { value: 'general', label: '通用' },
  { value: 'anime', label: '动漫' },
]
const MOVEMENT_OPTIONS = [
  { value: 'auto', label: '自动' },
  { value: 'small', label: '小幅度' },
  { value: 'medium', label: '中幅度' },
  { value: 'large', label: '大幅度' },
]
const AUDIO_TYPE_OPTIONS = [
  { value: 'all', label: '音效 + 人声' },
  { value: 'speech_only', label: '仅人声' },
  { value: 'sound_effect_only', label: '仅音效' },
]
const EMOTION_OPTIONS = [
  { value: 'happy', label: '开心' },
  { value: 'sad', label: '悲伤' },
  { value: 'angry', label: '愤怒' },
  { value: 'fearful', label: '恐惧' },
  { value: 'disgusted', label: '厌恶' },
  { value: 'surprised', label: '惊讶' },
  { value: 'calm', label: '平静' },
]

const ASSET_ACCEPT: Record<string, string> = {
  image: 'image/png,image/jpeg,image/webp',
  audio: 'audio/mpeg,audio/mp4,audio/wav',
  any: 'image/png,image/jpeg,image/webp,audio/mpeg,audio/mp4,audio/wav',
}

// ---- 模型库聚合：模型 → 支持的端点（模式）----
interface ModelEntry {
  model: string
  family: string
  endpoints: Array<{ sub_type: string; capability: ModelCapability }>
}

function buildModelMap(types: GenerationType[]): ModelEntry[] {
  const map = new Map<string, ModelEntry>()
  for (const t of types) {
    for (const m of t.models) {
      let entry = map.get(m.model)
      if (!entry) {
        entry = { model: m.model, family: m.capability.family || '', endpoints: [] }
        map.set(m.model, entry)
      }
      entry.endpoints.push({ sub_type: t.sub_type, capability: m.capability })
    }
  }
  return Array.from(map.values()).sort((a, b) => {
    if (a.family !== b.family) return a.family.localeCompare(b.family)
    return a.model.localeCompare(b.model)
  })
}

const endpointCategory = (st: string) => SUBTYPE_META[st]?.category || 'other'

// 文件名（URL 末段）
const fileNameOf = (url: string) => {
  try {
    return decodeURIComponent(url.split('/').pop() || '')
  } catch {
    return url.split('/').pop() || ''
  }
}

// ---- 素材库选择器（返回完整资产对象——引用需携带 name/kind）----
function AssetPicker(props: {
  open: boolean
  mode: 'single' | 'multi'
  accept: 'image' | 'audio' | 'any'
  title: string
  max?: number
  onClose: () => void
  onSelect: (assets: MediaAsset[]) => void
}) {
  const { open, mode, accept, title, max, onClose, onSelect } = props
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<MediaAsset[]>([])
  const [uploading, setUploading] = useState(false)

  const { data: assets = [] } = useQuery({
    queryKey: ['media-assets'],
    queryFn: () => businessApi.listAssets().then(r => r.assets),
    enabled: open,
  })

  const filtered = useMemo(() => {
    const kinds = accept === 'any' ? ['image', 'audio'] : [accept]
    return assets.filter(a => kinds.some(k => a.mime.startsWith(k)))
  }, [assets, accept])

  const uploadProps = {
    accept: ASSET_ACCEPT[accept],
    showUploadList: false,
    beforeUpload: async (file: File) => {
      setUploading(true)
      try {
        await businessApi.uploadAsset(file)
        message.success('上传成功')
        queryClient.invalidateQueries({ queryKey: ['media-assets'] })
      } catch {
        // 错误已由拦截器提示
      } finally {
        setUploading(false)
      }
      return false
    },
  }

  const toggleSelect = (a: MediaAsset) => {
    if (mode === 'single') {
      setSelected([a])
      onSelect([a])
      onClose()
      return
    }
    setSelected(prev => {
      if (prev.some(x => x.id === a.id)) return prev.filter(x => x.id !== a.id)
      if (max && prev.length >= max) {
        message.warning(`最多选择 ${max} 个`)
        return prev
      }
      return [...prev, a]
    })
  }

  const confirm = () => {
    if (selected.length === 0) {
      message.warning('请先选择素材')
      return
    }
    onSelect(selected)
    setSelected([])
    onClose()
  }

  return (
    <Modal
      title={title}
      open={open}
      onCancel={() => { setSelected([]); onClose() }}
      width={680}
      footer={mode === 'multi' ? [
        <Button key="cancel" onClick={() => { setSelected([]); onClose() }}>取消</Button>,
        <Button key="ok" type="primary" onClick={confirm} disabled={selected.length === 0}>
          确认选择（{selected.length}）
        </Button>,
      ] : null}
    >
      <Dragger {...uploadProps} disabled={uploading} style={{ marginBottom: 16 }}>
        <p className="ant-upload-drag-icon"><UploadOutlined /></p>
        <p className="ant-upload-text">点击或拖拽上传素材</p>
        <p className="ant-upload-hint">图片 png/jpg/webp · 音频 mp3/m4a/wav · 单文件 ≤ 20MB</p>
      </Dragger>

      {filtered.length === 0 ? (
        <Empty description="暂无素材，先上传一个吧" />
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, maxHeight: 320, overflow: 'auto' }}>
          {filtered.map(a => {
            const isImage = a.mime.startsWith('image')
            const active = selected.some(x => x.id === a.id)
            return (
              <div
                key={a.id}
                onClick={() => toggleSelect(a)}
                style={{
                  border: active ? '2px solid var(--wr-primary)' : '1px solid var(--wr-border, #e5e7eb)',
                  borderRadius: 8, overflow: 'hidden', cursor: 'pointer', position: 'relative',
                  background: '#fff',
                }}
              >
                <div style={{ height: 90, display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f5f5f5' }}>
                  {isImage ? (
                    <img src={a.url} alt={a.id} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
                  ) : (
                    <SoundOutlined style={{ fontSize: 28, color: '#8c8c8c' }} />
                  )}
                </div>
                <div style={{ padding: '6px 8px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Text style={{ fontSize: 11 }} type="secondary">
                    {Math.round(a.size_bytes / 1024)}KB · {isImage ? '图片' : '音频'}
                  </Text>
                  <Popconfirm title="删除该素材？" onConfirm={async () => { await businessApi.deleteAsset(a.id); queryClient.invalidateQueries({ queryKey: ['media-assets'] }) }}>
                    <DeleteOutlined style={{ color: '#999', fontSize: 12 }} onClick={e => e.stopPropagation()} />
                  </Popconfirm>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </Modal>
  )
}

// ---- 任务产物预览（任务表格内）----
function CreationPreview({ task }: { task: GenerationTask }) {
  const [preview, setPreview] = useState<{ url: string; cover?: string } | null>(null)
  const creations = task.creations || []

  if (creations.length === 0) return <Text type="secondary" style={{ fontSize: 12 }}>—</Text>

  return (
    <>
      <Space size={4}>
        {creations.map((c, i) => {
          const url = c.stored_url || c.url
          return (
            <Tooltip key={i} title="点击预览">
              <a onClick={() => setPreview({ url, cover: c.cover_url })} style={{ display: 'inline-block' }}>
                {task.type === 'video' || task.type === 'digital_human' ? (
                  <PlayCircleOutlined style={{ fontSize: 18, color: 'var(--wr-primary)' }} />
                ) : task.type === 'image' ? (
                  <FileImageOutlined style={{ fontSize: 18, color: 'var(--wr-primary)' }} />
                ) : (
                  <SoundOutlined style={{ fontSize: 18, color: 'var(--wr-primary)' }} />
                )}
              </a>
            </Tooltip>
          )
        })}
      </Space>
      <Modal
        open={!!preview}
        title="生成结果"
        footer={null}
        onCancel={() => setPreview(null)}
        width={task.type === 'image' ? 420 : 640}
      >
        {preview && task.type === 'image' ? (
          <img src={preview.url} alt="生成图片" style={{ width: '100%', borderRadius: 8 }} />
        ) : preview && (task.type === 'video' || task.type === 'digital_human') ? (
          <video src={preview.url} poster={preview.cover} controls style={{ width: '100%', borderRadius: 8 }} />
        ) : preview ? (
          <audio src={preview.url} controls style={{ width: '100%' }} />
        ) : null}
        {preview && <Text type="secondary" style={{ display: 'block', marginTop: 8, fontSize: 12, wordBreak: 'break-all' }}>{preview.url}</Text>}
      </Modal>
    </>
  )
}

// 任务 params 可能是 JSON 字符串（后端 ParamsJSON）或对象——统一解析
function parseTaskParams(r: GenerationTask): Record<string, any> {
  const raw = r.params as unknown
  if (raw && typeof raw === 'object') return raw as Record<string, any>
  if (typeof raw === 'string' && raw) {
    try {
      return JSON.parse(raw)
    } catch {
      return {}
    }
  }
  return {}
}

// ---- 创作工作台（即梦式布局：左画布 + 右面板）----
export default function CreationWorkbench() {
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState('create')
  const [category, setCategory] = useState<string>('video')
  const [model, setModel] = useState<string>('')
  const [subType, setSubType] = useState<string>('')
  const [params, setParams] = useState<Record<string, any>>({})
  const [offPeak, setOffPeak] = useState(false)
  const [watermark, setWatermark] = useState(false)
  const [brandId, setBrandId] = useState<string>('')
  const [taskFilter, setTaskFilter] = useState<string>('all')
  const [picker, setPicker] = useState<{ mode: 'single' | 'multi'; accept: 'image' | 'audio' | 'any'; key: string; title: string; max?: number } | null>(null)

  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then(r => r.types),
  })

  const { data: tasks = [] } = useQuery({
    queryKey: ['generation-tasks'],
    queryFn: () => businessApi.listGenerationTasks().then(r => r.tasks),
    refetchInterval: (query) => {
      const list = (query.state.data as GenerationTask[] | undefined) ?? []
      return list.some(t => ACTIVE_STATES.includes(t.state)) ? 5000 : false
    },
  })

  const submitMutation = useMutation({
    mutationFn: (data: Parameters<typeof businessApi.submitGenerationTask>[0]) => businessApi.submitGenerationTask(data),
    onSuccess: () => {
      message.success('生成任务已提交')
      queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })
    },
    onError: (e) => message.error(`提交失败：${(e as Error).message || '请检查参数'}`),
  })

  const cancelMutation = useMutation({
    mutationFn: (id: string) => businessApi.cancelGenerationTask(id),
    onSuccess: () => {
      message.success('已取消')
      queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })
    },
  })

  // ---- 模型 / 模式 ----
  const modelEntries = useMemo(() => buildModelMap(types), [types])
  const catModels = useMemo(
    () => modelEntries.filter(e => e.endpoints.some(ep => endpointCategory(ep.sub_type) === category)),
    [modelEntries, category],
  )
  const currentModel = useMemo(() => modelEntries.find(e => e.model === model), [modelEntries, model])
  const modes = currentModel?.endpoints || []
  const cap = useMemo(() => modes.find(m => m.sub_type === subType)?.capability, [modes, subType])

  const selectModel = (m: string) => {
    setModel(m)
    setParams({})
    const entry = modelEntries.find(e => e.model === m)
    setSubType(entry?.endpoints[0]?.sub_type || '')
  }

  // 引用能力（该模式支持 @引用 的类型）
  const refKinds = useMemo(() => {
    if (!cap) return [] as string[]
    const kinds: string[] = []
    if (cap.image_slots !== 0 || ['digital_human', 'multiframe', 'subject', 'reference2video', 'text2image', 'img2video', 'start_end2video'].includes(subType)) {
      kinds.push('image')
    }
    if (['voice_clone', 'digital_human'].includes(subType)) kinds.push('audio')
    if (subType === 'reference2video' && (cap.video_slots ?? 0) > 0) kinds.push('video')
    return kinds
  }, [cap, subType])

  // refs 视图
  const refs: PromptRef[] = (params.refs as PromptRef[]) || []

  // 素材选择回调 → 引用统一入 refs（服务端翻译层按端点映射参数）
  const onAssetPicked = (assets: MediaAsset[]) => {
    if (!picker) return
    const newRefs = assets.map(a => ({
      id: a.id,
      name: fileNameOf(a.url),
      url: a.url,
      kind: (a.mime.startsWith('audio') ? 'audio' : a.mime.startsWith('video') ? 'video' : 'image') as PromptRef['kind'],
    }))
    setParams(prev => {
      const next = { ...prev }
      if (picker.key === 'refs') {
        next.refs = [...(prev.refs || []), ...newRefs]
        // prompt 末尾追加 @名称（服务端翻译层还原为纯名称）
        const prompt = (prev.prompt as string) || ''
        const marks = newRefs.map(r => `@${r.name}`).join(' ')
        next.prompt = prompt ? `${prompt} ${marks}` : marks
      } else if (picker.key === 'subjects') {
        // 主体模式：本次选择作为第一个主体的图
        const subjects = Array.isArray(prev.subjects) ? (prev.subjects as any[]).map(s => ({ ...s })) : []
        if (subjects[0]) {
          subjects[0].images = (subjects[0].images || []).concat(newRefs.map(r => r.url))
        }
        next.subjects = subjects
      }
      return next
    })
  }

  const removeRef = (i: number) => {
    setParams(prev => {
      const next = { ...prev }
      const list = [...(prev.refs || [])]
      const removed = list.splice(i, 1)
      next.refs = list
      // 同步移除 prompt 中的 @名称
      const prompt = (prev.prompt as string) || ''
      for (const r of removed) {
        next.prompt = prompt.replace(`@${r.name}`, r.name).replace(/\s+$/, '')
      }
      return next
    })
  }

  // 打开引用选择器（按模式支持的类型）
  const openRefPicker = () => {
    if (refKinds.length === 0) return
    const accept = refKinds.length > 1 ? 'any' : (refKinds[0] as 'image' | 'audio')
    setPicker({
      mode: 'multi', accept, key: 'refs',
      title: refKinds.length > 1 ? '从素材库引用（图片/音频）' : `从素材库引用（${refKinds[0] === 'image' ? '图片' : refKinds[0] === 'audio' ? '音频' : '视频'}）`,
      max: 7,
    })
  }

  const regenerate = (task: GenerationTask) => {
    setActiveTab('create')
    setCategory(endpointCategory(task.sub_type))
    setModel(task.model)
    setSubType(task.sub_type)
    setParams(parseTaskParams(task))
    setOffPeak(task.off_peak)
    setWatermark(task.watermark)
    message.info('已回填参数，可调整后重新提交')
  }

  const canSubmit = model && subType && cap

  const submit = () => {
    if (!canSubmit) {
      message.warning('请选择模型与生成模式')
      return
    }
    const clean: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(params)) {
      if (v === undefined || v === null || v === '') continue
      if (Array.isArray(v) && v.length === 0) continue
      clean[k] = v
    }
    const submitRefs = (clean.refs as PromptRef[]) || []
    delete clean.refs
    if (submitRefs.length > 0 && refKinds.length === 0) {
      message.warning('该模式不支持素材引用')
      return
    }
    // 必填校验（服务端兜底）
    if (subType === 'tts' && !clean.text) { message.warning('请输入合成文本'); return }
    if (subType === 'tts' && !(clean.voice_setting_voice_id as string)) { message.warning('请输入音色 ID'); return }
    if (subType === 'voice_clone' && !submitRefs.some(r => r.kind === 'audio') && !clean.audio_url) { message.warning('请引用音频素材（声音克隆的原料）'); return }
    if (subType === 'digital_human' && !submitRefs.some(r => r.kind === 'image') && !clean.image) { message.warning('请引用人像图片'); return }
    if (subType === 'sound_effect' && !(clean.timing_prompts as any[])?.length) { message.warning('请至少添加一个音效事件'); return }
    if (!clean.prompt && !clean.text && subType !== 'subject' && subType !== 'multiframe') {
      if (subType !== 'tts' && subType !== 'voice_clone' && subType !== 'sound_effect') {
        message.warning('请输入提示词/文本'); return
      }
    }
    submitMutation.mutate({
      brand_id: brandId || undefined,
      sub_type: subType,
      model,
      params: clean,
      refs: submitRefs,
      off_peak: offPeak,
      watermark,
    })
  }

  // ---- 画布：当前模式最近成功产物 ----
  const canvasResult = useMemo(() => {
    if (!subType) return null
    return tasks
      .filter(t => t.sub_type === subType && t.state === 'success' && (t.creations || []).length > 0)
      .sort((a, b) => (b.created_at || '').localeCompare(a.created_at || ''))[0] || null
  }, [tasks, subType])

  // ---- 能力字段（折叠进高级参数）----
  const renderField = (kind: string): ReactNode | null => {
    if (!cap) return null
    const set = (key: string, value: any) => setParams(prev => ({ ...prev, [key]: value }))
    const get = (key: string) => params[key]

    switch (kind) {
      case 'duration': {
        const [min, max] = cap.durations || [0, 0]
        if (max <= 0) return null
        if (min === max) {
          return (
            <div key="duration" style={{ marginBottom: 12 }}>
              <Text strong style={{ fontSize: 13 }}>时长</Text>
              <Tag style={{ marginLeft: 8 }}>{min} 秒（固定）</Tag>
            </div>
          )
        }
        return (
          <div key="duration" style={{ marginBottom: 12 }}>
            <div style={{ marginBottom: 4 }}>
              <Text strong style={{ fontSize: 13 }}>时长</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>{min}-{max} 秒</Text>
            </div>
            <Slider min={min} max={max} step={1} value={get('duration') ?? max} onChange={v => set('duration', v)} />
          </div>
        )
      }
      case 'resolution': {
        const opts = cap.resolutions || []
        if (opts.length === 0) return null
        return (
          <div key="resolution" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>分辨率</Text>
            <Select size="small" style={{ width: 140, marginLeft: 10 }} value={get('resolution') || opts[0]} onChange={v => set('resolution', v)} options={opts.map(o => ({ value: o, label: o }))} />
          </div>
        )
      }
      case 'aspect': {
        const opts = cap.aspect_ratios || []
        if (opts.length === 0) return null
        return (
          <div key="aspect" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>画面比例</Text>
            <Select size="small" style={{ width: 140, marginLeft: 10 }} value={get('aspect_ratio') || '16:9'} onChange={v => set('aspect_ratio', v)} options={opts.map(o => ({ value: o, label: o }))} />
          </div>
        )
      }
      case 'audio': {
        if (!cap.audio_types && !cap.audio_default) return null
        return (
          <div key="audio" style={{ marginBottom: 12 }}>
            <Space size={12} wrap>
              <Space size={6}>
                <Text strong style={{ fontSize: 13 }}>生成音频</Text>
                <Switch size="small" checked={get('audio') ?? cap.audio_default ?? false} onChange={v => set('audio', v)} />
              </Space>
              {get('audio') && (
                <Select size="small" style={{ width: 150 }} placeholder="音频类型" value={get('audio_type')} onChange={v => set('audio_type', v)} options={AUDIO_TYPE_OPTIONS} />
              )}
            </Space>
          </div>
        )
      }
      case 'seed':
        return (
          <div key="seed" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>随机种子</Text>
            <InputNumber size="small" style={{ marginLeft: 10, width: 140 }} min={0} max={9999} placeholder="留空=随机" value={get('seed')} onChange={v => set('seed', v)} />
          </div>
        )
      case 'style':
        return (
          <div key="style" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>风格</Text>
            <Select size="small" style={{ width: 140, marginLeft: 10 }} allowClear placeholder="不指定" value={get('style')} onChange={v => set('style', v)} options={STYLE_OPTIONS} />
          </div>
        )
      case 'movement':
        if (!cap.supports_movement) return null
        return (
          <div key="movement" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>运动幅度</Text>
            <Select size="small" style={{ width: 140, marginLeft: 10 }} value={get('movement_amplitude') || 'auto'} onChange={v => set('movement_amplitude', v)} options={MOVEMENT_OPTIONS} />
          </div>
        )
      case 'voice_id':
        return (
          <div key="voice_id" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>声音 ID</Text>
            <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>8-256 位，字母开头（复用音色）</Text>
            <Input size="small" style={{ marginTop: 4, maxWidth: 300 }} value={get('voice_id')} onChange={e => set('voice_id', e.target.value)} placeholder="如 my-voice-001" />
          </div>
        )
      case 'voice_settings':
        return (
          <div key="voice_settings" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>声音设置</Text>
            <div style={{ display: 'flex', gap: 12, marginTop: 6, flexWrap: 'wrap' }}>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>语速</Text>
                <InputNumber size="small" min={0.5} max={2} step={0.1} style={{ width: 80, marginLeft: 4 }} value={get('voice_setting_speed')} onChange={v => set('voice_setting_speed', v)} placeholder="1.0" />
              </div>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>音量</Text>
                <InputNumber size="small" min={0} max={10} style={{ width: 80, marginLeft: 4 }} value={get('voice_setting_volume')} onChange={v => set('voice_setting_volume', v)} placeholder="5" />
              </div>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>语调</Text>
                <InputNumber size="small" min={-12} max={12} style={{ width: 80, marginLeft: 4 }} value={get('voice_setting_pitch')} onChange={v => set('voice_setting_pitch', v)} placeholder="0" />
              </div>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>情绪</Text>
                <Select size="small" style={{ width: 100, marginLeft: 4 }} allowClear placeholder="平静" value={get('voice_setting_emotion')} onChange={v => set('voice_setting_emotion', v)} options={EMOTION_OPTIONS} />
              </div>
            </div>
          </div>
        )
      case 'keyframes': {
        const frames: any[] = (get('image_settings') as any[]) || []
        const setFrames = (next: any[]) => set('image_settings', next)
        return (
          <div key="keyframes" style={{ marginBottom: 12 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>关键帧（2-9 个）</Text>
              <Space style={{ marginLeft: 10 }}>
                <InputNumber size="small" min={2} max={9} value={frames.length || 2} onChange={v => {
                  const n = v ?? 2
                  setFrames(Array.from({ length: n }, (_, i) => frames[i] || { key_image: '', prompt: '', duration: 5 }))
                }} />
                <Button size="small" icon={<PlusOutlined />} onClick={() => frames.length < 9 && setFrames([...frames, { key_image: '', prompt: '', duration: 5 }])} />
              </Space>
            </div>
            {frames.map((f, i) => (
              <div key={i} style={{ display: 'flex', gap: 6, alignItems: 'center', marginBottom: 6 }}>
                <Text type="secondary" style={{ fontSize: 12, width: 26 }}>#{i + 1}</Text>
                {f.key_image ? (
                  <img src={f.key_image} alt="" style={{ width: 36, height: 36, borderRadius: 6, objectFit: 'cover', border: '1px solid #eee' }} />
                ) : (
                  <Button size="small" icon={<PictureOutlined />} onClick={() => setPicker({ mode: 'single', accept: 'image', key: 'subjects', title: `选择关键帧 #${i + 1}` })}>选图</Button>
                )}
                <Input size="small" style={{ width: 140 }} placeholder={`帧 ${i + 1} 提示词`} value={f.prompt} onChange={e => setFrames(frames.map((x, j) => j === i ? { ...x, prompt: e.target.value } : x))} />
                <InputNumber size="small" min={2} max={7} placeholder="秒" value={f.duration} onChange={v => setFrames(frames.map((x, j) => j === i ? { ...x, duration: v } : x))} />
              </div>
            ))}
          </div>
        )
      }
      case 'subjects': {
        if (!cap.supports_subjects) return null
        const subjects: any[] = (get('subjects') as any[]) || []
        const setSubjects = (next: any[]) => set('subjects', next)
        return (
          <div key="subjects" style={{ marginBottom: 12 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>主体模式</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>提示词中用 @名称 引用</Text>
            </div>
            {subjects.length === 0 ? (
              <Button size="small" icon={<PlusOutlined />} onClick={() => setSubjects([{ name: '', images: [], voice_id: '' }])}>添加主体</Button>
            ) : (
              subjects.map((s, i) => (
                <div key={i} style={{ display: 'flex', gap: 6, alignItems: 'center', marginBottom: 6 }}>
                  <Input size="small" style={{ width: 100 }} placeholder="主体名称" value={s.name} onChange={e => setSubjects(subjects.map((x, j) => j === i ? { ...x, name: e.target.value } : x))} />
                  <Button size="small" icon={<PictureOutlined />} onClick={() => setPicker({ mode: 'multi', accept: 'image', key: 'subjects', title: '选择主体图片', max: 3 })}>
                    {s.images?.length ? `已选 ${s.images.length} 图` : '选图'}
                  </Button>
                  <Input size="small" style={{ width: 110 }} placeholder="音色 ID" value={s.voice_id} onChange={e => setSubjects(subjects.map((x, j) => j === i ? { ...x, voice_id: e.target.value } : x))} />
                  <MinusCircleOutlined style={{ color: '#999' }} onClick={() => setSubjects(subjects.filter((_, j) => j !== i))} />
                </div>
              ))
            )}
          </div>
        )
      }
      case 'timing': {
        const duration = Number(params.duration) || 10
        const events: any[] = (get('timing_prompts') as any[]) || []
        const setEvents = (next: any[]) => set('timing_prompts', next)
        return (
          <div key="timing" style={{ marginBottom: 12 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>音效事件</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>区间需在 [0,{duration}] 内</Text>
            </div>
            {events.map((e, i) => (
              <div key={i} style={{ display: 'flex', gap: 6, alignItems: 'center', marginBottom: 6 }}>
                <Text type="secondary" style={{ fontSize: 12, width: 26 }}>#{i + 1}</Text>
                <InputNumber size="small" min={0} max={duration} style={{ width: 70 }} placeholder="开始" value={e.from} onChange={v => setEvents(events.map((x, j) => j === i ? { ...x, from: v } : x))} />
                <Text type="secondary" style={{ fontSize: 12 }}>→</Text>
                <InputNumber size="small" min={0} max={duration} style={{ width: 70 }} placeholder="结束" value={e.to} onChange={v => setEvents(events.map((x, j) => j === i ? { ...x, to: v } : x))} />
                <Input size="small" style={{ width: 150 }} placeholder="音效描述" value={e.prompt} onChange={ev => setEvents(events.map((x, j) => j === i ? { ...x, prompt: ev.target.value } : x))} />
                <MinusCircleOutlined style={{ color: '#999' }} onClick={() => setEvents(events.filter((_, j) => j !== i))} />
              </div>
            ))}
            <Button size="small" icon={<PlusOutlined />} onClick={() => setEvents([...events, { from: 0, to: 2, prompt: '' }])}>添加事件</Button>
          </div>
        )
      }
      case 'subject_name':
        return (
          <div key="subject_name" style={{ marginBottom: 12 }}>
            <Text strong style={{ fontSize: 13 }}>主体名称</Text>
            <Input size="small" style={{ marginTop: 4, maxWidth: 300 }} value={get('name')} onChange={e => set('name', e.target.value)} placeholder="如：我的咖啡品牌 IP" />
          </div>
        )
      default:
        return null
    }
  }

  // 端点 → 高级参数字段
  const fieldsFor = (st: string): string[] => {
    switch (st) {
      case 'text2video': return ['duration', 'resolution', 'aspect', 'audio', 'style', 'movement', 'seed']
      case 'img2video': return ['duration', 'resolution', 'movement', 'seed']
      case 'start_end2video': return ['duration', 'resolution', 'movement', 'seed']
      case 'reference2video': return ['subjects', 'duration', 'resolution', 'aspect', 'movement', 'seed']
      case 'multiframe': return ['keyframes', 'resolution', 'aspect']
      case 'text2image': return ['seed']
      case 'text2audio': return ['duration', 'seed']
      case 'sound_effect': return ['timing', 'duration', 'seed']
      case 'tts': return ['voice_settings', 'voice_id']
      case 'voice_clone': return ['voice_id']
      case 'digital_human': return ['voice_id', 'resolution']
      case 'subject': return ['subject_name', 'voice_id']
      default: return []
    }
  }

  // 该模式主输入类型（画布/提示词引导）
  const mainInputKind = useMemo(() => {
    if (subType === 'tts' || subType === 'voice_clone') return 'text' as const
    if (subType === 'sound_effect') return 'events' as const
    return 'prompt' as const
  }, [subType])

  // ---- 右面板：模型/模式/提示词/引用/高级参数 ----
  const rightPanel = (
    <div className="wr-glass-card" style={{ width: 460, flexShrink: 0, padding: '20px 24px' }}>
      {/* 模型选择 */}
      <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: 14 }}>
        <Select
          style={{ flex: 1 }} size="large" value={model || undefined} placeholder="选择模型"
          onChange={selectModel}
          options={catModels.map(e => ({ value: e.model, label: `${e.model}（${e.endpoints.length} 模式）` }))}
          popupMatchSelectWidth={false}
        />
        {currentModel && <Tag color="blue" style={{ marginInlineEnd: 0 }}>{currentModel.family}</Tag>}
      </div>

      {/* 模式 pills */}
      {modes.length > 0 && (
        <Space size={6} wrap style={{ marginBottom: 14 }}>
          {modes.map(m => {
            const isActive = m.sub_type === subType
            return (
              <Tooltip key={m.sub_type} title={SUBTYPE_META[m.sub_type]?.desc}>
                <Tag.CheckableTag
                  checked={isActive}
                  onChange={() => { setSubType(m.sub_type); setParams({}) }}
                  style={{
                    fontSize: 12, padding: '3px 12px', borderRadius: 16,
                    border: isActive ? '1px solid var(--wr-primary)' : '1px solid #e5e7eb',
                    background: isActive ? 'var(--wr-primary)' : '#fff',
                    color: isActive ? '#fff' : 'inherit',
                  }}
                >
                  {SUBTYPE_META[m.sub_type]?.label || m.sub_type}
                </Tag.CheckableTag>
              </Tooltip>
            )
          })}
        </Space>
      )}

      {/* 主输入 */}
      {cap && mainInputKind === 'prompt' && (
        <div style={{ marginBottom: 14 }}>
          <TextArea
            rows={4} value={(params.prompt as string) || ''}
            onChange={e => setParams(prev => ({ ...prev, prompt: e.target.value }))}
            showCount maxLength={cap.max_prompt_len || 5000}
            placeholder="描述你想要生成的内容…在下方引用素材可 @ 自动带入"
            style={{ fontSize: 14, borderRadius: 10 }}
          />
        </div>
      )}
      {cap && mainInputKind === 'text' && (
        <div style={{ marginBottom: 14 }}>
          <TextArea
            rows={4} value={(params.text as string) || ''}
            onChange={e => setParams(prev => ({ ...prev, text: e.target.value }))}
            showCount maxLength={cap.max_prompt_len || 10000}
            placeholder="输入要合成/复刻的文本…"
            style={{ fontSize: 14, borderRadius: 10 }}
          />
        </div>
      )}

      {/* 参考素材（引用）——即梦式横排 */}
      {cap && refKinds.length > 0 && (
        <div style={{ marginBottom: 14 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <Text strong style={{ fontSize: 13 }}>参考素材</Text>
            <Button size="small" type="dashed" icon={<PlusOutlined />} onClick={openRefPicker}>@ 引用</Button>
          </div>
          {refs.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>从素材库引用图片/音频，提交时自动翻译为参数</Text>
          ) : (
            <Space size={6} wrap>
              {refs.map((r, i) => (
                <div key={i} style={{ position: 'relative' }}>
                  {r.kind === 'image' ? (
                    <img src={r.url} alt={r.name} style={{ width: 56, height: 56, borderRadius: 8, objectFit: 'cover', border: '1px solid #eee' }} />
                  ) : (
                    <div style={{ width: 56, height: 56, borderRadius: 8, border: '1px solid #eee', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f7f7f8' }}>
                      <SoundOutlined style={{ fontSize: 20, color: '#8c8c8c' }} />
                    </div>
                  )}
                  <Tooltip title={`@${r.name}`}>
                    <CloseCircleOutlined
                      onClick={() => removeRef(i)}
                      style={{ position: 'absolute', top: -6, right: -6, color: '#ff4d4f', background: '#fff', borderRadius: '50%', fontSize: 14 }}
                    />
                  </Tooltip>
                </div>
              ))}
            </Space>
          )}
        </div>
      )}

      {/* 高级参数 */}
      {cap && fieldsFor(subType).some(k => renderField(k) !== null) && (
        <Collapse
          size="small" ghost
          items={[{
            key: 'adv',
            label: <span><SettingOutlined style={{ marginRight: 6 }} />高级参数</span>,
            children: <div>{fieldsFor(subType).map(k => renderField(k))}</div>,
          }]}
          style={{ marginBottom: 14 }}
        />
      )}
      {cap && fieldsFor(subType).length === 0 && subType === 'tts' && (
        <Alert type="info" showIcon style={{ marginBottom: 14 }} message="语音合成：音色 ID 为必填（后续从声音克隆任务复用）" />
      )}

      {/* 提交区 */}
      {cap && (
        <div>
          <Space style={{ marginBottom: 10 }} size={14}>
            {brands.length > 0 && (
              <Space size={4}>
                <Text type="secondary" style={{ fontSize: 12 }}>品牌</Text>
                <Select size="small" style={{ width: 130 }} value={brandId || undefined} placeholder="选择" onChange={setBrandId}
                  options={brands.map((b: any) => ({ value: b.id, label: b.name }))} allowClear />
              </Space>
            )}
            <Space size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>错峰</Text>
              <Tooltip title="错峰模式积分更低，48 小时内完成">
                <Switch size="small" checked={offPeak} onChange={setOffPeak} />
              </Tooltip>
            </Space>
            <Space size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>水印</Text>
              <Switch size="small" checked={watermark} onChange={setWatermark} />
            </Space>
          </Space>
          <Button
            type="primary" size="large" block icon={<ThunderboltOutlined />}
            loading={submitMutation.isPending} onClick={submit}
            style={{ height: 44, borderRadius: 10, fontSize: 15 }}
          >
            立即生成
          </Button>
        </div>
      )}
      {!cap && model && <Alert type="warning" showIcon message="该模式暂无能力配置（可能已停用），请切换模式" />}
    </div>
  )

  // ---- 画布（结果区）----
  const canvasView = (
    <div
      className="wr-glass-card"
      style={{
        flex: 1, minWidth: 0, minHeight: 620,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        background: 'linear-gradient(145deg, #f8f9ff 0%, #eef0fb 100%)',
        borderRadius: 16, overflow: 'hidden', position: 'relative',
      }}
    >
      {!model ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={null}
          style={{ margin: 0 }}
        >
          <div style={{ textAlign: 'center', maxWidth: 320 }}>
            <div style={{ fontSize: 40, marginBottom: 12, color: 'var(--wr-primary)' }}>
              {category === 'video' ? <VideoCameraOutlined /> : category === 'image' ? <PictureOutlined /> : category === 'audio' ? <AudioOutlined /> : category === 'digital_human' ? <RobotOutlined /> : <AppstoreOutlined />}
            </div>
            <Text strong style={{ fontSize: 15 }}>选择模型，开始创作</Text>
            <div style={{ marginTop: 6 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                在右侧选择模型与生成模式，描述你的创意后一键生成
              </Text>
            </div>
          </div>
        </Empty>
      ) : canvasResult ? (
        (() => {
          const c = canvasResult.creations[0]
          const url = c.stored_url || c.url
          const type = canvasResult.type
          return (
            <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
              {type === 'image' ? (
                <img src={url} alt="生成结果" style={{ maxWidth: '100%', maxHeight: '100%', borderRadius: 12, boxShadow: '0 12px 40px rgba(0,0,0,0.12)' }} />
              ) : type === 'video' || type === 'digital_human' ? (
                <video src={url} poster={c.cover_url} controls style={{ maxWidth: '100%', maxHeight: '100%', borderRadius: 12, boxShadow: '0 12px 40px rgba(0,0,0,0.12)' }} />
              ) : (
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 56, color: 'var(--wr-primary)', marginBottom: 12 }}><SoundOutlined /></div>
                  <audio src={url} controls style={{ width: 320 }} />
                </div>
              )}
            </div>
          )
        })()
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={null}
          style={{ margin: 0 }}
        >
          <div style={{ textAlign: 'center', maxWidth: 340 }}>
            <Text strong style={{ fontSize: 15 }}>{SUBTYPE_META[subType]?.label || subType} · {model}</Text>
            <div style={{ marginTop: 6 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {SUBTYPE_META[subType]?.desc || ''}
                {'　'}右侧填写参数后点击「立即生成」，结果将展示在这里
              </Text>
            </div>
          </div>
        </Empty>
      )}
      {/* 画布角标：当前模式 */}
      {model && (
        <div style={{ position: 'absolute', top: 14, left: 16 }}>
          <Space size={6}>
            <Tag color="blue" style={{ fontSize: 11, borderRadius: 12 }}>{model}</Tag>
            <Tag style={{ fontSize: 11, borderRadius: 12 }}>{SUBTYPE_META[subType]?.label || subType}</Tag>
          </Space>
        </div>
      )}
    </div>
  )

  // ---- 生成任务视图 ----
  const taskTypeFilter = useMemo(() => {
    const set = new Set<string>()
    tasks.forEach(t => set.add(t.sub_type))
    return Array.from(set)
  }, [tasks])

  const filteredTasks = useMemo(() => {
    if (taskFilter === 'all') return tasks
    return tasks.filter(t => t.sub_type === taskFilter)
  }, [tasks, taskFilter])

  const activeCount = tasks.filter(t => ACTIVE_STATES.includes(t.state)).length

  const taskColumns = [
    {
      title: '时间', dataIndex: 'created_at', key: 'created_at', width: 150,
      render: (v: string) => <Text style={{ fontSize: 12 }} type="secondary">{v?.replace('T', ' ').slice(5, 19)}</Text>,
    },
    {
      title: '类型', key: 'type', width: 150,
      render: (_: unknown, r: GenerationTask) => (
        <Space direction="vertical" size={0}>
          <Text style={{ fontSize: 13 }}>{SUBTYPE_META[r.sub_type]?.label || r.sub_type}</Text>
          <Tag style={{ fontSize: 11 }}>{r.model}</Tag>
        </Space>
      ),
    },
    {
      title: '内容', key: 'content', ellipsis: true,
      render: (_: unknown, r: GenerationTask) => {
        const p = parseTaskParams(r)
        const summary = p.prompt || p.text || (Array.isArray(p.timing_prompts) ? `${p.timing_prompts.length} 个音效事件` : '')
        return <Text style={{ fontSize: 12 }} type="secondary">{String(summary || '').slice(0, 60) || r.id}</Text>
      },
    },
    {
      title: '状态', key: 'state', width: 130,
      render: (_: unknown, r: GenerationTask) => {
        const meta = STATE_META[r.state] || { color: 'default', label: r.state }
        const hint = ERR_CODE_HINT[r.err_code]
        return (
          <Tooltip title={r.state === 'failed' ? (r.err_msg || r.err_code || '失败') : hint}>
            <Tag color={meta.color}>{meta.label}</Tag>
          </Tooltip>
        )
      },
    },
    {
      title: '积分', dataIndex: 'credits', key: 'credits', width: 70,
      render: (v: number) => <Text style={{ fontSize: 12 }}>{v ?? 0}</Text>,
    },
    {
      title: '产物', key: 'creations', width: 90,
      render: (_: unknown, r: GenerationTask) => <CreationPreview task={r} />,
    },
    {
      title: '操作', key: 'actions', width: 130,
      render: (_: unknown, r: GenerationTask) => (
        <Space size={0}>
          {ACTIVE_STATES.includes(r.state) ? (
            <Popconfirm title="取消该任务？" onConfirm={() => cancelMutation.mutate(r.id)}>
              <Button size="small" type="text" danger>取消</Button>
            </Popconfirm>
          ) : null}
          {['failed', 'success'].includes(r.state) && (
            <Button size="small" type="text" icon={<ReloadOutlined />} onClick={() => regenerate(r)}>重新生成</Button>
          )}
        </Space>
      ),
    },
  ]

  const tasksView = (
    <div className="wr-glass-card" style={{ padding: '16px 24px 24px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
        <Space size={12}>
          <h3 style={{ margin: 0 }}>生成任务</h3>
          <Select
            size="small" style={{ width: 160 }} value={taskFilter} onChange={setTaskFilter}
            options={[{ value: 'all', label: '全部类型' }, ...taskTypeFilter.map(st => ({ value: st, label: SUBTYPE_META[st]?.label || st }))]}
          />
        </Space>
        <Space>
          {activeCount > 0 && <Badge count={activeCount} color="processing" />}
          <Button size="small" onClick={() => queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })}>刷新</Button>
        </Space>
      </div>
      <Table
        rowKey="id" size="small" dataSource={filteredTasks} columns={taskColumns} pagination={{ pageSize: 10, showSizeChanger: false }}
      />
    </div>
  )

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        <div className="wr-page-header">
          <h1>创作工作台</h1>
          <p>选模型 · 选模式 · 引用素材 @ 自动翻译——结果即画即得</p>
        </div>

        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'create',
              label: <span><Space size={6}><ThunderboltOutlined />创作</Space></span>,
              children: (
                <>
                  {/* 分类 Tab（模型库过滤） */}
                  <Segmented
                    block value={category} onChange={(v) => { setCategory(v as string); setModel(''); setSubType(''); setParams({}) }}
                    options={CATEGORIES.map(c => ({ value: c.key, label: <span><Space size={5}>{c.icon}{c.label}</Space></span> }))}
                    style={{ marginBottom: 16 }}
                  />
                  <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
                    {canvasView}
                    {rightPanel}
                  </div>
                </>
              ),
            },
            {
              key: 'tasks',
              label: (
                <span>
                  <Space size={6}>
                    <ApartmentOutlined />生成任务
                    {activeCount > 0 && <Badge count={activeCount} color="processing" />}
                  </Space>
                </span>
              ),
              children: tasksView,
            },
          ]}
        />
      </div>

      <AssetPicker
        open={!!picker}
        mode={picker?.mode || 'single'}
        accept={picker?.accept || 'any'}
        title={picker?.title || '选择素材'}
        max={picker?.max}
        onClose={() => setPicker(null)}
        onSelect={onAssetPicked}
      />
    </div>
  )
}

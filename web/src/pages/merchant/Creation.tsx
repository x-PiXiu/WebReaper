import { useState, useMemo } from 'react'
import type { ReactNode } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Tabs, Typography, Select, Input, InputNumber, Switch, Slider, Button, Space, Tag, message,
  Table, Modal, Upload, Empty, Popconfirm, Tooltip, Alert, Divider, Badge,
} from 'antd'
import {
  VideoCameraOutlined, PictureOutlined, AudioOutlined, RobotOutlined, AppstoreOutlined,
  UploadOutlined, DeleteOutlined, ReloadOutlined, PlayCircleOutlined, SoundOutlined,
  PlusOutlined, MinusCircleOutlined, FileImageOutlined, CloseCircleOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../api/business'
import type { GenerationTask } from '../../types/api'

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
  voice_clone: { category: 'audio', label: '声音克隆', desc: '上传音频复刻音色（voice_id 永久复用）' },
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

// 资产类型 → 可接受文件类型（上传白名单）
const ASSET_ACCEPT: Record<string, string> = {
  image: 'image/png,image/jpeg,image/webp',
  audio: 'audio/mpeg,audio/mp4,audio/wav',
  any: 'image/png,image/jpeg,image/webp,audio/mpeg,audio/mp4,audio/wav',
}

// ---- 素材库选择器 ----
function AssetPicker(props: {
  open: boolean
  mode: 'single' | 'multi'
  accept: 'image' | 'audio' | 'any'
  title: string
  max?: number
  onClose: () => void
  onSelect: (urls: string[]) => void
}) {
  const { open, mode, accept, title, max, onClose, onSelect } = props
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<string[]>([])
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
      return false // 手动控制，不走 antd 自动上传
    },
  }

  const toggleSelect = (url: string) => {
    if (mode === 'single') {
      // 单选：点击即选中并回填（footer 无确认按钮）
      setSelected([url])
      onSelect([url])
      onClose()
      return
    }
    setSelected(prev => {
      if (prev.includes(url)) return prev.filter(u => u !== url)
      if (max && prev.length >= max) {
        message.warning(`最多选择 ${max} 个`)
        return prev
      }
      return [...prev, url]
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
            const active = selected.includes(a.url)
            return (
              <div
                key={a.id}
                onClick={() => toggleSelect(a.url)}
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

// ---- 任务产物预览 ----
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

// ---- 创作工作台 ----
export default function CreationWorkbench() {
  const queryClient = useQueryClient()
  const [category, setCategory] = useState('video')
  const [subType, setSubType] = useState<string>('')
  const [model, setModel] = useState<string>('')
  const [params, setParams] = useState<Record<string, any>>({})
  const [offPeak, setOffPeak] = useState(false)
  const [watermark, setWatermark] = useState(false)
  const [brandId, setBrandId] = useState<string>('')
  const [picker, setPicker] = useState<{ mode: 'single' | 'multi'; accept: 'image' | 'audio' | 'any'; key: string; title: string; max?: number; subjectIndex?: number } | null>(null)

  // 品牌（任务归属）
  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  // 端点类型 + 能力向量（表单驱动源）
  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then(r => r.types),
  })

  // 任务列表（活跃任务 5s 轮询）
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

  // 当前分类可用的端点
  const catTypes = useMemo(
    () => types.filter(t => (SUBTYPE_META[t.sub_type]?.category || 'other') === category),
    [types, category],
  )

  // 当前模型能力
  const cap = useMemo(() => {
    const t = types.find(t => t.sub_type === subType)
    return t?.models.find(m => m.model === model)?.capability
  }, [types, subType, model])

  // 切换端点：重置模型与参数
  const onSubTypeChange = (v: string) => {
    setSubType(v)
    setModel('')
    setParams({})
  }
  const onModelChange = (v: string) => {
    setModel(v)
    setParams({})
  }

  // 素材选择回调 → 回填参数键
  const onAssetPicked = (urls: string[]) => {
    if (!picker) return
    setParams(prev => {
      if (picker.key === 'images') return { ...prev, images: urls }
      if (picker.key === 'image_settings') {
        // 关键帧：按序回填 key_image
        const frames = Array.isArray(prev.image_settings) ? (prev.image_settings as any[]) : []
        const next = frames.map((f, i) => (i < urls.length ? { ...f, key_image: urls[i] } : f))
        return { ...prev, image_settings: next }
      }
      if (picker.key === 'subject_imgs') {
        // 主体图片：回填到指定主体
        const subjects = Array.isArray(prev.subjects) ? (prev.subjects as any[]).map(s => ({ ...s })) : []
        if (picker.subjectIndex !== undefined && subjects[picker.subjectIndex]) {
          subjects[picker.subjectIndex].images = urls
        }
        return { ...prev, subjects }
      }
      if (picker.key === 'timing_prompts') return prev // 事件不适用
      return { ...prev, [picker.key]: urls[0] }
    })
  }

  // 任务 params → 表单回填（重新生成）
  const regenerate = (task: GenerationTask) => {
    const meta = SUBTYPE_META[task.sub_type]
    setCategory(meta?.category || 'other')
    setSubType(task.sub_type)
    setModel(task.model)
    setParams(parseTaskParams(task))
    setOffPeak(task.off_peak)
    setWatermark(task.watermark)
    message.info('已回填参数，可调整后重新提交')
  }

  const canSubmit = subType && model && cap

  const submit = () => {
    if (!canSubmit) {
      message.warning('请选择端点与模型')
      return
    }
    // 清理空值
    const clean: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(params)) {
      if (v === undefined || v === null || v === '') continue
      if (Array.isArray(v) && v.length === 0) continue
      clean[k] = v
    }
    // 必填校验（服务端兜底，前端提前提示）
    if (subType === 'tts' && !clean.text) { message.warning('请输入合成文本'); return }
    if (subType === 'tts' && !(clean.voice_setting_voice_id as string)) { message.warning('请输入音色 ID'); return }
    if (subType === 'voice_clone' && !clean.audio_url) { message.warning('请选择原音频素材'); return }
    if (subType === 'voice_clone' && !clean.voice_id) { message.warning('请输入声音 ID（8-256 位）'); return }
    if (subType === 'digital_human' && !clean.image && !(clean.images as string[])?.length) { message.warning('请选择数字人人像'); return }
    if (subType === 'multiframe' && !clean.start_image) { message.warning('请选择首帧图'); return }
    if (subType === 'sound_effect' && !(clean.timing_prompts as any[])?.length) { message.warning('请至少添加一个音效事件'); return }
    if (!clean.prompt && !clean.text && subType !== 'subject') {
      // 除 subject 外的端点都需 prompt/text
      if (subType !== 'tts' && subType !== 'voice_clone' && subType !== 'sound_effect') {
        message.warning('请输入提示词/文本'); return
      }
    }
    submitMutation.mutate({
      brand_id: brandId || undefined,
      sub_type: subType,
      model,
      params: clean,
      off_peak: offPeak,
      watermark,
    })
  }

  // ---- 能力向量驱动表单 ----
  const renderField = (kind: string): ReactNode | null => {
    if (!cap) return null
    const set = (key: string, value: any) => setParams(prev => ({ ...prev, [key]: value }))
    const get = (key: string) => params[key]

    switch (kind) {
      case 'prompt':
        return (
          <div key="prompt" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>提示词</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
                描述画面/内容（{cap.max_prompt_len || 5000} 字上限）
              </Text>
            </div>
            <TextArea
              rows={3}
              value={get('prompt')}
              onChange={e => set('prompt', e.target.value)}
              showCount
              maxLength={cap.max_prompt_len || 5000}
              placeholder="描述你想要生成的内容，越具体效果越好…"
            />
          </div>
        )
      case 'text':
        return (
          <div key="text" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>合成文本</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>支持 &lt;#x#&gt; 停顿标记（{cap.max_prompt_len || 10000} 字上限）</Text>
            </div>
            <TextArea rows={3} value={get('text')} onChange={e => set('text', e.target.value)} showCount maxLength={cap.max_prompt_len || 10000} placeholder="输入要合成语音的文本…" />
          </div>
        )
      case 'duration': {
        const [min, max] = cap.durations || [0, 0]
        if (max <= 0) return null
        if (min === max) {
          return (
            <div key="duration" style={{ marginBottom: 16 }}>
              <Text strong style={{ fontSize: 13 }}>时长</Text>
              <Tag style={{ marginLeft: 8 }}>{min} 秒（固定）</Tag>
            </div>
          )
        }
        return (
          <div key="duration" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>时长</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>{min}-{max} 秒</Text>
            </div>
            <Slider
              min={min} max={max} step={1}
              value={get('duration') ?? max}
              onChange={v => set('duration', v)}
              marks={{ [min]: `${min}s`, [max]: `${max}s` }}
            />
          </div>
        )
      }
      case 'resolution': {
        const opts = cap.resolutions || []
        if (opts.length === 0) return null
        return (
          <div key="resolution" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>分辨率</Text>
            <Select style={{ width: 160, marginLeft: 12 }} value={get('resolution') || opts[0]} onChange={v => set('resolution', v)} options={opts.map(o => ({ value: o, label: o }))} />
          </div>
        )
      }
      case 'aspect': {
        const opts = cap.aspect_ratios || []
        if (opts.length === 0) return null
        return (
          <div key="aspect" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>画面比例</Text>
            <Select style={{ width: 160, marginLeft: 12 }} value={get('aspect_ratio') || '16:9'} onChange={v => set('aspect_ratio', v)} options={opts.map(o => ({ value: o, label: o }))} />
          </div>
        )
      }
      case 'audio': {
        if (!cap.audio_types && !cap.audio_default) return null
        return (
          <div key="audio" style={{ marginBottom: 16 }}>
            <Space size={16} wrap>
              <Space>
                <Text strong style={{ fontSize: 13 }}>生成音频</Text>
                <Switch checked={get('audio') ?? cap.audio_default ?? false} onChange={v => set('audio', v)} />
              </Space>
              {get('audio') && (
                <Select style={{ width: 180 }} placeholder="音频类型" value={get('audio_type')} onChange={v => set('audio_type', v)} options={AUDIO_TYPE_OPTIONS} />
              )}
            </Space>
          </div>
        )
      }
      case 'seed':
        return (
          <div key="seed" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>随机种子</Text>
            <InputNumber style={{ marginLeft: 12, width: 160 }} min={0} max={9999} placeholder="留空=随机" value={get('seed')} onChange={v => set('seed', v)} />
          </div>
        )
      case 'style':
        return (
          <div key="style" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>风格</Text>
            <Select style={{ width: 160, marginLeft: 12 }} allowClear placeholder="不指定" value={get('style')} onChange={v => set('style', v)} options={STYLE_OPTIONS} />
          </div>
        )
      case 'movement':
        if (!cap.supports_movement) return null
        return (
          <div key="movement" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>运动幅度</Text>
            <Select style={{ width: 160, marginLeft: 12 }} value={get('movement_amplitude') || 'auto'} onChange={v => set('movement_amplitude', v)} options={MOVEMENT_OPTIONS} />
          </div>
        )
      case 'images': {
        const slots = cap.image_slots ?? 0
        if (slots === 0) return null
        const picked = (get('images') as string[]) || []
        const need = slots === 2 ? '首帧 + 尾帧（2 张）' : slots === -1 ? `1-7 张（已选 ${picked.length}）` : '1 张'
        const max = slots === -1 ? 7 : Math.abs(slots)
        return (
          <div key="images" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>参考图片</Text>
            <Space style={{ marginLeft: 12 }}>
              <Button size="small" icon={<PictureOutlined />} onClick={() => setPicker({ mode: slots === 1 ? 'single' : 'multi', accept: 'image', key: 'images', title: '选择参考图片', max })}>
                选择素材（{need}）
              </Button>
              {picked.length > 0 && (
                <Space size={2} wrap>
                  {picked.map((u, i) => (
                    <img key={i} src={u} alt="" style={{ width: 44, height: 44, borderRadius: 6, objectFit: 'cover', border: '1px solid #eee' }} />
                  ))}
                  <Button size="small" type="text" danger icon={<CloseCircleOutlined />} onClick={() => set('images', [])} />
                </Space>
              )}
            </Space>
          </div>
        )
      }
      case 'image': {
        const picked = get('image')
        return (
          <div key="image" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>人像图片</Text>
            <Space style={{ marginLeft: 12 }}>
              <Button size="small" icon={<PictureOutlined />} onClick={() => setPicker({ mode: 'single', accept: 'image', key: 'image', title: '选择数字人人像' })}>
                {picked ? '更换人像' : '选择人像'}
              </Button>
              {picked && <img src={picked} alt="" style={{ width: 44, height: 44, borderRadius: 6, objectFit: 'cover', border: '1px solid #eee' }} />}
            </Space>
          </div>
        )
      }
      case 'start_image': {
        const picked = get('start_image')
        return (
          <div key="start_image" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>首帧图</Text>
            <Space style={{ marginLeft: 12 }}>
              <Button size="small" icon={<PictureOutlined />} onClick={() => setPicker({ mode: 'single', accept: 'image', key: 'start_image', title: '选择首帧图' })}>
                {picked ? '更换首帧' : '选择首帧'}
              </Button>
              {picked && <img src={picked} alt="" style={{ width: 44, height: 44, borderRadius: 6, objectFit: 'cover', border: '1px solid #eee' }} />}
            </Space>
          </div>
        )
      }
      case 'keyframes': {
        const frames: any[] = (get('image_settings') as any[]) || []
        const setFrames = (next: any[]) => set('image_settings', next)
        return (
          <div key="keyframes" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>关键帧（2-9 个）</Text>
              <Space style={{ marginLeft: 12 }}>
                <InputNumber size="small" min={2} max={9} value={frames.length || 2} onChange={v => {
                  const n = v ?? 2
                  const next = Array.from({ length: n }, (_, i) => frames[i] || { key_image: '', prompt: '', duration: 5 })
                  setFrames(next)
                }} />
                <Button size="small" icon={<PlusOutlined />} onClick={() => frames.length < 9 && setFrames([...frames, { key_image: '', prompt: '', duration: 5 }])} />
              </Space>
            </div>
            {frames.map((f, i) => (
              <div key={i} style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                <Text type="secondary" style={{ fontSize: 12, width: 28 }}>#{i + 1}</Text>
                {f.key_image ? (
                  <img src={f.key_image} alt="" style={{ width: 40, height: 40, borderRadius: 6, objectFit: 'cover', border: '1px solid #eee' }} />
                ) : (
                  <Button size="small" icon={<PictureOutlined />} onClick={() => setPicker({ mode: 'multi', accept: 'image', key: 'image_settings', title: `选择关键帧 #${i + 1}`, max: 1 })}>选图</Button>
                )}
                <Input size="small" style={{ width: 180 }} placeholder={`关键帧 ${i + 1} 提示词`} value={f.prompt} onChange={e => setFrames(frames.map((x, j) => j === i ? { ...x, prompt: e.target.value } : x))} />
                <InputNumber size="small" min={2} max={7} placeholder="秒" value={f.duration} onChange={v => setFrames(frames.map((x, j) => j === i ? { ...x, duration: v } : x))} />
                <MinusCircleOutlined style={{ color: '#999' }} onClick={() => frames.length > 2 && setFrames(frames.filter((_, j) => j !== i))} />
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
          <div key="subjects" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>主体模式</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>提示词中用 @名称 引用主体（最多 3 个）</Text>
            </div>
            {subjects.length === 0 ? (
              <Button size="small" icon={<PlusOutlined />} onClick={() => setSubjects([{ name: '', images: [], voice_id: '' }])}>添加主体</Button>
            ) : (
              subjects.map((s, i) => (
                <div key={i} style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                  <Input size="small" style={{ width: 110 }} placeholder="主体名称" value={s.name} onChange={e => setSubjects(subjects.map((x, j) => j === i ? { ...x, name: e.target.value } : x))} />
                  <Button size="small" icon={<PictureOutlined />} onClick={() => setPicker({ mode: 'multi', accept: 'image', key: 'subject_imgs', title: `选择主体 ${i + 1} 图片（1-3 张）`, max: 3, subjectIndex: i })}>
                    {s.images?.length ? `已选 ${s.images.length} 图` : '选图'}
                  </Button>
                  <Input size="small" style={{ width: 140 }} placeholder="音色 ID（可选）" value={s.voice_id} onChange={e => setSubjects(subjects.map((x, j) => j === i ? { ...x, voice_id: e.target.value } : x))} />
                  <MinusCircleOutlined style={{ color: '#999' }} onClick={() => setSubjects(subjects.filter((_, j) => j !== i))} />
                </div>
              ))
            )}
            {subjects.length > 0 && subjects.length < 3 && (
              <Button size="small" type="link" icon={<PlusOutlined />} onClick={() => setSubjects([...subjects, { name: '', images: [], voice_id: '' }])}>再加一个主体</Button>
            )}
          </div>
        )
      }
      case 'audio_url': {
        const picked = get('audio_url')
        return (
          <div key="audio_url" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>原音频</Text>
            <Space style={{ marginLeft: 12 }}>
              <Button size="small" icon={<AudioOutlined />} onClick={() => setPicker({ mode: 'single', accept: 'audio', key: 'audio_url', title: '选择原音频（10s-5min）' })}>
                {picked ? '更换音频' : '选择音频'}
              </Button>
              {picked && <SoundOutlined style={{ color: 'var(--wr-primary)' }} />}
            </Space>
          </div>
        )
      }
      case 'voice_id':
        return (
          <div key="voice_id" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>声音 ID</Text>
            <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>8-256 位，字母开头，可含数字/-/_（后续克隆任务复用）</Text>
            <Input style={{ marginTop: 6, maxWidth: 420 }} value={get('voice_id')} onChange={e => set('voice_id', e.target.value)} placeholder="如 my-voice-001" />
          </div>
        )
      case 'voice_settings':
        return (
          <div key="voice_settings" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>声音设置</Text>
            <div style={{ display: 'flex', gap: 16, marginTop: 8, flexWrap: 'wrap' }}>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>语速</Text>
                <InputNumber size="small" min={0.5} max={2} step={0.1} style={{ width: 90, marginLeft: 6 }} value={get('voice_setting_speed')} onChange={v => set('voice_setting_speed', v)} placeholder="1.0" />
              </div>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>音量</Text>
                <InputNumber size="small" min={0} max={10} style={{ width: 90, marginLeft: 6 }} value={get('voice_setting_volume')} onChange={v => set('voice_setting_volume', v)} placeholder="5" />
              </div>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>语调</Text>
                <InputNumber size="small" min={-12} max={12} style={{ width: 90, marginLeft: 6 }} value={get('voice_setting_pitch')} onChange={v => set('voice_setting_pitch', v)} placeholder="0" />
              </div>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>情绪</Text>
                <Select size="small" style={{ width: 110, marginLeft: 6 }} allowClear placeholder="平静" value={get('voice_setting_emotion')} onChange={v => set('voice_setting_emotion', v)} options={EMOTION_OPTIONS} />
              </div>
            </div>
          </div>
        )
      case 'timing': {
        const duration = Number(params.duration) || 10
        const events: any[] = (get('timing_prompts') as any[]) || []
        const setEvents = (next: any[]) => set('timing_prompts', next)
        return (
          <div key="timing" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6 }}>
              <Text strong style={{ fontSize: 13 }}>音效事件（时间轴）</Text>
              <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>总时长 {duration} 秒，事件区间需在 [0,{duration}] 内</Text>
            </div>
            {events.map((e, i) => (
              <div key={i} style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                <Text type="secondary" style={{ fontSize: 12, width: 28 }}>#{i + 1}</Text>
                <InputNumber size="small" min={0} max={duration} style={{ width: 80 }} placeholder="开始" value={e.from} onChange={v => setEvents(events.map((x, j) => j === i ? { ...x, from: v } : x))} />
                <Text type="secondary" style={{ fontSize: 12 }}>→</Text>
                <InputNumber size="small" min={0} max={duration} style={{ width: 80 }} placeholder="结束" value={e.to} onChange={v => setEvents(events.map((x, j) => j === i ? { ...x, to: v } : x))} />
                <Input size="small" style={{ width: 220 }} placeholder="该时段音效描述" value={e.prompt} onChange={ev => setEvents(events.map((x, j) => j === i ? { ...x, prompt: ev.target.value } : x))} />
                <MinusCircleOutlined style={{ color: '#999' }} onClick={() => setEvents(events.filter((_, j) => j !== i))} />
              </div>
            ))}
            <Button size="small" icon={<PlusOutlined />} onClick={() => setEvents([...events, { from: 0, to: 2, prompt: '' }])}>添加事件</Button>
          </div>
        )
      }
      case 'subject_name':
        return (
          <div key="subject_name" style={{ marginBottom: 16 }}>
            <Text strong style={{ fontSize: 13 }}>主体名称</Text>
            <Input style={{ marginTop: 6, maxWidth: 420 }} value={get('name')} onChange={e => set('name', e.target.value)} placeholder="如：我的咖啡品牌 IP" />
          </div>
        )
      default:
        return null
    }
  }

  // 端点 → 渲染哪些字段
  const fieldsFor = (st: string): string[] => {
    switch (st) {
      case 'text2video': return ['prompt', 'duration', 'resolution', 'aspect', 'audio', 'style', 'movement', 'seed']
      case 'img2video': return ['images', 'prompt', 'duration', 'resolution', 'movement', 'seed']
      case 'start_end2video': return ['images', 'prompt', 'duration', 'resolution', 'movement', 'seed']
      case 'reference2video': return ['subjects', 'images', 'prompt', 'duration', 'resolution', 'aspect', 'movement', 'seed']
      case 'multiframe': return ['start_image', 'keyframes', 'resolution', 'aspect']
      case 'text2image': return ['prompt', 'images', 'seed']
      case 'text2audio': return ['prompt', 'duration', 'seed']
      case 'sound_effect': return ['timing', 'duration', 'seed']
      case 'tts': return ['text', 'voice_settings', 'voice_id']
      case 'voice_clone': return ['audio_url', 'voice_id', 'text']
      case 'digital_human': return ['image', 'prompt', 'audio_url', 'voice_id', 'resolution']
      case 'subject': return ['subject_name', 'images', 'voice_id']
      default: return ['prompt']
    }
  }

  // 提交表单区
  const formArea = (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <div>
          <Text strong style={{ fontSize: 13 }}>端点</Text>
          <Select
            style={{ width: 220, marginLeft: 8 }} value={subType || undefined} placeholder="选择生成方式"
            onChange={onSubTypeChange}
            options={catTypes.map(t => ({ value: t.sub_type, label: `${SUBTYPE_META[t.sub_type]?.label || t.sub_type}（${t.models.length} 模型）` }))}
          />
        </div>
        <div>
          <Text strong style={{ fontSize: 13 }}>模型</Text>
          <Select
            style={{ width: 220, marginLeft: 8 }} value={model || undefined} placeholder={subType ? '选择模型' : '先选端点'}
            disabled={!subType} onChange={onModelChange}
            options={catTypes.find(t => t.sub_type === subType)?.models.map(m => ({ value: m.model, label: m.model })) || []}
          />
        </div>
        {brands.length > 0 && (
          <div>
            <Text strong style={{ fontSize: 13 }}>品牌</Text>
            <Select style={{ width: 180, marginLeft: 8 }} value={brandId || undefined} placeholder="选择归属品牌" onChange={setBrandId}
              options={brands.map((b: any) => ({ value: b.id, label: b.name }))} allowClear />
          </div>
        )}
      </Space>

      {subType && cap && (
        <>
          <Divider style={{ margin: '8px 0 16px' }} />
          <Alert
            type="info" showIcon style={{ marginBottom: 16 }}
            message={`${SUBTYPE_META[subType]?.label} · ${model}`}
            description={SUBTYPE_META[subType]?.desc}
          />
          {fieldsFor(subType).map(kind => renderField(kind))}

          <Space style={{ marginTop: 4 }} wrap>
            <Space size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>错峰</Text>
              <Tooltip title="错峰模式积分更低，48 小时内完成，可手动取消">
                <Switch size="small" checked={offPeak} onChange={setOffPeak} />
              </Tooltip>
            </Space>
            <Space size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>水印</Text>
              <Switch size="small" checked={watermark} onChange={setWatermark} />
            </Space>
            <Button type="primary" icon={<VideoCameraOutlined />} loading={submitMutation.isPending} onClick={submit} style={{ marginLeft: 16 }}>
              提交生成
            </Button>
          </Space>
        </>
      )}
      {subType && !cap && (
        <Alert type="warning" showIcon style={{ marginTop: 16 }} message="该模型暂无能力配置（可能已停用），请选择其他模型" />
      )}
    </div>
  )

  // 任务列表
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

  const activeCount = tasks.filter(t => ACTIVE_STATES.includes(t.state)).length

  return (
    <div className="wr-page-content wr-aurora-bg" style={{ paddingTop: 8, position: 'relative' }}>
      <div style={{ position: 'relative', zIndex: 1 }}>
        <div className="wr-page-header">
          <h1>创作工作台</h1>
          <p>视频 · 图片 · 音频 · 数字人——统一生成，能力向量驱动（端点/模型可在管理后台动态配置）</p>
        </div>

        <div className="wr-glass-card" style={{ padding: '16px 24px 24px' }}>
          <Tabs
            activeKey={category}
            onChange={setCategory}
            items={CATEGORIES.map(c => ({
              key: c.key,
              label: <span><Space size={6}>{c.icon}{c.label}</Space></span>,
            }))}
          />
          {formArea}
        </div>

        <div className="wr-glass-card" style={{ padding: '16px 24px 24px', marginTop: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <h3 style={{ margin: 0 }}>生成任务</h3>
            <Space>
              {activeCount > 0 && <Badge count={activeCount} color="processing" />}
              <Button size="small" onClick={() => queryClient.invalidateQueries({ queryKey: ['generation-tasks'] })}>刷新</Button>
            </Space>
          </div>
          <Table
            rowKey="id" size="small" dataSource={tasks} columns={taskColumns} pagination={{ pageSize: 8, showSizeChanger: false }}
          />
        </div>
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

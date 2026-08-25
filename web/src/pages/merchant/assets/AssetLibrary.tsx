import { useEffect, useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert, Button, Empty, Input, InputNumber, Modal, Popconfirm, Segmented, Select, Space, Tag, Typography, Upload, message,
} from 'antd'
import {
  SoundOutlined, PictureOutlined, VideoCameraOutlined, FileTextOutlined, UserOutlined,
  PlusOutlined, DeleteOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { useMediaAssets, MEDIA_ASSETS_QUERY_KEY } from '../../../hooks/useMediaAssets'
import { useBrandContext } from '../../../hooks/useBrands'
import AssetPicker from '../../../components/AssetPicker'
import type { GenerationTask, ModelCapability } from '../../../types/api'
import VoicePicker from '../../../components/VoicePicker'
import { MODAL_W, modalBodyScroll } from '../../../ui/modalFit'
import { submitUnified } from '../../../api/generationSubmit'

const { Text } = Typography

type TabKey = 'audio' | 'image' | 'video' | 'article' | 'digital_human'

const TABS = [
  { value: 'digital_human', label: '数字人', icon: <UserOutlined /> },
  { value: 'video', label: '视频', icon: <VideoCameraOutlined /> },
  { value: 'image', label: '图片', icon: <PictureOutlined /> },
  { value: 'audio', label: '音频', icon: <SoundOutlined /> },
  { value: 'article', label: '文章', icon: <FileTextOutlined /> },
] as const

function formatSize(bytes: number) {
  if (bytes > 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB'
  if (bytes > 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
  if (bytes > 1 << 10) return (bytes >> 10) + ' KB'
  return bytes + ' B'
}

function timeAgo(iso: string) {
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60_000)
  if (m < 1) return '刚刚'
  if (m < 60) return `${m} 分钟前`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} 小时前`
  return `${Math.floor(h / 24)} 天前`
}

interface ViduSubject {
  taskId: string
  state: string
  name: string
  serverId: string
  voiceId: string
  kind: string
  hasVideo: boolean
  imageCount: number
  errMsg: string
  createdAt: string
}

/** 解析任务 params（后端存的是 JSON 字符串） */
function taskParams(t: GenerationTask): Record<string, any> {
  if (t.params && typeof t.params === 'object') return t.params as Record<string, any>
  if (typeof t.params === 'string' && t.params) {
    try { return JSON.parse(t.params) } catch { return {} }
  }
  return {}
}

/**
 * 资产库：统一媒体库（数字人/视频/图片/音频/文章 5 tab）。
 * 数字人 tab = Vidu 主体管理（创建→列表→用于 reference2video 视频生成 @引用）。
 * 其他 tab = 真实数据（MediaAsset 素材/AI 产物 + 作品库文章）。
 */
export default function AssetLibrary() {
  const [tab, setTab] = useState<TabKey>('digital_human')
  const [q, setQ] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [generateOpen, setGenerateOpen] = useState(false)
  const [generateType, setGenerateType] = useState<'image' | 'video' | 'audio' | 'voice'>('image')
  const queryClient = useQueryClient()

  const { data: assets = [], isLoading } = useMediaAssets()

  const { data: genTasks = [], refetch: refetchSubjects } = useQuery({
    queryKey: ['generation-tasks'],
    queryFn: () => businessApi.listGenerationTasks().then(r => r.tasks).catch(() => [] as GenerationTask[]),
    enabled: tab === 'digital_human' || tab === 'audio' || generateOpen,
  })
  const subjects: ViduSubject[] = useMemo(() => {
    return (genTasks || [])
      .filter((t: GenerationTask) => t.sub_type === 'subject')
      .map((t: GenerationTask) => {
        const p = taskParams(t)
        return {
          taskId: t.id,
          state: t.state,
          name: (p?.name as string) || t.id.slice(0, 12),
          serverId: t.provider_task_id || '',
          voiceId: (p?.voice_id as string) || '',
          kind: (p?.kind as string) === 'scene' ? 'scene' : 'person',
          hasVideo: Array.isArray(p?.videos) && p.videos.length > 0,
          imageCount: Array.isArray(p?.images) ? p.images.length : 0,
          errMsg: t.err_msg || '',
          createdAt: t.created_at,
        }
      })
  }, [genTasks])

  // 我的音色库：声音克隆成功的 voice_id（创建数字人可直接绑定）
  const myVoices = useMemo(() => {
    const ids = new Set<string>()
    for (const t of genTasks || []) {
      if (t.sub_type !== 'voice_clone' || t.state !== 'success') continue
      const vid = taskParams(t).voice_id
      if (typeof vid === 'string' && vid) ids.add(vid)
    }
    return Array.from(ids)
  }, [genTasks])

  const filtered = useMemo(() => {
    const prefix = tab === 'audio' ? 'audio/' : tab === 'image' ? 'image/' : tab === 'video' ? 'video/' : ''
    let list = prefix ? assets.filter(a => a.mime.startsWith(prefix)) : []
    const needle = q.trim().toLowerCase()
    if (needle) list = list.filter(a => a.url.toLowerCase().includes(needle))
    return list
  }, [tab, assets, q])

  const { data: works = [] } = useQuery({
    queryKey: ['merchant-works'],
    queryFn: () => businessApi.listWorks().catch(() => []),
    enabled: tab === 'article',
  })
  const articles = useMemo(() => works.filter(w => w.kind === 'article'), [works])

  const hint = {
    digital_human: '通过 Vidu 主体 API 创建数字分身——上传形象照/主体视频并绑定音色，视频生成中用 server_id 引用',
    video: 'AI 生成的视频产物（Vidu 视频/数字人视频）',
    image: 'AI 生成的图片 / 上传的素材图（封面/图文创作引用源）',
    audio: '配音/音效素材（视频创作的音频引用源）',
    article: '内容合成生成的文章作品（可发布到社媒平台）',
  }[tab]

  return (
    <div className="wr-page-content ip-page">
      <div className="ip-page-hero">
        <div>
          <p className="ip-kicker">Digital Twin</p>
          <h1>素材库</h1>
          <p className="ip-lead">{hint}——形象、音色与封面素材，供口播数字人与成片取用</p>
        </div>
        {tab === 'digital_human' && (
          <Button type="primary" size="large" className="ip-btn-primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            创建数字人
          </Button>
        )}
      </div>

      <div className="ip-toolbar">
        <Segmented value={tab} onChange={v => setTab(v as TabKey)} options={TABS.map(t => ({ ...t }))} />
        <Space>
          {(tab === 'video' || tab === 'image' || tab === 'audio') && (
            <Input.Search allowClear placeholder="搜索" style={{ maxWidth: 240 }} value={q} onChange={e => setQ(e.target.value)} />
          )}
          {/* 生成按钮 */}
          {tab === 'image' && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { setGenerateType('image'); setGenerateOpen(true) }}>
              生成图片
            </Button>
          )}
          {tab === 'video' && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { setGenerateType('video'); setGenerateOpen(true) }}>
              生成视频
            </Button>
          )}
          {tab === 'audio' && (
            <Space>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { setGenerateType('audio'); setGenerateOpen(true) }}>
                生成音频
              </Button>
              <Button icon={<SoundOutlined />} onClick={() => { setGenerateType('voice'); setGenerateOpen(true) }}>
                克隆音色
              </Button>
            </Space>
          )}
        </Space>
      </div>

      {tab === 'digital_human' && (
        subjects.length === 0 ? (
          <Empty style={{ padding: 60 }} description="还没有数字人——点击「创建数字人」上传形象照/视频开始">
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>创建数字人</Button>
          </Empty>
        ) : (
          <div className="ip-asset-grid">
            {subjects.map(s => (
              <div key={s.taskId} className="ip-asset-card">
                <div className="ip-asset-cover ip-asset-cover--voice" style={{ justifyContent: 'center', alignItems: 'center' }}>
                  <UserOutlined style={{ fontSize: 48, color: 'var(--wr-accent)' }} />
                </div>
                <div className="ip-asset-body">
                  <Text strong style={{ fontSize: 14 }}>{s.name}</Text>
                  <div style={{ marginTop: 6, fontSize: 12, display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                    <Tag color={s.state === 'success' ? 'green' : s.state === 'failed' ? 'red' : 'processing'}>
                      {s.state === 'success' ? '就绪' : s.state === 'failed' ? '失败' : '创建中'}
                    </Tag>
                    <Tag style={{ margin: 0 }} color={s.kind === 'scene' ? 'cyan' : undefined}>
                      {s.kind === 'scene' ? '场景' : s.hasVideo ? `视频主体${s.imageCount > 0 ? '（仅视频生效）' : ''}` : `${s.imageCount} 张图`}
                    </Tag>
                    {s.voiceId && <Tag style={{ margin: 0 }} color="purple">音色 {s.voiceId}</Tag>}
                    <span style={{ color: 'var(--wr-text-secondary)' }}>{timeAgo(s.createdAt)}</span>
                  </div>
                  {s.state === 'failed' && s.errMsg && (
                    <Text type="danger" style={{ fontSize: 12, display: 'block', marginTop: 6 }} ellipsis={{ tooltip: s.errMsg }}>{s.errMsg}</Text>
                  )}
                  {s.state === 'success' && s.serverId && (
                    <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', gap: 6 }}>
                      <Text type="secondary" style={{ fontSize: 12 }} copyable={{ text: s.serverId, tooltips: ['复制 server_id', '已复制'] }}>
                        {s.serverId.slice(0, 16)}{s.serverId.length > 16 ? '…' : ''}
                      </Text>
                    </div>
                  )}
                  {s.state === 'success' && (
                    <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
                      {s.kind === 'person' && (
                        <Button size="small" type="primary"
                          onClick={() => { window.location.href = `/m/compose/lipsync?subject=${encodeURIComponent(s.serverId)}` }}>
                          去生成口播
                        </Button>
                      )}
                      <Button size="small"
                        onClick={() => message.info(`server_id 已复制——创作页参考生模式主体引用粘贴：${s.serverId.slice(0, 20)}…`)}>
                        用于参考生
                      </Button>
                      <Popconfirm
                        title="删除这个数字人？"
                        description="仅移除本地记录，Vidu 侧主体不受影响（官方无删除 API）"
                        okText="删除" okButtonProps={{ danger: true }} cancelText="取消"
                        onConfirm={async () => {
                          try {
                            await businessApi.deleteGenerationTask(s.taskId)
                            message.success('已删除')
                            refetchSubjects()
                          } catch { /* 拦截器已提示 */ }
                        }}
                      >
                        <Button size="small" type="text" danger icon={<DeleteOutlined />} />
                      </Popconfirm>
                    </div>
                  )}
                  {s.state !== 'success' && (
                    <Popconfirm
                      title="删除这条记录？"
                      description={s.state === 'failed' ? '失败记录删除后不可恢复' : '任务仍在创建中，删除前会尝试取消'}
                      okText="删除" okButtonProps={{ danger: true }} cancelText="取消"
                      onConfirm={async () => {
                        try {
                          await businessApi.deleteGenerationTask(s.taskId)
                          message.success('已删除')
                          refetchSubjects()
                        } catch { /* 拦截器已提示 */ }
                      }}
                    >
                      <Button size="small" type="text" danger icon={<DeleteOutlined />} style={{ marginTop: 8 }} />
                    </Popconfirm>
                  )}
                </div>
              </div>
            ))}
          </div>
        )
      )}

      {tab === 'article' && (
        articles.length === 0 ? (
          <Empty style={{ padding: 60 }} description="还没有文章——去内容合成写第一篇">
            <Button type="primary" onClick={() => window.location.href = '/m/compose/tools?tab=article'}>去写文章</Button>
          </Empty>
        ) : (
          <div className="ip-asset-grid">
            {articles.map(w => (
              <div key={w.id} className="ip-asset-card">
                <div className="ip-asset-cover" style={{ background: 'linear-gradient(145deg, #12121a, #1f2937)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <FileTextOutlined style={{ fontSize: 40, color: 'var(--wr-accent)' }} />
                </div>
                <div className="ip-asset-body">
                  <Text strong style={{ fontSize: 13 }} ellipsis={{ tooltip: w.title }}>{w.title}</Text>
                  <div style={{ marginTop: 6, fontSize: 12 }}>
                    <Tag>{w.status === 'published' ? '已发布' : w.status === 'ready' ? '待发布' : '草稿'}</Tag>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )
      )}

      {(tab === 'video' || tab === 'image' || tab === 'audio') && (
        isLoading ? <Empty description="加载中…" style={{ padding: 60 }} /> :
        filtered.length === 0 ? (
          <Empty style={{ padding: 60 }} description={`暂无${tab === 'video' ? '视频' : tab === 'image' ? '图片' : '音频'}资产`} />
        ) : (
          <div className="ip-asset-grid">
            {filtered.map(a => (
              <div key={a.id} className="ip-asset-card">
                <div
                  className={`ip-asset-cover ${a.mime.startsWith('audio/') ? 'ip-asset-cover--voice' : ''}`}
                  style={a.mime.startsWith('image/') ? { background: `linear-gradient(180deg, rgba(0,0,0,0.05), rgba(0,0,0,0.45)), url(${a.url}) center/cover`, display: 'flex', alignItems: 'flex-end', padding: 12 } : undefined}
                >
                  <Tag style={{ margin: 0 }} color={a.owner_type === 'creation' ? 'cyan' : 'blue'}>
                    {a.owner_type === 'creation' ? 'AI 产物' : '上传素材'}
                  </Tag>
                </div>
                <div className="ip-asset-body">
                  <Text strong style={{ fontSize: 13 }} ellipsis={{ tooltip: a.url }}>
                    {a.url.split('/').pop()?.split('?')[0] || '资产'}
                  </Text>
                  <div style={{ display: 'flex', gap: 10, marginTop: 6, fontSize: 12, color: 'var(--wr-text-secondary)' }}>
                    <span>{a.mime.split('/')[1]?.toUpperCase()}</span>
                    <span>{formatSize(a.size_bytes)}</span>
                    <span>{timeAgo(a.created_at)}</span>
                  </div>
                  <Space style={{ marginTop: 10 }}>
                    <Button size="small" onClick={() => window.open(a.url, '_blank', 'noopener')}>
                      {a.mime.startsWith('image/') ? '查看' : '播放'}
                    </Button>
                    <Button size="small" type="text" danger onClick={async () => {
                      try { await businessApi.deleteAsset(a.id); message.success('已删除'); queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY }) } catch { }
                    }}>删除</Button>
                  </Space>
                </div>
              </div>
            ))}
          </div>
        )
      )}

      <CreateSubjectModal open={createOpen} voices={myVoices} onClose={() => setCreateOpen(false)}
        onCreated={() => { refetchSubjects(); message.success('数字人创建成功') }} />

      {/* 生成素材模态框 */}
      <GenerateAssetModal
        open={generateOpen}
        type={generateType}
        myVoices={myVoices}
        onClose={() => setGenerateOpen(false)}
        onGenerated={() => {
          queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY })
          message.success('素材已生成并入库')
        }}
      />
    </div>
  )
}

/** 客户端预检主体视频时长（≤5s；元数据读不出的容器放行，由上游兜底校验） */
function checkVideoDuration(file: File): Promise<void> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const v = document.createElement('video')
    v.preload = 'metadata'
    v.onloadedmetadata = () => {
      URL.revokeObjectURL(url)
      if (v.duration > 5.5) {
        message.error('主体视频不能超过 5 秒')
        reject(new Error('视频超过 5 秒'))
      } else {
        resolve()
      }
    }
    v.onerror = () => { URL.revokeObjectURL(url); resolve() }
    v.src = url
  })
}

function CreateSubjectModal({ open, voices, onClose, onCreated: _onCreated }: {
  open: boolean
  voices: string[]
  onClose: () => void
  onCreated: () => void
}) {
  const [name, setName] = useState('')
  const [kind, setKind] = useState<'person' | 'scene'>('person')
  const [imageUrls, setImageUrls] = useState<string[]>([])
  const [videoUrl, setVideoUrl] = useState('')
  const [voiceId, setVoiceId] = useState('')
  const [creating, setCreating] = useState(false)

  const handleCreate = async () => {
    if (!name.trim()) { message.warning(kind === 'scene' ? '请输入场景名称' : '请输入数字人名称'); return }
    if (imageUrls.length === 0 && !videoUrl) {
      message.warning(kind === 'scene' ? '请上传 1-3 张场景照片' : '请至少上传 1 张形象照或 1 个主体视频'); return
    }
    setCreating(true)
    try {
      // 主体创建未纳入统一 submit（Docs 09：POST /generation/tasks 已删除）
      message.warning('主体创建暂未纳入统一生成接口。请先用「生成图片/视频」准备素材，或使用「快速生成」模板。')
    } finally { setCreating(false) }
  }

  return (
    <Modal open={open} title="创建主体" okText="创建" cancelText="取消" onOk={handleCreate} onCancel={onClose} confirmLoading={creating} width={MODAL_W.md}>
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <Segmented
          value={kind} onChange={v => setKind(v as 'person' | 'scene')}
          options={[
            { value: 'person', label: '人物分身', icon: <UserOutlined /> },
            { value: 'scene', label: '场景主体', icon: <VideoCameraOutlined /> },
          ]}
        />
        <Text type="secondary" style={{ fontSize: 12 }}>
          {kind === 'person'
            ? '上传 1-3 张形象照或 1 个 5 秒内的主体视频——创建后可生成口播/参考生视频'
            : '上传 2-3 张场景照片（厨房/门店/工作室）——生成视频时场景可复用，画面一致'}
        </Text>
        <Input
          placeholder={kind === 'scene' ? '场景名称（如：主厨房、门店前台）' : '数字人名称（如：张师傅、李老板）'}
          value={name} onChange={e => setName(e.target.value)} maxLength={64}
        />
        <div>
          <Text strong style={{ fontSize: 13 }}>{kind === 'scene' ? '场景照片（1-3 张）' : '形象照（1-3 张）'}</Text>
          <Upload
            listType="picture-card"
            maxCount={3}
            accept="image/png,image/jpeg,image/jpg,image/webp"
            customRequest={async ({ file, onSuccess, onError }) => {
              try {
                const r = await businessApi.uploadAsset(file as File)
                setImageUrls(prev => [...prev, r.url])
                onSuccess?.(r)
              } catch (e) { onError?.(e as Error) }
            }}
            onRemove={(file) => {
              const url = (file.response as any)?.url
              if (url) setImageUrls(prev => prev.filter(u => u !== url))
            }}
          >
            {imageUrls.length < 3 && <div><PlusOutlined /><div style={{ fontSize: 12, marginTop: 4 }}>上传形象照</div></div>}
          </Upload>
        </div>
        {kind === 'person' && (
          <div>
            <Text strong style={{ fontSize: 13 }}>主体视频（可选，1 个 ≤5 秒）</Text>
          <Upload
            maxCount={1}
            accept="video/mp4,video/x-msvideo,video/quicktime"
            beforeUpload={checkVideoDuration}
            customRequest={async ({ file, onSuccess, onError }) => {
              try {
                const r = await businessApi.uploadAsset(file as File)
                setVideoUrl(r.url)
                onSuccess?.(r)
              } catch (e) { onError?.(e as Error) }
            }}
            onRemove={() => setVideoUrl('')}
          >
            <Button icon={<VideoCameraOutlined />}>{videoUrl ? '重新上传' : '上传视频（mp4/avi/mov）'}</Button>
          </Upload>
        </div>
        )}
        {kind === 'person' && (
        <div>
          <Text strong style={{ fontSize: 13 }}>绑定音色（可选）</Text>
          <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
            音视频直出时使用；q2-pro 及 q3 系列模型不支持音色
          </Text>
          <div style={{ marginTop: 4 }}>
            <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={voices} />
          </div>
          <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
            官方音色可试听后选择；想要自己的声音？
            <a href="/m/compose/tools?tab=media" target="_blank" rel="noreferrer">去声音克隆</a>
            （复刻音色 7 天内在语音合成中调用一次即永久保留）
          </Text>
        </div>
        )}
      </Space>
    </Modal>
  )
}

/** 生成素材弹窗（居中）：参数由 listGenerationTypes 能力向量驱动，对齐 Vidu 端点校验 */
function GenerateAssetModal({ open, type, myVoices = [], onClose, onGenerated }: {
  open: boolean
  type: 'image' | 'video' | 'audio' | 'voice'
  myVoices?: string[]
  onClose: () => void
  onGenerated: () => void
}) {
  const { brandId } = useBrandContext()
  const [text, setText] = useState('')
  const [model, setModel] = useState('')
  const [duration, setDuration] = useState<number>()
  const [resolution, setResolution] = useState('')
  const [aspectRatio, setAspectRatio] = useState('')
  const [refImages, setRefImages] = useState<{ id: string; url: string }[]>([])
  const [pickerOpen, setPickerOpen] = useState(false)
  const [voiceId, setVoiceId] = useState('')
  const [audioFile, setAudioFile] = useState<File | null>(null)
  const [voiceName, setVoiceName] = useState('')
  const [generating, setGenerating] = useState(false)
  const [taskId, setTaskId] = useState<string | null>(null)
  const [taskState, setTaskState] = useState<string | null>(null)

  const subType = type === 'image' ? 'text2image'
    : type === 'video' ? 'text2video'
    : type === 'audio' ? 'tts'
    : 'voice_clone'

  const { data: types = [], isLoading: typesLoading } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then(r => r.types),
    enabled: open,
  })

  const models = useMemo(() => {
    const t = types.find(x => x.sub_type === subType)
    return t?.models || []
  }, [types, subType])

  const cap: ModelCapability | undefined = useMemo(() => {
    const entry = models.find(m => m.model === model) || models[0]
    return entry?.capability
  }, [models, model])

  // 打开时重置；模型列表到位后选默认并回填能力默认值
  useEffect(() => {
    if (!open) return
    setText('')
    setVoiceId('')
    setAudioFile(null)
    setVoiceName('')
    setRefImages([])
    setTaskId(null)
    setTaskState(null)
    setGenerating(false)
  }, [open, type])

  useEffect(() => {
    if (!open || models.length === 0) return
    const preferred = type === 'image'
      ? (models.find(m => m.model === 'viduq2') || models[0])
      : models[0]
    setModel(preferred.model)
    const c = preferred.capability
    const [dMin, dMax] = c.durations || [0, 0]
    if (dMax > 0) {
      const def = dMin === dMax ? dMin : Math.min(Math.max(4, dMin), dMax)
      setDuration(def)
    } else {
      setDuration(undefined)
    }
    setResolution(c.resolutions?.[0] || '')
    setAspectRatio(c.aspect_ratios?.[0] || (type === 'video' ? '16:9' : ''))
  }, [open, type, models])

  const title = type === 'image' ? '生成图片'
    : type === 'video' ? '生成视频'
    : type === 'audio' ? '生成配音'
    : '克隆音色'

  const lead = type === 'image' ? '一句话描述画面——可选参考图，结果自动入库供封面/图文引用'
    : type === 'video' ? '文生短视频——时长、清晰度与比例按当前模型能力可选'
    : type === 'audio' ? '选音色 + 文案，合成可复用的配音素材'
    : '上传一段人声，克隆后可在配音/数字人中反复使用'

  const maxPrompt = cap?.max_prompt_len
    || (type === 'image' ? 2000 : type === 'voice' ? 1000 : type === 'audio' ? 10000 : 5000)

  const imageNeedsRef = model === 'viduq1'

  const resetForAnother = () => {
    setTaskId(null)
    setTaskState(null)
  }

  const handleGenerate = async () => {
    if (type === 'voice') {
      if (!audioFile) { message.warning('请上传参考音频'); return }
      if (!text.trim()) { message.warning('请输入试听文本'); return }
      const vid = (voiceName.trim() || `voice_${Date.now().toString(36)}`).replace(/[^a-zA-Z0-9_-]/g, '_')
      if (!/^[a-zA-Z]/.test(vid) || vid.length < 8) {
        message.warning('音色 ID 需以英文字母开头，且至少 8 位（字母/数字/-/_）')
        return
      }
    } else if (!text.trim()) {
      message.warning(type === 'audio' ? '请输入要合成的文案' : '请输入描述')
      return
    }
    if ((type === 'image' || type === 'video') && models.length === 0) {
      message.warning('当前未开通对应生成能力，请联系管理员')
      return
    }
    if (type === 'audio' && !voiceId) {
      // 统一 submit 暂不透传音色；不阻断，用系统默认
    }
    if (type === 'image' && imageNeedsRef && refImages.length === 0) {
      message.warning('当前模型需要至少 1 张参考图')
      return
    }
    if (!brandId) {
      message.warning('请先选择人设/品牌')
      return
    }

    setGenerating(true)
    try {
      let result: GenerationTask
      if (type === 'image') {
        result = await submitUnified({
          brand_id: brandId,
          text: text.trim(),
          type: 'image',
          materials: refImages.length ? refImages.map((r) => r.id) : undefined,
        })
      } else if (type === 'video') {
        result = await submitUnified({
          brand_id: brandId,
          text: text.trim(),
          type: 'video',
          duration: duration || undefined,
          quality: resolution || undefined,
          aspect_ratio: aspectRatio || undefined,
        })
      } else if (type === 'audio') {
        result = await submitUnified({
          brand_id: brandId,
          text: text.trim(),
          type: 'audio',
        })
      } else {
        const uploaded = await businessApi.uploadAsset(audioFile!)
        result = await submitUnified({
          brand_id: brandId,
          text: text.trim(),
          type: 'voice',
          materials: [uploaded.id],
        })
      }

      setTaskId(result.id)
      setTaskState(result.state)
      message.success('任务已提交')
      if (result.state === 'success') {
        onGenerated()
      } else {
        pollTask(result.id)
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : ''
      if (msg && !msg.includes('配额')) message.error(msg)
    } finally {
      setGenerating(false)
    }
  }

  const pollTask = async (id: string) => {
    const poll = async () => {
      try {
        const task = await businessApi.getGenerationTask(id)
        setTaskState(task.state)
        if (task.state === 'success') {
          message.success('生成完成')
          onGenerated()
          return
        }
        if (task.state === 'failed' || task.state === 'cancelled') {
          message.error(task.err_msg || '生成失败')
          return
        }
        setTimeout(poll, 2800)
      } catch {
        setTimeout(poll, 4000)
      }
    }
    setTimeout(poll, 2800)
  }

  const busy = generating || (!!taskId && taskState !== 'success' && taskState !== 'failed' && taskState !== 'cancelled')

  return (
    <Modal
      open={open}
      onCancel={onClose}
      width={MODAL_W.lg}
      destroyOnClose
      title={title}
      styles={{ body: { ...modalBodyScroll.body, paddingTop: 8 } }}
      footer={
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 }}>
          <Button onClick={onClose}>{taskState === 'success' ? '关闭' : '取消'}</Button>
          <Space>
            {taskState === 'success' && (
              <Button onClick={resetForAnother}>再生成一份</Button>
            )}
            <Button
              type="primary"
              className="ip-btn-primary"
              loading={generating}
              disabled={busy && taskState !== 'failed'}
              onClick={handleGenerate}
            >
              {generating ? '提交中…' : taskId && busy ? '生成中…' : taskState === 'failed' ? '重试' : '开始生成'}
            </Button>
          </Space>
        </div>
      }
    >
      <p style={{ color: 'var(--wr-text-secondary)', fontSize: 13, lineHeight: 1.6, marginBottom: 20 }}>{lead}</p>

      {typesLoading ? (
        <Text type="secondary">加载模型能力…</Text>
      ) : models.length === 0 && (type === 'image' || type === 'video') ? (
        <Alert type="warning" showIcon message="未开通此生成能力" description="管理后台需启用对应端点模型后才能生成。" />
      ) : (
        <Space direction="vertical" size={18} style={{ width: '100%' }}>
          {(type === 'image' || type === 'video' || type === 'audio') && models.length > 1 && (
            <div>
              <Text strong style={{ fontSize: 13 }}>模型</Text>
              <Select
                style={{ width: '100%', marginTop: 8 }}
                value={model || models[0]?.model}
                onChange={setModel}
                disabled={busy}
                options={models.map(m => ({
                  value: m.model,
                  label: `${m.model}${m.capability?.family ? ` · ${m.capability.family}` : ''}`,
                }))}
              />
            </div>
          )}

          <div>
            <Text strong style={{ fontSize: 13 }}>
              {type === 'audio' ? '合成文案' : type === 'voice' ? '试听文案' : '画面描述'}
            </Text>
            <Input.TextArea
              style={{ marginTop: 8 }}
              placeholder={
                type === 'image' ? '例如：午市套餐平铺在木质餐桌上，自然光，竖版种草风，无水印'
                  : type === 'video' ? '例如：咖啡从拉花杯缓缓倒出，镜头缓慢推进，暖色调'
                  : type === 'audio' ? '例如：欢迎光临，今日午市套餐限时优惠…'
                  : '用这段话试听克隆音色，如：你好，欢迎来到我们的门店'
              }
              autoSize={{ minRows: 4, maxRows: 8 }}
              showCount
              maxLength={maxPrompt}
              value={text}
              onChange={e => setText(e.target.value)}
              disabled={busy}
            />
          </div>

          {type === 'video' && cap && (
            <div style={{
              padding: 14,
              borderRadius: 12,
              background: 'var(--wr-bg-elevated)',
              border: '1px solid var(--wr-border)',
            }}>
              <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>成片参数</Text>
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                {(() => {
                  const [dMin, dMax] = cap.durations || [0, 0]
                  if (dMax <= 0) return null
                  if (dMin === dMax) {
                    return <Text type="secondary" style={{ fontSize: 12 }}>时长固定 {dMin} 秒</Text>
                  }
                  return (
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>时长（{dMin}–{dMax} 秒）</Text>
                      <div style={{ marginTop: 6 }}>
                        <InputNumber
                          min={dMin}
                          max={dMax}
                          value={duration}
                          onChange={v => setDuration(typeof v === 'number' ? v : dMin)}
                          disabled={busy}
                          addonAfter="秒"
                          style={{ width: 140 }}
                        />
                      </div>
                    </div>
                  )
                })()}
                {(cap.resolutions?.length || 0) > 0 && (
                  <div>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>清晰度</Text>
                    <Segmented
                      value={resolution || cap.resolutions![0]}
                      onChange={v => setResolution(String(v))}
                      options={cap.resolutions!.map(r => ({ value: r, label: r }))}
                      disabled={busy}
                    />
                  </div>
                )}
                {(cap.aspect_ratios?.length || 0) > 0 && (
                  <div>
                    <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>画面比例</Text>
                    <Segmented
                      value={aspectRatio || cap.aspect_ratios![0]}
                      onChange={v => setAspectRatio(String(v))}
                      options={cap.aspect_ratios!.map(r => ({ value: r, label: r }))}
                      disabled={busy}
                    />
                  </div>
                )}
              </Space>
            </div>
          )}

          {type === 'image' && (
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <Text strong style={{ fontSize: 13 }}>
                  参考图{imageNeedsRef ? '（必填）' : '（可选）'}
                </Text>
                <Button size="small" type="dashed" icon={<PlusOutlined />} disabled={busy || refImages.length >= 7} onClick={() => setPickerOpen(true)}>
                  从素材库选
                </Button>
              </div>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
                {imageNeedsRef ? '当前模型需 1–7 张参考图' : '不传则为纯文生图；最多 7 张'}
              </Text>
              {refImages.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未选参考图" style={{ margin: '8px 0' }} />
              ) : (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {refImages.map((img, i) => (
                    <div key={img.id + i} style={{ position: 'relative', width: 72, height: 72 }}>
                      <img src={img.url} alt="" style={{ width: 72, height: 72, objectFit: 'cover', borderRadius: 8, border: '1px solid var(--wr-border)' }} />
                      <Button
                        size="small"
                        type="text"
                        danger
                        icon={<DeleteOutlined />}
                        disabled={busy}
                        style={{ position: 'absolute', top: -6, right: -6, background: 'var(--wr-bg)', borderRadius: '50%' }}
                        onClick={() => setRefImages(prev => prev.filter((_, j) => j !== i))}
                      />
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {type === 'audio' && (
            <div>
              <Text strong style={{ fontSize: 13 }}>音色</Text>
              <Alert
                type="info"
                showIcon
                style={{ marginTop: 8, marginBottom: 8 }}
                message="统一生成接口按文档使用系统默认音色（type=audio），暂不支持指定 voice_id"
              />
              <div style={{ marginTop: 8 }}>
                <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={myVoices} />
              </div>
            </div>
          )}

          {type === 'voice' && (
            <>
              <div>
                <Text strong style={{ fontSize: 13 }}>音色 ID</Text>
                <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
                  英文字母开头，8–256 位；留空则自动生成
                </Text>
                <Input
                  style={{ marginTop: 8 }}
                  placeholder="如：shop_host_01"
                  value={voiceName}
                  onChange={e => setVoiceName(e.target.value)}
                  disabled={busy}
                />
              </div>
              <div>
                <Text strong style={{ fontSize: 13 }}>参考人声</Text>
                <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4, marginBottom: 8 }}>
                  mp3 / wav / m4a，约 10 秒–5 分钟
                </Text>
                <Upload
                  maxCount={1}
                  accept="audio/mp3,audio/wav,audio/m4a,audio/mpeg"
                  beforeUpload={(file) => { setAudioFile(file); return false }}
                  onRemove={() => setAudioFile(null)}
                  disabled={busy}
                  fileList={audioFile ? [{ uid: '-1', name: audioFile.name, status: 'done' }] : []}
                >
                  <Button icon={<SoundOutlined />}>{audioFile ? '重新上传' : '上传参考音频'}</Button>
                </Upload>
              </div>
            </>
          )}

          {taskId && (
            <div style={{
              padding: '12px 14px',
              borderRadius: 10,
              background: 'var(--wr-bg-elevated)',
              border: '1px solid var(--wr-border)',
            }}>
              <Space wrap>
                <Tag color={
                  taskState === 'success' ? 'success'
                    : taskState === 'failed' || taskState === 'cancelled' ? 'error'
                    : 'processing'
                }>
                  {taskState === 'success' ? '已完成'
                    : taskState === 'failed' ? '失败'
                    : taskState === 'cancelled' ? '已取消'
                    : '生成中'}
                </Tag>
                <Text type="secondary" style={{ fontSize: 12 }}>完成后会自动写入素材库</Text>
              </Space>
            </div>
          )}
        </Space>
      )}

      <AssetPicker
        open={pickerOpen}
        mode="multi"
        accept="image"
        title="选择参考图"
        max={7}
        onClose={() => setPickerOpen(false)}
        onSelect={(assets) => {
          setRefImages(prev => [...prev, ...assets.map(a => ({ id: a.id, url: a.url }))].slice(0, 7))
          setPickerOpen(false)
        }}
      />
    </Modal>
  )
}

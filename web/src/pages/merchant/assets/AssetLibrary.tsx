import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { Button, Empty, Input, Modal, Popconfirm, Segmented, Space, Tag, Typography, Upload } from 'antd'
import { message } from '../../../utils/antdApp'
import {
  SoundOutlined, PictureOutlined, VideoCameraOutlined, UserOutlined,
  PlusOutlined, DeleteOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../api/business'
import { MODAL_W } from '../../../ui/modalFit'
import { buildSubjectRegisterPayload } from '../../../api/generationSubmit'
import { useMediaAssets, MEDIA_ASSETS_QUERY_KEY } from '../../../hooks/useMediaAssets'
import { GENERATION_TASKS_KEY, useGenerationTasks } from '../../../hooks/useGenerationTasks'
import { useBrandContext } from '../../../hooks/useBrands'
import { SubjectGridCard } from '../../../components/compose/SubjectCard'
import type { MediaAsset } from '../../../types/api'
import {
  listSubjectsFromTasks,
  parseGenerationTaskParams,
  type ViduSubject,
} from '../../../utils/subjectTask'
import VoicePicker from '../../../components/VoicePicker'
import { GenerateAssetModal } from '../../../components/assets/GenerateAssetModal'
import { isAudioMedia, isImageMedia, isVideoMedia, inferMediaKind, mediaAssetsFromGenerationTasks, mergeMediaByUrl } from '../../../utils/generationTask'
import { MediaPreviewModal } from '../../../components/MediaPreviewModal'
import { ImageCover } from '../../../components/ImageCover'
import { VideoFrameCover } from '../../../components/VideoFrameCover'

const { Text } = Typography

type TabKey = 'audio' | 'image' | 'video' | 'digital_human'

const TABS = [
  { value: 'digital_human', label: '数字人', icon: <UserOutlined /> },
  { value: 'video', label: '视频', icon: <VideoCameraOutlined /> },
  { value: 'image', label: '图片', icon: <PictureOutlined /> },
  { value: 'audio', label: '音频', icon: <SoundOutlined /> },
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

/**
 * 资产库：统一媒体库（数字人 / 视频 / 图片 / 音频 4 tab）。
 * 数字人 tab = Vidu 主体管理（创建→列表→用于 reference2video 视频生成 @引用）。
 * 其他 tab = 上传素材 + AI 转存产物。
 */
export default function AssetLibrary() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [tab, setTab] = useState<TabKey>('digital_human')
  const [q, setQ] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [generateOpen, setGenerateOpen] = useState(false)
  const [generateType, setGenerateType] = useState<'image' | 'video' | 'audio' | 'voice'>('image')
  const [previewAsset, setPreviewAsset] = useState<MediaAsset | null>(null)
  const queryClient = useQueryClient()

  // 图片/视频/音频：含 AI 转存产物（creation）
  const mediaOwner = tab === 'image' || tab === 'video' || tab === 'audio' ? 'all' : 'material'
  const { data: assets = [], isLoading } = useMediaAssets(true, mediaOwner)

  const { tasks: genTasks = [], refetch: refetchSubjects } = useGenerationTasks()
  const subjects: ViduSubject[] = useMemo(
    () => listSubjectsFromTasks(genTasks || []),
    [genTasks],
  )

  // 我的音色库：声音克隆成功的 voice_id（创建数字人可直接绑定）
  const myVoices = useMemo(() => {
    const ids = new Set<string>()
    for (const t of genTasks || []) {
      if (t.sub_type !== 'voice_clone' || t.state !== 'success') continue
      const vid = parseGenerationTaskParams(t).voice_id
      if (typeof vid === 'string' && vid) ids.add(vid)
    }
    return Array.from(ids)
  }, [genTasks])

  // 从向导/Creation「创建分身」深链打开创建弹窗
  useEffect(() => {
    if (searchParams.get('create') === 'subject') {
      setTab('digital_human')
      setCreateOpen(true)
      const next = new URLSearchParams(searchParams)
      next.delete('create')
      setSearchParams(next, { replace: true })
    }
  }, [searchParams, setSearchParams])

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    if (tab === 'audio') {
      const fromDisk = assets.filter(a => isAudioMedia(a.mime, a.url, a.type))
      const fromTasks = mediaAssetsFromGenerationTasks(genTasks, 'audio')
      let list = mergeMediaByUrl(fromDisk, fromTasks)
      if (needle) {
        list = list.filter(a =>
          a.url.toLowerCase().includes(needle)
          || (a.name || '').toLowerCase().includes(needle),
        )
      }
      return list
    }
    if (tab === 'image') {
      const fromDisk = assets.filter(a => isImageMedia(a.mime, a.url, a.type))
      const fromTasks = mediaAssetsFromGenerationTasks(genTasks, 'image')
      let list = mergeMediaByUrl(fromDisk, fromTasks)
      if (needle) list = list.filter(a => a.url.toLowerCase().includes(needle) || (a.name || '').toLowerCase().includes(needle))
      return list
    }
    if (tab === 'video') {
      const fromDisk = assets.filter(a => isVideoMedia(a.mime, a.url, a.type))
      const fromTasks = mediaAssetsFromGenerationTasks(genTasks, 'video')
      let list = mergeMediaByUrl(fromDisk, fromTasks)
      if (needle) {
        list = list.filter(a =>
          a.url.toLowerCase().includes(needle)
          || (a.name || '').toLowerCase().includes(needle),
        )
      }
      return list
    }
    return []
  }, [tab, assets, genTasks, q])

  const hint = {
    digital_human: '通过 Vidu 主体 API 创建数字分身——上传形象照/主体视频并绑定音色，视频生成中用 server_id 引用',
    video: 'AI 生成的视频产物（Vidu 视频/数字人视频）',
    image: 'AI 生成的图片 / 上传的素材图（封面/图文创作引用源）',
    audio: '配音/音效素材（含 AI 生成转存与上传文件）',
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
            {subjects.map((s) => (
              <SubjectGridCard
                key={s.taskId}
                subject={s}
                timeLabel={timeAgo(s.createdAt)}
                footer={(
                  <>
                    {s.state === 'success' && (
                      <div className="wr-subject-grid-actions">
                        {s.kind === 'person' && s.serverId && (
                          <Button
                            size="small"
                            type="primary"
                            onClick={() => {
                              window.location.href = `/m/compose/lipsync?subject=${encodeURIComponent(s.serverId)}`
                            }}
                          >
                            拍口播
                          </Button>
                        )}
                        <Button
                          size="small"
                          onClick={() => message.info(`server_id：${s.serverId.slice(0, 20)}…`)}
                        >
                          用于参考生
                        </Button>
                        <Popconfirm
                          title="删除这个数字人？"
                          description="仅移除本地记录，Vidu 侧主体不受影响（官方无删除 API）"
                          okText="删除"
                          okButtonProps={{ danger: true }}
                          cancelText="取消"
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
                        okText="删除"
                        okButtonProps={{ danger: true }}
                        cancelText="取消"
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
                    )}
                  </>
                )}
              />
            ))}
          </div>
        )
      )}

      {(tab === 'video' || tab === 'image' || tab === 'audio') && (
        isLoading ? <Empty description="加载中…" style={{ padding: 60 }} /> :
        filtered.length === 0 ? (
          <Empty style={{ padding: 60 }} description={
            tab === 'audio'
              ? '暂无音频资产——可点「生成音频」或在内容合成里生成配音'
              : tab === 'video'
              ? '暂无视频资产——可点「生成视频」或在口播向导里生成成片片段'
              : `暂无${tab === 'image' ? '图片' : ''}资产`
          } />
        ) : (
          <div className="ip-asset-grid">
            {filtered.map(a => {
              const kind = inferMediaKind(a.mime, a.url, a.type)
              return (
              <div key={a.id} className="ip-asset-card">
                <div
                  className={`ip-asset-cover ${kind === 'audio' ? 'ip-asset-cover--voice' : ''}`}
                  style={{
                    minHeight: kind === 'audio' ? 120 : undefined,
                    display: 'flex',
                    alignItems: 'stretch',
                    justifyContent: 'center',
                    position: 'relative',
                    padding: 0,
                    overflow: 'hidden',
                  }}
                >
                  {kind === 'image' && <ImageCover url={a.url} />}
                  {kind === 'video' && (
                    <VideoFrameCover url={a.url} poster={a.cover_url} />
                  )}
                  {kind === 'audio' && (
                    <SoundOutlined style={{ fontSize: 32, color: 'rgba(255,255,255,0.85)', margin: 'auto' }} />
                  )}
                  <Tag style={{ margin: 0, position: 'absolute', left: 12, bottom: 12, zIndex: 1 }} color={a.owner_type === 'creation' ? 'cyan' : 'blue'}>
                    {a.owner_type === 'creation' ? 'AI 产物' : '上传素材'}
                  </Tag>
                  {(kind === 'image' || kind === 'video') && (
                    <div
                      aria-hidden
                      style={{
                        position: 'absolute',
                        inset: 0,
                        background: 'linear-gradient(180deg, transparent 40%, rgba(0,0,0,0.55))',
                        pointerEvents: 'none',
                        zIndex: 0,
                      }}
                    />
                  )}
                </div>
                <div className="ip-asset-body">
                  <Text strong style={{ fontSize: 13 }} ellipsis={{ tooltip: a.name || a.url }}>
                    {a.name || a.url.split('/').pop()?.split('?')[0] || '资产'}
                  </Text>
                  <div style={{ display: 'flex', gap: 10, marginTop: 6, fontSize: 12, color: 'var(--wr-text-secondary)' }}>
                    <span>{(a.mime.split('/')[1] || kind).toUpperCase()}</span>
                    <span>{formatSize(a.size_bytes)}</span>
                    <span>{timeAgo(a.created_at)}</span>
                  </div>
                  <Space style={{ marginTop: 10 }}>
                    <Button size="small" onClick={() => setPreviewAsset(a)}>
                      {kind === 'image' ? '查看' : '播放'}
                    </Button>
                    <Button size="small" type="text" danger onClick={async () => {
                      if (a.id.startsWith('gen-task:')) {
                        message.info('该条目来自生成任务记录，请在工作台任务列表管理')
                        return
                      }
                      try { await businessApi.deleteAsset(a.id); message.success('已删除'); queryClient.invalidateQueries({ queryKey: MEDIA_ASSETS_QUERY_KEY }) } catch { }
                    }}>删除</Button>
                  </Space>
                </div>
              </div>
            )})}
          </div>
        )
      )}

      <MediaPreviewModal
        open={!!previewAsset}
        asset={previewAsset}
        onClose={() => setPreviewAsset(null)}
      />

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
          queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
          refetchSubjects()
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
  const [imageAssets, setImageAssets] = useState<Array<{ id: string; url: string }>>([])
  const [videoAsset, setVideoAsset] = useState<{ id: string; url: string } | null>(null)
  const [voiceId, setVoiceId] = useState('')
  const [creating, setCreating] = useState(false)
  const { brandId } = useBrandContext()
  const queryClient = useQueryClient()

  const resetForm = () => {
    setName('')
    setKind('person')
    setImageAssets([])
    setVideoAsset(null)
    setVoiceId('')
  }

  const handleCreate = async () => {
    if (!name.trim()) { message.warning(kind === 'scene' ? '请输入场景名称' : '请输入数字人名称'); return }
    if (!brandId) { message.warning('请先在顶栏或人设页选择品牌/人设'); return }
    if (imageAssets.length === 0 && !videoAsset) {
      message.warning(kind === 'scene' ? '请上传 1-3 张场景照片' : '请至少上传 1 张形象照或 1 个主体视频'); return
    }
    setCreating(true)
    try {
      const task = await businessApi.submitGeneration(buildSubjectRegisterPayload({
        brand_id: brandId,
        name: name.trim(),
        imageMaterialIds: imageAssets.map((a) => a.id),
        imageUrls: imageAssets.map((a) => a.url),
        videoUrl: videoAsset?.url,
        voice_id: voiceId || undefined,
      }))
      message.success(`数字分身「${name.trim()}」已创建（任务 ${task.id}）——生成视频时可直接复用该形象`)
      queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
      resetForm()
      onClose()
    } catch { /* 拦截器已提示 */ } finally { setCreating(false) }
  }

  return (
    <Modal
      open={open}
      title="创建主体"
      okText="创建"
      cancelText="取消"
      onOk={handleCreate}
      onCancel={() => { resetForm(); onClose() }}
      confirmLoading={creating}
      width={MODAL_W.md}
    >
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
                setImageAssets((prev) => [...prev, { id: r.id, url: r.url }])
                onSuccess?.(r)
              } catch (e) { onError?.(e as Error) }
            }}
            onRemove={(file) => {
              const id = (file.response as { id?: string } | undefined)?.id
              const url = (file.response as { url?: string } | undefined)?.url
              if (id || url) {
                setImageAssets((prev) => prev.filter((a) => a.id !== id && a.url !== url))
              }
            }}
          >
            {imageAssets.length < 3 && <div><PlusOutlined /><div style={{ fontSize: 12, marginTop: 4 }}>上传形象照</div></div>}
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
                setVideoAsset({ id: r.id, url: r.url })
                onSuccess?.(r)
              } catch (e) { onError?.(e as Error) }
            }}
            onRemove={() => setVideoAsset(null)}
          >
            <Button icon={<VideoCameraOutlined />}>{videoAsset ? '重新上传' : '上传视频（mp4/avi/mov）'}</Button>
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


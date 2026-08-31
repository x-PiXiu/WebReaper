import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import {
  Button,
  Checkbox,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Spin,
  Tag,
} from 'antd'
import {
  DeleteOutlined,
  EnvironmentOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  SearchOutlined,
  UserOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../../api/business'
import { CreateSubjectModal } from '../../../../components/compose/CreateSubjectModal'
import { useSubjectList } from '../../../../hooks/useSubjectList'
import { useOfficialSubjects } from '../../../../hooks/useSubjectAssets'
import { parseGenerationTaskParams, listSceneSubjects } from '../../../../utils/subjectTask'
import type { ViduSubject } from '../../../../utils/subjectTask'
import { message } from '../../../../utils/antdApp'
import { CREATIVE_CDN } from '../../../../config/creativeCdn'
import type { GenerationTask } from '../../../../types/api'


type LibCard = {
  id: string
  name: string
  portraitUrl: string
  timeLabel: string
  tag: string
  duration?: string
  selectable: boolean
  ready: boolean
  serverId?: string
  subject?: ViduSubject
}

/** 官方主体展示区已改用真实API（27号优化）：useOfficialSubjects hook 从 subject_assets(scope=official) 读取 */

function formatUploadTime(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `上传于 ${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 分身三态徽标（25 号阶段二：创建中 → 形象视频生成中 → 可用；D4 失败可重试） */
function subjectTag(s: ViduSubject, avatarTask?: GenerationTask) {
  if (s.state !== 'success') {
    return { label: s.state === 'failed' ? '失败' : '创建中', tone: s.state === 'failed' ? 'danger' : 'processing' }
  }
  if (s.avatarTaskId && avatarTask) {
    if (avatarTask.state === 'success') return { label: '可用', tone: 'success' }
    if (avatarTask.state === 'failed' || avatarTask.state === 'cancelled') return { label: '形象视频失败', tone: 'warning' }
    return { label: '形象视频生成中', tone: 'processing' }
  }
  return { label: '可用 · 无形象视频', tone: 'success' }
}

function subjectToCard(s: ViduSubject, avatarTask?: GenerationTask): LibCard {
  return {
    id: s.taskId,
    name: s.name,
    portraitUrl: s.portraitUrl,
    timeLabel: formatUploadTime(s.createdAt),
    tag: subjectTag(s, avatarTask).label,
    duration: s.hasVideo ? '≤5s' : undefined,
    selectable: true,
    ready: s.state === 'success' && !!s.serverId,
    serverId: s.serverId || undefined,
    subject: s,
  }
}

function sceneToCard(s: ViduSubject): LibCard {
  return {
    id: s.taskId,
    name: s.name,
    portraitUrl: s.portraitUrl,
    timeLabel: formatUploadTime(s.createdAt),
    tag: '环境',
    selectable: true,
    ready: s.state === 'success' && !!s.serverId,
    serverId: s.serverId || undefined,
    subject: s,
  }
}

type PreviewState = { subject: ViduSubject; url?: string } | null

/** 数字资产管理页（23 号 §2.3 + 25 号 §6.5）：官方区 / 我的环境 / 我的分身 */
export default function AvatarModule() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { subjects, tasks, refetch, isLoading } = useSubjectList({ refetchInterval: 8_000 })
  const { subjects: officialSubjects, isLoading: officialLoading } = useOfficialSubjects({ limit: 50 })
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [createKind, setCreateKind] = useState<'person' | 'scene'>('person')
  const [deleting, setDeleting] = useState(false)
  const [preview, setPreview] = useState<PreviewState>(null)
  const [retrying, setRetrying] = useState('')

  useEffect(() => {
    if (searchParams.get('create') === '1' || searchParams.get('create') === 'subject') {
      setCreateOpen(true)
      const next = new URLSearchParams(searchParams)
      next.delete('create')
      setSearchParams(next, { replace: true })
    }
  }, [searchParams, setSearchParams])

  const myVoices = useMemo(() => {
    const ids = new Set<string>()
    for (const t of tasks || []) {
      if (t.sub_type !== 'voice_clone' || t.state !== 'success') continue
      const vid = parseGenerationTaskParams(t).voice_id
      if (typeof vid === 'string' && vid) ids.add(vid)
    }
    return Array.from(ids)
  }, [tasks])

  // 链式形象视频任务 join（params.avatar_video=true 的 reference2video 任务）
  const avatarTaskById = useMemo(() => {
    const m = new Map<string, GenerationTask>()
    for (const t of tasks || []) {
      if (t.sub_type !== 'reference2video') continue
      if (parseGenerationTaskParams(t).avatar_video !== true) continue
      m.set(t.id, t)
    }
    return m
  }, [tasks])

  const sceneSubjects = useMemo(() => listSceneSubjects(subjects), [subjects])
  const personSubjects = useMemo(
    () => subjects.filter((s) => s.kind === 'person'),
    [subjects],
  )

  const needle = q.trim().toLowerCase()
  const personCards = useMemo(
    () => personSubjects
      .map((s) => subjectToCard(s, avatarTaskById.get(s.avatarTaskId)))
      .filter((c) => !needle || c.name.toLowerCase().includes(needle)),
    [personSubjects, avatarTaskById, needle],
  )
  const sceneCards = useMemo(
    () => sceneSubjects.map(sceneToCard).filter((c) => !needle || c.name.toLowerCase().includes(needle)),
    [sceneSubjects, needle],
  )

  const avatarVideoUrl = (t?: GenerationTask) =>
    t?.state === 'success' ? (t.creations?.[0]?.stored_url || t.creations?.[0]?.url || '') : ''

  const toggleSelect = (id: string, checked: boolean) => {
    setSelected((prev) => {
      if (checked) return prev.includes(id) ? prev : [...prev, id]
      return prev.filter((x) => x !== id)
    })
  }

  const batchDelete = async () => {
    if (selected.length === 0) return
    setDeleting(true)
    try {
      await Promise.all(selected.map((id) => businessApi.deleteGenerationTask(id)))
      message.success(`已删除 ${selected.length} 项`)
      setSelected([])
      refetch()
    } catch { /* 拦截器 */ } finally {
      setDeleting(false)
    }
  }

  /** 重试/补建形象视频（D4：幂等，未终态任务服务端直接返回） */
  const retryAvatarVideo = async (s: ViduSubject) => {
    setRetrying(s.taskId)
    try {
      await businessApi.retryAvatarVideo(s.taskId)
      message.success('形象视频任务已提交——生成中')
      refetch()
    } catch { /* 拦截器已提示 */ } finally {
      setRetrying('')
    }
  }

  const onPersonCardAction = (card: LibCard) => {
    const s = card.subject!
    if (!card.ready) {
      message.info(s.state === 'failed' ? '该数字人创建失败，可删除后重试' : '数字人仍在创建中')
      return
    }
    const avatarTask = avatarTaskById.get(s.avatarTaskId)
    const url = avatarVideoUrl(avatarTask)
    if (url) {
      setPreview({ subject: s, url })
      return
    }
    navigate(`/m/compose/lipsync?subject=${encodeURIComponent(card.serverId!)}`)
  }

  const renderCard = (card: LibCard, opts: { official?: boolean; scene?: boolean }) => {
    const checked = selected.includes(card.id)
    const actionLabel = opts.official ? '敬请期待'
      : opts.scene ? (card.ready ? '向导中选择' : '查看状态')
        : card.ready ? (avatarVideoUrl(avatarTaskById.get(card.subject!.avatarTaskId)) ? '预览形象' : '拍口播') : '查看状态'
    const tagTone = opts.scene || opts.official ? undefined
      : subjectTag(card.subject!, avatarTaskById.get(card.subject!.avatarTaskId)).tone
    const publicTag = opts.official
    return (
      <li key={card.id} className="dh-lib-card">
        {card.selectable && (
          <span
            className="dh-lib-card-check"
            onClick={(e) => {
              e.stopPropagation()
              toggleSelect(card.id, !checked)
            }}
          >
            <Checkbox checked={checked} />
          </span>
        )}

        <button
          type="button"
          className="dh-lib-card-cover"
          onClick={() => {
            if (opts.official) return
            if (opts.scene) {
              message.info('在口播向导第②步「出镜环境」中选择使用——与数字分身组合出镜')
              return
            }
            onPersonCardAction(card)
          }}
          aria-label={actionLabel}
        >
          {card.portraitUrl ? (
            <img src={card.portraitUrl} alt="" loading="lazy" />
          ) : (
            <span className="dh-lib-card-placeholder">
              {opts.scene ? <EnvironmentOutlined /> : <UserOutlined />}
            </span>
          )}

          {opts.official && <span className="dh-lib-card-badge">官方</span>}

          <span className={`dh-lib-card-tag${publicTag ? ' is-public' : ''}`} data-tone={tagTone}>
            {card.tag}
          </span>

          <span className="dh-lib-card-overlay" aria-hidden>
            <Button type="primary" size="small" tabIndex={-1}>
              {actionLabel}
            </Button>
          </span>
        </button>

        <div className="dh-lib-card-body">
          <strong className="dh-lib-card-name" title={card.name}>{card.name}</strong>
          <span className="dh-lib-card-meta">
            <span className="dh-lib-card-time">{card.timeLabel}</span>
            {card.duration && (
              <span className="dh-lib-card-dur">
                <PlayCircleOutlined />
                {card.duration}
              </span>
            )}
          </span>
          {!opts.official && !opts.scene && card.ready && (() => {
            const s = card.subject!
            const avatarTask = avatarTaskById.get(s.avatarTaskId)
            const hasVideo = !!avatarVideoUrl(avatarTask)
            const failed = !!s.avatarTaskId && (avatarTask?.state === 'failed' || avatarTask?.state === 'cancelled')
            if (hasVideo) return null
            return (
              <a
                role="button"
                style={{ fontSize: 12, display: 'inline-block', marginTop: 4 }}
                onClick={(e) => { e.stopPropagation(); retryAvatarVideo(s) }}
              >
                {retrying === s.taskId ? '提交中…' : failed ? '重试形象视频' : '生成形象视频'}
              </a>
            )
          })()}
        </div>
      </li>
    )
  }

  return (
    <div className="dh-lib">
      <header className="dh-lib-head">
        <div className="dh-lib-titles">
          <h1 className="dh-lib-title">数字资产管理</h1>
          <p className="dh-lib-lead">数字分身即选即用；注册自己的店内环境，组合出镜——分身在你的店里口播</p>
        </div>

        <div className="dh-lib-actions">
          <Input
            allowClear
            className="dh-lib-search"
            placeholder="搜索分身 / 环境"
            prefix={<SearchOutlined />}
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <Button
            icon={<EnvironmentOutlined />}
            className="dh-lib-btn-ghost"
            onClick={() => { setCreateKind('scene'); setCreateOpen(true) }}
          >
            添加环境
          </Button>
          <Button
            type="primary"
            className="dh-lib-btn-primary"
            icon={<PlusOutlined />}
            onClick={() => { setCreateKind('person'); setCreateOpen(true) }}
          >
            定制数字人
          </Button>
        </div>
      </header>

      {/* 官方主体区（平台自建资产——25 号阶段二′b 管理后台上线前占位） */}
      <section className="dh-lib-section">
        <div className="dh-lib-section-head">
          <h2 className="dh-lib-section-title">官方资产</h2>
          <Tag color="orange">筹备中</Tag>
          <span className="dh-lib-section-note">平台定制的人物形象 / 环境模板 / 音色——陆续上线</span>
          <Popconfirm
            title={`删除选中的 ${selected.length} 项？`}
            description="仅移除本地记录，Vidu 侧主体不受影响"
            okText="删除"
            okButtonProps={{ danger: true, loading: deleting }}
            cancelText="取消"
            disabled={selected.length === 0}
            onConfirm={batchDelete}
          >
            <Button
              className="dh-lib-btn-ghost"
              icon={<DeleteOutlined />}
              disabled={selected.length === 0}
            >
              批量删除 ({selected.length})
            </Button>
          </Popconfirm>
        </div>
        {officialLoading ? (
          <div className="dh-lib-empty"><Spin /></div>
        ) : officialSubjects.length === 0 ? (
          <div className="dh-lib-empty">
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="官方主体暂未上线，敬请期待" />
          </div>
        ) : (
          <ul className="dh-lib-grid" role="list">
            {officialSubjects.map((asset) => {
              const card: LibCard = {
                id: asset.id,
                name: asset.name,
                portraitUrl: asset.portrait_url || CREATIVE_CDN.pipeline.copy,
                timeLabel: '官方主体',
                tag: asset.kind === 'scene' ? '环境' : '人物',
                selectable: true,
                ready: true,
                serverId: asset.server_id,
              }
              return renderCard(card, { official: true })
            })}
          </ul>
        )}
      </section>

      {/* 我的环境（25 号 §6.5：组合出镜资产——分身 × 环境） */}
      <section className="dh-lib-section">
        <div className="dh-lib-section-head">
          <h2 className="dh-lib-section-title">我的环境</h2>
          <span className="dh-lib-section-note">{sceneCards.length} 个 · 拍 2-3 张店内照片注册，口播时与分身组合出镜</span>
        </div>
        {sceneCards.length === 0 ? (
          <div className="dh-lib-empty">
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有环境——拍两张店内/产品照片，让分身在你的店里口播">
              <Button icon={<EnvironmentOutlined />} onClick={() => { setCreateKind('scene'); setCreateOpen(true) }}>
                添加出镜环境
              </Button>
            </Empty>
          </div>
        ) : (
          <ul className="dh-lib-grid" role="list">
            {sceneCards.map((card) => renderCard(card, { scene: true }))}
          </ul>
        )}
      </section>

      {/* 我的分身（§2.3 下区；三态：创建中/形象视频生成中/可用） */}
      <section className="dh-lib-section">
        <div className="dh-lib-section-head">
          <h2 className="dh-lib-section-title">我的分身</h2>
          <span className="dh-lib-section-note">{personCards.length} 个 · 上传形象照即可创建，形象视频自动生成供预览</span>
        </div>
        {isLoading && personCards.length === 0 ? (
          <div className="dh-lib-empty">
            <Spin size="large" />
          </div>
        ) : personCards.length === 0 ? (
          <div className="dh-lib-empty">
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有数字分身，先定制一个吧">
              <Button type="primary" className="dh-lib-btn-primary" icon={<PlusOutlined />} onClick={() => { setCreateKind('person'); setCreateOpen(true) }}>
                定制数字人
              </Button>
            </Empty>
          </div>
        ) : (
          <ul className="dh-lib-grid" role="list">
            {personCards.map((card) => renderCard(card, {}))}
          </ul>
        )}
      </section>

      <CreateSubjectModal
        open={createOpen}
        kind={createKind}
        voices={myVoices}
        onClose={() => setCreateOpen(false)}
        onCreated={(serverId) => {
          refetch()
          if (searchParams.get('from') === 'wizard' && serverId && createKind === 'person') {
            navigate(`/m/compose/lipsync?subject=${encodeURIComponent(serverId)}`)
          }
        }}
      />

      {/* 分身预览弹窗（§2.3：形象视频 10s 循环 + 用此分身去创作） */}
      <Modal
        open={!!preview}
        onCancel={() => setPreview(null)}
        footer={null}
        width={480}
        title={<span><VideoCameraOutlined /> 分身预览 · {preview?.subject.name}</span>}
      >
        {preview?.url ? (
          <video
            src={preview.url}
            controls
            autoPlay
            loop
            muted
            style={{ width: '100%', maxHeight: 520, borderRadius: 12, background: '#000' }}
          />
        ) : null}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 12 }}>
          <Button onClick={() => setPreview(null)}>关闭</Button>
          <Button
            type="primary"
            onClick={() => {
              navigate(`/m/compose/lipsync?subject=${encodeURIComponent(preview!.subject.serverId)}`)
              setPreview(null)
            }}
          >
            用此分身去创作
          </Button>
        </div>
      </Modal>
    </div>
  )
}

import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import {
  Button,
  Checkbox,
  Empty,
  Input,
  Popconfirm,
  Spin,
  Tag,
} from 'antd'
import {
  DeleteOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  SearchOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../../api/business'
import { CreateSubjectModal } from '../../../../components/compose/CreateSubjectModal'
import { SubjectPreviewModal } from '../../../../components/compose/SubjectPreviewModal'
import { useSubjectList } from '../../../../hooks/useSubjectList'
import { parseGenerationTaskParams } from '../../../../utils/subjectTask'
import type { ViduSubject } from '../../../../utils/subjectTask'
import { message } from '../../../../utils/antdApp'
import { CREATIVE_CDN } from '../../../../config/creativeCdn'
import OralJourneyNav from '../../../../components/compose/OralJourneyNav'

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

/**
 * 官方主体展示区（23 号计划 §2.3）。
 * ⚠️ 临时静态样例：服务端缓存代理端点 GET /subjects?ownership=system 尚未提供
 * （见 Docs/问题反馈.md #1），端点就绪后替换为 API 数据——结构已按"即选即用"预留。
 */
const OFFICIAL_SHOWCASE: LibCard[] = [
  {
    id: 'official-1',
    name: '数字人-女-坐',
    portraitUrl: CREATIVE_CDN.pipeline.copy,
    timeLabel: '官方主体',
    tag: '公共',
    duration: '12s',
    selectable: false,
    ready: false,
  },
  {
    id: 'official-2',
    name: '数字人-男-站',
    portraitUrl: CREATIVE_CDN.pipeline.voice,
    timeLabel: '官方主体',
    tag: '公共',
    duration: '8s',
    selectable: false,
    ready: false,
  },
  {
    id: 'official-3',
    name: '数字人-女-站',
    portraitUrl: CREATIVE_CDN.pipeline.mic,
    timeLabel: '官方主体',
    tag: '影视',
    duration: '10s',
    selectable: false,
    ready: false,
  },
  {
    id: 'official-4',
    name: '数字人-男-坐',
    portraitUrl: CREATIVE_CDN.pipeline.film,
    timeLabel: '官方主体',
    tag: '公共',
    duration: '15s',
    selectable: false,
    ready: false,
  },
  {
    id: 'official-5',
    name: '数字人-女-半身',
    portraitUrl: CREATIVE_CDN.pipeline.publish,
    timeLabel: '官方主体',
    tag: '商务',
    duration: '9s',
    selectable: false,
    ready: false,
  },
]

function formatUploadTime(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `上传于 ${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 分身状态徽标（23 号计划 §2.3：创建中 / 失败 / 可用；链式形象视频上线后补"形象视频生成中"态） */
function subjectTag(s: ViduSubject) {
  if (s.state !== 'success') {
    if (s.state === 'failed') return '失败'
    return '创建中'
  }
  if (s.hasVideo) return '可用 · 视频'
  return '可用'
}

function subjectToCard(s: ViduSubject): LibCard {
  return {
    id: s.taskId,
    name: s.name,
    portraitUrl: s.portraitUrl,
    timeLabel: formatUploadTime(s.createdAt),
    tag: subjectTag(s),
    duration: s.hasVideo ? '≤5s' : undefined,
    selectable: true,
    ready: s.state === 'success' && !!s.serverId,
    serverId: s.serverId || undefined,
    subject: s,
  }
}

/** 分身管理页（23 号计划 §2.3）：上「官方主体」网格 + 下「我的分身」列表 */
export default function AvatarModule() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { subjects, tasks, refetch, isLoading } = useSubjectList({ refetchInterval: 8_000 })
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [previewSubject, setPreviewSubject] = useState<ViduSubject | null>(null)
  const [deleting, setDeleting] = useState(false)

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

  const mineCards = useMemo(() => {
    const needle = q.trim().toLowerCase()
    const all = subjects.map(subjectToCard)
    if (!needle) return all
    return all.filter((c) => c.name.toLowerCase().includes(needle) || c.tag.toLowerCase().includes(needle))
  }, [subjects, q])

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
      message.success(`已删除 ${selected.length} 个数字人`)
      setSelected([])
      refetch()
    } catch { /* 拦截器 */ } finally {
      setDeleting(false)
    }
  }

  const onCardAction = (card: LibCard) => {
    if (card.subject) {
      setPreviewSubject(card.subject)
      return
    }
    if (card.ready && card.serverId) {
      navigate(`/m/compose/lipsync?subject=${encodeURIComponent(card.serverId)}`)
      return
    }
    message.info('数字人仍在创建中')
  }

  const renderCard = (card: LibCard, opts: { official?: boolean }) => {
    const checked = selected.includes(card.id)
    const actionLabel = opts.official ? '即将接入' : '预览'
    const publicTag = card.tag === '公共' || card.tag === '影视' || card.tag === '商务'
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
            if (opts.official) {
              // 官方主体端点未接入前：样例卡引导定制同款
              setCreateOpen(true)
              return
            }
            onCardAction(card)
          }}
          aria-label={actionLabel}
        >
          {card.portraitUrl ? (
            <img src={card.portraitUrl} alt="" loading="lazy" />
          ) : (
            <span className="dh-lib-card-placeholder">
              <UserOutlined />
            </span>
          )}

          {opts.official && <span className="dh-lib-card-badge">官方</span>}

          <span className={`dh-lib-card-tag${publicTag ? ' is-public' : ''}`}>
            {card.tag}
          </span>

          <span className="dh-lib-card-overlay" aria-hidden>
            <Button type="primary" size="small" tabIndex={-1}>
              {opts.official ? '去定制同款' : actionLabel}
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
        </div>
      </li>
    )
  }

  return (
    <div className="dh-lib">
      <OralJourneyNav />
      <header className="dh-lib-head">
        <div className="dh-lib-titles">
          <h1 className="dh-lib-title">分身管理</h1>
          <p className="dh-lib-lead">官方主体即选即用；定制个人分身，跨视频人物形象一致</p>
        </div>

        <div className="dh-lib-actions">
          <Input
            allowClear
            className="dh-lib-search"
            placeholder="搜索我的分身"
            prefix={<SearchOutlined />}
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <Button
            type="primary"
            className="dh-lib-btn-primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateOpen(true)}
          >
            定制数字人
          </Button>
        </div>
      </header>

      {/* 官方主体区（§2.3 上区）：服务端缓存代理端点就绪后改为即选即用 */}
      <section className="dh-lib-section">
        <div className="dh-lib-section-head">
          <h2 className="dh-lib-section-title">官方主体</h2>
          <Tag color="orange">接入中</Tag>
          <span className="dh-lib-section-note">官方主体库即将开放（等服务端端点），当前展示样例</span>
        </div>
        <ul className="dh-lib-grid" role="list">
          {OFFICIAL_SHOWCASE.map((card) => renderCard(card, { official: true }))}
        </ul>
      </section>

      {/* 我的分身区（§2.3 下区） */}
      <section className="dh-lib-section">
        <div className="dh-lib-section-head">
          <h2 className="dh-lib-section-title">我的分身</h2>
          <span className="dh-lib-section-note">{mineCards.length} 个 · 上传形象照即可创建</span>
          <Popconfirm
            title={`删除选中的 ${selected.length} 个数字人？`}
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

        {isLoading && mineCards.length === 0 ? (
          <div className="dh-lib-empty">
            <Spin size="large" />
          </div>
        ) : mineCards.length === 0 ? (
          <div className="dh-lib-empty">
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有数字分身，先定制一个吧">
              <Button type="primary" className="dh-lib-btn-primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                定制数字人
              </Button>
            </Empty>
          </div>
        ) : (
          <ul className="dh-lib-grid" role="list">
            {mineCards.map((card) => renderCard(card, {}))}
          </ul>
        )}
      </section>

      <CreateSubjectModal
        open={createOpen}
        voices={myVoices}
        onClose={() => setCreateOpen(false)}
        onCreated={(serverId) => {
          refetch()
          // 来自向导的深链创建：完成后带 subject 回向导并自动预选新分身
          if (searchParams.get('from') === 'wizard' && serverId) {
            navigate(`/m/compose/lipsync?subject=${encodeURIComponent(serverId)}`)
          }
        }}
      />

      <SubjectPreviewModal
        open={!!previewSubject}
        subject={previewSubject}
        onClose={() => setPreviewSubject(null)}
        onUse={(serverId) => {
          setPreviewSubject(null)
          navigate(`/m/compose/lipsync?subject=${encodeURIComponent(serverId)}`)
        }}
      />
    </div>
  )
}

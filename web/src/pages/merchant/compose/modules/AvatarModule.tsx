import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import {
  Button,
  Checkbox,
  Empty,
  Input,
  Popconfirm,
  Spin,
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
import { useSubjectList } from '../../../../hooks/useSubjectList'
import { parseGenerationTaskParams } from '../../../../utils/subjectTask'
import type { ViduSubject } from '../../../../utils/subjectTask'
import { message } from '../../../../utils/antdApp'
import { CREATIVE_CDN } from '../../../../config/creativeCdn'

type Scope = 'all' | 'mine' | 'recommend'

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
  recommend?: boolean
}

const RECOMMEND_SHOWCASE: LibCard[] = [
  {
    id: 'rec-1',
    name: '数字人-女-坐',
    portraitUrl: CREATIVE_CDN.pipeline.copy,
    timeLabel: '精选模板',
    tag: '公共',
    duration: '12s',
    selectable: false,
    ready: false,
    recommend: true,
  },
  {
    id: 'rec-2',
    name: '数字人-男-站',
    portraitUrl: CREATIVE_CDN.pipeline.voice,
    timeLabel: '精选模板',
    tag: '公共',
    duration: '8s',
    selectable: false,
    ready: false,
    recommend: true,
  },
  {
    id: 'rec-3',
    name: '数字人-女-站',
    portraitUrl: CREATIVE_CDN.pipeline.mic,
    timeLabel: '精选模板',
    tag: '影视',
    duration: '10s',
    selectable: false,
    ready: false,
    recommend: true,
  },
  {
    id: 'rec-4',
    name: '数字人-男-坐',
    portraitUrl: CREATIVE_CDN.pipeline.film,
    timeLabel: '精选模板',
    tag: '公共',
    duration: '15s',
    selectable: false,
    ready: false,
    recommend: true,
  },
  {
    id: 'rec-5',
    name: '数字人-女-半身',
    portraitUrl: CREATIVE_CDN.pipeline.publish,
    timeLabel: '精选模板',
    tag: '商务',
    duration: '9s',
    selectable: false,
    ready: false,
    recommend: true,
  },
]

function formatUploadTime(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `上传于 ${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function subjectTag(s: ViduSubject) {
  if (s.state !== 'success') {
    if (s.state === 'failed') return '失败'
    return '创建中'
  }
  if (s.kind === 'scene') return '场景'
  if (s.hasVideo) return '视频'
  return '我的'
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

/** 数字人库：库式浏览 / 定制 / 批量管理（列表布局） */
export default function AvatarModule() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { subjects, tasks, refetch, isLoading } = useSubjectList({ refetchInterval: 8_000 })
  const [scope, setScope] = useState<Scope>('all')
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [createOpen, setCreateOpen] = useState(false)
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

  const mineCards = useMemo(() => subjects.map(subjectToCard), [subjects])

  const cards = useMemo(() => {
    const needle = q.trim().toLowerCase()
    let list: LibCard[] = []
    if (scope === 'recommend') {
      list = RECOMMEND_SHOWCASE
    } else if (scope === 'mine') {
      list = mineCards
    } else {
      list = [...mineCards, ...RECOMMEND_SHOWCASE]
    }
    if (!needle) return list
    return list.filter((c) => c.name.toLowerCase().includes(needle) || c.tag.toLowerCase().includes(needle))
  }, [scope, mineCards, q])

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
    if (card.recommend) {
      setCreateOpen(true)
      return
    }
    if (card.ready && card.serverId) {
      navigate(`/m/compose/lipsync?subject=${encodeURIComponent(card.serverId)}`)
      return
    }
    message.info(card.subject?.state === 'failed' ? '该数字人创建失败，可删除后重试' : '数字人仍在创建中')
  }

  return (
    <div className="dh-lib">
      <header className="dh-lib-head">
        <div className="dh-lib-titles">
          <h1 className="dh-lib-title">数字人库</h1>
          <p className="dh-lib-lead">独家高逼真数字人，满足多种应用场景</p>
        </div>

        <div className="dh-lib-toolbar">
          <div className="dh-lib-tabs" role="tablist">
            {(
              [
                { key: 'all', label: '全部' },
                { key: 'mine', label: '我的' },
                { key: 'recommend', label: '推荐' },
              ] as const
            ).map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={scope === t.key}
                className={`dh-lib-tab${scope === t.key ? ' is-active' : ''}`}
                onClick={() => {
                  setScope(t.key)
                  setSelected([])
                }}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className="dh-lib-actions">
            <Input
              allowClear
              className="dh-lib-search"
              placeholder="搜索数字人"
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
        </div>
      </header>

      {isLoading && mineCards.length === 0 ? (
        <div className="dh-lib-empty">
          <Spin size="large" />
        </div>
      ) : cards.length === 0 ? (
        <div className="dh-lib-empty">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={scope === 'mine' || scope === 'all' ? '还没有数字人，先定制一个吧' : '暂无推荐'}
          >
            {(scope === 'mine' || scope === 'all') && (
              <Button type="primary" className="dh-lib-btn-primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                定制数字人
              </Button>
            )}
          </Empty>
        </div>
      ) : (
        <ul className="dh-lib-grid" role="list">
          {cards.map((card) => {
            const checked = selected.includes(card.id)
            const actionLabel = card.recommend ? '去定制同款' : card.ready ? '拍口播' : '查看状态'
            const publicTag = card.tag === '公共' || card.tag === '影视' || card.tag === '商务'
            return (
              <li
                key={card.id}
                className={`dh-lib-card${checked ? ' is-selected' : ''}${card.recommend ? ' is-recommend' : ''}`}
              >
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
                  onClick={() => onCardAction(card)}
                  aria-label={actionLabel}
                >
                  {card.portraitUrl ? (
                    <img src={card.portraitUrl} alt="" loading="lazy" />
                  ) : (
                    <span className="dh-lib-card-placeholder">
                      <UserOutlined />
                    </span>
                  )}

                  {card.recommend && <span className="dh-lib-card-badge">推荐</span>}

                  <span className={`dh-lib-card-tag${publicTag ? ' is-public' : ''}`}>
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
                </div>
              </li>
            )
          })}
        </ul>
      )}

      <CreateSubjectModal
        open={createOpen}
        voices={myVoices}
        onClose={() => setCreateOpen(false)}
        onCreated={(serverId) => {
          refetch()
          setScope('mine')
          // 来自向导的深链创建：完成后带 subject 回向导并自动预选新分身
          if (searchParams.get('from') === 'wizard' && serverId) {
            navigate(`/m/compose/lipsync?subject=${encodeURIComponent(serverId)}`)
          }
        }}
      />
    </div>
  )
}

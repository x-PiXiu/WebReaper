import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Button,
  Checkbox,
  Empty,
  Input,
  Pagination,
  Popconfirm,
  Spin,
} from 'antd'
import {
  CaretRightOutlined,
  DeleteOutlined,
  PauseOutlined,
  PlusOutlined,
  SearchOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../../../api/business'
import { GenerateAssetModal } from '../../../../components/assets/GenerateAssetModal'
import { GENERATION_TASKS_KEY, useGenerationTasks } from '../../../../hooks/useGenerationTasks'
import type { GenerationVoice } from '../../../../types/api'
import { toast } from '../../../../utils/feedback'
import { parseGenerationTaskParams } from '../../../../utils/subjectTask'

type Scope = 'all' | 'mine' | 'recommend'

/** 每页展示条数（官方+克隆音色可能上百条，分页避免长列表） */
const PAGE_SIZE = 12

type VoiceCard = {
  id: string
  name: string
  timeLabel: string
  tag: string
  sampleUrl?: string
  selectable: boolean
  mine: boolean
  taskId?: string
  voiceId: string
}

/** 全局试听（切换条目自动停上一段） */
let previewAudio: HTMLAudioElement | null = null
function stopPreview() {
  if (previewAudio) {
    previewAudio.pause()
    previewAudio = null
  }
}

function formatUploadTime(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `上传于 ${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 音色库：官方音色 + 我的克隆音色（列表布局） */
export default function VoiceModule() {
  const queryClient = useQueryClient()
  const { tasks, refetch, isLoading: tasksLoading } = useGenerationTasks({ refetchInterval: 8_000 })
  const [scope, setScope] = useState<Scope>('all')
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [cloneOpen, setCloneOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [playingId, setPlayingId] = useState('')
  const playingRef = useRef('')

  const { data: official = [], isLoading: voicesLoading } = useQuery({
    queryKey: ['generation-voices'],
    queryFn: () => businessApi.listGenerationVoices().then((r) => r.voices),
    staleTime: 24 * 60 * 60 * 1000,
  })

  useEffect(() => () => { stopPreview() }, [])

  const mineCards = useMemo((): VoiceCard[] => {
    const cards: VoiceCard[] = []
    for (const t of tasks || []) {
      if (t.sub_type !== 'voice_clone') continue
      const p = parseGenerationTaskParams(t)
      const voiceId = typeof p.voice_id === 'string' && p.voice_id
        ? p.voice_id
        : (t.provider_task_id || t.id)
      const name = (typeof p.name === 'string' && p.name)
        || (typeof p.voice_name === 'string' && p.voice_name)
        || `克隆音色 ${voiceId.slice(0, 8)}`
      const sample = t.creations?.[0]?.url || ''
      let tag = '我的'
      if (t.state === 'failed') tag = '失败'
      else if (t.state !== 'success') tag = '克隆中'
      cards.push({
        id: `mine-${t.id}`,
        name,
        timeLabel: formatUploadTime(t.created_at),
        tag,
        sampleUrl: sample || undefined,
        selectable: true,
        mine: true,
        taskId: t.id,
        voiceId,
      })
    }
    return cards
  }, [tasks])

  const myVoiceIds = useMemo(
    () => mineCards.filter((c) => c.tag === '我的').map((c) => c.voiceId),
    [mineCards],
  )

  const officialCards = useMemo((): VoiceCard[] => {
    return (official as GenerationVoice[]).map((v) => ({
      id: `off-${v.voice_id}`,
      name: `${v.name}（${v.language}）`,
      timeLabel: '官方音色',
      tag: '公共',
      sampleUrl: v.sample_url || undefined,
      selectable: false,
      mine: false,
      voiceId: v.voice_id,
    }))
  }, [official])

  // 推荐走服务端 recommend 标记（077 迁移）；旧库未打标时兜底前 12 条
  const recommendCards = useMemo(() => {
    const marked = (official as GenerationVoice[]).filter((v) => v.recommend)
    const base = marked.length ? marked : (official as GenerationVoice[]).slice(0, 12)
    const ids = new Set(base.map((v) => v.voice_id))
    return officialCards
      .filter((c) => ids.has(c.voiceId))
      .map((c) => ({ ...c, tag: c.tag || '精选', id: `rec-${c.voiceId}` }))
  }, [official, officialCards])

  const cards = useMemo(() => {
    const needle = q.trim().toLowerCase()
    let list: VoiceCard[] = []
    if (scope === 'mine') list = mineCards
    else if (scope === 'recommend') list = recommendCards.length ? recommendCards : officialCards
    else list = [...mineCards, ...officialCards]
    if (!needle) return list
    return list.filter((c) =>
      c.name.toLowerCase().includes(needle)
      || c.voiceId.toLowerCase().includes(needle)
      || c.tag.toLowerCase().includes(needle),
    )
  }, [scope, mineCards, officialCards, recommendCards, q])

  // 分页：筛选/搜索/切换 Tab 后回到第一页；翻页停掉试听
  const [page, setPage] = useState(1)
  const safePage = Math.min(page, Math.max(1, Math.ceil(cards.length / PAGE_SIZE)))
  useEffect(() => { setPage(1) }, [scope, q])
  useEffect(() => {
    if (page !== safePage) setPage(safePage)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [safePage])
  const paged = useMemo(
    () => cards.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE),
    [cards, safePage],
  )
  const changePage = (p: number) => {
    stopPreview()
    playingRef.current = ''
    setPlayingId('')
    setPage(p)
  }

  const toggleSelect = (id: string, checked: boolean) => {
    setSelected((prev) => {
      if (checked) return prev.includes(id) ? prev : [...prev, id]
      return prev.filter((x) => x !== id)
    })
  }

  const batchDelete = async () => {
    if (selected.length === 0) return
    const taskIds = selected
      .map((id) => mineCards.find((c) => c.id === id)?.taskId)
      .filter((id): id is string => !!id)
    if (taskIds.length === 0) return
    setDeleting(true)
    try {
      await Promise.all(taskIds.map((id) => businessApi.deleteGenerationTask(id)))
      toast.ok(`已删除 ${taskIds.length} 个音色`)
      setSelected([])
      refetch()
    } catch { /* 拦截器 */ } finally {
      setDeleting(false)
    }
  }

  const togglePlay = (card: VoiceCard) => {
    if (!card.sampleUrl) {
      toast.info(card.mine ? '克隆音色暂无试听样例' : '该音色暂无试听')
      return
    }
    if (playingRef.current === card.id && previewAudio) {
      stopPreview()
      playingRef.current = ''
      setPlayingId('')
      return
    }
    stopPreview()
    previewAudio = new Audio(card.sampleUrl)
    playingRef.current = card.id
    setPlayingId(card.id)
    previewAudio.onended = () => {
      playingRef.current = ''
      setPlayingId('')
      stopPreview()
    }
    previewAudio.play().catch(() => {
      playingRef.current = ''
      setPlayingId('')
      toast.warn('试听播放失败')
    })
  }

  const loading = (voicesLoading || tasksLoading) && cards.length === 0

  return (
    <div className="vc-lib">
      <header className="vc-lib-head">
        <div className="vc-lib-titles">
          <h1 className="vc-lib-title" style={{ marginTop: 10 }}>音色库</h1>
          <p className="vc-lib-lead">AI 情感音色，真人般自然流畅</p>
        </div>

        <div className="vc-lib-toolbar">
          <div className="vc-lib-tabs" role="tablist">
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
                className={`vc-lib-tab${scope === t.key ? ' is-active' : ''}`}
                onClick={() => {
                  setScope(t.key)
                  setSelected([])
                }}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className="vc-lib-actions">
            <Input
              allowClear
              className="vc-lib-search"
              placeholder="搜索音色"
              prefix={<SearchOutlined />}
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
            <Button
              type="primary"
              className="vc-lib-btn-primary"
              icon={<PlusOutlined />}
              onClick={() => setCloneOpen(true)}
            >
              定制音色
            </Button>
            <Popconfirm
              title={`删除选中的 ${selected.length} 个音色？`}
              description="仅移除本地克隆记录"
              okText="删除"
              okButtonProps={{ danger: true, loading: deleting }}
              cancelText="取消"
              disabled={selected.length === 0}
              onConfirm={batchDelete}
            >
              <Button
                className="vc-lib-btn-ghost"
                icon={<DeleteOutlined />}
                disabled={selected.length === 0}
              >
                批量删除 ({selected.length})
              </Button>
            </Popconfirm>
          </div>
        </div>
      </header>

      {loading ? (
        <div className="vc-lib-empty">
          <Spin size="large" />
        </div>
      ) : cards.length === 0 ? (
        <div className="vc-lib-empty">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={scope === 'mine' ? '还没有克隆音色，先定制一个吧' : '暂无音色'}
          >
            {scope === 'mine' && (
              <Button type="primary" className="vc-lib-btn-primary" icon={<PlusOutlined />} onClick={() => setCloneOpen(true)}>
                定制音色
              </Button>
            )}
          </Empty>
        </div>
      ) : (
        <>
          <ul className="vc-lib-list" role="list">
            {paged.map((card) => {
              const checked = selected.includes(card.id)
              const playing = playingId === card.id
              return (
                <li
                  key={card.id}
                  className={`vc-lib-row${checked ? ' is-selected' : ''}${playing ? ' is-playing' : ''}`}
                >
                  {card.selectable ? (
                    <label className="vc-lib-row-check">
                      <Checkbox
                        checked={checked}
                        onChange={(e) => toggleSelect(card.id, e.target.checked)}
                      />
                    </label>
                  ) : (
                    <span className="vc-lib-row-check is-spacer" aria-hidden />
                  )}

                  <button
                    type="button"
                    className="vc-lib-row-play"
                    onClick={() => togglePlay(card)}
                    aria-label={playing ? '暂停试听' : '试听'}
                  >
                    {playing ? <PauseOutlined /> : <CaretRightOutlined />}
                  </button>

                  <div className="vc-lib-row-main">
                    <strong className="vc-lib-row-name" title={card.name}>{card.name}</strong>
                    <span className="vc-lib-row-time">{card.timeLabel}</span>
                  </div>

                  <span className={`vc-lib-row-tag${card.tag === '公共' ? ' is-public' : ''}`}>
                    {card.tag}
                  </span>
                </li>
              )
            })}
          </ul>

          {cards.length > PAGE_SIZE && (
            <div className="vc-lib-pagination">
              <Pagination
                current={safePage}
                pageSize={PAGE_SIZE}
                total={cards.length}
                onChange={changePage}
                showSizeChanger={false}
                showTotal={(total) => `共 ${total} 个音色`}
              />
            </div>
          )}
        </>
      )}

      <GenerateAssetModal
        open={cloneOpen}
        type="voice"
        myVoices={myVoiceIds}
        onClose={() => setCloneOpen(false)}
        onGenerated={() => {
          queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
          refetch()
          setScope('mine')
          setCloneOpen(false)
        }}
      />
    </div>
  )
}

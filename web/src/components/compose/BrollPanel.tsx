import { useEffect, useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Input, Space, Tag, Typography } from 'antd'
import {
  CheckCircleFilled, EditOutlined, FieldTimeOutlined, PlusOutlined,
  ReloadOutlined, VideoCameraAddOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../api/business'
import { waitGenerationTask } from '../../hooks/useLipSyncPipeline'
import { useGenerationTasks, GENERATION_TASKS_KEY } from '../../hooks/useGenerationTasks'
import AssetPicker from '../AssetPicker'
import { toast } from '../../utils/feedback'
import type { TaskTimeline } from '../../types/api'

const { Text, Title } = Typography

export interface BrollSource {
  /** 源成片的生成任务 ID（timeline / compose 的锚点） */
  taskId: string
  title?: string
  /** 源片预览地址（作品卡带出的 media_urls[0]） */
  videoUrl?: string
}

const fmt = (ms: number) => {
  const s = Math.max(0, Math.round(ms / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

type ViewTab = 'source' | 'composed'

type Props = {
  source: BrollSource
  /** page = 作品详情全页；embedded = 弹窗内 */
  variant?: 'page' | 'embedded'
  onClose?: () => void
  /** 页头额外操作（如去发布） */
  extraActions?: React.ReactNode
}

/**
 * B-Roll 核心面板（23 §5 / §8#4）：播放器 + 源/新片 + 台词轴挂片段 + 合成。
 * 抽屉与作品详情页共用。
 */
export function BrollPanel({ source, variant = 'embedded', onClose, extraActions }: Props) {
  const queryClient = useQueryClient()
  const [timeline, setTimeline] = useState<TaskTimeline | null>(null)
  const [locating, setLocating] = useState(false)
  const [locateError, setLocateError] = useState('')
  const [segments, setSegments] = useState<Record<number, string>>({})
  const [pickerIndex, setPickerIndex] = useState<number | null>(null)
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [editingText, setEditingText] = useState('')
  const [composeTaskId, setComposeTaskId] = useState('')
  const [composing, setComposing] = useState(false)
  const [chainSource, setChainSource] = useState<BrollSource | null>(null)
  const [viewTab, setViewTab] = useState<ViewTab>('source')
  const activeSource = chainSource || source

  const { tasks: genTasks = [] } = useGenerationTasks({ enabled: true })
  const composeTask = useMemo(
    () => genTasks.find((t) => t.id === composeTaskId),
    [genTasks, composeTaskId],
  )
  const composeDone = composeTask?.state === 'success'
  const composeUrl = composeTask?.creations?.[0]?.stored_url || composeTask?.creations?.[0]?.url || ''

  const playerUrl = viewTab === 'composed' && composeUrl
    ? composeUrl
    : (activeSource.videoUrl || '')

  useEffect(() => {
    if (!activeSource.taskId) return
    setTimeline(null)
    setLocateError('')
    setSegments({})
    setComposeTaskId('')
    setViewTab('source')
    setLocating(true)
    businessApi.getTaskTimeline(activeSource.taskId)
      .then(setTimeline)
      .catch(async () => {
        try {
          setTimeline(await businessApi.locateTaskTimeline(activeSource.taskId))
        } catch (e: any) {
          setLocateError(e?.response?.data?.msg || e?.message || '台词定位失败，请重试')
        }
      })
      .finally(() => setLocating(false))
  }, [activeSource.taskId])

  useEffect(() => {
    if (composeDone && composeUrl) setViewTab('composed')
  }, [composeDone, composeUrl])

  // 片段变更后允许再次合成（不再锁死在上一轮成功态）
  useEffect(() => {
    if (!composeTaskId) return
    setComposeTaskId('')
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 仅跟随 segments
  }, [segments])

  const segmentCount = Object.keys(segments).length

  const saveLineText = async (index: number, text: string) => {
    setEditingIndex(null)
    try {
      const next = await businessApi.locateTaskTimeline(activeSource.taskId, {
        lines_override: [{ index, text }],
      })
      setTimeline(next)
      toast.ok('台词已修正', 'broll-line')
    } catch (e: any) {
      toast.fail(e?.response?.data?.msg || '修正失败，请重试', 'broll-line')
    }
  }

  const doCompose = async () => {
    if (segmentCount === 0) return
    setComposing(true)
    try {
      const t = await businessApi.submitCompose({
        source_task_id: activeSource.taskId,
        segments: Object.entries(segments).map(([idx, media_url]) => ({
          sentence_index: Number(idx),
          media_url,
        })),
      })
      setComposeTaskId(t.id)
      toast.info('正在合成画面…', 'broll-compose')
      await waitGenerationTask(t.id, () => {
        queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
      })
      queryClient.invalidateQueries({ queryKey: ['merchant-works'] })
      toast.ok('合成完成，新成片已入作品库', 'broll-compose')
    } catch (e: any) {
      toast.fail(e?.response?.data?.msg || e?.message || '合成失败，可重试', 'broll-compose')
    } finally {
      setComposing(false)
    }
  }

  const chainToComposed = () => {
    if (!composeTaskId || !composeUrl) return
    setChainSource({
      taskId: composeTaskId,
      title: `${activeSource.title || '口播成片'} · 新成片`,
      videoUrl: composeUrl,
    })
  }

  return (
    <>
      <div className={`wr-broll-detail${variant === 'page' ? ' wr-broll-detail--page' : ''}`}>
        <aside className="wr-broll-player">
          <div className="wr-broll-tabs" role="tablist" aria-label="成片版本">
            <button
              type="button"
              role="tab"
              aria-selected={viewTab === 'source'}
              className={viewTab === 'source' ? 'is-active' : undefined}
              onClick={() => setViewTab('source')}
            >
              {chainSource ? '当前源片' : '源片'}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={viewTab === 'composed'}
              className={viewTab === 'composed' ? 'is-active' : undefined}
              disabled={!composeUrl}
              onClick={() => composeUrl && setViewTab('composed')}
            >
              含插入{composeDone ? '' : composeTaskId ? '（合成中）' : ''}
            </button>
          </div>
          {playerUrl ? (
            <video key={playerUrl} src={playerUrl} controls className="wr-broll-video" />
          ) : (
            <div className="wr-broll-video-empty">暂无预览地址</div>
          )}
          {composeTaskId && (
            <div className="wr-broll-compose-status">
              {composeDone ? (
                <Space wrap size={8}>
                  <Tag color="green" icon={<CheckCircleFilled />}>合成完成 · 已入作品库</Tag>
                  <Button size="small" icon={<VideoCameraAddOutlined />} onClick={chainToComposed}>
                    对新成片继续插入
                  </Button>
                </Space>
              ) : (
                <Tag color="processing">{composeTask?.state === 'running' ? '合成中…' : '排队中…'}</Tag>
              )}
            </div>
          )}
        </aside>

        <section className="wr-broll-script">
          <Alert
            type="info"
            showIcon
            icon={<FieldTimeOutlined />}
            message="插入点按文案行对齐，一行一句效果最佳"
            description={
              timeline?.script_source === 'asr'
                ? '台词来自语音识别，可点铅笔修正文字（不改切换点）；片段支持视频与图片'
                : '给指定句挂素材后点合成；该句画面切换为素材，口播声全程不变'
            }
            style={{ marginBottom: 12 }}
          />

          {locating && (
            <div className="wr-broll-center">
              <Text type="secondary">正在按语音定位台词时间轴…</Text>
            </div>
          )}

          {locateError && !locating && (
            <div className="wr-broll-center">
              <Text type="secondary">{locateError}</Text>
              <div style={{ marginTop: 12 }}>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={async () => {
                    setLocateError(''); setLocating(true)
                    try {
                      setTimeline(await businessApi.locateTaskTimeline(activeSource.taskId, { force: true }))
                    } catch (e: any) {
                      setLocateError(e?.response?.data?.msg || e?.message || '定位失败')
                    } finally {
                      setLocating(false)
                    }
                  }}
                >
                  重新定位
                </Button>
              </div>
            </div>
          )}

          {timeline && timeline.lines.length > 0 && (
            <>
              <div className="wr-broll-script-head">
                <Title level={5} style={{ margin: 0 }}>台词与插入点</Title>
                <Text type="secondary" style={{ fontSize: 12 }}>已挂 {segmentCount} 句</Text>
              </div>
              <div className="wr-broll-lines">
                {timeline.lines.map((line) => {
                  const media = segments[line.index]
                  return (
                    <div
                      key={line.index}
                      className={`wr-broll-line${media ? ' is-hung' : ''}`}
                    >
                      <Tag style={{ margin: 0, fontVariantNumeric: 'tabular-nums' }}>
                        {fmt(line.start_ms)}–{fmt(line.end_ms)}
                      </Tag>
                      {editingIndex === line.index ? (
                        <Space size={6} style={{ flex: 1, minWidth: 0 }}>
                          <Input
                            size="small"
                            value={editingText}
                            onChange={(e) => setEditingText(e.target.value)}
                            onPressEnter={() => saveLineText(line.index, editingText)}
                          />
                          <Button size="small" type="primary" onClick={() => saveLineText(line.index, editingText)}>保存</Button>
                          <Button size="small" type="text" onClick={() => setEditingIndex(null)}>取消</Button>
                        </Space>
                      ) : (
                        <Text className="wr-broll-line-text">
                          {line.text}
                          {line.estimated && (
                            <Tag style={{ marginLeft: 6, fontSize: 10 }} title="边界为估算">估算</Tag>
                          )}
                        </Text>
                      )}
                      {media ? (
                        <div className="wr-broll-hung">
                          {/\.(png|jpe?g|webp|gif)(\?|$)/i.test(media) ? (
                            <img src={media} alt="" className="wr-broll-thumb" />
                          ) : (
                            <span className="wr-broll-thumb wr-broll-thumb--video" aria-hidden />
                          )}
                          <Button size="small" type="text" onClick={() => setPickerIndex(line.index)}>更换</Button>
                          <Button
                            size="small"
                            type="text"
                            danger
                            onClick={() => setSegments((prev) => {
                              const next = { ...prev }
                              delete next[line.index]
                              return next
                            })}
                          >
                            移除
                          </Button>
                        </div>
                      ) : (
                        editingIndex !== line.index && (
                          <Space size={0}>
                            <Button
                              size="small"
                              type="text"
                              icon={<EditOutlined />}
                              title="修正该句文字"
                              onClick={() => { setEditingIndex(line.index); setEditingText(line.text) }}
                            />
                            <Button
                              size="small"
                              icon={<PlusOutlined />}
                              onClick={() => setPickerIndex(line.index)}
                            >
                              插入
                            </Button>
                          </Space>
                        )
                      )}
                    </div>
                  )
                })}
              </div>
            </>
          )}

          <div className="wr-broll-footer">
            {chainSource && (
              <button type="button" className="wr-text-btn" onClick={() => setChainSource(null)}>
                回到最初源片
              </button>
            )}
            <div className="wr-broll-footer-right">
              {extraActions}
              {onClose && variant !== 'page' && (
                <button type="button" className="wr-text-btn" onClick={onClose}>关闭</button>
              )}
              <Button
                type="primary"
                className="ip-btn-primary"
                disabled={segmentCount === 0}
                loading={composing}
                onClick={() => { void doCompose() }}
              >
                {composeDone
                  ? `重新合成（${segmentCount} 句）`
                  : `合成成片${segmentCount > 0 ? `（${segmentCount} 句）` : ''}`}
              </Button>
            </div>
          </div>
        </section>
      </div>

      <AssetPicker
        open={pickerIndex !== null}
        mode="single"
        accept="visual"
        title={`选择插入片段（第 ${(pickerIndex ?? 0) + 1} 句）`}
        onClose={() => setPickerIndex(null)}
        onSelect={(assets) => {
          if (pickerIndex !== null && assets[0]?.url) {
            setSegments((prev) => ({ ...prev, [pickerIndex]: assets[0].url }))
          }
        }}
      />
    </>
  )
}

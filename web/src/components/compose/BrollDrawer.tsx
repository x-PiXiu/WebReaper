import { useEffect, useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Input, Modal, Space, Tag, Typography } from 'antd'
import {
  CheckCircleFilled, EditOutlined, FieldTimeOutlined, PlusOutlined,
  ReloadOutlined, VideoCameraAddOutlined,
} from '@ant-design/icons'
import { businessApi } from '../../api/business'
import { waitGenerationTask } from '../../hooks/useLipSyncPipeline'
import { useGenerationTasks, GENERATION_TASKS_KEY } from '../../hooks/useGenerationTasks'
import AssetPicker from '../AssetPicker'
import { MODAL_W, modalBodyScroll } from '../../ui/modalFit'
import { message } from '../../utils/antdApp'
import type { TaskTimeline } from '../../types/api'

const { Text, Title } = Typography

// mm:ss 格式（台词行时间戳）
const fmt = (ms: number) => {
  const s = Math.max(0, Math.round(ms / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

export interface BrollSource {
  /** 源成片的生成任务 ID（timeline / compose 的锚点） */
  taskId: string
  title?: string
  /** 源片预览地址（作品卡带出的 media_urls[0]） */
  videoUrl?: string
}

/**
 * B-Roll 插入抽屉（22/23 号计划 · 阶段3 成片后处理）：
 * 打开即读时间轴（未定位自动触发定位）→ 逐句 [+插入画面] 挂素材（纯前端配置）
 * → [合成成片] 提交 compose 任务（后端换算时间窗）→ 轮询进度 → 新成片入作品库。
 * 三条音频路径（TTS/文本直生/上传录音）的成片都按句对齐——静音检测定位三路通吃。
 */
export default function BrollDrawer({ open, onClose, source }: {
  open: boolean
  onClose: () => void
  source: BrollSource | null
}) {
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
  // 链式再调整（23 号计划 §5.1：以新成片为源继续插入，时间轴服务端自动继承）
  const [chainSource, setChainSource] = useState<BrollSource | null>(null)
  const activeSource = chainSource || source

  const { tasks: genTasks = [] } = useGenerationTasks({ enabled: open })
  const composeTask = useMemo(
    () => genTasks.find((t) => t.id === composeTaskId),
    [genTasks, composeTaskId],
  )
  const composeDone = composeTask?.state === 'success'
  const composeUrl = composeTask?.creations?.[0]?.stored_url || composeTask?.creations?.[0]?.url || ''

  useEffect(() => {
    if (!open || !activeSource?.taskId) return
    setTimeline(null)
    setLocateError('')
    setSegments({})
    setComposeTaskId('')
    setLocating(true)
    // 先读缓存时间轴；未定位（404）则自动触发首次定位
    // 链式场景（compose 产物）服务端已继承源片时间轴——这里直接命中缓存
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
  }, [open, activeSource?.taskId])

  const segmentCount = Object.keys(segments).length

  const saveLineText = async (index: number, text: string) => {
    if (!activeSource) return
    setEditingIndex(null)
    try {
      const next = await businessApi.locateTaskTimeline(activeSource.taskId, {
        lines_override: [{ index, text }],
      })
      setTimeline(next)
      message.success('台词已修正（切换点不受影响）')
    } catch (e: any) {
      message.error(e?.response?.data?.msg || '修正失败')
    }
  }

  const doCompose = async () => {
    if (!activeSource || segmentCount === 0) return
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
      message.info('合成任务已提交')
      await waitGenerationTask(t.id, () => {
        queryClient.invalidateQueries({ queryKey: GENERATION_TASKS_KEY })
      })
      queryClient.invalidateQueries({ queryKey: ['merchant-works'] })
      message.success('合成完成——新成片已入作品库（源片保留）')
    } catch (e: any) {
      message.error(e?.response?.data?.msg || e?.message || '合成失败，可重试')
    } finally {
      setComposing(false)
    }
  }

  /** 以合成产物为源继续调整（时间轴自动继承，无需重新定位） */
  const chainToComposed = () => {
    if (!composeTaskId || !composeUrl) return
    setChainSource({
      taskId: composeTaskId,
      title: `${activeSource?.title || '口播成片'} · 新成片`,
      videoUrl: composeUrl,
    })
    // effect 依赖 activeSource.taskId 变化——自动重载（时间轴命中继承缓存）
  }

  if (!source) return null

  // 关闭时回到源片视角（下次打开重新从源片开始）
  const handleWrapperClose = () => {
    setChainSource(null)
    onClose()
  }

  return (
    <Modal
      open={open}
      onCancel={handleWrapperClose}
      width={MODAL_W.lg}
      title={<Space><VideoCameraAddOutlined /> 插入画面 · {activeSource?.title || '口播成片'}</Space>}
      footer={null}
      destroyOnHidden
      styles={{ body: { ...modalBodyScroll.body, background: 'var(--wr-bg)' } }}
    >
      <Space direction="vertical" size={14} style={{ width: '100%' }}>
        {activeSource?.videoUrl && (
          <video
            src={activeSource.videoUrl}
            controls
            style={{ width: '100%', maxHeight: 320, borderRadius: 12, background: '#000' }}
          />
        )}

        <Alert
          type="info"
          showIcon
          icon={<FieldTimeOutlined />}
          message="插入点按台词句对齐——一行一句效果最佳"
          description={
            timeline?.script_source === 'asr'
              ? '台词来自语音识别，文字如有出入可直接点击修改（只改文字，不影响画面切换点）；片段支持视频与图片（图片按该句时长自动转视频）'
              : '给指定句挂上素材片段（视频或图片），合成时该句画面切换为素材，口播声音全程不变'
          }
        />

        {locating && (
          <div style={{ textAlign: 'center', padding: '32px 0' }}>
            <Text type="secondary">正在按语音定位台词时间轴…</Text>
          </div>
        )}

        {locateError && !locating && (
          <div style={{ textAlign: 'center', padding: '24px 0' }}>
            <Text type="secondary">{locateError}</Text>
            <div style={{ marginTop: 12 }}>
              <Button
                icon={<ReloadOutlined />}
                onClick={async () => {
                  if (!activeSource) return
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
          <div className="ip-panel" style={{ padding: '12px 16px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
              <Title level={5} style={{ margin: 0 }}>台词与插入点</Title>
              <Text type="secondary" style={{ fontSize: 12 }}>已挂片段 {segmentCount} 句</Text>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {timeline.lines.map((line) => {
                const media = segments[line.index]
                return (
                  <div
                    key={line.index}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 10,
                      padding: '8px 10px', borderRadius: 10,
                      border: `1px solid ${media ? 'var(--wr-primary-border)' : 'var(--wr-border)'}`,
                      background: media ? 'var(--wr-primary-bg)' : 'transparent',
                    }}
                  >
                    <Tag style={{ margin: 0, fontVariantNumeric: 'tabular-nums' }}>
                      {fmt(line.start_ms)}–{fmt(line.end_ms)}
                    </Tag>
                    {editingIndex === line.index ? (
                      <Space size={6} style={{ flex: 1 }}>
                        <Input
                          size="small"
                          value={editingText}
                          onChange={(e) => setEditingText(e.target.value)}
                          onPressEnter={() => saveLineText(line.index, editingText)}
                          style={{ maxWidth: 360 }}
                        />
                        <Button size="small" type="primary" onClick={() => saveLineText(line.index, editingText)}>保存</Button>
                        <Button size="small" type="text" onClick={() => setEditingIndex(null)}>取消</Button>
                      </Space>
                    ) : (
                      <Text style={{ flex: 1, fontSize: 13 }}>
                        {line.text}
                        {line.estimated && (
                          <Tag style={{ marginLeft: 6, fontSize: 10 }} title="该句边界为估算，插入点可能略有偏移">估算</Tag>
                        )}
                      </Text>
                    )}
                    {media ? (
                      <Space size={4}>
                        <Tag color="purple" style={{ margin: 0 }} icon={<CheckCircleFilled />}>已挂片段</Tag>
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
                      </Space>
                    ) : (
                      editingIndex !== line.index && (
                        <Space size={0}>
                          <Button
                            size="small"
                            type="text"
                            icon={<EditOutlined />}
                            title="修正该句文字（ASR 识别有误时用）"
                            onClick={() => { setEditingIndex(line.index); setEditingText(line.text) }}
                          />
                          <Button
                            size="small"
                            icon={<PlusOutlined />}
                            onClick={() => setPickerIndex(line.index)}
                          >
                            插入画面
                          </Button>
                        </Space>
                      )
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {composeTaskId && (
          <div className="ip-panel" style={{ padding: 14 }}>
            <Space direction="vertical" size={6} style={{ width: '100%' }}>
              <Text strong>插入合成</Text>
              {composeDone ? (
                <Space wrap>
                  <Tag color="green" icon={<CheckCircleFilled />}>合成完成</Tag>
                  {composeUrl && (
                    <Button size="small" type="primary" onClick={() => window.open(composeUrl, '_blank', 'noopener')}>
                      播放新成片
                    </Button>
                  )}
                  {composeUrl && (
                    <Button size="small" icon={<VideoCameraAddOutlined />} onClick={chainToComposed}>
                      对新成片继续插入
                    </Button>
                  )}
                  <Text type="secondary" style={{ fontSize: 12 }}>已入作品库，源片保留；时间轴自动继承，可继续调整</Text>
                </Space>
              ) : (
                <Space>
                  <Tag color="processing">{composeTask?.state === 'running' ? '合成中…' : '排队中…'}</Tag>
                  <Text type="secondary" style={{ fontSize: 12 }}>任务 {composeTaskId}</Text>
                </Space>
              )}
            </Space>
          </div>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          {chainSource && (
            <Button style={{ marginRight: 'auto' }} onClick={() => setChainSource(null)}>
              回到源片
            </Button>
          )}
          <Button onClick={handleWrapperClose}>关闭</Button>
          <Button
            type="primary"
            disabled={segmentCount === 0 || !!composeDone}
            loading={composing}
            onClick={doCompose}
          >
            合成成片{segmentCount > 0 ? `（${segmentCount} 句）` : ''}
          </Button>
        </div>
      </Space>

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
    </Modal>
  )
}

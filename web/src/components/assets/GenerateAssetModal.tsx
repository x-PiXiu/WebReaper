import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Alert, Button, Input, InputNumber, Modal, Segmented, Select, Space, Typography, Upload,
} from 'antd'
import {
  DeleteOutlined, PictureOutlined, PlusOutlined, SoundOutlined,
  UploadOutlined, VideoCameraOutlined, BulbOutlined, CheckCircleFilled,
} from '@ant-design/icons'
import { businessApi } from '../../api/business'
import {
  mergeSubmitParams,
  submitGenerationTaskCompat,
  submitUnified,
} from '../../api/generationSubmit'
import { useBrandContext } from '../../hooks/useBrands'
import { useGenerationTask } from '../../hooks/useGenerationTasks'
import AssetPicker from '../AssetPicker'
import { GenerationTaskStatusBar } from '../compose/GenerationTaskStatusBar'
import { ImageCover } from '../ImageCover'
import VoicePicker from '../VoicePicker'
import { MODAL_W, modalBodyScroll } from '../../ui/modalFit'
import { IMAGE_PROMPT_CHIPS, VIDEO_PROMPT_CHIPS } from '../../data/generatePromptChips'
import type { GenerationTask, ModelCapability } from '../../types/api'
import { resolveMediaUrl, taskCoverUrl, taskPrimaryUrl } from '../../utils/generationTask'
import { message } from '../../utils/antdApp'

const { Text } = Typography

type GenType = 'image' | 'video' | 'audio' | 'voice'

const META: Record<GenType, { title: string; lead: string; icon: ReactNode; tone: string }> = {
  image: {
    title: '生成图片',
    lead: '描述画面即可，可选参考图；完成后自动进入素材库',
    icon: <PictureOutlined />,
    tone: 'violet',
  },
  video: {
    title: '生成视频',
    lead: '文生短视频，可调时长与比例；适合片段素材',
    icon: <VideoCameraOutlined />,
    tone: 'teal',
  },
  audio: {
    title: '生成配音',
    lead: '输入文案并选择音色，合成可复用音频素材',
    icon: <SoundOutlined />,
    tone: 'amber',
  },
  voice: {
    title: '克隆音色',
    lead: '上传参考人声，生成专属音色 ID',
    icon: <SoundOutlined />,
    tone: 'rose',
  },
}

type Props = {
  open: boolean
  type: GenType
  myVoices?: string[]
  onClose: () => void
  onGenerated: () => void
}

export function GenerateAssetModal({ open, type, myVoices = [], onClose, onGenerated }: Props) {
  const { brandId } = useBrandContext()
  const meta = META[type]
  const subType = type === 'image' ? 'text2image'
    : type === 'video' ? 'text2video'
    : type === 'audio' ? 'tts'
    : 'voice_clone'

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
  const [resultPreview, setResultPreview] = useState<{ url: string; cover?: string } | null>(null)
  const doneNotified = useRef(false)

  const { data: types = [], isLoading: typesLoading } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
    enabled: open,
  })

  const models = useMemo(() => {
    const t = types.find((x) => x.sub_type === subType)
    return t?.models || []
  }, [types, subType])

  const cap: ModelCapability | undefined = useMemo(() => {
    const entry = models.find((m) => m.model === model) || models[0]
    return entry?.capability
  }, [models, model])

  const { task } = useGenerationTask(taskId || undefined)
  const taskState = task?.state ?? (taskId ? 'processing' : null)
  const isVisual = type === 'image' || type === 'video'

  useEffect(() => {
    if (!open) return
    setText('')
    setVoiceId('')
    setAudioFile(null)
    setVoiceName('')
    setRefImages([])
    setTaskId(null)
    setResultPreview(null)
    setGenerating(false)
    doneNotified.current = false
  }, [open, type])

  useEffect(() => {
    if (!open || models.length === 0) return
    const preferred = type === 'image'
      ? (models.find((m) => m.model === 'viduq2') || models[0])
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
    const ratios = c.aspect_ratios || []
    setAspectRatio(
      ratios.includes('9:16') ? '9:16'
        : ratios.includes('16:9') ? '16:9'
        : ratios[0] || (type === 'video' ? '16:9' : ''),
    )
  }, [open, type, models])

  useEffect(() => {
    if (!task || !taskId) return
    if (task.state === 'success' && !doneNotified.current) {
      doneNotified.current = true
      const url = taskPrimaryUrl(task)
      if (url) setResultPreview({ url, cover: taskCoverUrl(task) || undefined })
      onGenerated()
      message.success('生成完成，已写入素材库')
    }
  }, [task, taskId, onGenerated])

  const maxPrompt = cap?.max_prompt_len
    || (type === 'image' ? 2000 : type === 'voice' ? 1000 : type === 'audio' ? 10000 : 5000)

  const imageNeedsRef = model === 'viduq1'
  const promptChips = type === 'image' ? IMAGE_PROMPT_CHIPS : type === 'video' ? VIDEO_PROMPT_CHIPS : []

  const busy = generating || (!!taskId && taskState !== 'success' && taskState !== 'failed' && taskState !== 'cancelled')
  const succeeded = taskState === 'success' && !!resultPreview

  const resetForAnother = () => {
    setTaskId(null)
    setResultPreview(null)
    doneNotified.current = false
  }

  const handleGenerate = async () => {
    let voiceCloneId: string | undefined
    if (type === 'voice') {
      if (!audioFile) { message.warning('请上传参考音频'); return }
      if (!text.trim()) { message.warning('请输入试听文本'); return }
      voiceCloneId = (voiceName.trim() || `voice_${Date.now().toString(36)}`).replace(/[^a-zA-Z0-9_-]/g, '_')
      if (!/^[a-zA-Z]/.test(voiceCloneId) || voiceCloneId.length < 8) {
        message.warning('音色 ID 需以英文字母开头，且至少 8 位')
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
    if (type === 'image' && imageNeedsRef && refImages.length === 0) {
      message.warning('当前模型需要至少 1 张参考图')
      return
    }
    if (!brandId) {
      message.warning('请先选择人设/品牌')
      return
    }

    setGenerating(true)
    doneNotified.current = false
    try {
      const effectiveModel = model || models[0]?.model
      let result: GenerationTask

      if (type === 'image') {
        result = await submitGenerationTaskCompat({
          brand_id: brandId,
          sub_type: 'text2image',
          model: effectiveModel || undefined,
          params: {
            prompt: text.trim(),
            ...(aspectRatio ? { aspect_ratio: aspectRatio } : {}),
          },
          refs: refImages.map((r, i) => ({
            id: r.id,
            url: r.url,
            kind: 'image' as const,
            name: r.url.split('/').pop() || `ref-${i + 1}`,
          })),
        })
      } else if (type === 'video') {
        result = await submitUnified({
          brand_id: brandId,
          text: text.trim(),
          type: 'video',
          duration: duration || undefined,
          quality: resolution || undefined,
          aspect_ratio: aspectRatio || undefined,
          params: mergeSubmitParams(effectiveModel ? { model: effectiveModel } : undefined),
        })
      } else if (type === 'audio') {
        result = await submitUnified({
          brand_id: brandId,
          text: text.trim(),
          type: 'audio',
          params: mergeSubmitParams(
            effectiveModel ? { model: effectiveModel } : undefined,
            voiceId ? { voice_setting_voice_id: voiceId } : undefined,
          ),
        })
      } else {
        const uploaded = await businessApi.uploadAsset(audioFile!)
        result = await submitUnified({
          brand_id: brandId,
          text: text.trim(),
          type: 'voice',
          materials: [uploaded.id],
          params: mergeSubmitParams({ voice_id: voiceCloneId }),
        })
      }

      setTaskId(result.id)
      if (result.state === 'success') {
        const url = taskPrimaryUrl(result)
        if (url) setResultPreview({ url, cover: taskCoverUrl(result) || undefined })
        doneNotified.current = true
        onGenerated()
        message.success('生成完成')
      } else {
        message.success('任务已提交，右侧可查看进度')
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : ''
      if (msg && !msg.includes('配额')) message.error(msg)
    } finally {
      setGenerating(false)
    }
  }

  const previewPanel = () => {
    if (succeeded && resultPreview) {
      return (
        <div className="gam-preview gam-preview--done">
          <div className="gam-preview-badge"><CheckCircleFilled /> 已入库</div>
          <div className="gam-preview-media">
            {type === 'image' && <ImageCover url={resultPreview.url} />}
            {type === 'video' && (
              <video
                src={resolveMediaUrl(resultPreview.url)}
                poster={resultPreview.cover ? resolveMediaUrl(resultPreview.cover) : undefined}
                controls
                playsInline
                className="gam-preview-video"
              />
            )}
            {type === 'audio' && (
              <audio src={resolveMediaUrl(resultPreview.url)} controls className="gam-preview-audio" />
            )}
          </div>
          <Text type="secondary" className="gam-preview-hint">可关闭弹窗，在列表中查看或用于创作</Text>
        </div>
      )
    }

    if (taskId && taskState === 'failed') {
      return (
        <div className="gam-preview gam-preview--idle">
          <GenerationTaskStatusBar
            taskId={taskId}
            fallbackPending={meta.title}
            onRetry={handleGenerate}
            onClearFailed={resetForAnother}
          />
        </div>
      )
    }

    if (taskId && busy) {
      return (
        <div className="gam-preview gam-preview--pending">
          <div className="gam-preview-spinner" aria-hidden />
          <GenerationTaskStatusBar taskId={taskId} fallbackPending="AI 生成中" />
          <Text type="secondary" style={{ fontSize: 12, textAlign: 'center' }}>
            通常需要几十秒到数分钟，可保持弹窗打开
          </Text>
        </div>
      )
    }

    return (
      <div className={`gam-preview gam-preview--idle gam-preview--${meta.tone}`}>
        <div className="gam-preview-icon">{meta.icon}</div>
        <Text strong style={{ fontSize: 15 }}>预览区</Text>
        <Text type="secondary" style={{ fontSize: 12, textAlign: 'center', lineHeight: 1.6 }}>
          {isVisual ? '填写左侧描述并点击「开始生成」，结果将在此预览' : '提交后在此查看状态与试听'}
        </Text>
      </div>
    )
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      width={isVisual ? MODAL_W.xxl : MODAL_W.lg}
      destroyOnHidden
      className="gam-modal"
      title={(
        <div className={`gam-header gam-header--${meta.tone}`}>
          <span className="gam-header-icon">{meta.icon}</span>
          <div>
            <div className="gam-header-title">{meta.title}</div>
            <div className="gam-header-lead">{meta.lead}</div>
          </div>
        </div>
      )}
      styles={{ body: { ...modalBodyScroll.body, paddingTop: 4 } }}
      footer={(
        <div className="gam-footer">
          <Button onClick={onClose}>{succeeded ? '完成' : '取消'}</Button>
          <Space>
            {succeeded && (
              <Button onClick={resetForAnother}>再生成一份</Button>
            )}
            <Button
              type="primary"
              className="ip-btn-primary"
              loading={generating}
              disabled={busy && !succeeded && taskState !== 'failed'}
              onClick={succeeded ? onClose : handleGenerate}
            >
              {generating ? '提交中…' : busy ? '生成中…' : taskState === 'failed' ? '重试' : succeeded ? '完成' : '开始生成'}
            </Button>
          </Space>
        </div>
      )}
    >
      {typesLoading ? (
        <Text type="secondary">加载模型能力…</Text>
      ) : models.length === 0 && (type === 'image' || type === 'video') ? (
        <Alert
          type="warning"
          showIcon
          message={`未开通${type === 'image' ? '文生图' : '文生视频'}`}
          description="请联系管理员在后台「生成规格」中启用对应端点与模型。"
        />
      ) : (
        <div className={isVisual ? 'gam-shell' : 'gam-shell gam-shell--single'}>
          <div className="gam-form">
            {promptChips.length > 0 && (
              <div className="gam-section">
                <div className="gam-section-label"><BulbOutlined /> 灵感快填</div>
                <div className="gam-chips">
                  {promptChips.map((chip) => (
                    <button
                      key={chip}
                      type="button"
                      className="gam-chip"
                      disabled={busy}
                      onClick={() => setText(chip)}
                    >
                      {chip}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {(type === 'image' || type === 'video' || type === 'audio') && models.length > 1 && (
              <div className="gam-section">
                <div className="gam-section-label">模型</div>
                <Select
                  style={{ width: '100%' }}
                  value={model || models[0]?.model}
                  onChange={setModel}
                  disabled={busy}
                  options={models.map((m) => ({
                    value: m.model,
                    label: `${m.model}${m.capability?.family ? ` · ${m.capability.family}` : ''}`,
                  }))}
                />
              </div>
            )}

            <div className="gam-section">
              <div className="gam-section-label">
                {type === 'audio' ? '合成文案' : type === 'voice' ? '试听文案' : '画面描述'}
              </div>
              <Input.TextArea
                className="gam-prompt"
                placeholder={
                  type === 'image' ? '例如：午市套餐平铺在木质餐桌上，自然光，竖版种草风'
                    : type === 'video' ? '例如：咖啡从拉花杯缓缓倒出，镜头缓慢推进'
                    : type === 'audio' ? '例如：欢迎光临，今日午市套餐限时优惠…'
                    : '用这段话试听克隆音色'
                }
                autoSize={{ minRows: 4, maxRows: 7 }}
                showCount
                maxLength={maxPrompt}
                value={text}
                onChange={(e) => setText(e.target.value)}
                disabled={busy}
              />
            </div>

            {type === 'video' && cap && (
              <div className="gam-section gam-params">
                <div className="gam-section-label">成片参数</div>
                <div className="gam-params-grid">
                  {(() => {
                    const [dMin, dMax] = cap.durations || [0, 0]
                    if (dMax <= 0) return null
                    if (dMin === dMax) {
                      return <Text type="secondary" style={{ fontSize: 12 }}>时长 {dMin} 秒</Text>
                    }
                    return (
                      <div className="gam-param">
                        <Text type="secondary" className="gam-param-label">时长</Text>
                        <InputNumber
                          min={dMin}
                          max={dMax}
                          value={duration}
                          onChange={(v) => setDuration(typeof v === 'number' ? v : dMin)}
                          disabled={busy}
                          addonAfter="秒"
                          style={{ width: '100%' }}
                        />
                      </div>
                    )
                  })()}
                  {(cap.resolutions?.length || 0) > 0 && (
                    <div className="gam-param gam-param--wide">
                      <Text type="secondary" className="gam-param-label">清晰度</Text>
                      <Segmented
                        block
                        value={resolution || cap.resolutions![0]}
                        onChange={(v) => setResolution(String(v))}
                        options={cap.resolutions!.map((r) => ({ value: r, label: r }))}
                        disabled={busy}
                      />
                    </div>
                  )}
                  {(cap.aspect_ratios?.length || 0) > 0 && (
                    <div className="gam-param gam-param--wide">
                      <Text type="secondary" className="gam-param-label">画面比例</Text>
                      <Segmented
                        block
                        value={aspectRatio || cap.aspect_ratios![0]}
                        onChange={(v) => setAspectRatio(String(v))}
                        options={cap.aspect_ratios!.map((r) => ({ value: r, label: r }))}
                        disabled={busy}
                      />
                    </div>
                  )}
                </div>
              </div>
            )}

            {type === 'image' && (cap?.aspect_ratios?.length || 0) > 0 && (
              <div className="gam-section">
                <div className="gam-section-label">画面比例</div>
                <Segmented
                  block
                  value={aspectRatio || cap!.aspect_ratios![0]}
                  onChange={(v) => setAspectRatio(String(v))}
                  options={cap!.aspect_ratios!.map((r) => ({ value: r, label: r }))}
                  disabled={busy}
                />
              </div>
            )}

            {type === 'image' && (
              <div className="gam-section">
                <div className="gam-ref-head">
                  <div className="gam-section-label" style={{ margin: 0 }}>
                    参考图{imageNeedsRef ? '（必填）' : '（可选）'}
                  </div>
                  <Space size={8} wrap>
                    <Upload
                      accept="image/png,image/jpeg,image/webp"
                      showUploadList={false}
                      multiple
                      disabled={busy || refImages.length >= 7}
                      beforeUpload={async (file) => {
                        if (refImages.length >= 7) {
                          message.warning('最多 7 张参考图')
                          return false
                        }
                        try {
                          const asset = await businessApi.uploadAsset(file)
                          setRefImages((prev) => [...prev, { id: asset.id, url: asset.url }].slice(0, 7))
                          message.success('已添加')
                        } catch { /* 拦截器 */ }
                        return false
                      }}
                    >
                      <Button size="small" icon={<UploadOutlined />} disabled={busy || refImages.length >= 7}>
                        上传
                      </Button>
                    </Upload>
                    <Button
                      size="small"
                      type="dashed"
                      icon={<PlusOutlined />}
                      disabled={busy || refImages.length >= 7}
                      onClick={() => setPickerOpen(true)}
                    >
                      素材库
                    </Button>
                  </Space>
                </div>
                <div
                  className={`gam-ref-zone ${refImages.length ? 'gam-ref-zone--filled' : ''}`}
                  onDragOver={(e) => e.preventDefault()}
                >
                  {refImages.length === 0 ? (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {imageNeedsRef ? '请上传或选择 1–7 张参考图' : '拖拽或点击上传；不选则为纯文生图'}
                    </Text>
                  ) : (
                    <div className="gam-ref-grid">
                      {refImages.map((img, i) => (
                        <div key={img.id + i} className="gam-ref-thumb">
                          <img src={img.url} alt="" />
                          <button
                            type="button"
                            className="gam-ref-remove"
                            disabled={busy}
                            onClick={() => setRefImages((prev) => prev.filter((_, j) => j !== i))}
                            aria-label="移除"
                          >
                            <DeleteOutlined />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}

            {type === 'audio' && (
              <div className="gam-section">
                <div className="gam-section-label">音色</div>
                <VoicePicker value={voiceId} onChange={setVoiceId} myVoices={myVoices} />
                <Text type="secondary" style={{ fontSize: 12, marginTop: 8, display: 'block' }}>
                  未选音色时使用系统默认
                </Text>
              </div>
            )}

            {type === 'voice' && (
              <>
                <div className="gam-section">
                  <div className="gam-section-label">音色 ID</div>
                  <Input
                    placeholder="如 shop_host_01（留空自动生成）"
                    value={voiceName}
                    onChange={(e) => setVoiceName(e.target.value)}
                    disabled={busy}
                  />
                </div>
                <div className="gam-section">
                  <div className="gam-section-label">参考人声</div>
                  <Upload
                    maxCount={1}
                    accept="audio/mp3,audio/wav,audio/m4a,audio/mpeg"
                    beforeUpload={(file) => { setAudioFile(file); return false }}
                    onRemove={() => setAudioFile(null)}
                    disabled={busy}
                    fileList={audioFile ? [{ uid: '-1', name: audioFile.name, status: 'done' }] : []}
                  >
                    <Button icon={<SoundOutlined />} block>
                      {audioFile ? '重新上传' : '上传 mp3 / wav / m4a（约 10 秒–5 分钟）'}
                    </Button>
                  </Upload>
                </div>
              </>
            )}
          </div>

          <div className="gam-aside">
            {previewPanel()}
          </div>
        </div>
      )}

      <AssetPicker
        open={pickerOpen}
        mode="multi"
        accept="image"
        title="选择参考图"
        max={7}
        onClose={() => setPickerOpen(false)}
        onSelect={(assets) => {
          setRefImages((prev) => [...prev, ...assets.map((a) => ({ id: a.id, url: a.url }))].slice(0, 7))
          setPickerOpen(false)
        }}
      />
    </Modal>
  )
}

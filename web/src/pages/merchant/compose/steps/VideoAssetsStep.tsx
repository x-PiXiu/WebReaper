import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Collapse, Input, Segmented, Select, Space, Upload } from 'antd'
import { PictureOutlined, SoundOutlined, UserOutlined } from '@ant-design/icons'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import {
  buildSubjectReferencePayload,
  deliverableWorkParams,
  ensureMaterialId,
  mergeSubmitParams,
  submitUnified,
} from '../../../../api/generationSubmit'
import { COVER_STYLES } from '../../../../data/coverStyles'
import { AssetPicker } from '../../../../components/compose/AssetPicker'
import { GenerationTaskStatusBar } from '../../../../components/compose/GenerationTaskStatusBar'
import { MediaResultCard } from '../../../../components/compose/MediaResultCard'
import { ManualUrlField } from '../../../../components/compose/ManualUrlField'
import { SubjectPicker } from '../../../../components/compose/SubjectPicker'
import { CapabilityBanner } from '../../../../components/wizard/CapabilityBanner'
import { useSubjectList } from '../../../../hooks/useSubjectList'
import { useGenerationTypes } from '../../../../hooks/useGenerationTypes'
import { catchGenerationError } from '../../../../utils/generationErrors'
import { toast } from '../../../../utils/feedback'

type AssetTab = 'voice' | 'avatar' | 'cover'

/** Step 2 发视频：配音 / 形象 / 封面（Tab 聚焦） */
export function VideoAssetsStep() {
  const navigate = useNavigate()
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [tab, setTab] = useState<AssetTab>('voice')
  const [busy, setBusy] = useState(false)
  const [ttsModel, setTtsModel] = useState<string>()
  const [avatarImage, setAvatarImage] = useState('')
  const [pickerOpen, setPickerOpen] = useState(false)
  const [subjectServerId, setSubjectServerId] = useState('')
  const [intent, setIntent] = useState('')
  const text = draft.rewritten || draft.script || ''

  const { isEnabled, types } = useGenerationTypes()
  const { ready: subjects } = useSubjectList()

  const ttsModels = useMemo(() => {
    const t = types.find((x) => x.sub_type === 'tts')
    return (t?.models || []).map((m) => m.model)
  }, [types])

  const selectedSubject = useMemo(
    () => subjects.find((s) => s.serverId === subjectServerId),
    [subjects, subjectServerId],
  )

  const runTts = async () => {
    if (!text.trim()) {
      toast.warn('请先填写口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      toast.warn('请先选择人设')
      return
    }
    if (!isEnabled('tts')) {
      toast.warn('语音合成暂未开通')
      return
    }
    setBusy(true)
    try {
      const res = await submitUnified({
        brand_id: bid,
        text,
        type: 'audio',
        params: mergeSubmitParams(ttsModel ? { model: ttsModel } : undefined),
      })
      draft.patch({ voiceTaskId: res.id, track: 'video', lastUpdatedAt: new Date().toISOString() })
      toast.ok('配音任务已提交')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  const runSubjectAvatar = async () => {
    if (!text.trim()) {
      toast.warn('请先填写口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      toast.warn('请先选择人设')
      return
    }
    if (!subjectServerId) {
      toast.warn('请选择数字分身')
      return
    }
    if (!isEnabled('reference2video')) {
      toast.warn('参考生视频暂未开通')
      return
    }
    setBusy(true)
    try {
      let audioMaterialId: string | undefined
      if (draft.voiceUrl) {
        audioMaterialId = await ensureMaterialId(draft.voiceUrl)
      }
      const scriptText = [intent.trim(), text.trim()].filter(Boolean).join('，')
      const res = await submitUnified(buildSubjectReferencePayload({
        brand_id: bid,
        server_id: subjectServerId,
        name: selectedSubject?.name,
        text: scriptText,
        audioMaterialId,
      }))
      draft.patch({ avatarTaskId: res.id, track: 'video', lastUpdatedAt: new Date().toISOString() })
      toast.ok('口播任务已提交')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  const runAvatarLegacy = async () => {
    if (!text.trim()) {
      toast.warn('请先填写口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      toast.warn('请先选择人设')
      return
    }
    if (!avatarImage) {
      toast.warn('请先上传或选择人像图')
      return
    }
    if (!draft.voiceUrl) {
      toast.warn('请先在「配音」生成音频')
      setTab('voice')
      return
    }
    setBusy(true)
    try {
      const imageId = await ensureMaterialId(avatarImage)
      const audioId = await ensureMaterialId(draft.voiceUrl)
      const res = await submitUnified({
        brand_id: bid,
        materials: [imageId, audioId],
        text: text.slice(0, 200),
        params: deliverableWorkParams(),
      })
      draft.patch({ avatarTaskId: res.id, track: 'video', lastUpdatedAt: new Date().toISOString() })
      toast.ok('口播任务已提交（图+音频）')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  const genCover = async () => {
    const bid = brandId || draft.brandId
    if (!bid) {
      toast.warn('请先选择人设')
      return
    }
    const title = draft.selectedTitle || '短视频封面'
    setBusy(true)
    try {
      const res = await submitUnified({
        brand_id: bid,
        type: 'image',
        text: `短视频封面，竖屏 9:16，大标题「${title}」，简洁醒目`,
        aspect_ratio: '9:16',
        params: mergeSubmitParams(),
      })
      draft.patch({ coverTaskId: res.id, track: 'video', lastUpdatedAt: new Date().toISOString() })
      toast.ok('封面任务已提交')
    } catch (e) {
      catchGenerationError(e)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="cf-panel cf-assets">
      <CapabilityBanner required={['tts', 'reference2video', 'lip_sync']} className="wz-draft-banner" />
      <Segmented
        className="cf-asset-tabs"
        value={tab}
        onChange={(v) => setTab(v as AssetTab)}
        options={[
          { label: '配音', value: 'voice', icon: <SoundOutlined /> },
          { label: '数字人', value: 'avatar', icon: <UserOutlined /> },
          { label: '封面', value: 'cover', icon: <PictureOutlined /> },
        ]}
      />

      {tab === 'voice' && (
        <section className="cf-asset-block">
          <GenerationTaskStatusBar
            taskId={draft.voiceTaskId}
            resultReady={!!draft.voiceUrl}
            doneLabel="配音已就绪"
            fallbackPending="配音"
            onClearFailed={() => draft.patch({ voiceTaskId: undefined })}
          />
          {draft.voiceUrl ? (
            <MediaResultCard
              kind="audio"
              url={draft.voiceUrl}
              onClear={() => draft.patch({ voiceUrl: undefined, voiceTaskId: undefined })}
            />
          ) : (
            <Space wrap style={{ width: '100%' }}>
              <Select
                style={{ minWidth: 200 }}
                placeholder="选择 TTS 模型"
                value={ttsModel || ttsModels[0]}
                onChange={setTtsModel}
                options={ttsModels.map((m) => ({ value: m, label: m }))}
                disabled={!isEnabled('tts')}
              />
              <Button
                type="primary"
                className="ip-btn-primary"
                loading={busy}
                onClick={runTts}
                disabled={!isEnabled('tts')}
              >
                生成配音
              </Button>
            </Space>
          )}
          <ManualUrlField
            value={draft.voiceUrl || ''}
            placeholder="配音 URL"
            onChange={(v) => draft.patch({ voiceUrl: v || undefined })}
          />
        </section>
      )}

      {tab === 'avatar' && (
        <section className="cf-asset-block">
          <GenerationTaskStatusBar
            taskId={draft.avatarTaskId}
            resultReady={!!draft.avatarVideoUrl}
            doneLabel="成片已就绪"
            fallbackPending="口播成片"
            onClearFailed={() => draft.patch({ avatarTaskId: undefined })}
          />
          {draft.avatarVideoUrl ? (
            <MediaResultCard
              kind="video"
              url={draft.avatarVideoUrl}
              label="数字人口播成片"
              onClear={() => draft.patch({ avatarVideoUrl: undefined, editedVideoUrl: undefined, avatarTaskId: undefined })}
            />
          ) : (
            <>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="推荐：数字分身 + 主体一致性（reference2video）"
                description="完整五步流程（音色、出镜方式、分段）请使用「拍口播」向导。"
                action={
                  <Button size="small" type="primary" onClick={() => navigate('/m/compose/lipsync')}>
                    打开拍口播
                  </Button>
                }
              />
              {!draft.voiceUrl && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="可选：先在「配音」Tab 生成配音，主体成片将附带音频"
                  action={<Button size="small" onClick={() => setTab('voice')}>去配音</Button>}
                />
              )}
              <SubjectPicker
                subjects={subjects}
                value={subjectServerId}
                onChange={setSubjectServerId}
                className="wz-subject-picks--block"
              />
              <Input
                placeholder="场景意图（可选，如：在厨房边做菜边对镜头讲解）"
                value={intent}
                onChange={(e) => setIntent(e.target.value)}
                maxLength={200}
                style={{ marginBottom: 12 }}
              />
              <Button
                type="primary"
                className="ip-btn-primary"
                loading={busy}
                disabled={!isEnabled('reference2video') || !subjectServerId}
                onClick={runSubjectAvatar}
              >
                提交主体一致性成片
              </Button>
              <Collapse
                ghost
                style={{ marginTop: 16 }}
                items={[{
                  key: 'legacy',
                  label: '降级：人像图 + 配音（无分身时）',
                  children: (
                    <>
                      <div className="cf-avatar-pick">
                        {avatarImage ? (
                          <div className="cf-media-thumb" style={{ backgroundImage: `url(${avatarImage})` }} />
                        ) : (
                          <div className="cf-media-thumb cf-media-thumb-empty">形象预览</div>
                        )}
                        <Space wrap>
                          <Upload
                            accept="image/*"
                            showUploadList={false}
                            beforeUpload={async (file) => {
                              try {
                                const asset = await businessApi.uploadAsset(file)
                                setAvatarImage(asset.url)
                                toast.ok('形象已上传')
                              } catch (e) {
                                catchGenerationError(e)
                              }
                              return false
                            }}
                          >
                            <Button size="small">上传形象</Button>
                          </Upload>
                          <Button size="small" onClick={() => setPickerOpen(true)}>素材库</Button>
                        </Space>
                      </div>
                      <Button
                        loading={busy}
                        style={{ marginTop: 12 }}
                        onClick={runAvatarLegacy}
                      >
                        提交降级口播
                      </Button>
                    </>
                  ),
                }]}
              />
            </>
          )}
        </section>
      )}

      {tab === 'cover' && (
        <section className="cf-asset-block">
          <GenerationTaskStatusBar
            taskId={draft.coverTaskId}
            resultReady={!!draft.coverUrl}
            doneLabel="封面已就绪"
            fallbackPending="封面"
            onClearFailed={() => draft.patch({ coverTaskId: undefined })}
          />
          {draft.coverUrl ? (
            <MediaResultCard
              kind="image"
              url={draft.coverUrl}
              label="视频封面"
              onClear={() => draft.patch({ coverUrl: undefined, coverTaskId: undefined })}
            />
          ) : (
            <>
              <div className="ip-pick-grid">
                {COVER_STYLES.map((c) => (
                  <button
                    key={c.id}
                    type="button"
                    className={`ip-pick-card${draft.coverAccent === c.accent ? ' is-active' : ''}`}
                    onClick={() => draft.patch({ coverAccent: c.accent })}
                  >
                    <span className="ip-pick-swatch" style={{ background: `linear-gradient(160deg,#111,${c.accent})` }} />
                    <strong>{c.name}</strong>
                  </button>
                ))}
              </div>
              <Button loading={busy} onClick={genCover} style={{ marginTop: 12 }}>
                AI 生成封面
              </Button>
            </>
          )}
          <ManualUrlField
            value={draft.coverUrl || ''}
            placeholder="封面图 URL"
            onChange={(v) => draft.patch({ coverUrl: v || undefined })}
          />
        </section>
      )}

      <AssetPicker
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        kind="image"
        title="选择数字人形象图"
        onPick={(url) => setAvatarImage(url)}
      />
    </div>
  )
}

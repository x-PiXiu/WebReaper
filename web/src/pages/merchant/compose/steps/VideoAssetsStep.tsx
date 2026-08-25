import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Alert, Button, Segmented, Select, Space, Upload, message } from 'antd'
import { PictureOutlined, SoundOutlined, UserOutlined } from '@ant-design/icons'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { ensureMaterialId, submitUnified } from '../../../../api/generationSubmit'
import { COVER_STYLES } from '../../../../data/coverStyles'
import { AssetPicker } from '../../../../components/compose/AssetPicker'
import { TaskStatusBar } from '../../../../components/compose/TaskStatusBar'
import { MediaResultCard } from '../../../../components/compose/MediaResultCard'
import { ManualUrlField } from '../../../../components/compose/ManualUrlField'

type AssetTab = 'voice' | 'avatar' | 'cover'

/** Step 2 发视频：配音 / 形象 / 封面（Tab 聚焦） */
export function VideoAssetsStep() {
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [tab, setTab] = useState<AssetTab>('voice')
  const [busy, setBusy] = useState(false)
  const [ttsModel, setTtsModel] = useState<string>()
  const [avatarImage, setAvatarImage] = useState('')
  const [pickerOpen, setPickerOpen] = useState(false)
  const text = draft.rewritten || draft.script || ''

  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
  })

  const ttsModels = useMemo(() => {
    const t = types.find((x) => x.sub_type === 'tts')
    return (t?.models || []).map((m) => m.model)
  }, [types])

  const runTts = async () => {
    if (!text.trim()) {
      message.warning('需要口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
      return
    }
    setBusy(true)
    try {
      const res = await submitUnified({
        brand_id: bid,
        text,
        type: 'audio',
      })
      draft.patch({ voiceTaskId: res.id, track: 'video', lastUpdatedAt: new Date().toISOString() })
      message.success('配音任务已提交，完成后自动填入')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  const runAvatar = async () => {
    if (!text.trim()) {
      message.warning('需要口播文案')
      return
    }
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
      return
    }
    if (!avatarImage) {
      message.warning('请先上传或选择人像图')
      return
    }
    if (!draft.voiceUrl) {
      message.warning('数字人口播需要配音——请先在「配音」Tab 生成')
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
      })
      draft.patch({ avatarTaskId: res.id, track: 'video', lastUpdatedAt: new Date().toISOString() })
      message.success('数字人口播任务已提交（图+音频 → digital_human）')
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : ''
      if (msg) message.error(msg)
    } finally {
      setBusy(false)
    }
  }

  const genCover = async () => {
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
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
      })
      draft.patch({ coverTaskId: res.id, track: 'video', lastUpdatedAt: new Date().toISOString() })
      message.success('封面生成任务已提交，完成后自动填入')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="cf-panel cf-assets">
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
          <TaskStatusBar
            pending={!!draft.voiceTaskId && !draft.voiceUrl}
            done={!!draft.voiceUrl}
            pendingLabel="配音生成中，完成后自动填入"
            doneLabel="配音已就绪"
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
              />
              <Button type="primary" className="ip-btn-primary" loading={busy} onClick={runTts}>
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
          <TaskStatusBar
            pending={!!draft.avatarTaskId && !draft.avatarVideoUrl}
            done={!!draft.avatarVideoUrl}
            pendingLabel="口播成片生成中，完成后自动填入"
            doneLabel="成片已就绪"
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
              {!draft.voiceUrl && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="数字人口播需要先有配音（图+音频 → digital_human）"
                  action={<Button size="small" onClick={() => setTab('voice')}>去配音</Button>}
                />
              )}
              <Space wrap style={{ width: '100%' }}>
                <Button type="primary" className="ip-btn-primary" loading={busy} onClick={runAvatar}>
                  提交口播成片
                </Button>
              </Space>
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
                        message.success('形象已上传')
                      } catch {
                        /* */
                      }
                      return false
                    }}
                  >
                    <Button size="small">上传形象</Button>
                  </Upload>
                  <Button size="small" onClick={() => setPickerOpen(true)}>素材库</Button>
                </Space>
              </div>
            </>
          )}
        </section>
      )}

      {tab === 'cover' && (
        <section className="cf-asset-block">
          <TaskStatusBar
            pending={!!draft.coverTaskId && !draft.coverUrl}
            done={!!draft.coverUrl}
            pendingLabel="封面生成中，完成后自动填入"
            doneLabel="封面已就绪"
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

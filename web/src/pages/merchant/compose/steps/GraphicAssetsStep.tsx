import { useMemo, useState } from 'react'
import { Button, Input, Segmented, Space, Upload } from 'antd'
import { PictureOutlined, PlusOutlined } from '@ant-design/icons'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { COVER_STYLES } from '../../../../data/coverStyles'
import { AssetPicker } from '../../../../components/compose/AssetPicker'
import { TaskStatusBar } from '../../../../components/compose/TaskStatusBar'
import { MediaResultCard } from '../../../../components/compose/MediaResultCard'
import { ManualUrlField } from '../../../../components/compose/ManualUrlField'
import { message } from '../../../../utils/antdApp'

type GraphicTab = 'images' | 'cover'

/** Step 2 发图文：配图 + 封面（Tab 聚焦） */
export function GraphicAssetsStep() {
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [tab, setTab] = useState<GraphicTab>('images')
  const [busy, setBusy] = useState(false)
  const [prompt, setPrompt] = useState(draft.selectedTitle || draft.refTitle || '')
  const [pickerOpen, setPickerOpen] = useState(false)
  const urls = draft.imageUrls || []

  const list = useMemo(() => urls.filter(Boolean), [urls])
  const pendingImages = (draft.imageTaskIds || []).length

  const gen = async () => {
    const bid = brandId || draft.brandId
    if (!bid) {
      message.warning('请先选择人设/品牌')
      return
    }
    setBusy(true)
    try {
      const res = await businessApi.submitGeneration({
        brand_id: bid,
        type: 'image',
        text: `小红书种草配图，竖版清爽，主题「${prompt || '产品种草'}」，真实场景感`,
        aspect_ratio: '9:16',
        params: undefined, // BE-GEN-01/03 已修
      })
      draft.patch({
        imageTaskIds: [...(draft.imageTaskIds || []), res.id],
        track: 'graphic',
        lastUpdatedAt: new Date().toISOString(),
      })
      message.success('配图任务已提交，完成后自动加入列表')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  const removeImage = (index: number) => {
    draft.patch({ imageUrls: list.filter((_, j) => j !== index) })
  }

  return (
    <div className="cf-panel cf-assets">
      <Segmented
        className="cf-asset-tabs"
        value={tab}
        onChange={(v) => setTab(v as GraphicTab)}
        options={[
          { label: `配图${list.length ? ` (${list.length})` : ''}`, value: 'images', icon: <PictureOutlined /> },
          { label: '封面', value: 'cover', icon: <PictureOutlined /> },
        ]}
      />

      {tab === 'images' && (
        <section className="cf-asset-block">
          <TaskStatusBar
            pending={pendingImages > 0}
            done={list.length > 0 && pendingImages === 0}
            pendingLabel={`${pendingImages} 张配图生成中…`}
            doneLabel={`已选 ${list.length} 张配图`}
          />
          <Input
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="配图主题提示词"
          />
          <Space wrap>
            <Button type="primary" className="ip-btn-primary" loading={busy} onClick={gen}>
              AI 生成配图
            </Button>
            <Upload
              accept="image/*"
              showUploadList={false}
              beforeUpload={async (file) => {
                setBusy(true)
                try {
                  const asset = await businessApi.uploadAsset(file)
                  draft.patch({ imageUrls: [...list, asset.url], track: 'graphic', lastUpdatedAt: new Date().toISOString() })
                  message.success('已加入配图')
                } catch {
                  /* */
                } finally {
                  setBusy(false)
                }
                return false
              }}
            >
              <Button icon={<PlusOutlined />} loading={busy}>上传</Button>
            </Upload>
            <Button onClick={() => setPickerOpen(true)}>素材库</Button>
          </Space>

          {list.length > 0 ? (
            <div className="cf-image-grid">
              {list.map((u, i) => (
                <div key={u + i} className="cf-image-grid-item">
                  <div className="cf-media-thumb" style={{ backgroundImage: `url(${u})` }} />
                  <Button type="text" size="small" danger onClick={() => removeImage(i)}>移除</Button>
                </div>
              ))}
            </div>
          ) : (
            <p className="cf-muted">上传、生成或从素材库选择配图</p>
          )}
        </section>
      )}

      {tab === 'cover' && (
        <section className="cf-asset-block">
          {draft.coverUrl ? (
            <MediaResultCard
              kind="image"
              url={draft.coverUrl}
              label="图文封面"
              onClear={() => draft.patch({ coverUrl: undefined })}
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
              {list[0] && (
                <Button type="link" onClick={() => draft.patch({ coverUrl: list[0] })}>
                  用第一张配图作封面
                </Button>
              )}
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
        title="选择配图"
        onPick={(url) => {
          draft.patch({ imageUrls: [...list, url], track: 'graphic', lastUpdatedAt: new Date().toISOString() })
          message.success('已加入配图')
        }}
      />
    </div>
  )
}

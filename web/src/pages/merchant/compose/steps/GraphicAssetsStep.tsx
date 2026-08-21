import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button, Input, Space, Upload, message } from 'antd'
import { PictureOutlined, PlusOutlined } from '@ant-design/icons'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'
import { businessApi } from '../../../../api/business'
import { MOCK_COVERS } from '../../../../mock/ipAssets'

/** Step 2 发图文：配图 + 封面 */
export function GraphicAssetsStep() {
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const [busy, setBusy] = useState(false)
  const [prompt, setPrompt] = useState(draft.selectedTitle || draft.refTitle || '')
  const urls = draft.imageUrls || []

  const { data: types = [] } = useQuery({
    queryKey: ['generation-types'],
    queryFn: () => businessApi.listGenerationTypes().then((r) => r.types),
  })
  const imgModel = types.find((t) => t.sub_type === 'text2image')?.models?.[0]?.model

  const list = useMemo(() => urls, [urls])

  const gen = async () => {
    if (!imgModel) {
      message.warning('暂无文生图模型')
      return
    }
    setBusy(true)
    try {
      await businessApi.submitGenerationTask({
        brand_id: brandId || draft.brandId,
        sub_type: 'text2image',
        model: imgModel,
        params: {
          prompt: `小红书种草配图，竖版清爽，主题「${prompt || '产品种草'}」，真实场景感`,
        },
      })
      message.success('配图任务已提交，完成后把 URL 填入下方')
    } catch {
      /* */
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="cf-panel cf-assets">
      <section className="cf-asset-block">
        <div className="cf-asset-title"><PictureOutlined /> 图文配图</div>
        <Input
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="配图主题提示词"
          style={{ marginBottom: 10 }}
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
                const { asset } = await businessApi.uploadAsset(file)
                draft.patch({ imageUrls: [...list, asset.url], track: 'graphic' })
                message.success('已加入配图')
              } catch {
                /* */
              } finally {
                setBusy(false)
              }
              return false
            }}
          >
            <Button icon={<PlusOutlined />} loading={busy}>上传配图</Button>
          </Upload>
        </Space>
        <div className="cf-url-list">
          {list.length === 0 && <p className="cf-muted">暂无配图，可上传或 AI 生成后回填 URL</p>}
          {list.map((u, i) => (
            <div key={i} className="cf-url-row">
              <Input
                value={u}
                onChange={(e) => {
                  const next = [...list]
                  next[i] = e.target.value
                  draft.patch({ imageUrls: next })
                }}
              />
              <Button
                type="text"
                danger
                onClick={() => draft.patch({ imageUrls: list.filter((_, j) => j !== i) })}
              >
                移除
              </Button>
            </div>
          ))}
          <Button type="dashed" block onClick={() => draft.patch({ imageUrls: [...list, ''] })}>
            手动添加一张
          </Button>
        </div>
      </section>

      <section className="cf-asset-block">
        <div className="cf-asset-title">图文封面风格</div>
        <div className="ip-pick-grid">
          {MOCK_COVERS.map((c) => (
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
        <Input
          style={{ marginTop: 10 }}
          placeholder="封面图 URL（可用首张配图）"
          value={draft.coverUrl || ''}
          onChange={(e) => draft.patch({ coverUrl: e.target.value })}
        />
        {list[0] && !draft.coverUrl && (
          <Button type="link" onClick={() => draft.patch({ coverUrl: list[0] })}>
            用第一张配图作封面
          </Button>
        )}
      </section>
    </div>
  )
}

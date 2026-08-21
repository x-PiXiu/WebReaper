import { Alert } from 'antd'
import { useComposeDraft } from '../../../../store/composeDraft'

/** Step 3 发图文：发布前核对 */
export function GraphicProduceStep() {
  const draft = useComposeDraft()
  const body = draft.rewritten || draft.script || ''
  const images = (draft.imageUrls || []).filter(Boolean)

  return (
    <div className="cf-panel">
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="右侧为笔记预览。确认后进入一键发布，内容类型将预选为图文。"
      />
      <div className="cf-summary">
        <div><label>标题</label><strong>{draft.selectedTitle || '（未填）'}</strong></div>
        <div><label>正文字数</label><strong>{body.replace(/\s/g, '').length}</strong></div>
        <div><label>配图</label><strong>{images.length} 张</strong></div>
        <div><label>封面</label><strong>{draft.coverUrl || images[0] ? '已准备' : '未配'}</strong></div>
      </div>
      <p className="cf-muted" style={{ marginTop: 16 }}>
        点击右下角「去发布图文」将带上标题与配图参数进入发布台。
      </p>
    </div>
  )
}

import { Alert, Button, Input, Space, message } from 'antd'
import { useComposeDraft } from '../../../../store/composeDraft'
import { useBrandContext } from '../../../../hooks/useBrands'

/** Step 3 发视频：确认成片并准备发布 */
export function VideoProduceStep() {
  const { brandId } = useBrandContext()
  const draft = useComposeDraft()
  const text = draft.rewritten || draft.script || ''

  return (
    <div className="cf-panel">
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="右侧预览将显示封面 / 成片。确认无误后去一键发布。"
      />
      <div className="cf-summary">
        <div><label>标题</label><strong>{draft.selectedTitle || '（未填）'}</strong></div>
        <div><label>口播字数</label><strong>{text.replace(/\s/g, '').length}</strong></div>
        <div><label>配音</label><strong>{draft.voiceUrl || draft.voiceTaskId ? '已准备' : '未配'}</strong></div>
        <div><label>数字人</label><strong>{draft.avatarVideoUrl || draft.avatarTaskId ? '已准备' : '未配'}</strong></div>
      </div>
      <Space direction="vertical" style={{ width: '100%', marginTop: 16 }} size={10}>
        <Input
          placeholder="成片视频 URL（可回填数字人结果）"
          value={draft.avatarVideoUrl || draft.editedVideoUrl || ''}
          onChange={(e) => draft.patch({ avatarVideoUrl: e.target.value, editedVideoUrl: e.target.value })}
        />
        <Input
          placeholder="封面 URL"
          value={draft.coverUrl || ''}
          onChange={(e) => draft.patch({ coverUrl: e.target.value })}
        />
        <Button
          onClick={() => message.success('成片信息已核对，可点击右下角去发布')}
        >
          核对成片信息
        </Button>
        <Button type="link" href="/m/compose/tools?tab=media" target="_blank">
          打开多媒体细调台取回产物 →
        </Button>
        {!brandId && !draft.brandId && (
          <Alert type="warning" showIcon message="发布前请先选择人设档案" />
        )}
      </Space>
    </div>
  )
}

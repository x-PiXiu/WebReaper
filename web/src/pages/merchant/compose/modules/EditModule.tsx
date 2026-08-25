import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Input, Space, Typography, message } from 'antd'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'

const { Text } = Typography

/**
 * 成片确认：智能剪辑引擎未接通前，作为「确认成片 URL → 封面/发布」的过渡步骤。
 * 口播向导/数字人产物会自动写入草稿；也可粘贴外部成片。
 */
export default function EditModule() {
  const navigate = useNavigate()
  const draft = useComposeDraft()
  const videoUrl = draft.editedVideoUrl || draft.avatarVideoUrl || ''

  useEffect(() => {
    draft.setTrack('video')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader
        title="成片确认"
        lead="确认口播/数字人成片地址，再去封面与发布——字幕卡点引擎接入后会替换本步"
        badge="发视频"
      />
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="智能剪辑（字幕/卡点）尚未接通"
        description="当前可确认或粘贴成片 URL；复杂多镜可先去多媒体工作台生成后再回到这里。"
      />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>成片 URL</Text>
        <Input
          style={{ marginTop: 8 }}
          placeholder="https://… 或本站 /media/ 地址"
          value={videoUrl}
          onChange={(e) => draft.patch({ editedVideoUrl: e.target.value })}
        />
        <Space style={{ marginTop: 16 }} wrap>
          <Button
            type="primary"
            className="ip-btn-primary"
            disabled={!videoUrl.trim()}
            onClick={() => {
              draft.patch({ editedVideoUrl: videoUrl.trim() })
              message.success('成片已确认')
              navigate('/m/compose/cover')
            }}
          >
            确认并去封面
          </Button>
          <Button
            disabled={!videoUrl.trim()}
            onClick={() => {
              const q = new URLSearchParams({ contentType: 'video' })
              q.set('mediaUrls', videoUrl.trim())
              if (draft.brandId) q.set('brandId', draft.brandId)
              if (draft.selectedTitle) q.set('title', draft.selectedTitle)
              if (draft.coverUrl) q.set('coverUrl', draft.coverUrl)
              navigate(`/m/distribution?${q.toString()}`)
            }}
          >
            直接去发布
          </Button>
          <Button type="link" onClick={() => navigate('/m/compose/tools?tab=media')}>打开多媒体工作台</Button>
        </Space>
      </div>
    </div>
  )
}

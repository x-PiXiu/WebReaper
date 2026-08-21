import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Input, Space, Typography, message } from 'antd'
import { ComposeModuleHeader } from '../ComposeModuleHeader'
import { useComposeDraft } from '../../../../store/composeDraft'

const { Text } = Typography

/** 智能剪辑：能力预留壳——发视频专属 */
export default function EditModule() {
  const navigate = useNavigate()
  const draft = useComposeDraft()

  useEffect(() => {
    draft.setTrack('video')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className="wr-page-content ip-page">
      <ComposeModuleHeader title="智能剪辑" lead="字幕、卡点与多镜组装——发视频专属，管线接入中" badge="发视频" />
      <Alert
        style={{ marginBottom: 16 }}
        type="warning"
        showIcon
        message="智能剪辑引擎尚未接通"
        description="可先粘贴已有成片 URL 写入草稿，或在多媒体工作台用多帧/图生视频做替代；完成后去封面与发布。"
      />
      <div className="wr-glass-card" style={{ padding: 24 }}>
        <Text strong>成片 URL（可选）</Text>
        <Input
          style={{ marginTop: 8 }}
          placeholder="https://…"
          value={draft.editedVideoUrl || draft.avatarVideoUrl || ''}
          onChange={(e) => draft.patch({ editedVideoUrl: e.target.value })}
        />
        <Space style={{ marginTop: 16 }} wrap>
          <Button
            type="primary"
            className="ip-btn-primary"
            onClick={() => {
              message.success('已保存成片地址到草稿')
              navigate('/m/compose/cover')
            }}
          >
            保存并去封面
          </Button>
          <Button type="link" onClick={() => navigate('/m/compose/tools?tab=media')}>打开多媒体工作台</Button>
        </Space>
      </div>
    </div>
  )
}

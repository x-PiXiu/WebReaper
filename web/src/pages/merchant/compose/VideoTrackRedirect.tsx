import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Spin, Typography } from 'antd'
import { useComposeDraft } from '../../../store/composeDraft'
import { getComposeBody } from '../../../utils/composeProgress'

const { Text } = Typography

/**
 * 发视频三步流已并入口播向导——旧链接自动跳转并尽量迁移草稿文案。
 */
export default function VideoTrackRedirect() {
  const navigate = useNavigate()
  const draft = useComposeDraft()

  useEffect(() => {
    const body = getComposeBody(draft)
    if (body && !(draft.wizardScript || '').trim()) {
      draft.patch({
        track: 'lipsync',
        wizardScript: body,
        wizardStep: 1,
      })
    } else if (draft.track === 'video') {
      draft.patch({ track: 'lipsync' })
    }
    navigate('/m/compose/lipsync', {
      replace: true,
      state: { fromVideoTrack: true },
    })
  }, [draft, navigate])

  return (
    <div className="wr-page-content" style={{ display: 'grid', placeItems: 'center', minHeight: 320 }}>
      <Spin tip="正在进入口播向导…" />
      <Text type="secondary" style={{ marginTop: 12, fontSize: 12 }}>发视频已升级为五步口播向导</Text>
    </div>
  )
}

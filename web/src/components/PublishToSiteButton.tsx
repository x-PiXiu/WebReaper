import { Button, Popconfirm, Tooltip } from 'antd'
import { publishGate } from '../hooks/usePublishGate'

// 「发布到公开站」统一按钮：评分门槛 + 确认弹窗 + 按钮形态全站一致。
// 规则见 hooks/usePublishGate（业务规则不写在 UI 里）。
export default function PublishToSiteButton({
  score,
  size = 'small',
  onPublish,
}: {
  score?: number
  size?: 'small' | 'middle' | 'large'
  onPublish: () => void
}) {
  const gate = publishGate(score)

  if (gate.blocked) {
    return (
      <Tooltip title={gate.hint}>
        <Button size={size} disabled>发布（需先优化）</Button>
      </Tooltip>
    )
  }
  if (gate.needConfirm) {
    return (
      <Popconfirm title={gate.hint} onConfirm={onPublish}>
        <Button
          size={size}
          type="primary"
          ghost
          style={{ borderColor: 'var(--wr-warning)', color: 'var(--wr-warning)' }}
        >
          发布到公开站
        </Button>
      </Popconfirm>
    )
  }
  return (
    <Button size={size} type="primary" ghost onClick={onPublish}>
      发布到公开站
    </Button>
  )
}

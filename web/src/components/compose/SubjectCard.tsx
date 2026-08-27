import type { ReactNode } from 'react'
import { Tag, Typography } from 'antd'
import { UserOutlined } from '@ant-design/icons'
import type { ViduSubject } from '../../utils/subjectTask'

const { Text } = Typography

export function subjectStateMeta(state: string) {
  if (state === 'success') return { color: 'green' as const, label: '就绪' }
  if (state === 'failed') return { color: 'red' as const, label: '失败' }
  return { color: 'processing' as const, label: '创建中' }
}

type BaseProps = {
  subject: ViduSubject
  active?: boolean
  highlight?: boolean
  onClick?: () => void
  className?: string
}

/** 选择器内紧凑卡片（按钮形态） */
export function SubjectPickCard({ subject, active, highlight, onClick, className }: BaseProps) {
  return (
    <button
      type="button"
      className={[
        'wz-subject-card',
        active ? 'is-active' : '',
        highlight ? 'is-highlight' : '',
        className,
      ].filter(Boolean).join(' ')}
      onClick={onClick}
      title={subject.serverId ? `ID: ${subject.serverId}` : subject.name}
    >
      <SubjectThumb subject={subject} />
      <span className="wz-subject-card-meta">
        <strong className="wz-subject-card-name">{subject.name}</strong>
        {subject.hasVideo && (
          <Tag color="processing" className="wz-subject-card-tag">视频分身</Tag>
        )}
      </span>
    </button>
  )
}

type GridProps = {
  subject: ViduSubject
  timeLabel?: string
  footer?: ReactNode
}

/** 资产库网格卡片 */
export function SubjectGridCard({ subject, timeLabel, footer }: GridProps) {
  const st = subjectStateMeta(subject.state)
  return (
    <div className="ip-asset-card wr-subject-grid-card">
      <div
        className="ip-asset-cover ip-asset-cover--voice wr-subject-grid-cover"
        style={
          subject.portraitUrl
            ? { backgroundImage: `url(${subject.portraitUrl})` }
            : undefined
        }
      >
        {!subject.portraitUrl && <UserOutlined className="wr-subject-grid-placeholder" />}
      </div>
      <div className="ip-asset-body">
        <Text strong className="wr-subject-grid-name">{subject.name}</Text>
        <div className="wr-subject-grid-tags">
          <Tag color={st.color}>{st.label}</Tag>
          {subject.state === 'success' && subject.serverId && (
            <Tag color="success">已注册 ✓</Tag>
          )}
          <Tag color={subject.kind === 'scene' ? 'cyan' : undefined}>
            {subject.kind === 'scene'
              ? '场景'
              : subject.hasVideo
                ? `视频主体${subject.imageCount > 0 ? '（仅视频生效）' : ''}`
                : `${subject.imageCount} 张图`}
          </Tag>
          {subject.voiceId && <Tag color="purple">音色 {subject.voiceId}</Tag>}
          {timeLabel && <span className="wr-subject-grid-time">{timeLabel}</span>}
        </div>
        {subject.state === 'failed' && subject.errMsg && (
          <Text type="danger" className="wr-subject-grid-err" ellipsis={{ tooltip: subject.errMsg }}>
            {subject.errMsg}
          </Text>
        )}
        {subject.state === 'success' && subject.serverId && (
          <Text
            type="secondary"
            className="wr-subject-grid-id"
            copyable={{ text: subject.serverId, tooltips: ['复制 server_id', '已复制'] }}
          >
            {subject.serverId.slice(0, 16)}{subject.serverId.length > 16 ? '…' : ''}
          </Text>
        )}
        {footer}
      </div>
    </div>
  )
}

function SubjectThumb({ subject }: { subject: ViduSubject }) {
  return (
    <span
      className="wz-subject-card-thumb"
      style={subject.portraitUrl ? { backgroundImage: `url(${subject.portraitUrl})` } : undefined}
    >
      {!subject.portraitUrl && <UserOutlined />}
    </span>
  )
}

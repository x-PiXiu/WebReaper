import { Button, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ViduSubject } from '../../utils/subjectTask'
import { SubjectPickCard } from './SubjectCard'

const { Text } = Typography

type Props = {
  subjects: ViduSubject[]
  value?: string
  onChange: (serverId: string) => void
  highlightServerId?: string
  createHref?: string
  className?: string
  emptyHint?: string
}

export function SubjectPicker({
  subjects,
  value,
  onChange,
  highlightServerId,
  createHref = '/m/assets?create=subject',
  className,
  emptyHint = '还没有可用的数字分身',
}: Props) {
  const navigate = useNavigate()
  const flashId = highlightServerId && highlightServerId !== value ? highlightServerId : undefined

  if (subjects.length === 0) {
    return (
      <div className="wz-subject-empty">
        <Text type="secondary">{emptyHint}</Text>
        <Button
          type="primary"
          size="small"
          icon={<PlusOutlined />}
          onClick={() => navigate(createHref)}
        >
          去创建数字分身
        </Button>
      </div>
    )
  }

  return (
    <div className={['wz-subject-picks', 'wz-subject-picks--cards', className].filter(Boolean).join(' ')}>
      {subjects.map((s) => (
        <SubjectPickCard
          key={s.taskId}
          subject={s}
          active={value === s.serverId}
          highlight={flashId === s.serverId}
          onClick={() => onChange(s.serverId)}
        />
      ))}
    </div>
  )
}

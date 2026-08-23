import { useNavigate } from 'react-router-dom'
import { Button, Space } from 'antd'
import {
  VideoCameraOutlined, ThunderboltOutlined, FireOutlined, SendOutlined,
} from '@ant-design/icons'

/** 获客管家底部快捷动作（引导至口播向导 / 快速生成等） */
export function ChatQuickActions({ disabled }: { disabled?: boolean }) {
  const navigate = useNavigate()
  return (
    <Space wrap size={6} className="chat-quick-actions">
      <Button size="small" icon={<VideoCameraOutlined />} disabled={disabled} onClick={() => navigate('/m/compose/lipsync')}>
        拍口播
      </Button>
      <Button size="small" icon={<ThunderboltOutlined />} disabled={disabled} onClick={() => navigate('/m/compose/quick')}>
        快速生成
      </Button>
      <Button size="small" icon={<FireOutlined />} disabled={disabled} onClick={() => navigate('/m/inspire')}>
        找爆款
      </Button>
      <Button size="small" icon={<SendOutlined />} disabled={disabled} onClick={() => navigate('/m/distribution')}>
        去发布
      </Button>
    </Space>
  )
}

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Typography } from 'antd'
import { BulbOutlined, DownOutlined, UpOutlined, VideoCameraOutlined } from '@ant-design/icons'

const { Paragraph, Text } = Typography

/** 增长顾问工具结果：可展开的诊断摘要 + 一键拍口播 */
export function AdvisorResultCard({ raw }: { raw: string }) {
  const navigate = useNavigate()
  const [expanded, setExpanded] = useState(false)
  const preview = raw.slice(0, 280)
  const long = raw.length > 280

  return (
    <div className="chat-advisor-card">
      <div className="chat-advisor-head">
        <Text strong><BulbOutlined /> 增长诊断</Text>
        {long && (
          <Button type="link" size="small" onClick={() => setExpanded(!expanded)}>
            {expanded ? '收起' : '展开全文'} {expanded ? <UpOutlined /> : <DownOutlined />}
          </Button>
        )}
      </div>
      <Paragraph className="chat-advisor-body">
        {expanded || !long ? raw : `${preview}…`}
      </Paragraph>
      <Button
        size="small"
        type="primary"
        icon={<VideoCameraOutlined />}
        style={{ marginTop: 8 }}
        onClick={() => navigate('/m/compose/lipsync', { state: { rawText: raw.slice(0, 800), method: 'advisor' } })}
      >
        按建议拍口播
      </Button>
    </div>
  )
}

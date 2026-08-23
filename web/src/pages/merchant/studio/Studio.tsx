import { useNavigate, useSearchParams } from 'react-router-dom'
import { Tabs, Alert, Button, Space } from 'antd'
import { EditOutlined, VideoCameraOutlined, ThunderboltOutlined } from '@ant-design/icons'
import Content from '../Content'
import CreationWorkbench from '../Creation'
import { ComposeModuleHeader } from '../compose/ComposeModuleHeader'

const TAB_KEYS = new Set(['media', 'article'])

/**
 * 多媒体 / 写文章工具台（高级）：从内容合成模块跳转细调生成参数。
 */
export default function Studio() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const raw = searchParams.get('tab') || 'media'
  const topicParam = searchParams.get('topic') || ''
  const refTitle = searchParams.get('refTitle') || ''
  const initialPrompt = topicParam
    ? `${topicParam}${refTitle ? `（参考爆款思路：「${refTitle}」，拍出同类获客效果）` : ''}`
    : ''
  const activeTab = TAB_KEYS.has(raw) ? raw : 'media'

  return (
    <div className="wr-page-content ip-page" style={{ paddingTop: 4 }}>
      <ComposeModuleHeader
        title="生成工具台"
        lead="专业参数面板：单步调试各生成端点。日常口播推荐用向导或快速生成。"
      />
      {activeTab === 'media' && (
        <Alert
          type="info" showIcon style={{ marginBottom: 12 }}
          message="做口播成片？"
          description={
            <Space wrap>
              <Button size="small" type="primary" icon={<ThunderboltOutlined />} onClick={() => navigate('/m/compose/lipsync')}>
                口播向导
              </Button>
              <Button size="small" onClick={() => navigate('/m/compose/quick')}>快速生成</Button>
            </Space>
          }
        />
      )}
      <Tabs
        activeKey={activeTab}
        onChange={(k) => {
          const next = new URLSearchParams(searchParams)
          next.set('tab', k)
          setSearchParams(next, { replace: true })
        }}
        items={[
          {
            key: 'media',
            label: <span><VideoCameraOutlined /> 做视频图片</span>,
            children: <CreationWorkbench embedded initialPrompt={initialPrompt} />,
          },
          {
            key: 'article',
            label: <span><EditOutlined /> 写文章</span>,
            children: <Content embedded />,
          },
        ]}
      />
    </div>
  )
}

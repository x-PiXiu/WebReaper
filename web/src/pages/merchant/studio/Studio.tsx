import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import { EditOutlined, VideoCameraOutlined } from '@ant-design/icons'
import Content from '../Content'
import CreationWorkbench from '../Creation'
import { ComposeModuleHeader } from '../compose/ComposeModuleHeader'

const TAB_KEYS = new Set(['media', 'article'])

/**
 * 多媒体 / 写文章工具台（高级）：从内容合成模块跳转细调生成参数。
 */
export default function Studio() {
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
        lead="写文章与做视频图片的完整参数面板——供配音/数字人等模块细调"
      />
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

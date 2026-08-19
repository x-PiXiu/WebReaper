import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import { EditOutlined, VideoCameraOutlined } from '@ant-design/icons'
import Content from '../Content'
import CreationWorkbench from '../Creation'

/**
 * 内容中心（四步主线第 3 步"造内容"）：写文章 / 做视频图片 两种形态合一。
 * 共享全局品牌上下文；两个子页保持各自状态（antd Tabs 已挂载面板不销毁）。
 * Tab 用 searchParams 持久化（?tab=media 深链）。
 */
export default function Studio() {
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = searchParams.get('tab') === 'media' ? 'media' : 'article'

  return (
    <div className="wr-page-content" style={{ paddingTop: 4 }}>
      <div className="wr-page-header" style={{ marginBottom: 8 }}>
        <h1>内容中心</h1>
        <p>让 AI 帮你写文章、做视频——内容发布得越多，AI 越有东西可以引用你</p>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={(k) => setSearchParams({ tab: k }, { replace: true })}
        items={[
          {
            key: 'article',
            label: <span><EditOutlined /> 写文章</span>,
            children: <Content embedded />,
          },
          {
            key: 'media',
            label: <span><VideoCameraOutlined /> 做视频图片</span>,
            children: <CreationWorkbench embedded />,
          },
        ]}
      />
    </div>
  )
}

import { useSearchParams } from 'react-router-dom'
import { Tabs, Tag } from 'antd'
import { EditOutlined, VideoCameraOutlined, RocketOutlined } from '@ant-design/icons'
import Content from '../Content'
import CreationWorkbench from '../Creation'
import ComposeWizard from '../compose/ComposeWizard'

const TAB_KEYS = new Set(['media', 'article', 'wizard'])

/**
 * 内容合成中心：写文章 / 做视频图片（真实 API）+ 成片向导（演示）。
 * Tab 用 searchParams 持久化（?tab=media|article|wizard）。
 */
export default function Studio() {
  const [searchParams, setSearchParams] = useSearchParams()
  const raw = searchParams.get('tab') || 'media'
  // 热门同款「拍摄同款」跳转预填：topic=选题文案，refTitle=参考爆款标题（拼进提示词供 AI 参考）
  const topicParam = searchParams.get('topic') || ''
  const refTitle = searchParams.get('refTitle') || ''
  const initialPrompt = topicParam
    ? `${topicParam}${refTitle ? `（参考爆款思路：「${refTitle}」，拍出同类获客效果）` : ''}`
    : ''
  const activeTab = TAB_KEYS.has(raw) ? raw : 'media'

  return (
    <div className="wr-page-content ip-page" style={{ paddingTop: 4 }}>
      <div className="ip-page-hero" style={{ marginBottom: 12 }}>
        <div>
          <p className="ip-kicker">Compose</p>
          <h1>内容合成</h1>
          <p className="ip-lead">写文章 / 做视频图片，生成后去发布中心分发</p>
        </div>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={(k) => setSearchParams({ tab: k }, { replace: true })}
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
          {
            key: 'wizard',
            label: (
              <span>
                <RocketOutlined /> 成片向导{' '}
                <Tag style={{ marginInlineStart: 4, fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>演示</Tag>
              </span>
            ),
            children: <ComposeWizard embedded />,
          },
        ]}
      />
    </div>
  )
}

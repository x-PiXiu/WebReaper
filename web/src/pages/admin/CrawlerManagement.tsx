import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import CrawlerAccounts from './CrawlerAccounts'
import CrawlerConfigs from './CrawlerConfigs'
import CrawlerTasks from './CrawlerTasks'
import AdminInspirations from './Inspirations'

/**
 * 爬虫管理（复合页）：
 * - 平台方账号：各平台爬虫登录态管理
 * - 爬虫配置：平台/关键词/频率
 * - 任务监控：实时执行状态
 * - 灵感运营：热门视频采集审核
 */
function CrawlerManagement() {
  const [searchParams] = useSearchParams()
  const [tab, setTab] = useState(searchParams.get('tab') || 'accounts')
  return (
    <div className="wr-page-content ip-page">
      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          { key: 'accounts', label: '平台方账号', children: <CrawlerAccounts embedded /> },
          { key: 'configs', label: '爬虫配置', children: <CrawlerConfigs embedded /> },
          { key: 'tasks', label: '任务监控', children: <CrawlerTasks embedded /> },
          { key: 'inspirations', label: '灵感运营', children: <AdminInspirations embedded /> },
        ]}
      />
    </div>
  )
}

export default CrawlerManagement

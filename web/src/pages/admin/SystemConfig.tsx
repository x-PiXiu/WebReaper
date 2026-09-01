import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import AdminSettings from './Settings'
import Integrations from './Integrations'
import TaskMonitor from './TaskMonitor'

/**
 * 系统配置（复合页）：
 * - 平台设置：运行时开关（自动监测/浏览器模式/链式形象视频等 gen_* 配置）
 * - 第三方集成：生成厂商/ASR/LLM/搜索 能力路由与凭据管理
 * - 任务监控：跨租户生成任务实时监控/取消
 */
function SystemConfig() {
  const [searchParams] = useSearchParams()
  const [tab, setTab] = useState(searchParams.get('tab') || 'settings')
  return (
    <div className="wr-page-content ip-page">
      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          { key: 'settings', label: '平台设置', children: <AdminSettings embedded /> },
          { key: 'integrations', label: '第三方集成', children: <Integrations embedded /> },
          { key: 'monitor', label: '任务监控', children: <TaskMonitor embedded /> },
        ]}
      />
    </div>
  )
}

export default SystemConfig

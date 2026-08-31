import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import AdminSubjects from './Subjects'
import AdminVoices from './Voices'

/**
 * 平台资产管理（复合页——减少菜单项，与商户端风格统一）：
 * - 官方主体：管理后台上传形象照 → Vidu 注册 + 链式形象视频 → 用户端即选即用
 * - 官方音色：从 Vidu 克隆 / 上传样本克隆 → 设为平台默认 → 用户端白牌化展示
 */
function PlatformAssets() {
  const [searchParams] = useSearchParams()
  const [tab, setTab] = useState(searchParams.get('tab') || 'subjects')
  return (
    <div className="wr-page-content ip-page">
      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          {
            key: 'subjects',
            label: '官方主体',
            children: <AdminSubjects embedded />,
          },
          {
            key: 'voices',
            label: '官方音色',
            children: <AdminVoices embedded />,
          },
        ]}
      />
    </div>
  )
}

export default PlatformAssets

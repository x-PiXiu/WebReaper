import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import AdminUsers from './Users'
import AdminBrands from './Brands'
import AdminContents from './Contents'
import AdminWorks from './Works'

/**
 * 商户与品牌管理（复合页）：
 * - 商户列表：创建/删除商户
 * - 品牌管理：查看/删除品牌
 * - 内容管理：查看/删除优化内容
 * - 作品管理：成片巡查流 + 下架/恢复（32号内容安全）
 */
function TenantManagement() {
  const [searchParams] = useSearchParams()
  const [tab, setTab] = useState(searchParams.get('tab') || 'users')
  return (
    <div className="wr-page-content ip-page">
      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          { key: 'users', label: '商户列表', children: <AdminUsers embedded /> },
          { key: 'brands', label: '品牌管理', children: <AdminBrands embedded /> },
          { key: 'contents', label: '内容管理', children: <AdminContents embedded /> },
          { key: 'works', label: '作品管理', children: <AdminWorks embedded /> },
        ]}
      />
    </div>
  )
}

export default TenantManagement

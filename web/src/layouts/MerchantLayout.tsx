import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell, type NavItem } from './MainLayout'
import { businessApi } from '../api/business'
import type { Brand } from '../types/api'
import {
  DashboardOutlined,
  AppstoreOutlined,
  SearchOutlined,
  EditOutlined,
  ExportOutlined,
  MessageOutlined,
  CrownOutlined,
  EnvironmentOutlined,
  BellOutlined,
  FundOutlined,
  VideoCameraOutlined,
} from '@ant-design/icons'

// 商户端导航：按 GEO 闭环流程分组（对齐摘星五部曲：立身份→建资产→发全域→盯数据）。
// 设计动机：此前 8 个功能页平行铺开，用户不知道"先做什么、后做什么"——
// 按阶段组织后，菜单本身就是工作流：从上到下即执行顺序，每阶段 1-3 个页面。
const baseMenu: NavItem[] = [
  {
    key: 'overview', label: '总览',
    children: [
      { key: '/m/dashboard', label: '工作台', icon: <DashboardOutlined /> },
    ],
  },
  {
    key: 'identity', label: '① 立身份',
    children: [
      { key: '/m/brands', label: '品牌管理', icon: <AppstoreOutlined /> },
      { key: '/m/nearby', label: '附近同行', icon: <EnvironmentOutlined /> },
    ],
  },
  {
    key: 'asset', label: '② 建资产',
    children: [
      { key: '/m/keywords', label: '关键词工程', icon: <SearchOutlined /> },
      { key: '/m/content', label: '内容生成', icon: <EditOutlined /> },
      { key: '/m/creation', label: '多媒体创作', icon: <VideoCameraOutlined /> },
    ],
  },
  {
    key: 'distribute', label: '③ 发全域',
    children: [
      { key: '/m/distribution', label: '社媒分发', icon: <ExportOutlined /> },
    ],
  },
  {
    key: 'monitor', label: '④ 盯数据',
    children: [
      { key: '/m/indexing-report', label: 'AI 可见度', icon: <FundOutlined /> },
    ],
  },
  {
    key: 'ops', label: '运营',
    children: [
      { key: '/m/notifications', label: '通知中心', icon: <BellOutlined /> },
      { key: '/m/chat', label: 'AI 对话', icon: <MessageOutlined /> },
      { key: '/m/my-plan', label: '我的套餐', icon: <CrownOutlined /> },
    ],
  },
]

export default function MerchantLayout() {
  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const menu = useMemo(() => {
    if (brands.length === 0) return baseMenu
    const hasLocal = brands.some((b: Brand) => b.biz_type !== 'online')
    if (hasLocal) return baseMenu
    // 全部线上品牌：隐藏「附近同行」（门店/附近功能不适用）
    return baseMenu.map(group => {
      if (group.key === 'identity' && group.children) {
        return { ...group, children: group.children.filter(item => item.key !== '/m/nearby') }
      }
      return group
    })
  }, [brands])

  return <AppShell menuItems={menu} brandName="智擎AI" brandIcon="智" noPaddingKeys={['/m/chat']} />
}

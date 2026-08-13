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
  RadarChartOutlined,
  EnvironmentOutlined,
  BellOutlined,
  FundOutlined,
} from '@ant-design/icons'

// 商户端导航：对齐「工作台 + 内容模块」信息架构。
// 内容生成合并原「内容工作台 / 创作工作台」入口（多媒体创作从内容页进入，路由仍保留）。
const baseMenu: NavItem[] = [
  {
    key: 'biz', label: '业务',
    children: [
      { key: '/m', label: '工作台', icon: <DashboardOutlined /> },
      { key: '/m/keywords', label: '关键词工程', icon: <SearchOutlined /> },
      { key: '/m/brands', label: '品牌知识库', icon: <AppstoreOutlined /> },
      { key: '/m/content', label: '内容生成', icon: <EditOutlined /> },
      { key: '/m/distribution', label: '自动发布', icon: <ExportOutlined /> },
      { key: '/m/visibility', label: '可见度报表', icon: <RadarChartOutlined /> },
      { key: '/m/indexing-report', label: '平台收录报表', icon: <FundOutlined /> },
      { key: '/m/nearby', label: '附近同行', icon: <EnvironmentOutlined /> },
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
    return baseMenu.map(group => {
      if (group.key === 'biz' && group.children) {
        return { ...group, children: group.children.filter(item => item.key !== '/m/nearby') }
      }
      return group
    })
  }, [brands])

  return <AppShell menuItems={menu} brandName="智擎AI" brandIcon="智" noPaddingKeys={['/m/chat']} />
}

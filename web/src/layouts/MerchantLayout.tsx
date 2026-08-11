import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell, type NavItem } from './MainLayout'
import { businessApi } from '../api/business'
import {
  DashboardOutlined,
  AppstoreOutlined,
  SearchOutlined,
  EditOutlined,
  VideoCameraOutlined,
  ExportOutlined,
  MessageOutlined,
  CrownOutlined,
  RadarChartOutlined,
  EnvironmentOutlined,
} from '@ant-design/icons'

// 商户端布局：分组导航（资产 / 可见度 / 创作 / 运营）。
// P0-3：菜单动态化——纯 online 品牌（线上业务）隐藏"附近同行"（无地理约束）。
const baseMenu: NavItem[] = [
  {
    key: 'assets', label: '品牌资产',
    children: [
      { key: '/m', label: '数据驾驶舱', icon: <DashboardOutlined /> },
      { key: '/m/brands', label: '品牌管理', icon: <AppstoreOutlined /> },
      { key: '/m/keywords', label: '关键词管理', icon: <SearchOutlined /> },
    ],
  },
  {
    key: 'visibility', label: 'AI 可见度',
    children: [
      { key: '/m/visibility', label: '可见度总览', icon: <RadarChartOutlined /> },
      { key: '/m/nearby', label: '附近同行', icon: <EnvironmentOutlined /> },
    ],
  },
  {
    key: 'creation', label: '创作',
    children: [
      { key: '/m/content', label: '内容工作台', icon: <EditOutlined /> },
      { key: '/m/video', label: '视频工作台', icon: <VideoCameraOutlined /> },
    ],
  },
  {
    key: 'operation', label: '运营',
    children: [
      { key: '/m/distribution', label: '分发中心', icon: <ExportOutlined /> },
      { key: '/m/chat', label: 'AI 对话', icon: <MessageOutlined /> },
      { key: '/m/my-plan', label: '我的套餐', icon: <CrownOutlined /> },
    ],
  },
]

// 商户端布局。
export default function MerchantLayout() {
  // 查询品牌列表——用于判断是否纯 online（全线上 → 隐藏附近同行菜单）
  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
  })

  const menu = useMemo(() => {
    if (brands.length === 0) return baseMenu // 无品牌时显示全部（注册初期引导）
    const hasLocal = brands.some((b: any) => b.biz_type !== 'online')
    if (hasLocal) return baseMenu // 有本地品牌 → 保留附近同行
    // 全 online：过滤掉附近同行菜单项
    return baseMenu.map(group => {
      if (group.key === 'visibility' && group.children) {
        return { ...group, children: group.children.filter(item => item.key !== '/m/nearby') }
      }
      return group
    })
  }, [brands])

  return <AppShell menuItems={menu} brandName="WebReaper" brandIcon="W" noPaddingKeys={['/m/chat']} />
}

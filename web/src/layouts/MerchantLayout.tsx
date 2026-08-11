import { AppShell, type NavItem } from './MainLayout'
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
// 设计原则：按业务域分组，每组 ≥2 项（避免单项目组——原 概览/分发/AI助手/账户 各 1 项过碎）。
const merchantMenu: NavItem[] = [
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
  return <AppShell menuItems={merchantMenu} brandName="WebReaper" brandIcon="W" noPaddingKeys={['/m/chat']} />
}

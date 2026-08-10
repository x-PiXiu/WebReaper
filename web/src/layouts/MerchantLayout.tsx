import { AppShell, type NavItem } from './MainLayout'
import {
  DashboardOutlined,
  AppstoreOutlined,
  SearchOutlined,
  EditOutlined,
  VideoCameraOutlined,
  ExportOutlined,
  MessageOutlined,
} from '@ant-design/icons'

// 商户端布局：分组导航（概览 / 资产 / 创作 / 分发 / AI 助手）。
// AntD Menu group 分组渲染，选中态 Linear 化（CSS 覆盖）。
const merchantMenu: NavItem[] = [
  {
    key: 'overview', label: '概览',
    children: [
      { key: '/m', label: '数据驾驶舱', icon: <DashboardOutlined /> },
    ],
  },
  {
    key: 'assets', label: '资产',
    children: [
      { key: '/m/brands', label: '品牌管理', icon: <AppstoreOutlined /> },
      { key: '/m/keywords', label: '关键词管理', icon: <SearchOutlined /> },
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
    key: 'distribution', label: '分发',
    children: [
      { key: '/m/distribution', label: '分发中心', icon: <ExportOutlined /> },
    ],
  },
  {
    key: 'assistant', label: 'AI 助手',
    children: [
      { key: '/m/chat', label: 'AI 对话', icon: <MessageOutlined /> },
    ],
  },
]

// 商户端布局。
export default function MerchantLayout() {
  return <AppShell menuItems={merchantMenu} brandName="WebReaper" brandIcon="W" noPaddingKeys={['/m/chat']} />
}

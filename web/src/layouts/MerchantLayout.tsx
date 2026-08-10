import { AppShell, type NavItem } from './MainLayout'
import {
  DashboardOutlined,
  AppstoreOutlined,
  SearchOutlined,
  EditOutlined,
  UserOutlined,
  ExportOutlined,
  MessageOutlined,
} from '@ant-design/icons'

// 商户端布局：GEO 核心功能菜单。
// 菜单按功能分组：概览 / 资产管理 / 内容运营
const merchantMenu: NavItem[] = [
  { key: '/m', label: '数据驾驶舱', icon: <DashboardOutlined /> },
  { key: '/m/brands', label: '品牌管理', icon: <AppstoreOutlined /> },
  { key: '/m/keywords', label: '关键词管理', icon: <SearchOutlined /> },
  { key: '/m/content', label: '内容工作台', icon: <EditOutlined /> },
  { key: '/m/accounts', label: '账号管理', icon: <UserOutlined /> },
  { key: '/m/publish', label: '内容发布', icon: <ExportOutlined /> },
  { key: '/m/chat', label: 'AI 对话', icon: <MessageOutlined /> },
]

// 商户端布局。
export default function MerchantLayout() {
  return <AppShell menuItems={merchantMenu} brandName="GEO 商户端" brandIcon="G" noPaddingKeys={['/m/chat']} />
}

import { AppShell, type NavItem } from './MainLayout'
import {
  DashboardOutlined,
  ExportOutlined,
  FundOutlined,
  MessageOutlined,
  CrownOutlined,
  BellOutlined,
  DatabaseOutlined,
  FolderOpenOutlined,
  AppstoreAddOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { PRODUCT } from '../config/product'

/**
 * 老板向精简导航：爆款获客内分「发视频 / 发图文」双轨。
 */
const menu: NavItem[] = [
  {
    key: 'overview', label: '拓客',
    children: [
      { key: '/m/dashboard', label: '工作台', icon: <DashboardOutlined /> },
    ],
  },
  {
    key: 'ip', label: '我的 IP',
    children: [
      { key: '/m/brands', label: '人设档案', icon: <UserOutlined /> },
      { key: '/m/assets', label: '数字分身', icon: <DatabaseOutlined /> },
    ],
  },
  {
    key: 'create', label: '内容获客',
    children: [
      { key: '/m/compose', label: '爆款获客', icon: <AppstoreAddOutlined /> },
      { key: '/m/works', label: '我的作品', icon: <FolderOpenOutlined /> },
    ],
  },
  {
    key: 'growth', label: '增长',
    children: [
      { key: '/m/distribution', label: '一键发布', icon: <ExportOutlined /> },
      { key: '/m/analytics', label: '获客数据', icon: <FundOutlined /> },
    ],
  },
  {
    key: 'ops', label: '助理',
    children: [
      { key: '/m/chat', label: 'AI 助手', icon: <MessageOutlined /> },
      { key: '/m/notifications', label: '通知', icon: <BellOutlined /> },
      { key: '/m/my-plan', label: '套餐额度', icon: <CrownOutlined /> },
    ],
  },
]

export default function MerchantLayout() {
  return (
    <AppShell
      menuItems={menu}
      brandName={PRODUCT.name}
      brandIcon="获"
      noPaddingKeys={['/m/chat']}
    />
  )
}

import { AppShell, type NavItem } from './MainLayout'
import {
  DashboardOutlined,
  AppstoreOutlined,
  VideoCameraOutlined,
  ExportOutlined,
  FundOutlined,
  MessageOutlined,
  CrownOutlined,
  BellOutlined,
  DatabaseOutlined,
  FolderOpenOutlined,
} from '@ant-design/icons'

/** 商户端导航：账号 IP / 获客智能体闭环 */
const menu: NavItem[] = [
  {
    key: 'overview', label: '总览',
    children: [
      { key: '/m/dashboard', label: '工作台', icon: <DashboardOutlined /> },
    ],
  },
  {
    key: 'ip', label: '打造 IP',
    children: [
      { key: '/m/brands', label: '人设档案', icon: <AppstoreOutlined /> },
      { key: '/m/assets', label: '资产库', icon: <DatabaseOutlined /> },
    ],
  },
  {
    key: 'create', label: '创作',
    children: [
      { key: '/m/compose', label: '内容合成', icon: <VideoCameraOutlined /> },
      { key: '/m/works', label: '我的作品', icon: <FolderOpenOutlined /> },
    ],
  },
  {
    key: 'growth', label: '增长',
    children: [
      { key: '/m/distribution', label: '发布中心', icon: <ExportOutlined /> },
      { key: '/m/analytics', label: '作品数据', icon: <FundOutlined /> },
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
  return <AppShell menuItems={menu} brandName="获客智能体" brandIcon="获" noPaddingKeys={['/m/chat']} />
}

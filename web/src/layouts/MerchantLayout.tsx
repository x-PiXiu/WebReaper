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
} from '@ant-design/icons'

// 商户端导航：获客智能体四步主线（建档案 → 做视频 → 发出去 → 看效果）。
// 菜单本身就是用户旅程：从上到下走完即见效，每步一个入口。
// 转型变更：GEO 导向（体检第二步）→ 获客导向（视频第二步，效果最后）。
const menu: NavItem[] = [
  {
    key: 'overview', label: '总览',
    children: [
      { key: '/m/dashboard', label: '工作台', icon: <DashboardOutlined /> },
    ],
  },
  {
    key: 'profile', label: '① 建档案',
    children: [
      { key: '/m/brands', label: '品牌档案', icon: <AppstoreOutlined /> },
    ],
  },
  {
    key: 'create', label: '② 做视频',
    children: [
      { key: '/m/studio', label: '内容中心', icon: <VideoCameraOutlined /> },
    ],
  },
  {
    key: 'distribute', label: '③ 发出去',
    children: [
      { key: '/m/distribution', label: '分发中心', icon: <ExportOutlined /> },
    ],
  },
  {
    key: 'results', label: '④ 看效果',
    children: [
      { key: '/m/checkup', label: 'AI 效果', icon: <FundOutlined /> },
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
  // noPaddingKeys 仅保留 Chat（自带全屏布局）
  return <AppShell menuItems={menu} brandName="获客智能体" brandIcon="获" noPaddingKeys={['/m/chat']} />
}

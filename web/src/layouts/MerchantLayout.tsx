import { AppShell, type NavItem } from './MainLayout'
import {
  DashboardOutlined,
  AppstoreOutlined,
  SearchOutlined,
  EditOutlined,
  ExportOutlined,
  MessageOutlined,
  CrownOutlined,
  BellOutlined,
} from '@ant-design/icons'

// 商户端导航：四步主线（建档案 → 做体检 → 造内容 → 发出去）——
// 菜单本身就是用户旅程：从上到下走完即见效，每步一个入口。
// （原"附近同行/关键词工程/内容生成/多媒体创作/AI 可见度"五个功能页已收编进
//   AI 体检与内容中心的子层；附近同行保留直链路由，菜单不再显示。
//   本地/线上品牌差异在页面内呈现，菜单不再动态裁剪——减少菜单跳动。）
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
    key: 'checkup', label: '② 做体检',
    children: [
      { key: '/m/checkup', label: 'AI 体检', icon: <SearchOutlined /> },
    ],
  },
  {
    key: 'studio', label: '③ 造内容',
    children: [
      { key: '/m/studio', label: '内容中心', icon: <EditOutlined /> },
    ],
  },
  {
    key: 'distribute', label: '④ 发出去',
    children: [
      { key: '/m/distribution', label: '分发中心', icon: <ExportOutlined /> },
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
  // noPaddingKeys 仅保留 Chat（自带全屏布局）——checkup/studio 恢复内容区外边距，
  // 修复 Tab 与内容左右贴边（此前误入无外边距名单）
  return <AppShell menuItems={menu} brandName="智擎AI" brandIcon="智" noPaddingKeys={['/m/chat']} />
}

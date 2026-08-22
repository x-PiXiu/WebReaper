import { AppShell, type NavItem } from './MainLayout'
import {
  DashboardOutlined,
  ExportOutlined,
  FundOutlined,
  MessageOutlined,
  CrownOutlined,
  DatabaseOutlined,
  FolderOpenOutlined,
  AppstoreAddOutlined,
  UserOutlined,
  FireOutlined,
} from '@ant-design/icons'
import { PRODUCT } from '../config/product'

/**
 * 商户侧栏：一级扁平；文案对齐「账号 IP 获客智能体」闭环。
 * 建人设 → 找爆款 → 创作 → 作品 → 发出去 → 看线索 → 问管家
 */
const menu: NavItem[] = [
  { key: '/m/dashboard', label: '工作台', icon: <DashboardOutlined /> },
  { key: '/m/brands', label: '账号人设', icon: <UserOutlined /> },
  { key: '/m/assets', label: '素材库', icon: <DatabaseOutlined /> },
  { key: '/m/inspire', label: '灵感广场', icon: <FireOutlined /> },
  { key: '/m/compose', label: '创作台', icon: <AppstoreAddOutlined /> },
  { key: '/m/works', label: '我的作品', icon: <FolderOpenOutlined /> },
  { key: '/m/distribution', label: '一键发布', icon: <ExportOutlined /> },
  { key: '/m/analytics', label: '获客复盘', icon: <FundOutlined /> },
  { key: '/m/chat', label: '获客管家', icon: <MessageOutlined /> },
  { key: '/m/my-plan', label: '套餐额度', icon: <CrownOutlined /> },
]

export default function MerchantLayout() {
  return (
    <AppShell
      menuItems={menu}
      brandName={PRODUCT.name}
      brandTagline={PRODUCT.tagline}
      brandIcon="获"
      siderWidth={300}
      noPaddingKeys={['/m/chat', '/m/compose/video', '/m/compose/graphic']}
    />
  )
}

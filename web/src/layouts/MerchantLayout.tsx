import { AppShell, type NavItem } from './MainLayout'
import {
  HomeOutlined,
  VideoCameraOutlined,
  FileImageOutlined,
  UserOutlined,
  AudioOutlined,
  DatabaseOutlined,
  ExportOutlined,
  FolderOpenOutlined,
  IdcardOutlined,
  FireOutlined,
  FundOutlined,
  GlobalOutlined,
  MessageOutlined,
} from '@ant-design/icons'
import { PRODUCT } from '../config/product'

/**
 * 商户侧栏：按「老板获客四步闭环」排布，与工作台 GROWTH_STAGES 同一叙事
 * （建人设 → 出内容 → 发出去 → 看线索），替代旧的工具抽屉式分组。
 * - 图文创作此前无导航入口，现与视频创作并列露出
 * - AI 搜索获客（GEO）首次给出一级入口，深链打开 AI 效果报告
 * - 套餐额度撤出导航：头像下拉与侧栏底部「续费」卡均可达，导航留给业务
 */
const menu: NavItem[] = [
  { key: '/m/compose', label: '首页', icon: <HomeOutlined /> },
  {
    key: 'grp-persona',
    label: '建人设',
    children: [
      { key: '/m/brands', label: '人设档案', icon: <IdcardOutlined /> },
      { key: '/m/compose/avatar', label: '数字人库', icon: <UserOutlined /> },
      { key: '/m/compose/voice', label: '音色库', icon: <AudioOutlined /> },
    ],
  },
  {
    key: 'grp-create',
    label: '出内容',
    children: [
      { key: '/m/inspire', label: '灵感广场', icon: <FireOutlined /> },
      { key: '/m/compose/lipsync', label: '视频创作', icon: <VideoCameraOutlined /> },
      { key: '/m/compose/graphic', label: '图文创作', icon: <FileImageOutlined /> },
      { key: '/m/assets', label: '分镜素材', icon: <DatabaseOutlined /> },
    ],
  },
  {
    key: 'grp-publish',
    label: '发出去',
    children: [
      { key: '/m/distribution', label: '一键发布', icon: <ExportOutlined /> },
      { key: '/m/works', label: '我的作品', icon: <FolderOpenOutlined /> },
    ],
  },
  {
    key: 'grp-measure',
    label: '看线索',
    children: [
      { key: '/m/analytics', label: '作品数据', icon: <FundOutlined /> },
      { key: '/m/analytics?tab=report', label: 'AI 搜索获客', icon: <GlobalOutlined /> },
      { key: '/m/chat', label: '获客管家', icon: <MessageOutlined /> },
    ],
  },
]

export default function MerchantLayout() {
  return (
    <AppShell
      menuItems={menu}
      brandName={PRODUCT.name}
      brandTagline={PRODUCT.tagline}
      brandIcon="获"
      siderWidth={284}
      noPaddingKeys={['/m/chat', '/m/compose/video', '/m/compose/graphic', '/m/compose/lipsync', '/m/compose/quick', '/m/compose/avatar', '/m/assets', '/m/compose/voice', '/m/compose/copy']}
    />
  )
}

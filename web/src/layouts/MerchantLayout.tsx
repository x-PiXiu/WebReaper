import { AppShell, type NavItem } from './MainLayout'
import {
  HomeOutlined,
  VideoCameraOutlined,
  UserOutlined,
  AudioOutlined,
  EditOutlined,
  DatabaseOutlined,
  ExportOutlined,
  FolderOpenOutlined,
  IdcardOutlined,
  FireOutlined,
  FundOutlined,
  MessageOutlined,
  CrownOutlined,
} from '@ant-design/icons'
import { PRODUCT } from '../config/product'

/**
 * 商户侧栏：对齐「创意工作台」工具化排布（参考口播智能体信息架构）。
 * 上半：创作链路工具；下半：经营与增长。
 * 路由仍指向现有模块，不新增后端能力。
 */
const menu: NavItem[] = [
  {
    key: 'grp-create',
    label: '创作',
    children: [
      { key: '/m/compose', label: '首页', icon: <HomeOutlined /> },
      { key: '/m/compose/lipsync', label: '视频创作', icon: <VideoCameraOutlined /> },
      { key: '/m/compose/avatar', label: '数字人库', icon: <UserOutlined /> },
      { key: '/m/compose/voice', label: '音色库', icon: <AudioOutlined /> },
      { key: '/m/compose/copy', label: '文案工作室', icon: <EditOutlined /> },
      { key: '/m/assets', label: '分镜素材', icon: <DatabaseOutlined /> },
    ],
  },
  {
    key: 'grp-grow',
    label: '经营',
    children: [
      { key: '/m/distribution', label: '账号发布', icon: <ExportOutlined /> },
      { key: '/m/works', label: '我的作品', icon: <FolderOpenOutlined /> },
      { key: '/m/brands', label: '账号人设', icon: <IdcardOutlined /> },
      { key: '/m/inspire', label: '灵感广场', icon: <FireOutlined /> },
      { key: '/m/analytics', label: '作品数据', icon: <FundOutlined /> },
      { key: '/m/chat', label: '获客管家', icon: <MessageOutlined /> },
      { key: '/m/my-plan', label: '套餐额度', icon: <CrownOutlined /> },
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
      siderWidth={260}
      noPaddingKeys={['/m/chat', '/m/compose/video', '/m/compose/graphic', '/m/compose/lipsync', '/m/compose/quick', '/m/compose/avatar', '/m/assets', '/m/compose/voice', '/m/compose/copy']}
    />
  )
}

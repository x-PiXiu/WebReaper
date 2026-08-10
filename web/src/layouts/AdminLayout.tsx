import { AppShell, type NavItem } from './MainLayout'

// 管理后台布局：菜单按功能域分组（AntD Menu 的 group 类型渲染）。
//   - 平台管理：SaaS 运营（总览/商户/品牌/内容/计费/设置）
//   - GEO 内容引擎：Agent/LLM 配置、AI 对话、工具面板、收录管理
const adminMenu: NavItem[] = [
  {
    key: 'platform', label: '平台管理',
    children: [
      { key: '/admin', label: '平台总览' },
      { key: '/admin/users', label: '商户管理' },
      { key: '/admin/brands', label: '品牌管理' },
      { key: '/admin/contents', label: '内容管理' },
      { key: '/admin/billing', label: '计费管理' },
      { key: '/admin/settings', label: '平台设置' },
    ],
  },
  {
    key: 'geo', label: 'GEO 内容引擎',
    children: [
      { key: '/admin/agent-configs', label: 'Agent 配置' },
      { key: '/admin/chat', label: 'AI 对话' },
      { key: '/admin/tools', label: '工具面板' },
      { key: '/admin/indexing', label: '收录管理' },
    ],
  },
]

// 管理后台布局。
export default function AdminLayout() {
  return <AppShell menuItems={adminMenu} brandName="GEO 管理后台" brandIcon="A" noPaddingKeys={['/admin/chat']} />
}

import { AppShell, type NavItem } from './MainLayout'

// 管理后台布局：菜单按功能域分组（AntD Menu 的 group 类型渲染）。
//   - 平台管理：SaaS 运营（总览/商户/品牌/内容/计费）
//   - 系统配置：平台运行时开关 + 第三方集成凭据（生成厂商/搜索/支付）
//   - GEO 内容引擎：Agent/LLM/工具、AI 对话、提示词模板、收录管理、生成规格
const adminMenu: NavItem[] = [
  {
    key: 'platform', label: '平台管理',
    children: [
      { key: '/admin', label: '平台总览' },
      { key: '/admin/users', label: '商户管理' },
      { key: '/admin/brands', label: '品牌管理' },
      { key: '/admin/contents', label: '内容管理' },
      { key: '/admin/billing', label: '计费管理' },
    ],
  },
  {
    key: 'system', label: '系统配置',
    children: [
      { key: '/admin/settings', label: '平台设置' },
      { key: '/admin/providers', label: '厂商配置' },
    ],
  },
  {
    key: 'geo', label: 'GEO 内容引擎',
    children: [
      { key: '/admin/chat', label: 'AI 对话' },
      { key: '/admin/agent-configs', label: 'Agent 配置' },
      { key: '/admin/prompt-templates', label: '提示词模板' },
      { key: '/admin/indexing', label: '收录管理' },
      { key: '/admin/generation-specs', label: '生成规格' },
    ],
  },
]

// 管理后台布局。
export default function AdminLayout() {
  return <AppShell menuItems={adminMenu} brandName="智擎AI 管理" brandIcon="智" noPaddingKeys={['/admin/chat']} />
}

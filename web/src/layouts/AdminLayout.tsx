import { AppShell, type NavItem } from './MainLayout'

// 管理后台布局：菜单按功能域分组（AntD Menu 的 group 类型渲染）。
// 分组依据（域辨别）：
//   - 平台管理：SaaS 平台本身的运营（总览 + 商户）
//   - GEO 内容引擎：改造后新增的 GEO 域（Agent/LLM 配置、AI 对话、工具、收录）
//   - 数据采集：改造前的"数据采集结构化"域（数据管理 + 任务监控 + 采集配置），
//     与 GEO 无耦合，独立成组保留——避免混淆且不阻塞演进。
const adminMenu: NavItem[] = [
  {
    key: 'platform', label: '平台管理',
    children: [
      { key: '/admin', label: '平台总览' },
      { key: '/admin/users', label: '商户管理' },
      { key: '/admin/brands', label: '品牌管理' },
      { key: '/admin/contents', label: '内容管理' },
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
  {
    key: 'data', label: '数据采集',
    children: [
      { key: '/admin/data', label: '数据管理' },
      { key: '/admin/tasks', label: '任务监控' },
      { key: '/admin/crawl-config', label: '采集配置' },
    ],
  },
]

// 管理后台布局。
export default function AdminLayout() {
  return <AppShell menuItems={adminMenu} brandName="GEO 管理后台" brandIcon="A" noPaddingKeys={['/admin/chat']} />
}

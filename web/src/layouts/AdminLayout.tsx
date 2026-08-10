import { AppShell, type NavItem } from './MainLayout'

// 管理后台布局：菜单按功能域分组（通过 type 区分标题项和路由项）。
// AntD Menu 的 inline 模式支持 type: 'group' 分组渲染。
const adminMenu: (NavItem & { group?: string })[] = [
  // 核心运营
  { key: '/admin', label: '平台总览' },
  { key: '/admin/users', label: '商户管理' },
  { key: '/admin/agent-configs', label: 'Agent 配置' },
  { key: '/admin/data', label: '数据管理' },
  { key: '/admin/tasks', label: '任务监控' },
  // 系统配置
  { key: '/admin/tools', label: '工具面板' },
  { key: '/admin/crawl-config', label: '采集配置' },
  { key: '/admin/indexing', label: '收录管理' },
  // AI 工具
  { key: '/admin/chat', label: 'AI 对话' },
]

// 管理后台布局。
export default function AdminLayout() {
  return <AppShell menuItems={adminMenu} brandName="GEO 管理后台" brandIcon="A" noPaddingKeys={['/admin/chat']} />
}

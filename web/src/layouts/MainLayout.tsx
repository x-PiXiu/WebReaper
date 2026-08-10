import { Layout, Menu, Button, Space, Avatar, Switch } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { useThemeStore } from '../store/theme'

const { Header, Sider, Content } = Layout

export interface NavItem {
  key: string   // 路由路径
  label: string // 菜单显示名
  icon?: React.ReactNode // 菜单图标（可选）
}

// AppShell 是通用应用骨架（侧边栏 + 顶栏 + 内容区）。
// 商户端和管理后台共用此骨架，各自传入不同的菜单项和品牌名。
//
// 设计动机（DRY + 开闭原则）：
//   - 两套布局的视觉结构完全一致，只有菜单内容不同
//   - 抽出 AppShell 后，新增角色布局只需传菜单，零重复代码
export function AppShell({
  menuItems,
  brandName = 'GEO 平台',
  brandIcon = 'G',
  noPaddingKeys = [],
}: {
  menuItems: NavItem[]
  brandName?: string
  brandIcon?: string
  noPaddingKeys?: string[] // 这些路由对应的页面不要外层 padding（如 Chat 自带布局）
}) {
  const navigate = useNavigate()
  const location = useLocation()
  const { username, clearAuth } = useAuthStore()
  const { mode: themeMode, toggle: toggleTheme } = useThemeStore()

  const handleLogout = () => {
    clearAuth()
    navigate('/login', { replace: true })
  }

  // selectedKey 取一级路径（/m/brands → /m/brands，按完整 key 匹配）
  const pathSegs = location.pathname.split('/').filter(Boolean)
  // 尝试用最长前缀匹配菜单项
  const selectedKey = menuItems.find((m) => m.key === location.pathname)?.key
    || menuItems.find((m) => pathSegs.length > 0 && m.key.endsWith('/' + pathSegs[pathSegs.length - 1]))?.key
    || menuItems[0]?.key
    || '/'

  const noPadding = noPaddingKeys.includes(selectedKey)

  return (
    <Layout style={{ minHeight: '100vh', background: '#0a0a0f' }}>
      {/* 侧边栏 */}
      <Sider
        width={240}
        collapsible
        breakpoint="lg"
        collapsedWidth={64}
        style={{
          background: 'var(--wr-bg-surface)',
          borderRight: '1px solid var(--wr-border)',
          position: 'sticky',
          top: 0,
          height: '100vh',
          overflow: 'auto',
        }}
      >
        {/* Logo 区 */}
        <div style={{
          height: 60,
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 20px',
          borderBottom: '1px solid var(--wr-border)',
        }}>
          <div style={{
            width: 32, height: 32, borderRadius: 10,
            background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 17, fontWeight: 800, color: '#fff', flexShrink: 0,
            boxShadow: 'var(--wr-shadow-glow)',
          }}>{brandIcon}</div>
          <span style={{ fontSize: 15, fontWeight: 700, letterSpacing: '-0.02em', color: 'var(--wr-text-primary)', whiteSpace: 'nowrap', overflow: 'hidden' }}>
            {brandName}
          </span>
        </div>

        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={({ key }) => navigate(key)}
          style={{ background: 'transparent', borderInlineEnd: 'none', marginTop: 12, padding: '0 10px' }}
          items={menuItems.map(m => ({ key: m.key, label: m.label, icon: m.icon }))}
        />
      </Sider>

      <Layout style={{ background: 'var(--wr-bg-base)' }}>
        {/* 顶栏：玻璃质感 */}
        <Header style={{
          background: 'var(--wr-bg-surface)',
          backdropFilter: 'blur(16px)',
          WebkitBackdropFilter: 'blur(16px)',
          borderBottom: '1px solid var(--wr-border)',
          padding: '0 24px',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          position: 'sticky',
          top: 0,
          zIndex: 100,
          height: 60,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ color: 'var(--wr-text-muted)', fontSize: 14 }}>{brandName}</span>
            <span style={{ color: 'var(--wr-border-hover)' }}>/</span>
            <span style={{ color: 'var(--wr-text-primary)', fontSize: 15, fontWeight: 600, letterSpacing: '-0.01em' }}>
              {menuItems.find((m) => m.key === selectedKey)?.label || '控制台'}
            </span>
          </div>

          <Space size={12}>
            {/* 主题切换 */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ fontSize: 14 }}>{themeMode === 'dark' ? '深' : '亮'}</span>
              <Switch
                size="small"
                checked={themeMode === 'light'}
                onChange={() => toggleTheme()}
              />
            </div>
            <Avatar size={28} style={{
              background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))',
              fontSize: 13, fontWeight: 600, flexShrink: 0,
            }}>
              {(username || '?')[0].toUpperCase()}
            </Avatar>
            <span style={{ color: 'var(--wr-text-secondary)', fontSize: 14 }}>{username}</span>
            <Button
              size="small"
              type="text"
              style={{ color: 'var(--wr-text-muted)' }}
              onClick={handleLogout}
            >
              退出
            </Button>
          </Space>
        </Header>

        {/* 内容区 */}
        <Content style={{
          margin: 0,
          padding: noPadding ? 0 : 24,
          minHeight: 'calc(100vh - 60px)',
          overflow: noPadding ? 'hidden' : 'visible',
        }}>
          <div className="wr-fade-in">
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}

// MainLayout 保留为默认导出（兼容旧路由引用，现由商户端/管理端布局取代）。
export default function MainLayout() {
  return <AppShell menuItems={[]} />
}

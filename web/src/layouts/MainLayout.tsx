import { Layout, Menu, Button, Space, Avatar, Switch } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { useThemeStore } from '../store/theme'

const { Header, Sider, Content } = Layout

export default function MainLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { username, clearAuth } = useAuthStore()
  const { mode: themeMode, toggle: toggleTheme } = useThemeStore()

  const handleLogout = () => {
    clearAuth()
    navigate('/login', { replace: true })
  }

  const selectedKey = '/' + (location.pathname.split('/')[1] || 'dashboard')

  const menuItems = [
    { key: '/dashboard', label: '仪表盘' },
    { key: '/chat', label: 'AI 对话' },
    { key: '/agent-configs', label: 'Agent 配置' },
    { key: '/data', label: '数据管理' },
    { key: '/tasks', label: '任务监控' },
    { key: '/tools', label: '工具面板' },
    { key: '/external-systems', label: '外部系统' },
    { key: '/crawl-config', label: '采集配置' },
  ]

  return (
    <Layout style={{ minHeight: '100vh', background: '#0a0a0f' }}>
      {/* 侧边栏 */}
      <Sider
        width={220}
        collapsible
        breakpoint="lg"
        style={{
          background: '#0d0d14',
          borderRight: '1px solid rgba(255,255,255,0.04)',
          position: 'sticky',
          top: 0,
          height: '100vh',
          overflow: 'auto',
        }}
      >
        {/* Logo 区 */}
        <div style={{
          height: 56,
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 20px',
          borderBottom: '1px solid rgba(255,255,255,0.04)',
        }}>
          <div style={{
            width: 28, height: 28, borderRadius: 8,
            background: 'linear-gradient(135deg, #6366f1, #22d3ee)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 16, fontWeight: 800, color: '#fff', flexShrink: 0,
            boxShadow: '0 0 12px rgba(99,102,241,0.4)',
          }}>W</div>
          <span style={{ fontSize: 17, fontWeight: 700, letterSpacing: '-0.02em', color: '#e4e4e7' }}>
            WebReaper
          </span>
        </div>

        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={({ key }) => navigate(key)}
          style={{ background: 'transparent', borderInlineEnd: 'none', marginTop: 8, padding: '0 8px' }}
          items={menuItems}
        />
      </Sider>

      <Layout style={{ background: '#0a0a0f' }}>
        {/* 顶栏：玻璃质感 */}
        <Header style={{
          background: 'rgba(18,18,24,0.7)',
          backdropFilter: 'blur(16px)',
          WebkitBackdropFilter: 'blur(16px)',
          borderBottom: '1px solid rgba(255,255,255,0.04)',
          padding: '0 24px',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          position: 'sticky',
          top: 0,
          zIndex: 100,
          height: 56,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ color: '#71717a', fontSize: 14 }}>WebReaper</span>
            <span style={{ color: '#3f3f46' }}>/</span>
            <span style={{ color: '#e4e4e7', fontSize: 15, fontWeight: 500 }}>
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
              background: 'linear-gradient(135deg, #6366f1, #22d3ee)',
              fontSize: 13, fontWeight: 600, flexShrink: 0,
            }}>
              {(username || '?')[0].toUpperCase()}
            </Avatar>
            <span style={{ color: '#a1a1aa', fontSize: 14 }}>{username}</span>
            <Button
              size="small"
              type="text"
              style={{ color: '#71717a' }}
              onClick={handleLogout}
            >
              退出
            </Button>
          </Space>
        </Header>

        {/* 内容区：Chat 页有自己的布局（含会话侧边栏），不需要外层 padding */}
        <Content style={{
          margin: 0,
          padding: selectedKey === '/chat' ? 0 : 24,
          minHeight: 'calc(100vh - 56px)',
          overflow: selectedKey === '/chat' ? 'hidden' : 'visible',
        }}>
          <div className="wr-fade-in">
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}

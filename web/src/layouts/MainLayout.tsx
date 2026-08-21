import { Layout, Menu, Button, Space, Avatar, Switch, AutoComplete, Input, Badge, Popover, List, Empty } from 'antd'
import { SearchOutlined, BellOutlined } from '@ant-design/icons'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../store/auth'
import { useThemeStore } from '../store/theme'
import { useBrandStore } from '../store/brand'
import { clearQueryCache } from '../queryClient'
import { businessApi } from '../api/business'
import { useNotificationList, useUnreadCount, useMarkNotificationRead } from '../hooks/useNotifications'

const { Header, Sider, Content } = Layout

export interface NavItem {
  key: string   // 路由路径（分组项的 key 仅为唯一标识，不参与路由）
  label: string // 菜单显示名
  icon?: React.ReactNode // 菜单图标（可选）
  children?: NavItem[]   // 有 children 时渲染为 AntD 分组（type: 'group'）
}

// 在菜单（含分组）中查找 key 对应的显示名（顶栏标题用）。
function findMenuLabel(items: NavItem[], key: string): string | undefined {
  for (const m of items) {
    if (m.key === key) return m.label
    const c = m.children?.find(c => c.key === key)
    if (c) return c.label
  }
  return undefined
}

// 把菜单项（含分组）映射为 AntD Menu 的 items。
function toMenuItems(items: NavItem[]) {
  return items.map(m => m.children
    ? { type: 'group' as const, label: m.label, children: m.children.map(c => ({ key: c.key, label: c.label, icon: c.icon })) }
    : { key: m.key, label: m.label, icon: m.icon })
}

// 在菜单（含分组）中查找当前路由对应的菜单项（按完整 key 匹配，其次按末段路径匹配）。
function findSelectedKey(items: NavItem[], pathname: string): string | undefined {
  const pathSegs = pathname.split('/').filter(Boolean)
  for (const m of items) {
    if (m.key === pathname) return m.key
    for (const c of m.children || []) {
      if (c.key === pathname) return c.key
      if (pathSegs.length > 0 && c.key.endsWith('/' + pathSegs[pathSegs.length - 1])) return c.key
    }
  }
  return undefined
}

// AppShell 是通用应用骨架（侧边栏 + 顶栏 + 内容区）。
// 商户端和管理后台共用此骨架，各自传入不同的菜单项和品牌名。
//
// 设计动机（DRY + 开闭原则）：
//   - 两套布局的视觉结构完全一致，只有菜单内容不同
//   - 抽出 AppShell 后，新增角色布局只需传菜单，零重复代码
export function AppShell({
  menuItems,
  brandName = '获客智能体',
  brandIcon = 'G',
  noPaddingKeys = [],
  banner,
}: {
  menuItems: NavItem[]
  brandName?: string
  brandIcon?: string
  noPaddingKeys?: string[] // 这些路由对应的页面不要外层 padding（如 Chat 自带布局）
  banner?: React.ReactNode // 内容区顶部横幅插槽（F1-5：admin 默认口令提醒等全局告示）
}) {
  const navigate = useNavigate()
  const location = useLocation()
  const { username, role, clearAuth } = useAuthStore()
  const { mode: themeMode, toggle: toggleTheme } = useThemeStore()

  // 角色切换入口：admin 在用户界面时显示「管理后台」，在管理后台时显示「返回用户界面」
  const inAdmin = location.pathname.startsWith('/admin')
  const showRoleSwitch = role === 'admin'
  const roleSwitchTarget = inAdmin ? '/m/dashboard' : '/admin'

  const handleLogout = () => {
    clearAuth()
    clearQueryCache() // 清数据缓存——防止下一账号看到上一账号的缓存数据
    useBrandStore.getState().setCurrentBrand(null) // 品牌上下文同样不跨账号残留
    navigate('/login', { replace: true })
  }

  // selectedKey 优先完整路径匹配，其次末段路径匹配（含分组子项）；兜底第一个非分组项
  const selectedKey = findSelectedKey(menuItems, location.pathname)
    || menuItems.find(m => !m.children)?.key
    || '/'

  const noPadding = noPaddingKeys.includes(selectedKey)
  const pageTitle = findMenuLabel(menuItems, selectedKey) || '控制台'

  useEffect(() => {
    document.title = `${pageTitle} · 获客智能体`
  }, [pageTitle])

  // 全局搜索：人设 + 作品库 + 发布任务 + 快捷入口
  const [searchReady, setSearchReady] = useState(false)
  const { data: works = [] } = useQuery({
    queryKey: ['merchant-works'],
    queryFn: () => businessApi.listWorks().catch(() => []),
    staleTime: 60_000,
    enabled: !inAdmin,
  })
  const { data: brands = [] } = useQuery({
    queryKey: ['geo-brands'],
    queryFn: () => businessApi.listBrands(),
    staleTime: 60_000,
    enabled: !inAdmin,
  })
  const { data: publishJobs = [] } = useQuery({
    queryKey: ['geo-publish-jobs'],
    queryFn: () => businessApi.listPublishJobs(),
    staleTime: 60_000,
    enabled: !inAdmin && searchReady,
  })

  const searchOptions = [
    ...brands.map((b: { id: string; name: string }) => ({
      value: `人设 · ${b.name}`,
      label: `人设 · ${b.name}`,
      target: '/m/brands',
    })),
    ...works.slice(0, 30).map((w) => ({
      value: `作品 · ${w.title}`,
      label: `作品 · ${w.title}${w.status === 'published' ? '（已发布）' : w.status === 'ready' ? '（待发布）' : '（草稿）'}`,
      target: '/m/works',
    })),
    ...publishJobs.slice(0, 20).map((j: { id: string; title: string; platform: string }) => ({
      value: `发布 · ${j.title || j.id}`,
      label: `发布 · ${j.title || j.id} (${j.platform})`,
      target: '/m/distribution',
    })),
    ...(searchReady ? [
      { value: '快捷 · 内容合成', label: '快捷 · 内容合成', target: '/m/compose' },
      { value: '快捷 · 资产库', label: '快捷 · 资产库', target: '/m/assets' },
      { value: '快捷 · 作品数据', label: '快捷 · 作品数据', target: '/m/analytics' },
    ] : []),
  ]

  const handleSearchSelect = (val: string) => {
    const hit = searchOptions.find(o => o.value === val)
    if (hit) navigate(hit.target)
  }

  return (
    <Layout className="wr-app-layout" style={{ minHeight: '100vh', background: 'var(--wr-bg-base)' }}>
      {/* 侧边栏 */}
      <Sider
        className="wr-app-sider"
        width={220}
        breakpoint="lg"
        collapsedWidth={64}
        style={{
          position: 'sticky',
          top: 0,
          height: '100vh',
          overflow: 'auto',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {/* Logo 区 */}
        <div className="wr-app-brand" style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 18px',
          borderBottom: '1px solid var(--wr-border)',
        }}>
          <div className="wr-app-brand-mark" style={{
            width: 34, height: 34, borderRadius: 11,
            background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 16, fontWeight: 800, color: '#fff', flexShrink: 0,
            boxShadow: 'var(--wr-shadow-glow)',
          }}>{brandIcon}</div>
          <span style={{ fontSize: 15, fontWeight: 700, letterSpacing: '-0.02em', color: 'var(--wr-text-primary)', whiteSpace: 'nowrap', overflow: 'hidden' }}>
            {brandName}
          </span>
        </div>

        <Menu
          theme={themeMode === 'dark' ? 'dark' : 'light'}
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={({ key }) => navigate(key)}
          style={{ background: 'transparent', borderInlineEnd: 'none', marginTop: 12, padding: '0 10px' }}
          items={toMenuItems(menuItems)}
        />
      </Sider>

      <Layout style={{ background: 'var(--wr-bg-base)' }}>
        {/* 顶栏：玻璃质感 */}
        <Header className="wr-app-header" style={{
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
          <div className="wr-header-left" style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
            <span className="wr-header-brand" style={{ color: 'var(--wr-text-muted)', fontSize: 14 }}>{brandName}</span>
            <span className="wr-header-sep" style={{ color: 'var(--wr-border-hover)' }}>/</span>
            <span style={{ color: 'var(--wr-text-primary)', fontSize: 15, fontWeight: 600, letterSpacing: '-0.01em', whiteSpace: 'nowrap' }}>
              {pageTitle}
            </span>
            {showRoleSwitch && (
              <Button
                className="wr-header-role-switch"
                size="small"
                onClick={() => navigate(roleSwitchTarget)}
                style={{
                  marginLeft: 12,
                  background: inAdmin ? 'var(--wr-bg-base)' : 'var(--wr-gradient)',
                  color: inAdmin ? 'var(--wr-text-primary)' : '#fff',
                  border: inAdmin ? '1px solid var(--wr-border-hover)' : 'none',
                  fontWeight: 600,
                  borderRadius: 8,
                  padding: '0 14px',
                  height: 32,
                  boxShadow: inAdmin ? 'none' : 'var(--wr-shadow-glow)',
                }}
              >
                {inAdmin ? '← 返回用户界面' : '管理后台 →'}
              </Button>
            )}
            {!inAdmin && (
              <AutoComplete
                className="wr-header-search"
                options={searchOptions}
                onSelect={handleSearchSelect}
                style={{ width: 240, marginLeft: 16 }}
                popupMatchSelectWidth={280}
                onFocus={() => setSearchReady(true)}
              >
                <Input
                  size="small"
                  prefix={<SearchOutlined style={{ color: 'var(--wr-text-muted)', fontSize: 13 }} />}
                  placeholder="搜索人设 / 作品"
                  variant="borderless"
                  style={{ background: 'var(--wr-input-bg)', borderRadius: 8 }}
                />
              </AutoComplete>
            )}
          </div>

          <Space size={12} className="wr-header-right">
            <NotificationBell />
            <div className="wr-header-theme" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span className="wr-header-theme-label" style={{ fontSize: 14 }}>{themeMode === 'dark' ? '深' : '亮'}</span>
              <Switch
                size="small"
                checked={themeMode === 'light'}
                onChange={() => toggleTheme()}
                aria-label="切换主题"
              />
            </div>
            <Avatar size={28} style={{
              background: 'linear-gradient(135deg, var(--wr-primary), var(--wr-accent))',
              fontSize: 13, fontWeight: 600, flexShrink: 0,
            }}>
              {(username || '?')[0].toUpperCase()}
            </Avatar>
            <span className="wr-header-username" style={{ color: 'var(--wr-text-secondary)', fontSize: 14 }}>{username}</span>
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

        {/* 内容区（外层极光背景）*/}
        <Content className="wr-app-main" style={{
          margin: 0,
          padding: noPadding ? 0 : '16px 20px 24px',
          minHeight: 'calc(100vh - 60px)',
          overflow: noPadding ? 'hidden' : 'visible',
        }}>
          <div className="wr-aurora-bg" style={{ minHeight: '100%', borderRadius: 0 }}>
            <div key={location.pathname} className="wr-page-enter">
              {banner}
              <Outlet />
            </div>
            <div style={{
              marginTop: 40,
              paddingTop: 16,
              borderTop: '1px solid var(--wr-border)',
              fontSize: 11,
              color: 'var(--wr-text-muted)',
              textAlign: 'center',
              opacity: 0.72,
              letterSpacing: '0.08em',
            }}>
              获客智能体
            </div>
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}

// NotificationBell 顶栏通知铃铛：未读数角标 + 通知列表 + 已读。
// 主动唤醒入口（提及率下降/竞品反超/自动复测完成/排期发布完成）。
// 数据走 hooks/useNotifications（与工作台/通知中心共享缓存，已读联动失效）。
const NOTIFY_TYPE_LABEL: Record<string, string> = {
  mention_drop: '提及率下降',
  competitor_overtake: '竞品反超',
  recheck_done: '复测完成',
  scheduled_publish: '排期发布',
  system: '系统通知',
}

function NotificationBell() {
  const navigate = useNavigate()
  const { data: unread } = useUnreadCount()
  const { data: items = [] } = useNotificationList()
  const markRead = useMarkNotificationRead()

  const markAll = () => markRead.mutate(undefined)
  const markOne = (id: string) => markRead.mutate(id)

  const content = (
    <div style={{ width: 340 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--wr-text-primary)' }}>通知</span>
        {(unread?.unread || 0) > 0 && (
          <Button size="small" type="link" style={{ fontSize: 12 }} onClick={markAll}>全部已读</Button>
        )}
      </div>
      {items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无通知" style={{ padding: '24px 0' }} />
      ) : (
        <List
          size="small"
          dataSource={items}
          style={{ maxHeight: 360, overflow: 'auto' }}
          renderItem={(n) => (
            <List.Item
              onClick={() => { if (!n.read) markOne(n.id); if (n.link) navigate(n.link) }}
              style={{
                cursor: 'pointer', padding: '10px 12px', borderRadius: 8,
                background: n.read ? 'transparent' : 'var(--wr-primary-bg)',
                border: 'none', display: 'block',
              }}
            >
              <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--wr-text-primary)', marginBottom: 2 }}>
                {NOTIFY_TYPE_LABEL[n.type] || n.type} · {n.title}
              </div>
              <div style={{ fontSize: 12, color: 'var(--wr-text-secondary)', lineHeight: 1.5 }}>{n.content}</div>
              <div style={{ fontSize: 10, color: 'var(--wr-text-muted)', marginTop: 2 }}>
                {new Date(n.created_at).toLocaleString()}
              </div>
            </List.Item>
          )}
        />
      )}
    </div>
  )

  return (
    <Popover content={content} placement="bottomRight" trigger="click" arrow={false}>
      <Badge count={unread?.unread || 0} size="small" offset={[-2, 2]}>
        <Button
          type="text"
          size="small"
          icon={<BellOutlined style={{ fontSize: 16 }} />}
          style={{ color: 'var(--wr-text-muted)' }}
        />
      </Badge>
    </Popover>
  )
}

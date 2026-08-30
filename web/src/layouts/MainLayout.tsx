import { Layout, Menu, Button, Space, Avatar, AutoComplete, Input, Badge, Popover, List, Empty, Dropdown, Tooltip } from 'antd'
import {
  SearchOutlined, BellOutlined, VideoCameraOutlined, RightOutlined,
  MoonOutlined, SunOutlined, LogoutOutlined, CrownOutlined, SwapOutlined,
} from '@ant-design/icons'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { usePublishableWorks } from '../hooks/usePublishableWorks'
import { useEffect, useRef, useState } from 'react'
import type { InputRef } from 'antd'
import { useAuthStore } from '../store/auth'
import { useThemeStore } from '../store/theme'
import { useBrandStore } from '../store/brand'
import { clearQueryCache } from '../queryClient'
import { businessApi } from '../api/business'
import { useNotificationList, useUnreadCount, useMarkNotificationRead } from '../hooks/useNotifications'
import { PRODUCT } from '../config/product'

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

// 在菜单（含分组）中查找当前路由对应的菜单项（完整匹配优先；其次最长前缀）。
function findSelectedKey(items: NavItem[], pathname: string): string | undefined {
  let best: string | undefined
  let bestLen = -1
  const consider = (key: string) => {
    if (key === pathname) {
      best = key
      bestLen = key.length
      return
    }
    if (pathname.startsWith(key + '/') && key.length > bestLen) {
      best = key
      bestLen = key.length
    }
  }
  for (const m of items) {
    consider(m.key)
    for (const c of m.children || []) consider(c.key)
  }
  if (best) return best
  const pathSegs = pathname.split('/').filter(Boolean)
  for (const m of items) {
    for (const c of m.children || []) {
      if (pathSegs.length > 0 && c.key.endsWith('/' + pathSegs[pathSegs.length - 1])) return c.key
    }
  }
  return undefined
}

// ⌘K 还是 Ctrl K（按平台显示）
const SEARCH_KBD = typeof navigator !== 'undefined' && /mac/i.test(navigator.platform || navigator.userAgent)
  ? '⌘K'
  : 'Ctrl K'

// AppShell 是通用应用骨架（侧边栏 + 顶栏 + 内容区）。
// 商户端和管理后台共用此骨架，各自传入不同的菜单项和品牌名。
//
// 设计动机（DRY + 开闭原则）：
//   - 两套布局的视觉结构完全一致，只有菜单内容不同
//   - 抽出 AppShell 后，新增角色布局只需传菜单，零重复代码
export function AppShell({
  menuItems,
  brandName = PRODUCT.name,
  brandTagline,
  brandIcon = 'G',
  siderWidth = 284,
  noPaddingKeys = [],
  banner,
}: {
  menuItems: NavItem[]
  brandName?: string
  /** 侧栏品牌副标题（产品定位短句） */
  brandTagline?: string
  brandIcon?: string
  /** 侧栏展开宽度 */
  siderWidth?: number
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
  const roleSwitchTarget = inAdmin ? '/m/compose' : '/admin'

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

  const noPadding = noPaddingKeys.some(
    (k) => location.pathname === k || location.pathname.startsWith(k + '/'),
  )
  const pageTitle = findMenuLabel(menuItems, selectedKey) || '控制台'

  useEffect(() => {
    document.title = `${pageTitle} · ${PRODUCT.name}`
  }, [pageTitle])

  // 顶栏滚动感知：内容滚过 8px 后加阴影/分割线（玻璃条浮起）
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  // 全局搜索：人设 + 作品库 + 发布任务 + 快捷入口（⌘K / Ctrl+K 唤起）
  const searchInputRef = useRef<InputRef>(null)
  const [searchReady, setSearchReady] = useState(false)
  useEffect(() => {
    const onKeydown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        searchInputRef.current?.focus()
      }
    }
    window.addEventListener('keydown', onKeydown)
    return () => window.removeEventListener('keydown', onKeydown)
  }, [])

  const { works = [] } = usePublishableWorks({ enabled: !inAdmin, staleTime: 60_000 })
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
      { value: '快捷 · 灵感广场', label: '快捷 · 灵感广场', target: '/m/inspire' },
      { value: '快捷 · 工作台', label: '快捷 · 工作台', target: '/m/compose' },
      { value: '快捷 · 爆款对标', label: '快捷 · 爆款对标', target: '/m/compose/benchmark' },
      { value: '快捷 · 口播数字人', label: '快捷 · 口播数字人', target: '/m/compose/avatar' },
      { value: '快捷 · 一键发布', label: '快捷 · 一键发布', target: '/m/distribution' },
      { value: '快捷 · 分镜素材', label: '快捷 · 分镜素材', target: '/m/assets' },
      { value: '快捷 · 作品数据', label: '快捷 · 作品数据', target: '/m/analytics' },
      { value: '快捷 · 获客管家', label: '快捷 · 获客管家', target: '/m/chat' },
      { value: '快捷 · 账号人设', label: '快捷 · 账号人设', target: '/m/brands' },
    ] : []),
  ]

  const handleSearchSelect = (val: string) => {
    const hit = searchOptions.find(o => o.value === val)
    if (hit) navigate(hit.target)
  }

  // 用户下拉菜单（头像收纳：身份 + 套餐 + 角色切换 + 退出）
  const userMenuItems = [
    {
      key: 'plan',
      icon: <CrownOutlined />,
      label: '套餐额度',
      onClick: () => navigate('/m/my-plan'),
      hidden: inAdmin,
    },
    {
      key: 'role',
      icon: <SwapOutlined />,
      label: inAdmin ? '返回用户界面' : '管理后台',
      onClick: () => navigate(roleSwitchTarget),
      hidden: !showRoleSwitch,
    },
    { type: 'divider' as const },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      danger: true,
      onClick: handleLogout,
    },
  ].filter((i) => !('hidden' in i && i.hidden))

  return (
    <Layout className="wr-app-layout" style={{ minHeight: '100vh', background: 'var(--wr-bg-base)' }}>
      {/* 侧边栏 */}
      <Sider
        className="wr-app-sider"
        width={siderWidth}
        breakpoint="lg"
        collapsedWidth={72}
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
          minHeight: 64,
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          padding: '12px 16px',
          borderBottom: '1px solid var(--wr-border)',
        }}>
          <div className="wr-app-brand-mark" style={{
            width: 38, height: 38, borderRadius: 12,
            background: 'var(--wr-gradient)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 17, fontWeight: 800, color: '#fff', flexShrink: 0,
            boxShadow: '0 4px 14px var(--wr-primary-bg)',
          }}>{brandIcon}</div>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{
              fontSize: 15, fontWeight: 700, letterSpacing: '-0.02em',
              color: 'var(--wr-text-primary)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
            }}>
              {brandName}
            </div>
            {brandTagline ? (
              <div style={{
                fontSize: 11, lineHeight: 1.35, marginTop: 2,
                color: 'var(--wr-text-muted)',
                whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
              }}>
                {brandTagline}
              </div>
            ) : null}
          </div>
        </div>

        <div data-tour="merchant-nav" className="wr-app-sider-nav" style={{ flex: 1, overflow: 'auto', paddingBottom: 8 }}>
          <Menu
            theme={themeMode === 'dark' ? 'dark' : 'light'}
            mode="inline"
            selectedKeys={[selectedKey]}
            onClick={({ key }) => navigate(key)}
            style={{ background: 'transparent', borderInlineEnd: 'none', marginTop: 10, padding: '0 12px' }}
            items={toMenuItems(menuItems)}
          />
        </div>

        {!inAdmin && (
          <div className="wr-sider-member">
            <div className="wr-sider-member-avatar">
              {(username || '?')[0].toUpperCase()}
            </div>
            <div className="wr-sider-member-meta">
              <div className="wr-sider-member-name">{username || '创作者'}</div>
              <div className="wr-sider-member-plan">会员权益</div>
            </div>
            <Button
              size="small"
              type="primary"
              className="wr-sider-member-btn"
              onClick={() => navigate('/m/my-plan')}
            >
              续费
            </Button>
          </div>
        )}
      </Sider>

      <Layout style={{ background: 'var(--wr-bg-base)' }}>
        {/* 顶栏：玻璃质感 + 滚动浮起 */}
        <Header className={`wr-app-header${scrolled ? ' is-scrolled' : ''}`}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0, flex: 1 }}>
            <nav className="wr-crumb" aria-label="页面位置">
              <span className="wr-crumb-root">{brandName}</span>
              <span className="wr-crumb-sep">/</span>
              <span className="wr-crumb-current">{pageTitle}</span>
            </nav>
            {showRoleSwitch && (
              <Button
                className="wr-header-role-switch"
                size="small"
                onClick={() => navigate(roleSwitchTarget)}
                style={{
                  marginLeft: 4,
                  background: 'var(--wr-bg-elevated)',
                  color: 'var(--wr-text-secondary)',
                  border: '1px solid var(--wr-border)',
                }}
              >
                {inAdmin ? '← 返回用户界面' : '管理后台 →'}
              </Button>
            )}
            {!inAdmin && (
              <div data-tour="merchant-search" style={{ marginLeft: 'auto' }}>
                <AutoComplete
                  className="wr-header-search"
                  options={searchOptions}
                  onSelect={handleSearchSelect}
                  popupMatchSelectWidth={300}
                  onFocus={() => setSearchReady(true)}
                >
                  <Input
                    ref={searchInputRef}
                    prefix={<SearchOutlined style={{ color: 'var(--wr-text-muted)', fontSize: 13 }} />}
                    suffix={<span className="wr-kbd">{SEARCH_KBD}</span>}
                    placeholder="搜索人设 / 作品 / 模块"
                    variant="borderless"
                    style={{ background: 'var(--wr-input-bg)' }}
                  />
                </AutoComplete>
              </div>
            )}
          </div>

          <Space size={6} className="wr-header-right" style={{ marginLeft: 16, flexShrink: 0 }}>
            {!inAdmin && (
              <Button
                type="primary"
                size="middle"
                className="wr-cta-btn"
                icon={<VideoCameraOutlined />}
                onClick={() => navigate('/m/compose/lipsync')}
              >
                立即创作
                <RightOutlined style={{ fontSize: 10 }} />
              </Button>
            )}
            <NotificationBell />
            <Tooltip title={themeMode === 'dark' ? '切换到亮色模式' : '切换到暗色模式'} mouseEnterDelay={0.4}>
              <Button
                className="wr-icon-btn"
                type="text"
                aria-label="切换主题"
                icon={themeMode === 'dark' ? <SunOutlined style={{ fontSize: 16 }} /> : <MoonOutlined style={{ fontSize: 16 }} />}
                onClick={() => toggleTheme()}
              />
            </Tooltip>
            <Dropdown
              menu={{ items: userMenuItems }}
              trigger={['click']}
              placement="bottomRight"
              arrow={false}
            >
              <button type="button" className="wr-avatar-btn" aria-label="账号菜单">
                <Avatar size={30} style={{
                  background: 'var(--wr-gradient)',
                  fontSize: 13, fontWeight: 600, flexShrink: 0,
                }}>
                  {(username || '?')[0].toUpperCase()}
                </Avatar>
                <span className="wr-avatar-name">{username}</span>
              </button>
            </Dropdown>
          </Space>
        </Header>

        {/* 内容区（外层极光背景）*/}
        <Content className="wr-app-main" style={{
          margin: 0,
          padding: noPadding ? 0 : '20px 24px 28px',
          minHeight: 'calc(100vh - 64px)',
          overflow: noPadding ? 'hidden' : 'visible',
        }}>
          <div className="wr-aurora-bg" style={{ minHeight: '100%', borderRadius: 0 }}>
            <div key={location.pathname} className="wr-page-enter">
              {banner}
              <Outlet />
            </div>
            <div style={{
              marginTop: noPadding ? 0 : 40,
              paddingTop: noPadding ? 0 : 16,
              borderTop: noPadding ? 'none' : '1px solid var(--wr-border)',
              fontSize: 11,
              color: 'var(--wr-text-muted)',
              textAlign: 'center',
              opacity: noPadding ? 0 : 0.72,
              letterSpacing: '0.08em',
              display: noPadding ? 'none' : 'block',
            }}>
              {PRODUCT.name} · IP 营销拓客
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
        <Space size={4}>
          {(unread?.unread || 0) > 0 && (
            <Button size="small" type="link" style={{ fontSize: 12 }} onClick={markAll}>全部已读</Button>
          )}
          <Button size="small" type="link" style={{ fontSize: 12 }} onClick={() => navigate('/m/notifications')}>查看全部</Button>
        </Space>
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
          className="wr-icon-btn"
          icon={<BellOutlined style={{ fontSize: 16 }} />}
        />
      </Badge>
    </Popover>
  )
}

import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import ErrorBoundary from './components/ErrorBoundary'
import { useThemeStore } from './store/theme'
import './index.css'

const queryClient = new QueryClient()

// 全局登出清理：清空 React Query 缓存（避免旧租户/旧账号的数据残留闪现）。
// 401 强制重新登录与手动退出都调用——不同账号间切换绝不显示他人缓存数据。
export function clearQueryCache() {
  queryClient.clear()
}

// 暗色主题配置
const darkThemeConfig = {
  algorithm: antdTheme.darkAlgorithm,
  token: {
    colorPrimary: '#6366f1',
    colorInfo: '#6366f1',
    colorSuccess: '#22d3ee',
    borderRadius: 10,
    borderRadiusLG: 14,
    fontFamily: "'Inter', -apple-system, 'Segoe UI', 'Noto Sans SC', sans-serif",
    fontSize: 15,
    controlHeight: 38,
    controlHeightLG: 44,
    colorBgBase: '#0a0a0f',
    colorBgContainer: '#121218',
    colorBgElevated: '#1a1a24',
    colorBgSpotlight: '#22222e',
    colorBorder: 'rgba(255,255,255,0.08)',
    colorBorderSecondary: 'rgba(255,255,255,0.04)',
  },
  components: {
    Layout: { siderBg: '#0d0d14', headerBg: 'rgba(18,18,24,0.8)', headerHeight: 60, bodyBg: '#0a0a0f' },
    Menu: {
      darkItemBg: 'transparent', darkSubMenuItemBg: 'transparent',
      darkItemSelectedBg: 'rgba(99,102,241,0.15)', darkItemHoverBg: 'rgba(255,255,255,0.04)',
      darkItemColor: '#71717a', darkItemSelectedColor: '#818cf8',
      itemBorderRadius: 8, itemMarginInline: 8,
    },
    Card: { colorBgContainer: '#121218', headerBg: 'transparent' },
    Table: { headerBg: '#1a1a24', headerColor: '#a1a1aa', rowHoverBg: '#1a1a24', borderColor: 'rgba(255,255,255,0.06)' },
    Button: { primaryShadow: '0 0 16px rgba(99,102,241,0.3)', fontWeight: 500 },
  },
}

// 亮色主题配置（B2B SaaS 数据驾驶舱风格——参考 Stripe/Vercel Analytics）
const lightThemeConfig = {
  algorithm: antdTheme.defaultAlgorithm,
  token: {
    colorPrimary: '#6366f1',
    colorInfo: '#6366f1',
    colorSuccess: '#10b981',
    colorWarning: '#f59e0b',
    colorError: '#ef4444',
    borderRadius: 10,
    borderRadiusLG: 14,
    fontFamily: "'Inter', -apple-system, 'Segoe UI', 'Noto Sans SC', sans-serif",
    fontSize: 15,
    controlHeight: 38,
    controlHeightLG: 44,
    colorBgBase: '#f6f7f9',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',
    colorBgSpotlight: '#f0f1f3',
    colorBorder: 'rgba(0,0,0,0.08)',
    colorBorderSecondary: 'rgba(0,0,0,0.04)',
  },
  components: {
    Layout: { siderBg: '#ffffff', headerBg: '#ffffff', headerHeight: 60, bodyBg: '#f6f7f9' },
    Menu: {
      itemBg: 'transparent', subMenuItemBg: 'transparent',
      itemSelectedBg: 'rgba(99,102,241,0.08)', itemHoverBg: 'rgba(0,0,0,0.03)',
      itemColor: '#71717a', itemSelectedColor: '#6366f1',
      itemBorderRadius: 8, itemMarginInline: 8,
    },
    Card: { colorBgContainer: '#ffffff', headerBg: 'transparent' },
    Table: { headerBg: '#f6f7f9', headerColor: '#52525b', rowHoverBg: '#f6f7f9', borderColor: 'rgba(0,0,0,0.06)' },
    Button: { fontWeight: 500 },
  },
}

// 主题响应式包裹：监听 store 变化，切换 ConfigProvider
function ThemedApp({ children }: { children: React.ReactNode }) {
  const mode = useThemeStore((s) => s.mode)
  const config = mode === 'dark' ? darkThemeConfig : lightThemeConfig

  // 同步 data-theme 属性到 document（CSS 变量切换的关键）
  React.useEffect(() => {
    document.documentElement.setAttribute('data-theme', mode)
    document.body.style.background = mode === 'dark' ? '#0a0a0f' : '#f4f4f5'
  }, [mode])

  return <ConfigProvider locale={zhCN} theme={config}>{children}</ConfigProvider>
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ThemedApp>
          <App />
        </ThemedApp>
      </QueryClientProvider>
    </ErrorBoundary>
  </React.StrictMode>,
)

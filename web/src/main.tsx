import React from 'react'
import ReactDOM from 'react-dom/client'
import { App as AntdApp, ConfigProvider, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import AntdAppApiBridge from './components/AntdAppApiBridge'
import ErrorBoundary from './components/ErrorBoundary'
import { queryClient } from './queryClient'
import { useThemeStore } from './store/theme'
import './index.css'
import './styles/creative-studio.css'
import './styles/digital-human-lib.css'
import './styles/storyboard-lib.css'
import './styles/voice-lib.css'
import './styles/copy-lib.css'

export { clearQueryCache } from './queryClient'

const fontFamily = "'DM Sans', 'Noto Sans SC', -apple-system, 'Segoe UI', 'PingFang SC', sans-serif"

const darkThemeConfig = {
  algorithm: antdTheme.darkAlgorithm,
  token: {
    // 主色与 index.css 的 --wr-primary 统一（暗色亮紫 / 亮色深紫，同一色系）
    colorPrimary: '#8b7cf6',
    colorInfo: '#8b7cf6',
    // 语义色对齐 CSS 变量（--wr-success/warning/danger）
    colorSuccess: '#4ade80',
    colorWarning: '#fbbf24',
    colorError: '#fb7185',
    borderRadius: 10,
    borderRadiusLG: 14,
    fontFamily,
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
    Layout: { siderBg: '#0c0c11', headerBg: 'transparent', headerHeight: 64, bodyBg: '#07070a' },
    Menu: {
      darkItemBg: 'transparent', darkSubMenuItemBg: 'transparent',
      darkItemSelectedBg: 'rgba(139,124,246,0.13)', darkItemHoverBg: 'rgba(255,255,255,0.04)',
      darkItemColor: '#73737e', darkItemSelectedColor: '#a89bff',
      itemBorderRadius: 10, itemMarginInline: 8,
    },
    Card: { colorBgContainer: '#121218', headerBg: 'transparent' },
    Table: { headerBg: '#1a1a24', headerColor: '#a8a8b3', rowHoverBg: '#1a1a24', borderColor: 'rgba(255,255,255,0.06)' },
    Button: { primaryShadow: '0 4px 14px rgba(139,124,246,0.25)', fontWeight: 500 },
    Modal: { contentBg: '#121218' },
  },
}

const lightThemeConfig = {
  algorithm: antdTheme.defaultAlgorithm,
  token: {
    // 主色与 index.css 亮色 --wr-primary 统一；语义色对齐 CSS 变量
    colorPrimary: '#6d55f0',
    colorInfo: '#6d55f0',
    colorSuccess: '#16a34a',
    colorWarning: '#d97706',
    colorError: '#dc2626',
    borderRadius: 10,
    borderRadiusLG: 14,
    fontFamily,
    fontSize: 15,
    controlHeight: 38,
    controlHeightLG: 44,
    colorBgBase: '#f5f6fa',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',
    colorBgSpotlight: '#f1f2f7',
    colorBorder: 'rgba(20,20,40,0.08)',
    colorBorderSecondary: 'rgba(20,20,40,0.04)',
  },
  components: {
    Layout: { siderBg: '#ffffff', headerBg: 'transparent', headerHeight: 64, bodyBg: '#f5f6fa' },
    Menu: {
      itemBg: 'transparent', subMenuItemBg: 'transparent',
      itemSelectedBg: 'rgba(109,85,240,0.09)', itemHoverBg: 'rgba(20,20,40,0.03)',
      itemColor: '#555563', itemSelectedColor: '#6d55f0',
      itemBorderRadius: 10, itemMarginInline: 8,
    },
    Card: { colorBgContainer: '#ffffff', headerBg: 'transparent' },
    Table: { headerBg: '#f5f6fa', headerColor: '#555563', rowHoverBg: '#f1f2f7', borderColor: 'rgba(20,20,40,0.06)' },
    Button: { fontWeight: 600, primaryShadow: '0 4px 14px rgba(109,85,240,0.22)' },
    Modal: { contentBg: '#ffffff' },
  },
}

/** 全局弹窗：居中 + 内容高度自适应视口（超出可滚） */
const modalDefaults = {
  centered: true,
  styles: {
    body: {
      maxHeight: 'min(72vh, calc(100vh - 160px))',
      overflowY: 'auto' as const,
    },
  },
}

function ThemedApp({ children }: { children: React.ReactNode }) {
  const mode = useThemeStore((s) => s.mode)
  const config = mode === 'dark' ? darkThemeConfig : lightThemeConfig

  React.useEffect(() => {
    document.documentElement.setAttribute('data-theme', mode)
    document.body.style.background = mode === 'dark' ? '#0a0a0f' : '#f5f6fa'
  }, [mode])

  return (
    <ConfigProvider locale={zhCN} theme={config} modal={modalDefaults}>
      {/* AntdApp + 桥接：业务/拦截器用 utils/antdApp，避免静态 message/Modal 警告 */}
      <AntdApp>
        <AntdAppApiBridge />
        {children}
      </AntdApp>
    </ConfigProvider>
  )
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

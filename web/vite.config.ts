import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// Vite 配置：dev server 端口 5173，通过 proxy 代理 /api 和 /public 到后端。
// 代理目标来自 VITE_API_PROXY_TARGET（默认 localhost:8082；后端在别的机器时在 .env.local 覆盖）。
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const proxyTarget = env.VITE_API_PROXY_TARGET || 'http://localhost:8082'

  return {
    plugins: [react()],
    test: {
      environment: 'node',
      include: ['src/**/*.test.ts'],
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: proxyTarget,
          changeOrigin: true,
        },
        '/public': {
          target: proxyTarget,
          changeOrigin: true,
        },
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks: {
            'react-vendor': ['react', 'react-dom', 'react-router-dom'],
            antd: ['antd', '@ant-design/icons'],
            // 图表全家桶按需加载（LazyCharts / 首页地图均 lazy），首屏不拉取
            charts: ['@ant-design/charts', 'echarts', 'echarts-for-react'],
            'data-vendor': ['@tanstack/react-query', 'zustand', 'axios'],
            markdown: ['react-markdown', 'rehype-highlight', 'remark-gfm'],
          },
        },
      },
      // charts 分包（echarts + @antv）体积约 2MB，但仅在用到图表的页面按需加载
      chunkSizeWarningLimit: 2400,
    },
  }
})

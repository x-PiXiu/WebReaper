import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// Vite 配置：dev server 端口 5173，通过 proxy 代理 /api 和 /public 到后端。
// 代理目标来自 VITE_API_PROXY_TARGET（默认 192.168.1.34:8082）。
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const proxyTarget = env.VITE_API_PROXY_TARGET || 'http://192.168.1.34:8082'

  return {
    plugins: [react()],
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
            charts: ['@ant-design/charts'],
            'data-vendor': ['@tanstack/react-query', 'zustand', 'axios'],
            markdown: ['react-markdown', 'rehype-highlight', 'remark-gfm'],
          },
        },
      },
      chunkSizeWarningLimit: 1100,
    },
  }
})

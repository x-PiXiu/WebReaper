import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Vite 配置：dev server 端口 5173，通过 proxy 代理 /api 和 /public 到后端 8082。
// 这样前端请求 /api/v1/* 会被转发到 http://localhost:8082，避免跨域；
// /public/*（公开内容站）同样转发——否则相对路径的公开页链接会落到 SPA fallback 跳到登录页。
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
      '/public': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
    },
  },
})

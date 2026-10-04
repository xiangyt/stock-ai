import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  base: '/',                     // 生产环境与后端同域部署
  server: {
    host: true,                  // 监听 0.0.0.0，允许局域网设备访问
    port: 5173,
    // 前端统一使用相对路径 /api/*，由 dev server 代理到后端，
    // 避免局域网设备访问时请求到自身的 localhost:9100
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9100',
        changeOrigin: true,
      },
    },
  },
  preview: {
    host: true,
    port: 4173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9100',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
})

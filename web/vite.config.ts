import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  base: '/',                     // 生产环境与后端同域部署
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
})

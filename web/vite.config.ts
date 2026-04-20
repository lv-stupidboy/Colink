import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 26308,
    strictPort: true, // 端口冲突时报错而不是自动切换
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:26309',
        changeOrigin: true,
        ws: true, // 启用 WebSocket 代理
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
})
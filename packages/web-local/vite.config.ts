import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  base: '/', // Ensure absolute paths for dynamic imports
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@colink/shared-ui': path.resolve(__dirname, '../shared-ui/src'),
    },
  },
  server: {
    port: 26306,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:26305',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: process.env.SKIP_TYPE_CHECK === 'true' ? false : true,
    chunkSizeWarningLimit: 1000,
    minify: 'esbuild',
    target: 'es2020',
  },
  esbuild: {
    target: 'es2020',
    jsxFactory: 'React.createElement',
    jsxFragment: 'React.Fragment',
  },
  experimental: {
    renderBuiltUrl(filename, { hostType }) {
      if (hostType === 'js' || hostType === 'html') {
        if (filename.startsWith('assets/')) {
          return '/' + filename;
        }
      }
      return filename;
    },
  },
})
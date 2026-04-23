import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import fs from 'fs'
import yaml from 'js-yaml'

/**
 * 配置查找顺序（与后端一致）
 * 1. 环境变量 ISDP_CONFIG
 * 2. data/configs/config.yaml（本地配置，不提交）
 * 3. configs/config.yaml.example（默认模板）
 */
function loadConfig() {
  // 默认配置
  const defaultConfig = {
    server: { port: 26305 },
    web: {
      port: 26306,
      api_url: 'http://127.0.0.1:26305',
    },
  };

  // 查找配置文件（从项目根目录）
  const rootDir = path.resolve(__dirname, '..');
  const configPaths = [
    process.env.ISDP_CONFIG,  // 环境变量
    path.join(rootDir, 'data/configs/config.yaml'),  // 本地配置
    path.join(rootDir, 'configs/config.yaml.example'),  // 默认模板
  ];

  for (const configPath of configPaths) {
    if (configPath && fs.existsSync(configPath)) {
      try {
        const content = fs.readFileSync(configPath, 'utf-8');
        const config = yaml.load(content) as any;
        console.log(`Loaded config from: ${configPath}`);
        return { ...defaultConfig, ...config };
      } catch (err) {
        console.warn(`Failed to load config from ${configPath}:`, err);
      }
    }
  }

  console.log('Using default config');
  return defaultConfig;
}

const config = loadConfig();
const webConfig = config.web || { port: 26306, api_url: 'http://127.0.0.1:26305' };

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: webConfig.port,
    strictPort: true, // 端口冲突时报错而不是自动切换
    proxy: {
      '/api': {
        target: webConfig.api_url,
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
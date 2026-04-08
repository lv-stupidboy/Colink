import { defineConfig } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

/**
 * ISDP Playwright 测试配置
 * 用于多 Agent 协作测试开发工作流
 */
export default defineConfig({
  testDir: path.join(__dirname, 'tests/e2e'),
  timeout: 30000,
  expect: {
    timeout: 5000,
  },
  use: {
    baseURL: 'http://localhost:3001',
    headless: false, // 改为有头模式，方便调试
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    trace: 'retain-on-failure',
  },
  reporter: [
    ['html', { outputFolder: path.join(__dirname, 'playwright-report') }],
    ['json', { outputFile: path.join(__dirname, 'test-results.json') }],
    ['list'],
  ],
  projects: [
    {
      name: 'chromium',
      use: {
        viewport: { width: 1920, height: 1080 },
      },
    },
  ],
  outputDir: path.join(__dirname, 'test-results/'),
  preserveOutput: 'failures-only',
  retries: 0, // 先不设重试，快速看到结果
  workers: 1,
});

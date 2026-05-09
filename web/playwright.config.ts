import { defineConfig } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { getTestConfig } from '../auto-test/e2e/fixtures/test-config.ts';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const testConfig = getTestConfig();

/**
 * 生成时间戳格式的运行 ID
 * 格式: YYYYMMDD-HHMMSS
 */
function generateRunId(): string {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  const hours = String(now.getHours()).padStart(2, '0');
  const minutes = String(now.getMinutes()).padStart(2, '0');
  const seconds = String(now.getSeconds()).padStart(2, '0');
  return `${year}${month}${day}-${hours}${minutes}${seconds}`;
}

const runId = generateRunId();

/**
 * ISDP Playwright 测试配置
 * 用于多 Agent 协作测试开发工作流
 *
 * 报告命名规则：
 * - HTML 报告: playwright-report/{runId}/
 * - JSON 结果: test-results/{runId}/test-results.json
 * - 输出目录: test-results/{runId}/
 */
export default defineConfig({
  testDir: path.join(__dirname, '../auto-test/e2e'),
  // Resolve @playwright/test from web/node_modules
  globalSetup: undefined,
  timeout: 30000,
  expect: {
    timeout: 5000,
  },
  use: {
    baseURL: testConfig.webBaseUrl, // 从 config.yaml 读取前端端口
    headless: true, // headless 模式，适合 CI/CLI 环境
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    trace: 'retain-on-failure',
  },
  reporter: [
    ['html', { outputFolder: path.join(__dirname, `playwright-report/${runId}`) }],
    ['json', { outputFile: path.join(__dirname, `test-results/${runId}/test-results.json`) }],
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
  outputDir: path.join(__dirname, `test-results/${runId}/`),
  preserveOutput: 'failures-only',
  retries: 0, // 先不设重试，快速看到结果
  workers: 1,
});

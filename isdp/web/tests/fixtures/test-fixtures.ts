import { test as base, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

/**
 * 测试报告接口
 */
export interface TestReport {
  timestamp: string;
  tests: TestResult[];
  summary: {
    total: number;
    passed: number;
    failed: number;
  };
}

export interface TestResult {
  id: string;
  name: string;
  status: 'passed' | 'failed' | 'skipped';
  duration?: number;
  error?: string;
  issueType?: 'frontend' | 'backend';
  screenshot?: string;
}

/**
 * 扩展 Playwright test 对象，添加 ISDP 特定功能
 */
export const test = base.extend<{
  reportTestResult: (result: TestResult) => Promise<void>;
  generateReport: () => Promise<TestReport>;
}>({
  reportTestResult: async ({}, use, testInfo) => {
    const results: TestResult[] = [];

    await use(async (result: TestResult) => {
      results.push(result);
      // 保存结果到临时文件，供调度器读取
      const resultsPath = path.join(testInfo.project.outputDir, 'test-results.json');
      fs.mkdirSync(path.dirname(resultsPath), { recursive: true });
      fs.writeFileSync(resultsPath, JSON.stringify(results, null, 2));
    });
  },

  generateReport: async ({}, use, testInfo) => {
    await use(async () => {
      const resultsPath = path.join(testInfo.project.outputDir, 'test-results.json');
      if (!fs.existsSync(resultsPath)) {
        return { timestamp: new Date().toISOString(), tests: [], summary: { total: 0, passed: 0, failed: 0 } };
      }
      const results: TestResult[] = JSON.parse(fs.readFileSync(resultsPath, 'utf-8'));
      return {
        timestamp: new Date().toISOString(),
        tests: results,
        summary: {
          total: results.length,
          passed: results.filter(r => r.status === 'passed').length,
          failed: results.filter(r => r.status === 'failed').length,
        },
      } as TestReport;
    });
  },
});

export { expect };

#!/usr/bin/env node

/**
 * ISDP 测试运行器
 *
 * 功能：
 * 1. 运行 Playwright 测试
 * 2. 解析测试结果
 * 3. 生成测试报告
 * 4. 定位问题类型（前端/后端）
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

interface TestResult {
  id: string;
  name: string;
  status: 'passed' | 'failed' | 'skipped';
  duration?: number;
  error?: string;
  issueType?: 'frontend' | 'backend';
}

interface TestReport {
  timestamp: string;
  tests: TestResult[];
  summary: {
    total: number;
    passed: number;
    failed: number;
  };
}

/**
 * 识别问题类型
 */
function identifyIssueType(error: string): 'frontend' | 'backend' {
  const frontendPatterns = [
    /selector.*not found/i,
    /element.*not visible/i,
    /click.*timeout/i,
    /locator.*timeout/i,
    /expect.*failed/i,
    /render/i,
    /component/i,
  ];

  const backendPatterns = [
    /500/i,
    /502/i,
    /503/i,
    /504/i,
    /404/i,
    /timeout/i,
    /network.*error/i,
    /fetch.*failed/i,
    /api.*error/i,
  ];

  for (const pattern of frontendPatterns) {
    if (pattern.test(error)) return 'frontend';
  }

  for (const pattern of backendPatterns) {
    if (pattern.test(error)) return 'backend';
  }

  // 默认为前端问题
  return 'frontend';
}

/**
 * 解析 Playwright JSON 报告
 */
function parsePlaywrightReport(jsonPath: string): TestReport {
  if (!fs.existsSync(jsonPath)) {
    throw new Error(`测试报告不存在：${jsonPath}`);
  }

  const rawData = JSON.parse(fs.readFileSync(jsonPath, 'utf-8'));
  const tests: TestResult[] = [];

  // 解析每个测试用例
  for (const suite of rawData.suites || []) {
    for (const test of suite.tests || []) {
      const result = test.results?.[0];
      const testResult: TestResult = {
        id: test.testId || `T-${tests.length + 1}`,
        name: test.title,
        status: result?.status === 'passed' ? 'passed' :
                result?.status === 'failed' ? 'failed' : 'skipped',
        duration: result?.duration,
      };

      if (result?.status === 'failed' && result.error) {
        testResult.error = result.error.message || JSON.stringify(result.error);
        testResult.issueType = identifyIssueType(testResult.error);
      }

      tests.push(testResult);
    }
  }

  return {
    timestamp: new Date().toISOString(),
    tests,
    summary: {
      total: tests.length,
      passed: tests.filter(t => t.status === 'passed').length,
      failed: tests.filter(t => t.status === 'failed').length,
    },
  };
}

/**
 * 生成人类可读的测试报告
 */
function generateHumanReport(report: TestReport): string {
  let output = '\n';
  output += '═'.repeat(60) + '\n';
  output += '                    ISDP 测试报告\n';
  output += '═'.repeat(60) + '\n\n';
  output += `生成时间：${report.timestamp}\n\n`;

  output += '─'.repeat(60) + '\n';
  output += '测试摘要\n';
  output += '─'.repeat(60) + '\n';
  output += `总计：${report.summary.total} 个测试\n`;
  output += `✅ 通过：${report.summary.passed}\n`;
  output += `❌ 失败：${report.summary.failed}\n`;
  output += `通过率：${((report.summary.passed / report.summary.total) * 100).toFixed(1)}%\n\n`;

  const failedTests = report.tests.filter(t => t.status === 'failed');
  if (failedTests.length > 0) {
    output += '─'.repeat(60) + '\n';
    output += '失败详情\n';
    output += '─'.repeat(60) + '\n\n';

    for (const test of failedTests) {
      output += `🐛 ${test.id}: ${test.name}\n`;
      output += `   类型：${test.issueType === 'frontend' ? '前端' : '后端'}\n`;
      output += `   错误：${test.error?.substring(0, 200)}...\n\n`;
    }
  }

  output += '═'.repeat(60) + '\n';

  return output;
}

/**
 * 主函数
 */
function main() {
  const args = process.argv.slice(2);
  const mode = args[0] || 'run';

  console.log('🚀 ISDP 测试运行器');
  console.log(`模式：${mode}\n`);

  const webDir = path.join(__dirname, '..');
  const reportPath = path.join(webDir, 'test-results.json');
  const htmlReportPath = path.join(webDir, 'playwright-report', 'index.html');

  try {
    if (mode === 'run') {
      // 运行测试
      console.log('正在运行 Playwright 测试...\n');
      execSync('npx playwright test', {
        cwd: webDir,
        stdio: 'inherit',
      });
    } else if (mode === 'report') {
      // 生成报告
      if (!fs.existsSync(reportPath)) {
        console.error('❌ 测试报告不存在，请先运行测试');
        process.exit(1);
      }

      const report = parsePlaywrightReport(reportPath);
      console.log(generateHumanReport(report));

      // 保存简化报告供调度器使用
      const simpleReportPath = path.join(webDir, 'tests', 'test-report.json');
      fs.writeFileSync(simpleReportPath, JSON.stringify(report, null, 2));
      console.log(`\n📄 报告已保存到：${simpleReportPath}`);

      // 如果有失败的测试，返回非零退出码
      if (report.summary.failed > 0) {
        console.log('\n⚠️  发现失败的测试，请查看报告');
        process.exit(1);
      } else {
        console.log('\n✅ 所有测试通过！');
      }
    }
  } catch (error: any) {
    console.error('❌ 测试执行失败:', error.message);

    // 尝试生成报告
    if (fs.existsSync(reportPath)) {
      try {
        const report = parsePlaywrightReport(reportPath);
        console.log(generateHumanReport(report));
      } catch {}
    }

    process.exit(1);
  }
}

main();

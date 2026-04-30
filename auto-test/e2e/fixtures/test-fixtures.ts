// auto-test/e2e/fixtures/test-fixtures.ts
import { test as base, expect } from '@playwright/test';

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
  priority?: 'P0' | 'P1' | 'P2' | 'P3';
  feature?: string;
}

export const test = base.extend<{
  reportTestResult: (result: TestResult) => Promise<void>;
}>({
  reportTestResult: async ({}, use) => {
    const results: TestResult[] = [];
    await use(async (result: TestResult) => {
      results.push(result);
    });
  },
});

export { expect };
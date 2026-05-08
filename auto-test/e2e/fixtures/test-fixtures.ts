// auto-test/e2e/fixtures/test-fixtures.ts
import { test as base, expect, APIRequestContext } from '@playwright/test';

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

export interface TestData {
  projectId?: string;
  projectName?: string;
  threadId?: string;
  threadTitle?: string;
  agentId?: string;
  agentName?: string;
}

export interface TestFixtures {
  testData: TestData;
  apiClient: APIRequestContext;
  reportTestResult: (result: TestResult) => Promise<void>;
}

// API 基础 URL
const API_BASE_URL = 'http://localhost:26305/api/v1';

// 生成唯一 ID
function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

// 创建测试项目
async function createTestProject(request: APIRequestContext): Promise<{ id: string; name: string }> {
  const name = `E2E-Test-${generateId()}`;
  const response = await request.post(`${API_BASE_URL}/projects`, {
    data: {
      name,
      type: 'service',
      mode: 'new',
      status: 'draft',
    },
  });

  if (!response.ok()) {
    throw new Error(`Failed to create test project: ${response.status()}`);
  }

  const data = await response.json();
  return { id: data.id, name };
}

// 删除测试项目
async function deleteTestProject(request: APIRequestContext, projectId: string): Promise<void> {
  const response = await request.delete(`${API_BASE_URL}/projects/${projectId}`);
  if (!response.ok() && response.status() !== 404) {
    console.warn(`Failed to delete test project ${projectId}: ${response.status()}`);
  }
}

// 创建测试线程
async function createTestThread(
  request: APIRequestContext,
  projectId: string
): Promise<{ id: string; title: string }> {
  const title = `E2E-Thread-${generateId()}`;
  const response = await request.post(`${API_BASE_URL}/threads`, {
    data: {
      projectId,
      title,
      status: 'active',
    },
  });

  if (!response.ok()) {
    throw new Error(`Failed to create test thread: ${response.status()}`);
  }

  const data = await response.json();
  return { id: data.id, title };
}

// 删除测试线程
async function deleteTestThread(request: APIRequestContext, threadId: string): Promise<void> {
  const response = await request.delete(`${API_BASE_URL}/threads/${threadId}`);
  if (!response.ok() && response.status() !== 404) {
    console.warn(`Failed to delete test thread ${threadId}: ${response.status()}`);
  }
}

// 创建测试 Agent
async function createTestAgent(
  request: APIRequestContext
): Promise<{ id: string; name: string }> {
  const name = `E2E-Agent-${generateId()}`;
  const response = await request.post(`${API_BASE_URL}/agents`, {
    data: {
      name,
      role: 'agent',
      systemPrompt: 'E2E test agent',
    },
  });

  if (!response.ok()) {
    throw new Error(`Failed to create test agent: ${response.status()}`);
  }

  const data = await response.json();
  return { id: data.id, name };
}

// 删除测试 Agent
async function deleteTestAgent(request: APIRequestContext, agentId: string): Promise<void> {
  const response = await request.delete(`${API_BASE_URL}/agents/${agentId}`);
  if (!response.ok() && response.status() !== 404) {
    console.warn(`Failed to delete test agent ${agentId}: ${response.status()}`);
  }
}

export const test = base.extend<TestFixtures>({
  // 测试数据 fixture - 每个测试自动创建和清理
  testData: async ({ request }, use) => {
    const data: TestData = {};

    // beforeAll: 创建测试数据（可选，按需创建）
    // 这里不预创建，让具体测试按需调用 testDataApi

    await use(data);

    // afterAll: 清理创建的数据
    if (data.threadId) {
      await deleteTestThread(request, data.threadId);
    }
    if (data.projectId) {
      await deleteTestProject(request, data.projectId);
    }
    if (data.agentId) {
      await deleteTestAgent(request, data.agentId);
    }
  },

  // API 客户端 fixture
  apiClient: async ({ request }, use) => {
    await use(request);
  },

  reportTestResult: async ({}, use) => {
    const results: TestResult[] = [];
    await use(async (result: TestResult) => {
      results.push(result);
    });
  },
});

// 测试数据 API helper
export const testDataApi = {
  createProject: createTestProject,
  deleteProject: deleteTestProject,
  createThread: createTestThread,
  deleteThread: deleteTestThread,
  createAgent: createTestAgent,
  deleteAgent: deleteTestAgent,
};

export { expect };
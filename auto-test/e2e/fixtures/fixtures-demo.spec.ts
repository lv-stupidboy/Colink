// auto-test/e2e/fixtures/fixtures-demo.spec.ts
import { test, expect, testDataApi } from './test-fixtures';

/**
 * E2E-01: 测试 Fixtures 演示
 * 验证 testDataApi 和 testData fixture 正常工作
 * @feature F005 - 线程管理
 * @priority P0
 */

test.describe('E2E-01: Fixtures Demo', () => {
  test('E2E-01-01: 创建和清理测试项目', async ({ request, testData }) => {
    // @feature F005 - 线程管理
    // @priority P0
    // @id E2E-01-01

    // 使用 testDataApi 创建测试项目
    const project = await testDataApi.createProject(request);
    testData.projectId = project.id;
    testData.projectName = project.name;

    // 验证项目创建成功
    expect(project.id).toBeDefined();
    expect(project.name).toContain('E2E-Test');

    // 获取项目详情验证
    const response = await request.get(`http://localhost:26305/api/v1/projects/${project.id}`);
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data.id).toBe(project.id);
    expect(data.name).toBe(project.name);

    console.log('✅ E2E-01-01: 测试项目创建成功，将在 afterAll 自动清理');
  });

  test('E2E-01-02: 创建测试线程', async ({ request, testData }) => {
    // @feature F005 - 线程管理
    // @priority P0
    // @id E2E-01-02

    // 先创建项目
    const project = await testDataApi.createProject(request);
    testData.projectId = project.id;

    // 创建线程
    const thread = await testDataApi.createThread(request, project.id);
    testData.threadId = thread.id;
    testData.threadTitle = thread.title;

    expect(thread.id).toBeDefined();
    expect(thread.title).toContain('E2E-Thread');

    console.log('✅ E2E-01-02: 测试线程创建成功');
  });

  test('E2E-01-03: 创建测试 Agent', async ({ request, testData }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    // @id E2E-01-03

    const agent = await testDataApi.createAgent(request);
    testData.agentId = agent.id;
    testData.agentName = agent.name;

    expect(agent.id).toBeDefined();
    expect(agent.name).toContain('E2E-Agent');

    // 验证 Agent 详情
    const response = await request.get(`http://localhost:26305/api/v1/agents/${agent.id}`);
    expect(response.ok()).toBeTruthy();

    console.log('✅ E2E-01-03: 测试 Agent 创建成功');
  });
});
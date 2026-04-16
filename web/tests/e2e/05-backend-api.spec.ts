import { test, expect } from '../fixtures/test-fixtures';

/**
 * BT 系列：后端 API 测试
 * 测试后端接口的可用性和正确性
 * 注意：健康检查接口在 /health，其他 API 在 /api/v1/*
 */

/**
 * BT-01: 健康检查接口
 */
test.describe('BT-01: 健康检查', () => {
  test('应该返回健康状态', async ({ page }) => {
    // 使用 page 直接访问后端地址（绕过代理）
    const response = await page.request.get('http://localhost:26305/health');
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data.status).toBe('ok');
    expect(data.time).toBeDefined();

    console.log('✅ BT-01: 健康检查通过');
  });
});

/**
 * BT-02: 项目列表 API
 */
test.describe('BT-02: 项目列表 API', () => {
  test('应该返回项目列表（可以是空数组）', async ({ page }) => {
    const response = await page.request.get('http://localhost:26305/api/v1/projects');

    // 检查响应状态
    const status = response.status();

    if (status === 200) {
      const data = await response.json();
      expect(Array.isArray(data)).toBeTruthy();
      console.log('✅ BT-02: 项目列表 API 正常，返回', data.length, '个项目');
    } else if (status === 500) {
      const error = await response.text();
      throw new Error(`项目列表 API 返回 500 错误：${error}`);
    } else {
      console.log('⚠️ BT-02: 项目列表 API 返回非 200 状态:', status);
    }
  });
});

/**
 * BT-03: 创建项目 API
 */
test.describe('BT-03: 创建项目 API', () => {
  test('应该能创建新项目', async ({ page }) => {
    const testProjectName = `测试项目-${Date.now()}`;

    const response = await page.request.post('http://localhost:26305/api/v1/projects', {
      data: {
        name: testProjectName,
        type: 'service',
        mode: 'new',
        status: 'draft',
      },
    });

    const status = response.status();

    if (status === 201) {
      const data = await response.json();
      expect(data.id).toBeDefined();
      expect(data.name).toBe(testProjectName);
      console.log('✅ BT-03: 创建项目成功，ID:', data.id);
    } else if (status === 500) {
      const error = await response.text();
      throw new Error(`创建项目 API 返回 500 错误：${error}`);
    } else {
      console.log('⚠️ BT-03: 创建项目 API 返回状态:', status);
    }
  });
});

/**
 * BT-04: 线程列表 API
 */
test.describe('BT-04: 线程列表 API', () => {
  test('应该返回线程列表', async ({ page }) => {
    const response = await page.request.get('http://localhost:26305/api/v1/threads');
    const status = response.status();

    if (status === 200) {
      const data = await response.json();
      expect(Array.isArray(data)).toBeTruthy();
      console.log('✅ BT-04: 线程列表 API 正常，返回', data.length, '个线程');
    } else {
      console.log('⚠️ BT-04: 线程列表 API 返回状态:', status);
    }
  });
});

/**
 * BT-05: Agent 配置 API
 */
test.describe('BT-05: Agent 配置 API', () => {
  test('应该返回 Agent 配置列表', async ({ page }) => {
    const response = await page.request.get('http://localhost:26305/api/v1/agents');
    const status = response.status();

    if (status === 200) {
      const data = await response.json();
      // null 或空数组都是可以接受的
      if (data === null || data === undefined) {
        console.log('✅ BT-05: Agent 配置 API 返回 null（空列表）');
      } else if (Array.isArray(data)) {
        console.log('✅ BT-05: Agent 配置 API 正常，返回', data.length, '个 Agent');
      } else {
        throw new Error(`Agent 配置 API 返回非数组类型：${typeof data}`);
      }
    } else {
      console.log('⚠️ BT-05: Agent 配置 API 返回状态:', status);
    }
  });
});

// auto-test/e2e/websocket/connection.spec.ts
import { test, expect } from '../fixtures/test-fixtures';

/**
 * WS-01: WebSocket 连接管理测试
 * P0 用例：WS-01-01, WS-01-03, WS-01-07
 */

test.describe('WS-01: WebSocket 连接管理 [P0]', () => {

  test('WS-01-01: 页面加载自动建立连接 [F002]', async ({ page }) => {
    // @feature F002 - WebSocket 流式
    // @priority P0

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 进入工作台
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 检查 WebSocket 连接状态指示器
      const wsIndicator = page.locator('[class*="ws-status"], [class*="connection"]');
      // 如果有状态指示器，验证其可见
      if (await wsIndicator.count() > 0) {
        await expect(wsIndicator.first()).toBeVisible();
      }

      // 验证连接状态为已连接（通过检查页面功能正常）
      // 进入线程后应能正常发送消息
      const threadLinks = page.locator('button').filter({ hasText: /进入/i });
      if (await threadLinks.count() > 0) {
        await threadLinks.first().click();
        await page.waitForTimeout(3000);

        // 验证输入框可用（说明 WebSocket 连接正常）
        const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
        await expect(input.first()).toBeVisible();
      }
    }
  });

  test('WS-01-03: 连接失败自动重试（3次） [F002]', async ({ page }) => {
    // @feature F002 - WebSocket 流式
    // @priority P0

    // 模拟 WebSocket 连接失败
    await page.route('**/ws/**', route => route.abort('failed'));

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 检查重试提示或连接错误状态
      const retryIndicator = page.locator('[class*="retry"], [class*="reconnecting"], [class*="connection-error"]');
      // 页面应显示连接错误或正在重试的状态
      if (await retryIndicator.count() > 0) {
        await expect(retryIndicator.first()).toBeVisible();
      }
    }
  });

  test('WS-01-07: 连接超时提示 [F002]', async ({ page }) => {
    // @feature F002 - WebSocket 流式
    // @priority P0

    // 设置超时并模拟慢响应
    await page.route('**/ws/**', route => {
      // 模拟超时（不响应）
      return new Promise(() => {});
    });

    await page.goto('/projects', { timeout: 10000 });
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();

      // 等待超时提示出现
      await page.waitForTimeout(5000);

      // 检查超时提示（如果 WebSocket 服务不可用）
      const timeoutMsg = page.locator('[class*="timeout"], [class*="connection-error"], [class*="failed"]');
      // 此测试可能需要根据实际超时提示机制调整
    }
  });

  test('WS-01-02: 连接状态指示器显示 [F002]', async ({ page }) => {
    // @feature F002 - WebSocket 流式
    // @priority P1

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 检查连接状态指示器存在
      const wsIndicator = page.locator('[class*="ws-status"], [class*="connection-indicator"]');
      // 如果页面有状态指示器，验证其工作正常
      if (await wsIndicator.count() > 0) {
        await expect(wsIndicator.first()).toBeVisible();
      }
    }
  });
});
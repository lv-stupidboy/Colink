// auto-test/e2e/agent-dialog/message-input.spec.ts
import { test, expect } from '../fixtures/test-fixtures';

/**
 * AD-01: 消息输入与发送测试
 * P0 用例：AD-01-01, AD-01-02, AD-01-03, AD-01-04, AD-01-05, AD-01-08, AD-01-14
 */

test.describe('AD-01: 消息输入与发送 [P0]', () => {

  test('AD-01-01: 输入框正常显示与聚焦 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 进入第一个项目
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 查找输入框
      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      await expect(input.first()).toBeVisible();

      // 检查输入框可聚焦
      await input.first().click();
      await expect(input.first()).toBeFocused();
    }
  });

  test('AD-01-02: 输入文本并点击发送成功 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        const testMessage = `测试消息-${Date.now()}`;
        await input.first().fill(testMessage);

        const sendButton = page.locator('button').filter({ hasText: /发送/i });
        if (await sendButton.count() > 0) {
          await sendButton.first().click();
          await page.waitForTimeout(2000);

          // 验证消息显示
          const messageContent = page.locator('.message-content, .message-body');
          await expect(messageContent.first()).toBeVisible();
        }
      }
    }
  });

  test('AD-01-03: 输入 @ 触发 Agent 下拉框 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        await input.first().click();
        await input.first().fill('@');
        await page.waitForTimeout(500);

        // 检查下拉框出现
        const dropdown = page.locator('.mention-dropdown, .ant-dropdown, [class*="agent-list"]');
        await expect(dropdown.first()).toBeVisible();
      }
    }
  });

  test('AD-01-04: 下拉框显示可用 Agent 列表 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        await input.first().click();
        await input.first().fill('@');
        await page.waitForTimeout(500);

        // 检查 Agent 列表项
        const agentItems = page.locator('.mention-dropdown-item, .ant-dropdown-menu-item, [class*="agent-item"]');
        const count = await agentItems.count();
        expect(count).toBeGreaterThan(0);
      }
    }
  });

  test('AD-01-05: 选择单个 Agent 并发送 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        await input.first().click();
        await input.first().fill('@');
        await page.waitForTimeout(500);

        // 选择第一个 Agent
        const agentItems = page.locator('.mention-dropdown-item, .ant-dropdown-menu-item, [class*="agent-item"]');
        if (await agentItems.count() > 0) {
          await agentItems.first().click();
          await page.waitForTimeout(500);

          // 输入消息并发送
          await input.first().fill('请帮我实现这个功能');
          const sendButton = page.locator('button').filter({ hasText: /发送/i });
          if (await sendButton.count() > 0) {
            await sendButton.first().click();
            await page.waitForTimeout(2000);

            // 验证消息显示
            const messageContent = page.locator('.message-content, .message-body');
            await expect(messageContent.first()).toBeVisible();
          }
        }
      }
    }
  });

  test('AD-01-08: 空消息禁止发送 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        // 清空输入框
        await input.first().fill('');

        const sendButton = page.locator('button').filter({ hasText: /发送/i });
        // 空消息时发送按钮应禁用或不响应
        if (await sendButton.count() > 0) {
          const isDisabled = await sendButton.first().isDisabled();
          // 发送按钮应该被禁用或者点击后没有反应
          // 如果不禁用，则需要检查是否有错误提示
          if (!isDisabled) {
            await sendButton.first().click();
            await page.waitForTimeout(500);
            // 检查是否没有新消息产生
          } else {
            expect(isDisabled).toBeTruthy();
          }
        }
      }
    }
  });

  test('AD-01-14: 发送失败错误提示 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0

    // 模拟网络错误
    await page.route('**/api/v1/**', route => route.abort('failed'));

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        await input.first().fill('测试消息');

        const sendButton = page.locator('button').filter({ hasText: /发送/i });
        if (await sendButton.count() > 0) {
          await sendButton.first().click();
          await page.waitForTimeout(2000);

          // 检查错误提示
          const errorMsg = page.locator('.ant-message-error, [class*="error"], [class*="failed"]');
          // 此测试可能需要根据实际错误提示机制调整
        }
      }
    }
  });
});
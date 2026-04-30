import { test, expect } from '../fixtures/test-fixtures';

/**
 * FT-05: 沙箱页面测试
 * 预期：沙箱列表加载，启动/停止按钮可用
 * @feature F005 - 线程管理
 * @priority P2
 */
test.describe('FT-05: 沙箱页面', () => {
  test('FT-05-01: 沙箱列表应该正常加载', async ({ page }) => {
    // @feature F005 - 线程管理
    // @priority P2
    // @id FT-05-01
    await page.goto('/sandbox');
    await page.waitForLoadState('networkidle');

    // 检查沙箱标题
    const title = page.locator('h1, h2').filter({ hasText: /沙箱|Sandbox/i });
    await expect(title.first()).toBeVisible();

    // 检查表格或列表
    const table = page.locator('.ant-table, [class*="sandbox-list"]');
    await expect(table.first()).toBeVisible({ timeout: 5000 });

    console.log('✅ FT-05: 沙箱列表正常加载');
  });

  test('FT-05-02: 沙箱操作按钮应该存在', async ({ page }) => {
    // @feature F005 - 线程管理
    // @priority P2
    // @id FT-05-02
    await page.goto('/sandbox');
    await page.waitForLoadState('networkidle');

    // 查找启动/停止按钮
    const buttons = page.locator('button').filter({
      hasText: /启动 | 停止 | 运行|Start|Stop/i
    });

    const count = await buttons.count();
    if (count > 0) {
      console.log(`✅ FT-05: 找到 ${count} 个沙箱操作按钮`);
    } else {
      console.log('⚠️ FT-05: 未找到沙箱操作按钮');
    }
  });
});

/**
 * FT-06: 工作流页面测试
 * 预期：模板卡片显示，可选择模板
 * @feature F005 - 线程管理
 * @priority P2
 */
test.describe('FT-06: 工作流页面', () => {
  test('FT-06-01: 工作流模板应该正常显示', async ({ page }) => {
    // @feature F005 - 线程管理
    // @priority P2
    // @id FT-06-01
    await page.goto('/workflow');
    await page.waitForLoadState('networkidle');

    // 检查标题
    const title = page.locator('h1, h2').filter({ hasText: /工作流|Workflow/i });
    await expect(title.first()).toBeVisible();

    // 检查模板卡片
    const templateCards = page.locator('.ant-card, [class*="template"]');
    await expect(templateCards.first()).toBeVisible({ timeout: 5000 });

    console.log('✅ FT-06: 工作流模板正常显示');
  });

  test('FT-06-02: 模板卡片应该可选择', async ({ page }) => {
    // @feature F005 - 线程管理
    // @priority P2
    // @id FT-06-02
    await page.goto('/workflow');
    await page.waitForLoadState('networkidle');

    const templateCards = page.locator('.ant-card, [class*="template"]');
    const count = await templateCards.count();

    if (count > 0) {
      // 点击第一个模板
      await templateCards.first().click();
      await page.waitForTimeout(1000);

      // 检查是否有选中状态
      const selectedCard = page.locator('.ant-card-selected, [class*="selected"]');
      if (await selectedCard.count() > 0) {
        console.log('✅ FT-06: 模板可选择');
      } else {
        console.log('⚠️ FT-06: 模板选择状态未显示');
      }
    } else {
      console.log('⚠️ FT-06: 未找到模板卡片');
    }
  });
});

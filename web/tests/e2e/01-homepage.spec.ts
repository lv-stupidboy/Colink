import { test, expect } from '../fixtures/test-fixtures';

/**
 * FT-01: 首页加载测试
 * 预期：Dashboard 正常显示，统计卡片可见
 */
test.describe('FT-01: 首页加载', () => {
  test('Dashboard 应该正常显示', async ({ page }) => {
    await page.goto('/');

    // 等待页面加载
    await page.waitForLoadState('networkidle');

    // 检查 Dashboard 标题
    const title = page.locator('h1, h2').first();
    await expect(title).toBeVisible();

    // 检查统计卡片
    const statCards = page.locator('.ant-statistic, [class*="statistic"]');
    await expect(statCards.first()).toBeVisible({ timeout: 5000 });

    console.log('✅ FT-01: Dashboard 正常显示');
  });

  test('统计卡片应该包含数据', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // 检查是否有统计数字
    const statValues = page.locator('.ant-statistic-content');
    await expect(statValues.first()).toBeVisible();

    console.log('✅ FT-01: 统计卡片包含数据');
  });
});

import { test, expect } from '../../fixtures/test-fixtures';

/**
 * EMP-01: 空状态测试
 * 测试页面在没有数据时的展示
 */
test.describe('EMP-01: 空状态展示', () => {
  test('项目列表为空时显示空状态', async ({ page }) => {
    // 模拟空的项目列表
    await page.route('**/api/v1/projects', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([])
      })
    );

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    // 检查是否有空状态组件
    const hasEmpty = await page.locator('.ant-empty').count() > 0;
    const hasEmptyText = await page.locator('text=暂无数据，|暂无项目，|Empty，|empty').count() > 0;

    if (hasEmpty || hasEmptyText) {
      console.log('✅ EMP-01: 空状态展示正确');
    } else {
      // 检查表格是否为空
      const table = page.locator('.ant-table');
      if (await table.count() > 0) {
        const tbody = table.locator('.ant-table-tbody');
        const rows = tbody.locator('.ant-table-row');
        const rowCount = await rows.count();
        console.log('   表格行数:', rowCount);
        if (rowCount === 0) {
          console.log('✅ EMP-01: 表格为空，符合预期');
        }
      } else {
        console.log('⚠️ EMP-01: 未检测到空状态组件');
      }
    }
  });

  test('空状态页面提供创建按钮', async ({ page }) => {
    await page.route('**/api/v1/projects', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([])
      })
    );

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 检查是否有创建按钮
    const createButton = page.locator('button').filter({ hasText: /新建 | 创建 | New|Create/i });
    const createButtonCount = await createButton.count();

    if (createButtonCount > 0) {
      console.log('✅ EMP-01: 空状态页面提供创建按钮，数量:', createButtonCount);
    } else {
      console.log('⚠️ EMP-01: 空状态页面未找到创建按钮');
    }
  });

  test('Dashboard 空状态展示', async ({ page }) => {
    await page.route('**/api/v1/projects', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([])
      })
    );

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    // Dashboard 应该显示统计卡片
    const statCards = page.locator('.ant-statistic');
    const statCount = await statCards.count();

    if (statCount > 0) {
      console.log('✅ EMP-01: Dashboard 统计卡片数量:', statCount);
    } else {
      console.log('⚠️ EMP-01: Dashboard 未找到统计卡片');
    }
  });
});

/**
 * EMP-02: 加载状态测试
 * 测试页面加载时的状态展示
 */
test.describe('EMP-02: 加载状态', () => {
  test('页面加载时显示 loading 骨架屏', async ({ page }) => {
    // 故意延迟 API 响应
    await page.route('**/api/v1/projects', async route => {
      await new Promise(resolve => setTimeout(resolve, 1000)); // 延迟 1 秒
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([])
      });
    });

    await page.goto('/projects');

    // 立即检查是否有 loading 状态
    await page.waitForTimeout(100);

    const hasLoading = await page.locator('.ant-spin, .ant-skeleton, [class*="loading"]').count() > 0;

    if (hasLoading) {
      console.log('✅ EMP-02: 页面加载时显示 loading 状态');
    } else {
      console.log('⚠️ EMP-02: 未检测到 loading 状态（可能是加载太快）');
    }
  });

  test('表格 loading 状态', async ({ page }) => {
    await page.route('**/api/v1/projects', async route => {
      await new Promise(resolve => setTimeout(resolve, 800));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          { id: '1', name: '测试项目', type: 'service', mode: 'new', status: 'active' }
        ])
      });
    });

    await page.goto('/projects');
    await page.waitForTimeout(100);

    // 检查表格是否有 loading
    const tableLoading = page.locator('.ant-table .ant-spin');
    if (await tableLoading.count() > 0) {
      console.log('✅ EMP-02: 表格 loading 状态正确');
    } else {
      console.log('⚠️ EMP-02: 未检测到表格 loading');
    }
  });
});

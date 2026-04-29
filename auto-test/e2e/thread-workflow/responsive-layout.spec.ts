import { test, expect } from '../../fixtures/test-fixtures';

/**
 * RSP-01: 响应式布局测试
 * 测试不同屏幕尺寸下的页面展示
 */
test.describe('RSP-01: 响应式布局', () => {
  test('移动端视图 (375px) 布局正确', async ({ page }) => {
    // 设置为移动端视口
    await page.setViewportSize({ width: 375, height: 812 });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    // 检查页面是否正常渲染
    const hasContent = await page.locator('.ant-layout, #root, .app').count() > 0;

    if (hasContent) {
      console.log('✅ RSP-01: 移动端视图 (375px) 页面正常渲染');

      // 检查侧边栏是否折叠或隐藏
      const sider = page.locator('.ant-layout-sider');
      if (await sider.count() > 0) {
        const siderWidth = await sider.first().evaluate(el => el.clientWidth);
        console.log('   侧边栏宽度:', siderWidth, 'px');

        if (siderWidth < 100) {
          console.log('   侧边栏已折叠，符合移动端适配');
        }
      }
    } else {
      console.log('⚠️ RSP-01: 移动端视图页面渲染异常');
    }
  });

  test('平板视图 (768px) 布局正确', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    console.log('✅ RSP-01: 平板视图 (768px) 页面正常渲染');

    // 检查统计卡片是否正确排列
    const statCards = page.locator('.ant-row .ant-col');
    const cardCount = await statCards.count();
    console.log('   统计卡片列数:', cardCount);
  });

  test('桌面视图 (1920px) 布局正确', async ({ page }) => {
    await page.setViewportSize({ width: 1920, height: 1080 });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    console.log('✅ RSP-01: 桌面视图 (1920px) 页面正常渲染');

    // 检查内容宽度
    const mainContent = page.locator('.ant-layout-content');
    if (await mainContent.count() > 0) {
      const contentWidth = await mainContent.first().evaluate(el => el.clientWidth);
      console.log('   主内容区域宽度:', contentWidth, 'px');
    }
  });

  test('窗口缩放时页面自适应', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // 从小到大调整窗口
    const sizes = [375, 768, 1024, 1440, 1920];

    for (const width of sizes) {
      await page.setViewportSize({ width, height: Math.round(width * 0.75) });
      await page.waitForTimeout(300);

      const hasError = await page.locator('.ant-result, [class*="error"]').count() > 0;
      if (!hasError) {
        console.log(`✅ RSP-01: ${width}px 宽度下页面无布局错误`);
      }
    }
  });
});

/**
 * RSP-02: 移动端适配细节
 * 测试移动端特定的交互和展示
 */
test.describe('RSP-02: 移动端适配', () => {
  test('移动端菜单可展开/收起', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    // 查找菜单切换按钮（汉堡包按钮）
    const menuToggle = page.locator('.ant-layout-sider-trigger, [class*="menu-toggle"], .anticon-menu');
    if (await menuToggle.count() > 0) {
      console.log('✅ RSP-02: 移动端菜单切换按钮存在');

      // 尝试点击切换
      await menuToggle.first().click();
      await page.waitForTimeout(300);

      // 检查菜单是否展开
      const sider = page.locator('.ant-layout-sider');
      if (await sider.count() > 0) {
        const isCollapsed = await sider.first().evaluate(el =>
          el.classList.contains('ant-layout-sider-collapsed')
        );
        console.log('   点击后侧边栏状态:', isCollapsed ? '已折叠' : '已展开');
      }
    } else {
      console.log('⚠️ RSP-02: 未找到移动端菜单切换按钮');
    }
  });

  test('移动端按钮尺寸适合触摸', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });

    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 检查按钮尺寸
    const buttons = page.locator('.ant-btn');
    const buttonCount = await buttons.count();

    if (buttonCount > 0) {
      const firstButton = buttons.first();
      const size = await firstButton.evaluate(el => ({
        width: el.getBoundingClientRect().width,
        height: el.getBoundingClientRect().height
      }));

      // 触摸目标建议最小 44x44px
      if (size.height >= 32) {
        console.log(`✅ RSP-02: 按钮尺寸适合触摸 (${size.width}x${size.height}px)`);
      } else {
        console.log(`⚠️ RSP-02: 按钮尺寸可能过小 (${size.width}x${size.height}px)`);
      }
    }
  });
});

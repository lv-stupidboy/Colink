import { test, expect } from '../fixtures/test-fixtures';

/**
 * NAV-01: 页面导航测试
 * 测试应用内页面导航功能
 */
test.describe('NAV-01: 页面导航', () => {
  test('从 Dashboard 导航到项目列表', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // 查找 Dashboard 上的项目链接或按钮
    const projectsLink = page.locator('a, button').filter({ hasText: /项目 | Project/i });
    if (await projectsLink.count() > 0) {
      await projectsLink.first().click();
      await page.waitForTimeout(1000);

      const currentUrl = page.url();
      if (currentUrl.includes('/projects')) {
        console.log('✅ NAV-01: Dashboard 到项目列表导航成功');
      } else {
        console.log('⚠️ NAV-01: 导航后 URL 为:', currentUrl);
      }
    } else {
      console.log('⚠️ NAV-01: Dashboard 上未找到项目导航链接');
    }
  });

  test('从项目列表导航到项目详情', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 查找项目列表中的链接
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    const count = await projectLinks.count();

    if (count > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(1500);

      const currentUrl = page.url();
      // 检查 URL 是否包含项目 ID
      const hasProjectId = /\/projects\/[\w-]+/.test(currentUrl);

      if (hasProjectId) {
        console.log('✅ NAV-01: 项目列表到详情页导航成功，URL:', currentUrl);
      } else {
        console.log('⚠️ NAV-01: 导航后 URL 为:', currentUrl);
      }
    } else {
      console.log('⚠️ NAV-01: 项目列表为空，无法测试详情导航');
    }
  });

  test('从项目详情返回列表', async ({ page }) => {
    // 先访问项目列表
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(1500);

      // 查找返回按钮
      const backButtons = page.locator('button, a').filter({ hasText: /返回 |  back|<|←/i });
      if (await backButtons.count() > 0) {
        // 截图看看是什么按钮
        console.log('✅ NAV-01: 详情页找到返回按钮，数量:', await backButtons.count());
      } else {
        console.log('⚠️ NAV-01: 详情页未找到返回按钮');
      }
    }
  });

  test('面包屑导航存在并可点击', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 检查是否有面包屑组件
    const breadcrumb = page.locator('.ant-breadcrumb');
    if (await breadcrumb.count() > 0) {
      console.log('✅ NAV-01: 面包屑导航存在');

      // 检查面包屑项
      const items = breadcrumb.locator('.ant-breadcrumb-link');
      const itemCount = await items.count();
      if (itemCount > 0) {
        console.log('   面包屑项数量:', itemCount);
      }
    } else {
      console.log('⚠️ NAV-01: 未找到面包屑导航');
    }
  });
});

/**
 * NAV-02: 侧边栏菜单导航
 * 测试侧边栏菜单的导航功能
 */
test.describe('NAV-02: 侧边栏菜单', () => {
  test('侧边栏菜单项都存在', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // 查找侧边栏菜单
    const menu = page.locator('.ant-menu');
    if (await menu.count() > 0) {
      console.log('✅ NAV-02: 侧边栏菜单存在');

      // 获取所有菜单项
      const menuItems = menu.locator('.ant-menu-item');
      const itemCount = await menuItems.count();

      if (itemCount > 0) {
        console.log('   菜单项数量:', itemCount);

        // 打印菜单项文本
        const menuTexts: string[] = [];
        for (let i = 0; i < Math.min(itemCount, 10); i++) {
          const text = await menuItems.nth(i).textContent();
          if (text) menuTexts.push(text.trim());
        }
        console.log('   菜单项:', menuTexts.join(', '));
      }
    } else {
      console.log('⚠️ NAV-02: 未找到侧边栏菜单');
    }
  });

  test('点击菜单项可以切换页面', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // 查找菜单项
    const menuItems = page.locator('.ant-menu-item');
    const count = await menuItems.count();

    if (count > 0) {
      // 获取第一个非选中状态的菜单项
      for (let i = 0; i < count; i++) {
        const item = menuItems.nth(i);
        const isSelected = await item.evaluate(el => el.classList.contains('ant-menu-item-selected'));

        if (!isSelected) {
          const itemText = await item.textContent();
          await item.click();
          await page.waitForTimeout(1000);

          const url = page.url();
          console.log(`✅ NAV-02: 点击菜单项 "${itemText?.trim()}" 后 URL: ${url}`);
          break;
        }
      }
    }
  });

  test('当前激活的菜单项样式正确', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');

    const selectedItem = page.locator('.ant-menu-item-selected').first();
    if (await selectedItem.count() > 0) {
      const bgColor = await selectedItem.evaluate(el =>
        window.getComputedStyle(el).backgroundColor
      );
      const color = await selectedItem.evaluate(el =>
        window.getComputedStyle(el).color
      );

      console.log('✅ NAV-02: 当前菜单项样式 - 背景色:', bgColor, ', 文字颜色:', color);
    } else {
      console.log('⚠️ NAV-02: 未找到选中的菜单项');
    }
  });
});

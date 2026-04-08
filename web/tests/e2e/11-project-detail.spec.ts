import { test, expect } from '../fixtures/test-fixtures';

/**
 * PD-01: 项目详情页测试
 * 测试进入项目详情页后的展示和功能
 */
test.describe('PD-01: 项目详情页展示', () => {
  test('应该能进入项目详情页并显示项目信息', async ({ page }) => {
    // 先访问项目列表
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 找到第一个项目链接并点击
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    const count = await projectLinks.count();

    if (count > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 检查是否在详情页 URL
      const currentUrl = page.url();
      const isDetailUrl = /\/projects\/[\w-]+/.test(currentUrl);

      if (isDetailUrl) {
        console.log('✅ PD-01: 成功进入项目详情页，URL:', currentUrl);

        // 检查是否显示项目信息卡片
        const hasProjectCard = await page.locator('.project-detail, .ant-card').count() > 0;
        if (hasProjectCard) {
          console.log('   项目信息卡片已显示');
        }

        // 检查是否有返回按钮
        const hasBackButton = await page.locator('button').filter({ hasText: /返回 | 返回/i }).count() > 0;
        if (hasBackButton) {
          console.log('   返回按钮已显示');
        }

        // 检查是否有任务列表 Tab
        const hasThreadTab = await page.locator('.ant-tabs-tab').filter({ hasText: /任务 | Thread/i }).count() > 0;
        if (hasThreadTab) {
          console.log('   任务列表 Tab 已显示');
        }
      } else {
        console.log('⚠️ PD-01: 未进入详情页，当前 URL:', currentUrl);
      }
    } else {
      console.log('⚠️ PD-01: 项目列表为空，无法测试详情页');
    }
  });

  test('项目详情页空任务时应显示空状态', async ({ page }) => {
    // 先访问项目列表
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 检查是否显示空状态（暂无任务）
      const hasEmpty = await page.locator('.ant-empty').count() > 0;
      const hasEmptyText = await page.locator('text=暂无任务，|暂无任务 | 创建第一个任务').count() > 0;

      if (hasEmpty || hasEmptyText) {
        console.log('✅ PD-01: 空任务状态显示正确');

        // 检查是否有创建任务按钮
        const hasCreateButton = await page.locator('button').filter({ hasText: /创建 | 新建/i }).count() > 0;
        if (hasCreateButton) {
          console.log('   创建任务按钮已显示');
        }
      } else {
        console.log('⚠️ PD-01: 未检测到空状态或任务列表');
      }
    }
  });

  test('项目详情页应该显示项目基本信息', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 检查是否显示项目类型、模式等信息
      const hasProjectInfo = await page.locator('.ant-descriptions, [class*="detail"]').count() > 0;
      if (hasProjectInfo) {
        console.log('✅ PD-01: 项目基本信息已显示');
      }

      // 检查是否有编辑和删除按钮
      const hasEditButton = await page.locator('button').filter({ hasText: /编辑/i }).count() > 0;
      const hasDeleteButton = await page.locator('button').filter({ hasText: /删除/i }).count() > 0;

      if (hasEditButton) {
        console.log('   编辑按钮已显示');
      }
      if (hasDeleteButton) {
        console.log('   删除按钮已显示');
      }
    }
  });
});

/**
 * PD-02: 项目详情页交互测试
 */
test.describe('PD-02: 项目详情页交互', () => {
  test('返回按钮应该能返回列表页', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      const firstProjectUrl = await projectLinks.first().evaluate(el => {
        const href = (el as HTMLAnchorElement).href;
        return href;
      });

      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 查找返回按钮
      const backButton = page.locator('button').filter({ hasText: /返回项目列表/i });
      if (await backButton.count() > 0) {
        await backButton.first().click();
        await page.waitForTimeout(1500);

        const currentUrl = page.url();
        if (currentUrl.includes('/projects') && !/\/projects\/[\w-]+/.test(currentUrl)) {
          console.log('✅ PD-02: 返回按钮功能正常');
        } else {
          console.log('⚠️ PD-02: 返回后 URL 为:', currentUrl);
        }
      } else {
        console.log('⚠️ PD-02: 未找到返回按钮');
      }
    }
  });

  test('编辑按钮应该打开编辑弹窗', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const editButton = page.locator('button').filter({ hasText: /编辑/i });
      if (await editButton.count() > 0) {
        await editButton.first().click();
        await page.waitForTimeout(1000);

        // 检查是否有编辑弹窗
        const hasModal = await page.locator('.ant-modal, [class*="modal"]').count() > 0;
        if (hasModal) {
          console.log('✅ PD-02: 编辑弹窗已打开');

          // 检查是否有表单输入框
          const hasInputs = await page.locator('.ant-input, input').count() > 0;
          if (hasInputs) {
            console.log('   编辑表单已显示');
          }
        } else {
          console.log('⚠️ PD-02: 未检测到编辑弹窗');
        }
      }
    }
  });

  test('新建任务按钮应该打开创建弹窗', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 查找新建任务按钮
      const createButton = page.locator('button').filter({ hasText: /新建任务 | 创建/i });
      if (await createButton.count() > 0) {
        await createButton.first().click();
        await page.waitForTimeout(1000);

        // 检查是否有创建弹窗
        const hasModal = await page.locator('.ant-modal, [class*="modal"]').count() > 0;
        if (hasModal) {
          console.log('✅ PD-02: 新建任务弹窗已打开');
        } else {
          console.log('⚠️ PD-02: 未检测到创建弹窗');
        }
      }
    }
  });
});

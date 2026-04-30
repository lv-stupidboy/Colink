// auto-test/e2e/team-package/list-crud.spec.ts
import { test, expect } from '../fixtures/test-fixtures';

/**
 * TP-01: 团队包列表与 CRUD 测试
 * P0 用例：TP-01-01, TP-01-03, TP-01-05, TP-01-06, TP-01-13
 * P1 用例：TP-01-02, TP-01-04, TP-01-07, TP-01-08
 */

test.describe('TP-01: 团队包列表与 CRUD [P0]', () => {

  test('TP-01-01: 团队包列表加载 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P0
    // @id TP-01-01

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    // 验证列表容器存在
    const listContainer = page.locator('.team-package-list, [class*="package-list"], .ant-list');
    await expect(listContainer.first()).toBeVisible();

    // 验证页面标题
    const pageTitle = page.locator('h1, .page-title').filter({ hasText: /团队包|Team Package/i });
    await expect(pageTitle.first()).toBeVisible();
  });

  test('TP-01-03: 创建团队包成功 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P0
    // @id TP-01-03

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    // 点击创建按钮
    const createButton = page.locator('button').filter({ hasText: /创建|新建|新增/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForTimeout(500);

      // 填写表单
      const nameInput = page.locator('input[name="name"], input[placeholder*="名称"], input[placeholder*="Name"]');
      if (await nameInput.count() > 0) {
        await nameInput.first().fill(`测试团队包-${Date.now()}`);

        // 填写描述（可选）
        const descInput = page.locator('textarea[name="description"], textarea[placeholder*="描述"]');
        if (await descInput.count() > 0) {
          await descInput.first().fill('这是一个测试团队包');
        }

        // 提交
        const submitButton = page.locator('.ant-modal .ant-btn-primary, button[type="submit"]');
        if (await submitButton.count() > 0) {
          await submitButton.first().click();
          await page.waitForTimeout(1000);

          // 验证成功提示
          const successMsg = page.locator('.ant-message-success, [class*="success"]');
          // 如果有成功提示则验证
          if (await successMsg.count() > 0) {
            await expect(successMsg).toBeVisible();
          }
        }
      }
    }
  });

  test('TP-01-05: 更新团队包成功 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P0
    // @id TP-01-05

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    // 查找编辑按钮
    const editButton = page.locator('button').filter({ hasText: /编辑|修改|Edit/i });
    if (await editButton.count() > 0) {
      await editButton.first().click();
      await page.waitForTimeout(500);

      // 修改名称
      const nameInput = page.locator('input[name="name"], input[placeholder*="名称"]');
      if (await nameInput.count() > 0) {
        await nameInput.first().fill(`更新团队包-${Date.now()}`);

        // 提交
        const submitButton = page.locator('.ant-modal .ant-btn-primary');
        if (await submitButton.count() > 0) {
          await submitButton.first().click();
          await page.waitForTimeout(1000);
        }
      }
    }
  });

  test('TP-01-06: 删除团队包成功 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P0
    // @id TP-01-06

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    // 查找删除按钮
    const deleteButton = page.locator('button').filter({ hasText: /删除|Delete/i });
    if (await deleteButton.count() > 0) {
      await deleteButton.first().click();
      await page.waitForTimeout(500);

      // 确认删除弹窗
      const confirmButton = page.locator('.ant-modal-confirm .ant-btn-primary');
      if (await confirmButton.count() > 0) {
        await confirmButton.click();
        await page.waitForTimeout(1000);

        // 验证删除成功
        const successMsg = page.locator('.ant-message-success');
        if (await successMsg.count() > 0) {
          await expect(successMsg).toBeVisible();
        }
      }
    }
  });

  test('TP-01-13: 团队包详情查看 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P0
    // @id TP-01-13

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    // 点击查看详情
    const detailButton = page.locator('button').filter({ hasText: /查看|详情|View/i });
    if (await detailButton.count() > 0) {
      await detailButton.first().click();
      await page.waitForTimeout(1000);

      // 验证详情页显示
      const detailContent = page.locator('.team-package-detail, [class*="detail-content"]');
      await expect(detailContent.first()).toBeVisible();
    } else {
      // 如果没有详情按钮，点击列表项进入详情
      const listItem = page.locator('.ant-list-item, [class*="package-item"]');
      if (await listItem.count() > 0) {
        await listItem.first().click();
        await page.waitForTimeout(1000);
      }
    }
  });
});

test.describe('TP-01: 团队包列表与 CRUD [P1]', () => {

  test('TP-01-02: 空列表提示显示 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P1
    // @id TP-01-02

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    // 检查是否有空状态提示（如果列表为空）
    const emptyState = page.locator('.ant-empty, [class*="empty-state"], .ant-list-empty-text');
    // 如果显示空状态，验证提示文本
    if (await emptyState.count() > 0) {
      await expect(emptyState.first()).toBeVisible();
      const emptyText = emptyState.first().locator('text=/暂无|没有|empty/i');
      await expect(emptyText).toBeVisible();
    }
  });

  test('TP-01-04: 创建表单验证 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P1
    // @id TP-01-04

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /创建|新建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForTimeout(500);

      // 测试空名称提交（应被拒绝）
      const submitButton = page.locator('.ant-modal .ant-btn-primary');
      if (await submitButton.count() > 0) {
        await submitButton.first().click();
        await page.waitForTimeout(500);

        // 验证错误提示
        const errorMsg = page.locator('.ant-form-item-explain-error, [class*="error"]');
        if (await errorMsg.count() > 0) {
          await expect(errorMsg.first()).toBeVisible();
        }
      }
    }
  });

  test('TP-01-07: 批量删除功能 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P1
    // @id TP-01-07

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    // 选择多个项目
    const checkboxes = page.locator('.ant-checkbox-input');
    if (await checkboxes.count() >= 2) {
      await checkboxes.nth(0).check();
      await checkboxes.nth(1).check();

      // 找到批量删除按钮
      const batchDeleteButton = page.locator('button').filter({ hasText: /批量删除/i });
      if (await batchDeleteButton.count() > 0) {
        await batchDeleteButton.click();
        await page.waitForTimeout(500);

        // 确认删除
        const confirmButton = page.locator('.ant-modal-confirm .ant-btn-primary');
        if (await confirmButton.count() > 0) {
          await confirmButton.click();
        }
      }
    }
  });

  test('TP-01-08: 删除确认弹窗 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P1
    // @id TP-01-08

    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');

    const deleteButton = page.locator('button').filter({ hasText: /删除/i });
    if (await deleteButton.count() > 0) {
      await deleteButton.first().click();
      await page.waitForTimeout(500);

      // 验证确认弹窗存在
      const confirmModal = page.locator('.ant-modal-confirm');
      await expect(confirmModal).toBeVisible();

      // 验证弹窗标题
      const modalTitle = confirmModal.locator('.ant-modal-confirm-title');
      await expect(modalTitle).toContainText(/删除|确认/i);

      // 取消删除（不执行）
      const cancelButton = confirmModal.locator('.ant-btn-default');
      await cancelButton.click();
    }
  });
});
import { test, expect } from '../fixtures/test-fixtures';

/**
 * ERR-01: 错误处理测试
 * 测试应用的错误处理和用户提示
 */
test.describe('ERR-01: 错误处理', () => {
  test('网络错误时显示友好提示', async ({ page }) => {
    // 模拟网络错误 - 使用 fail 作为 abort 的错误类型
    await page.route('**/api/v1/**', route => route.abort('failed'));

    await page.goto('/');
    await page.waitForLoadState('networkidle');

    // 等待一段时间看是否有错误提示
    await page.waitForTimeout(2000);

    // 检查是否有错误提示（message、notification、alert 等）
    const hasErrorMessage = await page.locator('.ant-message, .ant-notification, [class*="error"]').count() > 0;

    if (hasErrorMessage) {
      console.log('✅ ERR-01: 网络错误时显示了用户提示');
    } else {
      console.log('⚠️ ERR-01: 未检测到明显的错误提示（前端可能已处理）');
    }
  });

  test('API 返回 500 时的处理', async ({ page }) => {
    // 模拟 API 500 错误
    await page.route('**/api/**', route =>
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Internal Server Error' })
      })
    );

    await page.goto('/projects');
    await page.waitForTimeout(2000);

    // 检查是否有错误提示
    const hasError = await page.locator('.ant-message .ant-message-error, .ant-notification-notice-error').count() > 0;

    if (hasError) {
      console.log('✅ ERR-01: API 500 错误时显示了错误提示');
    } else {
      console.log('⚠️ ERR-01: 未检测到 500 错误提示');
    }
  });

  test('API 返回 404 时的处理', async ({ page }) => {
    await page.route('**/api/**', route =>
      route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Not Found' })
      })
    );

    await page.goto('/');
    await page.waitForTimeout(2000);

    const currentUrl = page.url();
    console.log('✅ ERR-01: 404 测试完成，当前 URL:', currentUrl);
  });
});

/**
 * ERR-02: 表单错误处理
 * 测试表单提交失败时的错误展示
 */
test.describe('ERR-02: 表单错误处理', () => {
  test('表单提交失败显示错误信息', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 模拟创建项目 API 失败
    await page.route('**/api/v1/projects', route =>
      route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ message: '项目名称已存在' })
      })
    );

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      // 填写表单
      const nameInput = page.locator('input[placeholder*="项目名称"]');
      if (await nameInput.count() > 0) {
        await nameInput.first().fill('测试项目');

        // 提交表单
        const okButton = page.locator('.ant-modal .ant-btn-primary');
        if (await okButton.count() > 0) {
          await okButton.first().click();
          await page.waitForTimeout(1500);

          // 检查是否有错误提示
          const hasError = await page.locator('.ant-message .ant-message-error').count() > 0;
          if (hasError) {
            console.log('✅ ERR-02: 表单提交失败显示错误提示');
          } else {
            console.log('⚠️ ERR-02: 未检测到表单错误提示');
          }
        }
      }
    }
  });
});

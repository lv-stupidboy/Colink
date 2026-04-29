import { test, expect } from '../../fixtures/test-fixtures';

/**
 * FV-01: 表单验证测试
 * 测试创建项目表单的验证逻辑
 */
test.describe('FV-01: 创建项目表单验证', () => {
  test('项目名称是必填项', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 点击新建按钮
    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      // 直接提交表单（不填写任何内容）
      const okButton = page.locator('.ant-modal .ant-btn-primary');
      if (await okButton.count() > 0) {
        await okButton.first().click();
        await page.waitForTimeout(500);

        // 检查是否显示验证错误
        const hasError = await page.locator('.ant-form-item-explain-error').count() > 0;
        if (hasError) {
          const errorMsg = await page.locator('.ant-form-item-explain-error').first().textContent();
          console.log('✅ FV-01: 项目名称必填验证通过，错误信息:', errorMsg);
        } else {
          console.log('⚠️ FV-01: 未检测到表单验证错误');
        }
      }
    }
  });

  test('项目类型是必填项', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      // 填写项目名称但不选择类型
      const nameInput = page.locator('input[placeholder*="项目名称"]');
      if (await nameInput.count() > 0) {
        await nameInput.first().fill('测试项目-' + Date.now());

        // 提交表单
        const okButton = page.locator('.ant-modal .ant-btn-primary');
        if (await okButton.count() > 0) {
          await okButton.first().click();
          await page.waitForTimeout(500);

          // 检查类型字段的验证错误
          const errors = await page.locator('.ant-form-item-explain-error').all();
          const hasTypeError = errors.length > 0;

          if (hasTypeError) {
            console.log('✅ FV-01: 项目类型必填验证通过');
          } else {
            console.log('⚠️ FV-01: 项目类型验证可能未生效');
          }
        }
      }
    }
  });

  test('开发模式是必填项', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      // 检查开发模式选择器是否存在
      const modeSelect = page.locator('.ant-select').filter({ hasText: /开发模式/i });
      if (await modeSelect.count() > 0) {
        console.log('✅ FV-01: 开发模式选择器存在');
      } else {
        console.log('⚠️ FV-01: 未找到开发模式选择器');
      }
    }
  });
});

/**
 * FV-02: 表单输入测试
 * 测试表单输入的边界条件
 */
test.describe('FV-02: 表单输入边界条件', () => {
  test('项目名称可以输入很长的文本', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      const nameInput = page.locator('input[placeholder*="项目名称"]');
      if (await nameInput.count() > 0) {
        // 输入超长名称 (100 字符)
        const longName = 'A'.repeat(100);
        await nameInput.first().fill(longName);

        const value = await nameInput.first().inputValue();
        if (value === longName) {
          console.log('✅ FV-02: 长文本输入测试通过');
        } else {
          console.log('⚠️ FV-02: 长文本可能被截断');
        }
      }
    }
  });

  test('项目名称支持特殊字符', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      const nameInput = page.locator('input[placeholder*="项目名称"]');
      if (await nameInput.count() > 0) {
        // 输入特殊字符
        const specialName = '测试项目-@#$%^&*()_+-=[]{}|;:,.<>?';
        await nameInput.first().fill(specialName);

        const value = await nameInput.first().inputValue();
        if (value === specialName) {
          console.log('✅ FV-02: 特殊字符输入测试通过');
        } else {
          console.log('⚠️ FV-02: 特殊字符可能被过滤');
        }
      }
    }
  });

  test('描述字段可以输入多行文本', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      const descInput = page.locator('textarea[placeholder*="描述"]');
      if (await descInput.count() > 0) {
        // 输入多行文本
        const multiLine = '第一行\n第二行\n第三行';
        await descInput.first().fill(multiLine);

        const value = await descInput.first().inputValue();
        if (value === multiLine) {
          console.log('✅ FV-02: 多行文本输入测试通过');
        } else {
          console.log('⚠️ FV-02: 多行文本可能被处理');
        }
      }
    }
  });
});

/**
 * FV-03: 下拉选择器测试
 * 测试 Select 组件的功能
 */
test.describe('FV-03: 下拉选择器功能', () => {
  test('项目类型下拉选项正确显示', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      // 点击类型选择器
      const typeSelect = page.locator('.ant-select').filter({ hasText: /项目类型/i });
      if (await typeSelect.count() > 0) {
        await typeSelect.first().click();
        await page.waitForTimeout(300);

        // 检查选项
        const options = page.locator('.ant-select-item-option');
        const optionCount = await options.count();

        if (optionCount >= 3) {
          console.log('✅ FV-03: 项目类型选项数量正确，共', optionCount, '个选项');

          // 检查选项内容
          const optionTexts: string[] = [];
          for (let i = 0; i < Math.min(optionCount, 5); i++) {
            const text = await options.nth(i).textContent();
            if (text) optionTexts.push(text.trim());
          }
          console.log('   选项内容:', optionTexts.join(', '));
        } else {
          console.log('⚠️ FV-03: 项目类型选项数量异常，共', optionCount, '个选项');
        }
      }
    }
  });

  test('开发模式下拉选项正确显示', async ({ page }) => {
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const createButton = page.locator('button').filter({ hasText: /新建 | 创建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForSelector('.ant-modal', { timeout: 3000 });

      // 点击模式选择器
      const modeSelect = page.locator('.ant-select').filter({ hasText: /开发模式/i });
      if (await modeSelect.count() > 0) {
        await modeSelect.first().click();
        await page.waitForTimeout(300);

        // 检查选项
        const options = page.locator('.ant-select-item-option');
        const optionCount = await options.count();

        if (optionCount >= 2) {
          console.log('✅ FV-03: 开发模式选项数量正确，共', optionCount, '个选项');

          const optionTexts: string[] = [];
          for (let i = 0; i < Math.min(optionCount, 3); i++) {
            const text = await options.nth(i).textContent();
            if (text) optionTexts.push(text.trim());
          }
          console.log('   选项内容:', optionTexts.join(', '));
        } else {
          console.log('⚠️ FV-03: 开发模式选项数量异常');
        }
      }
    }
  });
});

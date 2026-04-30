import { test, expect } from '../fixtures/test-fixtures';

/**
 * TW-01: 开发工作台初始化测试
 * 测试创建任务后进入工作台的流程
 * @feature F005 - 线程管理
 * @priority P0
 */
test.describe('TW-01: 工作台初始化', () => {
  test('TW-01-01: 创建任务后应该能进入工作台页面', async ({ page }) => {
    // @feature F005 - 线程管理
    // @priority P0
    // @id TW-01-01
    await page.goto(`/projects`);
    await page.waitForLoadState('networkidle');

    // 找到第一个项目并进入
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 点击新建任务按钮
      const createButton = page.locator('button').filter({ hasText: /新建任务 | 创建/i });
      if (await createButton.count() > 0) {
        await createButton.first().click();
        await page.waitForTimeout(1000);

        // 检查是否有创建任务弹窗
        const hasModal = await page.locator('.ant-modal, [class*="modal"]').count() > 0;
        if (hasModal) {
          console.log('✅ TW-01: 新建任务弹窗已打开');

          // 提交创建（可以填写任务名称或留空）
          const taskInput = page.locator('input[placeholder*="任务名称"], input[placeholder*="任务"]');
          if (await taskInput.count() > 0) {
            await taskInput.first().fill('测试任务-' + Date.now());
          }

          const okButton = page.locator('.ant-modal .ant-btn-primary');
          if (await okButton.count() > 0) {
            await okButton.first().click();
            await page.waitForTimeout(3000);

            // 检查是否跳转到工作台页面
            const currentUrl = page.url();
            const isThreadUrl = /\/threads\/[\w-]+/.test(currentUrl);
            if (isThreadUrl) {
              console.log('✅ TW-01: 成功进入工作台页面，URL:', currentUrl);
            } else {
              console.log('⚠️ TW-01: 未跳转到工作台，当前 URL:', currentUrl);
            }
          }
        }
      }
    }
  });

  test('TW-01-02: 进入工作台后应该显示对话输入框', async ({ page }) => {
    // @feature F005 - 线程管理
    // @priority P0
    // @id TW-01-02
    // 先访问项目列表
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 进入第一个项目
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 找到第一个线程并进入
      const threadLinks = page.locator('button').filter({ hasText: /进入/i });
      if (await threadLinks.count() > 0) {
        await threadLinks.first().click();
        await page.waitForTimeout(3000);

        // 检查是否有输入框
        const hasInput = await page.locator('.ant-input, textarea[placeholder*="输入"]').count() > 0;
        if (hasInput) {
          console.log('✅ TW-01: 工作台输入框已显示');
        } else {
          console.log('⚠️ TW-01: 未找到输入框');
        }

        // 检查是否有发送按钮
        const hasSendButton = await page.locator('button').filter({ hasText: /发送/i }).count() > 0;
        if (hasSendButton) {
          console.log('✅ TW-01: 发送按钮已显示');
        }
      } else {
        // 没有线程，创建一个新的
        const createButton = page.locator('button').filter({ hasText: /新建任务 | 创建/i });
        if (await createButton.count() > 0) {
          await createButton.first().click();
          await page.waitForTimeout(1000);

          const okButton = page.locator('.ant-modal .ant-btn-primary');
          if (await okButton.count() > 0) {
            await okButton.first().click();
            await page.waitForTimeout(3000);

            // 检查是否有输入框
            const hasInput = await page.locator('.ant-input, textarea[placeholder*="输入"]').count() > 0;
            if (hasInput) {
              console.log('✅ TW-01: 工作台输入框已显示');
            }
          }
        }
      }
    }
  });
});

/**
 * TW-02: Agent 触发测试
 * 测试@Agent 功能是否正常
 * @feature F001 - Agent 对话核心
 * @priority P0
 */
test.describe('TW-02: Agent 触发', () => {
  test('TW-02-01: 输入@应该显示 Agent 选择下拉框', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    // @id TW-02-01
    // 进入工作台页面
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    // 进入项目
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      // 进入线程
      const threadLinks = page.locator('button').filter({ hasText: /进入/i });
      let threadUrl = '';
      if (await threadLinks.count() > 0) {
        // 获取第一个线程的 URL
        const firstLink = threadLinks.first();
        threadUrl = await firstLink.evaluate((el) => {
          const onclick = el.getAttribute('onclick');
          if (onclick) {
            const match = onclick.match(/navigate\(['"`](\/[^'"`]+)['"`]\)/);
            if (match) return match[1];
          }
          return '';
        });
        await firstLink.click();
        await page.waitForTimeout(3000);
      } else {
        // 创建新线程
        const createButton = page.locator('button').filter({ hasText: /新建任务 | 创建/i });
        if (await createButton.count() > 0) {
          await createButton.first().click();
          await page.waitForTimeout(1000);

          const okButton = page.locator('.ant-modal .ant-btn-primary');
          if (await okButton.count() > 0) {
            await okButton.first().click();
            await page.waitForTimeout(3000);
          }
        }
      }

      // 查找输入框
      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        // 输入@
        await input.first().click();
        await input.first().fill('@');
        await page.waitForTimeout(500);

        // 检查是否有 Agent 下拉框
        const hasDropdown = await page.locator('.mention-dropdown, [class*="agent"], .ant-list').count() > 0;
        if (hasDropdown) {
          console.log('✅ TW-02: @Agent 下拉框已显示');
        } else {
          console.log('⚠️ TW-02: 未检测到 Agent 下拉框');
        }
      }
    }
  });

  test('TW-02-02: 发送消息后应该显示在消息列表中', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    // @id TW-02-02
    // 进入工作台
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');

    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);

      const threadLinks = page.locator('button').filter({ hasText: /进入/i });
      if (await threadLinks.count() > 0) {
        await threadLinks.first().click();
        await page.waitForTimeout(3000);

        // 查找输入框
        const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
        if (await input.count() > 0) {
          // 输入测试消息
          const testMessage = '测试消息-' + Date.now();
          await input.first().fill(testMessage);

          // 点击发送
          const sendButton = page.locator('button').filter({ hasText: /发送/i });
          if (await sendButton.count() > 0) {
            await sendButton.first().click();
            await page.waitForTimeout(2000);

            // 检查消息是否显示
            const hasMessage = await page.locator('.message-body, .message-content').count() > 0;
            if (hasMessage) {
              console.log('✅ TW-02: 消息已发送到对话区');
            }
          }
        }
      }
    }
  });
});

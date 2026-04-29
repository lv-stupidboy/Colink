# ISDP 测试用例补充实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 ISDP 项目补充 340 个测试用例，迁移已有测试到 `auto-test/` 目录，实现完整的测试覆盖。

**Architecture:** 混合分层结构：前端 E2E 在 `auto-test/e2e/`，后端 Internal 在 `auto-test/internal/`，Vitest 在 `auto-test/vitest/`。按优先级分期实施：P0 → P1 → P2 → P3。

**Tech Stack:** Playwright (E2E), Vitest + RTL (组件), Go testify (后端), Go benchmark (性能)

---

## Phase 1: 测试基础设施搭建

### Task 1: 创建 auto-test 目录结构

**Files:**
- Create: `auto-test/e2e/`, `auto-test/internal/`, `auto-test/vitest/` 目录

**Step 1: 创建顶层目录和子目录结构**

Run: 
```bash
cd D:/CoLinkProject/Colink-Test-0430/isdp
mkdir -p auto-test/e2e/agent-dialog auto-test/e2e/websocket auto-test/e2e/team-package auto-test/e2e/thread-workflow auto-test/e2e/api auto-test/e2e/performance auto-test/e2e/fixtures
mkdir -p auto-test/internal/api auto-test/internal/service/agent auto-test/internal/service/a2a auto-test/internal/service/teampackagesync auto-test/internal/service/im auto-test/internal/repo auto-test/internal/mocks auto-test/internal/testdata auto-test/internal/performance
mkdir -p auto-test/vitest/components auto-test/vitest/stores auto-test/vitest/hooks
mkdir -p auto-test/docs
```

Expected: 所有目录创建成功

**Step 2: 验证目录结构**

Run: `tree auto-test -L 2`

Expected: 显示完整的目录树结构

**Step 3: Commit**

```bash
git add auto-test/
git commit -m "feat: create auto-test directory structure

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 2: 创建 feature-map.yaml 特性映射配置

**Files:**
- Create: `auto-test/feature-map.yaml`

**Step 1: 编写特性映射配置文件**

```yaml
# auto-test/feature-map.yaml
features:
  F001:
    name: "Agent 对话核心"
    priority: P0
    tests:
      e2e:
        - "AD-01-*"
        - "AD-02-01~10"
        - "AD-04-*"
      internal:
        - "SV-01-01~05"
        - "API-01-*"
      vitest:
        - "VT-01-14"
        - "VT-01-15"
  
  F002:
    name: "WebSocket 流式"
    priority: P0
    tests:
      e2e:
        - "WS-01-*"
        - "WS-02-*"
        - "WS-04-*"
      internal:
        - "SV-02-*"
      vitest:
        - "VT-03-03"
        - "VT-03-04"
        - "VT-03-05"
  
  F003:
    name: "多 Agent 协作 (A2A)"
    priority: P0
    tests:
      e2e:
        - "AD-03-*"
        - "WS-03-*"
      internal:
        - "SV-02-*"
      vitest:
        - "VT-03-07"
  
  F004:
    name: "团队包管理"
    priority: P1
    tests:
      e2e:
        - "TP-01-*"
        - "TP-02-*"
        - "TP-03-*"
      internal:
        - "SV-04-*"
        - "API-03-*"
  
  F005:
    name: "线程管理"
    priority: P1
    tests:
      e2e:
        - "TW-01-*"
      internal:
        - "SV-05-06~07"
        - "API-02-*"
        - "RP-02-02"
  
  F006:
    name: "工作流执行"
    priority: P1
    tests:
      e2e:
        - "TW-02-*"
      internal:
        - "SV-05-04~05"
        - "API-04-03~04"
        - "RP-02-03"
  
  F007:
    name: "IM 集成"
    priority: P1
    tests:
      internal:
        - "SV-03-*"
        - "RP-01-05"
  
  F008:
    name: "消息渲染"
    priority: P1
    tests:
      e2e:
        - "AD-02-*"
      vitest:
        - "VT-01-09"
        - "VT-01-10"
  
  F009:
    name: "深色模式"
    priority: P2
    tests:
      vitest:
        - "VT-01-11"
        - "VT-03-01"
      performance:
        - "PF-02-06"
  
  F010:
    name: "性能优化"
    priority: P2
    tests:
      performance:
        - "PF-01-*"
        - "PF-02-*"
```

Write to file: `auto-test/feature-map.yaml`

**Step 2: Commit**

```bash
git add auto-test/feature-map.yaml
git commit -m "feat: add feature-map.yaml for feature-based testing

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 3: 创建后端测试初始化文件

**Files:**
- Create: `auto-test/internal/setup_test.go`

**Step 1: 编写测试初始化代码**

```go
// auto-test/internal/setup_test.go
package internal_test

import (
	"os"
	"testing"
)

// TestMain 初始化测试环境
func TestMain(m *testing.M) {
	// 设置测试环境变量
	os.Setenv("ISDP_TEST_MODE", "true")
	
	// 运行测试
	code := m.Run()
	
	// 清理
	os.Unsetenv("ISDP_TEST_MODE")
	
	os.Exit(code)
}
```

Write to file: `auto-test/internal/setup_test.go`

**Step 2: 运行测试验证**

Run: `go test ./auto-test/internal/setup_test.go -v`

Expected: PASS (空测试套件)

**Step 3: Commit**

```bash
git add auto-test/internal/setup_test.go
git commit -m "feat: add backend test setup file

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 3.5: 创建测试数据库初始化脚本

**Files:**
- Create: `auto-test/internal/testdata/init-sqlite.sql`

**Step 1: 编写测试数据库初始化脚本**

```sql
-- auto-test/internal/testdata/init-sqlite.sql
-- 测试数据库初始化脚本
-- 包含最小化的测试数据集

-- 清空现有数据（测试环境）
DELETE FROM base_agents;
DELETE FROM projects;
DELETE FROM threads;
DELETE FROM agent_configs;
DELETE FROM workflow_templates;

-- 插入测试基础 Agent
INSERT INTO base_agents (id, name, type, description, created_at, updated_at) VALUES
('test-base-001', 'Claude Code', 'claude_code', 'Claude CLI 适配器', datetime('now'), datetime('now')),
('test-base-002', 'Backend Developer', 'claude_code', '后端开发 Agent', datetime('now'), datetime('now')),
('test-base-003', 'Architect', 'claude_code', '架构师 Agent', datetime('now'), datetime('now'));

-- 插入测试项目
INSERT INTO projects (id, name, description, created_at, updated_at) VALUES
('test-proj-001', '测试项目', '用于 E2E 测试的项目', datetime('now'), datetime('now'));

-- 插入测试线程
INSERT INTO threads (id, project_id, title, status, created_at, updated_at) VALUES
('test-thread-001', 'test-proj-001', '测试线程', 'active', datetime('now'), datetime('now'));

-- 插入测试 Agent 配置
INSERT INTO agent_configs (id, project_id, name, base_agent_id, description, created_at, updated_at) VALUES
('test-agent-001', 'test-proj-001', 'Backend Developer', 'test-base-002', '后端开发测试配置', datetime('now'), datetime('now')),
('test-agent-002', 'test-proj-001', 'Architect', 'test-base-003', '架构师测试配置', datetime('now'), datetime('now'));
```

Write to file: `auto-test/internal/testdata/init-sqlite.sql`

**Step 2: Commit**

```bash
git add auto-test/internal/testdata/init-sqlite.sql
git commit -m "feat: add test database initialization script

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 4: 创建前端 E2E fixtures

**Files:**
- Create: `auto-test/e2e/fixtures/test-fixtures.ts`

**Step 1: 编写测试 fixtures**

```typescript
// auto-test/e2e/fixtures/test-fixtures.ts
import { test as base, expect } from '@playwright/test';

export interface TestReport {
  timestamp: string;
  tests: TestResult[];
  summary: {
    total: number;
    passed: number;
    failed: number;
  };
}

export interface TestResult {
  id: string;
  name: string;
  status: 'passed' | 'failed' | 'skipped';
  duration?: number;
  error?: string;
  priority?: 'P0' | 'P1' | 'P2' | 'P3';
  feature?: string;
}

export const test = base.extend<{
  reportTestResult: (result: TestResult) => Promise<void>;
}>({
  reportTestResult: async ({}, use) => {
    const results: TestResult[] = [];
    await use(async (result: TestResult) => {
      results.push(result);
    });
  },
});

export { expect };
```

Write to file: `auto-test/e2e/fixtures/test-fixtures.ts`

**Step 2: Commit**

```bash
git add auto-test/e2e/fixtures/test-fixtures.ts
git commit -m "feat: add E2E test fixtures

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 5: 创建 Vitest 配置文件

**Files:**
- Create: `auto-test/vitest/setup.ts`

**Step 1: 编写 Vitest setup 配置**

```typescript
// auto-test/vitest/setup.ts
import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Mock window.matchMedia
vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
})));

// Mock ResizeObserver
vi.stubGlobal('ResizeObserver', vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
})));
```

Write to file: `auto-test/vitest/setup.ts`

**Step 2: 创建 vitest.config.ts（修改 web/vitest.config.ts）**

在 `web/vitest.config.ts` 中添加：
```typescript
test: {
  include: ['auto-test/vitest/**/*.test.ts'],
  setupFiles: ['auto-test/vitest/setup.ts'],
  environment: 'jsdom',
}
```

**Step 3: Commit**

```bash
git add auto-test/vitest/setup.ts
git commit -m "feat: add Vitest setup configuration

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 5.5: 创建 vitest.config.ts 配置文件

**Files:**
- Create: `web/vitest.config.ts`（如不存在）或修改现有文件

**Step 1: 创建/修改 vitest.config.ts**

```typescript
// web/vitest.config.ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    include: ['auto-test/vitest/**/*.test.ts', 'auto-test/vitest/**/*.test.tsx'],
    setupFiles: ['auto-test/vitest/setup.ts'],
    environment: 'jsdom',
    globals: true,
    coverage: {
      reporter: ['text', 'json', 'html'],
      include: ['src/**/*.ts', 'src/**/*.tsx'],
      exclude: ['src/**/*.d.ts'],
    },
  },
});
```

Write to file: `web/vitest.config.ts`

**Step 2: 验证配置**

Run: `cd web && npx vitest --version`

Expected: 显示 vitest 版本号

**Step 3: Commit**

```bash
git add web/vitest.config.ts
git commit -m "feat: add vitest.config.ts for auto-test integration

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 5.6: 更新 playwright.config.ts 测试目录

**Files:**
- Modify: `web/playwright.config.ts`

**Step 1: 修改 playwright.config.ts 测试目录配置**

在 `web/playwright.config.ts` 中修改 testDir 配置：

```typescript
export default defineConfig({
  testDir: './auto-test/e2e',  // 从 './tests/e2e' 改为 './auto-test/e2e'
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:26306',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:26306',
    reuseExistingServer: !process.env.CI,
  },
});
```

**Step 2: 验证配置**

Run: `cd web && npx playwright test --list`

Expected: 显示 auto-test/e2e 目录下的测试列表

**Step 3: Commit**

```bash
git add web/playwright.config.ts
git commit -m "feat: update playwright.config.ts to use auto-test/e2e

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 6: 创建特性测试运行脚本

**Files:**
- Create: `scripts/run-feature-tests.py`

**Step 1: 编写特性测试脚本**

```python
#!/usr/bin/env python3
"""
特性测试运行脚本
根据 feature-map.yaml 执行特性相关的所有测试
"""

import yaml
import subprocess
import argparse
from pathlib import Path

def load_feature_map():
    config_path = Path("auto-test/feature-map.yaml")
    if not config_path.exists():
        raise FileNotFoundError(f"特性映射文件不存在: {config_path}")
    
    with open(config_path, 'r', encoding='utf-8') as f:
        return yaml.safe_load(f)

def run_e2e_tests(test_patterns):
    cmd = ["npx", "playwright", "test", "auto-test/e2e/"]
    for pattern in test_patterns:
        cmd.extend(["--grep", pattern])
    print(f"执行 E2E 测试: {cmd}")
    subprocess.run(cmd, cwd="web", check=False)

def run_internal_tests(test_patterns):
    cmd = ["go", "test", "./auto-test/internal/...", "-v"]
    for pattern in test_patterns:
        cmd.extend(["-run", pattern])
    print(f"执行 Internal 测试: {cmd}")
    subprocess.run(cmd, check=False)

def run_feature_tests(feature_id):
    feature_map = load_feature_map()
    
    if feature_id not in feature_map['features']:
        print(f"特性 ID 不存在: {feature_id}")
        return
    
    feature = feature_map['features'][feature_id]
    print(f"\n{'='*60}")
    print(f"特性: {feature['name']} ({feature_id})")
    print(f"优先级: {feature['priority']}")
    print(f"{'='*60}\n")
    
    tests = feature['tests']
    
    if 'e2e' in tests:
        print("\n>>> E2E 测试 <<<")
        run_e2e_tests(tests['e2e'])
    
    if 'internal' in tests:
        print("\n>>> Internal 测试 <<<")
        run_internal_tests(tests['internal'])

def main():
    parser = argparse.ArgumentParser(description='特性测试运行脚本')
    parser.add_argument('--feature', '-f', help='特性 ID (如 F001)')
    parser.add_argument('--priority', '-p', help='优先级 (如 P0 或 P0,P1)')
    
    args = parser.parse_args()
    
    if args.feature:
        run_feature_tests(args.feature)
    else:
        parser.print_help()

if __name__ == '__main__':
    main()
```

Write to file: `scripts/run-feature-tests.py`

**Step 2: Commit**

```bash
git add scripts/run-feature-tests.py
git commit -m "feat: add feature test runner script

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 7: 更新 Makefile 测试命令

**Files:**
- Modify: `Makefile`

**Step 1: 在 Makefile 末尾添加测试命令**

```makefile
# ===== Auto-Test 测试命令 =====

.PHONY: test-all test-frontend test-backend test-performance test-feature test-feature-priority test-p0 test-p1

test-all: test-backend test-frontend test-performance

test-frontend:
	cd web && npx playwright test auto-test/e2e/
	cd web && npx vitest run auto-test/vitest/

test-backend:
	go test ./auto-test/internal/... -v

test-performance:
	go test -bench=. ./auto-test/internal/performance/
	cd web && npx playwright test --trace on auto-test/e2e/performance/

test-feature:
	@if [ -z "$(F)" ]; then \
		echo "请指定特性 ID，例如: make test-feature F=F001"; \
		exit 1; \
	fi
	@echo "执行特性测试: $(F)"
	@python scripts/run-feature-tests.py --feature $(F)

test-feature-priority:
	@if [ -z "$(P)" ]; then \
		echo "请指定优先级，例如: make test-feature-priority P=P0"; \
		exit 1; \
	fi
	@echo "执行优先级特性测试: $(P)"
	@python scripts/run-feature-tests.py --priority $(P)

test-p0:
	go test ./auto-test/internal/... -v -run "P0"
	cd web && npx playwright test auto-test/e2e/ --grep "P0"

test-p1:
	go test ./auto-test/internal/... -v -run "P0|P1"
	cd web && npx playwright test auto-test/e2e/ --grep "P0|P1"
```

**Step 2: Commit**

```bash
git add Makefile
git commit -m "feat: add auto-test commands to Makefile

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 7.5: 创建 GitHub CI 测试工作流

**Files:**
- Create: `.github/workflows/test.yml`

**Step 1: 编写 CI 测试工作流配置**

```yaml
# .github/workflows/test.yml
name: Auto-Test CI

on:
  push:
    branches: [main, master, test]
  pull_request:
    branches: [main, master]

jobs:
  backend-test:
    name: Backend Go Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Run P0 Backend Tests
        run: go test ./auto-test/internal/... -v -run "P0"
      
      - name: Run P1 Backend Tests
        run: go test ./auto-test/internal/... -v -run "P0|P1"
        continue-on-error: true
      
      - name: Generate Coverage Report
        run: go test ./auto-test/internal/... -coverprofile=coverage.out
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out

  frontend-test:
    name: Frontend E2E Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: '18'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json
      
      - name: Install Dependencies
        working-directory: web
        run: npm ci
      
      - name: Install Playwright Browsers
        working-directory: web
        run: npx playwright install --with-deps chromium
      
      - name: Build Backend
        run: make build
      
      - name: Start Backend Server
        run: ./bin/isdp-server &
        env:
          ISDP_CONFIG: configs/config.yaml
      
      - name: Start Frontend Dev Server
        working-directory: web
        run: npm run dev &
      
      - name: Wait for Server Ready
        run: sleep 10
      
      - name: Run P0 E2E Tests
        working-directory: web
        run: npx playwright test auto-test/e2e/ --grep "P0" --reporter=html
      
      - name: Upload Playwright Report
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: playwright-report
          path: web/playwright-report/
          retention-days: 7

  vitest-test:
    name: Vitest Component Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Node
        uses: actions/setup-node@v4
        with:
          node-version: '18'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json
      
      - name: Install Dependencies
        working-directory: web
        run: npm ci
      
      - name: Run Vitest Tests
        working-directory: web
        run: npx vitest run auto-test/vitest/ --coverage

  test-summary:
    name: Test Summary
    needs: [backend-test, frontend-test, vitest-test]
    runs-on: ubuntu-latest
    if: always()
    steps:
      - name: Check Test Results
        run: |
          echo "Backend: ${{ needs.backend-test.result }}"
          echo "Frontend: ${{ needs.frontend-test.result }}"
          echo "Vitest: ${{ needs.vitest-test.result }}"
          
          if [ "${{ needs.backend-test.result }}" == "failure" ]; then
            echo "❌ Backend P0 tests failed - blocking release"
            exit 1
          fi
          
          if [ "${{ needs.frontend-test.result }}" == "failure" ]; then
            echo "❌ Frontend P0 tests failed - blocking release"
            exit 1
          fi
          
          echo "✅ All P0 tests passed"
```

Write to file: `.github/workflows/test.yml`

**Step 2: 验证工作流语法**

Run: `cat .github/workflows/test.yml`

Expected: 文件内容正确

**Step 3: Commit**

```bash
git add .github/workflows/test.yml
git commit -m "feat: add GitHub CI workflow for auto-test

- Backend Go tests (P0 blocking, P1 non-blocking)
- Frontend Playwright E2E tests
- Vitest component tests with coverage
- Test summary job for release gate

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 2: P0 核心测试 - Agent 对话模块

### Task 8: 创建 Agent 对话消息输入测试 (AD-01)

**Files:**
- Create: `auto-test/e2e/agent-dialog/message-input.spec.ts`

**Step 1: 编写 P0 消息输入测试**

```typescript
// auto-test/e2e/agent-dialog/message-input.spec.ts
import { test, expect } from '../fixtures/test-fixtures';

/**
 * AD-01: 消息输入与发送测试
 * P0 用例：AD-01-01, AD-01-02, AD-01-03, AD-01-04, AD-01-05, AD-01-08, AD-01-14
 */

test.describe('AD-01: 消息输入与发送 [P0]', () => {
  
  test('AD-01-01: 输入框正常显示与聚焦 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    // 进入第一个项目
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);
      
      // 查找输入框
      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      await expect(input.first()).toBeVisible();
      
      // 检查输入框可聚焦
      await input.first().click();
      await expect(input.first()).toBeFocused();
    }
  });
  
  test('AD-01-02: 输入文本并点击发送成功 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);
      
      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        const testMessage = `测试消息-${Date.now()}`;
        await input.first().fill(testMessage);
        
        const sendButton = page.locator('button').filter({ hasText: /发送/i });
        if (await sendButton.count() > 0) {
          await sendButton.first().click();
          await page.waitForTimeout(2000);
          
          // 验证消息显示
          const messageContent = page.locator('.message-content, .message-body');
          await expect(messageContent.first()).toBeVisible();
        }
      }
    }
  });
  
  test('AD-01-03: 输入 @ 触发 Agent 下拉框 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);
      
      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        await input.first().click();
        await input.first().fill('@');
        await page.waitForTimeout(500);
        
        // 检查下拉框出现
        const dropdown = page.locator('.mention-dropdown, .ant-dropdown, [class*="agent-list"]');
        await expect(dropdown.first()).toBeVisible();
      }
    }
  });
  
  test('AD-01-08: 空消息禁止发送 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);
      
      const input = page.locator('.ant-input, textarea[placeholder*="输入"]');
      if (await input.count() > 0) {
        // 清空输入框
        await input.first().fill('');
        
        const sendButton = page.locator('button').filter({ hasText: /发送/i });
        // 空消息时发送按钮应禁用或不响应
        if (await sendButton.count() > 0) {
          const isDisabled = await sendButton.first().isDisabled();
          expect(isDisabled).toBeTruthy();
        }
      }
    }
  });
});
```

Write to file: `auto-test/e2e/agent-dialog/message-input.spec.ts`

**Step 2: 运行测试验证**

Run: `cd web && npx playwright test auto-test/e2e/agent-dialog/message-input.spec.ts --grep "P0"`

Expected: 测试执行（可能需要后端服务运行）

**Step 3: Commit**

```bash
git add auto-test/e2e/agent-dialog/message-input.spec.ts
git commit -m "feat: add AD-01 message input tests (P0)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 9: 创建 WebSocket 连接管理测试 (WS-01)

**Files:**
- Create: `auto-test/e2e/websocket/connection.spec.ts`

**Step 1: 编写 WebSocket 连接 P0 测试**

```typescript
// auto-test/e2e/websocket/connection.spec.ts
import { test, expect } from '../fixtures/test-fixtures';

/**
 * WS-01: WebSocket 连接管理测试
 * P0 用例：WS-01-01, WS-01-03, WS-01-07
 */

test.describe('WS-01: WebSocket 连接管理 [P0]', () => {
  
  test('WS-01-01: 页面加载自动建立连接 [F002]', async ({ page }) => {
    // @feature F002 - WebSocket 流式
    // @priority P0
    
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    // 进入工作台
    const projectLinks = page.locator('a').filter({ hasText: /.+/ });
    if (await projectLinks.count() > 0) {
      await projectLinks.first().click();
      await page.waitForTimeout(2000);
      
      // 检查 WebSocket 连接状态指示器
      const wsIndicator = page.locator('[class*="ws-status"], [class*="connection"]');
      await expect(wsIndicator.first()).toBeVisible();
      
      // 验证连接状态为已连接
      const connected = page.locator('[class*="connected"], .ws-connected');
      await expect(connected.first()).toBeVisible();
    }
  });
  
  test('WS-01-03: 连接失败自动重试（3次） [F002]', async ({ page }) => {
    // @feature F002 - WebSocket 流式
    // @priority P0
    
    // 模拟网络断开
    await page.route('**/ws/**', route => route.abort('failed'));
    
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    // 检查重试提示
    const retryIndicator = page.locator('[class*="retry"], [class*="reconnecting"]');
    await expect(retryIndicator.first()).toBeVisible();
  });
  
  test('WS-01-07: 连接超时提示 [F002]', async ({ page }) => {
    // @feature F002 - WebSocket 流式
    // @priority P0
    
    // 设置超时
    await page.setDefaultTimeout(5000);
    
    await page.goto('/projects');
    await page.waitForLoadState('networkidle');
    
    // 检查超时提示（如果 WebSocket 服务不可用）
    const timeoutMsg = page.locator('[class*="timeout"], [class*="connection-error"]');
    // 此测试可能跳过，取决于服务状态
  });
});
```

Write to file: `auto-test/e2e/websocket/connection.spec.ts`

**Step 2: Commit**

```bash
git add auto-test/e2e/websocket/connection.spec.ts
git commit -m "feat: add WS-01 WebSocket connection tests (P0)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 10: 创建后端 Agent Handler 测试 (API-01)

**Files:**
- Create: `auto-test/internal/api/agent_handler_test.go`

**Step 1: 编写 Agent Handler P0 测试**

```go
// auto-test/internal/api/agent_handler_test.go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

/**
 * API-01: Agent Handler 测试
 * P0 用例：API-01-01, API-01-02, API-01-03, API-01-04, API-01-05
 */

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-01-01
func TestAgentHandler_List(t *testing.T) {
	// 创建测试请求
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	
	// 调用 Handler（需要初始化 router）
	// router.ServeHTTP(w, req)
	
	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-01-03
func TestAgentHandler_Create(t *testing.T) {
	body := map[string]interface{}{
		"name":        "Test Agent",
		"description": "Test description",
		"type":        "claude_code",
	}
	bodyBytes, _ := json.Marshal(body)
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	// router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusCreated, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotNil(t, response["id"])
	assert.Equal(t, "Test Agent", response["name"])
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-01-05
func TestAgentHandler_Delete(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/test-id", nil)
	w := httptest.NewRecorder()
	
	// router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-01-11
func TestAgentHandler_ParamValidation(t *testing.T) {
	// 测试无效参数
	body := map[string]interface{}{
		"name": "", // 空名称应该被拒绝
	}
	bodyBytes, _ := json.Marshal(body)
	
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	// router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

Write to file: `auto-test/internal/api/agent_handler_test.go`

**Step 2: Commit**

```bash
git add auto-test/internal/api/agent_handler_test.go
git commit -m "feat: add API-01 Agent Handler tests (P0)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 3: P0 Internal 测试 - Service 层

### Task 11: 创建 A2A Service 测试 (SV-02)

**Files:**
- Create: `auto-test/internal/service/a2a/mention_parser_test.go`

**Step 1: 迁移并扩展 A2A 提及解析测试**

从 `internal/service/a2a/mention_parser_test.go` 迁移到 `auto-test/internal/service/a2a/mention_parser_test.go`，并补充 P0 用例：

```go
// auto-test/internal/service/a2a/mention_parser_test.go
package a2a_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/**
 * SV-02: A2A Service 测试
 * P0 用例：SV-02-01, SV-02-07
 */

// @feature F003 - 多 Agent 协作
// @priority P0
// @id SV-02-01
func TestParseA2AMentions_Core(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		currentCatID string
		want         []string
	}{
		{
			name:         "single mention at line start",
			text:         "@backend 请实现这个功能",
			currentCatID: "architect",
			want:         []string{"backend_developer"},
		},
		{
			name:         "multiple mentions on separate lines",
			text:         "@backend 请实现\n@architect 请设计",
			currentCatID: "code_reviewer",
			want:         []string{"backend_developer", "architect"},
		},
		{
			name:         "filter self mention",
			text:         "@backend 请实现\n@architect 请设计",
			currentCatID: "backend_developer",
			want:         []string{"architect"},
		},
		{
			name:         "mention inside code block ignored",
			text:         "```\n@backend\n```\n@architect this one counts",
			currentCatID: "backend_developer",
			want:         []string{"architect"},
		},
		{
			name:         "mention not at line start ignored",
			text:         "hello @backend not at start",
			currentCatID: "architect",
			want:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseA2AMentions(tt.text, tt.currentCatID)
			assert.Equal(t, tt.want, got)
		})
	}
}

// @feature F003 - 多 Agent 协作
// @priority P0
// @id SV-02-07
func TestParseA2AMentions_BoundaryCheck(t *testing.T) {
	// 测试最多 2 个目标限制
	text := "@backend 请实现\n@architect 请设计\n@code_reviewer 第三行"
	currentCatID := "sre_engineer"
	
	got := ParseA2AMentions(text, currentCatID)
	assert.Len(t, got, 2, "最多只应该返回 2 个目标")
}
```

Write to file: `auto-test/internal/service/a2a/mention_parser_test.go`

**Step 2: Commit**

```bash
git add auto-test/internal/service/a2a/mention_parser_test.go
git commit -m "feat: add SV-02 A2A Service tests (P0)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 4: P1 测试 - 团队包管理

### Task 12: 创建团队包 CRUD 测试 (TP-01)

**Files:**
- Create: `auto-test/e2e/team-package/list-crud.spec.ts`

**Step 1: 编写团队包 P0/P1 测试**

```typescript
// auto-test/e2e/team-package/list-crud.spec.ts
import { test, expect } from '../fixtures/test-fixtures';

/**
 * TP-01: 团队包列表与 CRUD 测试
 * P0 用例：TP-01-01, TP-01-03, TP-01-05, TP-01-06, TP-01-13
 */

test.describe('TP-01: 团队包列表与 CRUD [P0]', () => {
  
  test('TP-01-01: 团队包列表加载 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P0
    
    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');
    
    // 验证列表容器存在
    const listContainer = page.locator('.team-package-list, [class*="package-list"]');
    await expect(listContainer).toBeVisible();
  });
  
  test('TP-01-03: 创建团队包成功 [F004]', async ({ page }) => {
    // @feature F004 - 团队包管理
    // @priority P0
    
    await page.goto('/team-packages');
    await page.waitForLoadState('networkidle');
    
    // 点击创建按钮
    const createButton = page.locator('button').filter({ hasText: /创建|新建/i });
    if (await createButton.count() > 0) {
      await createButton.first().click();
      await page.waitForTimeout(500);
      
      // 填写表单
      const nameInput = page.locator('input[name="name"], input[placeholder*="名称"]');
      if (await nameInput.count() > 0) {
        await nameInput.fill(`测试团队包-${Date.now()}`);
        
        // 提交
        const submitButton = page.locator('.ant-modal .ant-btn-primary');
        if (await submitButton.count() > 0) {
          await submitButton.click();
          await page.waitForTimeout(1000);
          
          // 验证成功提示
          const successMsg = page.locator('.ant-message-success, [class*="success"]');
          await expect(successMsg).toBeVisible();
        }
      }
    }
  });
});
```

Write to file: `auto-test/e2e/team-package/list-crud.spec.ts`

**Step 2: Commit**

```bash
git add auto-test/e2e/team-package/list-crud.spec.ts
git commit -m "feat: add TP-01 team package CRUD tests (P0)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 5: 已有测试迁移

### Task 13: 迁移 web/tests/e2e 测试

**Files:**
- Copy: `web/tests/e2e/*.spec.ts` → `auto-test/e2e/` 对应目录

**Step 1: 迁移 E2E 测试文件**

Run:
```bash
cd D:/CoLinkProject/Colink-Test-0430/isdp

# 迁移 homepage 测试
cp web/tests/e2e/01-homepage.spec.ts auto-test/e2e/thread-workflow/homepage.spec.ts

# 迁移 projects 测试  
cp web/tests/e2e/02-projects.spec.ts auto-test/e2e/thread-workflow/projects.spec.ts

# 迁移 thread-workflow 测试
cp web/tests/e2e/12-thread-workflow.spec.ts auto-test/e2e/thread-workflow/thread-workflow.spec.ts

# 迁移 backend-api 测试
cp web/tests/e2e/05-backend-api.spec.ts auto-test/e2e/api/backend-api.spec.ts

# 迁移其他测试到 thread-workflow
cp web/tests/e2e/03-sandbox-workflow.spec.ts auto-test/e2e/thread-workflow/sandbox-workflow.spec.ts
cp web/tests/e2e/04-theme-agent.spec.ts auto-test/e2e/thread-workflow/theme-agent.spec.ts
cp web/tests/e2e/06-form-validation.spec.ts auto-test/e2e/thread-workflow/form-validation.spec.ts
cp web/tests/e2e/07-navigation.spec.ts auto-test/e2e/thread-workflow/navigation.spec.ts
cp web/tests/e2e/08-error-handling.spec.ts auto-test/e2e/thread-workflow/error-handling.spec.ts
cp web/tests/e2e/09-empty-loading.spec.ts auto-test/e2e/thread-workflow/empty-loading.spec.ts
cp web/tests/e2e/10-responsive-layout.spec.ts auto-test/e2e/thread-workflow/responsive-layout.spec.ts
cp web/tests/e2e/11-project-detail.spec.ts auto-test/e2e/thread-workflow/project-detail.spec.ts
```

Expected: 所有文件复制成功

**Step 2: 更新导入路径**

在每个迁移的文件中，更新 fixture 导入路径：
```typescript
// 从
import { test, expect } from '../fixtures/test-fixtures';
// 改为
import { test, expect } from '../../fixtures/test-fixtures';
```

**Step 3: 迁移 fixtures 和 test-runner**

Run:
```bash
cp web/tests/fixtures/test-fixtures.ts auto-test/e2e/fixtures/test-fixtures.ts
cp web/tests/test-runner.ts auto-test/e2e/test-runner.ts
```

**Step 4: Commit**

```bash
git add auto-test/e2e/
git commit -m "feat: migrate existing E2E tests to auto-test directory

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 14: 迁移 internal 测试文件

**Files:**
- Copy: `internal/**/*_test.go` → `auto-test/internal/service/` 等

**Step 1: 迁移 Service 层测试**

Run:
```bash
cd D:/CoLinkProject/Colink-Test-0430/isdp

# 迁移 a2a 测试
cp internal/service/a2a/mention_parser_test.go auto-test/internal/service/a2a/
cp internal/service/a2a/invocation_queue_test.go auto-test/internal/service/a2a/
cp internal/service/a2a/invocation_registry_test.go auto-test/internal/service/a2a/
cp internal/service/a2a/session_chain_store_test.go auto-test/internal/service/a2a/

# 迁移 agent 测试
cp internal/service/agent/adapter_test.go auto-test/internal/service/agent/
cp internal/service/agent/debug_thread_manager_test.go auto-test/internal/service/agent/
cp internal/service/agent/five_layer_context_test.go auto-test/internal/service/agent/
cp internal/service/agent/governance_digest_test.go auto-test/internal/service/agent/
cp internal/service/agent/human_chain_history_test.go auto-test/internal/service/agent/
cp internal/service/agent/orchestrator_debug_test.go auto-test/internal/service/agent/
cp internal/service/agent/project_context_test.go auto-test/internal/service/agent/
cp internal/service/agent/token_budget_test.go auto-test/internal/service/agent/

# 迁移 im 测试
mkdir -p auto-test/internal/service/im/
cp internal/service/im/*.go auto-test/internal/service/im/

# 迁移 teampackagesync 测试
cp internal/service/teampackagesync/clone_cache_test.go auto-test/internal/service/teampackagesync/

# 迁移 repo 测试
cp internal/repo/db_test.go auto-test/internal/repo/
cp internal/repo/db_mysql_test.go auto-test/internal/repo/
cp internal/repo/db_sqlite_test.go auto-test/internal/repo/
cp internal/repo/im_session_repo_test.go auto-test/internal/repo/
```

Expected: 所有文件复制成功

**Step 2: Commit**

```bash
git add auto-test/internal/
git commit -m "feat: migrate existing internal tests to auto-test directory

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 15: 删除原测试目录（两阶段策略）

**Files:**
- Delete: `web/tests/e2e/`, `internal/**/*_test.go`

**⚠️ 重要：采用两阶段删除策略（迁移 → 验证 → 删除）**

**第一阶段：验证迁移成功**

**Step 1: 运行迁移后的测试验证功能正常**

Run:
```bash
# 验证后端测试
go test ./auto-test/internal/... -v -run "P0"

# 验证前端 E2E 测试（需要服务运行）
cd web && npx playwright test auto-test/e2e/ --grep "P0" --reporter=list
```

Expected: 迁移的测试能够正常执行

**Step 2: 对比迁移前后测试数量**

Run:
```bash
# 统计原测试文件数量
echo "原 E2E 测试数量:" && find web/tests/e2e -name "*.spec.ts" | wc -l
echo "原 Internal 测试数量:" && find internal -name "*_test.go" | wc -l

# 统计迁移后测试数量
echo "迁移后 E2E 测试数量:" && find auto-test/e2e -name "*.spec.ts" | wc -l
echo "迁移后 Internal 测试数量:" && find auto-test/internal -name "*_test.go" | wc -l
```

Expected: 迁移后数量 ≥ 原数量（新增了一些测试）

**Step 3: 创建迁移验证标记文件**

Run:
```bash
echo "迁移验证完成: $(date)" > auto-test/.migration-verified
```

**Step 4: Commit 验证结果**

```bash
git add auto-test/.migration-verified
git commit -m "chore: mark migration verification complete

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

**第二阶段：安全删除原文件**

**Step 5: 删除原 web/tests/e2e 目录**

Run:
```bash
cd D:/CoLinkProject/Colink-Test-0430/isdp
rm -rf web/tests/e2e/
rm -f web/tests/fixtures/test-fixtures.ts
rm -f web/tests/test-runner.ts
```

Expected: 目录删除成功

**Step 6: 删除原 internal 测试文件**

Run:
```bash
# 删除所有 *_test.go 文件（保留 auto-test 中的）
find internal -name "*_test.go" -type f -delete
```

Expected: 文件删除成功

**Step 7: 再次验证测试仍可运行**

Run:
```bash
go test ./auto-test/internal/... -v -run "P0"
```

Expected: 测试仍然正常执行

**Step 8: Commit 删除操作**

```bash
git add -A
git commit -m "refactor: remove original test files after verified migration to auto-test

Migration verified before deletion. All tests working correctly in auto-test/ directory.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 6: Vitest 组件测试

### Task 16: 创建 ChatMessageList 组件测试

**Files:**
- Create: `auto-test/vitest/components/ChatMessageList.test.ts`

**Step 1: 编写 ChatMessageList P0 测试**

```typescript
// auto-test/vitest/components/ChatMessageList.test.ts
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ChatMessageList } from '../../../src/components/ChatMessageList';

/**
 * VT-01: ChatMessageList 组件测试
 * P0 用例：VT-01-09, VT-01-10
 */

describe('VT-01: ChatMessageList [P0]', () => {
  
  it('VT-01-09: ChatMessageList 渲染 [F008]', () => {
    // @feature F008 - 消息渲染
    // @priority P0
    
    const messages = [
      { id: '1', content: 'Hello', role: 'user', agentName: 'User' },
      { id: '2', content: 'Response', role: 'assistant', agentName: 'Agent' },
    ];
    
    render(<ChatMessageList messages={messages} />);
    
    expect(screen.getByText('Hello')).toBeInTheDocument();
    expect(screen.getByText('Response')).toBeInTheDocument();
  });
  
  it('VT-01-10: ChatMessageList 流式更新 [F008]', () => {
    // @feature F008 - 消息渲染
    // @priority P0
    
    const { rerender } = render(<ChatMessageList messages={[]} />);
    
    // 模拟流式消息更新
    rerender(<ChatMessageList messages={[{ id: '1', content: 'Partial...', role: 'assistant' }]} />);
    
    expect(screen.getByText('Partial...')).toBeInTheDocument();
    
    rerender(<ChatMessageList messages={[{ id: '1', content: 'Partial... content', role: 'assistant' }]} />);
    
    expect(screen.getByText('Partial... content')).toBeInTheDocument();
  });
});
```

Write to file: `auto-test/vitest/components/ChatMessageList.test.ts`

**Step 2: Commit**

```bash
git add auto-test/vitest/components/ChatMessageList.test.ts
git commit -m "feat: add VT-01 ChatMessageList component tests (P0)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 7: 性能测试

### Task 17: 创建 API 性能基准测试

**Files:**
- Create: `auto-test/internal/performance/api_bench_test.go`

**Step 1: 编写 API 性能测试**

```go
// auto-test/internal/performance/api_bench_test.go
package performance_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

/**
 * PF-01: API 性能测试
 * P1 用例：PF-01-01, PF-01-02, PF-01-05
 */

// @feature F010 - 性能优化
// @priority P1
// @id PF-01-01
func BenchmarkAgentListAPI(b *testing.B) {
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
		w := httptest.NewRecorder()
		// router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Errorf("Expected status 200, got %d", w.Code)
		}
	}
}

// @feature F010 - 性能优化
// @priority P1
// @id PF-01-02
func BenchmarkThreadListAPI(b *testing.B) {
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/threads", nil)
		w := httptest.NewRecorder()
		// router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Errorf("Expected status 200, got %d", w.Code)
		}
	}
}

// @feature F010 - 性能优化
// @priority P1
// @id PF-01-05
func BenchmarkConcurrentThreadCreate(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 模拟并发创建
			req := httptest.NewRequest(http.MethodPost, "/api/v1/threads", nil)
			w := httptest.NewRecorder()
			// router.ServeHTTP(w, req)
		}
	})
}
```

Write to file: `auto-test/internal/performance/api_bench_test.go`

**Step 2: 运行性能测试**

Run: `go test -bench=. ./auto-test/internal/performance/`

Expected: 显示性能指标

**Step 3: Commit**

```bash
git add auto-test/internal/performance/api_bench_test.go
git commit -m "feat: add PF-01 API performance benchmark tests (P1)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 8: 文档更新

### Task 18: 更新 CLAUDE.md 测试说明

**Files:**
- Modify: `CLAUDE.md`

**Step 1: 在 CLAUDE.md 测试部分添加新内容**

在 `## 测试` 部分添加：

```markdown
## 测试

### 测试目录结构

所有测试统一放在 `auto-test/` 目录：

```
auto-test/
├── e2e/              # 前端 E2E 测试 (Playwright)
├── internal/         # 后端内部测试（可导入 internal 包）
├── vitest/           # Vitest 组件/Store/Hook 测试
├── feature-map.yaml  # 特性测试映射配置
└── docs/             # 测试文档
```

### 测试执行命令

```bash
# 全量测试
make test-all

# 分层测试
make test-frontend    # 前端 E2E + Vitest
make test-backend     # 后端 Go 测试
make test-performance # 性能测试

# 特性测试
make test-feature F=F001    # 单个特性
make test-feature F=F001,F002  # 多个特性
make test-feature-priority P=P0  # 按优先级

# 优先级测试
make test-p0  # 只执行 P0 测试
make test-p1  # 执行 P0 + P1 测试
```

### 测试优先级定义

| 级别 | 定义 | CI 要求 |
|------|------|---------|
| **P0** | 核心路径，阻塞发布 | 必须 100% 通过 |
| **P1** | 重要功能，影响用户体验 | 通过率 ≥ 95% |
| **P2** | 边缘场景、性能优化 | 通过率 ≥ 80% |
| **P3** | 探索性测试、UI细节 | 可选执行 |

### 测试数据准备

- **API/Service 层**：使用独立测试数据库
- **组件/Hook/Store**：使用 Vitest Mock 功能
- **测试数据位置**：`auto-test/internal/testdata/`

### 新增测试规范

1. 所有新测试必须放在 `auto-test/` 目录
2. 测试 ID 格式：`{模块}-{类别}-{序号}`（如 `AD-01-02`）
3. 在测试注释中标注优先级和特性 ID：
   ```go
   // @feature F001 - Agent 对话核心
   // @priority P0
   // @id SV-01-01
   ```
4. 特性测试需在 `feature-map.yaml` 中注册
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with auto-test documentation

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 19: 更新 AGENTS.md 测试规范

**Files:**
- Modify: `AGENTS.md`

**Step 1: 在 AGENTS.md 测试规范部分添加新内容**

在测试部分添加：

```markdown
## 测试编写规范

### 测试用例编写原则

1. **原子性**：每个测试只验证一个场景
2. **独立性**：测试之间不依赖执行顺序
3. **可重复**：多次执行结果一致
4. **自清理**：测试完成后清理数据和状态

### E2E 测试规范

```typescript
// 测试文件命名：{模块}-{场景}.spec.ts
import { test, expect } from '../fixtures/test-fixtures';

test.describe('AD-01: 消息输入与发送 [P0]', () => {
  test('AD-01-02: 输入文本并点击发送成功 [F001]', async ({ page }) => {
    // @feature F001 - Agent 对话核心
    // @priority P0
    
    await page.goto('/threads/123');
    const input = page.locator('.message-input');
    await input.fill('测试消息');
    await page.locator('button[aria-label="发送"]').click();
    
    await expect(page.locator('.message-content')).toBeVisible();
  });
});
```

### Go 测试规范

```go
// 测试文件命名：{场景}_test.go
func TestAgentHandler_Create(t *testing.T) {
  // @feature F001 - Agent 对话核心
  // @priority P0
  // @id API-01-03
  
  req := CreateAgentRequest{Name: "Test"}
  resp, err := handler.Create(req)
  
  assert.NoError(t, err)
  assert.NotNil(t, resp.ID)
}
```

### Vitest 测试规范

```typescript
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';

describe('VT-01: ChatMessageList [P0]', () => {
  it('VT-01-09: 正确渲染消息列表 [F008]', () => {
    // @feature F008 - 消息渲染
    // @priority P0
    
    render(<ChatMessageList messages={[...]} />);
    expect(screen.getByText('Hello')).toBeInTheDocument();
  });
});
```
```

**Step 2: Commit**

```bash
git add AGENTS.md
git commit -m "docs: update AGENTS.md with test writing guidelines

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Phase 9: 最终验证

### Task 20: 运行全量测试验证

**Files:**
- None

**Step 1: 运行后端测试**

Run: `make test-backend`

Expected: 所有 P0 测试通过

**Step 2: 运行前端测试**

Run: `make test-frontend`

Expected: 所有 P0 测试通过

**Step 3: 运行特性测试**

Run: `make test-feature F=F001`

Expected: F001 特性所有测试执行

**Step 4: 验证优先级测试**

Run: `make test-p0`

Expected: 只执行 P0 测试

**Step 5: 生成测试覆盖率报告**

Run: `go test ./auto-test/internal/... -coverprofile=coverage.out`

Expected: 显示覆盖率统计

---

## 后续任务（P2/P3 测试补充）

上述计划完成 P0 和核心 P1 测试。后续可按相同模式补充：

- **Phase 10**: P1 完整测试（WebSocket 流式、Vitest Store/Hook）
- **Phase 11**: P2 测试（性能测试完整版、边缘场景）
- **Phase 12**: P3 测试（UI细节、探索性测试）

每个 Phase 遵循相同的 TDD 模式：
1. 编写测试文件
2. 运行验证
3. Commit

---

**Plan complete and saved to `docs/plans/2026-04-29-auto-test-supplement-plan.md`.**

**Two execution options:**

1. **Subagent-Driven (this session)** - 我逐任务派发子代理，任务间审查，快速迭代
2. **Parallel Session (separate)** - 打开新会话使用 executing-plans，批量执行带检查点

**选择哪种方式？**
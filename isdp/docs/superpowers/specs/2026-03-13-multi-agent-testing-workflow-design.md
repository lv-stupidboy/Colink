# 多 Agent 协作测试开发工作流设计文档

**日期：** 2026-03-13
**版本：** 1.0
**状态：** 已批准

---

## 1. 概述

### 1.1 目标
建立一个多 Agent 协作的测试开发工作流，实现前端/后端自动化测试与修复的闭环流程。

### 1.2 范围
- 前端功能自动化测试（Playwright）
- 后端 API 测试
- 问题自动定位与 Agent 调度
- 用户确认机制

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    调度中心 (Coordinator)                 │
│  - 管理 4 个 Agent 角色                                   │
│  - 解析测试结果，定位问题归属 (前端/后端)                 │
│  - 生成测试报告，请求用户确认                             │
│  - 切换角色执行修复                                       │
└─────────────────────────────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│ 前端测试 Agent │ │ 后端测试 Agent │ │ 开发 Agent    │
│ (Playwright)  │ │ (API 测试)     │ │ (前端/后端)   │
├───────────────┤ ├───────────────┤ ├───────────────┤
│ - 页面加载    │ │ - API 调用     │ │ - 代码分析    │
│ - 点击测试    │ │ - 数据验证    │ │ - Bug 修复     │
│ - 表单验证    │ │ - 性能检查    │ │ - 自测验证    │
│ - UI 检查     │ │ - 日志分析    │ │ - 提交说明    │
└───────────────┘ └───────────────┘ └───────────────┘
```

### 2.2 Agent 角色定义

| 角色 | 名称 | 触发条件 | 职责 |
|------|------|----------|------|
| **前端测试** | `frontend-tester` | 用户启动测试 | 使用 Playwright 测试页面功能，生成测试报告 |
| **后端测试** | `backend-tester` | 前端测试发现 API 问题 | 测试 API 接口，分析后端日志 |
| **前端开发** | `frontend-dev` | 确认是前端 Bug | 分析代码，修复 Bug，自测 |
| **后端开发** | `backend-dev` | 确认是后端 Bug | 分析代码，修复 Bug，自测 |

---

## 3. 测试用例设计

### 3.1 前端核心功能测试

| ID | 测试项 | 测试步骤 | 预期结果 |
|----|--------|----------|----------|
| FT-01 | 首页加载 | 访问 `/` | Dashboard 正常显示，统计卡片可见 |
| FT-02 | 项目空间导航 | 点击"项目空间"菜单 | 正确跳转到 `/projects` |
| FT-03 | 创建项目 | 点击创建按钮 | 弹窗显示，表单可提交 |
| FT-04 | 项目详情 | 点击项目卡片 | 进入项目详情页 |
| FT-05 | 沙箱页面 | 访问 `/sandbox` | 沙箱列表加载，启动/停止按钮可用 |
| FT-06 | 工作流页面 | 访问 `/workflow` | 模板卡片显示，可选择模板 |
| FT-07 | 主题样式 | 检查页面颜色 | 绿色主题正确应用 |
| FT-08 | Agent 提及 | 输入框输入 `@` | 触发 Agent 选择下拉框 |

### 3.2 后端 API 测试

| ID | 测试项 | 端点 | 预期结果 |
|----|--------|------|----------|
| BT-01 | 健康检查 | GET /api/health | 200 OK |
| BT-02 | 项目列表 | GET /api/projects | 返回项目列表 |
| BT-03 | 创建项目 | POST /api/projects | 201 Created |
| BT-04 | 线程列表 | GET /api/threads | 返回线程列表 |
| BT-05 | 沙箱状态 | GET /api/sandbox/:id/status | 返回沙箱状态 |

---

## 4. 问题定位规则

```
问题类型判断：
├─ 页面渲染问题 → 前端 (样式错乱、组件不显示)
├─ 交互无响应 → 前端 (点击没反应、表单不提交)
├─ API 返回错误 → 后端 (500/404/超时)
├─ 数据显示异常 → 判断：
│   ├─ 前端解析错误 → 前端
│   └─ 后端数据错误 → 后端
└─ 网络请求失败 → 判断：
    ├─ 请求参数错误 → 前端
    └─ 服务端异常 → 后端
```

### 4.1 判定逻辑

```typescript
function identifyIssueType(testResult: TestResult): 'frontend' | 'backend' {
  if (testResult.type === 'ui-render-error') return 'frontend';
  if (testResult.type === 'interaction-error') return 'frontend';
  if (testResult.type === 'api-error') {
    if (testResult.status >= 500) return 'backend';
    if (testResult.status === 404) return 'backend';
    if (testResult.status === 400) return 'frontend'; // 参数错误
  }
  if (testResult.type === 'network-error') {
    if (testResult.error === 'timeout') return 'backend';
    if (testResult.error === 'connection-refused') return 'backend';
  }
  return 'frontend'; // 默认前端
}
```

---

## 5. 用户确认机制

### 5.1 确认节点

```
测试开始
   │
   ▼
自动执行测试 ─────▶ 发现问题？─┬─ 否 ─▶ 测试通过 ✅
                             │
                             │ 是
                             ▼
                    ┌────────────────┐
                    │ 【确认节点 1】   │ ◀── 用户确认
                    │ 是否执行修复？  │
                    └────────────────┘
                             │
                             ▼ 确认执行
                    调度对应 Agent (前端/后端开发)
                             │
                             ▼
                    ┌────────────────┐
                    │ 【确认节点 2】   │ ◀── 用户确认
                    │ 修复方案是否 OK？│
                    └────────────────┘
                             │
                             ▼ 确认
                    执行代码修改
                             │
                             ▼
                    重新运行测试验证
```

### 5.2 确认信息格式

```markdown
## 🐛 问题报告

**测试 ID:** FT-03
**问题类型:** 前端 Bug
**描述:** 创建项目按钮点击无响应

**错误详情:**
```
Error: Click handler not defined
at ProjectCreateButton (src/components/ProjectCreateButton.tsx:45)
```

**修复方案:**
- 文件：`src/components/ProjectCreateButton.tsx`
- 行号：45
- 修改：添加 onClick 事件处理

---

是否执行此修复？
[确认] [取消] [查看详情]
```

---

## 6. 技术栈

### 6.1 前端测试
- **Playwright** - 浏览器自动化
- **TypeScript** - 测试脚本
- **Vitest** - 测试运行器

### 6.2 后端测试
- **Go testing** - 原生测试框架
- **httptest** - HTTP 测试
- **testify** - 断言库

### 6.3 调度中心
- **Claude Code Agent** - 角色切换与调度
- **Task List** - 任务跟踪

---

## 7. 目录结构

```
isdp/
├── docs/superpowers/specs/
│   └── 2026-03-13-multi-agent-testing-workflow-design.md
├── web/
│   └── tests/
│       ├── e2e/           # Playwright 端到端测试
│       │   ├── homepage.spec.ts
│       │   ├── projects.spec.ts
│       │   ├── sandbox.spec.ts
│       │   └── workflow.spec.ts
│       ├── fixtures/      # 测试夹具
│       └── playwright.config.ts
├── server/
│   └── tests/
│       └── api/           # API 测试
│           ├── health_test.go
│           ├── projects_test.go
│           └── sandbox_test.go
└── scripts/
    └── test-runner.ts     # 测试调度脚本
```

---

## 8. 验收标准

### 8.1 功能验收
- [ ] 8 个前端测试用例全部通过
- [ ] 5 个后端 API 测试全部通过
- [ ] 问题自动定位准确率 > 90%
- [ ] 用户确认机制正常工作

### 8.2 流程验收
- [ ] 测试报告清晰易懂
- [ ] 修复方案经过用户确认
- [ ] 修复后自动重新测试
- [ ] 所有操作可追溯

---

## 9. 附录

### 9.1 Playwright 配置示例

```typescript
// playwright.config.ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  use: {
    baseURL: 'http://localhost:5173',
    headless: false,
    screenshot: 'only-on-failure',
  },
  reporter: [['html'], ['json', { outputFile: 'test-results.json' }]],
});
```

### 9.2 测试报告格式

```json
{
  "timestamp": "2026-03-13T10:30:00Z",
  "tests": [
    {
      "id": "FT-01",
      "name": "首页加载",
      "status": "passed",
      "duration": 1234
    },
    {
      "id": "FT-03",
      "name": "创建项目",
      "status": "failed",
      "error": "Click handler not defined",
      "issueType": "frontend"
    }
  ],
  "summary": {
    "total": 8,
    "passed": 7,
    "failed": 1
  }
}
```

---

*文档结束*

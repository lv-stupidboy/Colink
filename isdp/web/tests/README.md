# ISDP 多 Agent 协作测试系统

## 📋 快速开始

### 1. 启动开发服务器

```bash
cd D:\Tools\ASDP\isdp\web
npm run dev
```

### 2. 运行测试

```bash
# 运行所有测试并生成报告
npm run test

# 仅生成报告（不运行测试）
npm run test:report

# 直接使用 Playwright
npm run test:e2e           # 无头模式
npm run test:e2e:ui        # UI 模式
npm run test:e2e:headed    # 有头模式（可见浏览器）
```

---

## 🎯 测试用例

| ID | 测试项 | 描述 |
|----|--------|------|
| FT-01 | 首页加载 | Dashboard 正常显示 |
| FT-02 | 项目空间导航 | 菜单跳转正确 |
| FT-03 | 创建项目 | 弹窗和表单可用 |
| FT-04 | 项目详情 | 卡片点击跳转 |
| FT-05 | 沙箱页面 | 列表和按钮可用 |
| FT-06 | 工作流页面 | 模板显示和选择 |
| FT-07 | 主题样式 | 翡翠绿主题应用 |
| FT-08 | Agent 提及 | @触发 Agent 选择 |

---

## 🤖 Agent 角色

### 测试流程

```
┌──────────────┐
│ 前端测试 Agent │ 运行 Playwright 测试
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  生成测试报告  │ 解析结果，定位问题
└──────┬───────┘
       │
       ▼
┌──────────────┐     是
│  发现问题？    │──────▶ 请求用户确认
└──────┬───────┘
       │ 否
       ▼
  ✅ 测试通过
```

### 问题定位

- **前端问题**: 选择器失败、元素不可见、点击超时
- **后端问题**: API 500/404/超时、网络错误

---

## 📁 目录结构

```
web/
├── tests/
│   ├── e2e/                    # Playwright 测试用例
│   │   ├── 01-homepage.spec.ts
│   │   ├── 02-projects.spec.ts
│   │   ├── 03-sandbox-workflow.spec.ts
│   │   └── 04-theme-agent.spec.ts
│   ├── fixtures/
│   │   └── test-fixtures.ts    # 测试夹具和工具
│   ├── test-runner.ts          # 测试运行器
│   └── test-report.json        # 测试报告
├── playwright.config.ts        # Playwright 配置
└── package.json
```

---

## 🔧 配置说明

### playwright.config.ts

```typescript
{
  testDir: './e2e',
  timeout: 30000,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
    screenshot: 'only-on-failure',
  },
}
```

---

## 📊 测试报告格式

```json
{
  "timestamp": "2026-03-13T10:30:00Z",
  "tests": [
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

## 🐛 调试技巧

### 1. 有头模式调试

```bash
npm run test:e2e:headed
```

### 2. 单个测试文件

```bash
npx playwright test tests/e2e/01-homepage.spec.ts
```

### 3. 单个测试用例

```bash
npx playwright test -g "首页加载"
```

### 4. 查看 HTML 报告

```bash
npx playwright show-report
```

---

## 📝 下一步

1. **启动开发服务器** 确保应用在 `http://localhost:5173` 运行
2. **运行测试** `npm run test`
3. **查看报告** 检查控制台输出或 `tests/test-report.json`
4. **确认修复** 如有失败，确认修复方案后执行

---

*详细设计文档：`../../docs/superpowers/specs/2026-03-13-multi-agent-testing-workflow-design.md`*

# 实施计划：删除 Setup 中 Node.js/Git 残留检测代码

## 任务概述

删除 `installer.ts` 中残留的 nodejs/git 检测命令定义，使代码与实际行为一致。

## 修改内容

### 文件：`installer/src/main/installer.ts`

**位置**：第 52-57 行，`checkDependency` 函数内的 `commands` 对象

**修改前**：
```typescript
const commands: Record<string, string> = {
  nodejs: 'node --version',
  git: 'git --version',
  claude: 'claude --version',
  opencode: 'opencode --version',
}
```

**修改后**：
```typescript
const commands: Record<string, string> = {
  claude: 'claude --version',
  opencode: 'opencode --version',
}
```

## 验证方式

1. TypeScript 编译通过
2. 运行 Setup 安装流程，确认"智能体检测"页面只显示 Claude CLI 和 OpenCode

## 影响范围

- 仅影响 `installer.ts` 文件
- 不影响任何实际功能（nodejs/git 已不被调用）
- 纯代码清理，无风险
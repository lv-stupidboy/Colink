# Colink Desktop App - Pre-Landing Code Review Report

**日期:** 2026-04-29
**分支:** master (15 commits ahead of origin/master)
**审查人:** QA Engineer (质量保证工程师)
**质量评分:** 5.0/10

---

## 概述

本次审查覆盖 Colink 桌面应用实现，包括：
- 新增 `apps/desktop/` 目录（Electron + Go daemon）
- 修改 `installer/src/main/shared/install-utils.ts`
- 移除 `RegistryTypeCodeHub`（前后端同步）
- Makefile 构建目标扩展

**Scope Check:** ✅ CLEAN - 交付内容与计划一致

---

## 关键发现

### 🔴 CRITICAL - Security (2 项)

| # | 文件 | 行号 | 问题 | 修复建议 |
|---|------|------|------|----------|
| 1 | `apps/desktop/src/main/index.ts` | 61 | **webSecurity: false** 禁用同源策略 | 移除或使用 `session.webRequest` 针对性允许跨域 |
| 2 | `installer/src/main/shared/install-utils.ts` | 144 | **Shell injection** - execSync 拼接来自注册表的 uninstallerPath | 使用 `execFile` + array arguments |

### 🔴 CRITICAL - Testing (8 项)

| # | 文件 | 行号 | 问题 |
|---|------|------|------|
| 1 | `daemon-manager.ts` | 16 | 无单元测试覆盖 daemon 状态机（7个状态） |
| 2 | `daemon-manager.ts` | 154 | startDaemon 缺少失败场景测试 |
| 3 | `daemon-manager.ts` | 188 | stopDaemon 缺少失败场景测试 |
| 4 | `daemon-manager.ts` | 34 | fetchHealthAtPort 缺少超时/abort测试 |
| 5 | `daemon-manager.ts` | 95 | fetchHealth 缺少远程模式测试 |
| 6 | `daemon-manager.ts` | 138 | withGuard 缺少并发操作防护测试 |
| 7 | `preload/index.ts` | 1 | IPC handlers 未测试 |
| 8 | `main/index.ts` | 30 | handleDeepLink 未测试 |

### 🟡 INFORMATIONAL - Maintainability (7 项)

| # | 文件 | 行号 | 问题 | 修复建议 |
|---|------|------|------|----------|
| 1 | `api-bridge.ts` | 9 | Magic number 26305 硬编码 4 次 | 创建共享常量 DEFAULT_SERVER_PORT |
| 2 | `daemon-manager.ts` | 37 | Timeout 值散落 (2000/15000/20000) | 定义命名常量 |
| 3 | `daemon-manager.ts` | 133 | '.colink' 目录路径重复定义 | 使用 PREFS_PATH 的 dirname |
| 4 | `daemon-manager.ts` | 247 | setupDaemonManager() 52行，职责过多 | 拆分为 registerIpcHandlers + setupQuitHandler |
| 5 | `daemon-manager.ts` | 294 | 空 catch 块吞噬错误 | 添加 console.warn |
| 6 | `main/index.ts` | 162 | targetApiBaseUrl stale reference | 使用 getter 函数 |
| 7 | `bundle-server.mjs` | 22 | binaryNameForPlatform 可内联 | 保留但添加注释 |

### 🟡 INFORMATIONAL - Security (4 项)

| # | 文件 | 行号 | 问题 |
|---|------|------|------|
| 1 | `daemon-manager.ts` | 194 | pkill -f 可能匹配非预期进程 |
| 2 | `main/index.ts` | 36 | Deep link token 未验证格式 |
| 3 | `daemon-manager.ts` | 274 | IPC handler 接收任意 URL |
| 4 | `external-url.ts` | 4 | URL allowlist 仅检查协议 |

---

## 专家审查统计

| 专家 | 派遣 | 发现 | Critical | Informational |
|------|------|------|----------|---------------|
| Testing | ✓ | 12 | 8 | 4 |
| Security | ✓ | 6 | 2 | 4 |
| Maintainability | ✓ | 7 | 0 | 7 |

---

## 用户确认事项

| 问题 | 用户选择 |
|------|----------|
| 修复安全问题 | ✅ 确认修复 |
| 添加测试 | ✅ 确认添加 |
| 移除 CodeHub Registry | ✅ 确认移除（正确变更） |

---

## 修复清单

### 必须修复（CRITICAL Security）

1. **webSecurity: false** (`apps/desktop/src/main/index.ts:61`)
   ```typescript
   // 修复：移除 webSecurity: false，使用 session.webRequest 针对性允许
   webPreferences: {
     preload: join(__dirname, "../preload/index.js"),
     sandbox: false,
     // webSecurity: false,  // 删除此行
   },
   ```

2. **Shell injection** (`installer/src/main/shared/install-utils.ts:144`)
   ```typescript
   // 修复：使用 execFile 替代 execSync
   import { execFile } from 'child_process';
   execFile(uninstallerPath, ['/S'], { timeout: 60000 }, (err) => { ... });
   ```

### 必须添加（Testing）

创建以下测试文件：
- `apps/desktop/src/main/daemon-manager.test.ts`
- `apps/desktop/src/preload/index.test.ts`
- `apps/desktop/src/main/index.test.ts` (handleDeepLink)
- `apps/desktop/src/renderer/src/platform/api-bridge.test.ts`

### 可选修复（Maintainability）

创建共享常量文件 `apps/desktop/src/shared/constants.ts`：
```typescript
export const DEFAULT_SERVER_PORT = 26305;
export const HEALTH_CHECK_TIMEOUT_MS = 2_000;
export const DAEMON_START_TIMEOUT_MS = 20_000;
export const DAEMON_STOP_TIMEOUT_MS = 15_000;
export const POLL_INTERVAL_MS = 5_000;
```

---

## 审查结论

**状态:** ISSUES_FOUND - 需要修复后方可发布

**下一步:**
1. 开发工程师修复 CRITICAL Security 问题
2. 开发工程师添加单元测试
3. QA 重新验证修复后的代码

---

## 审查日志

- 审查时间: 2026-04-29 17:00
- 派遣专家: Testing, Security, Maintainability
- 用户确认: 已完成
- 审查文件: 32 files, 9093 insertions, 707 deletions
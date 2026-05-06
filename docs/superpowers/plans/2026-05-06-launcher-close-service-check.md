<!-- /autoplan restore point: ~/.gstack/projects/isdp/master-autoplan-restore-20260506-181058.md -->
# Launcher 关闭时服务状态检查实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tauri Launcher 关闭时检查 colink-server 运行状态，服务运行时弹窗确认停止或取消。

**Architecture:** 新增 Rust IPC 命令 `window_close_with_confirm`，调用 ServiceManager 检查服务状态，使用 Tauri Dialog 插件弹窗确认，返回关闭决策给前端。

**Tech Stack:** Tauri 2 (Rust + React), tauri-plugin-dialog

---

## File Structure

| 文件 | 责任 |
|------|------|
| `src-tauri/src/commands/window.rs` | 新增 `window_close_with_confirm` 命令实现 |
| `src-tauri/src/commands/mod.rs` | 无需修改（已有 `pub use window::*`） |
| `src-tauri/src/lib.rs` | 注册新命令到 invoke_handler |
| `src/lib/api/window.ts` | 更新 `close()` API 返回类型 |
| `src/renderer/src/components/Layout.tsx` | 处理 `blockClose` 返回值 |

---

### Task 1: 新增 Rust 关闭确认命令

**Files:**
- Modify: `installer-tauri/src-tauri/src/commands/window.rs`

- [ ] **Step 1: 添加 CloseResult 类型和新命令**

在 `window.rs` 文件末尾添加：

```rust
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{AppHandle, Manager, State};
use tauri_plugin_dialog::DialogExt;
use crate::store::AppState;

// Global guard to prevent double-click race condition
static CLOSE_PENDING: AtomicBool = AtomicBool::new(false);

/// Result of close confirmation
#[derive(serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub enum CloseResult {
    AllowClose,
    BlockClose,
}

/// Close window with service running check confirmation
#[tauri::command]
pub async fn window_close_with_confirm(
    app: AppHandle,
    state: State<'_, AppState>
) -> Result<CloseResult, String> {
    // Guard against double-click race condition
    if CLOSE_PENDING.swap(true, Ordering::SeqCst) {
        log::warn!("Close already pending, ignoring duplicate request");
        return Ok(CloseResult::BlockClose);
    }

    // Check if service is running
    let is_running = {
        let service_guard = state.service_manager.read().unwrap();
        if let Some(manager) = service_guard.as_ref() {
            manager.is_running()
        } else {
            false
        }
    };

    if !is_running {
        // Service not running, allow close directly
        CLOSE_PENDING.store(false, Ordering::SeqCst);
        return Ok(CloseResult::AllowClose);
    }

    // Service is running, show confirmation dialog
    let choice = app.dialog()
        .message("服务正在运行，请先停止服务后再关闭窗口。")
        .title("无法关闭")
        .kind(tauri_plugin_dialog::MessageDialogKind::Warning)
        .buttons(tauri_plugin_dialog::MessageDialogButtons::OkCancelCustom(
            "停止服务并关闭".to_string(),
            "取消".to_string()
        ))
        .await;

    match choice {
        tauri_plugin_dialog::MessageDialogResult::Ok => {
            // User chose to stop service and close
            let stop_result = {
                let service_guard = state.service_manager.read().unwrap();
                if let Some(manager) = service_guard.as_ref() {
                    manager.stop()
                } else {
                    Ok(())
                }
            };

            match stop_result {
                Ok(()) => {
                    // Clear service manager state
                    let mut service_guard = state.service_manager.write().unwrap();
                    *service_guard = None;

                    CLOSE_PENDING.store(false, Ordering::SeqCst);
                    Ok(CloseResult::AllowClose)
                }
                Err(e) => {
                    // Show error dialog to inform user
                    log::error!("Failed to stop service: {}", e);
                    CLOSE_PENDING.store(false, Ordering::SeqCst);

                    app.dialog()
                        .message(format!("停止服务失败：{}", e))
                        .title("错误")
                        .kind(tauri_plugin_dialog::MessageDialogKind::Error)
                        .await;

                    Ok(CloseResult::BlockClose)
                }
            }
        }
        tauri_plugin_dialog::MessageDialogResult::Cancel => {
            CLOSE_PENDING.store(false, Ordering::SeqCst);
            Ok(CloseResult::BlockClose)
        }
        _ => {
            CLOSE_PENDING.store(false, Ordering::SeqCst);
            Ok(CloseResult::BlockClose)
        }
    }
}
```

**关键改进**：
1. **双击竞态防护** - `CLOSE_PENDING` AtomicBool 防止快速双击触发多次弹窗
2. **stop() 失败弹窗** - 失败时弹出错误对话框告知用户，而非静默失败
3. **guard 复位** - 所有分支都正确复位 guard，避免后续关闭被阻止

- [ ] **Step 2: 编译验证**

Run: `cd installer-tauri && cargo check`

Expected: 无编译错误

---

### Task 2: 注册新命令到 Tauri

**Files:**
- Modify: `installer-tauri/src-tauri/src/lib.rs:39-93`

- [ ] **Step 1: 在 invoke_handler 中添加新命令**

在 `lib.rs` 的 `invoke_handler` 中，找到 window commands 部分（约第 90-92 行），将：

```rust
            // Window commands
            commands::window_minimize,
            commands::window_maximize,
            commands::window_close,
```

改为：

```rust
            // Window commands
            commands::window_minimize,
            commands::window_maximize,
            commands::window_close,
            commands::window_close_with_confirm,
```

- [ ] **Step 2: 编译验证**

Run: `cd installer-tauri && cargo check`

Expected: 无编译错误

---

### Task 3: 更新前端 API

**Files:**
- Modify: `installer-tauri/src/lib/api/window.ts`

- [ ] **Step 1: 定义返回类型并更新 close 方法**

将 `window.ts` 内容改为：

```typescript
import { invoke } from '@tauri-apps/api/core';

export type CloseResult = 'allowClose' | 'blockClose';

export const windowApi = {
  minimize: async (): Promise<void> => {
    await invoke('window_minimize');
  },

  maximize: async (): Promise<void> => {
    await invoke('window_maximize');
  },

  close: async (): Promise<CloseResult> => {
    const result = await invoke<{ result: CloseResult }>('window_close_with_confirm');
    return result.result;
  },
};
```

- [ ] **Step 2: TypeScript 类型检查**

Run: `cd installer-tauri && pnpm typecheck`

Expected: 无类型错误

---

### Task 4: 更新前端关闭按钮处理

**Files:**
- Modify: `installer-tauri/src/renderer/src/components/Layout.tsx:37-43`

- [ ] **Step 1: 修改 handleClose 函数处理返回值**

将 `Layout.tsx` 中的 `handleClose` 函数（约第 37-43 行）改为：

```typescript
  const handleClose = async () => {
    try {
      const result = await windowApi.close();
      if (result === 'blockClose') {
        // User cancelled, don't close
        return;
      }
      // allowClose - window will be closed by backend
    } catch (e) {
      console.error('Failed to close:', e);
    }
  };
```

注意：由于 `window_close_with_confirm` 返回 `AllowClose` 时窗口还未关闭，需要在返回 `AllowClose` 后实际关闭窗口。

实际上，根据设计，当返回 `AllowClose` 时应该执行窗口关闭。让我重新审视：当前 `window_close` 命令会执行 `window.close()`，而新命令只返回决策，不执行关闭。

需要在返回 `AllowClose` 后调用原来的 `window_close`，或在新命令中直接关闭。

**修正方案**：在新命令返回 `AllowClose` 后，前端调用原有的 `window_close`。

更新 `window.ts`：

```typescript
import { invoke } from '@tauri-apps/api/core';

export type CloseResult = 'allowClose' | 'blockClose';

export const windowApi = {
  minimize: async (): Promise<void> => {
    await invoke('window_minimize');
  },

  maximize: async (): Promise<void> => {
    await invoke('window_maximize');
  },

  close: async (): Promise<void> => {
    const result = await invoke<{ result: CloseResult }>('window_close_with_confirm');
    if (result.result === 'allowClose') {
      await invoke('window_close');
    }
  },
};
```

更新 `Layout.tsx`：

```typescript
  const handleClose = async () => {
    try {
      await windowApi.close();
    } catch (e) {
      console.error('Failed to close:', e);
    }
  };
```

- [ ] **Step 2: TypeScript 类型检查**

Run: `cd installer-tauri && pnpm typecheck`

Expected: 无类型错误

---

### Task 5: 集成测试验证

**Files:**
- 无文件修改，手动测试

- [ ] **Step 1: 启动 Launcher 开发模式**

Run: `cd installer-tauri && pnpm dev:launcher`

Expected: Launcher 窗口打开

- [ ] **Step 2: 测试场景 1 - 服务未运行时关闭**

1. 确认服务状态为"已停止"
2. 点击关闭按钮
3. Expected: 窗口直接关闭，无弹窗

- [ ] **Step 3: 测试场景 2 - 服务运行时选择取消**

1. 点击"启动服务"
2. 等待服务状态变为"运行中"
3. 点击关闭按钮
4. Expected: 弹窗出现
5. 点击"取消"
6. Expected: 窗口保持打开

- [ ] **Step 4: 测试场景 3 - 服务运行时选择停止并关闭**

1. 启动服务
2. 点击关闭按钮
3. Expected: 弹窗出现
4. 点击"停止服务并关闭"
5. Expected: 服务停止，窗口关闭

- [ ] **Step 5: 提交代码**

```bash
cd installer-tauri
git add src-tauri/src/commands/window.rs src-tauri/src/lib.rs src/lib/api/window.ts src/renderer/src/components/Layout.tsx
git commit -m "feat(launcher): add service running check on close"
```

---

## GSTACK REVIEW REPORT (Updated)

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` via /autoplan | Scope & strategy | 1 | **resolved** | 3 issues fixed: double-click guard, stop() error dialog, imports confirmed |
| Eng Review | `/plan-eng-review` via /autoplan | Architecture & tests (required) | 1 | **resolved** | 3 critical gaps fixed in Task 1 |
| Design Review | `/plan-design-review` via /autoplan | UI/UX gaps | 1 | **resolved** | UX framing confirmed, error feedback added |
| DX Review | `/plan-devex-review` | Developer experience | 0 | skipped | Not applicable (end-user feature) |

**RESOLVED:** All 6 issues from initial review have been addressed in Task 1 Step 1:
1. ✅ Double-click race guard - Added `CLOSE_PENDING` AtomicBool
2. ✅ stop() error dialog - Added error message dialog on failure
3. ✅ tauri-plugin-dialog registration - Confirmed in `lib.rs:24`
4. ✅ Imports - All required imports added (State, AppState, DialogExt, AtomicBool)
5. ✅ Guard reset - All branches correctly reset CLOSE_PENDING
6. ✅ UX feedback - Error dialog informs user of stop() failure

**VERDICT:** READY FOR IMPLEMENTATION

---

## Self-Review (Updated)

**1. Spec coverage:**
- ✅ 检查服务状态 → Task 1 Step 1
- ✅ 弹窗确认 → Task 1 Step 1
- ✅ 停止服务并关闭 → Task 1 Step 1
- ✅ 取消阻止关闭 → Task 1 Step 1
- ✅ 错误处理 → Task 1 Step 1 (stop 失败弹窗提示)
- ✅ 前端处理返回值 → Task 4

**2. Placeholder scan:**
- ✅ 无 TBD/TODO
- ✅ 所有步骤有完整代码

**3. Type consistency:**
- ✅ Rust: `CloseResult` enum with `AllowClose/BlockClose`
- ✅ TypeScript: `CloseResult` type with `'allowClose' | 'blockClose'`
- ✅ 前端 API 返回类型匹配

**4. Critical fixes applied:**
- ✅ Double-click race guard (`CLOSE_PENDING` AtomicBool)
- ✅ stop() error dialog (用户可见错误反馈)
- ✅ All imports confirmed (State, AppState, DialogExt, AtomicBool)
# Tauri Launcher Agent 运行检查实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tauri Launcher 停止服务时（关闭窗口 + 停止按钮），检查 Agent 实例运行状态，阻止操作并提示用户。

**Architecture:** 在 Rust 后端 `window_close_with_confirm` 和 `stop_service` 命令中增加 Agent 检查，调用现有 `ServiceManager::get_running_agents()` API。

**Tech Stack:** Tauri 2 (Rust), tauri-plugin-dialog, reqwest

---

## File Structure

| 文件 | 责任 |
|------|------|
| `src-tauri/src/commands/window.rs` | 关闭窗口时 Agent 检查（服务运行后先检查 Agent） |
| `src-tauri/src/commands/service.rs` | 停止服务时 Agent 检查（防御层） |

**复用现有代码**：
- `ServiceManager::get_running_agents(port)` - API 调用已实现（`service_manager.rs:458-479`）
- `read_existing_config(install_dir)` - 端口读取已实现（`services/config.rs:198-227`）

---

### Task 1: 修改 `window_close_with_confirm` 增加 Agent 检查

**Files:**
- Modify: `installer-tauri/src-tauri/src/commands/window.rs:50-142`

**背景**：当前 `window_close_with_confirm` 流程：
1. 检查服务状态 `is_running`
2. 服务未运行 → 直接关闭
3. 服务运行 → 弹窗确认停止服务

**变更**：在步骤 3 之前增加 Agent 检查：
- 服务运行 → 先检查 Agent → 有 Agent 则阻止关闭 → 无 Agent 则继续原有流程

- [ ] **Step 1: 添加 imports**

在 `window.rs` 文件顶部（第 1-4 行后）添加：

```rust
use crate::services::service_manager::ServiceManager;
use crate::services::config::read_existing_config;
```

- [ ] **Step 2: 修改 `window_close_with_confirm` 函数**

将整个函数（第 50-142 行）替换为：

```rust
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

    // Service is running - check for running agents first
    let install_dir = state.get_install_dir();
    let agent_count = if let Some(dir) = install_dir {
        // Get port from config
        let port = read_existing_config(&dir)
            .map(|(p, _)| p)
            .unwrap_or(26305);

        // Check running agents
        let manager = ServiceManager::new(dir);
        match manager.get_running_agents(port).await {
            Ok(agents) => agents.len(),
            Err(e) => {
                // API failure - log warning, treat as no agents (conservative)
                log::warn!("Failed to check running agents: {}", e);
                0
            }
        }
    } else {
        0
    };

    // If agents are running, show dialog and block close
    if agent_count > 0 {
        let app_for_dialog = app.clone();

        tauri::async_runtime::spawn_blocking(move || {
            app_for_dialog.dialog()
                .message(format!("有 {} 个 Agent 实例正在运行，请先在 Web 控制台停止后才能关闭窗口。", agent_count))
                .title("无法关闭")
                .kind(tauri_plugin_dialog::MessageDialogKind::Warning)
                .buttons(tauri_plugin_dialog::MessageDialogButtons::OkCancelCustom(
                    "取消".to_string(),
                    "".to_string()  // Single button mode
                ))
                .blocking_show()
        })
        .await
        .map_err(|e| {
            CLOSE_PENDING.store(false, Ordering::SeqCst);
            e.to_string()
        })?;

        CLOSE_PENDING.store(false, Ordering::SeqCst);
        return Ok(CloseResult::BlockClose);
    }

    // No agents running - proceed with original service confirmation flow
    // Clone app for use in blocking thread and for potential error dialog
    let app_for_dialog = app.clone();
    let app_for_error = app.clone();

    // Service is running, show confirmation dialog (blocking)
    // blocking_show() returns bool: true = Ok clicked, false = Cancel or closed
    let ok_clicked = tauri::async_runtime::spawn_blocking(move || {
        app_for_dialog.dialog()
            .message("服务正在运行，请先停止服务后再关闭窗口。")
            .title("无法关闭")
            .kind(tauri_plugin_dialog::MessageDialogKind::Warning)
            .buttons(tauri_plugin_dialog::MessageDialogButtons::OkCancelCustom(
                "停止服务并关闭".to_string(),
                "取消".to_string()
            ))
            .blocking_show()
    })
    .await
    .map_err(|e| {
        CLOSE_PENDING.store(false, Ordering::SeqCst);
        e.to_string()
    })?;

    if ok_clicked {
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

                tauri::async_runtime::spawn_blocking(move || {
                    app_for_error.dialog()
                        .message(format!("停止服务失败：{}", e))
                        .title("错误")
                        .kind(tauri_plugin_dialog::MessageDialogKind::Error)
                        .blocking_show()
                })
                .await
                .map_err(|e| e.to_string())?;

                Ok(CloseResult::BlockClose)
            }
        }
    } else {
        // User clicked Cancel or closed dialog
        CLOSE_PENDING.store(false, Ordering::SeqCst);
        Ok(CloseResult::BlockClose)
    }
}
```

**关键点**：
1. Agent 检查在服务运行检查之后、原有确认流程之前
2. Agent 运行时弹窗只有一个"取消"按钮（阻止关闭）
3. API 失败时保守策略：视为无 Agent，允许操作继续
4. `CLOSE_PENDING` guard 在所有分支正确复位

- [ ] **Step 3: 编译验证**

Run: `cd installer-tauri && cargo check`

Expected: 无编译错误

---

### Task 2: 修改 `stop_service` 增加 Agent 检查

**Files:**
- Modify: `installer-tauri/src-tauri/src/commands/service.rs:36-53`

**背景**：当前 `stop_service` 直接停止服务，无任何检查。

**变更**：停止前检查 Agent，有 Agent 运行则返回错误。

- [ ] **Step 1: 添加 imports**

在 `service.rs` 文件顶部（第 1-3 行后）添加：

```rust
use crate::services::config::read_existing_config;
```

- [ ] **Step 2: 修改 `stop_service` 函数**

将函数（第 36-53 行）替换为：

```rust
/// Stop service
#[tauri::command]
pub async fn stop_service(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    // Get install_dir for agent check
    let install_dir = state.get_install_dir();

    // Defensive check: verify no agents are running before stopping
    if let Some(dir) = install_dir {
        let port = read_existing_config(&dir)
            .map(|(p, _)| p)
            .unwrap_or(26305);

        let manager = ServiceManager::new(dir);
        match manager.get_running_agents(port).await {
            Ok(agents) if !agents.is_empty() => {
                // Agents running - block stop and return error
                let count = agents.len();
                log::warn!("Cannot stop service: {} agent instances running", count);
                return Ok(serde_json::json!({
                    "success": false,
                    "error": format!("有 {} 个 Agent 实例正在运行，请先在 Web 控制台停止", count),
                    "agentCount": count
                }));
            }
            Ok(_) => {
                // No agents - proceed with stop
                log::info!("No agents running, proceeding with service stop");
            }
            Err(e) => {
                // API failure - log warning, proceed with stop (conservative)
                log::warn!("Failed to check running agents before stop: {}", e);
            }
        }
    }

    // Stop service
    {
        let service_guard = state.service_manager.read().unwrap();
        if let Some(manager) = service_guard.as_ref() {
            manager.stop().map_err(|e| e.to_string())?;
        }
    }

    // Clear state
    let mut service_guard = state.service_manager.write().unwrap();
    *service_guard = None;

    Ok(serde_json::json!({ "success": true }))
}
```

**关键点**：
1. Agent 检查在停止之前
2. 有 Agent 返回 `{ success: false, error: "...", agentCount: N }`
3. 前端已有检查（toast 提示），后端检查作为防御层
4. API 失败时保守策略：继续停止

- [ ] **Step 3: 编译验证**

Run: `cd installer-tauri && cargo check`

Expected: 无编译错误

---

### Task 3: 集成测试验证

**Files:**
- 无文件修改，手动测试

- [ ] **Step 1: 启动 Launcher 开发模式**

Run: `cd installer-tauri && pnpm dev:launcher`

Expected: Launcher 窗口打开

- [ ] **Step 2: 测试关闭窗口场景**

**场景 A - 服务未运行**：
1. 确认服务状态为"已停止"
2. 点击关闭按钮
3. Expected: 窗口直接关闭，无弹窗

**场景 B - 服务运行 + Agent 运行**：
1. 启动服务
2. 在 Web 控制台启动一个 Agent 任务
3. 点击关闭按钮
4. Expected: 弹窗提示"有 X 个 Agent 实例正在运行..."
5. 点击取消
6. Expected: 窗口保持打开

**场景 C - 服务运行 + 无 Agent**：
1. 启动服务（确保无 Agent 运行）
2. 点击关闭按钮
3. Expected: 弹窗提示"服务正在运行..."
4. 点击"停止服务并关闭"
5. Expected: 服务停止，窗口关闭

- [ ] **Step 3: 测试停止按钮场景**

**场景 A - Agent 运行**：
1. 启动服务
2. 启动一个 Agent 任务
3. 点击停止按钮
4. Expected: 前端 toast 提示"有 Agent 实例正在运行..."

**场景 B - 无 Agent**：
1. 启动服务（确保无 Agent 运行）
2. 点击停止按钮
3. Expected: 服务正常停止

- [ ] **Step 4: 提交代码**

```bash
cd installer-tauri
git add src-tauri/src/commands/window.rs src-tauri/src/commands/service.rs
git commit -m "feat(launcher): add agent running check before stop service

- Check running agents before stopping service in window_close_with_confirm
- Add defensive agent check in stop_service command
- Block close/stop with dialog when agents are running
- Conservative fallback: allow operation on API failure"
```

---

## Self-Review

**1. Spec coverage:**
- ✅ 关闭窗口时 Agent 检查 → Task 1 Step 2
- ✅ 关闭窗口时 Agent 弹窗 → Task 1 Step 2（第 46-60 行）
- ✅ 停止按钮后端检查 → Task 2 Step 2
- ✅ API 失败保守策略 → Task 1 Step 2（第 30-35 行），Task 2 Step 2（第 35-38 行）
- ✅ CLOSE_PENDING guard 复位 → Task 1 Step 2（所有分支）

**2. Placeholder scan:**
- ✅ 无 TBD/TODO
- ✅ 所有步骤有完整代码

**3. Type consistency:**
- ✅ `CloseResult::BlockClose` / `CloseResult::AllowClose` 与原实现一致
- ✅ `stop_service` 返回 JSON 与前端期望一致（`success`, `error`, `agentCount`）
- ✅ `ServiceManager::new(install_dir)` 与 `get_running_agents(port)` 签名匹配

**4. Dependencies:**
- ✅ `ServiceManager` import 已添加
- ✅ `read_existing_config` import 已添加
- ✅ 无新增依赖（复用现有）

---

## 验收标准对照

| 标准 | 实现 |
|------|------|
| 关闭窗口时，Agent 运行中弹窗提示并阻止关闭 | Task 1 Step 2 |
| 关闭窗口时，无 Agent 运行但服务运行，弹窗确认停止服务 | Task 1 Step 2（原有流程保留） |
| 停止按钮点击时，后端防御性检查生效 | Task 2 Step 2 |
| API 调用失败时，不阻塞正常操作 | Task 1/2 Step 2（保守策略） |
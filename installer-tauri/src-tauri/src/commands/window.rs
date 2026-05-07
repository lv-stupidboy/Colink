use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{AppHandle, Manager, State};
use tauri_plugin_dialog::DialogExt;
use crate::store::AppState;
use crate::services::service_manager::ServiceManager;
use crate::services::config::read_existing_config;

// Global guard to prevent double-click race condition
static CLOSE_PENDING: AtomicBool = AtomicBool::new(false);

/// Minimize window
#[tauri::command]
pub async fn window_minimize(app: AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        window.minimize().map_err(|e| e.to_string())?;
    }
    Ok(())
}

/// Maximize or unmaximize window
#[tauri::command]
pub async fn window_maximize(app: AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        if window.is_maximized().map_err(|e| e.to_string())? {
            window.unmaximize().map_err(|e| e.to_string())?;
        } else {
            window.maximize().map_err(|e| e.to_string())?;
        }
    }
    Ok(())
}

/// Close window
#[tauri::command]
pub async fn window_close(app: AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        window.close().map_err(|e| e.to_string())?;
    }
    Ok(())
}

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
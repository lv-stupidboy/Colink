use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{AppHandle, Manager, State};
use tauri_plugin_dialog::DialogExt;
use crate::store::AppState;

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
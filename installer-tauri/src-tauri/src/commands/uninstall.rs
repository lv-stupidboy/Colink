use crate::services::uninstall as uninstall_service;
use tauri::AppHandle;

/// Confirm uninstall with dialog
#[tauri::command]
pub async fn confirm_uninstall(app: AppHandle) -> Result<serde_json::Value, String> {
    use tauri_plugin_dialog::DialogExt;

    // Show confirmation dialog
    let result = tauri::async_runtime::spawn_blocking(move || {
        app.dialog()
            .message("确定要卸载 Colink？\n\n选择保留数据将保留 data 目录中的用户数据。")
            .title("卸载确认")
            .kind(tauri_plugin_dialog::MessageDialogKind::Warning)
            .blocking_show()
    })
    .await
    .map_err(|e| e.to_string())?;

    Ok(serde_json::json!({ "confirmed": result, "keepData": false }))
}

/// Run uninstall with specified directory
#[tauri::command]
pub async fn run_uninstall(
    #[allow(non_snake_case)] installDir: String,
    #[allow(non_snake_case)] keepData: bool,
) -> Result<serde_json::Value, String> {
    uninstall_service::uninstall(&installDir, keepData)
        .map_err(|e| e.to_string())?;

    Ok(serde_json::json!({ "success": true }))
}

/// Clean registry entries
#[tauri::command]
pub fn clean_registry() -> Result<serde_json::Value, String> {
    crate::services::registry::delete_registry()
        .map_err(|e| e.to_string())?;

    Ok(serde_json::json!({ "success": true }))
}

/// Remove shortcuts
#[tauri::command]
pub fn remove_shortcuts() -> Result<serde_json::Value, String> {
    uninstall_service::remove_shortcuts()
        .map_err(|e| e.to_string())?;

    Ok(serde_json::json!({ "success": true }))
}

/// Execute uninstall (legacy)
#[tauri::command]
pub async fn uninstall(
    #[allow(non_snake_case)] keepData: bool,
) -> Result<serde_json::Value, String> {
    // Get installed version
    let installed = crate::services::registry::get_installed_version()
        .map_err(|e| e.to_string())?;

    if !installed.installed {
        return Ok(serde_json::json!({ "success": false, "error": "未安装" }));
    }

    let install_dir = installed
        .install_dir
        .ok_or_else(|| "Install directory not found".to_string())?;

    uninstall_service::uninstall(&install_dir, keepData)
        .map_err(|e| e.to_string())?;

    Ok(serde_json::json!({ "success": true }))
}
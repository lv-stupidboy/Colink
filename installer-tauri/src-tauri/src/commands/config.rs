use crate::store::AppState;
use crate::services::config;
use tauri::State;

/// Read config file content
#[tauri::command]
pub fn read_config_file(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| "Install directory not set".to_string())?;

    match config::read_config_file(&install_dir) {
        Ok(content) => Ok(serde_json::json!({ "success": true, "content": content })),
        Err(e) => Ok(serde_json::json!({ "success": false, "error": e.to_string() })),
    }
}

/// Save config file
#[tauri::command]
pub async fn save_config(
    state: State<'_, AppState>,
    yaml: String,
) -> Result<serde_json::Value, String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| "Install directory not set".to_string())?;

    config::save_config_file(&install_dir, &yaml)
        .map_err(|e| e.to_string())?;

    Ok(serde_json::json!({ "success": true }))
}

/// Get existing parsed config
#[tauri::command]
pub fn get_existing_config(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| "Install directory not set".to_string())?;

    match config::read_existing_config(&install_dir) {
        Ok(config) => Ok(serde_json::json!({ "success": true, "config": config })),
        Err(e) => Ok(serde_json::json!({ "success": false, "error": e.to_string() })),
    }
}
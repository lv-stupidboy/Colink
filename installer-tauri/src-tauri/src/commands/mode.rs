use crate::store::AppState;
use tauri::{Manager, State};

/// Check if running in launcher mode
#[tauri::command]
pub fn is_launcher_mode(state: State<'_, AppState>) -> bool {
    state.get_mode() == crate::store::AppMode::Launcher
}

/// Get startup action (install/upgrade/uninstall)
#[tauri::command]
pub fn get_startup_action() -> Result<String, String> {
    crate::services::uninstall::get_startup_action()
        .map_err(|e| e.to_string())
}

/// Get app config directory (Tauri's app data directory)
#[tauri::command]
pub fn get_app_path(app: tauri::AppHandle) -> Result<String, String> {
    app.path()
        .app_config_dir()
        .map(|p| p.to_string_lossy().to_string())
        .map_err(|e| e.to_string())
}

/// Get the actual install directory from state (set during setup or loaded from registry for launcher)
#[tauri::command]
pub fn get_install_dir(state: State<'_, AppState>) -> Option<String> {
    state.get_install_dir()
}

/// Get resource path
#[tauri::command]
pub fn get_resource_path(app: tauri::AppHandle) -> Result<String, String> {
    app.path()
        .resource_dir()
        .map(|p| p.to_string_lossy().to_string())
        .map_err(|e| e.to_string())
}

/// Get version from VERSION file
#[tauri::command]
pub fn get_version(app: tauri::AppHandle) -> Result<String, String> {
    let resource_path = app
        .path()
        .resource_dir()
        .map_err(|e| e.to_string())?;

    // Try resources subdirectory first (dev mode), then root (release mode)
    let version_path = if resource_path.join("resources/VERSION").exists() {
        resource_path.join("resources/VERSION")
    } else if resource_path.join("VERSION").exists() {
        resource_path.join("VERSION")
    } else {
        // Fallback: try to read from exe directory
        let exe_path = std::env::current_exe().map_err(|e| e.to_string())?;
        let exe_dir = exe_path.parent().unwrap_or(std::path::Path::new("."));
        exe_dir.join("VERSION")
    };

    if version_path.exists() {
        let version = std::fs::read_to_string(&version_path)
            .map_err(|e| e.to_string())?
            .trim()
            .to_string();
        Ok(version)
    } else {
        Ok("1.0.0".to_string()) // Default version
    }
}
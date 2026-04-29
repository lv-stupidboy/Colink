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

    let exe_path = std::env::current_exe().map_err(|e| e.to_string())?;
    let exe_dir = exe_path.parent().unwrap_or(std::path::Path::new("."));

    // Try multiple locations to find VERSION file:
    // 1. resource_path/resources/VERSION (dev mode)
    // 2. resource_path/VERSION (release mode bundled)
    // 3. exe_dir/resources/VERSION (ZIP packaged mode: exe in exe/, VERSION in exe/resources/)
    // 4. exe_dir/../resources/VERSION (ZIP packaged mode alternative)
    // 5. exe_dir/VERSION (fallback)
    let version_candidates = vec![
        resource_path.join("resources/VERSION"),
        resource_path.join("VERSION"),
        exe_dir.join("resources/VERSION"),
        exe_dir.join("..").join("resources/VERSION"),
        exe_dir.join("VERSION"),
    ];

    for version_path in version_candidates {
        if version_path.exists() {
            let version = std::fs::read_to_string(&version_path)
                .map_err(|e| e.to_string())?
                .trim()
                .to_string();
            log::info!("Found VERSION at {:?}: {}", version_path, version);
            return Ok(version);
        }
    }

    log::warn!("VERSION file not found, using default 1.0.0");
    Ok("1.0.0".to_string()) // Default version
}
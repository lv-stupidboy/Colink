use crate::services::{installer, registry, shortcut};
use crate::services::installer::{InstallConfig, InstallProgress, InstallResult};
use serde::Deserialize;
use tauri::{AppHandle, Emitter, Manager};

/// Check installed version
#[tauri::command]
pub fn check_installed() -> Result<registry::InstalledVersion, String> {
    registry::get_installed_version()
        .map_err(|e| e.to_string())
}

/// Check old ISDP installation
#[tauri::command]
pub fn check_old_isdp() -> Result<registry::InstalledVersion, String> {
    registry::get_old_isdp_version()
        .map_err(|e| e.to_string())
}

/// Uninstall old ISDP silently
#[tauri::command]
pub fn uninstall_old_isdp() -> Result<serde_json::Value, String> {
    let old_version = registry::get_old_isdp_version()
        .map_err(|e| e.to_string())?;

    if !old_version.installed {
        return Ok(serde_json::json!({ "success": true }));
    }

    if let Some(dir) = &old_version.install_dir {
        registry::uninstall_old_isdp(dir)
            .map_err(|e| e.to_string())?;
    }

    Ok(serde_json::json!({ "success": true }))
}

/// Select directory using dialog
#[tauri::command]
pub async fn select_directory(
    app: AppHandle,
    default_path: Option<String>,
) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;

    let mut builder = app.dialog().file();

    if let Some(path) = default_path {
        if let Some(p) = std::path::PathBuf::from(&path).parent() {
            builder = builder.set_directory(p);
        }
    }

    let result = tauri::async_runtime::spawn_blocking(move || builder.blocking_pick_folder())
        .await
        .map_err(|e| e.to_string())?;

    Ok(result.map(|p| p.to_string()))
}

/// Generate config preview
#[tauri::command]
pub fn generate_config_preview(
    app: AppHandle,
    #[allow(non_snake_case)] _installDir: Option<String>,
    #[allow(non_snake_case)] database: Option<DatabaseConfig>,
    #[allow(non_snake_case)] serverPort: Option<u16>,
) -> Result<serde_json::Value, String> {
    let resource_path = app
        .path()
        .resource_dir()
        .map_err(|e| e.to_string())?;

    // Find config template - try multiple locations:
    // 1. resource_path/resources (dev mode)
    // 2. resource_path (release mode with bundled resources)
    // 3. exe_dir/../resources (ZIP packaged mode: exe in exe/, resources in sibling resources/)
    let exe_path = std::env::current_exe().ok();
    let exe_dir = exe_path.as_ref().and_then(|p| p.parent());

    let template_candidates = vec![
        resource_path.join("resources/config.yaml.example"),
        resource_path.join("config.yaml.example"),
        exe_dir.map(|d| d.join("..").join("resources").join("config.yaml.example")).unwrap_or_default(),
    ];

    let template_path = template_candidates
        .iter()
        .find(|p| p.exists())
        .cloned()
        .unwrap_or_else(|| resource_path.join("config.yaml.example"));

    log::info!("Looking for config template at: {:?}", template_path);

    let db_type = database
        .map(|d| d.r#type)
        .unwrap_or_else(|| "sqlite".to_string());
    let server_port = serverPort.unwrap_or(26305);

    match crate::services::config::generate_config_preview(&template_path, server_port, 0, &db_type) {
        Ok(yaml) => Ok(serde_json::json!({ "success": true, "yaml": yaml })),
        Err(e) => Ok(serde_json::json!({ "success": false, "error": e.to_string() })),
    }
}

#[derive(Debug, Deserialize)]
pub struct DatabaseConfig {
    pub r#type: String,
}

/// Read existing config
#[tauri::command]
pub fn read_existing_config(
    #[allow(non_snake_case)] installDir: String,
) -> Result<serde_json::Value, String> {
    match crate::services::config::read_existing_config(&installDir) {
        Ok((server_port, web_port)) => {
            Ok(serde_json::json!({
                "success": true,
                "config": [server_port, web_port]
            }))
        },
        Err(e) => Ok(serde_json::json!({ "success": false, "error": e.to_string() })),
    }
}

/// Start installation with progress events
#[tauri::command]
pub async fn start_installation(
    app: AppHandle,
    config: InstallConfig,
) -> Result<InstallResult, String> {
    log::info!("Starting installation with config: {:?}", config);

    let resource_path = app
        .path()
        .resource_dir()
        .map_err(|e| {
            log::error!("Failed to get resource dir: {}", e);
            e.to_string()
        })?;

    log::info!("Resource path: {:?}", resource_path);

    let app_handle = app.clone();

    let emit_progress = |progress: &InstallProgress| {
        log::info!("Progress: {:?}", progress);
        let _ = app_handle.emit("install-progress", progress);
    };

    let result = installer::run_installation(&config, &resource_path, emit_progress)
        .await;

    match result {
        Ok(r) => {
            log::info!("Installation completed: {:?}", r);
            Ok(r)
        }
        Err(e) => {
            log::error!("Installation failed: {:?}", e);
            Err(e.to_string())
        }
    }
}

/// Create shortcut at specific path
#[tauri::command]
pub fn create_shortcut(path: String) -> Result<serde_json::Value, String> {
    // Create shortcut at given path (for testing or custom location)
    shortcut::create_desktop_shortcut(&path)
        .map_err(|e| e.to_string())?;
    Ok(serde_json::json!({ "success": true }))
}
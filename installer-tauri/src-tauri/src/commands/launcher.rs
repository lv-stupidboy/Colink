use crate::store::AppState;
use tauri::{AppHandle, State};
use tauri_plugin_opener::OpenerExt;

/// Open logs directory
#[tauri::command]
pub async fn open_logs(
    state: State<'_, AppState>,
    app: AppHandle,
) -> Result<(), String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| "Install directory not set".to_string())?;

    let logs_path = std::path::Path::new(&install_dir)
        .join("data")
        .join("logs");

    // Create if not exists
    std::fs::create_dir_all(&logs_path)
        .map_err(|e| e.to_string())?;

    app.opener()
        .open_path(logs_path.to_string_lossy().to_string(), None::<&str>)
        .map_err(|e| e.to_string())?;

    Ok(())
}

/// Open data directory
#[tauri::command]
pub async fn open_data_dir(
    state: State<'_, AppState>,
    app: AppHandle,
) -> Result<(), String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| "Install directory not set".to_string())?;

    let data_path = std::path::Path::new(&install_dir).join("data");

    app.opener()
        .open_path(data_path.to_string_lossy().to_string(), None::<&str>)
        .map_err(|e| e.to_string())?;

    Ok(())
}

/// Open config directory
#[tauri::command]
pub async fn open_config(
    state: State<'_, AppState>,
    app: AppHandle,
) -> Result<(), String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| "Install directory not set".to_string())?;

    let config_path = std::path::Path::new(&install_dir)
        .join("data")
        .join("configs");

    app.opener()
        .open_path(config_path.to_string_lossy().to_string(), None::<&str>)
        .map_err(|e| e.to_string())?;

    Ok(())
}

/// Open console (browser to backend URL)
#[tauri::command]
pub async fn open_console(
    state: State<'_, AppState>,
    app: AppHandle,
) -> Result<(), String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| "Install directory not set".to_string())?;

    // Read port from config - use server port (first element of tuple)
    let server_port = match crate::services::config::read_existing_config(&install_dir) {
        Ok((server_port, _)) => server_port,
        Err(_) => 26305,
    };

    let url = format!("http://localhost:{}", server_port);

    // Use opener plugin to open URL directly (avoids cmd.exe being flagged by security software)
    app.opener()
        .open_url(url, None::<&str>)
        .map_err(|e| e.to_string())?;

    Ok(())
}

/// Open install directory (for user to manually launch Colink.exe)
#[tauri::command]
pub async fn open_install_dir(
    install_dir: Option<String>,
    app: AppHandle,
) -> Result<(), String> {
    // Get install directory from parameter or registry
    let install_dir = match install_dir {
        Some(dir) => dir,
        None => {
            crate::services::registry::get_installed_version()
                .ok()
                .and_then(|v| v.install_dir)
                .unwrap_or_else(|| "".to_string())
        },
    };

    if install_dir.is_empty() {
        return Err("无法获取安装目录".to_string());
    }

    let install_path = std::path::Path::new(&install_dir);

    if !install_path.exists() {
        return Err(format!("安装目录不存在: {}", install_dir));
    }

    // Open the directory in file explorer
    app.opener()
        .open_path(install_path.to_string_lossy().to_string(), None::<&str>)
        .map_err(|e| e.to_string())?;

    Ok(())
}
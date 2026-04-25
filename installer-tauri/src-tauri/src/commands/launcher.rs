use crate::store::AppState;
use tauri::{AppHandle, State};
use tauri_plugin_opener::OpenerExt;
use std::process::Command;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

#[cfg(target_os = "windows")]
const CREATE_NO_WINDOW: u32 = 0x08000000;

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

    #[cfg(target_os = "windows")]
    {
        Command::new("cmd")
            .args(["/C", "start", &url])
            .creation_flags(CREATE_NO_WINDOW)
            .spawn()
            .map_err(|e| e.to_string())?;
    }

    #[cfg(not(target_os = "windows"))]
    {
        Command::new("open").arg(&url).spawn().map_err(|e| e.to_string())?;
    }

    Ok(())
}
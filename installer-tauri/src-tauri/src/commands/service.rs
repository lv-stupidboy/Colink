use crate::store::AppState;
use crate::services::service_manager::{ServiceManager, RunningAgentInstance};
use tauri::State;

/// Start service
#[tauri::command]
pub async fn start_service(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    let install_dir = state
        .get_install_dir()
        .ok_or_else(|| {
            log::error!("Install directory not set");
            "Install directory not set".to_string()
        })?;

    log::info!("Starting service with install_dir: {}", install_dir);

    let manager = ServiceManager::new(install_dir.clone());

    let result = manager
        .start()
        .map_err(|e| {
            log::error!("Failed to start service: {}", e);
            e.to_string()
        })?;

    // Store manager in state
    let mut service_guard = state.service_manager.write().unwrap();
    *service_guard = Some(manager);

    log::info!("Service started successfully");
    Ok(serde_json::json!({ "success": true }))
}

/// Stop service
#[tauri::command]
pub async fn stop_service(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
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

/// Get service status
#[tauri::command]
pub fn get_service_status(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    let service_guard = state.service_manager.read().unwrap();

    let status = if let Some(manager) = service_guard.as_ref() {
        manager.get_status()
    } else {
        "stopped".to_string()
    };

    Ok(serde_json::json!({ "status": status }))
}

/// Get running agent instances
#[tauri::command]
pub async fn get_running_agents(
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    // Check if service manager exists, then release lock before await
    let has_manager = state.service_manager.read().unwrap().is_some();

    if has_manager {
        // Get the manager reference and call async method
        // We need to clone any necessary data before the await
        let port = 26305;

        // Create a temporary manager for the API call
        let install_dir = state
            .get_install_dir()
            .ok_or_else(|| "Install directory not set".to_string())?;

        // Use a fresh manager instance for the API call
        let manager = ServiceManager::new(install_dir);
        let instances: Vec<RunningAgentInstance> = manager
            .get_running_agents(port)
            .await
            .map_err(|e| e.to_string())?;

        Ok(serde_json::json!({ "instances": instances }))
    } else {
        Ok(serde_json::json!({ "instances": [] }))
    }
}
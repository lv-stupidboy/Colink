use crate::store::AppState;
use crate::services::service_manager::{ServiceManager, RunningAgentInstance};
use crate::services::config::read_existing_config;
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

    manager
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
    // Get install_dir for agent check
    let install_dir = state.get_install_dir();

    // Defensive check: verify no agents are running before stopping
    if let Some(dir) = install_dir {
        let port = read_existing_config(&dir)
            .map(|(p, _)| p)
            .unwrap_or(26305);

        let manager = ServiceManager::new(dir);
        match manager.get_running_agents(port).await {
            Ok(agents) if !agents.is_empty() => {
                // Agents running - block stop and return error
                let count = agents.len();
                log::warn!("Cannot stop service: {} agent instances running", count);
                return Ok(serde_json::json!({
                    "success": false,
                    "error": format!("有 {} 个 Agent 实例正在运行，请先在 Web 控制台停止", count),
                    "agentCount": count
                }));
            }
            Ok(_) => {
                // No agents - proceed with stop
                log::info!("No agents running, proceeding with service stop");
            }
            Err(e) => {
                // API failure - log warning, proceed with stop (conservative)
                log::warn!("Failed to check running agents before stop: {}", e);
            }
        }
    }

    // Stop service
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
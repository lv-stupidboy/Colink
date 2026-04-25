use crate::services::dependency::{
    check_all_dependencies as check_all_deps,
    check_dependency as check_dep,
    install_dependency as install_dep,
    DependencyInfo
};

/// Check single dependency
#[tauri::command]
pub fn check_dependency(key: String) -> DependencyInfo {
    check_dep(&key)
}

/// Install dependency
#[tauri::command]
pub async fn install_dependency(key: String) -> Result<serde_json::Value, String> {
    // Run in blocking task since npm install can take time
    tauri::async_runtime::spawn_blocking(move || {
        install_dep(&key)
            .map(|_| serde_json::json!({ "success": true }))
            .map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

/// Check all dependencies
#[tauri::command]
pub fn check_all_dependencies() -> Vec<DependencyInfo> {
    check_all_deps()
}
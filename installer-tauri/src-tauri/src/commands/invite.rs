use crate::services::invite::{
    verify_invite_code as verify_code,
    save_invite_code as save_code,
    load_invite_code as load_code,
    get_system_username as get_username_from_system,
    InviteVerificationResponse
};
use crate::store::AppState;
use tauri::State;

/// Verify invite code
#[tauri::command]
pub async fn verify_invite_code(
    state: State<'_, AppState>,
    #[allow(non_snake_case)] code: String,
    #[allow(non_snake_case)] username: String,
) -> Result<InviteVerificationResponse, String> {
    // Clone the URL before the await to avoid holding the lock across await
    let verify_url = state.installer_config.read().unwrap()
        .invite_verify_url.clone();

    verify_code(&code, &username, verify_url.as_deref())
        .await
        .map_err(|e| e.to_string())
}

/// Save invite code
#[tauri::command]
pub fn save_invite_code(
    state: State<'_, AppState>,
    #[allow(non_snake_case)] inviteCode: String,
    #[allow(non_snake_case)] installDir: Option<String>,
) -> Result<serde_json::Value, String> {
    let dir = installDir
        .or_else(|| state.get_install_dir())
        .unwrap_or_else(|| ".".to_string());

    save_code(&dir, &inviteCode)
        .map_err(|e| e.to_string())?;

    Ok(serde_json::json!({ "success": true, "message": "邀请码已保存" }))
}

/// Load invite code
#[tauri::command]
pub fn load_invite_code(
    #[allow(non_snake_case)] installDir: Option<String>,
    state: State<'_, AppState>,
) -> Result<serde_json::Value, String> {
    let dir = installDir
        .or_else(|| state.get_install_dir())
        .unwrap_or_else(|| ".".to_string());

    match load_code(&dir) {
        Ok(code) => Ok(serde_json::json!({
            "success": true,
            "inviteCode": code
        })),
        Err(e) => Ok(serde_json::json!({
            "success": false,
            "message": e.to_string()
        })),
    }
}

/// Get system username
#[tauri::command]
pub fn get_system_username() -> String {
    get_username_from_system()
}
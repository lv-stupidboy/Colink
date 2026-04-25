use crate::error::{InstallerError, Result};
use serde::{Deserialize, Serialize};
use std::path::Path;

/// Invite verification response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InviteVerificationResponse {
    pub success: bool,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user: Option<UserInfo>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UserInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub username: Option<String>,
}

/// Verify invite code with backend API
pub async fn verify_invite_code(
    code: &str,
    username: &str,
    verify_url: Option<&str>,
) -> Result<InviteVerificationResponse> {
    let url = verify_url.unwrap_or("http://localhost:26305/api/v1/invite/verify");

    log::info!("Verifying invite code: code={}, username={}, url={}", code, username, url);

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "code": code,
        "username": username,
    });

    log::info!("Request body: {}", body.to_string());

    let response = client
        .post(url)
        .json(&body)
        .timeout(std::time::Duration::from_secs(10))
        .send()
        .await
        .map_err(|e| InstallerError::Network(e.to_string()))?;

    log::info!("Response status: {}", response.status());

    if response.status().is_success() {
        let result: InviteVerificationResponse = response
            .json()
            .await
            .map_err(|e| InstallerError::JsonParse(e.to_string()))?;
        log::info!("Verification result: success={}, message={}", result.success, result.message);
        Ok(result)
    } else {
        let status = response.status();
        let error_text = response
            .text()
            .await
            .unwrap_or_else(|_| "Unknown error".to_string());
        log::error!("Verification failed with status {}: {}", status, error_text);
        Err(InstallerError::Network(error_text))
    }
}

/// Save invite code to file
pub fn save_invite_code(install_dir: &str, invite_code: &str) -> Result<()> {
    let invite_path = Path::new(install_dir)
        .join("data")
        .join("invite-code.json");

    // Ensure directory exists
    std::fs::create_dir_all(invite_path.parent().unwrap())
        .map_err(|e| InstallerError::Io {
            context: "create data dir".to_string(),
            source: e,
        })?;

    let content = serde_json::json!({
        "inviteCode": invite_code,
    });

    crate::services::file_ops::atomic_write(&invite_path, content.to_string().as_bytes())?;
    Ok(())
}

/// Load invite code from file
pub fn load_invite_code(install_dir: &str) -> Result<Option<String>> {
    let invite_path = Path::new(install_dir)
        .join("data")
        .join("invite-code.json");

    if !invite_path.exists() {
        return Ok(None);
    }

    let content = std::fs::read_to_string(&invite_path)
        .map_err(|e| InstallerError::Io {
            context: "read invite code".to_string(),
            source: e,
        })?;

    let json: serde_json::Value = serde_json::from_str(&content)
        .map_err(|e| InstallerError::JsonParse(e.to_string()))?;

    Ok(json
        .get("inviteCode")
        .and_then(|v| v.as_str())
        .map(|s| s.to_string()))
}

/// Get system username
pub fn get_system_username() -> String {
    std::env::var("USERNAME")
        .or_else(|_| std::env::var("USER"))
        .unwrap_or_else(|_| "unknown".to_string())
}
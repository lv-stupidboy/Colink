use crate::error::{InstallerError, Result};
use serde::{Deserialize, Serialize};
use std::process::Command;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

#[cfg(target_os = "windows")]
const CREATE_NO_WINDOW: u32 = 0x08000000;

/// Dependency info
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DependencyInfo {
    pub key: String,
    pub name: String,
    pub installed: bool,
    pub version: Option<String>,
}

/// Check if a dependency is installed
pub fn check_dependency(key: &str) -> DependencyInfo {
    let (name, tool) = match key {
        "node" => ("Node.js", "node"),
        "git" => ("Git", "git"),
        "claude" => ("Claude CLI", "claude"),
        "opencode" => ("OpenCode", "opencode"),
        "claude-acp" => ("Claude ACP", "claude-agent-acp"),
        _ => ("Unknown", key),
    };

    let version = get_tool_version(tool);

    DependencyInfo {
        key: key.to_string(),
        name: name.to_string(),
        installed: version.is_some(),
        version,
    }
}

/// Get tool version string
fn get_tool_version(tool: &str) -> Option<String> {
    #[cfg(target_os = "windows")]
    {
        let output = Command::new("cmd")
            .args(["/C", &format!("{} --version", tool)])
            .creation_flags(CREATE_NO_WINDOW)
            .output();

        if let Ok(o) = output {
            if o.status.success() {
                let version = String::from_utf8_lossy(&o.stdout).trim().to_string();
                if !version.is_empty() {
                    return Some(version);
                }
            }
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        let output = Command::new(tool).arg("--version").output();

        if let Ok(o) = output {
            if o.status.success() {
                let version = String::from_utf8_lossy(&o.stdout).trim().to_string();
                if !version.is_empty() {
                    return Some(version);
                }
            }
        }
    }

    None
}

/// Check all dependencies (only agents)
pub fn check_all_dependencies() -> Vec<DependencyInfo> {
    let keys = ["claude", "opencode"];
    keys.iter().map(|k| check_dependency(k)).collect()
}

/// Install a dependency (npm package globally)
pub fn install_dependency(key: &str) -> Result<()> {
    #[cfg(target_os = "windows")]
    {
        let package = match key {
            "claude" => "@anthropic-ai/claude-code",
            "opencode" => "opencode",
            "claude-acp" => "@agentclientprotocol/claude-agent-acp",
            _ => return Err(InstallerError::DependencyNotFound(key.to_string())),
        };

        let output = Command::new("npm")
            .args(["install", "-g", package])
            .creation_flags(CREATE_NO_WINDOW)
            .output();

        match output {
            Ok(o) => {
                if !o.status.success() {
                    return Err(InstallerError::Process(
                        String::from_utf8_lossy(&o.stderr).to_string(),
                    ));
                }
                Ok(())
            }
            Err(e) => Err(InstallerError::Process(e.to_string())),
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        Ok(())
    }
}

/// Uninstall a dependency (npm package globally)
pub fn uninstall_dependency(key: &str) -> Result<()> {
    #[cfg(target_os = "windows")]
    {
        let package = match key {
            "claude" => "@anthropic-ai/claude-code",
            "opencode" => "opencode",
            "claude-acp" => "@agentclientprotocol/claude-agent-acp",
            _ => return Err(InstallerError::DependencyNotFound(key.to_string())),
        };

        let output = Command::new("npm")
            .args(["uninstall", "-g", package])
            .creation_flags(CREATE_NO_WINDOW)
            .output();

        match output {
            Ok(o) => {
                if !o.status.success() {
                    log::warn!("Failed to uninstall {}: {}", key, String::from_utf8_lossy(&o.stderr));
                }
                Ok(())
            }
            Err(e) => Err(InstallerError::Process(e.to_string())),
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        Ok(())
    }
}
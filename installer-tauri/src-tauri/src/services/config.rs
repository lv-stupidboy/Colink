use crate::error::{InstallerError, Result};
use crate::services::file_ops::atomic_write;
use std::path::Path;

/// Generate config preview for production deployment
/// Removes web section and all comments for clean release config
pub fn generate_config_preview(
    template_path: &Path,
    server_port: u16,
    _web_port: u16, // Unused in production, but kept for API compatibility
    database_type: &str,
) -> Result<String> {
    let template_content = std::fs::read_to_string(template_path)
        .map_err(|e| InstallerError::Io {
            context: "read template".to_string(),
            source: e,
        })?;

    // Process template: remove comments and web section, replace values
    let lines: Vec<&str> = template_content.lines().collect();
    let mut in_server_section = false;
    let mut in_database_section = false;
    let mut skip_web_section = false;
    let mut new_lines: Vec<String> = Vec::new();

    for line in lines {
        let trimmed = line.trim();

        // Skip all comment lines (lines starting with #)
        if trimmed.starts_with('#') {
            continue;
        }

        // Track section boundaries
        if trimmed == "web:" || (trimmed.starts_with("web:") && trimmed.len() == 4) {
            // Start of web section - skip entire section
            skip_web_section = true;
            continue;
        }

        // If we're in web section, skip until we exit
        if skip_web_section {
            // Check if we're exiting web section (non-indented, non-empty, non-comment line)
            if !trimmed.is_empty()
                && !line.starts_with(' ')
                && !line.starts_with('\t')
                && !trimmed.starts_with('#') {
                // This is a new top-level section, exit web section
                skip_web_section = false;
                // Process this line normally (it's a new section)
            } else {
                // Still in web section, skip this line
                continue;
            }
        }

        if trimmed.starts_with("server:") {
            in_server_section = true;
            in_database_section = false;
            new_lines.push(line.to_string());
        } else if trimmed.starts_with("database:") {
            in_database_section = true;
            in_server_section = false;
            new_lines.push(line.to_string());
        } else if !trimmed.is_empty()
                   && !line.starts_with(' ')
                   && !line.starts_with('\t') {
            // Non-indented, non-empty line means we're leaving a section
            if in_server_section || in_database_section {
                in_server_section = false;
                in_database_section = false;
            }
            new_lines.push(line.to_string());
        } else if in_server_section && trimmed.starts_with("port:") {
            // Replace server port
            let indent_len = line.len() - line.trim_start().len();
            let indent = " ".repeat(indent_len);
            new_lines.push(format!("{}port: {}", indent, server_port));
        } else if in_server_section && trimmed.starts_with("mode:") {
            // Keep mode setting but remove comments
            new_lines.push(line.to_string());
        } else if in_database_section && trimmed.starts_with("type:") {
            // Replace database type
            let indent_len = line.len() - line.trim_start().len();
            let indent = " ".repeat(indent_len);
            new_lines.push(format!("{}type: {}", indent, database_type));
        } else if in_database_section && trimmed.starts_with("path:") && database_type == "sqlite" {
            // Keep sqlite path
            let indent_len = line.len() - line.trim_start().len();
            let indent = " ".repeat(indent_len);
            new_lines.push(format!("{}path: ./data/sqlite/colink.db", indent));
        } else if !trimmed.is_empty() {
            // Keep other non-empty, non-comment lines (remove inline comments if any)
            // For lines with inline comments like "key: value # comment", keep only "key: value"
            if let Some(comment_pos) = line.find('#') {
                // Has inline comment - strip it
                let without_comment = &line[..comment_pos].trim_end();
                new_lines.push(without_comment.to_string());
            } else {
                new_lines.push(line.to_string());
            }
        }
    }

    Ok(new_lines.join("\n"))
}

/// Save config file
#[cfg(target_os = "windows")]
pub fn save_config_file(install_dir: &str, yaml_content: &str) -> Result<()> {
    let config_path = Path::new(install_dir)
        .join("data")
        .join("configs")
        .join("config.yaml");

    // Ensure directory exists
    std::fs::create_dir_all(config_path.parent().unwrap())
        .map_err(|e| InstallerError::Io {
            context: "create config dir".to_string(),
            source: e,
        })?;

    atomic_write(&config_path, yaml_content.as_bytes())?;
    Ok(())
}

#[cfg(target_os = "macos")]
pub fn save_config_file(_install_dir: &str, yaml_content: &str) -> Result<()> {
    // CRITICAL-01: Mac config is in ~/Library/Application Support/Colink/
    let data_dir = dirs::data_dir()
        .unwrap_or_else(|| std::path::PathBuf::from("~/.local/share"))
        .join("Colink");

    let config_path = data_dir.join("configs/config.yaml");

    // Ensure directory exists
    std::fs::create_dir_all(config_path.parent().unwrap())
        .map_err(|e| InstallerError::Io {
            context: "create config dir".to_string(),
            source: e,
        })?;

    atomic_write(&config_path, yaml_content.as_bytes())?;
    Ok(())
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
pub fn save_config_file(_install_dir: &str, _yaml_content: &str) -> Result<()> {
    Ok(())
}

/// Read existing config file content
#[cfg(target_os = "windows")]
pub fn read_config_file(install_dir: &str) -> Result<String> {
    let config_path = Path::new(install_dir)
        .join("data")
        .join("configs")
        .join("config.yaml");

    if !config_path.exists() {
        return Err(InstallerError::Config("配置文件不存在".into()));
    }

    std::fs::read_to_string(&config_path)
        .map_err(|e| InstallerError::Io {
            context: "read config".to_string(),
            source: e,
        })
}

#[cfg(target_os = "macos")]
pub fn read_config_file(_install_dir: &str) -> Result<String> {
    // CRITICAL-01: Mac config is in ~/Library/Application Support/Colink/
    let data_dir = dirs::data_dir()
        .unwrap_or_else(|| std::path::PathBuf::from("~/.local/share"))
        .join("Colink");

    let config_path = data_dir.join("configs/config.yaml");

    if !config_path.exists() {
        return Err(InstallerError::Config("配置文件不存在".into()));
    }

    std::fs::read_to_string(&config_path)
        .map_err(|e| InstallerError::Io {
            context: "read config".to_string(),
            source: e,
        })
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
pub fn read_config_file(_install_dir: &str) -> Result<String> {
    Err(InstallerError::Config("不支持的平台".into()))
}

/// Read existing config and extract server port
/// Note: web section is removed in production, only server port is relevant
pub fn read_existing_config(install_dir: &str) -> Result<(u16, u16)> {
    let content = read_config_file(install_dir)?;

    let mut server_port: u16 = 26305;
    let mut in_server_section = false;

    // Parse YAML to find server.port
    for line in content.lines() {
        let trimmed = line.trim();

        // Track section boundaries
        if trimmed.starts_with("server:") {
            in_server_section = true;
        } else if !trimmed.is_empty()
                   && !line.starts_with(' ')
                   && !line.starts_with('\t')
                   && !trimmed.starts_with('#') {
            // Non-indented, non-comment line means we're leaving a section
            in_server_section = false;
        } else if in_server_section && trimmed.starts_with("port:") {
            let port_str = trimmed.split(':').nth(1).unwrap_or("26305").trim();
            if let Ok(port) = port_str.parse::<u16>() {
                server_port = port;
            }
        }
    }

    // Return server_port twice for backward compatibility (web_port is no longer used)
    Ok((server_port, server_port))
}
#[cfg(target_os = "macos")]
use crate::error::{InstallerError, Result};

#[cfg(not(target_os = "macos"))]
use crate::error::Result;

use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[cfg(target_os = "macos")]
use plist;

/// Information about installed version (extended with data_dir)
/// CRITICAL-01: data_dir is separate from install_dir (App Bundle)
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InstalledVersionPlist {
    pub installed: bool,
    pub install_dir: Option<String>,  // App Bundle path (/Applications/Colink.app)
    pub data_dir: Option<String>,     // CRITICAL-01: User data directory (~/Library/Application Support/Colink/)
    pub version: Option<String>,
    pub has_data: Option<bool>,
}

/// Get Mac data directory (CRITICAL-01: outside App Bundle)
#[cfg(target_os = "macos")]
pub fn get_mac_data_dir() -> PathBuf {
    dirs::data_dir()
        .unwrap_or_else(|| PathBuf::from("~/.local/share"))
        .join("Colink")
}

#[cfg(not(target_os = "macos"))]
pub fn get_mac_data_dir() -> PathBuf {
    PathBuf::new()
}

/// Get installed version from Mac App Bundle and plist
#[cfg(target_os = "macos")]
pub fn get_installed_version_plist() -> Result<InstalledVersionPlist> {
    let app_path = PathBuf::from("/Applications/Colink.app");

    if !app_path.exists() {
        return Ok(InstalledVersionPlist::default());
    }

    // Read Info.plist for version
    let plist_path = app_path.join("Contents/Info.plist");
    let version = parse_plist_version(&plist_path)?;

    // CRITICAL-01: Check data directory in ~/Library/Application Support/
    let data_dir = get_mac_data_dir();
    let has_data = data_dir.join("sqlite/colink.db").exists();

    Ok(InstalledVersionPlist {
        installed: true,
        install_dir: Some(app_path.to_string_lossy().to_string()),
        data_dir: Some(data_dir.to_string_lossy().to_string()),
        version,
        has_data: Some(has_data),
    })
}

#[cfg(not(target_os = "macos"))]
pub fn get_installed_version_plist() -> Result<InstalledVersionPlist> {
    Ok(InstalledVersionPlist::default())
}

/// Parse version from Info.plist using plist crate
#[cfg(target_os = "macos")]
fn parse_plist_version(plist_path: &PathBuf) -> Result<Option<String>> {
    if !plist_path.exists() {
        return Ok(None);
    }

    let plist_value = plist::Value::from_file(plist_path).map_err(|e| InstallerError::Io {
        context: "parse Info.plist".to_string(),
        source: std::io::Error::new(std::io::ErrorKind::Other, e),
    })?;

    if let Some(dict) = plist_value.as_dictionary() {
        if let Some(version) = dict.get("CFBundleShortVersionString").and_then(|v| v.as_string()) {
            return Ok(Some(version.to_string()));
        }
    }
    Ok(None)
}

#[cfg(not(target_os = "macos"))]
#[allow(dead_code)]
fn parse_plist_version(_plist_path: &PathBuf) -> Result<Option<String>> {
    Ok(None)
}

/// Write installation plist (for user preferences)
#[cfg(target_os = "macos")]
pub fn write_install_plist(install_dir: &str, data_dir: &str, version: &str) -> Result<()> {
    let prefs_dir = dirs::home_dir()
        .map(|h| h.join("Library/Preferences"))
        .unwrap_or_default();

    let plist_path = prefs_dir.join("com.colink.installer.plist");

    // Ensure directory exists
    if !prefs_dir.exists() {
        std::fs::create_dir_all(&prefs_dir).map_err(|e| InstallerError::Io {
            context: "create Preferences directory".to_string(),
            source: e,
        })?;
    }

    // Build plist dictionary
    let mut plist_dict = plist::Dictionary::new();
    plist_dict.insert("InstallDir".to_string(), plist::Value::String(install_dir.to_string()));
    plist_dict.insert("DataDir".to_string(), plist::Value::String(data_dir.to_string()));  // CRITICAL-01
    plist_dict.insert("Version".to_string(), plist::Value::String(version.to_string()));
    plist_dict.insert("InstallDate".to_string(), plist::Value::String(
        chrono::Local::now().format("%Y-%m-%d").to_string()
    ));

    plist::Value::Dictionary(plist_dict).to_file_xml(&plist_path).map_err(|e| InstallerError::Io {
        context: "write install plist".to_string(),
        source: std::io::Error::new(std::io::ErrorKind::Other, e),
    })?;

    Ok(())
}

#[cfg(not(target_os = "macos"))]
pub fn write_install_plist(_install_dir: &str, _data_dir: &str, _version: &str) -> Result<()> {
    Ok(())
}

/// Delete installation plist
#[cfg(target_os = "macos")]
pub fn delete_install_plist() -> Result<()> {
    let prefs_dir = dirs::home_dir()
        .map(|h| h.join("Library/Preferences"))
        .unwrap_or_default();

    let plist_path = prefs_dir.join("com.colink.installer.plist");
    if plist_path.exists() {
        std::fs::remove_file(plist_path).map_err(|e| InstallerError::Io {
            context: "delete install plist".to_string(),
            source: e,
        })?;
    }
    Ok(())
}

#[cfg(not(target_os = "macos"))]
pub fn delete_install_plist() -> Result<()> {
    Ok(())
}
use crate::error::{InstallerError, Result};
use serde::{Deserialize, Serialize};

#[cfg(target_os = "windows")]
use winreg::RegKey;
#[cfg(target_os = "windows")]
use winreg::enums::*;

/// Information about installed version
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InstalledVersion {
    pub installed: bool,
    pub install_dir: Option<String>,
    pub version: Option<String>,
    pub has_data: Option<bool>,
}

/// Get installed version from Windows registry
#[cfg(target_os = "windows")]
pub fn get_installed_version() -> Result<InstalledVersion> {
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let hklm = RegKey::predef(HKEY_LOCAL_MACHINE);

    log::info!("Checking registry for installed version...");

    // Try HKLM first
    log::info!("Trying HKLM for Colink uninstall key...");
    if let Ok(key) = hklm.open_subkey("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink") {
        log::info!("Found Colink key in HKLM");
        if let Some(dir) = extract_install_info(&key, "HKLM") {
            return Ok(dir);
        }
    }

    // Try HKCU
    log::info!("Trying HKCU for Colink uninstall key...");
    if let Ok(key) = hkcu.open_subkey("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink") {
        log::info!("Found Colink key in HKCU");
        if let Some(info) = extract_install_info(&key, "HKCU") {
            return Ok(info);
        }
    }

    log::info!("No Colink registry entry found");
    Ok(InstalledVersion::default())
}

#[cfg(target_os = "windows")]
fn extract_install_info(key: &RegKey, root_name: &str) -> Option<InstalledVersion> {
    let install_dir: Option<String> = key.get_value("InstallLocation").ok();
    log::info!("InstallLocation from {}: {:?}", root_name, install_dir);

    if let Some(dir) = &install_dir {
        let version: Option<String> = key.get_value("DisplayVersion").ok();
        let has_data = std::path::Path::new(dir).join("data").exists();
        log::info!("Colink installed at: {}, has_data: {}", dir, has_data);

        return Some(InstalledVersion {
            installed: true,
            install_dir: Some(dir.clone()),
            version,
            has_data: Some(has_data),
        });
    } else {
        // Key exists but no InstallLocation - return partial info
        let version: Option<String> = key.get_value("DisplayVersion").ok();
        log::info!("Colink key found but no InstallLocation");
        return Some(InstalledVersion {
            installed: true,
            install_dir: None,
            version,
            has_data: None,
        });
    }
}

#[cfg(not(target_os = "windows"))]
#[cfg(target_os = "macos")]
pub fn get_installed_version() -> Result<InstalledVersion> {
    // Mac uses plist instead of registry
    use crate::services::plist::get_installed_version_plist;

    let plist_info = get_installed_version_plist()?;

    // Convert InstalledVersionPlist to InstalledVersion
    Ok(InstalledVersion {
        installed: plist_info.installed,
        install_dir: plist_info.install_dir,
        version: plist_info.version,
        has_data: plist_info.has_data,
    })
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
pub fn get_installed_version() -> Result<InstalledVersion> {
    Ok(InstalledVersion::default())
}

/// Check old ISDP installation (brand rename migration)
#[cfg(target_os = "windows")]
pub fn get_old_isdp_version() -> Result<InstalledVersion> {
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let hklm = RegKey::predef(HKEY_LOCAL_MACHINE);

    for root in [hklm, hkcu] {
        if let Ok(key) = root.open_subkey("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP") {
            let install_dir: String = key
                .get_value("InstallLocation")
                .map_err(|e| InstallerError::Registry(e.to_string()))?;

            let version: Option<String> = key.get_value("DisplayVersion").ok();

            return Ok(InstalledVersion {
                installed: true,
                install_dir: Some(install_dir),
                version,
                has_data: None,
            });
        }
    }

    Ok(InstalledVersion::default())
}

#[cfg(not(target_os = "windows"))]
pub fn get_old_isdp_version() -> Result<InstalledVersion> {
    Ok(InstalledVersion::default())
}

/// Write registry entry for uninstall
#[cfg(target_os = "windows")]
pub fn write_registry(install_dir: &str, version: &str) -> Result<()> {
    use std::path::PathBuf;

    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let hklm = RegKey::predef(HKEY_LOCAL_MACHINE);

    let launcher_path = PathBuf::from(install_dir).join("Colink.exe");
    let launcher_path_str = launcher_path.to_string_lossy().to_string();

    // Try HKLM first, fallback to HKCU if permission denied
    let result = try_write_to_root(&hklm, install_dir, version, &launcher_path_str);
    if result.is_err() {
        try_write_to_root(&hkcu, install_dir, version, &launcher_path_str)?;
    }

    Ok(())
}

#[cfg(target_os = "windows")]
fn try_write_to_root(
    root: &RegKey,
    install_dir: &str,
    version: &str,
    launcher_path: &str,
) -> Result<()> {
    let (key, _) = root
        .create_subkey("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink")
        .map_err(|e| InstallerError::Registry(e.to_string()))?;

    key.set_value("DisplayName", &"Colink")?;
    key.set_value("DisplayVersion", &version)?;
    key.set_value("Publisher", &"Colink Team")?;
    key.set_value("InstallLocation", &install_dir)?;
    key.set_value("DisplayIcon", &launcher_path)?;
    key.set_value("NoModify", &1u32)?;
    key.set_value("NoRepair", &1u32)?;

    Ok(())
}

#[cfg(not(target_os = "windows"))]
#[cfg(target_os = "macos")]
pub fn write_registry(install_dir: &str, version: &str) -> Result<()> {
    // Mac uses plist instead of registry
    use crate::services::plist::{write_install_plist, get_mac_data_dir};

    let data_dir = get_mac_data_dir();
    write_install_plist(install_dir, &data_dir.to_string_lossy(), version)?;
    Ok(())
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
pub fn write_registry(_install_dir: &str, _version: &str) -> Result<()> {
    Ok(())
}

/// Delete registry entry
#[cfg(target_os = "windows")]
pub fn delete_registry() -> Result<()> {
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let hklm = RegKey::predef(HKEY_LOCAL_MACHINE);

    // Delete from both locations
    let _ = hklm.delete_subkey_all("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink");
    let _ = hkcu.delete_subkey_all("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Colink");

    Ok(())
}

#[cfg(not(target_os = "windows"))]
#[cfg(target_os = "macos")]
pub fn delete_registry() -> Result<()> {
    // Mac uses plist instead of registry
    use crate::services::plist::delete_install_plist;

    delete_install_plist()?;
    Ok(())
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
pub fn delete_registry() -> Result<()> {
    Ok(())
}

/// Uninstall old ISDP silently
#[cfg(target_os = "windows")]
pub fn uninstall_old_isdp(install_dir: &str) -> Result<()> {
    use std::process::Command;
    use std::os::windows::process::CommandExt;

    const CREATE_NO_WINDOW: u32 = 0x08000000;

    // Find uninstaller
    let uninstaller = std::path::Path::new(install_dir).join("uninstall.exe");
    if !uninstaller.exists() {
        return Ok(());
    }

    // Run silent uninstall
    let output = Command::new(&uninstaller)
        .args(["/S"])
        .creation_flags(CREATE_NO_WINDOW)
        .output();

    match output {
        Ok(o) => {
            if !o.status.success() {
                log::warn!("Old ISDP uninstall failed: {}", String::from_utf8_lossy(&o.stderr));
            }
        }
        Err(e) => {
            log::warn!("Failed to run old ISDP uninstaller: {}", e);
        }
    }

    // Also delete registry
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let hklm = RegKey::predef(HKEY_LOCAL_MACHINE);
    let _ = hklm.delete_subkey_all("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP");
    let _ = hkcu.delete_subkey_all("Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\ISDP");

    Ok(())
}

#[cfg(not(target_os = "windows"))]
pub fn uninstall_old_isdp(_install_dir: &str) -> Result<()> {
    Ok(())
}
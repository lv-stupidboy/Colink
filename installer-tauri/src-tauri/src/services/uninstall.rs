use crate::error::{InstallerError, Result};
use crate::services::file_ops::{delete_except_whitelist, move_to_backup, kill_all_processes};
use crate::services::registry::{delete_registry, get_installed_version};
use crate::services::shortcut::delete_all_shortcuts;
use std::path::Path;

/// Uninstall the application
pub fn uninstall(install_dir: &str, keep_data: bool) -> Result<()> {
    let dir = Path::new(install_dir);

    if !dir.exists() {
        return Err(InstallerError::NotInstalled);
    }

    // Step 1: Kill running processes
    kill_all_processes("colink-server.exe")?;
    kill_all_processes("Colink.exe")?;

    // Step 2: Delete shortcuts
    delete_all_shortcuts()?;

    // Step 3: Clean backup directory from previous upgrade
    let backup_dir = dir.join("backup");
    if backup_dir.exists() {
        std::fs::remove_dir_all(&backup_dir)
            .map_err(|e| InstallerError::Io {
                context: "remove backup".to_string(),
                source: e,
            })?;
    }

    // Step 4: Handle data directory
    if keep_data {
        // Whitelist: keep data only (old Electron resources are obsolete)
        let whitelist = ["data"];
        delete_except_whitelist(dir, &whitelist)?;
    } else {
        // Full uninstall - delete everything
        std::fs::remove_dir_all(dir)
            .map_err(|e| InstallerError::Io {
                context: "remove install dir".to_string(),
                source: e,
            })?;
    }

    // Step 5: Delete registry entry
    delete_registry()?;

    Ok(())
}

/// Prepare for upgrade - move non-whitelisted items to backup
pub fn prepare_upgrade(install_dir: &str) -> Result<()> {
    let dir = Path::new(install_dir);

    if !dir.exists() {
        return Ok(()); // Fresh install, no upgrade needed
    }

    // Check for running processes
    if crate::services::file_ops::is_process_running("Colink.exe")? {
        return Err(InstallerError::ProcessAlreadyRunning("Colink.exe".into()));
    }
    if crate::services::file_ops::is_process_running("colink-server.exe")? {
        return Err(InstallerError::ProcessAlreadyRunning("colink-server.exe".into()));
    }

    // Delete shortcuts
    delete_all_shortcuts()?;

    // Clean old backup
    let backup_dir = dir.join("backup");
    if backup_dir.exists() {
        std::fs::remove_dir_all(&backup_dir)
            .map_err(|e| InstallerError::Io {
                context: "remove old backup".to_string(),
                source: e,
            })?;
    }

    // Move non-whitelisted items to backup (atomic rename on same drive)
    // Include 'backup' in whitelist since it's the destination and was just created
    // Note: Do NOT whitelist 'resources' - old Electron resources are obsolete
    let whitelist = ["data", "backup"];
    move_to_backup(dir, &backup_dir, &whitelist)?;

    // Delete registry (will be recreated after upgrade)
    delete_registry()?;

    Ok(())
}

/// Get startup action based on existing installation
pub fn get_startup_action() -> Result<String> {
    let installed = get_installed_version()?;

    if !installed.installed {
        return Ok("install".to_string());
    }

    // If installed, check if upgrade or allow uninstall
    Ok("upgrade".to_string())
}

/// Remove shortcuts only (for uninstall)
pub fn remove_shortcuts() -> Result<()> {
    delete_all_shortcuts()?;
    Ok(())
}
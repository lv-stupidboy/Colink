use crate::error::{InstallerError, Result};
use crate::services::file_ops::{delete_except_whitelist, move_to_backup, kill_all_processes, remove_dir_all_with_retry, cleanup_staging_dirs};
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
    #[cfg(target_os = "windows")]
    {
        kill_all_processes("colink-server.exe")?;
        kill_all_processes("Colink.exe")?;
    }

    #[cfg(target_os = "macos")]
    {
        // CRITICAL-02: Mac process names without .exe
        kill_all_processes("colink-server")?;
        kill_all_processes("Colink")?;
    }

    // Step 2: Delete shortcuts
    delete_all_shortcuts()?;

    // Step 3: Clean backup directory from previous upgrade
    // Use retry mechanism for Windows temporary file locks
    let backup_dir = dir.join("backup");
    if backup_dir.exists() {
        remove_dir_all_with_retry(&backup_dir, 3, 500)?;
    }

    // Step 4: Handle data directory
    if keep_data {
        // Whitelist: keep data only (old Electron resources are obsolete)
        let whitelist = ["data"];
        delete_except_whitelist(dir, &whitelist)?;
    } else {
        // Full uninstall - delete everything
        // Use retry mechanism for Windows temporary file locks
        remove_dir_all_with_retry(dir, 3, 500)?;
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
    #[cfg(target_os = "windows")]
    {
        if crate::services::file_ops::is_process_running("Colink.exe")? {
            return Err(InstallerError::ProcessAlreadyRunning("Colink.exe".into()));
        }
        if crate::services::file_ops::is_process_running("colink-server.exe")? {
            return Err(InstallerError::ProcessAlreadyRunning("colink-server.exe".into()));
        }
    }

    #[cfg(target_os = "macos")]
    {
        // CRITICAL-02: Mac process names without .exe
        if crate::services::file_ops::is_process_running("Colink")? {
            return Err(InstallerError::ProcessAlreadyRunning("Colink".into()));
        }
        if crate::services::file_ops::is_process_running("colink-server")? {
            return Err(InstallerError::ProcessAlreadyRunning("colink-server".into()));
        }
    }

    // Delete shortcuts
    delete_all_shortcuts()?;

    // Clean old backup directory
    // Important: backup may contain executable files from previous failed install
    // Kill processes first to ensure files are not locked
    log::info!("Preparing to clean old backup directory");

    #[cfg(target_os = "windows")]
    {
        kill_all_processes("colink-server.exe")?;
        kill_all_processes("Colink.exe")?;
    }

    #[cfg(target_os = "macos")]
    {
        // CRITICAL-02: Mac process names without .exe
        kill_all_processes("colink-server")?;
        kill_all_processes("Colink")?;
    }

    let backup_dir = dir.join("backup");
    if backup_dir.exists() {
        log::info!("Removing old backup directory: {}", backup_dir.display());
        // Use retry mechanism to handle temporary file locks (Windows Defender, etc.)
        remove_dir_all_with_retry(&backup_dir, 3, 500)?;
        log::info!("Old backup directory removed successfully");
    }

    // Clean up any leftover .staging-* directories from previous failed installs
    // These are temp dirs created by atomic_copy_dir that weren't cleaned up
    log::info!("Cleaning up leftover staging directories");
    cleanup_staging_dirs(dir)?;

    // Move non-whitelisted items to backup (atomic rename on same drive)
    // Include 'backup' in whitelist since it's the destination and was just created
    // Hidden files/dirs (starting with '.') are automatically skipped by move_to_backup
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
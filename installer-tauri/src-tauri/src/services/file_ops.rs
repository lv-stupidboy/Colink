use crate::error::{InstallerError, Result};
use std::fs;
use std::path::Path;

/// Atomic file write: write to temp file, then rename
pub fn atomic_write(path: &Path, content: &[u8]) -> Result<()> {
    let temp_path = path.with_extension("tmp");

    fs::write(&temp_path, content)
        .map_err(|e| InstallerError::Io {
            context: format!("write temp file: {}", path.display()),
            source: e,
        })?;

    fs::rename(&temp_path, path)
        .map_err(|e| {
            let _ = fs::remove_file(&temp_path);
            InstallerError::Io {
                context: format!("rename to final: {}", path.display()),
                source: e,
            }
        })?;

    Ok(())
}

/// Atomic directory copy with staging
pub fn atomic_copy_dir(src: &Path, dest: &Path) -> Result<()> {
    // Create staging directory on same drive (for atomic rename on Windows)
    let parent = dest.parent().unwrap_or(dest);
    let staging = parent.join(format!(".staging-{}", std::process::id()));

    if staging.exists() {
        remove_dir_all_with_retry(&staging, 3, 500)?;
    }

    copy_dir_recursive(src, &staging)?;

    // Atomic replace
    if dest.exists() {
        // Use retry mechanism for Windows temporary file locks
        remove_dir_all_with_retry(dest, 3, 500)?;
    }

    fs::rename(&staging, dest)
        .map_err(|e| {
            let _ = remove_dir_all_with_retry(&staging, 1, 0);
            InstallerError::Io {
                context: format!("rename staging to dest: {}", dest.display()),
                source: e,
            }
        })?;

    Ok(())
}

/// Copy directory recursively
pub fn copy_dir_recursive(src: &Path, dest: &Path) -> Result<()> {
    fs::create_dir_all(dest)
        .map_err(|e| InstallerError::Io {
            context: format!("创建目录: {}", dest.display()),
            source: e,
        })?;

    let entries = fs::read_dir(src)
        .map_err(|e| InstallerError::Io {
            context: format!("读取源目录: {}", src.display()),
            source: e,
        })?;

    for entry in entries {
        let entry = entry.map_err(|e| InstallerError::Io {
            context: "读取目录项".to_string(),
            source: e,
        })?;

        let src_path = entry.path();
        let dest_path = dest.join(entry.file_name());

        let file_type = entry
            .file_type()
            .map_err(|e| InstallerError::Io {
                context: format!("获取文件类型: {}", src_path.display()),
                source: e,
            })?;

        if file_type.is_dir() {
            copy_dir_recursive(&src_path, &dest_path)?;
        } else {
            fs::copy(&src_path, &dest_path)
                .map_err(|e| InstallerError::Io {
                    context: format!("复制文件: {} -> {}", src_path.display(), dest_path.display()),
                    source: e,
                })?;
        }
    }

    Ok(())
}

/// Remove directory with retry on Windows (handles "access denied" due to virus scan etc)
/// Windows often returns "access denied" when a file is temporarily locked by:
/// - Windows Defender scanning
/// - Windows Search Indexer
/// - Other processes reading the file
#[cfg(target_os = "windows")]
pub fn remove_dir_all_with_retry(path: &Path, max_retries: u32, retry_delay_ms: u64) -> Result<()> {
    use std::thread;
    use std::time::Duration;

    for attempt in 0..max_retries {
        if !path.exists() {
            return Ok(());
        }

        let result = fs::remove_dir_all(path);
        match result {
            Ok(_) => return Ok(()),
            Err(e) => {
                let os_code = e.raw_os_error().unwrap_or(0);
                // Windows error 5 = ACCESS_DENIED
                // Windows error 32 = SHARING_VIOLATION (file in use)
                if os_code == 5 || os_code == 32 {
                    if attempt < max_retries - 1 {
                        log::warn!(
                            "remove_dir_all failed (attempt {}), retrying after {}ms: {} (os error {})",
                            attempt + 1,
                            retry_delay_ms,
                            path.display(),
                            os_code
                        );
                        thread::sleep(Duration::from_millis(retry_delay_ms));
                        continue;
                    }
                }
                return Err(InstallerError::Io {
                    context: format!("remove directory (after {} retries): {}", max_retries, path.display()),
                    source: e,
                });
            }
        }
    }
    Ok(())
}

#[cfg(not(target_os = "windows"))]
pub fn remove_dir_all_with_retry(path: &Path, _max_retries: u32, _retry_delay_ms: u64) -> Result<()> {
    if path.exists() {
        fs::remove_dir_all(path)
            .map_err(|e| InstallerError::Io {
                context: format!("remove directory: {}", path.display()),
                source: e,
            })?;
    }
    Ok(())
}

/// Remove file with retry on Windows (handles "access denied" due to virus scan etc)
#[cfg(target_os = "windows")]
pub fn remove_file_with_retry(path: &Path, max_retries: u32, retry_delay_ms: u64) -> Result<()> {
    use std::thread;
    use std::time::Duration;

    for attempt in 0..max_retries {
        if !path.exists() {
            return Ok(());
        }

        let result = fs::remove_file(path);
        match result {
            Ok(_) => return Ok(()),
            Err(e) => {
                let os_code = e.raw_os_error().unwrap_or(0);
                // Windows error 5 = ACCESS_DENIED
                // Windows error 32 = SHARING_VIOLATION (file in use)
                if os_code == 5 || os_code == 32 {
                    if attempt < max_retries - 1 {
                        log::warn!(
                            "remove_file failed (attempt {}), retrying after {}ms: {} (os error {})",
                            attempt + 1,
                            retry_delay_ms,
                            path.display(),
                            os_code
                        );
                        thread::sleep(Duration::from_millis(retry_delay_ms));
                        continue;
                    }
                }
                return Err(InstallerError::Io {
                    context: format!("remove file (after {} retries): {}", max_retries, path.display()),
                    source: e,
                });
            }
        }
    }
    Ok(())
}

#[cfg(not(target_os = "windows"))]
pub fn remove_file_with_retry(path: &Path, _max_retries: u32, _retry_delay_ms: u64) -> Result<()> {
    if path.exists() {
        fs::remove_file(path)
            .map_err(|e| InstallerError::Io {
                context: format!("remove file: {}", path.display()),
                source: e,
            })?;
    }
    Ok(())
}

/// Delete directory contents except whitelisted items
pub fn delete_except_whitelist(dir: &Path, whitelist: &[&str]) -> Result<()> {
    let entries = fs::read_dir(dir)
        .map_err(|e| InstallerError::Io {
            context: format!("read dir: {}", dir.display()),
            source: e,
        })?;

    for entry in entries {
        let entry = entry.map_err(|e| InstallerError::Io {
            context: "read entry".to_string(),
            source: e,
        })?;

        let name = entry.file_name().to_string_lossy().to_string();
        if whitelist.contains(&name.as_str()) {
            continue;
        }

        let path = entry.path();
        if path.is_dir() {
            // Use retry mechanism for Windows temporary file locks
            remove_dir_all_with_retry(&path, 3, 500)?;
        } else {
            // Retry for file deletion as well
            remove_file_with_retry(&path, 3, 500)?;
        }
    }

    Ok(())
}

/// Move non-whitelisted items to backup directory
pub fn move_to_backup(dir: &Path, backup_dir: &Path, whitelist: &[&str]) -> Result<()> {
    fs::create_dir_all(backup_dir)
        .map_err(|e| InstallerError::Io {
            context: format!("create backup dir: {}", backup_dir.display()),
            source: e,
        })?;

    let entries = fs::read_dir(dir)
        .map_err(|e| InstallerError::Io {
            context: format!("read dir: {}", dir.display()),
            source: e,
        })?;

    for entry in entries {
        let entry = entry.map_err(|e| InstallerError::Io {
            context: "read entry".to_string(),
            source: e,
        })?;

        let name = entry.file_name().to_string_lossy().to_string();
        if whitelist.contains(&name.as_str()) {
            continue;
        }

        let src_path = entry.path();
        let dest_path = backup_dir.join(&name);

        // Rename is atomic on same drive
        fs::rename(&src_path, &dest_path)
            .map_err(|e| InstallerError::Io {
                context: format!("move to backup: {}", src_path.display()),
                source: e,
            })?;
    }

    Ok(())
}

/// Check if a process is running (Windows)
#[cfg(target_os = "windows")]
pub fn is_process_running(process_name: &str) -> Result<bool> {
    use std::os::windows::process::CommandExt;
    use std::process::Command;

    const CREATE_NO_WINDOW: u32 = 0x08000000;

    let output = Command::new("tasklist")
        .args(["/fi", &format!("imagename eq {}", process_name), "/fo", "csv"])
        .creation_flags(CREATE_NO_WINDOW)
        .output();

    match output {
        Ok(o) => {
            let stdout = String::from_utf8_lossy(&o.stdout);
            // CSV format: "Image Name","PID","Session Name","Session#","Mem Usage"
            // If process exists, there will be more than just header line
            let lines: Vec<&str> = stdout.trim().split('\n').collect();
            // Check if any line contains the process name (not just header)
            for line in lines.iter().skip(1) {
                if line.contains(process_name) {
                    return Ok(true);
                }
            }
            Ok(false)
        }
        Err(e) => Err(InstallerError::Process(e.to_string())),
    }
}

#[cfg(not(target_os = "windows"))]
pub fn is_process_running(_process_name: &str) -> Result<bool> {
    Ok(false)
}

/// Kill all processes with given name (Windows)
#[cfg(target_os = "windows")]
pub fn kill_all_processes(process_name: &str) -> Result<()> {
    use std::os::windows::process::CommandExt;
    use std::process::Command;

    const CREATE_NO_WINDOW: u32 = 0x08000000;

    let output = Command::new("taskkill")
        .args(["/f", "/im", process_name])
        .creation_flags(CREATE_NO_WINDOW)
        .output();

    match output {
        Ok(_) => Ok(()),
        Err(e) => Err(InstallerError::Process(e.to_string())),
    }
}

#[cfg(not(target_os = "windows"))]
pub fn kill_all_processes(_process_name: &str) -> Result<()> {
    Ok(())
}
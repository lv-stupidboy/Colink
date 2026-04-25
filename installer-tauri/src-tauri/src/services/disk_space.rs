use crate::error::Result;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiskSpace {
    pub free: u64,
    pub total: u64,
}

/// Get disk space for a given path (Windows only)
#[cfg(target_os = "windows")]
pub fn get_disk_space(path: &str) -> Result<DiskSpace> {
    use std::os::windows::process::CommandExt;
    use std::process::Command;

    const CREATE_NO_WINDOW: u32 = 0x08000000;

    // Extract drive letter (support D: or D:\ format)
    let drive_chars: String = path.chars().take(2).collect();
    if drive_chars.len() < 2 || !drive_chars.chars().next().unwrap().is_ascii_alphabetic() {
        return Ok(DiskSpace { free: 0, total: 0 });
    }

    let drive_letter = drive_chars.chars().next().unwrap();

    // Method 1: PowerShell (recommended for Windows 10+)
    let output = Command::new("powershell")
        .args([
            "-Command",
            &format!(
                "(Get-PSDrive -Name '{}').Free",
                drive_letter
            ),
        ])
        .creation_flags(CREATE_NO_WINDOW)
        .output();

    if let Ok(out) = output {
        let stdout = String::from_utf8_lossy(&out.stdout);
        let free: u64 = stdout.trim().parse().unwrap_or(0);

        if free > 0 {
            // Get total (free + used)
            let output2 = Command::new("powershell")
                .args([
                    "-Command",
                    &format!(
                        "(Get-PSDrive -Name '{}').Used + (Get-PSDrive -Name '{}').Free",
                        drive_letter, drive_letter
                    ),
                ])
                .creation_flags(CREATE_NO_WINDOW)
                .output();

            let total: u64 = if let Ok(out2) = output2 {
                String::from_utf8_lossy(&out2.stdout).trim().parse().unwrap_or(free)
            } else {
                free
            };

            return Ok(DiskSpace { free, total });
        }
    }

    // Method 2: Fallback to wmic
    let output = Command::new("wmic")
        .args([
            "logicaldisk",
            "where",
            &format!("DeviceID='{}:'", drive_letter),
            "get",
            "FreeSpace,Size",
            "/format:csv",
        ])
        .creation_flags(CREATE_NO_WINDOW)
        .output();

    if let Ok(out) = output {
        let stdout = String::from_utf8_lossy(&out.stdout);
        for line in stdout.trim().split('\n').skip(1) {
            let parts: Vec<&str> = line.split(',').collect();
            if parts.len() >= 3 {
                let free: u64 = parts[1].parse().unwrap_or(0);
                let total: u64 = parts[2].parse().unwrap_or(0);
                if free > 0 || total > 0 {
                    return Ok(DiskSpace { free, total });
                }
            }
        }
    }

    Ok(DiskSpace { free: 0, total: 0 })
}

#[cfg(not(target_os = "windows"))]
pub fn get_disk_space(_path: &str) -> Result<DiskSpace> {
    Ok(DiskSpace { free: 0, total: 0 })
}
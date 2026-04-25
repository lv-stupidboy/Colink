use crate::error::{InstallerError, Result};
use std::path::PathBuf;
use std::process::Command;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

#[cfg(target_os = "windows")]
const CREATE_NO_WINDOW: u32 = 0x08000000;

/// Create desktop shortcut
pub fn create_desktop_shortcut(install_dir: &str) -> Result<()> {
    let launcher_path = PathBuf::from(install_dir).join("Colink.exe");
    let desktop_path = std::env::var("USERPROFILE")
        .map(|p| PathBuf::from(p).join("Desktop").join("Colink.lnk"))
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    create_shortcut(&launcher_path, &desktop_path, install_dir)?;
    Ok(())
}

/// Create start menu shortcut
pub fn create_start_menu_shortcut(install_dir: &str) -> Result<()> {
    let launcher_path = PathBuf::from(install_dir).join("Colink.exe");
    let start_menu_path = std::env::var("APPDATA")
        .map(|p| {
            PathBuf::from(p)
                .join("Microsoft")
                .join("Windows")
                .join("Start Menu")
                .join("Programs")
                .join("Colink.lnk")
        })
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    create_shortcut(&launcher_path, &start_menu_path, install_dir)?;
    Ok(())
}

/// Create shortcut using VBScript (Windows)
fn create_shortcut(target: &PathBuf, shortcut_path: &PathBuf, working_dir: &str) -> Result<()> {
    let vbs_content = format!(
        r#"Set WshShell = WScript.CreateObject("WScript.Shell")
Set oShellLink = WshShell.CreateShortcut("{}")
oShellLink.TargetPath = "{}"
oShellLink.WorkingDirectory = "{}"
oShellLink.Description = "Colink"
oShellLink.Save
"#,
        shortcut_path.display(),
        target.display(),
        working_dir
    );

    let temp_dir = std::env::temp_dir();
    let vbs_path = temp_dir.join("create_shortcut_colink.vbs");

    std::fs::write(&vbs_path, &vbs_content)
        .map_err(|e| InstallerError::Io {
            context: "write vbs script".to_string(),
            source: e,
        })?;

    #[cfg(target_os = "windows")]
    let output = Command::new("cscript")
        .args(["//nologo", &vbs_path.to_string_lossy()])
        .creation_flags(CREATE_NO_WINDOW)
        .output();

    #[cfg(not(target_os = "windows"))]
    let output = Command::new("cscript")
        .args(["//nologo", &vbs_path.to_string_lossy()])
        .output();

    // Clean up temp file
    let _ = std::fs::remove_file(&vbs_path);

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

/// Delete desktop shortcut
pub fn delete_desktop_shortcut() -> Result<()> {
    let desktop_path = std::env::var("USERPROFILE")
        .map(|p| PathBuf::from(p).join("Desktop").join("Colink.lnk"))
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    if desktop_path.exists() {
        std::fs::remove_file(&desktop_path)
            .map_err(|e| InstallerError::Io {
                context: "delete desktop shortcut".to_string(),
                source: e,
            })?;
    }
    Ok(())
}

/// Delete start menu shortcut
pub fn delete_start_menu_shortcut() -> Result<()> {
    let start_menu_path = std::env::var("APPDATA")
        .map(|p| {
            PathBuf::from(p)
                .join("Microsoft")
                .join("Windows")
                .join("Start Menu")
                .join("Programs")
                .join("Colink.lnk")
        })
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    if start_menu_path.exists() {
        std::fs::remove_file(&start_menu_path)
            .map_err(|e| InstallerError::Io {
                context: "delete start menu shortcut".to_string(),
                source: e,
            })?;
    }
    Ok(())
}

/// Delete all shortcuts
pub fn delete_all_shortcuts() -> Result<()> {
    delete_desktop_shortcut()?;
    delete_start_menu_shortcut()?;
    Ok(())
}
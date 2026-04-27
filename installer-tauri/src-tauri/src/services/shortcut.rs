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
/// Uses UTF-16 LE with BOM encoding to support Chinese characters in paths
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

    // Write VBS file with UTF-16 LE BOM encoding (Windows Script Host requires this for Unicode)
    write_vbs_utf16(&vbs_path, &vbs_content)?;

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

/// Write VBS content with UTF-16 LE BOM encoding
/// Windows Script Host (cscript/wscript) requires UTF-16 LE with BOM for Unicode support
fn write_vbs_utf16(path: &PathBuf, content: &str) -> Result<()> {
    use std::io::Write;

    // UTF-16 LE BOM: 0xFF 0xFE
    let bom: [u8; 2] = [0xFF, 0xFE];

    // Convert to UTF-16 LE (Windows native Unicode encoding)
    let utf16_bytes: Vec<u8> = content
        .encode_utf16()
        .flat_map(|u| u.to_le_bytes())
        .collect();

    let mut file = std::fs::File::create(path).map_err(|e| InstallerError::Io {
        context: "create vbs script file".to_string(),
        source: e,
    })?;

    // Write BOM first
    file.write_all(&bom).map_err(|e| InstallerError::Io {
        context: "write vbs bom".to_string(),
        source: e,
    })?;

    // Write UTF-16 LE content
    file.write_all(&utf16_bytes).map_err(|e| InstallerError::Io {
        context: "write vbs utf16 content".to_string(),
        source: e,
    })?;

    Ok(())
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
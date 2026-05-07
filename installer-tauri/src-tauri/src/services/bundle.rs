use crate::error::{InstallerError, Result};
use std::path::{Path, PathBuf};

/// Get Mac data directory path (outside App Bundle)
/// CRITICAL-01 fix: data directory is NOT inside App Bundle to prevent upgrade data loss
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

/// Create macOS App Bundle structure (without data directory)
/// CRITICAL-01 fix: data directory is created separately in ~/Library/Application Support/
#[cfg(target_os = "macos")]
pub fn create_app_bundle(
    source_dir: &Path,
    app_bundle_path: &Path,
    version: &str,
) -> Result<()> {
    // Create directory structure (without data directory)
    let contents = app_bundle_path.join("Contents");
    let macos = contents.join("MacOS");
    let resources = contents.join("Resources");

    std::fs::create_dir_all(&macos).map_err(|e| InstallerError::Io {
        context: "create MacOS directory".to_string(),
        source: e,
    })?;
    std::fs::create_dir_all(&resources).map_err(|e| InstallerError::Io {
        context: "create Resources directory".to_string(),
        source: e,
    })?;

    // Copy executables (CRITICAL-02: remove .exe extension)
    copy_exe_without_extension(
        source_dir.join("colink-server.exe"),
        macos.join("colink-server"),
    )?;
    copy_exe_without_extension(
        source_dir.join("bin/migrate.exe"),
        macos.join("migrate"),
    )?;

    // Copy Launcher
    copy_exe_without_extension(
        source_dir.join("launcher/Colink.exe"),
        macos.join("Colink"),
    )?;

    // Copy Resources
    if source_dir.join("web").exists() {
        copy_dir_recursive(&source_dir.join("web"), &resources.join("web"))?;
    }
    if source_dir.join("sql-change").exists() {
        copy_dir_recursive(&source_dir.join("sql-change"), &resources.join("sql-change"))?;
    }
    if source_dir.join("config.yaml.example").exists() {
        std::fs::copy(
            source_dir.join("config.yaml.example"),
            resources.join("config.yaml.example"),
        ).map_err(|e| InstallerError::Io {
            context: "copy config.yaml.example".to_string(),
            source: e,
        })?;
    }

    // Generate Info.plist
    generate_info_plist(&contents, version)?;

    // Copy icon (if exists)
    if let Some(icon_source) = find_icns_icon(source_dir) {
        std::fs::copy(&icon_source, resources.join("AppIcon.icns")).map_err(|e| InstallerError::Io {
            context: "copy AppIcon.icns".to_string(),
            source: e,
        })?;
    }

    // CRITICAL-01: Do NOT create data directory inside App Bundle
    // Data directory is created by Launcher on first run in ~/Library/Application Support/Colink/

    Ok(())
}

#[cfg(not(target_os = "macos"))]
pub fn create_app_bundle(
    _source_dir: &Path,
    _app_bundle_path: &Path,
    _version: &str,
) -> Result<()> {
    Ok(())
}

/// Copy executable removing .exe extension (macOS doesn't use .exe)
/// CRITICAL-02 fix: handle both Windows build output (.exe) and Mac build output (no extension)
#[cfg(target_os = "macos")]
fn copy_exe_without_extension(src: PathBuf, dest: PathBuf) -> Result<()> {
    // Windows build: src has .exe suffix
    // Mac build: src may not have suffix
    let src_display = src.display().to_string();
    let actual_src = if src.exists() {
        src
    } else {
        // Remove .exe and try again
        PathBuf::from(src.to_string_lossy().replace(".exe", ""))
    };

    if actual_src.exists() {
        std::fs::copy(&actual_src, &dest).map_err(|e| InstallerError::Io {
            context: format!("copy executable: {} -> {}", actual_src.display(), dest.display()),
            source: e,
        })?;
        // Set executable permission (chmod +x)
        set_executable_permission(&dest)?;
    } else {
        log::warn!("Executable not found at {} or {}", src_display, actual_src.display());
    }
    Ok(())
}

#[cfg(not(target_os = "macos"))]
#[allow(dead_code)]
fn copy_exe_without_extension(_src: PathBuf, _dest: PathBuf) -> Result<()> {
    Ok(())
}

/// Set executable permission (chmod +x)
#[cfg(target_os = "macos")]
fn set_executable_permission(path: &Path) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    let mut perms = std::fs::metadata(path).map_err(|e| InstallerError::Io {
        context: "get metadata for executable permission".to_string(),
        source: e,
    })?.permissions();
    perms.set_mode(0o755); // rwxr-xr-x
    std::fs::set_permissions(path, perms).map_err(|e| InstallerError::Io {
        context: "set executable permission".to_string(),
        source: e,
    })?;
    Ok(())
}

#[cfg(not(target_os = "macos"))]
#[allow(dead_code)]
fn set_executable_permission(_path: &Path) -> Result<()> {
    Ok(())
}

/// Generate Info.plist for App Bundle
#[cfg(target_os = "macos")]
fn generate_info_plist(contents_dir: &Path, version: &str) -> Result<()> {
    let plist_content = format!(r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>Colink</string>
    <key>CFBundleDisplayName</key>
    <string>Colink</string>
    <key>CFBundleIdentifier</key>
    <string>com.colink.installer</string>
    <key>CFBundleVersion</key>
    <string>{}</string>
    <key>CFBundleShortVersionString</key>
    <string>{}</string>
    <key>CFBundleExecutable</key>
    <string>Colink</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon.icns</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
"#, version, version);

    std::fs::write(contents_dir.join("Info.plist"), plist_content).map_err(|e| InstallerError::Io {
        context: "write Info.plist".to_string(),
        source: e,
    })?;
    Ok(())
}

#[cfg(not(target_os = "macos"))]
#[allow(dead_code)]
fn generate_info_plist(_contents_dir: &Path, _version: &str) -> Result<()> {
    Ok(())
}

/// Find .icns icon file in source directory
#[allow(dead_code)]
fn find_icns_icon(source_dir: &Path) -> Option<PathBuf> {
    // Try common locations
    let candidates = [
        source_dir.join("AppIcon.icns"),
        source_dir.join("icons/AppIcon.icns"),
        source_dir.join("icon.icns"),
    ];
    candidates.iter().find(|p| p.exists()).cloned()
}

/// Copy directory recursively
#[allow(dead_code)]
fn copy_dir_recursive(src: &Path, dest: &Path) -> Result<()> {
    std::fs::create_dir_all(dest).map_err(|e| InstallerError::Io {
        context: format!("create directory: {}", dest.display()),
        source: e,
    })?;

    let entries = std::fs::read_dir(src).map_err(|e| InstallerError::Io {
        context: format!("read source directory: {}", src.display()),
        source: e,
    })?;

    for entry in entries {
        let entry = entry.map_err(|e| InstallerError::Io {
            context: "read directory entry".to_string(),
            source: e,
        })?;

        let src_path = entry.path();
        let dest_path = dest.join(entry.file_name());

        let file_type = entry.file_type().map_err(|e| InstallerError::Io {
            context: format!("get file type: {}", src_path.display()),
            source: e,
        })?;

        if file_type.is_dir() {
            copy_dir_recursive(&src_path, &dest_path)?;
        } else {
            std::fs::copy(&src_path, &dest_path).map_err(|e| InstallerError::Io {
                context: format!("copy file: {} -> {}", src_path.display(), dest_path.display()),
                source: e,
            })?;
        }
    }

    Ok(())
}
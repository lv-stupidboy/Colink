use crate::error::{InstallerError, Result};
use crate::services::{
    disk_space::get_disk_space,
    file_ops::{atomic_copy_dir, kill_all_processes, delete_except_whitelist, copy_dir_recursive, remove_dir_all_with_retry, cleanup_staging_dirs},
    registry::write_registry,
    shortcut::{create_desktop_shortcut, create_start_menu_shortcut, delete_all_shortcuts},
    uninstall::prepare_upgrade,
    config::generate_config_preview,
    config::save_config_file,
};
use serde::{Deserialize, Serialize};
use std::path::Path;
use std::process::Command;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

#[cfg(target_os = "windows")]
const CREATE_NO_WINDOW: u32 = 0x08000000;

/// Installation configuration from frontend
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InstallConfig {
    pub install_dir: String,
    pub install_mode: String,
    #[serde(default)]
    pub install_type: Option<String>, // "fresh", "upgrade", "reinstall"
    #[serde(default)]
    pub old_install_dir: Option<String>, // For reinstall mode
    #[serde(default)]
    pub dependencies: Vec<DependencyStatus>,
    pub database: DatabaseConfigInput,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub server_port: Option<u16>,
    #[serde(default = "default_create_shortcut")]
    pub create_shortcut: bool,
    #[serde(default)]
    pub launch_now: bool,
    #[serde(default)]
    pub keep_data: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub config_yaml: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub current_version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub new_version: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DependencyStatus {
    pub key: String,
    pub installed: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DatabaseConfigInput {
    pub r#type: String,
}

fn default_create_shortcut() -> bool {
    true
}

/// Installation progress event
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InstallProgress {
    pub step: String,
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub progress: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
}

/// Installation result
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InstallResult {
    pub success: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub db_changes: Option<Vec<DbChange>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DbChange {
    pub version: String,
    pub files: Vec<String>,
}

/// Database changes info
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DatabaseChangesResult {
    pub versions: Vec<String>,
    pub changes: Vec<DbChange>,
}

/// Check database changes between versions
pub fn check_database_changes(
    install_dir: &str,
    current_version: Option<&str>,
    new_version: Option<&str>,
) -> Result<DatabaseChangesResult> {
    let sql_change_dir = Path::new(install_dir).join("sql-change");

    if !sql_change_dir.exists() {
        return Ok(DatabaseChangesResult {
            versions: vec![],
            changes: vec![],
        });
    }

    // Read version directories
    let mut versions: Vec<String> = vec![];
    let mut changes: Vec<DbChange> = vec![];

    let entries = std::fs::read_dir(&sql_change_dir)
        .map_err(|e| InstallerError::Io {
            context: "read sql-change dir".to_string(),
            source: e,
        })?;

    // Check if this is a fresh install (no existing database)
    let db_path = Path::new(install_dir)
        .join("data")
        .join("sqlite")
        .join("colink.db");
    let is_fresh_install = !db_path.exists();

    for entry in entries {
        let entry = entry.map_err(|e| InstallerError::Io {
            context: "read entry".to_string(),
            source: e,
        })?;

        let name = entry.file_name().to_string_lossy().to_string();
        if name.starts_with("v") {
            // Determine if this version should be migrated
            let should_migrate = if is_fresh_install {
                // Fresh install: run all migrations
                log::info!("Fresh install detected, will migrate all versions");
                true
            } else {
                // Upgrade: only migrate versions between current and new
                match (current_version, new_version) {
                    (Some(current), Some(new)) => {
                        // Compare versions (simple string comparison)
                        name > format!("v{}", current) && name <= format!("v{}", new)
                    }
                    (None, Some(new)) => name <= format!("v{}", new),
                    _ => true,
                }
            };

            if should_migrate {
                versions.push(name.clone());

                // Check sqlite migration files
                let sqlite_dir = entry.path().join("sqlite");
                if sqlite_dir.exists() {
                    let files: Vec<String> = std::fs::read_dir(&sqlite_dir)
                        .map(|d| {
                            d.filter_map(|f| f.ok())
                                .map(|f| f.file_name().to_string_lossy().to_string())
                                .collect()
                        })
                        .unwrap_or_default();

                    if !files.is_empty() {
                        changes.push(DbChange { version: name, files });
                    }
                }
            }
        }
    }

    Ok(DatabaseChangesResult { versions, changes })
}

/// Run database migration for a single version
pub fn run_single_migration(
    install_dir: &str,
    db_path: &str,
    version: &str,
) -> Result<String> {
    let migrate_exe = Path::new(install_dir)
        .join("bin")
        .join("migrate.exe");

    if !migrate_exe.exists() {
        log::warn!("migrate.exe not found at {:?}, skipping migration", migrate_exe);
        return Ok("跳过迁移：migrate.exe 未找到".to_string());
    }

    // Point to the specific version's sqlite directory
    let migration_dir = Path::new(install_dir)
        .join("sql-change")
        .join(format!("v{}", version))
        .join("sqlite");

    if !migration_dir.exists() {
        log::warn!("Migration directory not found: {:?}", migration_dir);
        return Ok(format!("跳过版本 {}: 迁移目录不存在", version));
    }

    log::info!("Running migrate.exe for version {} with db={}, dir={}",
        version, db_path, migration_dir.to_string_lossy());

    #[cfg(target_os = "windows")]
    let output = Command::new(&migrate_exe)
        .args([
            "up",
            "--db",
            db_path,
            "--dir",
            &migration_dir.to_string_lossy(),
        ])
        .creation_flags(CREATE_NO_WINDOW)
        .output();

    #[cfg(not(target_os = "windows"))]
    let output = Command::new(&migrate_exe)
        .args([
            "up",
            "--db",
            db_path,
            "--dir",
            &migration_dir.to_string_lossy(),
        ])
        .output();

    match output {
        Ok(o) => {
            let stdout = String::from_utf8_lossy(&o.stdout).to_string();
            let stderr = String::from_utf8_lossy(&o.stderr).to_string();

            log::info!("Migration v{} stdout: {}", version, stdout);
            if !stderr.is_empty() {
                log::info!("Migration v{} stderr: {}", version, stderr);
            }

            if !o.status.success() {
                log::warn!("Migration v{} failed with exit code: {:?}", version, o.status.code());
                return Err(InstallerError::Process(
                    format!("版本 {} 迁移失败: {}", version, stderr)
                ));
            }
            Ok(format!("版本 {} 迁移成功: {}", version, stdout.trim()))
        }
        Err(e) => {
            log::warn!("Failed to run migrate.exe for version {}: {}", version, e);
            Err(InstallerError::Process(e.to_string()))
        }
    }
}

/// Run database migration for all detected versions
pub fn run_database_migration(
    install_dir: &str,
    db_path: &str,
    versions: &[String],
) -> Result<String> {
    if versions.is_empty() {
        return Ok("无需迁移".to_string());
    }

    let migrate_exe = Path::new(install_dir)
        .join("bin")
        .join("migrate.exe");

    if !migrate_exe.exists() {
        log::warn!("migrate.exe not found at {:?}, skipping migration", migrate_exe);
        return Ok("跳过迁移：migrate.exe 未找到".to_string());
    }

    let mut results: Vec<String> = vec![];

    // Execute migration for each version in order
    for version in versions {
        // Extract version number (remove 'v' prefix)
        let version_num = version.strip_prefix("v").unwrap_or(version);
        let result = run_single_migration(install_dir, db_path, version_num)?;
        results.push(result);
    }

    Ok(results.join("\n"))
}

/// Full installation pipeline
pub async fn run_installation<F>(
    config: &InstallConfig,
    resource_path: &Path,
    emit_progress: F,
) -> Result<InstallResult>
where
    F: Fn(&InstallProgress),
{
    let install_type = config.install_type.as_deref().unwrap_or("fresh");
    log::info!("Installation type: {}", install_type);

    // Handle reinstall mode separately
    if install_type == "reinstall" {
        return run_reinstall(config, resource_path, emit_progress).await;
    }

    // Standard installation (fresh or upgrade)
    let install_dir = Path::new(&config.install_dir);

    // Step 0: Check disk space
    emit_progress(&InstallProgress {
        step: "prepare".to_string(),
        status: "running".to_string(),
        progress: Some(0),
        message: Some("检查磁盘空间...".into()),
        details: None,
    });

    let disk_space = get_disk_space(&config.install_dir)?;
    let required_space = 500 * 1024 * 1024; // 500MB minimum
    if disk_space.free < required_space {
        return Err(InstallerError::DiskSpaceInsufficient {
            required: required_space,
            available: disk_space.free,
        });
    }

    // Step 1: Prepare for upgrade if existing installation
    if install_dir.exists() {
        emit_progress(&InstallProgress {
            step: "prepare".to_string(),
            status: "running".to_string(),
            progress: Some(10),
            message: Some("准备升级...".into()),
            details: None,
        });

        // Kill running processes
        kill_all_processes("colink-server.exe")?;
        kill_all_processes("Colink.exe")?;

        // Prepare upgrade (move old files to backup)
        prepare_upgrade(&config.install_dir)?;
    }

    emit_progress(&InstallProgress {
        step: "prepare".to_string(),
        status: "success".to_string(),
        progress: Some(20),
        message: Some("准备完成".into()),
        details: None,
    });

    // Copy files and complete installation
    copy_files_and_complete(config, resource_path, install_dir, emit_progress).await
}

/// Reinstall pipeline: uninstall old, then install fresh
async fn run_reinstall<F>(
    config: &InstallConfig,
    resource_path: &Path,
    emit_progress: F,
) -> Result<InstallResult>
where
    F: Fn(&InstallProgress),
{
    let install_dir = Path::new(&config.install_dir);
    let old_install_dir = config.old_install_dir.as_ref()
        .map(|d| Path::new(d))
        .unwrap_or(install_dir);

    let dir_changed = old_install_dir != install_dir;
    log::info!("Reinstall mode: old={}, new={}, changed={}, keepData={}",
        old_install_dir.display(), install_dir.display(), dir_changed, config.keep_data);

    // Step 0: Prepare - check disk space, stop processes
    emit_progress(&InstallProgress {
        step: "prepare".to_string(),
        status: "running".to_string(),
        progress: Some(0),
        message: Some("检查磁盘空间，停止运行中的进程...".into()),
        details: None,
    });

    let disk_space = get_disk_space(&config.install_dir)?;
    let required_space = 500 * 1024 * 1024; // 500MB minimum
    if disk_space.free < required_space {
        return Err(InstallerError::DiskSpaceInsufficient {
            required: required_space,
            available: disk_space.free,
        });
    }

    // Kill running processes
    kill_all_processes("colink-server.exe")?;
    kill_all_processes("Colink.exe")?;

    emit_progress(&InstallProgress {
        step: "prepare".to_string(),
        status: "success".to_string(),
        progress: Some(10),
        message: Some("准备完成".into()),
        details: None,
    });

    // Step 1: Uninstall old version
    emit_progress(&InstallProgress {
        step: "uninstall".to_string(),
        status: "running".to_string(),
        progress: Some(10),
        message: Some("卸载旧版本...".into()),
        details: None,
    });

    // Delete shortcuts
    delete_all_shortcuts()?;

    // Delete old installation directory contents
    if old_install_dir.exists() {
        // Clean backup directory first (use retry for Windows temporary locks)
        let backup_dir = old_install_dir.join("backup");
        if backup_dir.exists() {
            remove_dir_all_with_retry(&backup_dir, 3, 500)?;
        }

        // Clean up leftover .staging-* directories from previous failed installs
        cleanup_staging_dirs(old_install_dir)?;

        if config.keep_data {
            // Keep only data directory
            let whitelist = ["data"];
            delete_except_whitelist(old_install_dir, &whitelist)?;
        } else {
            // Full delete (use retry for Windows temporary locks)
            remove_dir_all_with_retry(old_install_dir, 3, 500)?;
        }
    }

    emit_progress(&InstallProgress {
        step: "uninstall".to_string(),
        status: "success".to_string(),
        progress: Some(20),
        message: Some("旧版本已卸载".into()),
        details: None,
    });

    // Step 2: Migrate data (if keepData and directory changed)
    if config.keep_data && dir_changed {
        emit_progress(&InstallProgress {
            step: "migratedata".to_string(),
            status: "running".to_string(),
            progress: Some(20),
            message: Some("迁移用户数据...".into()),
            details: Some(format!("从 {} 到 {}", old_install_dir.display(), install_dir.display())),
        });

        let old_data_dir = old_install_dir.join("data");
        let new_data_dir = install_dir.join("data");

        if old_data_dir.exists() {
            // Create new install directory
            std::fs::create_dir_all(install_dir)
                .map_err(|e| InstallerError::Io {
                    context: "create new install dir".to_string(),
                    source: e,
                })?;

            // Copy data directory
            copy_dir_recursive(&old_data_dir, &new_data_dir)?;

            log::info!("Data migrated from {} to {}", old_data_dir.display(), new_data_dir.display());
        } else {
            log::info!("Old data directory does not exist, skipping migration");
        }

        emit_progress(&InstallProgress {
            step: "migratedata".to_string(),
            status: "success".to_string(),
            progress: Some(30),
            message: Some("数据迁移完成".into()),
            details: None,
        });
    } else if !config.keep_data {
        emit_progress(&InstallProgress {
            step: "migratedata".to_string(),
            status: "success".to_string(),
            progress: Some(30),
            message: Some("未保留数据".into()),
            details: None,
        });
    } else {
        // Same directory, no need to migrate
        emit_progress(&InstallProgress {
            step: "migratedata".to_string(),
            status: "success".to_string(),
            progress: Some(30),
            message: Some("目录未改变，无需迁移数据".into()),
            details: None,
        });
    }

    // Delete registry entry (will be recreated)
    crate::services::registry::delete_registry()?;

    // Copy files and complete installation
    copy_files_and_complete(config, resource_path, install_dir, emit_progress).await
}

/// Copy files and complete installation (shared between fresh/upgrade/reinstall)
async fn copy_files_and_complete<F>(
    config: &InstallConfig,
    resource_path: &Path,
    install_dir: &Path,
    emit_progress: F,
) -> Result<InstallResult>
where
    F: Fn(&InstallProgress),
{
    // Step: Copy application files
    emit_progress(&InstallProgress {
        step: "copy".to_string(),
        status: "running".to_string(),
        progress: Some(30),
        message: Some("复制应用文件...".into()),
        details: None,
    });

    // Create install directory
    std::fs::create_dir_all(install_dir)
        .map_err(|e| InstallerError::Io {
            context: "create install dir".to_string(),
            source: e,
        })?;

    // Find resources directory - try multiple locations:
    // 1. resource_path/resources (dev mode)
    // 2. resource_path (release mode with bundled resources)
    // 3. exe_dir/../resources (ZIP packaged mode: exe in exe/, resources in sibling resources/)
    let exe_path = std::env::current_exe().ok();
    let exe_dir = exe_path.as_ref().and_then(|p| p.parent());

    let resources_candidates = vec![
        resource_path.join("resources"),
        resource_path.to_path_buf(),
        exe_dir.map(|d| d.join("..").join("resources")).unwrap_or_default(),
    ];

    let resources_base = resources_candidates
        .iter()
        .find(|p| p.exists() && p.join("colink-server.exe").exists())
        .cloned()
        .unwrap_or_else(|| resource_path.to_path_buf());

    log::info!("Using resources from: {:?}", resources_base);

    // Copy server exe
    let server_src = resources_base.join("colink-server.exe");
    let server_dest = install_dir.join("colink-server.exe");
    if server_src.exists() {
        std::fs::copy(&server_src, &server_dest)
            .map_err(|e| InstallerError::Io {
                context: format!("复制服务程序: {} -> {}", server_src.to_string_lossy(), server_dest.to_string_lossy()),
                source: e,
            })?;
        log::info!("Copied server exe to {:?}", server_dest);
    } else {
        log::warn!("Server exe not found at {:?}", server_src);
    }

    // Copy web directory
    let web_src = resources_base.join("web");
    let web_dest = install_dir.join("web");
    if web_src.exists() {
        atomic_copy_dir(&web_src, &web_dest)?;
        log::info!("Copied web directory to {:?}", web_dest);
    } else {
        log::warn!("Web directory not found at {:?}", web_src);
    }

    // Copy sql-change directory
    let sql_src = resources_base.join("sql-change");
    let sql_dest = install_dir.join("sql-change");
    if sql_src.exists() {
        atomic_copy_dir(&sql_src, &sql_dest)?;
        log::info!("Copied sql-change directory to {:?}", sql_dest);
    } else {
        log::warn!("sql-change directory not found at {:?}", sql_src);
    }

    // Copy bin directory (migrate.exe etc)
    let bin_src = resources_base.join("bin");
    let bin_dest = install_dir.join("bin");
    if bin_src.exists() {
        atomic_copy_dir(&bin_src, &bin_dest)?;
        log::info!("Copied bin directory to {:?}", bin_dest);
    } else {
        log::warn!("bin directory not found at {:?}", bin_src);
    }

    // Create data directories (if not already exist from migration)
    let data_dir = install_dir.join("data");
    std::fs::create_dir_all(&data_dir)
        .map_err(|e| InstallerError::Io {
            context: "create data dir".to_string(),
            source: e,
        })?;

    for subdir in ["configs", "logs", "agent-assets", "agent-configs", "repos", "sqlite"] {
        std::fs::create_dir_all(data_dir.join(subdir))
            .map_err(|e| InstallerError::Io {
                context: format!("create data subdir: {}", subdir),
                source: e,
            })?;
    }

    emit_progress(&InstallProgress {
        step: "copy".to_string(),
        status: "success".to_string(),
        progress: Some(50),
        message: Some("文件复制完成".into()),
        details: None,
    });

    // Step: Copy launcher files
    emit_progress(&InstallProgress {
        step: "launcher".to_string(),
        status: "running".to_string(),
        progress: Some(50),
        message: Some("复制 Launcher 文件...".into()),
        details: None,
    });

    let launcher_exe_dest = install_dir.join("Colink.exe");
    let mut launcher_copied = false;

    // Try multiple locations to find Colink.exe:
    // 1. exe same directory (dev mode: target/debug or target/release)
    // 2. resources/launcher directory (packaged NSIS install)
    let launcher_candidates = vec![
        // Dev mode: Colink.exe in same directory as installer exe
        exe_dir.map(|d| d.join("Colink.exe")),
        // Packaged mode: resources/launcher/Colink.exe
        Some(resources_base.join("launcher").join("Colink.exe")),
    ];

    for candidate in launcher_candidates.iter().flatten() {
        if candidate.exists() {
            log::info!("Found launcher at {:?}", candidate);
            std::fs::copy(candidate, &launcher_exe_dest)
                .map_err(|e| InstallerError::Io {
                    context: "copy launcher exe".to_string(),
                    source: e,
                })?;
            log::info!("Copied launcher exe to {:?}", launcher_exe_dest);
            launcher_copied = true;
            break;
        } else {
            log::debug!("Launcher not found at {:?}", candidate);
        }
    }

    if !launcher_copied {
        log::warn!("Launcher exe not found in any location, skipping launcher copy");
    }

    emit_progress(&InstallProgress {
        step: "launcher".to_string(),
        status: "success".to_string(),
        progress: Some(60),
        message: Some("Launcher 文件已复制".into()),
        details: None,
    });

    // Step: Check database changes
    emit_progress(&InstallProgress {
        step: "dbcheck".to_string(),
        status: "running".to_string(),
        progress: Some(60),
        message: Some("检查数据库变更...".into()),
        details: None,
    });

    let db_changes = check_database_changes(
        &config.install_dir,
        config.current_version.as_deref(),
        config.new_version.as_deref(),
    )?;

    emit_progress(&InstallProgress {
        step: "dbcheck".to_string(),
        status: "success".to_string(),
        progress: Some(65),
        message: Some("数据库检查完成".into()),
        details: Some(format!("发现 {} 个版本需要迁移", db_changes.versions.len())),
    });

    // Step: Database migration
    if !db_changes.versions.is_empty() {
        emit_progress(&InstallProgress {
            step: "migration".to_string(),
            status: "running".to_string(),
            progress: Some(65),
            message: Some("执行数据库迁移...".into()),
            details: Some(format!("待迁移版本: {}", db_changes.versions.join(", "))),
        });

        let db_path = install_dir
            .join("data")
            .join("sqlite")
            .join("colink.db");

        log::info!("Running database migration for versions: {:?}", db_changes.versions);
        // Sort versions to ensure correct order
        let mut sorted_versions = db_changes.versions.clone();
        sorted_versions.sort();
        let migration_log = run_database_migration(&config.install_dir, &db_path.to_string_lossy(), &sorted_versions)?;

        emit_progress(&InstallProgress {
            step: "migration".to_string(),
            status: "success".to_string(),
            progress: Some(75),
            message: Some("数据库迁移完成".into()),
            details: Some(migration_log),
        });
    } else {
        // 首次安装或无需迁移
        emit_progress(&InstallProgress {
            step: "migration".to_string(),
            status: "success".to_string(),
            progress: Some(70),
            message: Some("无需数据库迁移".into()),
            details: Some("首次安装或未发现数据库变更".into()),
        });
    }

    // Step: Configuration file
    emit_progress(&InstallProgress {
        step: "config".to_string(),
        status: "running".to_string(),
        progress: Some(75),
        message: Some("写入配置文件...".into()),
        details: None,
    });

    // Write config
    if let Some(yaml_content) = &config.config_yaml {
        save_config_file(&config.install_dir, yaml_content)?;
    } else {
        // Generate default config using resources_base (already found earlier)
        let template_path = resources_base.join("config.yaml.example");
        log::info!("Looking for config template at: {:?}", template_path);
        let server_port = config.server_port.unwrap_or(26305);
        let yaml_content = generate_config_preview(&template_path, server_port, 0, &config.database.r#type)?;
        save_config_file(&config.install_dir, &yaml_content)?;
    }

    emit_progress(&InstallProgress {
        step: "config".to_string(),
        status: "success".to_string(),
        progress: Some(85),
        message: Some("配置已写入".into()),
        details: None,
    });

    // Step: Create shortcuts
    if config.create_shortcut {
        emit_progress(&InstallProgress {
            step: "shortcut".to_string(),
            status: "running".to_string(),
            progress: Some(85),
            message: Some("创建快捷方式...".into()),
            details: None,
        });

        create_desktop_shortcut(&config.install_dir)?;
        create_start_menu_shortcut(&config.install_dir)?;

        emit_progress(&InstallProgress {
            step: "shortcut".to_string(),
            status: "success".to_string(),
            progress: Some(95),
            message: Some("快捷方式已创建".into()),
            details: None,
        });
    }

    // Step: Write registry
    emit_progress(&InstallProgress {
        step: "registry".to_string(),
        status: "running".to_string(),
        progress: Some(95),
        message: Some("写入注册表...".into()),
        details: None,
    });

    let version = config.new_version.clone().unwrap_or_else(|| "1.0.0".to_string());
    write_registry(&config.install_dir, &version)?;

    emit_progress(&InstallProgress {
        step: "registry".to_string(),
        status: "success".to_string(),
        progress: Some(98),
        message: Some("注册表已写入".into()),
        details: None,
    });

    // Step: Complete
    emit_progress(&InstallProgress {
        step: "complete".to_string(),
        status: "success".to_string(),
        progress: Some(100),
        message: Some("安装完成".into()),
        details: None,
    });

    Ok(InstallResult {
        success: true,
        error: None,
        db_changes: Some(db_changes.changes),
    })
}
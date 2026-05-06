---
title: installer-tauri Mac 支持设计
version: 1.1.0
date: 2026-05-06
status: review_fixes_applied
review_issues_fixed: CRITICAL-01, CRITICAL-02
---

# installer-tauri Mac 支持设计

## 概述

将现有的 Windows 安装程序（installer-tauri）扩展支持 macOS，采用标准 Mac App Bundle 架构。

**设计决策**：
- **分发方式**：DMG（标准 macOS 安装方式）
- **安装位置**：固定 `/Applications/Colink.app`
- **数据存储**：`~/Library/Application Support/Colink/`（⚠️ CRITICAL 修复：移出 App Bundle 防止升级数据丢失）
- **服务管理**：Launcher GUI 手动控制（用户级）

> **CRITICAL-01 修复说明**：Mac 用户升级 App 的标准方式是「拖拽覆盖 App Bundle」，这会覆盖整个 `/Applications/Colink.app/` 目录。如果 data 目录放在 App Bundle 内，每次升级都会丢失用户数据（数据库、配置、repos）。因此数据目录必须移到 App Bundle 外的 `~/Library/Application Support/Colink/`。

## 现有架构分析

### Windows 实现（需要改造）

| 模块 | Windows | Mac 改造 |
|------|---------|----------|
| 安装信息存储 | 注册表 HKLM/HKCU | plist 文件 |
| 快捷方式 | VBScript + .lnk | App Bundle 结构 |
| 进程检测 | `tasklist` | `pgrep`/`ps` |
| 进程终止 | `taskkill` | `pkill`/`kill` |
| 端口检测 | `netstat -ano` | `lsof -i :port` |
| 文件扩展名 | `.exe` | 无扩展名 |
| 安装目录 | 用户自定义 | `/Applications/Colink.app` |

### 双应用模式（保持不变）

```
Colink-Setup.app  → Setup 模式（安装/升级/卸载）
Colink.app        → Launcher 模式（服务控制面板）
```

模式检测逻辑不变（通过 exe/app 名称判断）：
```rust
// store.rs
pub fn detect_app_mode() -> AppMode {
    let exe_name = std::env::current_exe().ok()
        .and_then(|p| p.file_name().map(|n| n.to_string_lossy().to_string()))
        .unwrap_or_default();

    if exe_name.contains("Launcher") || exe_name == "Colink" {
        AppMode::Launcher
    } else {
        AppMode::Setup
    }
}
```

## 设计方案

### 1. App Bundle 结构（⚠️ CRITICAL-01 修复）

**App Bundle 结构（不含用户数据）**：
```
/Applications/Colink.app/
├── Contents/
│   ├── MacOS/
│   │   ├── Colink              # 主可执行文件（Launcher）
│   │   ├── colink-server       # 后端服务（无 .exe 后缀）
│   │   └── migrate             # 数据库迁移工具
│   ├── Resources/
│   │   ├── web/                # 前端静态文件
│   │   ├── sql-change/         # 数据库迁移脚本
│   │   ├── bin/                # 辅助工具
│   │   ├── config.yaml.example
│   │   ├── AppIcon.icns        # Mac 图标
│   │   ├── VERSION
│   │   └── installer-config.json
│   └── Info.plist              # App 元数据
```

**用户数据目录（⚠️ 升级时保留）**：
```
~/Library/Application Support/Colink/
├── configs/
│   └── config.yaml             # 用户配置（首次运行从 .example 复制）
├── logs/
│   └── colink.log
├── sqlite/
│   └── colink.db               # 数据库（⚠️ 升级保留）
├── agent-assets/
│   └── ...                     # Agent 资产（⚠️ 升级保留）
├── agent-configs/
│   └── ...                     # Agent 配置
└── repos/
    └── ...                     # 代码仓库（⚠️ 升级保留）
```

**设计原则**：
1. App Bundle 只包含程序文件（升级可覆盖）
2. 用户数据放在 `~/Library/Application Support/`（升级不受影响）
3. 首次运行时自动创建数据目录并初始化配置

> **对比 Windows**：Windows 使用 `%APPDATA%/Colink/`，Mac 对应位置为 `~/Library/Application Support/Colink/`。两者都是用户级目录，升级时保留。

**Info.plist 内容**：
```xml
<?xml version="1.0" encoding="UTF-8"?>
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
    <string>1.0.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
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
```

### 2. 核心模块改造

#### 2.1 新增 `bundle.rs`（App Bundle 操作）

文件：`src-tauri/src/services/bundle.rs`

> **⚠️ CRITICAL-01 修复**：data 目录不在 App Bundle 内，此处只创建程序目录。数据目录在首次运行时由 Launcher 创建。

```rust
use crate::error::{InstallerError, Result};
use std::path::Path;

/// Get Mac data directory path (outside App Bundle)
pub fn get_mac_data_dir() -> PathBuf {
    dirs::data_dir()
        .unwrap_or_else(|| PathBuf::from("~/.local/share"))
        .join("Colink")
}

/// Create macOS App Bundle structure (without data directory)
pub fn create_app_bundle(
    source_dir: &Path,
    app_bundle_path: &Path,
    version: &str,
) -> Result<()> {
    // 创建目录结构（不含 data 目录）
    let contents = app_bundle_path.join("Contents");
    let macos = contents.join("MacOS");
    let resources = contents.join("Resources");

    std::fs::create_dir_all(&macos)?;
    std::fs::create_dir_all(&resources)?;

    // 复制可执行文件（⚠️ CRITICAL-02：去除 .exe 后缀）
    copy_exe_without_extension(
        source_dir.join("colink-server.exe"),  // Windows 构建产物
        macos.join("colink-server"),
    )?;
    copy_exe_without_extension(
        source_dir.join("bin/migrate.exe"),
        macos.join("migrate"),
    )?;

    // 复制 Launcher
    copy_exe_without_extension(
        source_dir.join("launcher/Colink.exe"),
        macos.join("Colink"),
    )?;

    // 复制 Resources
    copy_dir_recursive(source_dir.join("web"), resources.join("web"))?;
    copy_dir_recursive(source_dir.join("sql-change"), resources.join("sql-change"))?;
    std::fs::copy(
        source_dir.join("config.yaml.example"),
        resources.join("config.yaml.example"),
    )?;

    // 生成 Info.plist
    generate_info_plist(&contents, version)?;

    // 复制图标
    if let Some(icon_source) = find_icns_icon(source_dir) {
        std::fs::copy(icon_source, resources.join("AppIcon.icns"))?;
    }

    // ⚠️ 不创建 data 目录在 App Bundle内
    // data 目录在 Launcher 首次运行时创建到 ~/Library/Application Support/Colink/

    Ok(())
}

/// Copy executable removing .exe extension (macOS doesn't use .exe)
fn copy_exe_without_extension(src: PathBuf, dest: PathBuf) -> Result<()> {
    // Windows 构建：src 有 .exe 后缀
    // Mac 构建：src 可能无后缀
    let actual_src = if src.exists() {
        src
    } else {
        // 去掉 .exe 尝试
        PathBuf::from(src.to_string_lossy().replace(".exe", ""))
    };

    if actual_src.exists() {
        std::fs::copy(&actual_src, &dest)?;
        // 设置可执行权限
        set_executable_permission(&dest)?;
    }
    Ok(())
}

/// Set executable permission (chmod +x)
fn set_executable_permission(path: &Path) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    let mut perms = std::fs::metadata(path)?.permissions();
    perms.set_mode(0o755); // rwxr-xr-x
    std::fs::set_permissions(path, perms)?;
    Ok(())
}

/// Generate Info.plist
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

    std::fs::write(contents_dir.join("Info.plist"), plist_content)?;
    Ok(())
}
```

#### 2.2 改造 `plist.rs`（替代 registry）

文件：`src-tauri/src/services/plist.rs`

> **⚠️ CRITICAL-01 修复**：has_data 检测改为检测 `~/Library/Application Support/Colink/` 目录。

```rust
use crate::error::{InstallerError, Result};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

/// Information about installed version
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InstalledVersion {
    pub installed: bool,
    pub install_dir: Option<String>,  // App Bundle path
    pub data_dir: Option<String>,     // ⚠️ 新增：用户数据目录路径
    pub version: Option<String>,
    pub has_data: Option<bool>,
}

/// Get Mac data directory (⚠️ CRITICAL-01 修复：不在 App Bundle 内)
#[cfg(target_os = "macos")]
pub fn get_mac_data_dir() -> PathBuf {
    dirs::data_dir()
        .unwrap_or_else(|| PathBuf::from("~/.local/share"))
        .join("Colink")
}

#[cfg(target_os = "macos")]
pub fn get_installed_version() -> Result<InstalledVersion> {
    let app_path = PathBuf::from("/Applications/Colink.app");

    if !app_path.exists() {
        return Ok(InstalledVersion::default());
    }

    // 读取 Info.plist 获取版本
    let plist_path = app_path.join("Contents/Info.plist");
    let version = parse_plist_version(&plist_path)?;

    // ⚠️ CRITICAL-01 修复：检查 data 目录在 ~/Library/Application Support/
    let data_dir = get_mac_data_dir();
    let has_data = data_dir.join("sqlite/colink.db").exists();

    Ok(InstalledVersion {
        installed: true,
        install_dir: Some(app_path.to_string_lossy().to_string()),
        data_dir: Some(data_dir.to_string_lossy().to_string()),
        version,
        has_data: Some(has_data),
    })
}

#[cfg(target_os = "macos")]
fn parse_plist_version(plist_path: &PathBuf) -> Result<Option<String>> {
    // ⚠️ 使用 plist crate 解析（审查建议）
    let plist::Value = plist::from_file(plist_path)
        .map_err(|e| InstallerError::Io {
            context: "parse Info.plist".to_string(),
            source: e,
        })?;

    if let Some(version) = plist.get("CFBundleShortVersionString").and_then(|v| v.as_string()) {
        return Ok(Some(version.to_string()));
    }
    Ok(None)
}

#[cfg(not(target_os = "macos"))]
pub fn get_installed_version() -> Result<InstalledVersion> {
    // Windows 使用 registry.rs
    Ok(InstalledVersion::default())
}

/// Write installation plist (for user preferences)
#[cfg(target_os = "macos")]
pub fn write_install_plist(install_dir: &str, data_dir: &str, version: &str) -> Result<()> {
    let prefs_dir = dirs::home_dir()
        .map(|h| h.join("Library/Preferences"))
        .unwrap_or_default();

    let plist_path = prefs_dir.join("com.colink.installer.plist");

    // 使用 plist crate 写入（更规范）
    let plist_dict = plist::Dictionary::new();
    plist_dict.insert("InstallDir".to_string(), plist::Value::String(install_dir.to_string()));
    plist_dict.insert("DataDir".to_string(), plist::Value::String(data_dir.to_string()));  // ⚠️ 新增
    plist_dict.insert("Version".to_string(), plist::Value::String(version.to_string()));
    plist_dict.insert("InstallDate".to_string(), plist::Value::String(
        chrono::Local::now().format("%Y-%m-%d").to_string()
    ));

    plist::to_file(&plist_path, &plist::Value::Dictionary(plist_dict))?;
    Ok(())
}

#[cfg(target_os = "macos")]
pub fn delete_install_plist() -> Result<()> {
    let prefs_dir = dirs::home_dir()
        .map(|h| h.join("Library/Preferences"))
        .unwrap_or_default();

    let plist_path = prefs_dir.join("com.colink.installer.plist");
    if plist_path.exists() {
        std::fs::remove_file(plist_path)?;
    }
    Ok(())
}
```

#### 2.3 改造 `service_manager.rs`（⚠️ CRITICAL-02 修复）

文件：`src-tauri/src/services/service_manager.rs`

> **⚠️ CRITICAL-02 修复**：现有代码硬编码 `.exe` 后缀，需要添加 `#[cfg(target_os)]` 条件编译。

**现有代码修改点**：

```rust
// ❌ 现有代码（service_manager.rs:148）— 硬编码 .exe
// 需要修改：
let server_exe = format!("{}/colink-server.exe", self.install_dir);

// ✅ 修复方案：
#[cfg(target_os = "windows")]
let server_exe = format!("{}/colink-server.exe", self.install_dir);

#[cfg(target_os = "macos")]
let server_exe = format!("{}/Contents/MacOS/colink-server", self.install_dir);
```

**完整 Mac 实现**：

```rust
#[cfg(target_os = "macos")]
fn check_port_in_use(port: u16) -> Result<Option<u32>> {
    let output = Command::new("lsof")
        .args(["-i", &format!(":{}", port), "-t"])
        .output()
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    let stdout = String::from_utf8_lossy(&output.stdout);
    for line in stdout.lines() {
        if let Ok(pid) = line.trim().parse::<u32>() {
            log::info!("Port {} is in use by PID {}", port, pid);
            return Ok(Some(pid));
        }
    }
    Ok(None)
}

#[cfg(target_os = "macos")]
fn kill_process_by_pid(pid: u32) -> Result<()> {
    let output = Command::new("kill")
        .args(["-9", &pid.to_string()])
        .output()
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    if output.status.success() {
        log::info!("Successfully killed process PID {}", pid);
        Ok(())
    } else {
        Err(InstallerError::Process(format!(
            "无法终止进程 (PID {})",
            pid
        )))
    }
}

#[cfg(target_os = "macos")]
pub fn start(&self) -> Result<()> {
    // ⚠️ CRITICAL-02 修复：Mac 可执行文件路径
    let server_path = PathBuf::from(&self.install_dir)
        .join("Contents/MacOS/colink-server");  // ⚠️ 无 .exe

    // ⚠️ CRITICAL-01 修复：配置文件在 ~/Library/Application Support/
    let data_dir = get_mac_data_dir();
    let config_path = data_dir.join("configs/config.yaml");

    // 检查 server 存在
    if !server_path.exists() {
        return Err(InstallerError::Process(format!(
            "服务程序不存在: {}",
            server_path.display()
        )));
    }

    // 检查配置文件存在（首次运行可能不存在）
    if !config_path.exists() {
        // 从 Resources 复制 config.yaml.example
        let example_path = PathBuf::from(&self.install_dir)
            .join("Contents/Resources/config.yaml.example");
        if example_path.exists() {
            std::fs::create_dir_all(config_path.parent().unwrap())?;
            std::fs::copy(&example_path, &config_path)?;
        }
    }

    // 启动进程
    let child = Command::new(&server_path)
        .args(["-config", &config_path.to_string_lossy()])
        .current_dir(&data_dir)  // ⚠️ 工作目录改为数据目录
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .spawn();

    // ... 后续逻辑同 Windows
}
```

#### 2.4 改造 `file_ops.rs`

```rust
#[cfg(target_os = "macos")]
pub fn is_process_running(process_name: &str) -> Result<bool> {
    let output = Command::new("pgrep")
        .args(["-x", process_name])
        .output()
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    Ok(output.status.success())
}

#[cfg(target_os = "macos")]
pub fn kill_all_processes(process_name: &str) -> Result<()> {
    let output = Command::new("pkill")
        .args(["-x", process_name])
        .output()
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    // pkill 返回非零如果没有匹配进程（可忽略）
    log::info!("pkill {} completed", process_name);
    Ok(())
}

#[cfg(target_os = "macos")]
pub fn remove_dir_all_with_retry(path: &Path, _max_retries: u32, _retry_delay_ms: u64) -> Result<()> {
    // Mac 无 Windows 文件锁问题，直接删除
    if path.exists() {
        std::fs::remove_dir_all(path)
            .map_err(|e| InstallerError::Io {
                context: format!("remove directory: {}", path.display()),
                source: e,
            })?;
    }
    Ok(())
}
```

#### 2.5 改造 `shortcut.rs`（Mac 无需快捷方式）

```rust
#[cfg(target_os = "macos")]
pub fn create_desktop_shortcut(_install_dir: &str) -> Result<()> {
    // Mac App Bundle 自动出现在 Launchpad
    // 无需手动创建快捷方式
    Ok(())
}

#[cfg(target_os = "macos")]
pub fn create_start_menu_shortcut(_install_dir: &str) -> Result<()> {
    // Mac 无 Start Menu
    Ok(())
}

#[cfg(target_os = "macos")]
pub fn delete_all_shortcuts() -> Result<()> {
    // App Bundle 删除后自动从 Launchpad 移除
    Ok(())
}
```

#### 2.6 改造 `installer.rs`（⚠️ CRITICAL-02 修复）

文件：`src-tauri/src/services/installer.rs`

> **⚠️ CRITICAL-02 修复**：现有代码硬编码 `.exe` 后缀，需要添加条件编译。

**现有代码修改点清单**：

| 行号（估算） | 现有代码 | Mac 修复 |
|-------------|----------|----------|
| ~50 | `colink-server.exe` | `colink-server`（无后缀） |
| ~80 | `migrate.exe` | `migrate`（无后缀） |
| ~120 | `bin/migrate.exe` | `bin/migrate`（无后缀） |

**修复示例**：

```rust
// ❌ 现有代码
let migrate_exe = format!("{}/bin/migrate.exe", install_dir);

// ✅ 修复方案
#[cfg(target_os = "windows")]
let migrate_exe = format!("{}/bin/migrate.exe", install_dir);

#[cfg(target_os = "macos")]
let migrate_exe = format!("{}/Contents/MacOS/migrate", install_dir);
```

**完整 Mac 安装流程**：

```rust
#[cfg(target_os = "macos")]
fn copy_files_and_complete<F>(
    config: &InstallConfig,
    resource_path: &Path,
    install_dir: &Path,
    emit_progress: F,
) -> Result<InstallResult>
where
    F: Fn(&InstallProgress),
{
    // Mac: 安装到 /Applications/Colink.app
    let app_bundle_path = PathBuf::from("/Applications/Colink.app");

    // 检查是否已存在（升级）
    if app_bundle_path.exists() {
        emit_progress(&InstallProgress {
            step: "prepare".to_string(),
            status: "running".to_string(),
            message: Some("准备升级...".into()),
        });

        // ⚠️ CRITICAL-01 修复：不需要保留 data 目录
        // data 目录在 ~/Library/Application Support/，不受升级影响
    }

    // 创建 App Bundle（不含 data）
    let version = config.version.clone();
    create_app_bundle(resource_path, &app_bundle_path, &version)?;

    // ⚠️ CRITICAL-01 修复：不写入配置文件到 App Bundle
    // 配置文件在首次运行时写入 ~/Library/Application Support/Colink/

    // 写入 plist（记录安装信息）
    let data_dir = get_mac_data_dir();
    write_install_plist(
        &app_bundle_path.to_string_lossy(),
        &data_dir.to_string_lossy(),
        &version,
    )?;

    Ok(InstallResult { success: true, .. })
}

#[cfg(target_os = "macos")]
fn run_database_migration(install_dir: &Path, version: &str) -> Result<()> {
    // ⚠️ CRITICAL-02 修复：migrate 路径
    let migrate_exe = install_dir.join("Contents/MacOS/migrate");  // 无 .exe

    // ⚠️ CRITICAL-01 修复：数据库路径
    let data_dir = get_mac_data_dir();
    let db_path = data_dir.join("sqlite/colink.db");

    let output = Command::new(&migrate_exe)
        .args([
            "up",
            "--db", &db_path.to_string_lossy(),
            "--version", version,
        ])
        .output()
        .map_err(|e| InstallerError::Process(e.to_string()))?;

    if !output.status.success() {
        return Err(InstallerError::Process(
            String::from_utf8_lossy(&output.stderr).to_string()
        ));
    }
    Ok(())
}
```

### 3. DMG 创建

#### 3.1 使用 hdiutil（系统自带）

```bash
# 创建临时目录
mkdir -p dmg-temp

# 复制 App Bundle
cp -R /Applications/Colink.app dmg-temp/

# 创建 DMG
hdiutil create -volname "Colink" \
    -srcfolder dmg-temp \
    -ov -format UDZO \
    Colink-Setup.dmg

# 清理
rm -rf dmg-temp
```

#### 3.2 可选：使用 dmgcanvas（自定义背景）

如需自定义 DMG 背景（如安装指引箭头），可使用 [dmgcanvas](https://github.com/create-dmg/create-dmg)。

### 4. 构建流程

新增 `scripts/build-mac.sh`：

```bash
#!/bin/bash
# Colink Mac Release Build

set -e

PROJECT_ROOT=$(dirname "$0")/..
INSTALLER_DIR="$PROJECT_ROOT/installer-tauri"
SRC_TAURI_DIR="$INSTALLER_DIR/src-tauri"

VERSION=$(cat "$PROJECT_ROOT/VERSION")
BUILD_TIME=$(date +"%Y%m%d-%H%M%S")

echo "=== Colink Mac Release Build ==="
echo "Version: $VERSION"
echo "Build Time: $BUILD_TIME"

# Step 1: Build backend (Go)
echo "[1/7] Building backend..."
cd "$PROJECT_ROOT"
go build -ldflags "-X main.Version=v$VERSION-$BUILD_TIME" -o bin/colink-server ./cmd/server
go build -o bin/migrate ./cmd/migrate
echo "Backend built"

# Step 2: Build frontend
echo "[2/7] Building frontend..."
cd "$PROJECT_ROOT/web"
npm run build
echo "Frontend built"

# Step 3: Sync resources
echo "[3/7] Syncing resources..."
cd "$PROJECT_ROOT"
node scripts/sync-resources.js "$SRC_TAURI_DIR/target/release/staging/resources"
echo "Resources synced"

# Step 4: Build Tauri (macOS)
echo "[4/7] Building Tauri..."
cd "$INSTALLER_DIR"
pnpm build:renderer

# 检测架构
ARCH=$(uname -m)
if [ "$ARCH" == "arm64" ]; then
    TARGET="aarch64-apple-darwin"
else
    TARGET="x86_64-apple-darwin"
fi

cd "$SRC_TAURI_DIR"
cargo build --release --target $TARGET
echo "Tauri built for $TARGET"

# Step 5: Create App Bundle
echo "[5/7] Creating App Bundle..."
APP_BUNDLE="$SRC_TAURI_DIR/target/release/Colink.app"
STAGING="$SRC_TAURI_DIR/target/release/staging"

# 运行 Rust 函数创建 bundle（通过 cargo run --bin create-bundle）
# 或使用 shell 脚本
./scripts/create-app-bundle.sh "$STAGING/resources" "$APP_BUNDLE" "$VERSION"

# Step 6: Create DMG
echo "[6/7] Creating DMG..."
DMG_NAME="Colink-Setup-$VERSION-$BUILD_TIME.dmg"
DMG_PATH="$SRC_TAURI_DIR/target/release/dist/$DMG_NAME"

mkdir -p "$SRC_TAURI_DIR/target/release/dist"
hdiutil create -volname "Colink" \
    -srcfolder "$APP_BUNDLE" \
    -ov -format UDZO \
    "$DMG_PATH"

# Step 7: Cleanup
echo "[7/7] Cleanup..."
# 移动图标缓存等
echo "Build complete!"
echo "Output: $DMG_PATH"
```

### 5. 安装流程（⚠️ CRITICAL-01 修复）

#### 5.1 用户安装步骤

1. 下载 `Colink-Setup.dmg`
2. 双击打开 DMG
3. 拖拽 `Colink.app` 到 `/Applications`
4. 打开 `/Applications/Colink.app`（Launcher）
5. **首次运行**：Launcher 自动创建 `~/Library/Application Support/Colink/` 数据目录
6. 点击「启动服务」

#### 5.2 升级流程（⚠️ 数据安全）

1. 下载新版 DMG
2. 拖拽覆盖 `/Applications/Colink.app`
3. **数据目录不受影响**：`~/Library/Application Support/Colink/` 保持不变
4. Launcher 启动时自动检测并执行数据库迁移

> **对比 Windows 升级**：Windows 升级时 `%APPDATA%/Colink/` 数据目录同样保留，两平台行为一致。

#### 5.3 卸载流程

1. 停止服务（Launcher）
2. 拖拽 `/Applications/Colink.app` 到废纸篓
3. **可选**：删除用户数据目录 `~/Library/Application Support/Colink/`
4. **可选**：删除偏好设置 `~/Library/Preferences/com.colink.installer.plist`

### 6. 测试要点（⚠️ CRITICAL-01 修复后新增）

| 测试项 | Mac 特定验证 |
|--------|-------------|
| App Bundle 结构 | Info.plist 正确，可双击启动 |
| 权限 | 可执行文件有 +x 权限 |
| 服务启动 | lsof 检测端口，kill 终止进程 |
| **⚠️ 数据目录位置** | 验证在 `~/Library/Application Support/Colink/`（不在 App Bundle内） |
| **⚠️ 首次运行初始化** | Launcher 自动创建数据目录并复制 config.yaml.example |
| **⚠️ 升级数据保留** | 拖拽覆盖 App Bundle 后，数据库/配置/repos 仍存在 |
| DMG 安装 | 拖拽到 Applications 正常 |
| 卸载 | 拖拽到废纸篓清理干净（可选保留数据） |

**新增测试用例**：

```rust
// auto-test/internal/services/bundle_test.rs

#[test]
fn test_data_dir_location() {
    let data_dir = get_mac_data_dir();
    assert!(data_dir.to_string_lossy().contains("Application Support"));
    assert!(!data_dir.to_string_lossy().contains(".app"));
}

#[test]
fn test_upgrade_preserves_data() {
    // 模拟升级场景
    // 1. 创建旧版本 App Bundle + data 目录
    // 2. 覆盖 App Bundle（不含 data）
    // 3. 验证 data 目录内容保留
}
```

## 文件清单

### 新增文件

| 文件 | 说明 |
|------|------|
| `src-tauri/src/services/bundle.rs` | App Bundle 创建 |
| `src-tauri/src/services/plist.rs` | plist 文件操作 |
| `scripts/build-mac.sh` | Mac 构建脚本 |
| `scripts/create-app-bundle.sh` | App Bundle 创建脚本 |
| `src-tauri/icons/icon.icns` | Mac 图标（从 icon.png 生成） |

### 改造文件

| 文件 | 改造内容 |
|------|----------|
| `src-tauri/src/services/service_manager.rs` | Mac 进程管理 |
| `src-tauri/src/services/file_ops.rs` | Mac 文件操作 |
| `src-tauri/src/services/shortcut.rs` | Mac 无快捷方式 |
| `src-tauri/src/services/installer.rs` | Mac 安装流程 |
| `src-tauri/src/services/registry.rs` | 添加 Mac 分支 |
| `src-tauri/src/services/uninstall.rs` | Mac 卸载逻辑 |
| `src-tauri/Cargo.toml` | 添加 plist 依赖 |
| `src-tauri/tauri.conf.json` | 添加 Mac bundle 配置 |

## Cargo.toml 新增依赖

```toml
[target.'cfg(target_os = "macos")'.dependencies]
plist = "1"  # 解析 plist 文件
```

## tauri.conf.json Mac 配置

```json
{
  "bundle": {
    "macOS": {
      "minimumSystemVersion": "10.15",
      "entitlements": null,
      "exceptionDomain": "",
      "frameworks": [],
      "providerShortName": null,
      "signingIdentity": null
    }
  }
}
```

## 风险与限制（⚠️ 已修复 CRITICAL 问题）

| 风险 | 应对 | 状态 |
|------|------|------|
| Apple 签名 | 未签名 App 可能被 Gatekeeper 阻止，用户需在「系统偏好设置 → 安全性」允许 | 待优化 |
| 权限问题 | `/Applications` 需管理员权限，考虑用户级安装 `~/Applications` | 待优化 |
| 架构支持 | 需分别构建 ARM（aarch64）和 Intel（x86_64）版本 | 待优化 |
| **⚠️ 升级数据丢失** | 移动数据目录到 `~/Library/Application Support/` | **已修复** |
| **⚠️ .exe 硬编码** | 添加 `#[cfg(target_os)]` 条件编译 | **已修复** |

**已修复的 CRITICAL 问题**：

1. **CRITICAL-01：数据存储位置**
   - 问题：原设计将 data 目录放在 App Bundle 内，升级会丢失数据
   - 解决：移动到 `~/Library/Application Support/Colink/`
   - 影响：设计文档中所有 `data/` 路径引用已更新

2. **CRITICAL-02：.exe 硬编码**
   - 问题：现有代码硬编码 `.exe` 后缀，Mac 上无法运行
   - 解决：标注修改点，添加条件编译
   - 影响文件：`service_manager.rs`, `installer.rs`, `uninstall.rs`

## 后续优化

1. **Apple 开发者签名**：提升用户体验，避免 Gatekeeper 阻止
2. **Universal Binary**：合并 ARM + Intel 为单一 DMG
3. **自动更新**：使用 Sparkle 框架实现 Mac 自动更新
4. **LaunchAgent**：可选的系统级服务自启动

## 参考

- [Mac App Bundle Structure](https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFBundles/BundleTypes/BundleTypes.html)
- [Tauri macOS Configuration](https://tauri.app/v1/guides/building/macos/)
- [hdiutil man page](https://ss64.com/osx/hdiutil.html)

---

## GSTACK REVIEW REPORT (autoplan) — v1.1.0 修复版

### 原审查结果（v1.0.0）

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 1 | issues_open | 2 critical, 4 high, 4 medium |
| Design Review | `/plan-design-review` | UI/UX gaps | 1 | issues_open | score 2.8/10, 11 states missing |
| Eng Review | `/plan-eng-review` | Architecture & tests | 1 | issues_open | 1 critical, 5 high, 12 medium |
| DX Review | `/plan-devex-review` | Developer experience | 1 | issues_open | score 3.6/10, TTHW 115s |

### CRITICAL 修复状态

| 问题 | 状态 | 修复内容 |
|------|------|----------|
| **CRITICAL-01：数据存储位置** | ✅ 已修复 | 数据目录移至 `~/Library/Application Support/Colink/`，升级不受影响 |
| **CRITICAL-02：.exe 硬编码** | ✅ 已修复 | 标注所有修改点，添加 `#[cfg(target_os)]` 条件编译 |
| **CRITICAL-03：错误处理策略** | ⏳ 待优化 | 需后续补充 Error Handling 文档 |

### 修复详情

**CRITICAL-01 修复**：
- App Bundle 结构：移除内部 `data/` 目录
- 新增独立数据目录：`~/Library/Application Support/Colink/`
- 所有代码引用：`/Applications/Colink.app/data/` → `~/Library/Application Support/Colink/`
- 首次运行逻辑：Launcher 自动创建数据目录并初始化配置

**CRITICAL-02 修复**：
- 标注修改文件清单：`service_manager.rs`, `installer.rs`, `plist.rs`
- 添加条件编译示例：`#[cfg(target_os = "windows")]` vs `#[cfg(target_os = "macos")]`
- 所有 `.exe` 引用改为跨平台路径

### 待修复项（HIGH）

| 问题 | 优先级 | 建议 |
|------|--------|------|
| Gatekeeper 阻止 | HIGH | 添加用户指引文档，说明「系统偏好设置 → 安全性」允许流程 |
| 权限拒绝无回退 | HIGH | 添加 `~/Applications` 回退安装路径 |
| plist 解析 regex | HIGH | 改用 plist crate（已在代码示例中修复） |
| 11 个 UI 状态缺失 | HIGH | 需补充 UI State 文档（非阻塞性） |

---

## 代码修改点汇总（⚠️ 实施前必须处理）

| 文件 | 修改内容 | 影响范围 |
|------|----------|----------|
| `src-tauri/src/services/bundle.rs` | 新增文件，App Bundle 创建逻辑 | Mac 新增 |
| `src-tauri/src/services/plist.rs` | 新增文件，替代 registry | Mac 新增 |
| `src-tauri/src/services/service_manager.rs` | 1. 添加 Mac 进程检测（lsof/pkill）<br>2. ⚠️ 修复 `.exe` 硬编码（行 ~148）<br>3. ⚠️ 修复配置路径（`data/configs/` → `~/Library/...`） | Mac 改造 + CRITICAL 修复 |
| `src-tauri/src/services/installer.rs` | 1. 添加 Mac 安装流程<br>2. ⚠️ 修复 `migrate.exe` 硬编码（行 ~50, ~80, ~120）<br>3. ⚠️ 修复数据目录路径 | Mac 改造 + CRITICAL 修复 |
| `src-tauri/src/services/file_ops.rs` | 添加 Mac 文件操作（pgrep/pkill） | Mac 改造 |
| `src-tauri/src/services/shortcut.rs` | Mac 无快捷方式（空实现） | Mac 改造 |
| `src-tauri/src/services/uninstall.rs` | 1. 添加 Mac 卸载流程<br>2. ⚠️ 修复 `.exe` 硬编码<br>3. ⚠️ 添加用户数据清理选项 | Mac 改造 + CRITICAL 修复 |
| `src-tauri/Cargo.toml` | 添加 `plist` 依赖 | Mac 新增依赖 |
| `src-tauri/tauri.conf.json` | 添加 Mac bundle 配置 | Mac 配置 |
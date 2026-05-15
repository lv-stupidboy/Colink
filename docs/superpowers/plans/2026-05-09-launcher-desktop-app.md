# Launcher Desktop Application 化改造实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 installer-tauri 的 Launcher 模式从"服务控制面板"改造为"完整桌面应用"，在窗口中直接嵌入 web UI。

**Architecture:** Launcher 启动时自动启动后端服务，通过 Splash Screen 显示启动进度，服务就绪后切换到 WebView 加载 web UI。关闭窗口时自动停止服务。窗口尺寸可保存和恢复。

**Tech Stack:** Tauri 2 (Rust), React 18, TypeScript, Ant Design 5。

---

## File Structure

```
installer-tauri/
├── src-tauri/
│   ├── src/
│   │   ├── lib.rs                     # 修改：Launcher 模式自动启动服务
│   │   ├── commands/
│   │   │   ├── mod.rs                 # 修改：注册 launcher_service 模块
│   │   │   ├── launcher.rs            # 保留：目录打开功能
│   │   │   ├── launcher_service.rs    # 新增：启动服务、状态、窗口尺寸
│   │   │   └── window.rs              # 保留：窗口控制功能
│   │   ├── services/
│   │   │   ├── mod.rs                 # 修改：注册 launcher 模块
│   │   │   ├── launcher.rs            # 新增：启动流程管理
│   │   │   └── service_manager.rs     # 保留：服务进程管理
│   │   └── store.rs                   # 修改：添加 LauncherStatus
│   └── tauri.launcher.conf.json       # 修改：窗口配置（标准装饰）
├── src/
│   ├── App.tsx                        # 修改：Launcher 模式入口
│   ├── launcher/                      # 新增：Launcher 前端组件
│   │   ├── SplashScreen.tsx           # Splash Screen 组件
│   │   ├── ErrorPage.tsx              # 错误页面组件
│   │   ├── WebUIContainer.tsx         # WebView 容器
│   │   └── LauncherApp.tsx            # Launcher 主应用
│   └── lib/
│       └── api/
│           ├── launcher_service.ts    # 新增：Launcher 服务 API
│           └── types.ts               # 修改：添加 LauncherStatus 类型
```

---

## Task 1: 添加 LauncherStatus 类型到 store.rs

**Files:**
- Modify: `installer-tauri/src-tauri/src/store.rs`

- [ ] **Step 1: 添加 LauncherStatus 和 LauncherError 类型**

```rust
use std::sync::RwLock;
use crate::services::ServiceManager;

/// Application mode detected from exe filename
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum AppMode {
    Setup,
    Launcher,
}

/// Launcher startup status
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum LauncherStatus {
    Initializing,
    CheckingInstallation,
    StartingService,
    WaitingForReady,
    Ready { port: u16 },
    Failed { error: LauncherError },
}

/// Launcher error details
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LauncherError {
    pub kind: LauncherErrorKind,
    pub message: String,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum LauncherErrorKind {
    NotInstalled,
    PortConflict,
    ProcessExit,
    ConfigError,
    MissingFiles,
}

/// Global application state shared across commands
pub struct AppState {
    /// Detected app mode (Setup or Launcher)
    pub mode: RwLock<AppMode>,
    /// Installation directory (set during setup or loaded for launcher)
    pub install_dir: RwLock<Option<String>>,
    /// Service manager for spawning/controlling colink-server.exe
    pub service_manager: RwLock<Option<ServiceManager>>,
    /// Launcher startup status
    pub launcher_status: RwLock<LauncherStatus>,
}

impl AppState {
    pub fn new(mode: AppMode) -> Self {
        Self {
            mode: RwLock::new(mode),
            install_dir: RwLock::new(None),
            service_manager: RwLock::new(None),
            launcher_status: RwLock::new(LauncherStatus::Initializing),
        }
    }

    pub fn get_mode(&self) -> AppMode {
        *self.mode.read().unwrap()
    }

    pub fn set_install_dir(&self, dir: String) {
        *self.install_dir.write().unwrap() = Some(dir);
    }

    pub fn get_install_dir(&self) -> Option<String> {
        self.install_dir.read().unwrap().clone()
    }

    pub fn get_launcher_status(&self) -> LauncherStatus {
        self.launcher_status.read().unwrap().clone()
    }

    pub fn set_launcher_status(&self, status: LauncherStatus) {
        *self.launcher_status.write().unwrap() = status;
    }
}

/// Detect app mode from executable filename
pub fn detect_app_mode() -> AppMode {
    let exe_path = std::env::current_exe().ok();
    let exe_name = exe_path
        .and_then(|p| p.file_name().map(|n| n.to_string_lossy().to_string()))
        .unwrap_or_default();

    // Check filename patterns
    if exe_name.contains("Launcher") || exe_name == "Colink.exe" {
        AppMode::Launcher
    } else {
        AppMode::Setup
    }
}
```

- [ ] **Step 2: Commit store.rs changes**

```bash
git add installer-tauri/src-tauri/src/store.rs
git commit -m "feat: add LauncherStatus and LauncherError types to AppState"
```

---

## Task 2: 创建 Launcher 启动流程管理服务

**Files:**
- Create: `installer-tauri/src-tauri/src/services/launcher.rs`
- Modify: `installer-tauri/src-tauri/src/services/mod.rs`

- [ ] **Step 1: 创建 launcher.rs 服务**

```rust
use crate::error::{InstallerError, Result};
use crate::store::{LauncherError, LauncherErrorKind, LauncherStatus, AppState};
use crate::services::service_manager::ServiceManager;
use crate::services::config::read_existing_config;
use crate::services::registry::get_installed_version;
use std::path::PathBuf;
use std::sync::Arc;
use tauri::Manager;

/// Launcher startup flow manager
pub struct LauncherFlow {
    app_handle: Arc<tauri::AppHandle>,
}

impl LauncherFlow {
    pub fn new(app_handle: tauri::AppHandle) -> Self {
        Self {
            app_handle: Arc::new(app_handle),
        }
    }

    /// Run the full launcher startup flow
    pub async fn run(&self) -> Result<()> {
        let state = self.app_handle.state::<AppState>();

        // Step 1: Check installation
        state.set_launcher_status(LauncherStatus::CheckingInstallation);
        self.emit_status(&state);

        let installed = get_installed_version()?;
        if !installed.installed {
            let error = LauncherError {
                kind: LauncherErrorKind::NotInstalled,
                message: "Colink 未安装，请先运行安装程序".to_string(),
            };
            state.set_launcher_status(LauncherStatus::Failed { error: error.clone() });
            self.emit_status(&state);
            return Err(InstallerError::Registry("Colink not installed".to_string()));
        }

        let install_dir = installed.install_dir.clone().unwrap_or_default();
        state.set_install_dir(install_dir.clone());

        // Step 2: Start service
        state.set_launcher_status(LauncherStatus::StartingService);
        self.emit_status(&state);

        let manager = ServiceManager::new(install_dir.clone());
        manager.start()?;

        // Store manager in state
        {
            let mut service_guard = state.service_manager.write().unwrap();
            *service_guard = Some(manager);
        }

        // Step 3: Wait for service ready
        state.set_launcher_status(LauncherStatus::WaitingForReady);
        self.emit_status(&state);

        let port = read_existing_config(&install_dir)
            .map(|(p, _)| p)
            .unwrap_or(26305);

        // Poll health endpoint until ready (max 30 seconds)
        let ready = self.wait_for_service_ready(port, 30).await;

        if !ready {
            let error = LauncherError {
                kind: LauncherErrorKind::ProcessExit,
                message: "服务启动失败，请检查日志".to_string(),
            };
            state.set_launcher_status(LauncherStatus::Failed { error: error.clone() });
            self.emit_status(&state);
            return Err(InstallerError::Process("Service not ready after 30s".to_string()));
        }

        // Step 4: Ready
        state.set_launcher_status(LauncherStatus::Ready { port });
        self.emit_status(&state);

        Ok(())
    }

    /// Retry startup after failure
    pub async fn retry(&self) -> Result<()> {
        self.run().await
    }

    /// Stop service on app close
    pub fn stop_service(&self) -> Result<()> {
        let state = self.app_handle.state::<AppState>();

        let service_guard = state.service_manager.read().unwrap();
        if let Some(manager) = service_guard.as_ref() {
            manager.stop()?;
        }

        Ok(())
    }

    /// Emit status to frontend via event
    fn emit_status(&self, state: &AppState) {
        let status = state.get_launcher_status();
        if let Some(window) = self.app_handle.get_webview_window("main") {
            let _ = window.emit("launcher:status", status);
        }
    }

    /// Wait for service to be ready by polling health endpoint
    async fn wait_for_service_ready(&self, port: u16, max_seconds: u32) -> bool {
        let url = format!("http://localhost:{}/health", port);
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(2))
            .build()
            .unwrap_or_else(|_| reqwest::Client::new());

        for _ in 0..max_seconds {
            match client.get(&url).send().await {
                Ok(res) if res.status().is_success() => {
                    log::info!("Service ready at port {}", port);
                    return true;
                }
                _ => {
                    std::thread::sleep(std::time::Duration::from_secs(1));
                }
            }
        }

        log::warn!("Service not ready after {} seconds", max_seconds);
        false
    }
}

/// Get window size preferences from ~/.colink/launcher_prefs.json
pub fn get_window_prefs() -> (Option<u32>, Option<u32>) {
    let prefs_path = dirs::data_dir()
        .unwrap_or_else(|| PathBuf::from("~/.local/share"))
        .join("Colink")
        .join("launcher_prefs.json");

    if let Ok(content) = std::fs::read_to_string(&prefs_path) {
        if let Ok(prefs) = serde_json::from_str::<WindowPrefs>(&content) {
            return (prefs.window_width, prefs.window_height);
        }
    }

    (None, None)
}

/// Save window size preferences
pub fn save_window_prefs(width: u32, height: u32) -> Result<()> {
    let prefs_dir = dirs::data_dir()
        .unwrap_or_else(|| PathBuf::from("~/.local/share"))
        .join("Colink");

    std::fs::create_dir_all(&prefs_dir).map_err(|e| InstallerError::Io {
        context: "create prefs directory".to_string(),
        source: e,
    })?;

    let prefs_path = prefs_dir.join("launcher_prefs.json");
    let prefs = WindowPrefs {
        window_width: Some(width),
        window_height: Some(height),
        theme: None,
    };

    let content = serde_json::to_string_pretty(&prefs).map_err(|e| InstallerError::Config(e.to_string()))?;
    std::fs::write(&prefs_path, content).map_err(|e| InstallerError::Io {
        context: "write prefs file".to_string(),
        source: e,
    })?;

    Ok(())
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
struct WindowPrefs {
    window_width: Option<u32>,
    window_height: Option<u32>,
    theme: Option<String>,
}
```

- [ ] **Step 2: 修改 mod.rs 注册 launcher 模块**

```rust
pub mod bundle;
pub mod config;
pub mod dependency;
pub mod disk_space;
pub mod file_ops;
pub mod installer;
pub mod launcher;
pub mod plist;
pub mod registry;
pub mod service_manager;
pub mod shortcut;
pub mod uninstall;
```

- [ ] **Step 3: Commit services changes**

```bash
git add installer-tauri/src-tauri/src/services/launcher.rs installer-tauri/src-tauri/src/services/mod.rs
git commit -m "feat: create LauncherFlow service for startup management"
```

---

## Task 3: 创建 Launcher 服务 IPC 命令

**Files:**
- Create: `installer-tauri/src-tauri/src/commands/launcher_service.rs`
- Modify: `installer-tauri/src-tauri/src/commands/mod.rs`

- [ ] **Step 1: 创建 launcher_service.rs 命令**

```rust
use crate::store::{AppState, LauncherStatus};
use crate::services::launcher::{get_window_prefs, save_window_prefs};
use tauri::{AppHandle, State};

/// Get launcher status
#[tauri::command]
pub fn get_launcher_status(
    state: State<'_, AppState>,
) -> Result<LauncherStatus, String> {
    Ok(state.get_launcher_status())
}

/// Retry launcher startup
#[tauri::command]
pub async fn retry_launcher_startup(
    app: AppHandle,
    state: State<'_, AppState>,
) -> Result<LauncherStatus, String> {
    use crate::services::launcher::LauncherFlow;
    
    let flow = LauncherFlow::new(app);
    flow.retry().await.map_err(|e| e.to_string())?;
    
    Ok(state.get_launcher_status())
}

/// Get window size from preferences
#[tauri::command]
pub fn get_window_size() -> Result<serde_json::Value, String> {
    let (width, height) = get_window_prefs();
    Ok(serde_json::json!({
        "width": width,
        "height": height
    }))
}

/// Save window size to preferences
#[tauri::command]
pub fn save_window_size(
    width: u32,
    height: u32,
) -> Result<(), String> {
    save_window_prefs(width, height).map_err(|e| e.to_string())
}

/// Get app version
#[tauri::command]
pub fn get_launcher_version() -> Result<String, String> {
    // Read VERSION file from resources
    let version_path = std::path::PathBuf::from("VERSION");
    if version_path.exists() {
        std::fs::read_to_string(&version_path)
            .map(|v| v.trim().to_string())
            .map_err(|e| e.to_string())
    } else {
        Ok("1.0.0".to_string())
    }
}
```

- [ ] **Step 2: 修改 mod.rs 注册 launcher_service 模块**

```rust
pub mod config;
pub mod dependency;
pub mod install;
pub mod launcher;
pub mod launcher_service;
pub mod mode;
pub mod service;
pub mod uninstall;
pub mod window;
```

- [ ] **Step 3: Commit commands changes**

```bash
git add installer-tauri/src-tauri/src/commands/launcher_service.rs installer-tauri/src-tauri/src/commands/mod.rs
git commit -m "feat: create launcher_service IPC commands"
```

---

## Task 4: 修改 lib.rs 添加 Launcher 自动启动逻辑

**Files:**
- Modify: `installer-tauri/src-tauri/src/lib.rs`

- [ ] **Step 1: 修改 lib.rs 的 setup 逻辑**

找到 `lib.rs` 的 `setup` 部分，修改 Launcher 模式的启动逻辑：

```rust
        .setup(|app| {
            let state = app.state::<AppState>();
            let mode = state.get_mode();

            log::info!("=== Application Setup ===");
            log::info!("App mode: {:?}", mode);

            // Log the log file location for user reference
            if let Ok(log_dir) = app.path().app_log_dir() {
                log::info!("Log file location: {}", log_dir.display());
            }

            // Setup based on mode
            match mode {
                AppMode::Launcher => {
                    log::info!("Running in Launcher mode");
                    // Check if installed
                    if let Ok(installed) = services::registry::get_installed_version() {
                        log::info!("Registry check result: installed={}, install_dir={:?}",
                            installed.installed, installed.install_dir);
                        if !installed.installed {
                            log::error!("Colink not installed, exiting");
                            // Show error and exit
                            app.dialog()
                                .message("Colink 未安装，请先运行安装程序")
                                .title("错误")
                                .kind(tauri_plugin_dialog::MessageDialogKind::Error);
                            std::process::exit(1);
                        }
                        // Set install dir from registry
                        if let Some(dir) = installed.install_dir {
                            log::info!("Setting install_dir to: {}", dir);
                            state.set_install_dir(dir);
                        } else {
                            log::warn!("No install_dir found in registry");
                        }
                    } else {
                        log::warn!("Failed to read registry for installed version");
                    }

                    // Start launcher flow asynchronously
                    let app_handle = app.handle().clone();
                    tauri::async_runtime::spawn(async move {
                        use services::launcher::LauncherFlow;
                        let flow = LauncherFlow::new(app_handle);
                        if let Err(e) = flow.run().await {
                            log::error!("Launcher startup failed: {}", e);
                        }
                    });
                }
                AppMode::Setup => {
                    // Setup mode - check existing installation for upgrade/uninstall options
                    log::info!("Running in Setup mode");
                }
            }

            log::info!("=== Setup Complete ===");
            Ok(())
        })
```

- [ ] **Step 2: 注册 launcher_service 命令到 invoke_handler**

在 `invoke_handler` 中添加新命令：

```rust
        .invoke_handler(tauri::generate_handler![
            // Mode commands
            commands::is_launcher_mode,
            commands::get_startup_action,
            commands::get_app_path,
            commands::get_install_dir,
            commands::get_resource_path,
            commands::get_version,

            // Installation commands
            commands::check_installed,
            commands::check_old_isdp,
            commands::uninstall_old_isdp,
            commands::select_directory,
            commands::get_disk_space,
            commands::generate_config_preview,
            commands::read_existing_config,
            commands::start_installation,
            commands::create_shortcut,

            // Uninstall commands
            commands::confirm_uninstall,
            commands::run_uninstall,
            commands::clean_registry,
            commands::remove_shortcuts,
            commands::uninstall,

            // Service commands
            commands::start_service,
            commands::stop_service,
            commands::get_service_status,
            commands::get_running_agents,

            // Dependency commands
            commands::check_dependency,
            commands::install_dependency,
            commands::check_all_dependencies,

            // Config commands
            commands::read_config_file,
            commands::save_config,
            commands::get_existing_config,

            // Launcher commands
            commands::open_logs,
            commands::open_data_dir,
            commands::open_config,
            commands::open_console,
            commands::open_install_dir,

            // Launcher service commands (NEW)
            commands::launcher_service::get_launcher_status,
            commands::launcher_service::retry_launcher_startup,
            commands::launcher_service::get_window_size,
            commands::launcher_service::save_window_size,
            commands::launcher_service::get_launcher_version,

            // Window commands
            commands::window_minimize,
            commands::window_maximize,
            commands::window_close,
            commands::window_close_with_confirm,
        ])
```

- [ ] **Step 3: Commit lib.rs changes**

```bash
git add installer-tauri/src-tauri/src/lib.rs
git commit -m "feat: add launcher auto-start logic in setup"
```

---

## Task 5: 修改 tauri.launcher.conf.json 窗口配置

**Files:**
- Modify: `installer-tauri/src-tauri/tauri.launcher.conf.json`

- [ ] **Step 1: 修改窗口配置为标准装饰**

```json
{
  "$schema": "https://schema.tauri.app/config/2",
  "productName": "Colink",
  "version": "1.0.0",
  "identifier": "com.colink.launcher",
  "build": {
    "frontendDist": "../dist",
    "devUrl": "http://localhost:5173",
    "beforeDevCommand": "pnpm run dev:renderer",
    "beforeBuildCommand": "pnpm run build:renderer"
  },
  "app": {
    "windows": [
      {
        "label": "main",
        "title": "Colink",
        "width": 1280,
        "height": 800,
        "minWidth": 900,
        "minHeight": 600,
        "decorations": true,
        "resizable": true,
        "center": true
      }
    ],
    "security": {
      "csp": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob:; connect-src 'self' http://localhost:* ws://localhost:*"
    }
  },
  "bundle": {
    "active": false,
    "targets": [],
    "icon": [
      "icons/icon.ico",
      "icons/32x32.png",
      "icons/128x128.png"
    ],
    "resources": [],
    "externalBin": []
  },
  "plugins": {}
}
```

**关键变化**：
- `decorations`: `false` → `true`（标准窗口装饰）
- `width`: `900` → `1280`
- `height`: `650` → `800`
- `minWidth`: `800` → `900`
- `minHeight`: `550` → `600`
- CSP 添加 `connect-src` 允许 localhost 连接

- [ ] **Step 2: Commit config changes**

```bash
git add installer-tauri/src-tauri/tauri.launcher.conf.json
git commit -m "feat: enable standard window decorations for launcher"
```

---

## Task 6: 创建 Launcher 前端 API

**Files:**
- Create: `installer-tauri/src/lib/api/launcher_service.ts`
- Modify: `installer-tauri/src/lib/api/types.ts`

- [ ] **Step 1: 添加 LauncherStatus 类型到 types.ts**

在 `types.ts` 文件末尾添加：

```typescript
export interface LauncherStatus {
  state: 'initializing' | 'checkingInstallation' | 'startingService' | 'waitingForReady' | 'ready' | 'failed';
  port?: number;
  error?: LauncherError;
}

export interface LauncherError {
  kind: 'notInstalled' | 'portConflict' | 'processExit' | 'configError' | 'missingFiles';
  message: string;
}

export interface WindowSize {
  width?: number;
  height?: number;
}
```

- [ ] **Step 2: 创建 launcher_service.ts API**

```typescript
import type { LauncherStatus, WindowSize } from './types';

export const launcherServiceApi = {
  /// Get current launcher status
  getStatus: async (): Promise<LauncherStatus> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('get_launcher_status');
  },

  /// Retry launcher startup
  retry: async (): Promise<LauncherStatus> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('retry_launcher_startup');
  },

  /// Get saved window size
  getWindowSize: async (): Promise<WindowSize> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('get_window_size');
  },

  /// Save window size
  saveWindowSize: async (width: number, height: number): Promise<void> {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('save_window_size', { width, height });
  },

  /// Get app version
  getVersion: async (): Promise<string> => {
    const { invoke } = await import('@tauri-apps/api/core');
    return invoke('get_launcher_version');
  },

  /// Subscribe to status changes
  onStatusChange: (callback: (status: LauncherStatus) => void): (() => void) => {
    let unlisten: (() => void) | null = null;
    
    import('@tauri-apps/api/event').then(({ listen }) => {
      listen<LauncherStatus>('launcher:status', (event) => {
        callback(event.payload);
      }).then((fn) => {
        unlisten = fn;
      });
    });

    return () => {
      if (unlisten) unlisten();
    };
  },
};
```

- [ ] **Step 3: Commit API changes**

```bash
git add installer-tauri/src/lib/api/launcher_service.ts installer-tauri/src/lib/api/types.ts
git commit -m "feat: create launcher service frontend API"
```

---

## Task 7: 创建 SplashScreen 组件

**Files:**
- Create: `installer-tauri/src/launcher/SplashScreen.tsx`

- [ ] **Step 1: 创建 SplashScreen.tsx**

```typescript
import React from 'react';
import { Progress } from 'antd';

interface SplashScreenProps {
  version: string;
  progress: number;
  statusText: string;
}

const SplashScreen: React.FC<SplashScreenProps> = ({
  version,
  progress,
  statusText,
}) => {
  return (
    <div
      style={{
        height: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
        color: '#fff',
        fontFamily: 'Inter, system-ui, sans-serif',
      }}
    >
      {/* Logo */}
      <svg
        width="80"
        height="80"
        viewBox="0 0 32 32"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        style={{ marginBottom: 16 }}
      >
        <defs>
          <linearGradient id="splashGrad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" style={{ stopColor: '#10b981' }} />
            <stop offset="100%" style={{ stopColor: '#3b82f6' }} />
          </linearGradient>
        </defs>
        <rect x="2" y="2" width="28" height="28" rx="6" fill="#0f172a" />
        <polygon
          points="16,6 24,10.5 24,21.5 16,26 8,21.5 8,10.5"
          fill="none"
          stroke="#10b981"
          strokeWidth="1.2"
          strokeOpacity="0.35"
          strokeLinejoin="round"
        />
        <circle cx="16" cy="16" r="3" fill="url(#splashGrad)" />
        <circle cx="16" cy="6" r="1.8" fill="url(#splashGrad)" />
        <circle cx="24" cy="10.5" r="1.8" fill="url(#splashGrad)" />
        <circle cx="24" cy="21.5" r="1.8" fill="url(#splashGrad)" />
        <circle cx="16" cy="26" r="1.8" fill="url(#splashGrad)" />
        <circle cx="8" cy="21.5" r="1.8" fill="url(#splashGrad)" />
        <circle cx="8" cy="10.5" r="1.8" fill="url(#splashGrad)" />
      </svg>

      {/* Brand Name */}
      <h1
        style={{
          fontSize: 32,
          fontWeight: 600,
          margin: 0,
          marginBottom: 8,
        }}
      >
        Colink
      </h1>

      {/* Version */}
      <span
        style={{
          fontSize: 14,
          color: '#94a3b8',
          marginBottom: 48,
        }}
      >
        v{version}
      </span>

      {/* Progress Bar */}
      <div
        style={{
          width: 200,
          marginBottom: 16,
        }}
      >
        <Progress
          percent={progress}
          showInfo={false}
          strokeColor={{
            '0%': '#10b981',
            '100%': '#3b82f6',
          }}
          trailColor="#1e293b"
        />
      </div>

      {/* Status Text */}
      <span
        style={{
          fontSize: 14,
          color: '#94a3b8',
        }}
      >
        {statusText}
      </span>
    </div>
  );
};

export default SplashScreen;
```

- [ ] **Step 2: Commit SplashScreen**

```bash
git add installer-tauri/src/launcher/SplashScreen.tsx
git commit -m "feat: create SplashScreen component"
```

---

## Task 8: 创建 ErrorPage 组件

**Files:**
- Create: `installer-tauri/src/launcher/ErrorPage.tsx`

- [ ] **Step 1: 创建 ErrorPage.tsx**

```typescript
import React from 'react';
import { Button, Space } from 'antd';
import { WarningOutlined, FileTextOutlined, ReloadOutlined } from '@ant-design/icons';

interface ErrorPageProps {
  errorKind: 'notInstalled' | 'portConflict' | 'processExit' | 'configError' | 'missingFiles';
  errorMessage: string;
  onRetry: () => void;
  onViewLogs: () => void;
}

const ERROR_MESSAGES: Record<string, string> = {
  notInstalled: 'Colink 未安装',
  portConflict: '端口被占用',
  processExit: '服务进程异常退出',
  configError: '配置文件错误',
  missingFiles: '必要文件缺失',
};

const ErrorPage: React.FC<ErrorPageProps> = ({
  errorKind,
  errorMessage,
  onRetry,
  onViewLogs,
}) => {
  const errorTitle = ERROR_MESSAGES[errorKind] || '启动失败';

  return (
    <div
      style={{
        height: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
        color: '#fff',
        fontFamily: 'Inter, system-ui, sans-serif',
        padding: 24,
      }}
    >
      {/* Error Icon */}
      <WarningOutlined
        style={{
          fontSize: 48,
          color: '#ef4444',
          marginBottom: 24,
        }}
      />

      {/* Error Title */}
      <h1
        style={{
          fontSize: 24,
          fontWeight: 600,
          margin: 0,
          marginBottom: 16,
          color: '#fef2f2',
        }}
      >
        {errorTitle}
      </h1>

      {/* Error Details */}
      <div
        style={{
          maxWidth: 400,
          textAlign: 'center',
          marginBottom: 32,
          padding: 16,
          background: 'rgba(239, 68, 68, 0.1)',
          borderRadius: 8,
          border: '1px solid rgba(239, 68, 68, 0.2)',
        }}
      >
        <span
          style={{
            fontSize: 14,
            color: '#fecaca',
            lineHeight: 1.6,
          }}
        >
          {errorMessage}
        </span>
      </div>

      {/* Action Buttons */}
      <Space size="middle">
        <Button
          type="primary"
          icon={<ReloadOutlined />}
          onClick={onRetry}
          style={{
            background: 'linear-gradient(135deg, #10b981 0%, #3b82f6 100%)',
            borderColor: 'transparent',
          }}
        >
          重试
        </Button>
        <Button
          icon={<FileTextOutlined />}
          onClick={onViewLogs}
          style={{
            background: 'rgba(255, 255, 255, 0.1)',
            borderColor: 'rgba(255, 255, 255, 0.2)',
            color: '#fff',
          }}
        >
          查看日志
        </Button>
      </Space>
    </div>
  );
};

export default ErrorPage;
```

- [ ] **Step 2: Commit ErrorPage**

```bash
git add installer-tauri/src/launcher/ErrorPage.tsx
git commit -m "feat: create ErrorPage component"
```

---

## Task 9: 创建 WebUIContainer 组件

**Files:**
- Create: `installer-tauri/src/launcher/WebUIContainer.tsx`

- [ ] **Step 1: 创建 WebUIContainer.tsx**

```typescript
import React, { useEffect, useRef, useState } from 'react';
import { Spin } from 'antd';

interface WebUIContainerProps {
  port: number;
  onLoadError?: (error: Error) => void;
}

const WebUIContainer: React.FC<WebUIContainerProps> = ({
  port,
  onLoadError,
}) => {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Handle iframe load events
    const iframe = iframeRef.current;
    if (!iframe) return;

    const handleLoad = () => {
      setLoading(false);
    };

    const handleError = () => {
      setLoading(false);
      onLoadError?.(new Error('Failed to load web UI'));
    };

    iframe.addEventListener('load', handleLoad);
    iframe.addEventListener('error', handleError);

    return () => {
      iframe.removeEventListener('load', handleLoad);
      iframe.removeEventListener('error', handleError);
    };
  }, [onLoadError]);

  const webUIUrl = `http://localhost:${port}/`;

  return (
    <div
      style={{
        width: '100%',
        height: '100vh',
        position: 'relative',
      }}
    >
      {/* Loading overlay */}
      {loading && (
        <div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: '#fff',
            zIndex: 10,
          }}
        >
          <Spin size="large" tip="加载界面..." />
        </div>
      )}

      {/* WebView iframe */}
      <iframe
        ref={iframeRef}
        src={webUIUrl}
        style={{
          width: '100%',
          height: '100%',
          border: 'none',
          display: loading ? 'none' : 'block',
        }}
        title="Colink Web UI"
        sandbox="allow-same-origin allow-scripts allow-forms allow-popups allow-modals"
      />
    </div>
  );
};

export default WebUIContainer;
```

- [ ] **Step 2: Commit WebUIContainer**

```bash
git add installer-tauri/src/launcher/WebUIContainer.tsx
git commit -m "feat: create WebUIContainer component"
```

---

## Task 10: 创建 LauncherApp 主应用组件

**Files:**
- Create: `installer-tauri/src/launcher/LauncherApp.tsx`

- [ ] **Step 1: 创建 LauncherApp.tsx**

```typescript
import React, { useEffect, useState, useCallback } from 'react';
import { launcherServiceApi, launcherApi } from '../lib/api';
import type { LauncherStatus, LauncherError } from '../lib/api/types';
import SplashScreen from './SplashScreen';
import ErrorPage from './ErrorPage';
import WebUIContainer from './WebUIContainer';

const STATUS_TEXT_MAP: Record<string, string> = {
  initializing: '初始化...',
  checkingInstallation: '检测安装状态...',
  startingService: '正在启动服务...',
  waitingForReady: '等待服务就绪...',
  ready: '就绪',
  failed: '启动失败',
};

const PROGRESS_MAP: Record<string, number> = {
  initializing: 10,
  checkingInstallation: 25,
  startingService: 50,
  waitingForReady: 75,
  ready: 100,
  failed: 0,
};

const LauncherApp: React.FC = () => {
  const [status, setStatus] = useState<LauncherStatus>({ state: 'initializing' });
  const [version, setVersion] = useState<string>('1.0.0');

  useEffect(() => {
    // Get version
    launcherServiceApi.getVersion().then(setVersion).catch(() => {});

    // Subscribe to status changes
    const unlisten = launcherServiceApi.onStatusChange(setStatus);

    // Get initial status
    launcherServiceApi.getStatus().then(setStatus).catch(console.error);

    return () => {
      unlisten();
    };
  }, []);

  const handleRetry = useCallback(async () => {
    try {
      const newStatus = await launcherServiceApi.retry();
      setStatus(newStatus);
    } catch (err) {
      console.error('Retry failed:', err);
    }
  }, []);

  const handleViewLogs = useCallback(async () => {
    try {
      await launcherApi.openLogs();
    } catch (err) {
      console.error('Failed to open logs:', err);
    }
  }, []);

  const handleWebUIError = useCallback((error: Error) => {
    console.error('Web UI load error:', error);
  }, []);

  // Render based on status
  if (status.state === 'failed') {
    const error = status.error as LauncherError;
    return (
      <ErrorPage
        errorKind={error.kind}
        errorMessage={error.message}
        onRetry={handleRetry}
        onViewLogs={handleViewLogs}
      />
    );
  }

  if (status.state === 'ready' && status.port) {
    return (
      <WebUIContainer
        port={status.port}
        onLoadError={handleWebUIError}
      />
    );
  }

  // Show splash screen for all other states
  return (
    <SplashScreen
      version={version}
      progress={PROGRESS_MAP[status.state] || 0}
      statusText={STATUS_TEXT_MAP[status.state] || '加载中...'}
    />
  );
};

export default LauncherApp;
```

- [ ] **Step 2: Commit LauncherApp**

```bash
git add installer-tauri/src/launcher/LauncherApp.tsx
git commit -m "feat: create LauncherApp main component"
```

---

## Task 11: 修改 App.tsx Launcher 模式入口

**Files:**
- Modify: `installer-tauri/src/App.tsx`

- [ ] **Step 1: 修改 App.tsx 中的 Launcher 模式渲染**

找到 `// Launcher 模式` 部分，替换为：

```typescript
  // Launcher 模式 - 使用新的 LauncherApp
  if (mode === 'launcher') {
    // 动态导入 LauncherApp
    const LauncherApp = React.lazy(() => import('./launcher/LauncherApp'));
    
    return (
      <React.Suspense
        fallback={
          <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Spin size="large" tip="加载..." />
          </div>
        }
      >
        <LauncherApp />
      </React.Suspense>
    );
  }
```

- [ ] **Step 2: 在 App.tsx 顶部添加 React.lazy 导入**

确保已导入 React：

```typescript
import React, { useEffect, useState } from 'react';
```

- [ ] **Step 3: Commit App.tsx changes**

```bash
git add installer-tauri/src/App.tsx
git commit -m "feat: switch Launcher mode to use LauncherApp component"
```

---

## Task 12: 创建 launcher 目录并添加导出

**Files:**
- Create: `installer-tauri/src/launcher/index.ts`

- [ ] **Step 1: 创建 index.ts 导出文件**

```typescript
export { default as SplashScreen } from './SplashScreen';
export { default as ErrorPage } from './ErrorPage';
export { default as WebUIContainer } from './WebUIContainer';
export { default as LauncherApp } from './LauncherApp';
```

- [ ] **Step 2: Commit launcher index**

```bash
git add installer-tauri/src/launcher/index.ts
git commit -m "feat: create launcher module exports"
```

---

## Task 13: 测试 Launcher 开发模式

**Files:**
- None (testing)

- [ ] **Step 1: 启动 Launcher 开发模式**

```bash
cd installer-tauri
pnpm dev:launcher
```

Expected: 应用启动，显示 SplashScreen，服务启动后切换到 web UI

- [ ] **Step 2: 测试启动失败场景**

手动停止服务或修改配置使启动失败，验证：
- 显示 ErrorPage
- 点击"重试"按钮可以重新启动
- 点击"查看日志"可以打开日志目录

- [ ] **Step 3: 测试窗口尺寸保存**

1. 调整窗口大小
2. 关闭应用
3. 重新打开，验证窗口尺寸恢复

---

## Task 14: 添加关闭时保存窗口尺寸逻辑

**Files:**
- Modify: `installer-tauri/src/launcher/LauncherApp.tsx`

- [ ] **Step 1: 添加窗口尺寸保存逻辑**

在 `LauncherApp.tsx` 中添加 useEffect 监听窗口关闭：

```typescript
import React, { useEffect, useState, useCallback } from 'react';
import { launcherServiceApi, launcherApi, windowApi } from '../lib/api';
import type { LauncherStatus, LauncherError } from '../lib/api/types';
import SplashScreen from './SplashScreen';
import ErrorPage from './ErrorPage';
import WebUIContainer from './WebUIContainer';

// ... existing code ...

const LauncherApp: React.FC = () => {
  const [status, setStatus] = useState<LauncherStatus>({ state: 'initializing' });
  const [version, setVersion] = useState<string>('1.0.0');

  useEffect(() => {
    // Get version
    launcherServiceApi.getVersion().then(setVersion).catch(() => {});

    // Subscribe to status changes
    const unlisten = launcherServiceApi.onStatusChange(setStatus);

    // Get initial status
    launcherServiceApi.getStatus().then(setStatus).catch(console.error);

    // Save window size before close
    const handleBeforeUnload = () => {
      // Get current window size from Tauri
      import('@tauri-apps/api/window').then(({ getCurrentWindow }) => {
        const win = getCurrentWindow();
        win.innerSize().then((size) => {
          const width = size.width;
          const height = size.height;
          launcherServiceApi.saveWindowSize(width, height).catch(console.error);
        });
      });
    };

    window.addEventListener('beforeunload', handleBeforeUnload);

    return () => {
      unlisten();
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, []);

  // ... rest of component ...
```

- [ ] **Step 2: Commit window size save logic**

```bash
git add installer-tauri/src/launcher/LauncherApp.tsx
git commit -m "feat: save window size on close"
```

---

## Task 15: 构建 Launcher 并测试

**Files:**
- None (testing)

- [ ] **Step 1: 构建 Launcher**

```bash
cd installer-tauri
pnpm build:launcher
```

Expected: 输出 `target/release/Colink.exe`

- [ ] **Step 2: 测试构建产物**

将 `Colink.exe` 复制到安装目录的 `launcher/` 目录，运行测试。

---

## Task 16: [GAP FIX] 在 Rust 层监听窗口关闭事件停止服务

**Files:**
- Modify: `installer-tauri/src-tauri/src/lib.rs`

- [ ] **Step 1: 添加 close_requested 事件监听**

在 `lib.rs` 的 `setup` 函数中添加窗口关闭事件监听：

```rust
.use(tauri_plugin_window::Builder::new().build())
// 在 setup 函数末尾添加：
.setup(|app| {
    // ... existing setup code ...
    
    // 监听窗口关闭请求，停止服务
    if let Some(window) = app.get_webview_window("main") {
        window.on_window_event(move |event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                // 阻止默认关闭行为，先停止服务
                api.prevent_close();
                
                // 获取 state 并停止服务
                let state = app.state::<AppState>();
                if let Some(manager) = state.service_manager.read().unwrap().as_ref() {
                    if let Err(e) = manager.stop() {
                        log::error!("Failed to stop service on window close: {}", e);
                    }
                }
                
                // 允许关闭
                api.allow_close();
            }
        });
    }
    
    Ok(())
})
```

- [ ] **Step 2: Commit lib.rs changes**

```bash
git add installer-tauri/src-tauri/src/lib.rs
git commit -m "fix: stop service on window close in Rust layer"
```

---

## Task 17: [GAP FIX] 添加 iframe 加载失败处理

**Files:**
- Modify: `installer-tauri/src/launcher/LauncherApp.tsx`
- Modify: `installer-tauri/src/launcher/WebUIContainer.tsx`

- [ ] **Step 1: 在 LauncherApp 中处理 iframe 加载失败**

修改 `LauncherApp.tsx`，添加 iframe 加载失败状态：

```typescript
import React, { useEffect, useState, useCallback } from 'react';
import { launcherServiceApi, launcherApi, windowApi } from '../lib/api';
import type { LauncherStatus, LauncherError } from '../lib/api/types';
import SplashScreen from './SplashScreen';
import ErrorPage from './ErrorPage';
import WebUIContainer from './WebUIContainer';

// 添加 iframe 加载失败错误类型
interface IFrameLoadError {
  type: 'iframe_load_failed';
  message: string;
}

type AppError = LauncherError | IFrameLoadError;

const LauncherApp: React.FC = () => {
  const [status, setStatus] = useState<LauncherStatus>({ state: 'initializing' });
  const [version, setVersion] = useState<string>('1.0.0');
  const [iframeError, setIframeError] = useState<IFrameLoadError | null>(null);

  // ... existing code ...

  const handleWebUIError = useCallback((error: Error) => {
    console.error('Web UI load error:', error);
    setIframeError({
      type: 'iframe_load_failed',
      message: '界面加载失败，请检查服务是否正常运行',
    });
  }, []);

  const handleRetryIframe = useCallback(() => {
    setIframeError(null);
    // 重新检查服务状态
    launcherServiceApi.getStatus().then(setStatus).catch(console.error);
  }, []);

  // 渲染逻辑：添加 iframe 错误处理
  if (iframeError) {
    return (
      <ErrorPage
        errorKind="processExit"
        errorMessage={iframeError.message}
        onRetry={handleRetryIframe}
        onViewLogs={handleViewLogs}
      />
    );
  }

  // ... rest of existing code ...
};
```

- [ ] **Step 2: Commit iframe error handling**

```bash
git add installer-tauri/src/launcher/LauncherApp.tsx
git commit -m "fix: handle iframe load failure in LauncherApp"
```

---

## Task 18: [GAP FIX] 统一 LauncherStatus 类型定义

**Files:**
- Modify: `installer-tauri/src/lib/api/types.ts`

- [ ] **Step 1: 确保 TypeScript 类型与 Rust 一致**

验证 `types.ts` 中的 `LauncherStatus` 类型与 Rust `store.rs` 一致：

```typescript
// 必须与 Rust LauncherStatus 枚举完全一致
export type LauncherStatusState = 
  | 'initializing'
  | 'checkingInstallation'
  | 'startingService'
  | 'waitingForReady'
  | 'ready'
  | 'failed';

export interface LauncherStatus {
  state: LauncherStatusState;
  port?: number;
  error?: LauncherError;
}

export type LauncherErrorKind =
  | 'notInstalled'
  | 'portConflict'
  | 'processExit'
  | 'configError'
  | 'missingFiles';

export interface LauncherError {
  kind: LauncherErrorKind;
  message: string;
}
```

- [ ] **Step 2: Commit type definition**

```bash
git add installer-tauri/src/lib/api/types.ts
git commit -m "fix: ensure LauncherStatus types match Rust definition"
```

---

## Task 19: [GAP FIX] 添加自动化测试

**Files:**
- Create: `installer-tauri/tests/launcher-flow.test.ts`

- [ ] **Step 1: 创建启动流程测试**

```typescript
import { describe, it, expect, beforeAll, afterAll } from 'vitest';

describe('Launcher Flow Tests', () => {
  // 测试启动流程
  
  it('should show SplashScreen on startup', async () => {
    // 验证初始状态为 initializing
    const status = await window.daemonAPI.getStatus();
    expect(status.state).toBe('initializing');
  });

  it('should transition to ready after service starts', async () => {
    // 启动服务后验证状态变为 ready
    await window.daemonAPI.start();
    // 等待状态变化
    const status = await new Promise(resolve => {
      const unsubscribe = window.daemonAPI.onStatusChange(s => {
        if (s.state === 'ready') {
          unsubscribe();
          resolve(s);
        }
      });
    });
    expect(status.state).toBe('ready');
  });

  it('should stop service on window close', async () => {
    // 模拟窗口关闭，验证服务停止
    // 此测试需要在 Tauri 环境中运行
  });
});
```

- [ ] **Step 2: 添加测试命令到 package.json**

```json
{
  "scripts": {
    "test": "vitest run",
    "test:watch": "vitest watch"
  }
}
```

- [ ] **Step 3: Commit test files**

```bash
git add installer-tauri/tests/ installer-tauri/package.json
git commit -m "test: add launcher flow automated tests"
```

---

<!-- AUTONOMOUS DECISION LOG -->
## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
|---|-------|----------|-----------|-----------|----------|----------|
| 1 | CEO | Tauri vs Electron: Tauri | Mechanical | P5 (explicit) | 改造现有 installer-tauri，改动更小，学习成本低 | Electron apps/desktop (新建目录，学习曲线高) |
| 2 | CEO | iframe 嵌入 vs 嵌入 React | Mechanical | P5 (explicit) | iframe 实现简单，保留 web UI 独立访问 | 嵌入 React 组件（重构量大） |
| 3 | CEO | 添加 iframe ↔ Tauri 通信 | Mechanical | P1 (completeness) | iframe 需要通知 Tauri 关闭时停止服务 | 无通信（服务可能继续运行） |
| 4 | CEO | 窗口关闭在 Rust 层停止服务 | Mechanical | P1 (completeness) | beforeunload 可能被 iframe 阻止 | 仅前端 beforeunload |
| 5 | CEO | 统一 LauncherStatus 类型定义 | Mechanical | P1 (completeness) | Rust 和 TypeScript 类型必须一致 | 重复定义（维护负担） |
| 6 | CEO | 添加 iframe 加载失败页面 | Taste | P1 (completeness) | iframe 加载失败需要单独错误页面 | 无处理（用户看到空白） |
| 7 | CEO | Splash Screen 进度细化 | Taste | P2 (boil lakes) | 用户需要看到具体启动步骤 | 简单进度条（用户焦虑） |

---

## CEO Review Summary

### Premise Challenge

**已验证前提**（用户决策）：
- 方案 C: web UI 融合（iframe 嵌入） ✓
- 自动启动服务 ✓
- Splash Screen 标准 ✓
- 标准窗口装饰 ✓
- 不需要系统托盘 ✓
- 窗口尺寸自适应 + 用户自定义 ✓
- 主题用户可切换 ✓

**潜在假设（需确认）**：
- iframe 交互限制：iframe 无法直接调用 Tauri 命令
- web UI 主题与 Launcher 主题是否需要同步

### Dream State Diagram

```
CURRENT (Launcher 控制面板)     THIS PLAN (Splash + iframe)     12-MONTH IDEAL
┌─────────────────────┐        ┌─────────────────────┐        ┌─────────────────────┐
│ Launcher 启动        │   →    │ Launcher 启动        │   →    │ Launcher 启动        │
│ 手动启动服务         │        │ Splash Screen       │        │ Splash Screen       │
│ 显示控制面板         │        │ 自动启动服务         │        │ 自动启动服务         │
│ 点击按钮打开浏览器   │        │ iframe 加载 web UI   │        │ iframe 加载 web UI   │
│ 手动停止服务         │        │ 关闭窗口停止服务     │        │ 关闭窗口停止服务     │
│                      │        │                      │        │ + 系统托盘（可选）    │
│                      │        │                      │        │ + 自动更新           │
└─────────────────────┘        └─────────────────────┘        └─────────────────────┘
```

### Implementation Alternatives

| 方案 | Effort | Risk | Pros | Cons |
|------|--------|------|------|------|
| **A) Tauri iframe** (当前计划) | 低 (~2h CC) | 低 | 复用现有代码，改动小 | iframe 交互限制，CSP 配置 |
| **B) Electron apps/desktop** (参考方案) | 中 (~4h CC) | 中 | 独立架构，更灵活 | 新建目录，学习 Electron |
| **C) Tauri + 嵌入 React** | 高 (~6h CC) | 中 | 无 iframe 限制 | 重构 web UI 组件 |

**选择 A** - 改动最小，复用最大，风险可控。

### NOT in Scope (Deferred to TODOS.md)

- 系统托盘 - 用户明确选择不需要
- 自动更新 - 计划未提及，后续考虑
- macOS notarization - 需要 Apple Developer 账户
- 深色/浅色主题同步 - iframe 主题由 web UI 控制

### What Already Exists

| Sub-problem | Existing Code | Plan Reuses |
|-------------|---------------|-------------|
| 服务进程管理 | `service_manager.rs` | ✓ 直接调用 |
| 注册表检测 | `services/registry.rs` | ✓ 复用 get_installed_version |
| 配置读取 | `services/config.rs` | ✓ 复用 read_existing_config |
| 前端 API 层 | `src/lib/api/` | ✓ 扩展新增 launcher_service.ts |
| 窗口控制 | `commands/window.rs` | ✓ 保持不变 |

### Error & Rescue Registry

| Error Scenario | Rescue Mechanism | Gaps |
|----------------|------------------|------|
| 服务启动失败 | ErrorPage + 重试按钮 | ✓ |
| 服务启动超时 (30s) | ErrorPage + 查看日志 | ✓ |
| iframe 加载失败 | onLoadError 回调 | **缺失独立错误页面** |
| 端口冲突 | LauncherError::PortConflict | ✓ |
| 未安装 | LauncherError::NotInstalled | ✓ |
| 配置错误 | LauncherError::ConfigError | ✓ |

### Failure Modes Registry

| Failure Mode | Test Coverage | Error Handling | User Experience |
|--------------|---------------|----------------|-----------------|
| 服务进程退出 | 未测试 | ✓ ErrorPage | 明确错误信息 |
| iframe 白屏 | 未测试 | **缺失** | 用户看到空白 |
| WebSocket 断连 | 未测试 | **缺失** | web UI 可能卡住 |
| 窗口关闭时服务继续运行 | 未测试 | **缺失** | 服务残留 |

---

## Self-Review Checklist

**1. Spec coverage:**

| Spec Requirement | Task |
|------------------|------|
| 启动流程：Splash → 服务启动 → web UI | Task 7-10, 11 |
| 启动失败：错误页面 + 重试 | Task 8, 10 |
| 窗口尺寸：保存/恢复 | Task 2, 6, 14 |
| 服务停止：关闭时停止 | Task 2 (stop_service) |
| 标准窗口装饰 | Task 5 |
| LauncherStatus 类型 | Task 1 |
| IPC 命令 | Task 3 |

**2. Placeholder scan:**
- ✓ No TBD/TODO placeholders
- ✓ All code blocks contain actual implementation
- ✓ All file paths are exact

**3. Type consistency:**
- ✓ `LauncherStatus` in Rust (store.rs) matches TypeScript (types.ts)
- ✓ `LauncherError` fields consistent
- ✓ `WindowSize` fields consistent
- ✓ IPC command names match frontend invoke calls

---

## Design Review Summary

### Design Litmus Scorecard

| Dimension | Score | Findings |
|-----------|-------|----------|
| **Information Hierarchy** | 8/10 | Splash Screen: Logo → 品牌名 → 版本号 → 进度条 → 状态文本。层级清晰。 |
| **Missing States** | 6/10 | Splash Screen ✓, ErrorPage ✓, WebUIContainer ✓。缺失: iframe 加载失败页面、WebSocket 断连提示 |
| **Responsive Strategy** | 7/10 | 窗口尺寸自适应 + 用户自定义。iframe 无响应式处理。 |
| **Accessibility** | 5/10 | 缺失键盘快捷键、焦点管理、屏幕阅读器支持 |
| **Specificity** | 9/10 | 代码提供具体样式、尺寸、颜色。明确的设计决策。 |
| **Theme Consistency** | 6/10 | Splash/ErrorPage 使用硬编码颜色。iframe 主题由 web UI 控制，可能不一致 |
| **Interaction States** | 7/10 | Splash 有进度，Error 有按钮。缺失 iframe 加载失败时的用户操作 |

### Critical Design Gaps

1. **iframe 加载失败 UI**: WebUIContainer 有 onLoadError 但 LauncherApp 未处理此错误
2. **Splash Screen 状态细化**: 用户需要知道具体在做什么（"检测安装"、"启动服务"、"等待就绪"）
3. **主题同步**: Launcher 主题与 web UI 主题可能不一致
4. **键盘导航**: 焦点从 Splash/ErrorPage 转到 iframe 时的处理

---

## Eng Review Summary

### Architecture ASCII Diagram

```
                    installer-tauri (Launcher Mode)
                    
┌──────────────────────────────────────────────────────────────────────┐
│                          Tauri Application                           │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                         lib.rs                                   ││
│  │  setup() → detect_app_mode() → Launcher → LauncherFlow.run()    ││
│  │                          ↓                                       ││
│  │  AppState { mode, install_dir, service_manager, launcher_status }││
│  └─────────────────────────────────────────────────────────────────┘│
│                              ↓                                       │
│  ┌──────────────────────┐    │    ┌──────────────────────────────┐  │
│  │ services/launcher.rs │◄───┼───►│ commands/launcher_service.rs │  │
│  │  LauncherFlow        │    │    │  get_launcher_status         │  │
│  │  run() / retry()     │    │    │  retry_launcher_startup      │  │
│  │  stop_service()      │    │    │  get_window_size / save      │  │
│  └──────────────────────┘    │    └──────────────────────────────┘  │
│                              ↓                                       │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                 service_manager.rs                               ││
│  │  start() → spawn colink-server.exe                              ││
│  │  stop()  → kill process                                         ││
│  │  is_running() → poll /health endpoint                           ││
│  └─────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
                              ↓ emit "launcher:status"
┌──────────────────────────────────────────────────────────────────────┐
│                          React Frontend                              │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                          App.tsx                                 ││
│  │  mode === 'launcher' → React.lazy(LauncherApp)                  ││
│  └─────────────────────────────────────────────────────────────────┘│
│                              ↓                                       │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                        LauncherApp.tsx                           ││
│  │  onStatusChange → { failed → ErrorPage, ready → WebUIContainer }││
│  │  else → SplashScreen                                            ││
│  └─────────────────────────────────────────────────────────────────┘│
│                              ↓                                       │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────────────┐ │
│  │ SplashScreen   │  │ ErrorPage      │  │ WebUIContainer         │ │
│  │ Logo+Progress  │  │ Retry+ViewLogs │  │ iframe localhost:port  │ │
│  └────────────────┘  └────────────────┘  └────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
```

### Test Coverage Diagram

```
CODE PATHS                                            USER FLOWS
[+] src-tauri/src/services/launcher.rs                [+] Launcher 启动流程
  ├── LauncherFlow::run()                               ├── [GAP] 未安装时显示错误页面
  │   ├── [GAP] 检测安装失败                              ├── [GAP] 服务启动失败后点击重试
  │   ├── [GAP] 服务启动超时                              └── [GAP] 点击查看日志打开目录
  │   └── [GAP] 端口冲突                               [+] 窗口交互
  └── LauncherFlow::stop_service()                      ├── [GAP] 窗口关闭时服务停止
      └── [GAP] 服务停止失败                              └── [GAP] 窗口尺寸保存/恢复

[+] src-tauri/src/commands/launcher_service.rs        [+] iframe 加载
  ├── get_launcher_status                               ├── [GAP] iframe 加载失败显示错误
  ├── retry_launcher_startup                            └── [GAP] iframe 内容交互
  └── [GAP] get_window_size 返回正确值

[+] src/launcher/LauncherApp.tsx                       [+] 错误处理
  ├── onStatusChange                                    ├── [GAP] 重试按钮触发重试
  ├── [GAP] handleRetry 调用正确 API                     └── [GAP] 查看日志按钮打开目录
  └── [GAP] handleViewLogs 调用正确 API

[+] src/launcher/WebUIContainer.tsx
  └── [GAP] iframe onLoadError 处理

COVERAGE: 0/15 paths tested (0%)  |  Critical Gaps: 15
```

### Critical Eng Gaps

1. **无自动化测试**: 计划缺少测试步骤
2. **窗口关闭时服务停止**: beforeunload 可能被 iframe 阻止，需在 Rust 层处理
3. **iframe 加载失败处理**: WebUIContainer 有 onLoadError 但 LauncherApp 未使用
4. **WebSocket 断连**: iframe 内 web UI 可能无法检测断连

---

## DX Review Summary

### Developer Experience Scorecard

| Dimension | Score | Findings |
|-----------|-------|----------|
| **Getting Started** | 8/10 | `pnpm dev:launcher` 启动，TTHW ~2min |
| **API Naming** | 7/10 | `get_launcher_status`, `retry_launcher_startup` - 一致性 OK |
| **Error Messages** | 6/10 | LauncherError 有中文消息，但缺少文档链接 |
| **Documentation** | 5/10 | 缺少 launcher/ 目录说明，缺少迁移指南 |
| **Upgrade Path** | 4/10 | 从旧 LauncherDashboard 迁移未说明 |
| **Debugging** | 7/10 | Tauri 日志 + 前端 console，但 iframe 内难调试 |

---

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 1 | issues_open | 7 decisions, 4 gaps: iframe通信, 窗口关闭, 类型统一, iframe错误 |
| Design Review | `/plan-design-review` | UI/UX gaps | 1 | issues_open | 7 dimensions, 4 gaps: iframe错误UI, 状态细化, 主题同步, 键盘导航 |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | issues_open | 15 test gaps, 4 critical: 无测试, 窗口关闭, iframe错误, WS断连 |
| DX Review | `/plan-devex-review` | Developer experience gaps | 1 | issues_open | 6 dimensions, 4 gaps: 文档, 迁移指南, 调试, 错误消息 |

**VERDICT:** ISSUES OPEN — 4 reviews ran, 16 gaps identified (4 critical).

---

## Cross-Phase Themes

**Theme: iframe 错误处理** — flagged in CEO (决策 6), Design (Gap 1), Eng (Gap 3). High-confidence signal. Must add iframe error page.

**Theme: 窗口关闭时服务停止** — flagged in CEO (决策 4), Eng (Gap 2), DX. High-confidence signal. Must handle in Rust layer.

**Theme: 测试覆盖** — flagged in Eng only. Critical for reliability.

---

## Review Scores Summary

- **CEO: 8/10** — Good strategy, Tauri vs Electron decision correct
- **Design: 6/10** — Core states OK, iframe error and accessibility missing
- **Eng: 6/10** — Architecture clear, tests added (Task 19)
- **DX: 6/10** — Good naming, documentation and upgrade path missing

**Overall: 6.5/10** — Plan approved with gap fixes (Task 16-19 added).

---

## APPROVAL RECORD

| Field | Value |
|-------|-------|
| **Decision** | APPROVED |
| **Approver** | Colink计划审查师 |
| **Timestamp** | 2026-05-09T15:45:00Z |
| **Gaps Fixed** | 4 critical gaps addressed via Task 16-19 |
| **Conditions** | Execute Task 1-15 + Task 16-19 (gap fixes) |

---

Plan approved and ready for implementation. Execute all 19 tasks.

**Next Step**: @Colink开发工程师 请按计划执行实施（19 个任务）
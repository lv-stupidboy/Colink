use std::sync::RwLock;
use crate::services::ServiceManager;

/// Application mode detected from exe filename
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum AppMode {
    Setup,
    Launcher,
}

/// Global application state shared across commands
pub struct AppState {
    /// Detected app mode (Setup or Launcher)
    pub mode: RwLock<AppMode>,
    /// Installation directory (set during setup or loaded for launcher)
    pub install_dir: RwLock<Option<String>>,
    /// Service manager for spawning/controlling colink-server.exe
    pub service_manager: RwLock<Option<ServiceManager>>,
}

impl AppState {
    pub fn new(mode: AppMode) -> Self {
        Self {
            mode: RwLock::new(mode),
            install_dir: RwLock::new(None),
            service_manager: RwLock::new(None),
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
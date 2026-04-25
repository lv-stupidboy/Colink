use crate::error::{InstallerError, Result};
use std::path::PathBuf;
use std::process::{Child, Command};
use std::sync::{Arc, Mutex};

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

#[cfg(target_os = "windows")]
const CREATE_NO_WINDOW: u32 = 0x08000000;

/// Service manager for spawning and controlling colink-server.exe
pub struct ServiceManager {
    process: Arc<Mutex<Option<Child>>>,
    install_dir: String,
}

impl ServiceManager {
    pub fn new(install_dir: String) -> Self {
        Self {
            process: Arc::new(Mutex::new(None)),
            install_dir,
        }
    }

    /// Start the server process
    pub fn start(&self) -> Result<()> {
        log::info!("ServiceManager::start() called, install_dir={}", self.install_dir);
        let mut process_guard = self.process.lock().unwrap();

        if process_guard.is_some() {
            log::info!("Service already running, skipping");
            return Ok(()); // Already running
        }

        let server_path = PathBuf::from(&self.install_dir).join("colink-server.exe");
        let config_path = PathBuf::from(&self.install_dir)
            .join("data")
            .join("configs")
            .join("config.yaml");

        log::info!("Server path: {:?}", server_path);
        log::info!("Config path: {:?}", config_path);

        if !server_path.exists() {
            log::error!("Server exe not found at {:?}", server_path);
            return Err(InstallerError::Process("服务程序不存在".into()));
        }

        if !config_path.exists() {
            log::error!("Config file not found at {:?}", config_path);
            return Err(InstallerError::Config("配置文件不存在".into()));
        }

        log::info!("Spawning server process...");

        #[cfg(target_os = "windows")]
        let child = Command::new(&server_path)
            .args(["-config", &config_path.to_string_lossy()])
            .current_dir(&self.install_dir)
            .creation_flags(CREATE_NO_WINDOW)
            .spawn();

        #[cfg(not(target_os = "windows"))]
        let child = Command::new(&server_path)
            .args(["-config", &config_path.to_string_lossy()])
            .current_dir(&self.install_dir)
            .spawn();

        match child {
            Ok(c) => {
                log::info!("Process spawned successfully, PID: {:?}", c.id());
                *process_guard = Some(c);
                log::info!("Service start initiated, will verify status on next check");
                Ok(())
            }
            Err(e) => {
                log::error!("Failed to spawn process: {}", e);
                Err(InstallerError::Process(e.to_string()))
            }
        }
    }

    /// Stop the server process
    pub fn stop(&self) -> Result<()> {
        let mut process_guard = self.process.lock().unwrap();

        if let Some(mut process) = process_guard.take() {
            // Try graceful kill first
            process.kill().map_err(|e| InstallerError::Process(e.to_string()))?;
        }

        // Also kill any orphan processes
        #[cfg(target_os = "windows")]
        {
            crate::services::file_ops::kill_all_processes("colink-server.exe")?;
        }

        Ok(())
    }

    /// Check if service is running by checking if API is responsive
    pub fn is_running(&self) -> bool {
        // First check if we have a process reference
        let process_running = {
            let mut process_guard = self.process.lock().unwrap();
            if let Some(p) = process_guard.as_mut() {
                match p.try_wait() {
                    Ok(None) => true, // Still running
                    _ => {
                        // Process exited, clear reference
                        *process_guard = None;
                        false
                    }
                }
            } else {
                false
            }
        };

        // If process reference says running, trust it
        if process_running {
            return true;
        }

        // Otherwise, check by trying to connect to API (quick check)
        // This handles cases where process was started elsewhere or reference lost
        let port = 26305; // Default port
        let url = format!("http://localhost:{}/api/v1/health", port);

        // Quick synchronous check with timeout
        let client = reqwest::blocking::Client::builder()
            .timeout(std::time::Duration::from_secs(1))
            .build()
            .unwrap_or_else(|_| reqwest::blocking::Client::new());

        client.get(&url).send().map(|r| r.status().is_success()).unwrap_or(false)
    }

    /// Get service status as string
    pub fn get_status(&self) -> String {
        if self.is_running() {
            "running".to_string()
        } else {
            "stopped".to_string()
        }
    }

    /// Get running agent instances from backend API
    pub async fn get_running_agents(&self, port: u16) -> Result<Vec<RunningAgentInstance>> {
        let url = format!("http://localhost:{}{}", port, "/api/v1/invocations/running");

        let client = reqwest::Client::new();
        let response = client
            .get(&url)
            .timeout(std::time::Duration::from_secs(5))
            .send()
            .await;

        match response {
            Ok(res) => {
                if res.status().is_success() {
                    let instances: Vec<RunningAgentInstance> = res.json().await?;
                    Ok(instances)
                } else {
                    Ok(vec![])
                }
            }
            Err(_) => Ok(vec![]),
        }
    }
}

/// Running agent instance info
#[derive(Debug, Clone, serde::Deserialize, serde::Serialize)]
pub struct RunningAgentInstance {
    #[serde(rename = "invocationId")]
    pub invocation_id: String,
    #[serde(rename = "agentName")]
    pub agent_name: String,
    #[serde(rename = "projectName")]
    pub project_name: String,
    #[serde(rename = "threadTitle")]
    pub thread_title: String,
    #[serde(rename = "startedAt")]
    pub started_at: String,
    #[serde(rename = "runningDurationSeconds")]
    pub running_duration_seconds: u64,
}
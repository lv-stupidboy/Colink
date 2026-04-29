use crate::error::{InstallerError, Result};
use crate::services::config::read_existing_config;
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

    /// Check if a port is in use and get the PID of the process using it
    #[cfg(target_os = "windows")]
    fn check_port_in_use(port: u16) -> Result<Option<u32>> {
        // Use netstat to find process using the port
        let output = Command::new("netstat")
            .args(["-ano", "-p", "TCP"])
            .creation_flags(CREATE_NO_WINDOW)
            .output()
            .map_err(|e| InstallerError::Process(e.to_string()))?;

        let stdout = String::from_utf8_lossy(&output.stdout);

        // Look for lines with the port (e.g., "127.0.0.1:26305" or "0.0.0.0:26305")
        for line in stdout.lines() {
            // Check for LISTENING state on the port
            if line.contains("LISTENING") && line.contains(&format!(":{} ", port)) {
                // Extract PID (last column)
                let parts: Vec<&str> = line.split_whitespace().collect();
                if let Some(pid_str) = parts.last() {
                    if let Ok(pid) = pid_str.parse::<u32>() {
                        log::info!("Port {} is in use by PID {}", port, pid);
                        return Ok(Some(pid));
                    }
                }
            }
        }

        Ok(None)
    }

    #[cfg(not(target_os = "windows"))]
    fn check_port_in_use(port: u16) -> Result<Option<u32>> {
        // On non-Windows, use lsof or ss
        let output = Command::new("ss")
            .args(["-tlnp", &format!("sport = :{}", port)])
            .output();

        match output {
            Ok(o) => {
                let stdout = String::from_utf8_lossy(&o.stdout);
                for line in stdout.lines() {
                    if line.contains(&format!(":{}", port)) {
                        // Extract PID from ss output (format varies)
                        // For now, return None as we can't reliably parse
                        log::warn!("Port {} appears to be in use on non-Windows", port);
                        return Ok(None);
                    }
                }
            }
            Err(_) => {
                // ss not available, try lsof
                let lsof_output = Command::new("lsof")
                    .args(["-i", &format!(":{}", port)])
                    .output();

                if let Ok(o) = lsof_output {
                    if !o.stdout.is_empty() {
                        log::warn!("Port {} appears to be in use (lsof)", port);
                        return Ok(None);
                    }
                }
            }
        }
        Ok(None)
    }

    /// Kill a process by PID
    #[cfg(target_os = "windows")]
    fn kill_process_by_pid(pid: u32) -> Result<()> {
        log::info!("Attempting to kill process with PID {}", pid);

        let output = Command::new("taskkill")
            .args(["/F", "/PID", &pid.to_string()])
            .creation_flags(CREATE_NO_WINDOW)
            .output()
            .map_err(|e| InstallerError::Process(e.to_string()))?;

        if output.status.success() {
            log::info!("Successfully killed process PID {}", pid);
            Ok(())
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr);
            log::warn!("Failed to kill process PID {}: {}", pid, stderr);
            Err(InstallerError::Process(format!(
                "无法终止占用端口的进程 (PID {}): {}",
                pid, stderr
            )))
        }
    }

    #[cfg(not(target_os = "windows"))]
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
                "无法终止占用端口的进程 (PID {})",
                pid
            )))
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

        // Read port from config
        let port = read_existing_config(&self.install_dir)
            .map(|(server_port, _)| server_port)
            .unwrap_or(26305);

        log::info!("Checking if port {} is in use...", port);

        // Check if port is in use and handle it
        if let Some(pid) = Self::check_port_in_use(port)? {
            log::warn!("Port {} is occupied by PID {}, attempting to kill...", port, pid);

            // Try to kill the process
            match Self::kill_process_by_pid(pid) {
                Ok(_) => {
                    log::info!("Successfully freed port {}", port);
                    // Wait a moment for the port to be released
                    std::thread::sleep(std::time::Duration::from_millis(500));
                }
                Err(e) => {
                    log::error!("Failed to kill process occupying port {}: {}", port, e);
                    return Err(InstallerError::Process(format!(
                        "端口 {} 已被其他程序占用 (PID {})，无法终止该进程。请手动关闭占用端口的程序后重试。",
                        port, pid
                    )));
                }
            }
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
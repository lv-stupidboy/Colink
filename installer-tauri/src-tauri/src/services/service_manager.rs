use crate::error::{InstallerError, Result};
use crate::services::config::read_existing_config;
use std::io::Read;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
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
        log::info!("=== ServiceManager::start() ===");
        log::info!("Install directory: {}", self.install_dir);

        let mut process_guard = self.process.lock().unwrap();

        if process_guard.is_some() {
            log::info!("Service already running (process reference exists), skipping");
            return Ok(()); // Already running
        }

        #[cfg(target_os = "windows")]
        let server_path = PathBuf::from(&self.install_dir).join("colink-server.exe");

        #[cfg(target_os = "macos")]
        let server_path = PathBuf::from(&self.install_dir)
            .join("Contents/MacOS/colink-server");  // CRITICAL-02: no .exe extension

        #[cfg(target_os = "windows")]
        let config_path = PathBuf::from(&self.install_dir)
            .join("data")
            .join("configs")
            .join("config.yaml");

        #[cfg(target_os = "macos")]
        let config_path = {
            // CRITICAL-01: config file is in ~/Library/Application Support/Colink/
            let data_dir = dirs::data_dir()
                .unwrap_or_else(|| PathBuf::from("~/.local/share"))
                .join("Colink");
            data_dir.join("configs/config.yaml")
        };

        log::info!("Server exe path: {}", server_path.display());
        log::info!("Config file path: {}", config_path.display());

        // Check server exe exists
        if !server_path.exists() {
            log::error!("Server exe NOT FOUND at: {}", server_path.display());
            log::error!("Directory contents:");
            if let Ok(entries) = std::fs::read_dir(&self.install_dir) {
                for entry in entries.flatten() {
                    log::error!("  - {}", entry.file_name().to_string_lossy());
                }
            }
            return Err(InstallerError::Process(format!(
                "服务程序不存在: {}",
                server_path.display()
            )));
        }

        // Check config file exists
        #[cfg(target_os = "windows")]
        {
            if !config_path.exists() {
                log::error!("Config file NOT FOUND at: {}", config_path.display());
                log::error!("data/configs directory contents:");
                let configs_dir = PathBuf::from(&self.install_dir).join("data").join("configs");
                if configs_dir.exists() {
                    if let Ok(entries) = std::fs::read_dir(&configs_dir) {
                        for entry in entries.flatten() {
                            log::error!("  - {}", entry.file_name().to_string_lossy());
                        }
                    }
                } else {
                    log::error!("  configs directory does not exist");
                }
                return Err(InstallerError::Config(format!(
                    "配置文件不存在: {}",
                    config_path.display()
                )));
            }
        }

        #[cfg(target_os = "macos")]
        {
            // CRITICAL-01: Mac first run - auto create data directory and config
            if !config_path.exists() {
                log::info!("Config file not found, initializing data directory for first run...");

                // Create data directory structure
                let data_dir = dirs::data_dir()
                    .unwrap_or_else(|| PathBuf::from("~/.local/share"))
                    .join("Colink");

                std::fs::create_dir_all(data_dir.join("configs")).map_err(|e| InstallerError::Io {
                    context: "create configs directory".to_string(),
                    source: e,
                })?;
                std::fs::create_dir_all(data_dir.join("logs")).map_err(|e| InstallerError::Io {
                    context: "create logs directory".to_string(),
                    source: e,
                })?;
                std::fs::create_dir_all(data_dir.join("sqlite")).map_err(|e| InstallerError::Io {
                    context: "create sqlite directory".to_string(),
                    source: e,
                })?;
                std::fs::create_dir_all(data_dir.join("agent-assets")).map_err(|e| InstallerError::Io {
                    context: "create agent-assets directory".to_string(),
                    source: e,
                })?;
                std::fs::create_dir_all(data_dir.join("agent-configs")).map_err(|e| InstallerError::Io {
                    context: "create agent-configs directory".to_string(),
                    source: e,
                })?;
                std::fs::create_dir_all(data_dir.join("repos")).map_err(|e| InstallerError::Io {
                    context: "create repos directory".to_string(),
                    source: e,
                })?;

                // Copy config.yaml.example from Resources
                let example_path = PathBuf::from(&self.install_dir)
                    .join("Contents/Resources/config.yaml.example");

                if example_path.exists() {
                    std::fs::copy(&example_path, &config_path).map_err(|e| InstallerError::Io {
                        context: "copy config.yaml.example".to_string(),
                        source: e,
                    })?;
                    log::info!("Config file created from template: {}", config_path.display());
                } else {
                    log::warn!("config.yaml.example not found in Resources, creating default config");
                    // Create minimal default config
                    let default_config = "server:\n  port: 26305\ndatabase:\n  type: sqlite\n";
                    std::fs::write(&config_path, default_config).map_err(|e| InstallerError::Io {
                        context: "write default config".to_string(),
                        source: e,
                    })?;
                }
            }
        }

        // Read port from config
        let port = read_existing_config(&self.install_dir)
            .map(|(server_port, _)| server_port)
            .unwrap_or(26305);

        log::info!("Server port from config: {}", port);

        // Check if port is in use and handle it
        if let Some(pid) = Self::check_port_in_use(port)? {
            log::warn!("Port {} is occupied by PID {}", port, pid);

            // Try to kill the process
            match Self::kill_process_by_pid(pid) {
                Ok(_) => {
                    log::info!("Successfully killed process PID {}, waiting 500ms for port release", pid);
                    std::thread::sleep(std::time::Duration::from_millis(500));
                }
                Err(e) => {
                    log::error!("Failed to kill process PID {}: {}", pid, e);
                    return Err(InstallerError::Process(format!(
                        "端口 {} 已被其他程序占用 (PID {})，无法终止该进程。请手动关闭占用端口的程序后重试。",
                        port, pid
                    )));
                }
            }
        } else {
            log::info!("Port {} is available", port);
        }

        log::info!("Spawning server process...");
        log::info!("Command: {} -config {}", server_path.display(), config_path.display());
        log::info!("Working directory: {}", self.install_dir);

        #[cfg(target_os = "windows")]
        let child = Command::new(&server_path)
            .args(["-config", &config_path.to_string_lossy()])
            .current_dir(&self.install_dir)
            .creation_flags(CREATE_NO_WINDOW)
            .stdout(Stdio::null())
            .stderr(Stdio::piped())
            .spawn();

        #[cfg(not(target_os = "windows"))]
        let child = Command::new(&server_path)
            .args(["-config", &config_path.to_string_lossy()])
            .current_dir(&self.install_dir)
            .stdout(Stdio::null())
            .stderr(Stdio::piped())
            .spawn();

        match child {
            Ok(c) => {
                let pid = c.id();
                log::info!("Process spawned SUCCESSFULLY, PID: {}", pid);
                *process_guard = Some(c);

                // Wait a moment and check if process is still running
                std::thread::sleep(std::time::Duration::from_millis(1000));

                // Verify process didn't immediately exit
                if let Some(ref mut proc) = *process_guard {
                    match proc.try_wait() {
                        Ok(None) => {
                            log::info!("Process is still running after 1s (PID: {})", pid);
                        }
                        Ok(Some(status)) => {
                            log::error!("Process EXITED immediately after spawn! Status: {:?}", status);
                            // Capture stderr to get the actual error message
                            let stderr_output = if let Some(mut stderr) = proc.stderr.take() {
                                let mut stderr_buf = String::new();
                                if stderr.read_to_string(&mut stderr_buf).is_ok() {
                                    stderr_buf
                                } else {
                                    "Unable to read stderr".to_string()
                                }
                            } else {
                                "No stderr available".to_string()
                            };
                            log::error!("Server stderr output: {}", stderr_output);
                            *process_guard = None;
                            return Err(InstallerError::Process(format!(
                                "服务进程启动后立即退出 (PID {}, status: {})\n错误输出: {}",
                                pid, status, stderr_output
                            )));
                        }
                        Err(e) => {
                            log::warn!("Could not check process status: {}", e);
                        }
                    }
                }

                log::info!("=== Service start completed ===");
                Ok(())
            }
            Err(e) => {
                log::error!("FAILED to spawn process: {}", e);
                log::error!("Error kind: {:?}", e.kind());

                // Provide more context
                let error_msg = match e.kind() {
                    std::io::ErrorKind::NotFound => "找不到可执行文件",
                    std::io::ErrorKind::PermissionDenied => "权限不足",
                    std::io::ErrorKind::InvalidInput => "无效的命令参数",
                    _ => &e.to_string(),
                };

                Err(InstallerError::Process(format!(
                    "启动服务失败: {} ({})",
                    error_msg, server_path.display()
                )))
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

        #[cfg(target_os = "macos")]
        {
            // CRITICAL-02: Mac process name without .exe
            crate::services::file_ops::kill_all_processes("colink-server")?;
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
        // Read port from config file
        let port = read_existing_config(&self.install_dir)
            .map(|(server_port, _)| server_port)
            .unwrap_or(26305);
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
                    // API returns { "instances": [...] }, need to deserialize the wrapper
                    let wrapper: RunningAgentsResponse = res.json().await?;
                    Ok(wrapper.instances)
                } else {
                    Ok(vec![])
                }
            }
            Err(_) => Ok(vec![]),
        }
    }
}

/// Wrapper for API response { "instances": [...] }
#[derive(Debug, Clone, serde::Deserialize)]
struct RunningAgentsResponse {
    instances: Vec<RunningAgentInstance>,
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
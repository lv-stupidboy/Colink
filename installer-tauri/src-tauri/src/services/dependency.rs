use crate::error::{InstallerError, Result};
use serde::{Deserialize, Serialize};
use std::process::{Command, Stdio};
use std::time::Duration;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

#[cfg(target_os = "windows")]
const CREATE_NO_WINDOW: u32 = 0x08000000;

const DETECT_TIMEOUT_SECS: u64 = 10;

/// Dependency info
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DependencyInfo {
    pub key: String,
    pub name: String,
    pub installed: bool,
    pub version: Option<String>,
    /// 检测失败时的诊断信息（PATH 命中路径、exit code、stderr 等）。
    /// 安装/未安装两种"正常"结果不填这个字段。
    #[serde(skip_serializing_if = "Option::is_none")]
    pub detect_error: Option<String>,
    /// 下载页面 URL（仅 code-agent 等只能手动下载的依赖）。
    /// 该字段非空时 UI 显示"前往下载"按钮，点击在外部浏览器打开。
    #[serde(skip_serializing_if = "Option::is_none")]
    pub download_url: Option<String>,
}

/// Check if a dependency is installed
pub fn check_dependency(key: &str) -> DependencyInfo {
    let (name, tool) = match key {
        "node" => ("Node.js", "node"),
        "git" => ("Git", "git"),
        "claude" => ("Claude CLI", "claude"),
        "opencode" => ("OpenCode", "opencode"),
        "code-agent" => ("CodeAgent", "nga"),
        "claude-acp" => ("Claude ACP", "claude-agent-acp"),
        _ => ("Unknown", key),
    };

    let (version, detect_error) = if key == "claude-acp" {
        match check_npm_package("@agentclientprotocol/claude-agent-acp") {
            (v @ Some(_), _) => (v, None),
            (None, err) => (None, err),
        }
    } else {
        match probe_tool_version(tool) {
            (v @ Some(_), _) => (v, None),
            (None, err) => (None, err),
        }
    };

    DependencyInfo {
        key: key.to_string(),
        name: name.to_string(),
        installed: version.is_some(),
        version,
        detect_error,
        download_url: download_url_for(key),
    }
}

/// 对于只能从内网下载页手动安装的依赖（目前仅 code-agent），
/// 从 config.yaml.example 读取 download_url。模板里没配置 / 读不到时返回 None。
fn download_url_for(key: &str) -> Option<String> {
    match key {
        "code-agent" => {
            let url = read_code_agent_template().ok()?.download_url;
            if url.trim().is_empty() {
                None
            } else {
                Some(url)
            }
        }
        _ => None,
    }
}

/// Check all dependencies (only agents)
pub fn check_all_dependencies() -> Vec<DependencyInfo> {
    let keys = ["claude", "opencode", "code-agent"];
    keys.iter().map(|k| check_dependency(k)).collect()
}

/// 探测 tool 的版本号，并在失败时返回诊断字符串。
///
/// 流程：
///   1. 用补齐后的 PATH 跑 `where <tool>`（Windows）/ `command -v <tool>`（unix）
///      拿到全部候选路径
///   2. 逐个候选路径执行 `<full-path> --version`，第一个成功的即返回
///   3. 全部失败时返回包含尝试过的路径、exit code、stderr 摘要的诊断字符串
fn probe_tool_version(tool: &str) -> (Option<String>, Option<String>) {
    let augmented_path = augmented_path_env();

    let mut diagnostics: Vec<String> = Vec::new();

    // 1) where / command -v 拿候选
    let candidates = locate_candidates(tool, &augmented_path);
    if candidates.is_empty() {
        diagnostics.push(format!(
            "`{}` not found via PATH lookup (augmented PATH searched)",
            tool
        ));
    }

    // 2) 逐个尝试
    for candidate in &candidates {
        match run_version_command(candidate, &augmented_path) {
            VersionProbe::Ok(v) => {
                log::info!(
                    "[dependency] detected {} via {} -> {}",
                    tool,
                    candidate,
                    v
                );
                return (Some(v), None);
            }
            VersionProbe::Err(detail) => {
                log::warn!(
                    "[dependency] {} candidate `{}` failed: {}",
                    tool,
                    candidate,
                    detail
                );
                diagnostics.push(format!("{}: {}", candidate, detail));
            }
        }
    }

    // 3) 兜底：直接靠 PATH 跑一次（不走 where；某些用户的 where 输出可能被 doskey/alias 干扰）
    match run_version_command(tool, &augmented_path) {
        VersionProbe::Ok(v) => {
            log::info!("[dependency] detected {} via PATH lookup -> {}", tool, v);
            return (Some(v), None);
        }
        VersionProbe::Err(detail) => {
            diagnostics.push(format!("(PATH lookup) {}", detail));
        }
    }

    let diag = if diagnostics.is_empty() {
        format!("{} detection failed with no diagnostics", tool)
    } else {
        diagnostics.join(" | ")
    };
    log::error!("[dependency] {} detection failed: {}", tool, diag);
    (None, Some(diag))
}

enum VersionProbe {
    Ok(String),
    Err(String),
}

fn run_version_command(executable: &str, augmented_path: &str) -> VersionProbe {
    // 在 Windows 上保留 `cmd /C` 以正确解析 .cmd / .ps1 shim。
    // 直接 `Command::new("foo.cmd")` 也行，但当 executable 不带后缀时可能 ENOENT。
    #[cfg(target_os = "windows")]
    let mut cmd = {
        let mut c = Command::new("cmd");
        c.args(["/C", executable, "--version"])
            .creation_flags(CREATE_NO_WINDOW);
        c
    };

    #[cfg(not(target_os = "windows"))]
    let mut cmd = {
        let mut c = Command::new(executable);
        c.arg("--version");
        c
    };

    cmd.env("PATH", augmented_path)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    match run_with_timeout(&mut cmd, Duration::from_secs(DETECT_TIMEOUT_SECS)) {
        Ok(Some(output)) => {
            let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
            let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
            if output.status.success() && !stdout.is_empty() {
                return VersionProbe::Ok(stdout);
            }
            // 有些 CLI 把版本输出写到 stderr 也算成功（例如某些 java 风格工具）
            if output.status.success() && !stderr.is_empty() {
                return VersionProbe::Ok(stderr);
            }
            VersionProbe::Err(format!(
                "exit={:?} stdout={:?} stderr={:?}",
                output.status.code(),
                truncate(&stdout, 200),
                truncate(&stderr, 200)
            ))
        }
        Ok(None) => VersionProbe::Err(format!("timeout after {}s", DETECT_TIMEOUT_SECS)),
        Err(e) => VersionProbe::Err(format!("spawn error: {}", e)),
    }
}

fn locate_candidates(tool: &str, augmented_path: &str) -> Vec<String> {
    #[cfg(target_os = "windows")]
    {
        let mut cmd = Command::new("cmd");
        cmd.args(["/C", "where", tool])
            .env("PATH", augmented_path)
            .creation_flags(CREATE_NO_WINDOW)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        match run_with_timeout(&mut cmd, Duration::from_secs(5)) {
            Ok(Some(output)) if output.status.success() => String::from_utf8_lossy(&output.stdout)
                .lines()
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty())
                .collect(),
            _ => Vec::new(),
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        let mut cmd = Command::new("sh");
        cmd.args(["-c", &format!("command -v {}", tool)])
            .env("PATH", augmented_path)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        match run_with_timeout(&mut cmd, Duration::from_secs(5)) {
            Ok(Some(output)) if output.status.success() => String::from_utf8_lossy(&output.stdout)
                .lines()
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty())
                .collect(),
            _ => Vec::new(),
        }
    }
}

/// 把 npm 全局目录、Node 安装目录、nvm/fnm shim 目录追加到 PATH。
/// 一次构造，多次复用。
fn augmented_path_env() -> String {
    let existing = std::env::var("PATH").unwrap_or_default();
    let mut extras: Vec<String> = Vec::new();

    #[cfg(target_os = "windows")]
    {
        // 1) npm 全局：%APPDATA%\npm
        if let Some(appdata) = std::env::var_os("APPDATA") {
            let p = std::path::PathBuf::from(appdata).join("npm");
            if p.is_dir() {
                extras.push(p.to_string_lossy().to_string());
            }
        }
        // 2) %LOCALAPPDATA%\npm 以及 LocalAppData 下的 npm cache
        if let Some(local) = std::env::var_os("LOCALAPPDATA") {
            let p = std::path::PathBuf::from(&local).join("npm");
            if p.is_dir() {
                extras.push(p.to_string_lossy().to_string());
            }
            // fnm shims
            let fnm = std::path::PathBuf::from(&local).join("fnm_multishells");
            if fnm.is_dir() {
                if let Ok(entries) = std::fs::read_dir(&fnm) {
                    for e in entries.flatten() {
                        let p = e.path();
                        if p.is_dir() {
                            extras.push(p.to_string_lossy().to_string());
                        }
                    }
                }
            }
        }
        // 3) Program Files\nodejs
        if let Some(pf) = std::env::var_os("ProgramFiles") {
            let p = std::path::PathBuf::from(pf).join("nodejs");
            if p.is_dir() {
                extras.push(p.to_string_lossy().to_string());
            }
        }
        if let Some(pf86) = std::env::var_os("ProgramFiles(x86)") {
            let p = std::path::PathBuf::from(pf86).join("nodejs");
            if p.is_dir() {
                extras.push(p.to_string_lossy().to_string());
            }
        }
        // 4) nvm-windows: 当前激活的 node 版本目录
        //    NVM_SYMLINK 通常指向 ...\nodejs 当前激活版本
        if let Some(symlink) = std::env::var_os("NVM_SYMLINK") {
            let p = std::path::PathBuf::from(symlink);
            if p.is_dir() {
                extras.push(p.to_string_lossy().to_string());
            }
        }
        if let Some(home) = std::env::var_os("NVM_HOME") {
            let p = std::path::PathBuf::from(home);
            if p.is_dir() {
                extras.push(p.to_string_lossy().to_string());
            }
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        if let Some(home) = std::env::var_os("HOME") {
            let home = std::path::PathBuf::from(home);
            // 常见全局 npm 前缀
            for sub in [".npm-global/bin", ".npm-packages/bin", ".yarn/bin"] {
                let p = home.join(sub);
                if p.is_dir() {
                    extras.push(p.to_string_lossy().to_string());
                }
            }
            // nvm: ~/.nvm/versions/node/*/bin
            let nvm_root = home.join(".nvm/versions/node");
            if let Ok(entries) = std::fs::read_dir(&nvm_root) {
                for e in entries.flatten() {
                    let bin = e.path().join("bin");
                    if bin.is_dir() {
                        extras.push(bin.to_string_lossy().to_string());
                    }
                }
            }
            // fnm: ~/.local/share/fnm/aliases/default/bin 或 ~/.fnm/...
            for fnm_root in [".local/share/fnm/node-versions", ".fnm/node-versions"] {
                if let Ok(entries) = std::fs::read_dir(home.join(fnm_root)) {
                    for e in entries.flatten() {
                        let bin = e.path().join("installation/bin");
                        if bin.is_dir() {
                            extras.push(bin.to_string_lossy().to_string());
                        }
                    }
                }
            }
        }
        // Homebrew
        for p in ["/opt/homebrew/bin", "/usr/local/bin"] {
            if std::path::Path::new(p).is_dir() {
                extras.push(p.to_string());
            }
        }
    }

    #[cfg(target_os = "windows")]
    let sep = ";";
    #[cfg(not(target_os = "windows"))]
    let sep = ":";

    // 去重并把 extras 放在前面（优先命中我们补齐的目录）
    let mut seen: std::collections::HashSet<String> = std::collections::HashSet::new();
    let mut parts: Vec<String> = Vec::new();
    for p in extras
        .into_iter()
        .chain(existing.split(sep).map(|s| s.to_string()))
    {
        if p.is_empty() {
            continue;
        }
        let key = p.to_lowercase();
        if seen.insert(key) {
            parts.push(p);
        }
    }
    parts.join(sep)
}

/// 跑命令并带超时。返回 Ok(Some(output)) 表示完成、Ok(None) 表示超时。
fn run_with_timeout(
    cmd: &mut Command,
    timeout: Duration,
) -> std::io::Result<Option<std::process::Output>> {
    let child = cmd.spawn()?;
    let pid = child.id();

    // 看门狗：超时后强杀进程树（cmd.exe + 子进程）
    let (tx, rx) = std::sync::mpsc::channel::<()>();
    let timeout_thread = std::thread::spawn(move || {
        if rx.recv_timeout(timeout).is_err() {
            // 主线程没在超时前发信号 → 触发 kill
            #[cfg(target_os = "windows")]
            {
                let _ = Command::new("taskkill")
                    .args(["/F", "/T", "/PID", &pid.to_string()])
                    .creation_flags(CREATE_NO_WINDOW)
                    .output();
            }
            #[cfg(not(target_os = "windows"))]
            {
                let _ = Command::new("kill")
                    .args(["-9", &pid.to_string()])
                    .output();
            }
            true // killed by timeout
        } else {
            false
        }
    });

    let output = child.wait_with_output();
    // 通知看门狗：进程已退出，不必再杀
    let _ = tx.send(());
    let killed = timeout_thread.join().unwrap_or(false);

    match output {
        Ok(o) => {
            if killed {
                Ok(None)
            } else {
                Ok(Some(o))
            }
        }
        Err(e) => Err(e),
    }
}

fn truncate(s: &str, max: usize) -> String {
    if s.chars().count() <= max {
        s.to_string()
    } else {
        let mut out: String = s.chars().take(max).collect();
        out.push_str("...");
        out
    }
}

/// Check if npm package is installed globally.
/// 返回 (version, detect_error)。
fn check_npm_package(package: &str) -> (Option<String>, Option<String>) {
    let augmented_path = augmented_path_env();

    #[cfg(target_os = "windows")]
    let mut cmd = {
        let mut c = Command::new("cmd");
        c.args(["/C", &format!("npm list -g {} --depth=0", package)])
            .creation_flags(CREATE_NO_WINDOW);
        c
    };

    #[cfg(not(target_os = "windows"))]
    let mut cmd = {
        let mut c = Command::new("npm");
        c.args(["list", "-g", package, "--depth=0"]);
        c
    };

    cmd.env("PATH", &augmented_path)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    match run_with_timeout(&mut cmd, Duration::from_secs(DETECT_TIMEOUT_SECS)) {
        Ok(Some(output)) => {
            let stdout = String::from_utf8_lossy(&output.stdout).to_string();
            let stderr = String::from_utf8_lossy(&output.stderr).to_string();
            if stdout.contains(package) {
                for line in stdout.lines() {
                    if line.contains(package) && line.contains('@') {
                        if let Some(pos) = line.rfind('@') {
                            let version_part = &line[pos + 1..];
                            if let Some(v) = version_part.split_whitespace().next() {
                                return (Some(v.to_string()), None);
                            }
                        }
                    }
                }
                return (Some("installed".to_string()), None);
            }
            (
                None,
                Some(format!(
                    "npm list did not list package; exit={:?} stderr={}",
                    output.status.code(),
                    truncate(stderr.trim(), 200)
                )),
            )
        }
        Ok(None) => (
            None,
            Some(format!("npm list timeout after {}s", DETECT_TIMEOUT_SECS)),
        ),
        Err(e) => (None, Some(format!("npm spawn error: {}", e))),
    }
}

/// Install a dependency (npm package globally).
/// 注：code-agent 不支持自动安装 —— 仅在内网下载页提供安装包，
/// 调用方应改为打开 `DependencyInfo.download_url` 而非调这个 IPC。
pub fn install_dependency(key: &str) -> Result<()> {
    #[cfg(target_os = "windows")]
    {
        let package = match key {
            "claude" => "@anthropic-ai/claude-code",
            "opencode" => "opencode",
            "claude-acp" => "@agentclientprotocol/claude-agent-acp",
            "code-agent" => {
                return Err(InstallerError::Config(
                    "CodeAgent 仅支持从内网下载页手动安装，请使用下载链接".into(),
                ));
            }
            _ => return Err(InstallerError::DependencyNotFound(key.to_string())),
        };

        let output = Command::new("cmd")
            .args(["/C", &format!("npm install -g {}", package)])
            .env("PATH", augmented_path_env())
            .creation_flags(CREATE_NO_WINDOW)
            .output();

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

    #[cfg(not(target_os = "windows"))]
    {
        let _ = key;
        Ok(())
    }
}

/// 从 config.yaml.example 模板中反序列化的 code_agent 配置段。
#[derive(Debug, Default, Deserialize)]
struct CodeAgentTemplate {
    #[serde(default)]
    download_url: String,
}

/// 解析 config.yaml.example，提取 code_agent 配置。
fn read_code_agent_template() -> Result<CodeAgentTemplate> {
    let template = find_config_template()
        .ok_or_else(|| InstallerError::Config("找不到 config.yaml.example 模板文件".into()))?;

    let content = std::fs::read_to_string(&template)
        .map_err(|e| InstallerError::Io {
            context: format!("读取 config.yaml.example ({:?})", template),
            source: e,
        })?;

    // 用 serde_yaml 从完整配置中只提取 code_agent 段
    #[derive(Debug, Deserialize)]
    struct RootYaml {
        #[serde(default)]
        code_agent: CodeAgentTemplate,
    }

    let root: RootYaml = serde_yaml::from_str(&content)
        .map_err(|e| InstallerError::YamlParse(format!("解析 config.yaml.example 失败: {}", e)))?;

    log::info!(
        "[dependency] read code_agent config from template: download_url={:?}",
        root.code_agent.download_url
    );

    Ok(root.code_agent)
}

/// 遍历候选路径，找到 config.yaml.example 模板文件。
fn find_config_template() -> Option<std::path::PathBuf> {
    let exe_path = std::env::current_exe().ok()?;
    let exe_dir = exe_path.parent()?;

    let resource_path = |p: &str| std::path::PathBuf::from(p);

    // 与 installer.rs 保持一致的候选路径
    let candidates: Vec<std::path::PathBuf> = vec![
        // dev / staging 模式：可执行文件同目录
        exe_dir.join("config.yaml.example"),
        // 打包后：exe_dir/resources/
        exe_dir.join("resources/config.yaml.example"),
        // ZIP 打包：exe 在 exe/ 子目录下
        exe_dir.join("..").join("resources/config.yaml.example"),
        // macOS app bundle
        exe_dir.join("..").join("Resources/config.yaml.example"),
        // Tauri resource path
        resource_path("resources/config.yaml.example"),
        resource_path("configs/config.yaml.example"),
        // 开发模式：从 isdp 项目根目录
        exe_dir.join("..").join("..").join("configs/config.yaml.example"),
    ];

    for p in &candidates {
        if p.exists() {
            log::info!("[dependency] found config template at: {:?}", p);
            return Some(p.clone());
        }
    }

    log::warn!("[dependency] config.yaml.example not found at any candidate path");
    None
}
pub fn uninstall_dependency(key: &str) -> Result<()> {
    #[cfg(target_os = "windows")]
    {
        let package = match key {
            "claude" => "@anthropic-ai/claude-code",
            "opencode" => "opencode",
            "claude-acp" => "@agentclientprotocol/claude-agent-acp",
            _ => return Err(InstallerError::DependencyNotFound(key.to_string())),
        };

        let output = Command::new("cmd")
            .args(["/C", &format!("npm uninstall -g {}", package)])
            .env("PATH", augmented_path_env())
            .creation_flags(CREATE_NO_WINDOW)
            .output();

        match output {
            Ok(o) => {
                if !o.status.success() {
                    log::warn!(
                        "Failed to uninstall {}: {}",
                        key,
                        String::from_utf8_lossy(&o.stderr)
                    );
                }
                Ok(())
            }
            Err(e) => Err(InstallerError::Process(e.to_string())),
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        let _ = key;
        Ok(())
    }
}

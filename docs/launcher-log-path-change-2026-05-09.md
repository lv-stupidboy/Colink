---
name: launcher-log-path-change
description: 将 Launcher 日志路径从 %APPDATA% 改为安装目录下的 data/logs/
type: project
---

# Launcher 日志路径改造

**变更内容**：将 Launcher 模式的日志输出位置从 `%APPDATA%\Colink\logs\launcher.log` 改为 `{install_dir}/data/logs/launcher.log`。

**Why**：用户反馈日志文件在默认位置 `%APPDATA%\Colink\logs\` 不存在，希望能直接在安装目录查看日志。Colink 的数据目录结构是 `{install_dir}/data/`（包含 configs、sqlite、logs 等），日志统一放在此处更符合项目约定。

**How to apply**：开发工程师修改 `lib.rs` 中的日志配置逻辑，在 Launcher 模式启动时动态获取安装目录并设置日志路径。

---

## 实施方案

### 核心逻辑

```
启动流程：
1. detect_app_mode() → 判断是 Launcher 还是 Setup
2. 如果 Launcher → get_installed_version() → 获取 install_dir
3. 创建 {install_dir}/data/logs/ 目录
4. 配置日志插件使用 Folder target
5. 启动 Tauri 应用
```

### 修改文件

**`src-tauri/src/lib.rs`**：

```rust
pub fn run() {
    let mode = detect_app_mode();
    
    // Determine log directory based on mode
    let log_targets = match mode {
        AppMode::Launcher => {
            // Try to get install directory from registry
            if let Ok(installed) = services::registry::get_installed_version() {
                if let Some(install_dir) = installed.install_dir {
                    let log_dir = std::path::PathBuf::from(&install_dir).join("data").join("logs");
                    // Create directory if not exists
                    if let Err(e) = std::fs::create_dir_all(&log_dir) {
                        log::warn!("Failed to create log directory: {}", e);
                    }
                    vec![
                        tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Stdout),
                        tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Webview),
                        tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Folder {
                            path: log_dir,
                            file_name: Some("launcher.log".into()),
                        }),
                    ]
                } else {
                    // Fallback to default LogDir
                    default_log_targets()
                }
            } else {
                default_log_targets()
            }
        }
        AppMode::Setup => {
            // Setup mode uses default LogDir
            default_log_targets()
        }
    };
    
    let log_builder = tauri_plugin_log::Builder::new();
    for target in log_targets {
        log_builder.target(target);
    }
    
    // ... rest of the code
}

fn default_log_targets() -> Vec<tauri_plugin_log::Target> {
    vec![
        tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Stdout),
        tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Webview),
        tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::LogDir { 
            file_name: Some("launcher.log".into()) 
        }),
    ]
}
```

### 注意事项

1. **目录创建时机**：日志配置前必须先创建目录，否则日志插件可能失败
2. **注册表读取时机**：`get_installed_version()` 不依赖 Tauri 上下文，可在 `run()` 开始时调用
3. **Setup 模式**：保持默认 `%APPDATA%\Colink Setup\logs\`，因为 Setup 不需要固定安装目录
4. **macOS 兼容**：registry.rs 已支持 macOS plist，逻辑一致

---

## 任务列表

### Task 1: 修改 lib.rs 日志配置逻辑

**文件**: `src-tauri/src/lib.rs`

**内容**:
1. 添加 `default_log_targets()` 函数
2. 修改 `run()` 函数，根据模式动态配置日志路径
3. Launcher 模式：读取注册表 → 获取 install_dir → 创建 `{install_dir}/data/logs/` → 配置 Folder target

### Task 2: 更新 CLAUDE.md 文档

**文件**: `installer-tauri/CLAUDE.md`

**内容**: 添加日志位置说明：
- Launcher 模式：`{install_dir}/data/logs/launcher.log`
- Setup 模式：`%APPDATA%\Colink Setup\logs\setup.log`

---

## 验证方式

构建后测试：
1. 安装到 `D:\colink`
2. 运行 `Colink.exe`
3. 检查 `D:\colink\data\logs\launcher.log` 是否存在
4. 查看日志内容是否正确记录启动流程
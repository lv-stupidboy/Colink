# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Colink Installer (Tauri) - Windows 安装程序，基于 Tauri 2 + React。

## 常用命令

```bash
# 开发
pnpm dev                    # 启动 Setup 模式开发调试
pnpm dev:launcher           # 启动 Launcher 模式开发调试
pnpm dev:renderer           # 仅启动前端 Vite 开发服务器

# 构建
pnpm build                  # 构建 Setup 安装程序 (NSIS)
pnpm build:launcher         # 构建 Launcher exe (不打包)
pnpm build:all              # 构建两者

# 其他
pnpm typecheck              # TypeScript 类型检查
pnpm format                 # Prettier 格式化
```

## 完整发布构建

从主项目目录执行：

```bash
cd D:\workspace\isdp
pwsh -File scripts/build-release.ps1
```

构建步骤（7步）：
1. **ISDP 后端** - 编译 `bin/colink-server.exe` 和 `bin/migrate.exe`
2. **ISDP 前端** - 构建 `web/dist/`
3. **资源同步** - 复制到 `staging/resources/`
4. **配置文件** - 复制 `VERSION` 和 `installer-config.json`
5. **Installer 前端** - 构建 Tauri 前端 `dist/`
5.5. **图标生成** - 从 `icon.png` 生成各平台图标（`.ico`, `.icns`, PNG）
6. **Tauri exe** - 编译 `Colink-Setup.exe` 和 `Colink.exe`
7. **ZIP 打包** - 输出到 `target/release/dist/Colink-Setup-{VERSION}.zip`

### 图标管理

源文件：`src-tauri/icons/icon.png` (512x512 PNG)

```bash
# 从源图片生成所有图标
pnpm tauri icon src-tauri/icons/icon.png
```

`.gitignore` 只保留源图片，其他图标在构建时自动生成：
```
src-tauri/icons/*
!src-tauri/icons/icon.png
```

### ZIP 目录结构

```
Colink-Setup-{VERSION}/
├── Start-Setup.bat      # 启动脚本
├── README.txt           # 安装说明
├── dist/                # Installer 前端资源
└── exe/
    ├── Colink-Setup.exe # 安装程序
    └── resources/
        ├── colink-server.exe
        ├── migrate.exe
        ├── web/          # ISDP 前端
        ├── sql-change/   # 数据库迁移脚本
        ├── launcher/
        │   └── Colink.exe
        ├── config.yaml.example
        ├── VERSION
        └── installer-config.json
```

## 双应用模式架构

单个二进制文件通过 exe 文件名检测运行模式：

| 文件名 | 模式 | 功能 |
|--------|------|------|
| `Colink Setup.exe` | Setup | 安装/升级/卸载向导 |
| `Colink.exe` | Launcher | 运行时服务控制面板 |

模式检测逻辑在 `store.rs:detect_app_mode()`：

```rust
fn detect_app_mode() -> AppMode {
    let exe_name = std::env::current_exe().ok()
        .and_then(|p| p.file_name().map(|n| n.to_string_lossy().to_string()))
        .unwrap_or_default();

    if exe_name.contains("Launcher") || exe_name == "Colink.exe" {
        AppMode::Launcher
    } else {
        AppMode::Setup
    }
}
```

## 架构

```
React 前端 (src/)
    ↓ Tauri invoke (IPC)
Rust 后端 (src-tauri/src/)
  ├── commands/     # IPC 命令处理 (9 模块)
  ├── services/     # 业务逻辑 (10 模块)
  ├── store.rs      # AppState 全局状态
  └── lib.rs        # Tauri 插件注册
```

### Rust 后端模块

**commands/** (IPC 命令层):
- `install.rs` - 安装流程命令
- `service.rs` - 服务启动/停止
- `config.rs` - 配置文件读写
- `dependency.rs` - 依赖检测
- `invite.rs` - 邀请码验证
- `launcher.rs` - Launcher 操作命令
- `uninstall.rs` - 卸载命令
- `window.rs` - 窗口控制
- `mode.rs` - 模式检测

**services/** (业务逻辑层):
- `installer.rs` - 安装流程核心逻辑
- `service_manager.rs` - colink-server.exe 进程管理
- `registry.rs` - Windows 注册表操作
- `shortcut.rs` - 快捷方式创建
- `config.rs` - 配置文件生成
- `dependency.rs` - 依赖检测与安装
- `invite.rs` - 邀请码验证 API 调用
- `uninstall.rs` - 卸载清理
- `file_ops.rs` - 文件操作工具
- `disk_space.rs` - 磁盘空间检测

### 前端结构

**页面流程** (App.tsx):
1. Welcome → 2. DirectorySelect → 3. InviteVerification → 4. DependencyCheck → 5. SystemConfig → Installing → Complete

升级模式跳过步骤 2 (目录选择)。

**API 层** (`src/lib/api/`):
- 每个 API 模块封装一类 Tauri invoke 命令
- 类型定义在 `types.ts`

## Tauri 配置

两个独立配置文件：
- `tauri.conf.json` - Setup 配置 (`bundle.active: true`, 打包 NSIS)
- `tauri.launcher.conf.json` - Launcher 配置 (`bundle.active: false`, 不打包)

构建 Launcher 时使用 `--config` 参数指定配置文件。

## Resources 目录

`src-tauri/resources/` 包含打包资源：
- `colink-server.exe` - 后端服务
- `web/` - 前端静态文件
- `launcher/Colink.exe` - Launcher exe
- `bin/` - 辅助工具 (migrate.exe)
- `sql-change/` - 数据库迁移脚本
- `config.yaml.example` - 配置模板
- `installer-config.json` - 安装器配置 (API URLs)

## 开发调试

Setup 模式调试时，`bundle.resources` glob 检查可能失败。确保 resources 目录有匹配的文件，或使用具体 glob patterns（如 `resources/*.exe`）而非 `resources/**`。

## 关键约束

### IPC 命名
- Rust 命令函数名使用 snake_case: `start_installation`
- 前端 invoke 使用相同名称: `invoke('start_installation')`

### JSON 字段命名
- 统一使用 camelCase（Rust struct 添加 `#[serde(rename_all = "camelCase")]`）

### 前端深色模式
- 使用 CSS 变量（同主项目 isdp）
- 变量定义在主题 CSS 文件中
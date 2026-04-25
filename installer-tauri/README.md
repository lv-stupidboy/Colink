# Colink Installer (Tauri)

基于 Tauri 2 的 Colink Windows 安装程序，替代原有的 Electron 实现。

## 项目结构

```
installer-tauri/
├── src-tauri/              # Rust 后端
│   ├── Cargo.toml          # Rust 依赖配置
│   ├── tauri.conf.json     # Tauri 配置
│   └── src/
│       ├── main.rs         # 入口点 + 模式检测
│       ├── lib.rs          # 插件注册
│       ├── error.rs        # 错误类型
│       ├── store.rs        # AppState
│       ├── commands/       # IPC 命令 (9 个模块)
│       └── services/       # 业务逻辑 (10 个模块)
├── src/                    # React 前端
│   ├── main.tsx            # 入口
│   ├── App.tsx             # 主组件 + 路由
│   ├── lib/api/            # Tauri invoke 包装层
│   └── renderer/src/       # 页面和组件
│       ├── pages/          # 9 个页面组件
│       └── components/     # Layout 等组件
├── resources/              # 打包资源
├── package.json            # 前端依赖
├── vite.config.ts          # Vite 配置
└── build-tauri.ps1         # Windows 构建脚本
```

## 开发命令

```bash
# 安装依赖
pnpm install

# 开发模式 (启动 Tauri 应用)
pnpm dev

# 仅启动前端开发服务器
pnpm dev:renderer

# 构建生产版本
pnpm build

# TypeScript 类型检查
pnpm typecheck
```

## 双应用模式

单个二进制文件通过文件名检测模式：
- `Colink Setup.exe` → Setup 模式 (安装/升级/卸载)
- `Colink.exe` → Launcher 模式 (运行时服务控制)

## IPC 命令

### 安装命令
- `check_installed` - 检查已安装版本
- `start_installation` - 执行安装流程
- `select_directory` - 选择安装目录
- `get_disk_space` - 获取磁盘空间

### 服务命令
- `start_service` - 启动 colink-server.exe
- `stop_service` - 停止服务
- `get_service_status` - 获取服务状态
- `get_running_agents` - 获取运行中的 Agent

### 配置命令
- `read_config_file` - 读取配置文件
- `save_config` - 保存配置

### 其他
- `is_launcher_mode` - 检测当前模式
- `verify_invite_code` - 验证邀请码
- `check_dependency` - 检查依赖

## 构建

```powershell
# Windows
.\build-tauri.ps1
```

输出目录: `output/`

## 依赖

### Rust
- Tauri 2 + 插件 (dialog, opener, store, process, single-instance)
- winreg (Windows 注册表)
- serde/serde_json/serde_yaml (序列化)
- tokio (异步运行时)
- thiserror (错误处理)

### 前端
- React 18 + React Router
- Ant Design 5
- Tauri API (@tauri-apps/api)
- Vite 5
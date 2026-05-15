# Launcher Desktop Application 化改造规格文档

> **日期**: 2026-05-09
> **参考**: `docs/colink-desktop-app-2026-04-29.md` (apps/desktop Electron 方案)

## 1. 概述

### 1.1 背景

当前 `installer-tauri` 项目有两种模式：
- **Setup 模式**：安装/升级/卸载向导（保持不变）
- **Launcher 模式**：服务控制面板，点击"打开控制台"打开外部浏览器访问 web UI

### 1.2 改造目标

将 Launcher 模式从"控制面板"升级为"完整桌面应用"：
- **核心变化**：Launcher 窗口直接嵌入 web UI，不再打开外部浏览器
- **服务管理简化**：应用启动自动启动服务，关闭停止服务，状态绑定应用生命周期
- **用户体验**：Splash Screen 启动体验，错误页面 + 重试

### 1.3 改造范围

| 模式 | 改造状态 | 说明 |
|------|----------|------|
| Setup 模式 | **不改造** | 保持安装向导流程不变 |
| Launcher 模式 | **改造** | 从控制面板变为 web UI 容器 |

---

## 2. 用户需求决策

### 2.1 已确认决策

| 决策点 | 选择 | 说明 |
|--------|------|------|
| **改造范围** | 仅 Launcher | Setup 模式保持不变 |
| **布局方案** | Web UI 融合 | Launcher 窗口只加载 web UI |
| **服务管理** | 生命周期绑定 | 启动→自动启动服务，关闭→停止服务 |
| **启动体验** | Splash Screen | 品牌启动画面 |
| **启动失败** | 错误页面 + 重试 | 不退出应用，提供重试按钮 |
| **Splash 内容** | 标准版 | Logo + 品牌名 + 版本号 + 进度条 |
| **窗口外观** | 标准窗口 | 有标题栏、可缩放、有最小化/最大化按钮 |
| **系统托盘** | 不需要 | 关闭 = 退出 = 停止服务 |
| **窗口尺寸** | 用户自定义 | 记住上次大小，首次自适应 |
| **主题** | 用户可切换 | 提供深色/浅色主题切换按钮 |

---

## 3. 功能规格

### 3.1 启动流程

```
应用启动 → 检测安装状态 → 显示 Splash Screen → 启动后端服务 → 服务就绪 → 加载 web UI
```

**详细步骤**：

1. **应用启动**：
   - 检测模式（通过 exe 文件名）
   - Launcher 模式：检测是否已安装（读取注册表）

2. **未安装处理**：
   - 显示错误对话框："Colink 未安装，请先运行安装程序"
   - 用户确认后退出应用

3. **Splash Screen**（已安装）：
   - 显示 Logo + 品牌名 + 版本号
   - 显示进度条 + "正在启动服务..." 文字
   - 后台启动 colink-server.exe 进程

4. **服务启动**：
   - 检查端口是否被占用（如被占用，尝试终止占用进程）
   - 启动 colink-server.exe
   - 等待服务就绪（轮询 /health 接口）

5. **服务就绪**：
   - 隐藏 Splash Screen
   - 显示 WebView 加载 web UI

### 3.2 启动失败处理

**失败场景**：
- 端口被占用且无法终止
- 服务进程启动后立即退出
- 配置文件错误
- 资源文件缺失

**处理方式**：
- 显示错误页面（非对话框）
- 错误信息详细展示（错误类型 + 具体原因）
- 提供"重试"按钮
- 提供"查看日志"按钮（打开日志目录）

### 3.3 主界面

**布局**：
- 标准 Tauri WebView 窗口
- 直接加载 `http://localhost:{port}/`（web UI）
- 无额外的控制面板 UI

**窗口属性**：
- 标准窗口装饰（标题栏、边框）
- 最小化/最大化/关闭按钮
- 可缩放，有最小尺寸限制
- 首次启动：自适应屏幕尺寸（80% 宽度，70% 高度）
- 后续启动：恢复上次关闭时的尺寸

### 3.4 关闭流程

```
用户关闭窗口 → 停止后端服务 → 退出应用
```

**详细步骤**：
1. 用户点击关闭按钮
2. 发送 SIGTERM/KILL 到 colink-server.exe 进程
3. 等待进程退出（最多 5 秒）
4. 保存窗口尺寸到配置
5. 退出应用

### 3.5 主题切换

**实现方式**：
- 通过 Tauri WebView 与 web UI 的 JavaScript 交互
- web UI 已支持深色/浅色主题（CSS 变量）
- Launcher 提供主题切换按钮（可选：通过窗口标题栏或 web UI 内的按钮）

---

## 4. 技术规格

### 4.1 技术栈

| 组件 | 技术 | 说明 |
|------|------|------|
| 桌面应用框架 | Tauri 2 | 现有 installer-tauri 已使用 |
| 后端语言 | Rust | 现有代码库 |
| 前端 | React + TypeScript | Splash Screen 和错误页面 |
| WebView | Tauri WebView | 加载 web UI |
| 服务管理 | 现有 ServiceManager | 复用 `services/service_manager.rs` |

### 4.2 架构设计

```
installer-tauri/
├── src-tauri/
│   ├── src/
│   │   ├── lib.rs                # Tauri 入口（修改启动逻辑）
│   │   ├── commands/
│   │   │   ├── launcher.rs       # 新增：启动服务、获取状态
│   │   │   └── window.rs         # 新增：窗口尺寸保存/恢复
│   │   ├── services/
│   │   │   ├── service_manager.rs # 复用，增强启动检测
│   │   │   └── launcher.rs       # 新增：启动流程管理
│   │   └── store.rs              # 修改：增加启动状态
│   └── tauri.conf.json           # 修改：Launcher 窗口配置
├── src/
│   ├── App.tsx                   # 修改：Launcher 模式入口
│   ├── launcher/                 # 新增：Launcher 专用前端
│   │   ├── SplashScreen.tsx      # Splash Screen 组件
│   │   ├── ErrorPage.tsx         # 错误页面组件
│   │   ├── WebUIContainer.tsx    # WebView 容器
│   │   └── LauncherApp.tsx       # Launcher 主应用
│   └── lib/
│       └── api/
│           └── launcher.ts       # 新增：Launcher API
```

### 4.3 核心组件

#### 4.3.1 SplashScreen（前端）

```typescript
interface SplashScreenProps {
  version: string;
  progress: number;        // 0-100
  statusText: string;      // "正在启动服务..."、"正在检测端口..."
}
```

**视觉元素**：
- Logo（居中，较大）
- 品牌名 "Colink"（Logo 下方）
- 版本号（如 "v1.0.0"，品牌名下方）
- 进度条（底部，线性进度）
- 状态文字（进度条下方）

**样式**：
- 背景：深色主题背景色
- 文字：白色/浅色
- 进度条：品牌色

#### 4.3.2 ErrorPage（前端）

```typescript
interface ErrorPageProps {
  errorType: 'port_conflict' | 'process_exit' | 'config_error' | 'missing_files';
  errorMessage: string;
  onRetry: () => void;
  onViewLogs: () => void;
}
```

**视觉元素**：
- 错误图标（红色警告图标）
- 错误标题（如 "启动失败"）
- 错误详情（具体错误信息）
- "重试"按钮（主要按钮）
- "查看日志"按钮（次要按钮）

#### 4.3.3 ServiceLauncher（Rust）

```rust
pub struct ServiceLauncher {
    service_manager: ServiceManager,
    status: LauncherStatus,
}

pub enum LauncherStatus {
    Initializing,
    CheckingInstallation,
    StartingService,
    WaitingForReady,
    Ready,
    Failed(LauncherError),
}

pub struct LauncherError {
    kind: LauncherErrorKind,
    message: String,
}

pub enum LauncherErrorKind {
    NotInstalled,
    PortConflict,
    ProcessExit,
    ConfigError,
    MissingFiles,
}
```

### 4.4 IPC 命令

**新增命令**：

| 命令 | 参数 | 返回 | 说明 |
|------|------|------|------|
| `launcher_start_service` | - | `LauncherStatus` | 启动服务并返回状态 |
| `launcher_get_status` | - | `LauncherStatus` | 获取当前启动状态 |
| `launcher_retry` | - | `LauncherStatus` | 重试启动 |
| `launcher_open_logs` | - | - | 打开日志目录 |
| `launcher_save_window_size` | width, height | - | 保存窗口尺寸 |
| `launcher_get_window_size` | - | width, height | 获取保存的窗口尺寸 |
| `launcher_get_version` | - | version | 获取应用版本 |

### 4.5 配置存储

**窗口尺寸配置**：
- 存储位置：`~/.colink/launcher_prefs.json`（跨平台）
- 内容：`{ "windowWidth": 1280, "windowHeight": 800, "theme": "dark" }`

**首次启动默认尺寸**：
- 计算：屏幕宽度 80%，屏幕高度 70%
- 最小限制：900px 宽，600px 高

---

## 5. 与参考方案对比

| 方面 | apps/desktop (Electron) | installer-tauri (Tauri 2) |
|------|-------------------------|---------------------------|
| **框架** | Electron 30+ | Tauri 2 |
| **后端语言** | TypeScript/Node.js | Rust |
| **WebView** | Chromium | 系统 WebView |
| **服务管理** | daemon-manager.ts | ServiceManager (Rust) |
| **启动体验** | Spin 组件 | SplashScreen 组件 |
| **嵌入方式** | iframe | WebView 直接加载 URL |
| **窗口配置** | BrowserWindow API | tauri.conf.json |

**关键差异**：
- Tauri 无需 iframe，直接通过 WebView 加载 URL
- Rust 比 TypeScript 更适合进程管理
- Tauri 体积更小（系统 WebView vs Chromium）

---

## 6. 验收标准

### 6.1 功能验收

| 功能 | 验收条件 |
|------|----------|
| 启动流程 | Launcher 启动 → Splash 显示 → 服务启动 → web UI 显示 |
| 启动失败 | 失败 → 错误页面 → 重试按钮 → 重试成功 |
| 窗口尺寸 | 首次自适应 → 关闭保存 → 再次打开恢复 |
| 服务停止 | 关闭窗口 → 服务停止 → 进程终止 |
| 主题切换 | 切换按钮 → web UI 主题同步变化 |

### 6.2 性能验收

| 指标 | 目标 |
|------|------|
| Splash Screen 显示 | < 500ms |
| 服务启动时间 | < 5s |
| web UI 加载时间 | < 2s（本地） |
| 应用关闭时间 | < 5s |

---

## 7. 风险与依赖

### 7.1 技术风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| WebView 兼容性 | 不同系统 WebView 行为差异 | 测试 Windows/macOS |
| 端口冲突检测 | 误判端口占用 | 使用多种检测方法 |
| 进程管理 | 进程孤儿/僵尸 | 确保清理逻辑完善 |

### 7.2 依赖

| 依赖 | 说明 |
|------|------|
| web UI 运行正常 | 需要 web UI 支持深色/浅色主题切换 |
| colink-server.exe | 后端服务正常运行 |
| 注册表数据 | Launcher 模式需要读取安装信息 |

---

## 8. 后续 TODO（不在本范围）

- 系统托盘支持（用户明确不需要）
- 自动更新功能（可选，暂不实现）
- 多语言支持（暂不实现）
- macOS 特定优化（后续版本）
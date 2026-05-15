# Launcher 桌面应用功能扩展设计文档

> 文档编号: 2026-05-14-launcher-desktop-features-extension-design
> 创建日期: 2026-05-14
> 状态: 设计完成

## 背景

Launcher 转化为桌面应用后，部分原有功能丢失。需要在 Web 控制台补充以下功能：

1. **系统配置编辑器** - 直接修改 config.yaml，修改后提醒用户重启生效
2. **智能体检测与安装** - 提供 4 种智能体的状态检测和安装功能
3. **快捷入口** - 打开日志目录、数据目录

## 需求确认

### 展示位置
- **确认**: Web 控制台"通用设置"页面（GeneralSettings.tsx）
- **技术方案**: 通过 iframe postMessage 与桌面应用通信

### 系统配置编辑
- **确认**: 纯文本 YAML 编辑，用户自由编辑（类似原 Launcher 的 ConfigEditor）
- **保存后行为**: 提醒用户需要重启服务才能生效

### 智能体安装
- **确认**: 命令行安装（npm/brew）
- **支持的智能体**: 4 种（Claude Code、OpenCode、Hermes、OpenClaw）

### 技术架构
- **确认**: Web → 桌面应用直通（postMessage + preload API）

## 智能体详细信息

| 智能体 | 类型标识 | 检测命令 | 安装命令 | 备注 |
|--------|---------|----------|----------|------|
| Claude Code | `claude_code` | `claude --version` | `npm install -g @anthropic-ai/claude-code` | 标准 npm 安装 |
| OpenCode | `open_code` | `openode --version` | `npm install -g openode` | 标准 npm 安装 |
| Hermes | `hermes` | `hermes --version` | **不支持一键安装** | Windows 需 WSL2，提示用户自行安装 |
| OpenClaw | `openclaw` | `openclaw --version` | `npm install -g openclaw` | 安装后需运行 `openclaw onboard` |

## 架构设计

### 通信流程

```
┌─────────────────────────────────────────────────────────┐
│                   Web 控制台 (iframe)                    │
│                                                          │
│  GeneralSettings.tsx                                     │
│    ├─ SystemConfigCard (系统配置编辑器)                   │
│    ├─ AgentManagementCard (智能体管理)                    │
│    └─ QuickAccessCard (快捷入口)                          │
│                                                          │
│    ↓ window.desktopAPI.xxx()                             │
└─────────────────────────────────────────────────────────┘
                         ↓ postMessage
┌─────────────────────────────────────────────────────────┐
│               桌面应用 preload (渲染进程)                 │
│                                                          │
│  preload/index.ts                                        │
│    ↓ ipcRenderer.invoke('xxx')                           │
└─────────────────────────────────────────────────────────┘
                         ↓ IPC
┌─────────────────────────────────────────────────────────┐
│               桌面应用 main (主进程)                      │
│                                                          │
│  main.ts                                                 │
│    ├─ fs.readFile/writeFile (配置读写)                   │
│    ├─ child_process.exec (版本检测、安装命令)            │
│    └─ shell.openPath (打开目录)                           │
└─────────────────────────────────────────────────────────┘
```

### API 定义

**扩展 desktopAPI 接口**（新增 6 个方法）：

```typescript
// apps/desktop/src/preload/index.d.ts
interface DesktopAPI {
  // 已有方法...
  
  // 新增方法
  readConfig(): Promise<string>;           // 读取 config.yaml
  saveConfig(content: string): Promise<void>;  // 保存 config.yaml
  checkAgent(agentType: string): Promise<AgentStatus>;  // 检测智能体状态
  installAgent(agentType: string): Promise<InstallResult>;  // 安装智能体
  openLogsDirectory(): Promise<void>;      // 打开日志目录
  openDataDirectory(): Promise<void>;      // 打开数据目录
}

interface AgentStatus {
  installed: boolean;
  version?: string;
  error?: string;
}

interface InstallResult {
  success: boolean;
  message?: string;
  error?: string;
}
```

**Tauri IPC 命令映射**：

| desktopAPI 方法 | IPC 命令 | 实现方式 |
|-----------------|----------|----------|
| `readConfig()` | `config:read` | `fs.readFile(data/configs/config.yaml)` |
| `saveConfig(content)` | `config:save` | `fs.writeFile(data/configs/config.yaml)` |
| `checkAgent(type)` | `agent:check` | 执行版本检测命令 |
| `installAgent(type)` | `agent:install` | 执行安装命令 |
| `openLogsDirectory()` | `fs:open-logs` | `shell.openPath(data/logs)` |
| `openDataDirectory()` | `fs:open-data` | `shell.openPath(data/)` |

## UI 设计

### 卡片 1：系统配置编辑器

```
┌─────────────────────────────────────────────────────────┐
│ 系统配置                                                 │
│                                                          │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ server:                                              │ │
│ │   port: 26305                                        │ │
│ │ web:                                                 │ │
│ │   port: 26306                                        │ │
│ │ database:                                            │ │
│ │   type: sqlite                                       │ │
│ │ ...                                                  │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                          │
│ [保存配置]                                [重新加载]      │
│                                                          │
│ ⚠️ 保存后需要重启服务才能生效                            │
└─────────────────────────────────────────────────────────┘
```

**交互逻辑**：
1. 页面加载时调用 `readConfig()` 获取配置内容
2. 用户编辑 YAML 文本
3. 点击"保存配置"调用 `saveConfig(content)`
4. 显示 Toast 提示"配置已保存，请重启服务生效"
5. "重新加载"按钮重新调用 `readConfig()` 刷新内容

### 卡片 2：智能体管理

```
┌─────────────────────────────────────────────────────────┐
│ 智能体管理                                               │
│                                                          │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Claude Code                          [✓ 已安装]     │ │
│ │ 版本: v1.0.0                                         │ │
│ │                                                      │ │
│ │ [重新检测]                                           │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                          │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ OpenCode                             [✗ 未安装]     │ │
│ │                                                      │ │
│ │ [检测并安装]                                         │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                          │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Hermes                               [⚠️ 需WSL2]    │ │
│ │ Windows 原生系统不支持，需要 WSL2                    │ │
│ │ 请自行在 WSL2 中安装 Hermes                         │ │
│ │                                                      │ │
│ │ [重新检测]                                           │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                          │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ OpenClaw                             [✗ 未安装]     │ │
│ │                                                      │ │
│ │ [检测并安装]                                         │ │
│ │                                                      │ │
│ │ ⚠️ 安装后需运行 openclaw onboard 完成初始化         │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                          │
│ [全部检测]                                              │
└─────────────────────────────────────────────────────────┘
```

**交互逻辑**：
1. 页面加载时自动调用全部智能体的 `checkAgent()`
2. 显示各智能体的安装状态和版本
3. "重新检测"按钮重新调用单个智能体的 `checkAgent()`
4. "检测并安装"按钮先检测，未安装则调用 `installAgent()`
5. 安装过程中显示 loading 状态
6. 安装完成显示成功/失败 Toast
7. Hermes 显示特殊提示：需要 WSL2，不支持一键安装
8. OpenClaw 显示特殊提示：安装后需运行 `openclaw onboard`

### 卡片 3：快捷入口

```
┌─────────────────────────────────────────────────────────┐
│ 快捷入口                                                 │
│                                                          │
│ 📁 日志目录：data/logs/                    [打开目录]   │
│ 📁 数据目录：data/                          [打开目录]   │
└─────────────────────────────────────────────────────────┘
```

**交互逻辑**：
1. 点击"打开目录"调用对应 API
2. 使用系统默认文件管理器打开目录

### 环境检测提示

- 在桌面应用中：功能正常可用
- 在浏览器中访问：显示"此功能仅在桌面应用中可用"提示

## 文件清单

| 文件 | 作用 | 变更类型 |
|------|------|----------|
| `apps/desktop/src/preload/index.ts` | 扩展 preload API | 修改 |
| `apps/desktop/src-tauri/src/commands/` | 新增 IPC 命令处理器 | 新增 |
| `apps/desktop/src-tauri/src/main.rs` | 注册 IPC 命令 | 修改 |
| `web/src/pages/Settings/GeneralSettings.tsx` | 添加三个功能模块 | 修改 |
| `web/src/components/SystemConfigCard.tsx` | 系统配置编辑器组件 | 新增 |
| `web/src/components/AgentManagementCard.tsx` | 智能体管理组件 | 新增 |
| `web/src/components/QuickAccessCard.tsx` | 快捷入口组件 | 新增 |

## 安全考虑

1. **配置文件权限** - 只允许读写 `data/configs/config.yaml`，不能访问其他路径
2. **命令执行白名单** - 只允许执行预定义的安装命令，不接受用户输入
3. **目录访问限制** - 只允许打开 `data/logs` 和 `data/` 目录

## 测试要点

1. 配置编辑器加载/保存功能
2. 各智能体检测功能（已安装/未安装）
3. 智能体安装流程（成功/失败）
4. 目录打开功能
5. 浏览器访问时的提示信息
6. Hermes 特殊提示显示
7. OpenClaw 特殊提示显示

---

**设计确认**: 用户已确认架构设计、API 设计和功能模块设计
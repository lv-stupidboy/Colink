---
name: launcher-tauri-desktop-features-extension
description: Tauri Launcher 桌面应用功能扩展 - 补充系统配置编辑、智能体检测管理、快捷操作功能
metadata:
  type: project
---

# Tauri Launcher 桌面应用功能扩展设计

## 背景

Launcher 从独立 Electron 应用转化为 Tauri 桌面应用后，丢失了以下原有功能：
1. 系统配置编辑 - 直接修改 config.yaml
2. 智能体检测与安装 - 检测 CLI 工具并提供安装按钮
3. 快捷操作 - 查看日志、打开数据目录

**Why:** Tauri Launcher 通过 iframe 嵌入 Web 控制台，需要通过 postMessage 机制调用原生功能，原有 Electron preload API 方式不再适用。

**How to apply:** 本设计文档定义 Web 控制台与 Tauri Launcher 的通信协议和功能实现方案。

---

## 需求确认

| 功能 | 确认方案 |
|------|---------|
| 展示位置 | Web 控制台"通用设置"页面（iframe postMessage → Tauri） |
| 系统配置编辑 | 纯文本 YAML 编辑，保存后提醒重启生效 |
| 智能体安装 | 命令行安装（npm） |
| 智能体展示 | 统一列表（4 个智能体） |
| 日志/目录查看 | 按钮组（独立按钮） |

---

## 4 种智能体详情

| 智能体 | 检测命令 | 安装命令 | 特殊说明 |
|--------|----------|----------|----------|
| Claude Code | `claude --version` | `npm install -g @anthropic-ai/claude-code` | 无 |
| OpenCode | `opencode --version` | `npm install -g @anthropic-ai/opencode-cli` | 无 |
| Hermes | `hermes --version` | 不提供安装按钮 | 提示需 WSL2 + 自行安装 |
| OpenClaw | `openclaw --version` | `npm install -g openclaw` | 安装后需运行 `openclaw onboard` |

---

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│  Web 控制台 (iframe 嵌入)                                │
│  - GeneralSettings.tsx 新增 3 个功能卡片                 │
│  - 通过 postMessage 调用 Tauri 命令                      │
└─────────────────────────────────────────────────────────┘
                         ↓ postMessage
┌─────────────────────────────────────────────────────────┐
│  WebUIContainer.tsx (launcher 前端)                      │
│  - 监听 iframe postMessage                               │
│  - 调用 invoke() 执行 Tauri 命令                         │
└─────────────────────────────────────────────────────────┘
                         ↓ Tauri invoke
┌─────────────────────────────────────────────────────────┐
│  Tauri Rust Commands                                    │
│  - read_config_file / save_config                       │
│  - check_dependency / install_dependency                │
│  - open_log_dir / open_data_dir / open_config_dir       │
└─────────────────────────────────────────────────────────┘
```

---

## postMessage 通信协议

### 消息类型映射

| 功能 | postMessage 类型 | Tauri 命令 |
|------|------------------|------------|
| 读取配置 | `config:read` | `read_config_file` |
| 保存配置 | `config:save` | `save_config` |
| 检测智能体 | `dependency:check` | `check_dependency` |
| 安装智能体 | `dependency:install` | `install_dependency` |
| 打开日志目录 | `open:log_dir` | `open_log_dir` |
| 打开数据目录 | `open:data_dir` | `open_data_dir` |
| 打开配置目录 | `open:config_dir` | `open_config_dir` |

### 消息格式

**请求格式**：
```typescript
// Web 控制台发送
window.parent.postMessage({
  type: 'dependency:check',
  payload: { agent: 'claude' }
}, '*');
```

**响应格式**：
```typescript
// WebUIContainer 返回
iframe.contentWindow.postMessage({
  type: 'dependency:check:result',
  payload: { installed: true, version: '1.0.0' }
}, '*');
```

---

## Web 控制台功能模块 UI

### 卡片 1：系统配置编辑器

```
┌─────────────────────────────────────────────────────────┐
│ 系统配置                                                 │
├─────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────────┐ │
│ │ [YAML 文本编辑区域 - TextArea]                       │ │
│ │                                                     │ │
│ │ server:                                             │ │
│ │   port: 26305                                       │ │
│ │ ...                                                 │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ [保存配置]  [重新加载]                                   │
│                                                         │
│ ⚠️ 提示：配置修改后需要重启服务才能生效                   │
└─────────────────────────────────────────────────────────┘
```

### 卡片 2：智能体管理

```
┌─────────────────────────────────────────────────────────┐
│ 智能体管理                                               │
├─────────────────────────────────────────────────────────┤
│ 智能体名称        状态            操作                   │
│ ─────────────────────────────────────────────────────── │
│ Claude Code       ✅ 已安装 v1.0    [重新检测]           │
│ OpenCode          ❌ 未安装         [安装] [重新检测]    │
│ Hermes            ❌ 未安装         [重新检测]           │
│                   ℹ️ 需在 WSL2 中安装                     │
│ OpenClaw          ❌ 未安装         [安装] [重新检测]    │
│                   ℹ️ 安装后需运行 onboard                 │
│                                                         │
│ [全部重新检测]                                           │
└─────────────────────────────────────────────────────────┘
```

### 卡片 3：快捷操作

```
┌─────────────────────────────────────────────────────────┐
│ 快捷操作                                                 │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ [查看日志]  [打开数据目录]  [打开配置目录]                │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 实施范围

### 需要修改的文件

**installer-tauri（Tauri Launcher）**：
- `src-tauri/src/services/dependency.rs` - 扩展支持 4 个智能体
- `src-tauri/src/services/file_ops.rs` - 新增目录打开命令
- `src-tauri/src/lib.rs` - 注册新命令
- `src/launcher/WebUIContainer.tsx` - 扩展 postMessage 处理

**web（Web 控制台）**：
- `src/utils/tauriBridge.ts` - 新增 iframe-to-Tauri 通信工具（如不存在）
- `src/components/settings/SystemConfigCard.tsx` - 新增系统配置编辑器组件
- `src/components/settings/AgentManagementCard.tsx` - 新增智能体管理组件
- `src/components/settings/QuickAccessCard.tsx` - 新增快捷操作组件
- `src/pages/Settings/GeneralSettings.tsx` - 集成新组件

---

## 成功标准

1. 用户可在通用设置页面直接编辑 config.yaml
2. 保存配置后显示重启提醒
3. 智能体管理显示 4 种智能体状态
4. Claude Code、OpenCode、OpenClaw 提供安装按钮
5. Hermes 显示 WSL2 安装提示
6. 快捷操作按钮可打开对应目录

---

## 依赖与约束

- **依赖**：Tauri Launcher 必须已启动 Web 控制台（iframe 嵌入）
- **约束**：在浏览器直接访问 Web 控制台时，功能不可用（需显示提示）
- **约束**：Hermes 安装需用户自行在 WSL2 中完成
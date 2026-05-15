# Tauri Launcher Agent 运行检查设计

## 需求

Tauri Launcher 停止服务时（包括关闭窗口和点击停止按钮），检查是否有 Agent 实例正在运行。若运行中，阻止操作并提示用户先在 Web 控制台停止 Agent。

## 背景

### 现状分析

| 位置 | 检查 Agent 运行 | 状态 |
|------|----------------|------|
| Tauri 前端停止按钮 (`LauncherDashboard.tsx:152-156`) | ✅ 有 | `message.warning` 提示 |
| Tauri 后端 `window_close_with_confirm` | ❌ 无 | 只检查服务状态 |
| Tauri 后端 `stop_service` | ❌ 无 | 直接停止 |
| Tauri 前端关闭按钮 (`Layout.tsx`) | ❌ 无 | 调用 `window_close_with_confirm` |

**问题**：
- 前端检查可被绕过（直接调用后端 API）
- 后端缺少防御性检查层

### 参考

- Electron Launcher 前端 `LauncherDashboard.tsx:156` 已实现停止按钮禁用
- Tauri `ServiceManager::get_running_agents()` 已实现 API 调用（`service_manager.rs:458-479`）

## 设计方案

### 方案选择

| 方案 | 描述 | 权衡 |
|------|------|------|
| **A: 仅后端检查** | 在 `stop_service` 和 `window_close_with_confirm` 增加 Agent 检查 | 防御性强，前端可简化 |
| **B: 仅前端检查** | 前端阻止调用，后端不改 | 可被绕过，不安全 |
| **C: 前后端双重检查（推荐）** | 前端已有检查保留，后端增加防御层 | 最安全，用户体验好 |

**推荐方案 C**：前端检查提供即时 UI 反馈，后端检查防止绕过。

### 交互流程

#### 关闭窗口时

```
用户点击关闭按钮
    ↓
前端调用 windowApi.close()
    ↓
后端 window_close_with_confirm
    ↓
检查服务状态 (is_running)
    ↓
┌─ 服务运行 ────────────────────────────┐
│  检查 Agent 实例 (get_running_agents)  │
│  ↓                                     │
│  ┌─ Agent 运行中 ────────────────────┐ │
│  │  弹窗提示：                       │ │
│  │  "有 X 个 Agent 实例正在运行，    │ │
│  │   请先在 Web 控制台停止后         │ │
│  │   才能关闭窗口"                   │ │
│  │  按钮：[取消]                     │ │
│  │  → 阻止关闭                       │ │
│  └───────────────────────────────────┘ │
│  ┌─ 无 Agent 运行 ──────────────────┐ │
│  │  弹窗确认：                       │ │
│  │  "服务正在运行，请先停止服务      │ │
│  │   后再关闭窗口"                   │ │
│  │  按钮：[停止服务并关闭] [取消]    │ │
│  │  → 原有逻辑                       │ │
│  └───────────────────────────────────┘ │
└───────────────────────────────────────┘
┌─ 服务未运行 ──────────────────────────┐
│  直接关闭                              │
└───────────────────────────────────────┘
```

#### 停止服务按钮时

```
用户点击停止按钮
    ↓
前端检查 agentCount > 0
    ↓
┌─ Agent 运行中 ────────────────────────┐
│  message.warning 提示                 │
│  阻止调用后端                          │
└───────────────────────────────────────┘
┌─ 无 Agent 运行 ────────────────────────┐
│  调用 serviceApi.stop()               │
│  ↓                                     │
│  后端 stop_service                     │
│  ↓                                     │
│  再次检查 Agent（防御层）              │
│  ↓                                     │
│  ┌─ Agent 运行中 ────────────────────┐ │
│  │  返回错误：                       │ │
│  │  { success: false,                │ │
│  │    error: "有 X 个 Agent...",     │ │
│  │    agentCount: X }                │ │
│  └───────────────────────────────────┘ │
│  ┌─ 无 Agent 运行 ──────────────────┐ │
│  │  正常停止服务                     │ │
│  └───────────────────────────────────┘ │
└───────────────────────────────────────┘
```

### 技术架构

#### 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `src-tauri/src/commands/window.rs` | `window_close_with_confirm` 增加 Agent 检查 |
| `src-tauri/src/commands/service.rs` | `stop_service` 增加 Agent 检查 |
| `src/renderer/src/pages/LauncherDashboard.tsx` | 前端处理新错误类型（可选，已有提示） |

#### 后端实现要点

**1. `window_close_with_confirm` 修改**

```rust
// 在检查服务运行后，增加 Agent 检查
if is_running {
    // 获取运行中的 Agent 实例
    let port = read_existing_config(&install_dir)
        .map(|(p, _)| p)
        .unwrap_or(26305);

    let manager = ServiceManager::new(install_dir.clone());
    let agents = manager.get_running_agents(port).await?;

    if !agents.is_empty() {
        // Agent 运行中，弹窗提示并阻止关闭
        show_agent_running_dialog(agents.len());
        return Ok(CloseResult::BlockClose);
    }

    // 无 Agent 运行，继续原有确认流程...
}
```

**2. `stop_service` 修改**

```rust
// 增加返回结构，包含 agentCount
#[derive(serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct StopResult {
    success: bool,
    error: Option<String>,
    agent_count: Option<usize>,
}

// 在停止前检查 Agent
let agents = manager.get_running_agents(port).await?;
if !agents.is_empty() {
    return Ok(StopResult {
        success: false,
        error: Some(format!("有 {} 个 Agent 实例正在运行...", agents.len())),
        agent_count: Some(agents.len()),
    });
}
```

### 弹窗文案设计

| 场景 | 标题 | 内容 | 按钮 |
|------|------|------|------|
| Agent 运行 + 关闭窗口 | "无法关闭" | "有 {N} 个 Agent 实例正在运行，请先在 Web 控制台停止后才能关闭窗口。" | [取消] |
| 仅服务运行 + 关闭窗口 | "无法关闭" | "服务正在运行，请先停止服务后再关闭窗口。" | [停止服务并关闭] [取消] |
| Agent 运行 + 停止按钮 | (前端提示) | "有 Agent 实例正在运行，请先在 Web 控制台停止" | (toast) |

### 错误处理

- API 调用失败（网络超时）：视为无 Agent 运行，允许操作继续（保守策略）
- Agent 列表解析失败：日志警告，视为无 Agent 运行

### 测试要点

1. **关闭窗口场景**
   - 服务运行 + Agent 运行 → 弹窗提示 Agent，阻止关闭
   - 服务运行 + 无 Agent → 弹窗确认停止服务，可关闭
   - 服务未运行 → 直接关闭

2. **停止按钮场景**
   - Agent 运行 → 前端 toast 提示，不调用后端
   - 无 Agent → 正常停止服务

3. **边界场景**
   - API 调用超时 → 允许操作继续
   - 双击关闭按钮 → `CLOSE_PENDING` guard 防护

## 与现有设计的关系

本设计基于 `docs/superpowers/plans/2026-05-06-launcher-close-service-check.md` 的服务检查实现，扩展 Agent 检查逻辑。

**变更点**：
- 原设计：仅检查服务状态
- 新设计：服务运行时，进一步检查 Agent 实例

## 验收标准

- [ ] 关闭窗口时，Agent 运行中弹窗提示并阻止关闭
- [ ] 关闭窗口时，无 Agent 运行但服务运行，弹窗确认停止服务
- [ ] 停止按钮点击时，Agent 运行中前端提示
- [ ] 停止按钮点击时，后端防御性检查生效
- [ ] API 调用失败时，不阻塞正常操作
# Agent实例展示与升级检测设计

> **目标:** 在启动器页面展示运行中的Agent实例列表，有Agent运行时禁止停止服务；升级前检测进程是否运行，若运行则报错提示用户手动停止，不再自动终止进程。

## 背景

升级包推送给用户后，发现大量用户报错"启动器可执行文件复制失败：Colink.exe不存在"，Colink.exe变成僵尸文件。根本原因是升级时colink-server.exe启动的Agent CLI子进程（Claude CLI、OpenCode等）未被正确清理，进程尝试连接外网API被安全软件拦截，导致异常状态影响文件操作。

原升级流程会自动终止进程，但这种方式不够安全，可能造成数据丢失或进程状态异常。改为检测进程并提示用户手动停止，让用户有机会保存工作、正常关闭Agent实例。

---

## 设计概览

### 新增功能

1. **启动器页面Agent实例列表** - 展示当前运行的Agent实例信息
2. **停止服务前置检测** - 有Agent实例运行时禁用停止按钮并提示用户
3. **升级前进程检测** - 检测Colink相关进程是否运行，运行则报错阻止升级

### 移除功能

1. **升级时自动终止进程** - 不再调用 `killAllProcesses()` / `stopAllProcesses()`

---

## Part 1: Go后端API

### 新增API

**路径:** `GET /api/v1/invocations/running`

**返回数据结构:**
```json
{
  "instances": [
    {
      "invocationId": "550e8400-e29b-41d4-a716-446655440000",
      "agentName": "代码审查员",
      "projectName": "isdp",
      "threadTitle": "修复登录bug",
      "startedAt": "2026-04-16T10:30:00Z",
      "runningDurationSeconds": 900
    }
  ]
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| invocationId | string | 调用ID（UUID） |
| agentName | string | Agent角色名称 |
| projectName | string | 所属项目名称 |
| threadTitle | string | 所属任务/Thread标题 |
| startedAt | string | 开始时间（ISO 8601） |
| runningDurationSeconds | number | 运行时长（秒） |

### 实现位置

| 文件 | 新增内容 |
|------|----------|
| `internal/service/agent/execution_service.go` | `GetAllRunningAgents()` 方法 |
| `internal/api/invocation_handler.go` | `/invocations/running` 路由 + `ListRunning()` 处理函数 |

### GetAllRunningAgents() 方法逻辑

```go
func (es *ExecutionService) GetAllRunningAgents(ctx context.Context) ([]RunningAgentInfo, error) {
    es.mu.RLock()
    defer es.mu.RUnlock()
    
    var result []RunningAgentInfo
    for _, agent := range es.runningAgents {
        // 通过ThreadID查询Project和Thread信息
        threadCtx, _ := es.threadContexts[agent.ThreadID]
        projectName := ""
        threadTitle := ""
        if threadCtx != nil {
            projectName = threadCtx.Project.Name
            threadTitle = threadCtx.Thread.Title
        }
        
        result = append(result, RunningAgentInfo{
            InvocationID:          agent.InvocationID,
            AgentName:             agent.AgentConfig.Name,
            ProjectName:           projectName,
            ThreadTitle:           threadTitle,
            StartedAt:             agent.StartedAt,
            RunningDurationSeconds: int(time.Since(agent.StartedAt).Seconds()),
        })
    }
    return result, nil
}
```

---

## Part 2: 启动器Agent实例列表

### UI位置

`installer/src/renderer/src/pages/LauncherDashboard.tsx` - 服务状态区域下方新增独立区块

### 组件结构

```
服务状态区域
  ├── 状态文字："服务状态：运行中/已停止"
  ├── [运行中] 按钮：停止服务（禁用/可用）
  └── [已停止] 按钮：启动服务
────────────────────
Agent实例列表区域（新增）
  ├── 标题："正在运行的Agent实例"
  ├── 空状态："当前无Agent实例运行"
  └── 实例列表（表格/列表）：
        │ 项目      │ 任务          │ Agent     │ 运行时间 │
        │ isdp      │ 修复登录bug   │ 代码审查员 │ 15分钟   │
        │ web       │ 添加新功能    │ 开发助手   │ 8分钟    │
  └── 提示文字（有实例时显示）："有X个Agent实例正在运行，请在Web控制台手动停止后才能停止服务"
```

### 停止服务按钮状态

| Agent实例数 | 按钮状态 | 提示信息 |
|-------------|----------|----------|
| 0 | 正常可用 | 无 |
| > 0 | 禁用（灰显） | "有X个Agent实例正在运行，请在Web控制台手动停止后才能停止服务" |

### 数据获取方式

1. 新增IPC handler: `get-running-agents`
2. IPC handler调用HTTP API: `GET http://localhost:{port}/api/v1/invocations/running`
3. 端口从 `data/configs/config.yaml` 读取，默认26305
4. 服务未运行时返回空列表

### 轮询更新

- 服务运行时：每5秒轮询一次
- 服务停止时：停止轮询，显示空列表

### 新增/修改文件

| 文件 | 变更 |
|------|------|
| `installer/src/main/index.ts` | 新增 `ipcMain.handle('get-running-agents', ...)` |
| `installer/src/main/launcher-entry.ts` | 新增同上IPC handler（启动器入口） |
| `installer/src/preload/index.ts` | 新增 `getRunningAgents: () => ipcRenderer.invoke('get-running-agents')` |
| `installer/src/renderer/src/types/index.ts` | 新增 `RunningAgentInstance` 类型定义 |
| `installer/src/renderer/src/pages/LauncherDashboard.tsx` | 新增Agent实例列表组件 + 停止按钮禁用逻辑 |

---

## Part 3: 升级前进程检测

### 检测时机

用户点击"开始升级"按钮时，**先检测再进入升级流程**

### 检测流程

```
点击"开始升级"
    │
    ▼
调用 checkProcessesRunning()
    │
    ├─ 返回运行中的进程列表
    │     │
    │     ▼ 有进程运行
    │   弹窗提示，不允许继续
    │   返回，不执行升级
    │
    └─ 返回空列表
          │
          ▼ 无进程运行
        进入正常升级流程
```

### 检测的进程列表

| 进程名 | 说明 |
|--------|------|
| `colink.exe` | 当前版本启动器 |
| `colink-server.exe` | 当前版本后端服务 |
| `ISDP.exe` | 旧版启动器（兼容升级） |
| `isdp-server.exe` | 旧版后端服务（兼容升级） |

### 检测方式

使用 `tasklist` 命令检测：
```typescript
function checkProcessRunning(processName: string): boolean {
  const output = execSync(`tasklist /fi "imagename eq ${processName}" /fo csv`, { encoding: 'utf8' })
  // CSV格式: "Image Name","PID","Session Name","Session#","Mem Usage"
  // 如果进程不存在，只返回一行标题
  // 如果进程存在，返回多行数据
  const lines = output.trim().split('\n')
  return lines.length > 1
}

function checkProcessesRunning(): string[] {
  const processesToCheck = ['colink.exe', 'colink-server.exe', 'ISDP.exe', 'isdp-server.exe']
  const running: string[] = []
  for (const name of processesToCheck) {
    if (checkProcessRunning(name)) {
      running.push(name)
    }
  }
  return running
}
```

### 错误提示弹窗

**内容:**

```
标题: 无法升级

内容: 检测到以下进程正在运行，请先手动停止后再升级：

      - Colink.exe（启动器）
      - colink-server.exe（后端服务）
      
      请在启动器中停止服务，或手动关闭相关程序后重试。

按钮: [确定]
```

### 新增/修改文件

| 文件 | 变更 |
|------|------|
| `installer/src/main/installer.ts` | 新增 `checkProcessRunning()` 和 `checkProcessesRunning()` 函数 |
| `installer/src/main/index.ts` | 修改 `start-installation` handler，先调用检测再决定是否执行 |

---

## Part 4: 移除自动停止逻辑

### 移除的调用（保留函数定义）

| 文件 | 位置 | 原调用 | 处理 |
|------|------|--------|------|
| `installer/src/main/installer.ts` | `runInstallation()` 函数开头 Step 0 | `await killAllProcesses()` | **移除调用** |
| `installer/src/main/index.ts` | `start-installation` handler | `await stopAllProcesses()` | **移除调用** |

### 保留的函数（仍有用途）

| 函数 | 保留用途 |
|------|----------|
| `killAllProcesses()` | 卸载流程仍需要，卸载前强制停止所有进程 |
| `stopAllProcesses()` | 用户在启动器页面手动点击"停止服务"时调用 |

### 不修改的部分

- 卸载流程 (`uninstall` handler) 继续调用 `stopAllProcesses()` 强制停止
- 启动器页面的"停止服务"按钮逻辑不变，调用 `stop-service` IPC

---

## 数据流图

```
┌─────────────────────────────────────────────────────────────┐
│                    启动器 (LauncherDashboard)                │
│                                                             │
│  [服务状态区域]                                              │
│     └── 停止服务按钮 ←── Agent实例数 > 0 ? 禁用 : 可用       │
│                                                             │
│  [Agent实例列表区域]                                         │
│     ├── 每5秒轮询                                           │
│     ├── IPC: get-running-agents                            │
│     └── 展示: 项目→任务→Agent名称 + 运行时间                 │
└─────────────────────────────────────────────────────────────┘
                         │ IPC调用
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                 Electron Main (index.ts)                    │
│                                                             │
│  ipcMain.handle('get-running-agents')                       │
│     └── HTTP: GET localhost:26305/api/v1/invocations/running│
│     └── 解析config.yaml获取端口                             │
└─────────────────────────────────────────────────────────────┘
                         │ HTTP请求
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                colink-server.exe (Go后端)                   │
│                                                             │
│  GET /api/v1/invocations/running                            │
│     └── ExecutionService.GetAllRunningAgents()              │
│     └── runningAgents map + ThreadContext缓存               │
│     └── 返回JSON列表                                        │
└─────────────────────────────────────────────────────────────┘


┌─────────────────────────────────────────────────────────────┐
│                    安装器 (Setup.exe)                       │
│                                                             │
│  点击"开始升级"                                             │
│     │                                                       │
│     ▼                                                       │
│  checkProcessesRunning()                                    │
│     ├── tasklist检测colink.exe                              │
│     ├── tasklist检测colink-server.exe                       │
│     ├── tasklist检测ISDP.exe                                │
│     ├── tasklist检测isdp-server.exe                         │
│     │                                                       │
│     ├─ 有进程 → 弹窗提示，停止流程                          │
│     │                                                       │
│     └─ 无进程 → 进入正常升级流程                            │
│            (不再调用killAllProcesses)                       │
└─────────────────────────────────────────────────────────────┘
```

---

## 测试要点

### 启动器Agent列表

1. 服务运行中，有Agent实例 → 正确展示列表，停止按钮禁用
2. 服务运行中，无Agent实例 → 显示空状态，停止按钮可用
3. 服务已停止 → 显示空列表，停止轮询

### 升级进程检测

1. colink.exe正在运行 → 弹窗提示，无法升级
2. colink-server.exe正在运行 → 弹窗提示，无法升级
3. 无进程运行 → 正常进入升级流程
4. 升级过程中不再自动终止任何进程

### 兼容性

1. 旧版ISDP.exe/isdp-server.exe进程检测正常
2. 端口从config.yaml读取，非默认端口时仍能正确调用API

---

## 变更文件清单

### Go后端

| 文件 | 变更类型 |
|------|----------|
| `internal/service/agent/execution_service.go` | 新增方法 |
| `internal/service/agent/types.go` | 新增类型 |
| `internal/api/invocation_handler.go` | 新增路由+处理函数 |

### 安装器

| 文件 | 变更类型 |
|------|----------|
| `installer/src/main/index.ts` | 新增IPC handler + 移除自动停止调用 |
| `installer/src/main/launcher-entry.ts` | 新增IPC handler |
| `installer/src/main/installer.ts` | 新增检测函数 + 移除自动停止调用 |
| `installer/src/preload/index.ts` | 新增IPC暴露 |
| `installer/src/renderer/src/types/index.ts` | 新增类型定义 |
| `installer/src/renderer/src/pages/LauncherDashboard.tsx` | 新增UI组件 + 按钮逻辑 |
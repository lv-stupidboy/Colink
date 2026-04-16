# Agent实例展示与升级检测实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在启动器页面展示运行中的Agent实例列表，有Agent运行时禁止停止服务；升级前检测进程是否运行，若运行则报错提示用户手动停止，不再自动终止进程。

**Architecture:** Go后端新增API返回runningAgents数据，安装器通过HTTP调用获取并展示在UI；升级前使用tasklist检测进程状态，有进程运行则弹窗阻止升级。

**Tech Stack:** Go (Gin), TypeScript (Electron), React (Ant Design)

---

## 文件结构

### Go后端

| 文件 | 责任 |
|------|------|
| `internal/service/agent/types.go` | 新增 `RunningAgentInfo` 类型定义 |
| `internal/service/agent/execution_service.go` | 新增 `GetAllRunningAgents()` 方法 |
| `internal/api/invocation_handler.go` | 新增 `/invocations/running` 路由 |

### 安装器

| 文件 | 责任 |
|------|------|
| `installer/src/main/installer.ts` | 新增进程检测函数，移除升级时自动停止调用 |
| `installer/src/main/index.ts` | 新增IPC handler，修改升级流程检测逻辑 |
| `installer/src/main/launcher-entry.ts` | 新增IPC handler（启动器入口） |
| `installer/src/preload/index.ts` | 新增 `getRunningAgents` API暴露 |
| `installer/src/renderer/src/types/index.ts` | 新增 `RunningAgentInstance` 类型 |
| `installer/src/renderer/src/pages/LauncherDashboard.tsx` | 新增Agent实例列表UI |

---

## Task 1: Go后端 - 新增类型定义

**Files:**
- Modify: `internal/service/agent/types.go`

- [ ] **Step 1: 添加 RunningAgentInfo 类型**

在 `internal/service/agent/types.go` 文件末尾添加：

```go
// RunningAgentInfo 运行中的Agent信息（用于API返回）
type RunningAgentInfo struct {
	InvocationID          uuid.UUID `json:"invocationId"`
	AgentName             string    `json:"agentName"`
	ProjectName           string    `json:"projectName"`
	ThreadTitle           string    `json:"threadTitle"`
	StartedAt             time.Time `json:"startedAt"`
	RunningDurationSeconds int       `json:"runningDurationSeconds"`
}
```

需要添加import: `"time"`

- [ ] **Step 2: 验证编译通过**

Run: `go build ./internal/service/agent`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add internal/service/agent/types.go
git commit -m "feat(agent): add RunningAgentInfo type for API"
```

---

## Task 2: Go后端 - 新增 GetAllRunningAgents 方法

**Files:**
- Modify: `internal/service/agent/execution_service.go`

- [ ] **Step 1: 添加 GetAllRunningAgents 方法**

在 `execution_service.go` 文件末尾添加：

```go
// GetAllRunningAgents 获取所有运行中的Agent信息
func (es *ExecutionService) GetAllRunningAgents(ctx context.Context) ([]RunningAgentInfo, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var result []RunningAgentInfo
	for _, agent := range es.runningAgents {
		// 通过ThreadContext缓存获取Project和Thread信息
		threadCtx, exists := es.threadContexts[agent.ThreadID]
		projectName := ""
		threadTitle := ""
		if exists && threadCtx != nil {
			if threadCtx.Project != nil {
				projectName = threadCtx.Project.Name
			}
			if threadCtx.Thread != nil {
				threadTitle = threadCtx.Thread.Title
			}
		}

		result = append(result, RunningAgentInfo{
			InvocationID:           agent.InvocationID,
			AgentName:              agent.AgentConfig.Name,
			ProjectName:            projectName,
			ThreadTitle:            threadTitle,
			StartedAt:              agent.StartedAt,
			RunningDurationSeconds: int(time.Since(agent.StartedAt).Seconds()),
		})
	}
	return result, nil
}
```

- [ ] **Step 2: 验证编译通过**

Run: `go build ./internal/service/agent`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add internal/service/agent/execution_service.go
git commit -m "feat(agent): add GetAllRunningAgents method"
```

---

## Task 3: Go后端 - 新增 API 路由

**Files:**
- Modify: `internal/api/invocation_handler.go`

- [ ] **Step 1: 添加 ListRunning 处理函数**

在 `invocation_handler.go` 的 `ListByThread` 函数后添加：

```go
// ListRunning 获取所有运行中的Agent实例
func (h *InvocationHandler) ListRunning(c *gin.Context) {
	instances, err := h.orchestrator.GetExecutionService().GetAllRunningAgents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if instances == nil {
		instances = []agent.RunningAgentInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"instances": instances})
}
```

- [ ] **Step 2: 注册路由**

在 `RegisterRoutes` 函数的 `invocations` group中添加：

```go
invocations := r.Group("/invocations")
{
	invocations.GET("/running", h.ListRunning)  // 新增：获取运行中的Agent
	invocations.GET("/:id", h.Get)
	invocations.POST("/:id/cancel", h.Cancel)
}
```

- [ ] **Step 3: 验证编译通过**

Run: `go build ./cmd/server`
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
git add internal/api/invocation_handler.go
git commit -m "feat(api): add /invocations/running endpoint"
```

---

## Task 4: 安装器 - 新增进程检测函数

**Files:**
- Modify: `installer/src/main/installer.ts`

- [ ] **Step 1: 添加进程检测函数**

在 `killAllProcesses` 函数之前添加：

```typescript
// 检测单个进程是否正在运行
export function checkProcessRunning(processName: string): boolean {
  try {
    const output = execSync(`tasklist /fi "imagename eq ${processName}" /fo csv`, { encoding: 'utf8' })
    // CSV格式: "Image Name","PID","Session Name","Session#","Mem Usage"
    // 如果进程不存在，只返回一行标题
    // 如果进程存在，返回多行数据
    const lines = output.trim().split('\n')
    // 过滤掉空行和只包含标题的行
    const dataLines = lines.filter(line => line.trim() && !line.includes('"Image Name"'))
    return dataLines.length > 0
  } catch {
    return false
  }
}

// 检测所有相关进程是否正在运行，返回运行中的进程列表
export function checkProcessesRunning(): string[] {
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

- [ ] **Step 2: 验证TypeScript编译通过**

Run: `cd installer && npm run build`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add installer/src/main/installer.ts
git commit -m "feat(installer): add process detection functions"
```

---

## Task 5: 安装器 - 移除升级时自动停止调用

**Files:**
- Modify: `installer/src/main/installer.ts:762-766`
- Modify: `installer/src/main/index.ts:385-388`

- [ ] **Step 1: 移除 installer.ts 中 runInstallation 的 killAllProcesses 调用**

将 `installer/src/main/installer.ts` 中第762-766行的Step 0删除：

```typescript
// 原代码（删除）:
//    // Step 0: 停止所有进程
//    sendProgress('prepare', 'running', 0, '停止服务...', '正在停止所有相关进程...')
//    await killAllProcesses()
//    sendProgress('prepare', 'success', 100, '服务已停止', '所有进程已停止')

// 替换为注释说明:
    // 注意：进程检测已在 start-installation handler 中前置完成
    // 此处不再自动终止进程，避免僵尸文件问题
```

- [ ] **Step 2: 移除 index.ts 中 start-installation handler 的 stopAllProcesses 调用**

修改 `installer/src/main/index.ts` 第385-388行：

```typescript
// 原代码:
ipcMain.handle('start-installation', async (_event, config) => {
  // 安装前停止所有进程
  await stopAllProcesses()
  ...
})

// 改为:
ipcMain.handle('start-installation', async (_event, config) => {
  // 升级前检测进程是否运行，若运行则报错提示用户手动停止
  const runningProcesses = checkProcessesRunning()
  if (runningProcesses.length > 0) {
    // 弹窗提示用户
    const processList = runningProcesses.map(p => `- ${p}`).join('\n')
    dialog.showMessageBox(mainWindow!, {
      type: 'error',
      title: '无法升级',
      message: '检测到以下进程正在运行，请先手动停止后再升级：',
      detail: `${processList}\n\n请在启动器中停止服务，或手动关闭相关程序后重试。`,
      buttons: ['确定'],
    })
    return { success: false, error: '进程正在运行，请先手动停止' }
  }

  const sourceDir = getExeDir()
  ...
})
```

需要在文件顶部添加 `checkProcessesRunning` 的导入：
```typescript
import { checkProcessesRunning, ... } from './installer'
```

- [ ] **Step 3: 验证编译通过**

Run: `cd installer && npm run build`
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
git add installer/src/main/installer.ts installer/src/main/index.ts
git commit -m "fix(installer): remove auto-kill on upgrade, add process check"
```

---

## Task 6: 安装器 - 新增 preload API

**Files:**
- Modify: `installer/src/preload/index.ts`

- [ ] **Step 1: 添加 getRunningAgents API**

在 `contextBridge.exposeInMainWorld` 中添加：

```typescript
// 服务管理 - 新增
getRunningAgents: () => ipcRenderer.invoke('get-running-agents'),
```

- [ ] **Step 2: Commit**

```bash
git add installer/src/preload/index.ts
git commit -m "feat(preload): add getRunningAgents API"
```

---

## Task 7: 安装器 - 新增类型定义

**Files:**
- Modify: `installer/src/renderer/src/types/index.ts`

- [ ] **Step 1: 添加 RunningAgentInstance 类型**

在类型定义文件中添加：

```typescript
// 运行中的Agent实例
export interface RunningAgentInstance {
  invocationId: string
  agentName: string
  projectName: string
  threadTitle: string
  startedAt: string
  runningDurationSeconds: number
}
```

- [ ] **Step 2: 更新 Window.electronAPI 类型声明**

在 `electronAPI` 接口中添加：

```typescript
// 服务管理 - 新增
getRunningAgents: () => Promise<{ instances: RunningAgentInstance[] }>
```

- [ ] **Step 3: Commit**

```bash
git add installer/src/renderer/src/types/index.ts
git commit -m "feat(types): add RunningAgentInstance type"
```

---

## Task 8: 安装器 - 新增 IPC handler (index.ts)

**Files:**
- Modify: `installer/src/main/index.ts`

- [ ] **Step 1: 添加 get-running-agents IPC handler**

在 `ipcMain.handle('get-service-status', ...)` 之后添加：

```typescript
ipcMain.handle('get-running-agents', async () => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { instances: [] }
  }

  // 尝试从配置文件读取端口
  let port = 26305 // 默认端口
  try {
    const configPath = join(installed.installDir, 'data', 'configs', 'config.yaml')
    if (existsSync(configPath)) {
      const content = await readFile(configPath, 'utf-8')
      const portMatch = content.match(/port:\s*(\d+)/)
      if (portMatch) {
        port = parseInt(portMatch[1])
      }
    }
  } catch (e) {
    console.warn('[GetRunningAgents] Failed to read config:', e)
  }

  // 调用 HTTP API 获取运行中的Agent
  try {
    const response = await fetch(`http://localhost:${port}/api/v1/invocations/running`, {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' },
      signal: AbortSignal.timeout(5000) // 5秒超时
    })
    if (!response.ok) {
      return { instances: [] }
    }
    const data = await response.json()
    return data
  } catch (e) {
    // 服务未运行或请求失败，返回空列表
    console.warn('[GetRunningAgents] Failed to fetch:', e)
    return { instances: [] }
  }
})
```

需要添加import:
```typescript
import { readFile } from 'fs/promises'
```

注意：Electron Node环境可能没有全局fetch，需要使用http模块或axios。改用原生http模块：

```typescript
ipcMain.handle('get-running-agents', async () => {
  const installed = getInstalledVersion()
  if (!installed.installed || !installed.installDir) {
    return { instances: [] }
  }

  // 尝试从配置文件读取端口
  let port = 26305
  try {
    const configPath = join(installed.installDir, 'data', 'configs', 'config.yaml')
    if (existsSync(configPath)) {
      const content = await readFile(configPath, 'utf-8')
      const portMatch = content.match(/port:\s*(\d+)/)
      if (portMatch) {
        port = parseInt(portMatch[1])
      }
    }
  } catch (e) {
    console.warn('[GetRunningAgents] Failed to read config:', e)
  }

  // 使用http模块调用API
  return new Promise((resolve) => {
    const req = http.request({
      hostname: 'localhost',
      port: port,
      path: '/api/v1/invocations/running',
      method: 'GET',
      timeout: 5000
    }, (res) => {
      let data = ''
      res.on('data', chunk => data += chunk)
      res.on('end', () => {
        try {
          const result = JSON.parse(data)
          resolve(result)
        } catch {
          resolve({ instances: [] })
        }
      })
    })
    req.on('error', () => resolve({ instances: [] }))
    req.on('timeout', () => {
      req.destroy()
      resolve({ instances: [] })
    })
    req.end()
  })
})
```

需要import http模块：
```typescript
import http from 'http'
```

- [ ] **Step 2: 验证编译通过**

Run: `cd installer && npm run build`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add installer/src/main/index.ts
git commit -m "feat(installer): add get-running-agents IPC handler"
```

---

## Task 9: 安装器 - 新增 IPC handler (launcher-entry.ts)

**Files:**
- Modify: `installer/src/main/launcher-entry.ts`

- [ ] **Step 1: 添加 get-running-agents IPC handler**

在 `launcher-entry.ts` 中添加相同的handler（复制index.ts中的逻辑）：

```typescript
ipcMain.handle('get-running-agents', async () => {
  if (!installDir) {
    return { instances: [] }
  }

  // 尝试从配置文件读取端口
  let port = 26305
  try {
    const configPath = join(installDir, 'data', 'configs', 'config.yaml')
    if (existsSync(configPath)) {
      const content = readFileSync(configPath, 'utf-8')
      const portMatch = content.match(/port:\s*(\d+)/)
      if (portMatch) {
        port = parseInt(portMatch[1])
      }
    }
  } catch (e) {
    console.warn('[Launcher] Failed to read config port:', e)
  }

  // 使用http模块调用API
  return new Promise((resolve) => {
    const req = http.request({
      hostname: 'localhost',
      port: port,
      path: '/api/v1/invocations/running',
      method: 'GET',
      timeout: 5000
    }, (res) => {
      let data = ''
      res.on('data', chunk => data += chunk)
      res.on('end', () => {
        try {
          const result = JSON.parse(data)
          resolve(result)
        } catch {
          resolve({ instances: [] })
        }
      })
    })
    req.on('error', () => resolve({ instances: [] }))
    req.on('timeout', () => {
      req.destroy()
      resolve({ instances: [] })
    })
    req.end()
  })
})
```

需要添加imports:
```typescript
import http from 'http'
import { readFileSync } from 'fs'
```

- [ ] **Step 2: 验证编译通过**

Run: `cd installer && npm run build`
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
git add installer/src/main/launcher-entry.ts
git commit -m "feat(launcher): add get-running-agents IPC handler"
```

---

## Task 10: 安装器 - 新增 LauncherDashboard UI

**Files:**
- Modify: `installer/src/renderer/src/pages/LauncherDashboard.tsx`

- [ ] **Step 1: 添加状态管理和轮询逻辑**

在组件顶部添加：

```typescript
import { useEffect, useState } from 'react'
import { Table, Alert } from 'antd'

// 新增状态
const [runningAgents, setRunningAgents] = useState<RunningAgentInstance[]>([])
const [agentCount, setAgentCount] = useState(0)

// 轮询获取运行中的Agent
useEffect(() => {
  if (!isRunning) {
    setRunningAgents([])
    setAgentCount(0)
    return
  }

  const fetchRunningAgents = async () => {
    try {
      const result = await window.electronAPI.getRunningAgents()
      setRunningAgents(result.instances || [])
      setAgentCount(result.instances?.length || 0)
    } catch {
      setRunningAgents([])
      setAgentCount(0)
    }
  }

  fetchRunningAgents()
  const interval = setInterval(fetchRunningAgents, 5000) // 每5秒轮询
  return () => clearInterval(interval)
}, [isRunning])
```

- [ ] **Step 2: 添加Agent实例列表UI**

在服务状态Card之后添加新的Card：

```tsx
{/* Agent实例列表 */}
<Card title="正在运行的Agent实例" size="small" style={{ marginBottom: 16 }}>
  {runningAgents.length === 0 ? (
    <Text type="secondary">当前无Agent实例运行</Text>
  ) : (
    <>
      <Table
        size="small"
        dataSource={runningAgents}
        rowKey="invocationId"
        pagination={false}
        columns={[
          { title: '项目', dataIndex: 'projectName', key: 'project', width: 100 },
          { title: '任务', dataIndex: 'threadTitle', key: 'thread', width: 150 },
          { title: 'Agent', dataIndex: 'agentName', key: 'agent', width: 120 },
          {
            title: '运行时间',
            key: 'duration',
            width: 80,
            render: (_, record) => {
              const mins = Math.floor(record.runningDurationSeconds / 60)
              if (mins < 1) return '<1分钟'
              if (mins < 60) return `${mins}分钟`
              const hours = Math.floor(mins / 60)
              const remainMins = mins % 60
              return `${hours}小时${remainMins}分钟`
            }
          },
        ]}
      />
      <Alert
        type="warning"
        showIcon
        style={{ marginTop: 8 }}
        message={`有${agentCount}个Agent实例正在运行，请在Web控制台手动停止后才能停止服务`}
      />
    </>
  )}
</Card>
```

- [ ] **Step 3: 修改停止服务按钮逻辑**

将停止服务按钮改为根据agentCount禁用：

```tsx
{isRunning ? (
  <Button
    icon={<StopOutlined />}
    onClick={onStopService}
    danger
    disabled={agentCount > 0}
  >
    停止服务
  </Button>
) : (
  <Button
    type="primary"
    icon={<PlayCircleOutlined />}
    onClick={onStartService}
  >
    启动服务
  </Button>
)}
```

- [ ] **Step 4: 导入 RunningAgentInstance 类型**

在文件顶部添加：
```typescript
import type { RunningAgentInstance } from '../types'
```

- [ ] **Step 5: 验证编译通过**

Run: `cd installer && npm run build`
Expected: 无错误输出

- [ ] **Step 6: Commit**

```bash
git add installer/src/renderer/src/pages/LauncherDashboard.tsx
git commit -m "feat(launcher): add Agent instance list UI"
```

---

## Task 11: 集成测试与验证

- [ ] **Step 1: 启动Go后端服务**

Run: `go run ./cmd/server`
Expected: 服务启动在端口26305

- [ ] **Step 2: 测试Go API**

Run: `curl http://localhost:26305/api/v1/invocations/running`
Expected: 返回 `{ "instances": [] }` 或包含运行Agent的列表

- [ ] **Step 3: 启动安装器开发模式**

Run: `cd installer && npm run dev`
Expected: Electron窗口启动

- [ ] **Step 4: 验证UI显示**

手动验证：
- 服务停止时，Agent列表显示"当前无Agent实例运行"
- 启动服务后，列表每5秒更新
- 有Agent运行时，停止按钮禁用并显示警告

- [ ] **Step 5: 验证升级检测**

手动验证：
- 保持colink.exe或colink-server.exe运行
- 点击升级按钮
- 预期：弹窗提示"无法升级"，显示运行中的进程列表

---

## Task 12: 最终提交

- [ ] **Step 1: 确认所有文件已提交**

Run: `git status`
Expected: 无未提交文件

- [ ] **Step 2: 最终commit（如有遗漏）**

```bash
git add -A
git commit -m "feat: Agent实例展示与升级检测功能完成"
```

---

## 变更文件清单确认

| 文件 | 变更类型 | Task |
|------|----------|------|
| `internal/service/agent/types.go` | 新增类型 | Task 1 |
| `internal/service/agent/execution_service.go` | 新增方法 | Task 2 |
| `internal/api/invocation_handler.go` | 新增路由 | Task 3 |
| `installer/src/main/installer.ts` | 新增检测函数 + 移除调用 | Task 4, 5 |
| `installer/src/main/index.ts` | 新增IPC + 修改升级逻辑 | Task 5, 8 |
| `installer/src/main/launcher-entry.ts` | 新增IPC | Task 9 |
| `installer/src/preload/index.ts` | 新增API暴露 | Task 6 |
| `installer/src/renderer/src/types/index.ts` | 新增类型 | Task 7 |
| `installer/src/renderer/src/pages/LauncherDashboard.tsx` | 新增UI | Task 10 |
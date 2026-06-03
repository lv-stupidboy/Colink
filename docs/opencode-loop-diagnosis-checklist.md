# OpenCode 无限循环问题诊断清单

**问题 ID**: BUG-OPENCODE-LOOP-001  
**创建日期**: 2026-06-02  
**最后更新**: 2026-06-02

## 问题现象

- [ ] 重复发送相同文本内容
- [ ] 无限循环不停止
- [ ] 需要手动刷新页面或重启后端才能停止
- [ ] 前端收到大量相同消息
- [ ] 后端日志显示重复的 notification

## 诊断步骤

### Step 1: 查看实时日志

```bash
tail -f data/logs/*.log | grep -E "ACP: notification|duplicate|cleanup"
```

**关键指标**:
- `notificationCount` 是否异常增长（正常应该 <100）
- `duplicateUpdateCount` 是否 >5（表示有重复）
- `cleanup` 是否被调用（正常应该调用）
- 进程退出时间（cleanup → process exited normally 应 <5秒）

**正常模式**:
```
ACP: notification count (count: 50)
ACP: tool_call_update completed
ACP: cleanup called (notificationCount: 50, outputLen: 1024)
ACP: process exited normally
```

**异常模式 A - 重复通知**:
```
ACP: notification count (count: 200)
ACP: duplicate session/update detected (duplicateCount: 10)
ACP: notification count (count: 300)
ACP: notification count (count: 400)
... (没有 cleanup)
```

**异常模式 B - 进程卡住**:
```
ACP: notification count (count: 150)
... (长时间无新日志)
ACP: cleanup called
ACP: process still running, terminating process tree
ACP: process tree terminated
... (但 notification 仍在继续)
```

### Step 2: 检查进程状态

```bash
./scripts/quick-diagnose.sh
```

**关键指标**:
- 进程是否仍在运行（应该已退出）
- 进程状态（State: S=sleeping, R=running, Z=zombie）
- stdout/stderr fd 是否仍打开
- 通知频率（每秒多少条）

**进程状态解读**:
- `S (sleeping)`: 正常，等待 I/O
- `R (running)`: 异常，如果长时间 R 状态表示卡住
- `Z (zombie)`: 异常，父进程未调用 wait()
- `T (stopped)`: 被信号停止

**Windows 特殊检查**:
```powershell
# 查看进程
tasklist /FI "IMAGENAME eq opencode.exe"

# 查看进程树
wmic process where "name='opencode.exe'" get processid,parentprocessid

# 强制终止
taskkill /F /T /PID <pid>
```

### Step 3: 检查 WebSocket 状态

前端浏览器控制台执行：

```javascript
// 查看最后收到的消息
console.log('[WS] Last message:', window.__lastWSMessage)

// 统计消息接收频率
let msgCount = 0
let startTime = Date.now()
const originalOnMessage = window.__ws.onmessage
window.__ws.onmessage = (e) => {
    msgCount++
    const elapsed = (Date.now() - startTime) / 1000
    if (msgCount % 10 == 0) {
        console.log(`[WS] Received ${msgCount} messages in ${elapsed}s (${msgCount/elapsed} msg/s)`)
    }
    originalOnMessage(e)
}

// 检查消息内容是否重复
let lastContent = ''
window.__ws.onmessage = (e) => {
    const data = JSON.parse(e.data)
    if (data.chunk === lastContent) {
        console.warn('[WS] Duplicate message detected:', data.chunk.substring(0, 100))
    }
    lastContent = data.chunk
}
```

**异常指标**:
- 消息频率 >10 msg/s（正常应该 <5）
- 连续收到完全相同的 chunk 内容
- chunkType 为 text 且内容重复

### Step 4: 检查后端状态

```bash
# 查看运行中的 agent（如果有 API）
curl http://localhost:8080/api/v1/agents/running

# 查看特定 invocation 状态
curl http://localhost:8080/api/v1/threads/{threadId}/invocations/{invocationId}

# 查看数据库中的 invocation 状态
# MySQL:
mysql -u root -p isdp -e "SELECT id, status, created_at FROM invocations WHERE status='running' ORDER BY created_at DESC LIMIT 10;"

# SQLite:
sqlite3 data/isdp.db "SELECT id, status, created_at FROM invocations WHERE status='running' ORDER BY created_at DESC LIMIT 10;"
```

**关键指标**:
- invocation 状态是否为 `completed`
- 是否有多个 invocation 同时 running（正常应该只有一个）
- invocation 的 created_at 时间（如果 >10 分钟前，可能卡住）

### Step 5: 分析日志模式

运行完整诊断：

```bash
./scripts/diagnose-opencode-loop.sh
```

查看关键日志段：

```bash
# 提取问题发生前后的日志
tail -500 data/logs/*.log | grep -A 5 -B 5 "duplicate session/update"

# 查看进程退出时间线
tail -500 data/logs/*.log | grep -E "cleanup|process exited|process still running"

# 查看 stderr 内容（可能的错误线索）
tail -500 data/logs/*.log | grep "ACP: stderr output"
```

## 根因定位

### 模式 A: OpenCode CLI 陷入循环

**证据**:
- `notificationCount` 异常增长（>200）
- `duplicateUpdateCount` >5
- `cleanup` 未被调用或延迟调用
- 进程仍在运行（State=R）
- stdout fd 打开

**原因**: 
- OpenCode CLI stdout 未正确关闭
- OpenCode CLI 内部循环 bug
- 配置错误导致 CLI 异常行为

**临时解决**: 
```bash
# 强制终止进程
kill -9 $(pgrep -f "opencode")

# Windows:
taskkill /F /T /PID $(pgrep -f "opencode")
```

**长期修复**:
1. 添加文本消息去重机制（adapter_base.go）
2. 强制关闭 stdout（jsonrpc.go Close()）
3. 添加重复检测和自动终止（连续 N 重复自动 kill）

### 模式 B: 进程未正确终止

**证据**:
- cleanup 调用但进程仍在运行
- `process still running, terminating process tree` 日志
- 进程状态为 R 或 Z
- killProcessTree 后进程仍不退出

**原因**:
- cleanup 的 killProcessTree 未生效
- Windows taskkill 失败
- 子进程（如 bun）未被终止

**临时解决**:
```bash
# 查看进程树
ps aux | grep opencode
ps aux | grep bun

# 终止整个进程树
pkill -9 -f "opencode"
pkill -9 -f "bun"
```

**长期修复**:
1. 改进 killProcessTree 实现
2. 添加进程监控 goroutine（cmd.Wait）
3. cleanup 中强制关闭 stdout/stdin

### 模式 C: 消息解析问题

**证据**:
- 日志显示 JSON 解析错误
- content 格式异常（非标准 OpenCode 格式）
- stderr 包含错误信息

**原因**:
- OpenCode 消息格式变化
- ACP 协议版本不兼容
- 解析器未正确处理嵌套结构

**临时解决**:
- 检查 OpenCode CLI 版本
- 查看 stderr 输出的具体错误

**长期修复**:
1. 更新 event_parser.go 支持新格式
2. 添加更详细的解析错误日志
3. stderr 错误自动上报

### 模式 D: WebSocket 连接问题

**证据**:
- WebSocket 未正确关闭
- 前端持续接收消息但后端已停止
- 日志显示 "Connection closed" 但消息仍在发送

**原因**:
- WebSocket 关闭码不正确（如 1005）
- Hub 未正确清理客户端
- broadcast channel 阻塞

**临时解决**:
- 刷新页面重新连接
- 检查 WebSocket handler 关闭逻辑

**长期修复**:
1. 改进 WebSocket 关闭码处理（已修复 1005 问题）
2. Hub 添加客户端超时清理
3. broadcast channel 添加满队列保护

## 数据收集清单

问题发生时，请收集以下数据：

### 必需数据（优先级 P0）

1. **完整日志文件**（最后 500-1000 行）
   ```bash
   tail -1000 data/logs/*.log > /tmp/opencode-loop-logs.txt
   ```

2. **进程状态截图**
   ```bash
   # Unix
   ps aux | grep opencode > /tmp/process-status.txt
   cat /proc/$(pgrep -f opencode)/status > /tmp/process-state.txt
   
   # Windows
   tasklist /V > /tmp/process-status.txt
   ```

3. **前端控制台输出**
   - 浏览器 F12 → Console
   - 复制所有 `[WS]` 相关消息
   - 消息频率统计

### 重要数据（优先级 P1）

4. **invocation 状态**
   ```bash
   curl http://localhost:8080/api/v1/threads/{threadId}/invocations/{invocationId} > /tmp/invocation-state.json
   ```

5. **stderr 内容**
   ```bash
   tail -200 data/logs/*.log | grep "ACP: stderr output" > /tmp/stderr-content.txt
   ```

6. **OpenCode CLI 版本**
   ```bash
   opencode --version 2>&1 || echo "version check failed"
   ```

### 辅助数据（优先级 P2）

7. **配置文件**
   ```bash
   cp configs/config.yaml /tmp/config.yaml.backup
   ```

8. **Agent 配置**
   - BaseAgent type (open_code)
   - Default model
   - Skills/Commands/Subagents 绑定

9. **对话上下文**
   - Thread ID
   - 用户输入的问题内容
   - Agent 开始回复的时间

## Bug Report 模板

提交 bug report 时请包含：

```
**标题**: OpenCode 无限循环问题 - [简短描述]

**环境**:
- OpenCode CLI 版本: [从 opencode --version 获取]
- Colink 版本: [从 VERSION 文件获取]
- 操作系统: [Windows/Linux/Mac]
- Node/bun 版本: [如果有]

**问题描述**:
[描述现象，参考"问题现象"清单]

**诊断结果**:
[粘贴 ./scripts/quick-diagnose.sh 输出]

**日志片段**:
[粘贴关键日志，包含重复通知、cleanup、进程退出等]

**进程状态**:
[粘贴 ps/tasklist 输出]

**前端观察**:
[粘贴浏览器控制台输出]

**复现步骤**:
1. [步骤 1]
2. [步骤 2]
3. [步骤 3]

**临时解决方法**:
[描述如何停止循环，如刷新页面/kill进程]

**期望行为**:
[描述正常情况下应该如何]

**附件**:
- [完整日志文件]
- [invocation 状态 JSON]
- [stderr 内容]
```

## 预防措施

### 开发阶段

1. **添加日志增强**（临时，问题解决后可移除）
   - 位置: `adapter_base.go:566-617`
   - 添加 notification 计数和 duplicate 检测
   - 参考: docs/opencode-loop-diagnosis-checklist.md#日志增强示例

2. **改进 cleanup 逻辑**
   - cleanup 中立即关闭 stdin/stdout
   - 添加进程退出超时强制 kill
   - 添加详细的进程状态日志

3. **添加去重机制**
   - 记录最后一次文本内容
   - 连续重复 >5 次 log warning
   - 连续重复 >10 次 自动终止进程

### 测试阶段

1. **压力测试**
   - 长时间对话（>50 轮）
   - 大量工具调用（>20 个）
   - 复杂技能链执行

2. **边界测试**
   - OpenCode CLI 异常退出
   - 网络断开重连
   - 取消执行

3. **日志监控**
   - 开启 debug 日志级别
   - 监控 notificationCount
   - 监控 duplicateUpdateCount

### 生产阶段

1. **日志告警**
   - duplicateUpdateCount >5 → Warning
   - notificationCount >200 → Error
   - cleanup 未调用 → Alert

2. **进程监控**
   - OpenCode 进程运行时间 >10 分钟 → Alert
   - 进程状态为 Z (zombie) → Alert

3. **自动恢复**
   - 检测到循环自动终止进程
   - 检测到异常自动重启 invocation

## 日志增强示例（临时添加）

**注意**: 这些代码修改仅用于诊断，问题解决后应移除。

### adapter_base.go:566 (handleNotification 入口)

```go
// 在 handleNotification 函数开头添加
func (a *BaseACPAdapter) handleNotification(session *acpSession, method string, params json.RawMessage, onChunk func(agent.Chunk)) {
    // === 临时诊断代码 - START ===
    session.mu.Lock()
    session.notificationCount++
    if session.notificationCount%10 == 0 {
        LogInfo("ACP: notification count (TEMP DIAG)",
            zap.String("sessionId", session.id),
            zap.Int("count", session.notificationCount),
            zap.String("method", method))
    }
    session.mu.Unlock()
    // === 临时诊断代码 - END ===
    
    LogDebug("ACP: received notification", ...)
    
    switch method {
    case "session/update":
        // === 临时诊断代码 - START ===
        contentHash := fmt.Sprintf("%x", sha256.Sum256(params))[:16]
        session.mu.Lock()
        if session.lastUpdateHash == contentHash {
            session.duplicateUpdateCount++
            LogWarn("ACP: duplicate session/update detected (TEMP DIAG)",
                zap.String("sessionId", session.id),
                zap.String("hash", contentHash),
                zap.Int("duplicateCount", session.duplicateUpdateCount))
        } else {
            session.duplicateUpdateCount = 0
            session.lastUpdateHash = contentHash
        }
        session.mu.Unlock()
        // === 临时诊断代码 - END ===
        
        ...
    }
}
```

### adapter_base.go:37-50 (acpSession 结构体)

```go
type acpSession struct {
    id              string
    isdpID          string
    transport       *acpTransport
    cmd             *exec.Cmd
    ctx             context.Context
    cancel          context.CancelFunc
    status          agent.SessionStatus
    output          strings.Builder
    stderrOutput    strings.Builder
    pendingQuestion *agent.Chunk
    thoughtChunkCount int
    // === 临时诊断字段 - START ===
    notificationCount    int       // 收到的通知总数
    duplicateUpdateCount int       // 重复通知计数
    lastUpdateHash       string    // 最后一次 update 的哈希
    lastTextContent      string    // 最后一次文本内容
    // === 临时诊断字段 - END ===
    mu              sync.Mutex
}
```

### adapter_base.go:873 (cleanup 函数)

```go
func (a *BaseACPAdapter) cleanup(session *acpSession) {
    // === 临时诊断代码 - START ===
    session.mu.Lock()
    LogInfo("ACP: cleanup called (TEMP DIAG)",
        zap.String("sessionId", session.id),
        zap.Int("notificationCount", session.notificationCount),
        zap.Int("outputLen", session.output.Len()),
        zap.Int("duplicateUpdateCount", session.duplicateUpdateCount))
    session.mu.Unlock()
    // === 临时诊断代码 - END ===
    
    if session.transport != nil {
        session.transport.Close()
    }
    ...
}
```

## 参考资料

- ACP stderr 处理改进: `docs/ACP-stderr实现记录-20260602-1630.md`
- 进程终止修复: commit `5b47b6b`
- tool_use 去重修复: commit `c658018`, `cc3f9c1`
- WebSocket 关闭码修复: commit `2929c58`
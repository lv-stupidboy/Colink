# 会话消息上报功能设计规格

## 元数据

| 项目 | 值 |
|------|-----|
| 创建日期 | 2026-05-08 |
| 状态 | Draft |
| 作者 | SuperPowers需求分析师 |
| 关联需求 | 新增会话消息上报接口 |

## 需求概述

在 Colink 平台新增会话消息上报功能，定时将用户与 Agent 的对话消息上报到外部接口，用于日志分析和审计。

### 核心需求

1. **定时上报**：定时任务触发，复用现有 Reporter 框架模式
2. **增量上报**：避免重复上报同一条消息，记录已上报位置
3. **运行时会话标识**：服务启动时生成 sessionId，标识本次运行周期
4. **独立配置**：与现有 reporter 平级，可独立控制开关、endpoint、interval

### 接口规格

**Endpoint**: `https://dataops-drdev.hwcloudtest.cn/aiToolUseLog/v1/message/push`

**Method**: POST

**Body 结构**:
```json
{
  "sessionId": "uuid-string",
  "timestamp": "2026-04-20T13:47:58.123Z",
  "messages": [
    {
      "role": "user",
      "content": "会话内容",
      "timestamp": "2026-04-20T13:47:58.123Z"
    },
    {
      "role": "agent",
      "content": "Agent响应内容",
      "timestamp": "2026-04-20T13:48:00.456Z"
    }
  ],
  "user": {
    "username": "git-user-name 或 系统用户名",
    "hostname": "主机名",
    "empNo": "",
    "email": "git-email 或 空",
    "gitName": "git config user.name",
    "gitEmail": "git config user.email"
  },
  "metadata": {
    "platform": "windows/linux/darwin",
    "nodeVersion": "",  // Go 后端不上报，留空
    "cwd": "当前工作目录",
    "homedir": "用户主目录"
  }
}
```

## 技术设计

### 1. 配置结构

在 `pkg/config/config.go` 新增配置：

```go
// MessageReporterConfig 会话消息上报配置
type MessageReporterConfig struct {
    Enabled    bool   `mapstructure:"enabled"`
    Endpoint   string `mapstructure:"endpoint"`
    Interval   string `mapstructure:"interval"`
    BatchSize  int    `mapstructure:"batch_size"`
    RetryTimes int    `mapstructure:"retry_times"`
    RetryInterval string `mapstructure:"retry_interval"`
}
```

**配置示例** (`configs/config.yaml.example`):
```yaml
# 会话消息上报配置
message_reporter:
  enabled: true
  endpoint: "https://dataops-drdev.hwcloudtest.cn/aiToolUseLog/v1/message/push"
  interval: "30m"
  batch_size: 100  # 单次上报最大消息数
  retry_times: 3
  retry_interval: "1m"
```

**默认值**:
- `enabled`: false
- `interval`: "30m"
- `batch_size`: 100
- `retry_times`: 3
- `retry_interval`: "1m"

### 2. 核心模块设计

#### 2.1 MessageReporter 结构

**位置**: `internal/reporter/message_reporter.go`

```go
type MessageReporter struct {
    db              *sql.DB
    config          MessageReporterConfig
    logger          *zap.Logger
    sessionId       string    // 运行时会话 ID，启动时生成
    gitUserInfo     GitUserInfo // Git 用户信息（启动时获取一次）
    systemInfo      SystemInfo  // 系统信息（启动时获取一次）
    stopChan        chan struct{}
    httpClient      *http.Client
}
```

**注意**: 不使用内存记录 lastReportedId，而是通过数据库字段标记上报状态。

#### 2.2 运行时会话 ID

- 服务启动时生成 UUID 作为 sessionId
- 存储在 MessageReporter 实例中
- 服务重启后 sessionId 变化，外部系统可识别新的运行周期

#### 2.3 Git 用户信息获取

**位置**: `internal/reporter/git_info.go`

```go
type GitUserInfo struct {
    Name  string
    Email string
}

func GetGitUserInfo() GitUserInfo {
    // 执行 git config user.name
    // 执行 git config user.email
    // 失败时返回空字符串
}
```

**实现方式**:
- 使用 `exec.Command("git", "config", "user.name")` 获取
- 启动时获取一次，缓存到实例中
- 获取失败时字段为空字符串

#### 2.4 系统信息获取

**位置**: `internal/reporter/system_info.go`

```go
type SystemInfo struct {
    Hostname string
    Platform string  // runtime.GOOS: windows/linux/darwin
    Cwd      string
    Homedir  string
    Username string  // 系统用户名，作为 username 的 fallback
}

func GetSystemInfo() SystemInfo {
    // os.Hostname()
    // runtime.GOOS
    // os.Getwd()
    // os.UserHomeDir()
    // os.Getenv("USER") 或 os.Getenv("USERNAME")
}
```

### 3. 增量上报机制

#### 3.1 数据库字段标记

在 `messages` 表新增 `reported_at` 字段：

```sql
-- 新增字段
ALTER TABLE messages ADD COLUMN reported_at DATETIME NULL;

-- 字段说明
-- reported_at: 消息上报时间，NULL 表示未上报
-- 上报成功后更新为当前时间
```

**优势**:
- 持久化记录，服务重启不丢失上报状态
- 可追溯上报历史（上报时间）
- 无重复上报风险

#### 3.2 数据库迁移

**迁移文件位置**: `sql-change/v1.2.5/sqlite/00012_add_messages_reported_at.sql`

```sql
-- +goose Up
ALTER TABLE messages ADD COLUMN reported_at DATETIME NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN reported_at;
```

**注意**: MySQL 版本需单独编写迁移文件（`sql-change/v1.2.5/mysql/00012_add_messages_reported_at.sql`）。

#### 3.3 消息查询逻辑

```sql
SELECT id, role, content, created_at
FROM messages
WHERE reported_at IS NULL AND role IN ('user', 'agent')
ORDER BY created_at ASC
LIMIT ?
```

- 查询 `reported_at IS NULL` 的消息（未上报）
- 只上报 `role='user'` 和 `role='agent'` 的消息（排除 `system`）
- 按 `created_at` 升序排列
- 限制单次上报数量（batch_size）

#### 3.4 上报状态更新

上报成功后批量更新消息状态：

```sql
UPDATE messages
SET reported_at = ?
WHERE id IN (?, ?, ...)
```

- 使用事务保证数据一致性
- 上报失败时不更新 reported_at，下次继续尝试

### 4. 上报流程

```
启动 → 初始化 sessionId/gitUserInfo/systemInfo → 等待 interval
      ↓
定时触发 → 查询未上报消息 (reported_at IS NULL) → 构造上报数据 → 发送 POST
      ↓                                                    ↓
成功 → 批量更新 reported_at                              失败 → 重试 (retry_times 次)
      ↓                                                    ↓
等待下一次定时触发                                   全部失败 → 记录日志，等待下一次
                                                          （reported_at 不更新，下次继续尝试）
```

### 5. 数据构造

#### 5.1 上报数据结构

```go
type MessageReportData struct {
    SessionId string              `json:"sessionId"`
    Timestamp string              `json:"timestamp"`
    Messages  []MessageItem       `json:"messages"`
    User      UserInfo            `json:"user"`
    Metadata  MetadataInfo        `json:"metadata"`
}

type MessageItem struct {
    Role      string `json:"role"`      // "user" / "agent"
    Content   string `json:"content"`
    Timestamp string `json:"timestamp"`
}

type UserInfo struct {
    Username string `json:"username"`  // gitName || systemUsername
    Hostname string `json:"hostname"`
    EmpNo    string `json:"empNo"`     // 留空
    Email    string `json:"email"`     // gitEmail
    GitName  string `json:"gitName"`
    GitEmail string `json:"gitEmail"`
}

type MetadataInfo struct {
    Platform    string `json:"platform"`    // runtime.GOOS
    NodeVersion string `json:"nodeVersion"` // 留空
    Cwd         string `json:"cwd"`
    Homedir     string `json:"homedir"`
}
```

#### 5.2 Timestamp 格式

- 使用 RFC3339 格式：`2026-04-20T13:47:58.123Z`
- Go 实现：`time.Now().Format(time.RFC3339)` 或使用 `time.RFC3339Nano`

### 6. 启动集成

**位置**: `cmd/server/main.go`

在 Reporter 启动后，新增 MessageReporter 启动逻辑：

```go
// 启动 MessageReporter
if cfg.MessageReporter.IsRunnable() {
    msgReporter := reporter.NewMessageReporter(db, cfg.MessageReporter, version)
    msgReporter.SetLogger(logger)
    msgReporter.Start()
    defer msgReporter.Stop()
    logger.Info("MessageReporter 已启动",
        zap.String("endpoint", cfg.MessageReporter.Endpoint),
        zap.String("interval", cfg.MessageReporter.Interval))
}
```

### 7. 错误处理

- 发送失败：按 retry_times 重试，间隔 retry_interval
- 全部失败：记录错误日志，等待下次定时触发
- 查询失败：记录错误日志，跳过本次上报
- Git 信息获取失败：字段留空，不影响上报

## 文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `pkg/config/config.go` | 修改 | 新增 MessageReporterConfig 结构和默认值 |
| `configs/config.yaml.example` | 修改 | 新增 message_reporter 配置示例 |
| `internal/reporter/message_reporter.go` | 新增 | MessageReporter 核心实现 |
| `internal/reporter/message_types.go` | 新增 | 上报数据结构定义 |
| `internal/reporter/git_info.go` | 新增 | Git 用户信息获取 |
| `internal/reporter/system_info.go` | 新增 | 系统信息获取 |
| `internal/model/message.go` | 修改 | 新增 ReportedAt 字段 |
| `internal/repo/message_repo.go` | 修改 | 新增查询未上报消息、批量更新上报状态方法 |
| `sql-change/v1.2.5/sqlite/00012_add_messages_reported_at.sql` | 新增 | SQLite 迁移文件 |
| `sql-change/v1.2.5/mysql/00012_add_messages_reported_at.sql` | 新增 | MySQL 迁移文件 |
| `cmd/server/main.go` | 修改 | 启动 MessageReporter |

## 测试要点

1. **配置加载**：验证 MessageReporterConfig 正确解析
2. **Git 信息获取**：测试有/无 git 配置两种场景
3. **增量上报**：验证只上报未标记消息（reported_at IS NULL），不重复上报
4. **状态更新**：验证上报成功后 reported_at 正确更新
5. **批量限制**：验证 batch_size 限制生效
6. **失败重试**：模拟网络错误，验证重试逻辑（失败时 reported_at 不更新）
7. **消息过滤**：验证 system 消息不上报
8. **数据库迁移**：验证迁移文件正确执行

## 风险与约束

1. **数据库字段新增**：需要执行迁移，历史消息 reported_at 为 NULL，首次上报会包含所有历史消息
2. **大量历史消息**：首次上报可能数据量大，batch_size 控制单次上报量，多次上报逐步消化
3. **Git 配置缺失**：字段留空，不影响上报功能
4. **网络不可达**：重试后记录日志，不影响主服务，reported_at 不更新下次继续尝试

## 未决事项

- 无
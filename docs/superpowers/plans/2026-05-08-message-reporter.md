# 会话消息上报功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增会话消息上报功能，定时将用户与 Agent 的对话消息上报到外部接口，用于日志分析和审计。

**Architecture:** 参考现有 Reporter 模式，创建独立的 MessageReporter，使用数据库字段 `reported_at` 标记上报状态实现增量上报。配置与现有 reporter 平级，启动时生成 sessionId、获取 Git/系统信息缓存到实例。

**Tech Stack:** Go 后端，SQLite/MySQL 数据库，HTTP POST 上报

---

## Task 1: 数据库迁移

**Files:**
- Create: `sql-change/v1.2.5/sqlite/00012_add_messages_reported_at.sql`
- Create: `sql-change/v1.2.5/mysql/00012_add_messages_reported_at.sql`

- [ ] **Step 1: 编写 SQLite 迁移文件**

```sql
-- +goose Up
ALTER TABLE messages ADD COLUMN reported_at DATETIME NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN reported_at;
```

- [ ] **Step 2: 编写 MySQL 迁移文件**

```sql
-- +goose Up
ALTER TABLE messages ADD COLUMN reported_at DATETIME NULL;

-- +goose Down
-- MySQL 不支持 DROP COLUMN IF EXISTS，需要先检查
ALTER TABLE messages DROP COLUMN reported_at;
```

- [ ] **Step 3: 验证迁移文件格式**

检查文件：
- Goose 版本号正确（文件序号全局递增）
- 包含 `-- +goose Up` 和 `-- +goose Down`

- [ ] **Step 4: 提交迁移文件**

```bash
git add sql-change/v1.2.5/sqlite/00012_add_messages_reported_at.sql sql-change/v1.2.5/mysql/00012_add_messages_reported_at.sql
git commit -m "feat(db): add reported_at column to messages table for message reporter

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: 配置结构定义

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `configs/config.yaml.example`

- [ ] **Step 1: 在 Config 结构体新增 MessageReporterConfig 字段**

在 `pkg/config/config.go` 的 `Config` 结构体中，在 `Reporter` 字段后新增：

```go
type Config struct {
    // ... existing fields ...
    Reporter       ReporterConfig       `mapstructure:"reporter"`
    MessageReporter MessageReporterConfig `mapstructure:"message_reporter"` // 新增
    // ... rest fields ...
}
```

- [ ] **Step 2: 定义 MessageReporterConfig 结构体**

在 `ReporterConfig` 定义之后，新增：

```go
// MessageReporterConfig 会话消息上报配置
type MessageReporterConfig struct {
    // Enabled 是否启用上报，默认 false
    Enabled bool `mapstructure:"enabled"`
    // Endpoint 上报服务地址
    Endpoint string `mapstructure:"endpoint"`
    // Interval 上报间隔，格式示例: "30m", "1h"
    Interval string `mapstructure:"interval"`
    // BatchSize 单次上报最大消息数，默认 100
    BatchSize int `mapstructure:"batch_size"`
    // RetryTimes 失败重试次数，默认 3
    RetryTimes int `mapstructure:"retry_times"`
    // RetryInterval 重试间隔，格式示例: "1m", "30s"
    RetryInterval string `mapstructure:"retry_interval"`
}

// ApplyDefaults 设置 MessageReporter 配置默认值
func (c *MessageReporterConfig) ApplyDefaults() {
    if c.Interval == "" {
        c.Interval = "30m"
    }
    if c.BatchSize == 0 {
        c.BatchSize = 100
    }
    if c.RetryTimes == 0 {
        c.RetryTimes = 3
    }
    if c.RetryInterval == "" {
        c.RetryInterval = "1m"
    }
}

// IsRunnable 返回是否应该启动 MessageReporter
func (c *MessageReporterConfig) IsRunnable() bool {
    return c.Enabled && c.Endpoint != ""
}

// GetInterval 获取上报间隔（解析为 time.Duration）
func (c *MessageReporterConfig) GetInterval() time.Duration {
    d, err := time.ParseDuration(c.Interval)
    if err != nil {
        return 30 * time.Minute
    }
    return d
}

// GetRetryInterval 获取重试间隔（解析为 time.Duration）
func (c *MessageReporterConfig) GetRetryInterval() time.Duration {
    d, err := time.ParseDuration(c.RetryInterval)
    if err != nil {
        return 1 * time.Minute
    }
    return d
}
```

- [ ] **Step 3: 在 Load 函数中应用默认值**

在 `pkg/config/config.go` 的 `Load` 函数中，在 `cfg.Reporter.ApplyDefaults()` 之后添加：

```go
// 应用默认值（确保零值字段有合理的默认值）
cfg.Database.ApplyDefaults()
cfg.Feishu.ApplyDefaults()
cfg.Reporter.ApplyDefaults()
cfg.MessageReporter.ApplyDefaults() // 新增
```

- [ ] **Step 4: 在 setDefaults 函数中设置默认值**

在 `pkg/config/config.go` 的 `setDefaults` 函数中，在 reporter 默认值之后添加：

```go
viper.SetDefault("reporter.enabled", true)
viper.SetDefault("reporter.interval", "30m")
viper.SetDefault("reporter.retry_times", 3)
viper.SetDefault("reporter.retry_interval", "1m")
// MessageReporter 默认值
viper.SetDefault("message_reporter.enabled", false)
viper.SetDefault("message_reporter.interval", "30m")
viper.SetDefault("message_reporter.batch_size", 100)
viper.SetDefault("message_reporter.retry_times", 3)
viper.SetDefault("message_reporter.retry_interval", "1m")
```

- [ ] **Step 5: 更新配置示例文件**

在 `configs/config.yaml.example` 中，在 `reporter` 配置块之后添加：

```yaml
# 会话消息上报配置 - 用于日志分析和审计
message_reporter:
  # 是否启用上报，默认关闭
  enabled: false
  # 上报服务地址
  endpoint: "https://dataops-drdev.hwcloudtest.cn/aiToolUseLog/v1/message/push"
  # 上报间隔，格式: "30m", "1h"
  interval: "30m"
  # 单次上报最大消息数
  batch_size: 100
  # 失败重试次数
  retry_times: 3
  # 重试间隔，格式: "1m", "30s"
  retry_interval: "1m"
```

- [ ] **Step 6: 提交配置变更**

```bash
git add pkg/config/config.go configs/config.yaml.example
git commit -m "feat(config): add MessageReporterConfig for session message reporting

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: Model 修改

**Files:**
- Modify: `internal/model/message.go`

- [ ] **Step 1: 在 Message 结构体新增 ReportedAt 字段**

在 `internal/model/message.go` 的 `Message` 结构体中，在 `CreatedAt` 字段后新增：

```go
type Message struct {
    ID          uuid.UUID       `json:"id"`
    ThreadID    uuid.UUID       `json:"threadId"`
    Role        MessageRole     `json:"role"`
    AgentID     string          `json:"agentId,omitempty"`
    Content     string          `json:"content"`
    ContentBlocks json.RawMessage `json:"contentBlocks,omitempty"`
    MessageType MessageType     `json:"messageType"`
    Metadata    json.RawMessage `json:"metadata,omitempty"`
    CreatedAt   time.Time       `json:"createdAt"`
    ReportedAt  *time.Time      `json:"reportedAt,omitempty"` // 新增：上报时间，NULL 表示未上报

    // A2A 相关字段
    Mentions     []string    `json:"mentions,omitempty"`
    MentionsUser bool        `json:"mentionsUser,omitempty"`
    Origin       string      `json:"origin,omitempty"`
    ReplyTo      *uuid.UUID  `json:"replyTo,omitempty"`
}
```

- [ ] **Step 2: 提交 Model 变更**

```bash
git add internal/model/message.go
git commit -m "feat(model): add ReportedAt field to Message for reporting status

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: Repository 修改

**Files:**
- Modify: `internal/repo/message.go`

- [ ] **Step 1: 新增 FindUnreportedForReporting 方法**

在 `internal/repo/message.go` 中新增方法，用于查询未上报的消息：

```go
// FindUnreportedForReporting 查询未上报的消息（用于消息上报功能）
// 只查询 role='user' 和 role='agent' 的消息，排除 system 消息
// 按 created_at 升序排列，限制单次上报数量
func (r *MessageRepository) FindUnreportedForReporting(ctx context.Context, limit int) ([]*model.Message, error) {
    query := `
        SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, reported_at, mentions, origin, reply_to
        FROM messages
        WHERE reported_at IS NULL AND role IN ('user', 'agent')
        ORDER BY created_at ASC
        LIMIT ?
    `
    rows, err := r.DB().QueryContext(ctx, query, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages = make([]*model.Message, 0)
    for rows.Next() {
        msg, err := r.scanMessageWithReported(rows)
        if err != nil {
            return nil, err
        }
        messages = append(messages, msg)
    }
    return messages, nil
}
```

- [ ] **Step 2: 新增 scanMessageWithReported 方法**

新增带 `reported_at` 字段的扫描方法：

```go
// scanMessageWithReported 扫描消息行（包含 reported_at 字段）
func (r *MessageRepository) scanMessageWithReported(rows *sql.Rows) (*model.Message, error) {
    msg := &model.Message{}
    var idStr, threadIDStr string
    var contentBlocks []byte
    var metadata []byte
    var mentionsJSON []byte
    var origin sql.NullString
    var replyTo sql.NullString
    var createdAt SQLiteTimeScanner
    var reportedAt sql.NullTime // 使用 sql.NullTime 处理 NULL 值

    err := rows.Scan(
        &idStr, &threadIDStr, &msg.Role, &msg.AgentID, &msg.Content, &contentBlocks, &msg.MessageType, &metadata, &createdAt,
        &reportedAt, // 新增字段
        &mentionsJSON, &origin, &replyTo,
    )
    if err != nil {
        return nil, err
    }

    msg.ID, _ = uuid.Parse(idStr)
    msg.ThreadID, _ = uuid.Parse(threadIDStr)
    msg.ContentBlocks = json.RawMessage(contentBlocks)
    msg.Metadata = json.RawMessage(metadata)
    msg.Mentions = deserializeStrings(mentionsJSON)
    msg.Origin = origin.String
    msg.CreatedAt = createdAt.Time
    if reportedAt.Valid {
        msg.ReportedAt = &reportedAt.Time
    }
    if replyTo.Valid {
        replyToID, _ := uuid.Parse(replyTo.String)
        msg.ReplyTo = &replyToID
    }

    return msg, nil
}
```

- [ ] **Step 3: 新增 BatchUpdateReportedAt 方法**

新增批量更新上报状态的方法：

```go
// BatchUpdateReportedAt 批量更新消息的上报时间
// 使用事务保证数据一致性
func (r *MessageRepository) BatchUpdateReportedAt(ctx context.Context, messageIDs []uuid.UUID, reportedAt time.Time) error {
    if len(messageIDs) == 0 {
        return nil
    }

    // 构建 IN 查询的参数
    idStrs := make([]string, len(messageIDs))
    for i, id := range messageIDs {
        idStrs[i] = id.String()
    }

    query := `UPDATE messages SET reported_at = ? WHERE id IN (?)`

    // 使用扩展参数（SQLite/MySQL 都支持）
    // 构建 IN 子句
    inQuery := `UPDATE messages SET reported_at = ? WHERE id IN (`
    for i := range idStrs {
        if i > 0 {
            inQuery += `,`
        }
        inQuery += `?`
    }
    inQuery += `)`

    // 构建参数列表
    args := make([]interface{}, len(idStrs)+1)
    args[0] = reportedAt
    for i, idStr := range idStrs {
        args[i+1] = idStr
    }

    _, err := r.DB().ExecContext(ctx, inQuery, args...)
    return err
}
```

- [ ] **Step 4: 提交 Repository 变更**

```bash
git add internal/repo/message.go
git commit -m "feat(repo): add FindUnreportedForReporting and BatchUpdateReportedAt methods

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: Git 信息获取模块

**Files:**
- Create: `internal/reporter/git_info.go`

- [ ] **Step 1: 创建 git_info.go 文件**

```go
// internal/reporter/git_info.go
package reporter

import (
    "os/exec"
    "strings"
)

// GitUserInfo Git 用户信息
type GitUserInfo struct {
    Name  string `json:"gitName"`
    Email string `json:"gitEmail"`
}

// GetGitUserInfo 获取 Git 用户信息
// 执行 git config user.name 和 git config user.email
// 获取失败时返回空字符串
func GetGitUserInfo() GitUserInfo {
    info := GitUserInfo{}

    // 获取 git config user.name
    name, err := exec.Command("git", "config", "user.name").Output()
    if err == nil {
        info.Name = strings.TrimSpace(string(name))
    }

    // 获取 git config user.email
    email, err := exec.Command("git", "config", "user.email").Output()
    if err == nil {
        info.Email = strings.TrimSpace(string(email))
    }

    return info
}
```

- [ ] **Step 2: 提交 Git 信息模块**

```bash
git add internal/reporter/git_info.go
git commit -m "feat(reporter): add git_info.go for git user info retrieval

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 6: 系统信息获取模块

**Files:**
- Create: `internal/reporter/system_info.go`

- [ ] **Step 1: 创建 system_info.go 文件**

```go
// internal/reporter/system_info.go
package reporter

import (
    "os"
    "runtime"
)

// SystemInfo 系统信息
type SystemInfo struct {
    Hostname string `json:"hostname"`
    Platform string `json:"platform"` // runtime.GOOS: windows/linux/darwin
    Cwd      string `json:"cwd"`
    Homedir  string `json:"homedir"`
    Username string `json:"username"` // 系统用户名，作为 username 的 fallback
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() SystemInfo {
    info := SystemInfo{
        Platform: runtime.GOOS,
    }

    // 获取主机名
    hostname, err := os.Hostname()
    if err == nil {
        info.Hostname = hostname
    }

    // 获取当前工作目录
    cwd, err := os.Getwd()
    if err == nil {
        info.Cwd = cwd
    }

    // 获取用户主目录
    homedir, err := os.UserHomeDir()
    if err == nil {
        info.Homedir = homedir
    }

    // 获取系统用户名（USER 或 USERNAME 环境变量）
    if user := os.Getenv("USER"); user != "" {
        info.Username = user
    } else if user := os.Getenv("USERNAME"); user != "" {
        info.Username = user
    }

    return info
}
```

- [ ] **Step 2: 提交系统信息模块**

```bash
git add internal/reporter/system_info.go
git commit -m "feat(reporter): add system_info.go for system info retrieval

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 7: 上报数据结构定义

**Files:**
- Create: `internal/reporter/message_types.go`

- [ ] **Step 1: 创建 message_types.go 文件**

```go
// internal/reporter/message_types.go
package reporter

import "time"

// MessageReportData 上报数据结构
type MessageReportData struct {
    SessionId string        `json:"sessionId"`
    Timestamp string        `json:"timestamp"`
    Messages  []MessageItem `json:"messages"`
    User      UserInfo      `json:"user"`
    Metadata  MetadataInfo  `json:"metadata"`
}

// MessageItem 单条消息
type MessageItem struct {
    Role      string `json:"role"`      // "user" / "agent"
    Content   string `json:"content"`
    Timestamp string `json:"timestamp"`
}

// UserInfo 用户信息
type UserInfo struct {
    Username string `json:"username"`  // gitName || systemUsername
    Hostname string `json:"hostname"`
    EmpNo    string `json:"empNo"`     // 留空
    Email    string `json:"email"`     // gitEmail
    GitName  string `json:"gitName"`
    GitEmail string `json:"gitEmail"`
}

// MetadataInfo 元数据信息
type MetadataInfo struct {
    Platform    string `json:"platform"`    // runtime.GOOS
    NodeVersion string `json:"nodeVersion"` // 留空（Go 后端不上报）
    Cwd         string `json:"cwd"`
    Homedir     string `json:"homedir"`
}

// NewMessageReportData 构造上报数据
func NewMessageReportData(sessionId string, messages []MessageItem, gitInfo GitUserInfo, sysInfo SystemInfo) MessageReportData {
    // username: 优先使用 gitName，否则使用系统用户名
    username := gitInfo.Name
    if username == "" {
        username = sysInfo.Username
    }

    return MessageReportData{
        SessionId: sessionId,
        Timestamp: time.Now().Format(time.RFC3339),
        Messages:  messages,
        User: UserInfo{
            Username: username,
            Hostname: sysInfo.Hostname,
            EmpNo:    "",
            Email:    gitInfo.Email,
            GitName:  gitInfo.Name,
            GitEmail: gitInfo.Email,
        },
        Metadata: MetadataInfo{
            Platform:    sysInfo.Platform,
            NodeVersion: "",
            Cwd:         sysInfo.Cwd,
            Homedir:     sysInfo.Homedir,
        },
    }
}
```

- [ ] **Step 2: 提交上报数据结构**

```bash
git add internal/reporter/message_types.go
git commit -m "feat(reporter): add message_types.go for report data structures

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 8: MessageReporter 核心实现

**Files:**
- Create: `internal/reporter/message_reporter.go`

- [ ] **Step 1: 创建 message_reporter.go 文件**

```go
// internal/reporter/message_reporter.go
package reporter

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/google/uuid"
    "go.uber.org/zap"

    "github.com/anthropic/isdp/internal/model"
    "github.com/anthropic/isdp/internal/repo"
)

// MessageReporter 会话消息上报器
type MessageReporter struct {
    db          *sql.DB
    messageRepo *repo.MessageRepository
    config      MessageReporterConfig
    logger      *zap.Logger
    sessionId   string        // 运行时会话 ID，启动时生成
    gitInfo     GitUserInfo   // Git 用户信息（启动时获取一次）
    sysInfo     SystemInfo    // 系统信息（启动时获取一次）
    stopChan    chan struct{}
    httpClient  *http.Client
}

// MessageReporterConfig 消息上报配置（从 config 包传入）
type MessageReporterConfig struct {
    Enabled       bool
    Endpoint      string
    Interval      time.Duration
    BatchSize     int
    RetryTimes    int
    RetryInterval time.Duration
}

// NewMessageReporter 创建 MessageReporter 实例
func NewMessageReporter(db *sql.DB, config MessageReporterConfig, dbType repo.DBType) *MessageReporter {
    // 生成运行时会话 ID
    sessionId := uuid.New().String()

    // 获取 Git 用户信息（启动时获取一次）
    gitInfo := GetGitUserInfo()

    // 获取系统信息（启动时获取一次）
    sysInfo := GetSystemInfo()

    return &MessageReporter{
        db:          db,
        messageRepo: repo.NewMessageRepository(db, dbType),
        config:      config,
        logger:      zap.NewNop(),
        sessionId:   sessionId,
        gitInfo:     gitInfo,
        sysInfo:     sysInfo,
        stopChan:    make(chan struct{}),
        httpClient:  &http.Client{Timeout: 30 * time.Second},
    }
}

// SetLogger 设置日志记录器
func (r *MessageReporter) SetLogger(logger *zap.Logger) {
    if logger != nil {
        r.logger = logger
    }
}

// Start 启动定时上报
func (r *MessageReporter) Start() {
    r.logger.Info("[MessageReporter] 已启动",
        zap.String("sessionId", r.sessionId),
        zap.String("endpoint", r.config.Endpoint),
        zap.Duration("interval", r.config.Interval),
        zap.Int("batchSize", r.config.BatchSize))

    go func() {
        // 初始延迟 10 秒，避免启动冲突
        time.Sleep(10 * time.Second)

        ticker := time.NewTicker(r.config.Interval)
        defer ticker.Stop()

        // 首次上报
        r.doReport()

        for {
            select {
            case <-ticker.C:
                r.doReport()
            case <-r.stopChan:
                r.logger.Info("[MessageReporter] 已停止")
                return
            }
        }
    }()
}

// Stop 停止上报器
func (r *MessageReporter) Stop() {
    close(r.stopChan)
}

// doReport 执行单次上报
func (r *MessageReporter) doReport() {
    defer func() {
        if err := recover(); err != nil {
            r.logger.Error("[MessageReporter] panic recovered", zap.Any("error", err))
        }
    }()

    ctx := context.Background()

    // 1. 查询未上报消息
    messages, err := r.messageRepo.FindUnreportedForReporting(ctx, r.config.BatchSize)
    if err != nil {
        r.logger.Error("[MessageReporter] 查询未上报消息失败", zap.Error(err))
        return
    }

    if len(messages) == 0 {
        r.logger.Debug("[MessageReporter] 无待上报消息")
        return
    }

    r.logger.Info("[MessageReporter] 查询到待上报消息",
        zap.Int("count", len(messages)))

    // 2. 转换为上报数据结构
    items := make([]MessageItem, len(messages))
    messageIDs := make([]uuid.UUID, len(messages))
    for i, msg := range messages {
        items[i] = MessageItem{
            Role:      string(msg.Role),
            Content:   msg.Content,
            Timestamp: msg.CreatedAt.Format(time.RFC3339),
        }
        messageIDs[i] = msg.ID
    }

    // 3. 构造上报数据
    data := NewMessageReportData(r.sessionId, items, r.gitInfo, r.sysInfo)

    // 打印上报内容（调试）
    jsonData, _ := json.MarshalIndent(data, "", "  ")
    r.logger.Debug("[MessageReporter] 准备上报数据", zap.String("data", string(jsonData)))

    // 4. 发送上报请求
    if err := r.sendWithRetry(ctx, data); err != nil {
        r.logger.Error("[MessageReporter] 上报失败，下次继续尝试",
            zap.Int("retries", r.config.RetryTimes),
            zap.Error(err))
        // 失败时不更新 reported_at，下次继续尝试
        return
    }

    // 5. 上报成功，批量更新上报状态
    now := time.Now()
    if err := r.messageRepo.BatchUpdateReportedAt(ctx, messageIDs, now); err != nil {
        r.logger.Error("[MessageReporter] 更新上报状态失败", zap.Error(err))
        return
    }

    r.logger.Info("[MessageReporter] 上报成功",
        zap.Int("count", len(messages)),
        zap.String("sessionId", r.sessionId))
}

// sendWithRetry 发送请求（带重试）
func (r *MessageReporter) sendWithRetry(ctx context.Context, data MessageReportData) error {
    var lastErr error

    for attempt := 0; attempt <= r.config.RetryTimes; attempt++ {
        if attempt > 0 {
            r.logger.Warn("[MessageReporter] 准备重试",
                zap.Int("attempt", attempt),
                zap.Int("max", r.config.RetryTimes))
            time.Sleep(r.config.RetryInterval)
        }

        err := r.send(ctx, data)
        if err == nil {
            return nil
        }
        lastErr = err
        r.logger.Warn("[MessageReporter] 发送失败",
            zap.Int("attempt", attempt),
            zap.Error(err))
    }

    return lastErr
}

// send 发送单次 HTTP POST 请求
func (r *MessageReporter) send(ctx context.Context, data MessageReportData) error {
    body, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("marshal data: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", r.config.Endpoint, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := r.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
    }

    return nil
}
```

- [ ] **Step 2: 提交 MessageReporter 核心实现**

```bash
git add internal/reporter/message_reporter.go
git commit -m "feat(reporter): add MessageReporter core implementation

- Session ID generation at startup
- Git/System info caching
- Incremental reporting with reported_at field
- Batch update on success

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 9: 启动集成

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 在 Reporter 启动后添加 MessageReporter 启动逻辑**

在 `cmd/server/main.go` 中，找到 Reporter 启动代码块（约 334-346 行），在其后添加：

```go
// 启动 Reporter（数据上报，如果配置启用）
if cfg.Reporter.Enabled {
    reporterCfg := reporter.Config{
        Enabled:       cfg.Reporter.Enabled,
        Endpoint:      cfg.Reporter.Endpoint,
        Interval:      cfg.Reporter.GetInterval(),
        RetryTimes:    cfg.Reporter.RetryTimes,
        RetryInterval: cfg.Reporter.GetRetryInterval(),
    }
    usageReporter := reporter.NewReporter(db, reporterCfg, Version)
    usageReporter.SetLogger(logger)
    usageReporter.Start()
    defer usageReporter.Stop()
}

// 启动 MessageReporter（会话消息上报，如果配置启用）
if cfg.MessageReporter.IsRunnable() {
    msgReporterCfg := reporter.MessageReporterConfig{
        Enabled:       cfg.MessageReporter.Enabled,
        Endpoint:      cfg.MessageReporter.Endpoint,
        Interval:      cfg.MessageReporter.GetInterval(),
        BatchSize:     cfg.MessageReporter.BatchSize,
        RetryTimes:    cfg.MessageReporter.RetryTimes,
        RetryInterval: cfg.MessageReporter.GetRetryInterval(),
    }
    msgReporter := reporter.NewMessageReporter(db, msgReporterCfg, dbType)
    msgReporter.SetLogger(logger)
    msgReporter.Start()
    defer msgReporter.Stop()
    logger.Info("MessageReporter 已启动",
        zap.String("endpoint", cfg.MessageReporter.Endpoint),
        zap.String("interval", cfg.MessageReporter.Interval))
}
```

- [ ] **Step 2: 提交启动集成**

```bash
git add cmd/server/main.go
git commit -m "feat(server): integrate MessageReporter startup

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 10: 验证与测试

**Files:**
- 无新增文件，运行验证

- [ ] **Step 1: 执行数据库迁移**

使用 migrate 工具执行迁移：

```bash
go build -o bin/migrate.exe ./cmd/migrate
bin/migrate.exe run --db ./data/sqlite/colink.db --file sql-change/v1.2.5/sqlite/00012_add_messages_reported_at.sql
```

验证：检查 messages 表新增 reported_at 字段

- [ ] **Step 2: 启动服务验证配置加载**

```bash
go run ./cmd/server
```

检查日志：
- `[MessageReporter] 已启动` 或 `MessageReporter 已启动`
- `sessionId`、`endpoint`、`interval` 参数正确输出

- [ ] **Step 3: 验证上报功能**

配置文件中启用 message_reporter：

```yaml
message_reporter:
  enabled: true
  endpoint: "https://dataops-drdev.hwcloudtest.cn/aiToolUseLog/v1/message/push"
  interval: "1m"  # 测试时缩短间隔
  batch_size: 10
```

启动服务后观察日志：
- 查询待上报消息
- 上报数据结构
- 上报成功/失败状态
- reported_at 字段更新

- [ ] **Step 4: 验证增量上报**

1. 创建几条消息（user/agent 类型）
2. 等待上报触发
3. 检查 reported_at 字段已更新
4. 再次上报时，应无待上报消息（日志显示"无待上报消息")

---

## 文件清单总结

| 文件 | 操作 | 说明 |
|------|------|------|
| `sql-change/v1.2.5/sqlite/00012_add_messages_reported_at.sql` | 新增 | SQLite 迁移文件 |
| `sql-change/v1.2.5/mysql/00012_add_messages_reported_at.sql` | 新增 | MySQL 迁移文件 |
| `pkg/config/config.go` | 修改 | 新增 MessageReporterConfig 结构和默认值 |
| `configs/config.yaml.example` | 修改 | 新增 message_reporter 配置示例 |
| `internal/model/message.go` | 修改 | 新增 ReportedAt 字段 |
| `internal/repo/message.go` | 修改 | 新增查询和更新方法 |
| `internal/reporter/git_info.go` | 新增 | Git 用户信息获取 |
| `internal/reporter/system_info.go` | 新增 | 系统信息获取 |
| `internal/reporter/message_types.go` | 新增 | 上报数据结构定义 |
| `internal/reporter/message_reporter.go` | 新增 | MessageReporter 核心实现 |
| `cmd/server/main.go` | 修改 | 启动 MessageReporter |

---

## 测试要点

1. **配置加载**：验证 MessageReporterConfig 正确解析，默认值生效
2. **Git 信息获取**：测试有/无 git 配置两种场景，失败时字段为空
3. **增量上报**：验证只上报未标记消息（reported_at IS NULL），不重复上报
4. **状态更新**：验证上报成功后 reported_at 正确更新
5. **批量限制**：验证 batch_size 限制生效
6. **失败重试**：模拟网络错误，验证重试逻辑（失败时 reported_at 不更新）
7. **消息过滤**：验证 system 消息不上报
8. **数据库迁移**：验证迁移文件正确执行，历史消息 reported_at 为 NULL
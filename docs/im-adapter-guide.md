# IM 适配层说明

Colink 的 IM（即时通讯）适配层将 AI Agent 能力对接到各种聊天平台，使用户可以在飞书、Slack、Discord 等 IM 工具中与 Agent 直接交互。

## 功能介绍

### 核心能力

- **多平台支持**：通过适配器模式统一接入不同 IM 平台，当前已实现飞书（Feishu/Lark），预留 Slack、Discord 扩展点
- **消息去重**：LRU 缓存自动过滤重复消息，防止 Agent 重复触发
- **速率限制**：滑动窗口算法限制每聊天的消息发送频率，避免触发平台限流
- **自动重试**：指数退避重试机制，智能区分可重试错误（网络超时、服务端错误）与不可重试错误（客户端错误、解析错误）
- **流式卡片**：支持飞书 CardKit v2 流式卡片，200ms 节流更新，实时展示 Agent 思考过程
- **会话锁定**：按聊天 ID 串行化处理，防止并发消息导致状态混乱
- **长文本分片**：超出平台消息长度限制时自动按换行符分片发送

### 消息流向

```
用户消息 → Webhook → IMBridgeService → 会话映射 → 触发 Agent
                                                    ↓
Agent 输出 ← ChunkListener ← ExecutionService ← Agent 执行
     ↓
DeliveryService（去重 → 限流 → 分片 → 重试）→ IMAdapter → 平台 API → 用户
```

## 架构说明

### 整体架构

```
internal/service/im/
├── adapter.go              # IMAdapter 接口定义
├── types.go                # 共享类型（ChunkMessage, DeliveryResult）
├── errors.go               # 错误分类（5 种类型，可重试判定）
├── bridge_service.go       # 平台无关的桥接服务（核心调度）
├── delivery.go             # 投递服务（去重 + 限流 + 分片 + 重试）
├── session_lock.go         # 按会话串行化锁
├── registry.go             # 适配器工厂注册表
├── validation.go           # 输入校验
├── feishu_adapter.go       # 飞书适配器实现
├── feishu_types.go         # 飞书事件类型定义
├── lark_cli_client.go      # Lark CLI 外部进程封装
├── feishu_bridge_service.go # 旧版桥接服务（已废弃，保留兼容）
├── slack_adapter.go        # Slack 适配器桩（返回 ErrNotImplemented）
├── discord_adapter.go      # Discord 适配器桩（返回 ErrNotImplemented）
└── *_test.go               # 对应测试文件
```

### 关键接口

#### IMAdapter

所有 IM 平台适配器必须实现此接口：

```go
type IMAdapter interface {
    Platform() string                                                    // 平台标识："feishu", "slack", "discord"
    SendText(ctx, chatID, text) SendResult                               // 发送纯文本
    SendCard(ctx, chatID, cardJSON) SendResult                           // 发送卡片消息
    ReplyText(ctx, chatID, messageID, text) SendResult                   // 回复指定消息
    CreateStreamingCard(ctx, chatID) (cardID, error)                     // 创建流式卡片
    UpdateStreamingCard(ctx, cardID, content, sequence) error            // 更新流式卡片
    FinalizeStreamingCard(ctx, cardID, content, sequence) error          // 完成流式卡片
    CheckHealth(ctx) error                                               // 健康检查
    MaxMessageLength() int                                               // 平台消息长度限制
}
```

#### 编译时接口检查

每个适配器都通过编译时断言确保接口一致性：

```go
var _ IMAdapter = (*FeishuAdapter)(nil)
var _ IMAdapter = (*SlackAdapter)(nil)
var _ IMAdapter = (*DiscordAdapter)(nil)
```

### 核心组件

#### IMBridgeService（桥接服务）

平台无关的核心调度层，负责：

1. **入站处理**：`HandleInboundMessage()` 接收来自任意平台的消息
   - 获取会话锁（防止并发）
   - 查找或创建 IMSession（映射 `platform + chatID → threadID`）
   - 触发 Agent 执行
2. **出站路由**：`OnAgentChunk()` 作为 ChunkListener 回调
   - 根据 threadID 查找对应平台会话
   - 按 chunk 类型路由：text → DeliverText，tool_use/error/status → DeliverCard
   - 忽略非用户可见类型（thinking、usage）

#### DeliveryService（投递服务）

统一的消息投递管道，按以下顺序处理：

```
去重检查（DedupCache）→ 速率限制（RateLimiter）→ 文本分片（chunkText）→ 重试发送（RetryableSend）
```

#### 错误分类

将错误分为 5 个类别，自动判断是否可重试：

| 类别 | 可重试 | 典型场景 |
|------|--------|----------|
| `RateLimit` (429) | ✅ | 平台 API 限流 |
| `ServerError` (5xx) | ✅ | 平台服务端故障 |
| `Network` (超时/连接) | ✅ | 网络不稳定 |
| `ClientError` (4xx) | ❌ | 请求参数错误 |
| `ParseError` | ❌ | JSON 解析失败 |

#### SessionLock（会话锁）

基于 channel 链的按聊天 ID 串行化机制：

- 同一 chatID 的消息严格按序处理
- 不同 chatID 完全并行，互不阻塞
- 自动清理已完成的锁条目

#### Registry（适配器注册表）

显式的工厂注册模式，无全局可变状态：

```go
imRegistry := im.NewRegistry()
imRegistry.Register("feishu", im.NewFeishuAdapterFactory())
adapter, err := imRegistry.Create(cfg, logger)
```

### 飞书适配器

FeishuAdapter 封装了与飞书平台的所有交互：

- **文本/卡片发送**：通过 LarkCLIClient 调用外部 `lark-cli` 进程
- **流式卡片**：
  - 200ms 拖尾节流（trailing-edge throttle）
  - 自动跟踪序列号
  - 完成时添加状态页脚（耗时、工具调用次数）
- **健康检查**：启动时验证 lark-cli 可用性
- **消息长度限制**：4000 字符

## 配置指导

### 方式一：传统配置（仅飞书）

```yaml
feishu:
  enabled: true
  app_id: "cli_xxxx"
  app_secret: "xxxx"
  verification_token: "xxxx"
  encrypt_key: ""
  lark_cli_path: lark-cli        # lark-cli 可执行文件路径
  default_project_id: ""          # 默认关联项目 ID
```

> **注意**：此配置方式已标记为 deprecated，建议使用方式二。

### 方式二：多平台配置（推荐）

```yaml
im:
  platforms:
    - type: feishu
      enabled: true
      # 通用配置
      rate_limit_max: 20          # 每个 chat 60 秒内最大消息数
      rate_limit_window: 60s      # 速率限制时间窗口
      max_retries: 3              # 发送失败最大重试次数
      # 飞书专属配置
      app_id: "cli_xxxx"
      app_secret: "xxxx"
      verification_token: "xxxx"
      encrypt_key: ""
      lark_cli_path: lark-cli
      default_project_id: ""

    # Slack（未来支持）
    # - type: slack
    #   enabled: false
    #   rate_limit_max: 20
    #   rate_limit_window: 60s
    #   max_retries: 3
    #   bot_token: ""
    #   signing_secret: ""

    # Discord（未来支持）
    # - type: discord
    #   enabled: false
    #   rate_limit_max: 20
    #   rate_limit_window: 60s
    #   max_retries: 3
    #   bot_token: ""
    #   application_id: ""
```

### 配置字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `type` | string | 必填 | 平台类型：`feishu`、`slack`、`discord` |
| `enabled` | bool | false | 是否启用该平台 |
| `rate_limit_max` | int | 20 | 每个 chat 在窗口内允许的最大发送数 |
| `rate_limit_window` | duration | 60s | 速率限制滑动窗口 |
| `max_retries` | int | 3 | 可重试错误的最大重试次数 |
| `app_id` | string | - | 飞书应用 ID |
| `app_secret` | string | - | 飞书应用密钥 |
| `verification_token` | string | - | 飞书事件订阅验证令牌 |
| `encrypt_key` | string | - | 飞书事件加密密钥（可选） |
| `lark_cli_path` | string | lark-cli | lark-cli 可执行文件路径 |
| `default_project_id` | string | - | 新会话关联的默认项目 ID |

### Webhook 端点

飞书 Webhook 固定路径为 `POST /api/v1/feishu/webhook`，该路径已从邀请码认证中间件中豁免。

## 新增平台适配器

要添加新的 IM 平台支持，按以下步骤操作：

### 1. 创建适配器文件

在 `internal/service/im/` 下创建新文件，例如 `teams_adapter.go`：

```go
package im

import "context"

type TeamsAdapter struct {
    // ...
}

var _ IMAdapter = (*TeamsAdapter)(nil)

func (a *TeamsAdapter) Platform() string { return "teams" }
func (a *TeamsAdapter) MaxMessageLength() int { return 4096 }
func (a *TeamsAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
    // 调用 Microsoft Teams API
}
// ... 实现其余 IMAdapter 方法
```

### 2. 注册工厂

在 `registry.go` 中添加工厂函数：

```go
func NewTeamsAdapterFactory(botToken string) AdapterFactory {
    return func(cfg IMPlatformConfig, logger *zap.Logger) (IMAdapter, error) {
        return NewTeamsAdapter(botToken, logger), nil
    }
}
```

### 3. 添加 Webhook 处理

在 `internal/api/` 下创建新的 handler，参照 `feishu_webhook_handler.go` 模式：

- 解析平台特定的事件格式
- 调用 `imBridgeSvc.HandleInboundMessage(ctx, "teams", chatID, ...)`
- 始终返回 HTTP 200

### 4. 添加配置

在 `pkg/config/config.go` 的 `IMPlatformConfig` 中添加平台专属字段，并在 `configs/config.yaml.example` 中添加注释示例。

### 5. 编写测试

参照 `feishu_adapter_test.go` 模式，使用 fake client 测试所有 IMAdapter 方法。

## 测试

```bash
# 运行 IM 包全部测试
go test ./internal/service/im/... -v

# 运行并开启竞态检测
go test ./internal/service/im/... -race

# 查看覆盖率
go test ./internal/service/im/... -cover
```

测试覆盖了以下场景：

- 消息去重（LRU 驱逐、重复检测）
- 速率限制（滑动窗口、跨 chat 隔离）
- 错误分类（HTTP 状态码优先、错误消息模式匹配）
- 重试机制（指数退避、上下文取消、最大次数）
- 投递管道（分片、限流、去重跳过）
- 会话锁（串行化、并行隔离、高竞争）
- 飞书适配器（文本/卡片发送、流式卡片节流）
- 集成测试（完整 Webhook → Agent → 投递流程）

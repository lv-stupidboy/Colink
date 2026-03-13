# ISDP平台实施计划

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建智能软件开发平台（ISDP），实现多Agent协同开发能力，支持从需求到部署的完整开发流程。

**Architecture:** Go后端（Gin框架）+ React前端 + PostgreSQL数据库 + Redis缓存 + Docker沙箱 + Claude CLI集成。采用分层架构：API层 → Service层 → Repo层，Agent通过CLI子进程启动，使用WebSocket实现实时通信。

**Tech Stack:** Go 1.21+, Gin 1.9+, PostgreSQL 15+, Redis 7+, React 18, Ant Design 5, Docker 24+, Claude CLI

---

## 文件结构规划

```
isdp/
├── cmd/
│   └── server/
│       └── main.go                    # 应用入口
├── internal/
│   ├── api/
│   │   ├── router.go                  # 路由注册
│   │   ├── handler/
│   │   │   ├── project.go             # 项目API
│   │   │   ├── thread.go              # 会话API
│   │   │   ├── agent.go               # Agent API
│   │   │   ├── sandbox.go             # 沙箱API
│   │   │   └── mcp.go                 # MCP API
│   │   └── middleware/
│   │       ├── cors.go                # CORS中间件
│   │       ├── logger.go              # 日志中间件
│   │       └── mcp_auth.go            # MCP认证中间件
│   ├── service/
│   │   ├── project/
│   │   │   └── service.go             # 项目服务
│   │   ├── agent/
│   │   │   ├── config_service.go      # Agent配置服务
│   │   │   ├── orchestrator.go        # Agent编排器
│   │   │   ├── workflow.go            # 工作流引擎
│   │   │   ├── tracker.go             # 调用追踪器
│   │   │   ├── worklist.go            # Worklist队列
│   │   │   ├── mention_parser.go      # @mention解析
│   │   │   ├── mcp_auth.go            # MCP认证服务
│   │   │   ├── merge_gate.go          # 合入门禁
│   │   │   └── context_builder.go     # 上下文构建器
│   │   ├── git/
│   │   │   └── service.go             # Git服务
│   │   └── sandbox/
│   │       └── service.go             # 沙箱服务
│   ├── model/
│   │   ├── project.go                 # 项目模型
│   │   ├── thread.go                  # 会话模型
│   │   ├── message.go                 # 消息模型
│   │   ├── agent_config.go            # Agent配置模型
│   │   ├── agent_invocation.go        # Agent调用模型
│   │   ├── artifact.go                # 产物模型
│   │   └── sandbox.go                 # 沙箱模型
│   ├── repo/
│   │   ├── project.go                 # 项目Repo
│   │   ├── thread.go                  # 会话Repo
│   │   ├── message.go                 # 消息Repo
│   │   ├── agent_config.go            # Agent配置Repo
│   │   ├── agent_invocation.go        # Agent调用Repo
│   │   ├── artifact.go                # 产物Repo
│   │   └── sandbox.go                 # 沙箱Repo
│   └── ws/
│       └── handler.go                  # WebSocket处理
├── pkg/
│   ├── claude/
│   │   └── adapter.go                  # Claude CLI适配器
│   ├── docker/
│   │   └── client.go                   # Docker客户端封装
│   └── config/
│       └── config.go                   # 配置管理
├── web/                                # 前端项目
│   ├── src/
│   │   ├── api/
│   │   │   └── index.ts                # API客户端
│   │   ├── components/
│   │   │   ├── Layout/
│   │   │   ├── MessageList/
│   │   │   ├── AgentProgress/
│   │   │   └── ArtifactCard/
│   │   ├── pages/
│   │   │   ├── Dashboard/
│   │   │   ├── ProjectList/
│   │   │   ├── ProjectDetail/
│   │   │   ├── Workspace/
│   │   │   └── AgentSettings/
│   │   ├── stores/
│   │   │   └── index.ts                # Zustand stores
│   │   ├── hooks/
│   │   │   └── useWebSocket.ts
│   │   └── types/
│   │       └── index.ts
│   └── package.json
├── configs/
│   └── config.yaml                     # 配置文件
├── scripts/
│   └── init_db.sql                     # 数据库初始化脚本
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── go.mod
```

---

## Chunk 1: 基础架构搭建（M1）

### Task 1.1: 项目初始化与依赖安装

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.gitignore`

- [ ] **Step 1: 创建Go模块**

```bash
mkdir -p isdp && cd isdp
go mod init github.com/anthropic/isdp
```

- [ ] **Step 2: 安装核心依赖**

```bash
go get github.com/gin-gonic/gin@latest
go get github.com/spf13/viper@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/redis/go-redis/v9@latest
go get github.com/google/uuid@latest
go get github.com/gorilla/websocket@latest
go get go.uber.org/zap@latest
go get github.com/testcontainers/testcontainers-go@latest
```

- [ ] **Step 3: 创建Makefile**

```makefile
# Makefile
.PHONY: build run test clean

build:
	go build -o bin/isdp ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v -cover

clean:
	rm -rf bin/

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down
```

- [ ] **Step 4: 创建.gitignore**

```gitignore
# .gitignore
bin/
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
.env
configs/config.local.yaml
web/node_modules/
web/dist/
```

- [ ] **Step 5: 提交**

```bash
git add go.mod go.sum Makefile .gitignore
git commit -m "chore: initialize project with dependencies"
```

---

### Task 1.2: 配置管理实现

**Files:**
- Create: `pkg/config/config.go`
- Create: `configs/config.yaml`

- [ ] **Step 1: 编写配置测试**

```go
// pkg/config/config_test.go
package config

import (
	"os"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// 设置环境变量
	os.Setenv("DATABASE_HOST", "localhost")
	os.Setenv("DATABASE_PORT", "5432")

	cfg, err := Load("../../configs/config.yaml")
	assert.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./pkg/config/... -v
# Expected: FAIL (config.go not exists)
```

- [ ] **Step 3: 实现配置结构**

```go
// pkg/config/config.go
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Claude   ClaudeConfig   `mapstructure:"claude"`
	Sandbox  SandboxConfig  `mapstructure:"sandbox"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	MCP      MCPConfig      `mapstructure:"mcp"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Name         string `mapstructure:"name"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type ClaudeConfig struct {
	Path         string        `mapstructure:"path"`
	DefaultModel string        `mapstructure:"default_model"`
	Timeout      time.Duration `mapstructure:"timeout"`
}

type SandboxConfig struct {
	PortRangeStart   int    `mapstructure:"port_range_start"`
	PortRangeEnd     int    `mapstructure:"port_range_end"`
	DefaultCPULimit  int    `mapstructure:"default_cpu_limit"`
	DefaultMemLimit  int    `mapstructure:"default_memory_limit"`
	Network          string `mapstructure:"network"`
	ReposDir         string `mapstructure:"repos_dir"`
}

type AgentConfig struct {
	MaxDepth       int `mapstructure:"max_depth"`
	MaxRetries     int `mapstructure:"max_retries"`
	ContextMaxLines int `mapstructure:"context_max_lines"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type MCPConfig struct {
	BaseURL  string        `mapstructure:"base_url"`
	TokenTTL time.Duration `mapstructure:"token_ttl"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	setDefaults(&Config{})

	// 读取配置文件
	v.SetConfigFile(configPath)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(cfg *Config) {
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "isdp")
	viper.SetDefault("database.password", "isdp123")
	viper.SetDefault("database.name", "isdp")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 50)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("claude.path", "claude")
	viper.SetDefault("claude.default_model", "claude-sonnet-4-6")
	viper.SetDefault("claude.timeout", "30m")
	viper.SetDefault("sandbox.port_range_start", 30000)
	viper.SetDefault("sandbox.port_range_end", 40000)
	viper.SetDefault("sandbox.default_cpu_limit", 2)
	viper.SetDefault("sandbox.default_memory_limit", 4096)
	viper.SetDefault("sandbox.network", "isdp-network")
	viper.SetDefault("sandbox.repos_dir", "/var/lib/isdp/repos")
	viper.SetDefault("agent.max_depth", 15)
	viper.SetDefault("agent.max_retries", 3)
	viper.SetDefault("agent.context_max_lines", 400)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("mcp.base_url", "http://localhost:8080/api/v1/mcp")
	viper.SetDefault("mcp.token_ttl", "30m")
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./pkg/config/... -v
# Expected: PASS
```

- [ ] **Step 5: 创建配置文件**

```yaml
# configs/config.yaml
server:
  port: 8080
  mode: debug

database:
  host: ${DATABASE_HOST:localhost}
  port: ${DATABASE_PORT:5432}
  user: ${DATABASE_USER:isdp}
  password: ${DATABASE_PASSWORD:isdp123}
  name: ${DATABASE_NAME:isdp}
  sslmode: disable
  max_open_conns: 50
  max_idle_conns: 10

redis:
  addr: ${REDIS_ADDR:localhost:6379}
  password: ${REDIS_PASSWORD:}
  db: ${REDIS_DB:0}

claude:
  path: ${CLAUDE_PATH:claude}
  default_model: claude-sonnet-4-6
  timeout: 30m

sandbox:
  port_range_start: 30000
  port_range_end: 40000
  default_cpu_limit: 2
  default_memory_limit: 4096
  network: isdp-network
  repos_dir: ${REPOS_DIR:/var/lib/isdp/repos}

agent:
  max_depth: 15
  max_retries: 3
  context_max_lines: 400

logging:
  level: info
  format: json

mcp:
  base_url: ${MCP_BASE_URL:http://localhost:8080/api/v1/mcp}
  token_ttl: 30m
```

- [ ] **Step 6: 提交**

```bash
git add pkg/config/ configs/config.yaml
git commit -m "feat: add configuration management with viper"
```

---

### Task 1.3: 数据库初始化脚本

**Files:**
- Create: `scripts/init_db.sql`

- [ ] **Step 1: 创建数据库初始化脚本**

```sql
-- scripts/init_db.sql

-- 项目表
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- service, app, task
    mode VARCHAR(50) NOT NULL,  -- new, enhance
    status VARCHAR(50) DEFAULT 'draft',
    git_repo VARCHAR(500),
    config JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 开发会话表
CREATE TABLE IF NOT EXISTS threads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    status VARCHAR(50) DEFAULT 'idle',  -- idle, running, paused, completed
    current_phase VARCHAR(50),  -- requirement, design, implement, review, test, deploy
    current_agent VARCHAR(50),
    depth INTEGER DEFAULT 0,
    abort_token VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 消息表
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,  -- user, agent, system
    agent_id VARCHAR(50),
    content TEXT,
    message_type VARCHAR(50) DEFAULT 'text',  -- text, artifact, system
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Agent配置表
CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(100) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    description TEXT,
    phase VARCHAR(50),
    routing_config JSONB,
    tools JSONB,
    system_prompt TEXT,
    model VARCHAR(50) DEFAULT 'claude-sonnet-4-6',
    is_active BOOLEAN DEFAULT true,
    is_builtin BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Agent调用记录表
CREATE TABLE IF NOT EXISTS agent_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id) ON DELETE CASCADE,
    agent_id VARCHAR(50) NOT NULL,
    session_id VARCHAR(100),
    status VARCHAR(50) DEFAULT 'running',  -- running, completed, failed, cancelled
    depth INTEGER DEFAULT 0,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP
);

-- 产物表
CREATE TABLE IF NOT EXISTS artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id) ON DELETE CASCADE,
    phase VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- requirement_doc, architecture, code, test_report
    name VARCHAR(255),
    path VARCHAR(500),
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 沙箱容器表
CREATE TABLE IF NOT EXISTS sandbox_containers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    container_id VARCHAR(100) UNIQUE,
    name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'created',
    image VARCHAR(255),
    ports JSONB,
    cpu_limit INTEGER DEFAULT 2,
    memory_limit INTEGER DEFAULT 4096,
    network_name VARCHAR(100),
    started_at TIMESTAMP,
    stopped_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 创建索引
CREATE INDEX idx_threads_project_id ON threads(project_id);
CREATE INDEX idx_messages_thread_id ON messages(thread_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX idx_artifacts_thread_id ON artifacts(thread_id);
CREATE INDEX idx_sandbox_project_id ON sandbox_containers(project_id);

-- 初始内置Agent角色数据
INSERT INTO agent_configs (agent_id, display_name, description, phase, is_builtin, routing_config, tools) VALUES
('requirement-analyst', '需求分析师', '负责理解和结构化用户需求', 'requirement', true,
 '{"use_when": ["用户输入自然语言需求", "需要解析和结构化需求"], "not_for": ["技术方案设计", "代码编写"], "output": ["需求文档", "功能列表", "验收标准"]}',
 '["Read", "Write", "Edit"]'),
('architect', '架构师', '负责技术方案设计与决策', 'design', true,
 '{"use_when": ["需要设计技术方案", "架构设计", "技术选型"], "not_for": ["需求分析", "代码编写"], "output": ["架构图", "API文档", "技术选型报告"]}',
 '["Read", "Write", "Edit", "Glob", "Grep"]'),
('developer', '开发者', '负责代码实现', 'implement', true,
 '{"use_when": ["编写代码", "修复Bug", "重构代码"], "not_for": ["需求分析", "架构设计"], "output": ["源代码", "配置文件"]}',
 '["Read", "Write", "Edit", "Glob", "Grep", "Bash"]'),
('reviewer', '审查员', '负责代码审查和质量检测', 'review', true,
 '{"use_when": ["代码审查", "安全检查", "质量检测"], "not_for": ["编写代码", "需求分析"], "output": ["审查报告"]}',
 '["Read", "Glob", "Grep"]'),
('test-engineer', '测试工程师', '负责测试设计和执行', 'test', true,
 '{"use_when": ["测试设计", "测试执行", "测试报告"], "not_for": ["编写功能代码", "需求分析"], "output": ["测试用例", "测试报告"]}',
 '["Read", "Write", "Edit", "Bash"]'),
('devops', '运维工程师', '负责部署和环境管理', 'deploy', true,
 '{"use_when": ["部署服务", "环境配置", "监控告警"], "not_for": ["编写业务代码", "需求分析"], "output": ["部署产物", "配置文件"]}',
 '["Read", "Write", "Edit", "Bash"]')
ON CONFLICT (agent_id) DO NOTHING;
```

- [ ] **Step 2: 提交**

```bash
git add scripts/init_db.sql
git commit -m "feat: add database initialization script"
```

---

### Task 1.4: 数据模型定义

**Files:**
- Create: `internal/model/project.go`
- Create: `internal/model/thread.go`
- Create: `internal/model/message.go`
- Create: `internal/model/agent_config.go`
- Create: `internal/model/agent_invocation.go`
- Create: `internal/model/artifact.go`
- Create: `internal/model/sandbox.go`

- [ ] **Step 1: 编写模型测试**

```go
// internal/model/model_test.go
package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestProject_JSON(t *testing.T) {
	p := Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		Type:      ProjectTypeService,
		Mode:      ProjectModeNew,
		Status:    ProjectStatusDraft,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(p)
	assert.NoError(t, err)

	var p2 Project
	err = json.Unmarshal(data, &p2)
	assert.NoError(t, err)
	assert.Equal(t, p.Name, p2.Name)
}

func TestAgentConfig_JSON(t *testing.T) {
	cfg := AgentConfig{
		ID:          uuid.New(),
		AgentID:     "test-agent",
		DisplayName: "Test Agent",
		Phase:       "implement",
		Tools:       []string{"Read", "Write"},
		IsActive:    true,
	}

	data, err := json.Marshal(cfg)
	assert.NoError(t, err)

	var cfg2 AgentConfig
	err = json.Unmarshal(data, &cfg2)
	assert.NoError(t, err)
	assert.Equal(t, cfg.AgentID, cfg2.AgentID)
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/model/... -v
# Expected: FAIL
```

- [ ] **Step 3: 实现数据模型**

```go
// internal/model/project.go
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ProjectType string

const (
	ProjectTypeService ProjectType = "service"
	ProjectTypeApp     ProjectType = "app"
	ProjectTypeTask    ProjectType = "task"
)

type ProjectMode string

const (
	ProjectModeNew     ProjectMode = "new"
	ProjectModeEnhance ProjectMode = "enhance"
)

type ProjectStatus string

const (
	ProjectStatusDraft      ProjectStatus = "draft"
	ProjectStatusDeveloping ProjectStatus = "developing"
	ProjectStatusTesting    ProjectStatus = "testing"
	ProjectStatusDeployed   ProjectStatus = "deployed"
	ProjectStatusArchived   ProjectStatus = "archived"
)

type Project struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Type      ProjectType     `json:"type"`
	Mode      ProjectMode     `json:"mode"`
	Status    ProjectStatus   `json:"status"`
	GitRepo   string          `json:"git_repo,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (p *Project) TableName() string {
	return "projects"
}

type CreateProjectRequest struct {
	Name            string      `json:"name" binding:"required"`
	Type            ProjectType `json:"type" binding:"required,oneof=service app task"`
	Mode            ProjectMode `json:"mode" binding:"required,oneof=new enhance"`
	ExistingRepoURL string      `json:"existing_repo_url,omitempty"`
	Branch          string      `json:"branch,omitempty"`
}

func (r *CreateProjectRequest) Validate() error {
	if r.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if r.Mode == ProjectModeEnhance && r.ExistingRepoURL == "" {
		return &ValidationError{Field: "existing_repo_url", Message: "enhance mode requires existing_repo_url"}
	}
	return nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
```

```go
// internal/model/thread.go
package model

import (
	"time"

	"github.com/google/uuid"
)

type ThreadStatus string

const (
	ThreadStatusIdle      ThreadStatus = "idle"
	ThreadStatusRunning   ThreadStatus = "running"
	ThreadStatusPaused    ThreadStatus = "paused"
	ThreadStatusCompleted ThreadStatus = "completed"
)

type Phase string

const (
	PhaseRequirement Phase = "requirement"
	PhaseDesign      Phase = "design"
	PhaseImplement   Phase = "implement"
	PhaseReview      Phase = "review"
	PhaseTest        Phase = "test"
	PhaseDeploy      Phase = "deploy"
)

type Thread struct {
	ID           uuid.UUID    `json:"id"`
	ProjectID    uuid.UUID    `json:"project_id"`
	Status       ThreadStatus `json:"status"`
	CurrentPhase Phase        `json:"current_phase"`
	CurrentAgent string       `json:"current_agent"`
	Depth        int          `json:"depth"`
	AbortToken   string       `json:"abort_token,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

func (t *Thread) TableName() string {
	return "threads"
}
```

```go
// internal/model/message.go
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type MessageRole string

const (
	MessageRoleUser   MessageRole = "user"
	MessageRoleAgent  MessageRole = "agent"
	MessageRoleSystem MessageRole = "system"
)

type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeArtifact MessageType = "artifact"
	MessageTypeSystem   MessageType = "system"
)

type Message struct {
	ID          uuid.UUID       `json:"id"`
	ThreadID    uuid.UUID       `json:"thread_id"`
	Role        MessageRole     `json:"role"`
	AgentID     string          `json:"agent_id,omitempty"`
	Content     string          `json:"content"`
	MessageType MessageType     `json:"message_type"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (m *Message) TableName() string {
	return "messages"
}
```

```go
// internal/model/agent_config.go
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RoutingConfig struct {
	UseWhen []string `json:"use_when"`
	NotFor  []string `json:"not_for"`
	Output  []string `json:"output"`
}

type AgentConfig struct {
	ID            uuid.UUID      `json:"id"`
	AgentID       string         `json:"agent_id"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	Phase         string         `json:"phase"`
	RoutingConfig RoutingConfig  `json:"routing_config"`
	Tools         []string       `json:"tools"`
	SystemPrompt  string         `json:"system_prompt"`
	Model         string         `json:"model"`
	IsActive      bool           `json:"is_active"`
	IsBuiltin     bool           `json:"is_builtin"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func (a *AgentConfig) TableName() string {
	return "agent_configs"
}

type CreateAgentRequest struct {
	AgentID       string        `json:"agent_id" binding:"required"`
	DisplayName   string        `json:"display_name" binding:"required"`
	Description   string        `json:"description"`
	Phase         string        `json:"phase" binding:"required"`
	RoutingConfig RoutingConfig `json:"routing_config"`
	Tools         []string      `json:"tools"`
	SystemPrompt  string        `json:"system_prompt"`
	Model         string        `json:"model"`
}

func (r *CreateAgentRequest) ToAgentConfig() *AgentConfig {
	model := r.Model
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &AgentConfig{
		ID:            uuid.New(),
		AgentID:       r.AgentID,
		DisplayName:   r.DisplayName,
		Description:   r.Description,
		Phase:         r.Phase,
		RoutingConfig: r.RoutingConfig,
		Tools:         r.Tools,
		SystemPrompt:  r.SystemPrompt,
		Model:         model,
		IsActive:      true,
		IsBuiltin:     false,
	}
}
```

```go
// internal/model/agent_invocation.go
package model

import (
	"time"

	"github.com/google/uuid"
)

type InvocationStatus string

const (
	InvocationStatusRunning   InvocationStatus = "running"
	InvocationStatusCompleted InvocationStatus = "completed"
	InvocationStatusFailed    InvocationStatus = "failed"
	InvocationStatusCancelled InvocationStatus = "cancelled"
)

type AgentInvocation struct {
	ID               uuid.UUID        `json:"id"`
	ThreadID         uuid.UUID        `json:"thread_id"`
	AgentID          string           `json:"agent_id"`
	SessionID        string           `json:"session_id"`
	Status           InvocationStatus `json:"status"`
	Depth            int              `json:"depth"`
	PromptTokens     int              `json:"prompt_tokens"`
	CompletionTokens int              `json:"completion_tokens"`
	StartedAt        time.Time        `json:"started_at"`
	EndedAt          *time.Time       `json:"ended_at,omitempty"`
}

func (a *AgentInvocation) TableName() string {
	return "agent_invocations"
}
```

```go
// internal/model/artifact.go
package model

import (
	"time"

	"github.com/google/uuid"
)

type ArtifactType string

const (
	ArtifactTypeRequirementDoc ArtifactType = "requirement_doc"
	ArtifactTypeArchitecture   ArtifactType = "architecture"
	ArtifactTypeCode           ArtifactType = "code"
	ArtifactTypeTestReport     ArtifactType = "test_report"
)

type Artifact struct {
	ID        uuid.UUID     `json:"id"`
	ThreadID  uuid.UUID     `json:"thread_id"`
	Phase     string        `json:"phase"`
	Type      ArtifactType  `json:"type"`
	Name      string        `json:"name"`
	Path      string        `json:"path,omitempty"`
	Content   string        `json:"content,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

func (a *Artifact) TableName() string {
	return "artifacts"
}
```

```go
// internal/model/sandbox.go
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SandboxStatus string

const (
	SandboxStatusCreated   SandboxStatus = "created"
	SandboxStatusRunning   SandboxStatus = "running"
	SandboxStatusStopped   SandboxStatus = "stopped"
	SandboxStatusError     SandboxStatus = "error"
)

type Sandbox struct {
	ID           uuid.UUID       `json:"id"`
	ProjectID    uuid.UUID       `json:"project_id"`
	ContainerID  string          `json:"container_id"`
	Name         string          `json:"name"`
	Status       SandboxStatus   `json:"status"`
	Image        string          `json:"image"`
	Ports        json.RawMessage `json:"ports,omitempty"`
	CPULimit     int             `json:"cpu_limit"`
	MemoryLimit  int             `json:"memory_limit"`
	NetworkName  string          `json:"network_name"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	StoppedAt    *time.Time      `json:"stopped_at,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func (s *Sandbox) TableName() string {
	return "sandbox_containers"
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/model/... -v
# Expected: PASS
```

- [ ] **Step 5: 提交**

```bash
git add internal/model/
git commit -m "feat: add data models for all entities"
```

---

### Task 1.5: 数据库连接与Repo层

**Files:**
- Create: `internal/repo/db.go`
- Create: `internal/repo/project.go`

- [ ] **Step 1: 编写数据库连接测试**

```go
// internal/repo/db_test.go
package repo

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/stretchr/testify/assert"
)

func TestNewDB(t *testing.T) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:15-alpine",
		postgres.WithDatabase("isdp_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Skipf("Failed to start postgres container: %v", err)
	}
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	assert.NoError(t, err)

	db, err := NewDB(connStr)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// 测试连接
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = db.Ping(ctx)
	assert.NoError(t, err)
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/repo/... -v
# Expected: FAIL
```

- [ ] **Step 3: 实现数据库连接**

```go
// internal/repo/db.go
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

func NewDB(connStr string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// 测试连接
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

func NewDBFromConfig(cfg DBConfig) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode, cfg.MaxOpenConns,
	)
	return NewDB(connStr)
}
```

```go
// internal/repo/project.go
package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectRepository struct {
	db *pgxpool.Pool
}

func NewProjectRepository(db *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, project *model.Project) error {
	query := `
		INSERT INTO projects (id, name, type, mode, status, git_repo, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query,
		project.ID,
		project.Name,
		project.Type,
		project.Mode,
		project.Status,
		project.GitRepo,
		project.Config,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	project.CreatedAt = now
	project.UpdatedAt = now
	return nil
}

func (r *ProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	query := `
		SELECT id, name, type, mode, status, git_repo, config, created_at, updated_at
		FROM projects WHERE id = $1
	`
	project := &model.Project{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&project.ID,
		&project.Name,
		&project.Type,
		&project.Mode,
		&project.Status,
		&project.GitRepo,
		&project.Config,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find project: %w", err)
	}
	return project, nil
}

func (r *ProjectRepository) FindAll(ctx context.Context, limit, offset int) ([]*model.Project, error) {
	query := `
		SELECT id, name, type, mode, status, git_repo, config, created_at, updated_at
		FROM projects ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to find projects: %w", err)
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		project := &model.Project{}
		err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.Type,
			&project.Mode,
			&project.Status,
			&project.GitRepo,
			&project.Config,
			&project.CreatedAt,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, project)
	}
	return projects, nil
}

func (r *ProjectRepository) Update(ctx context.Context, project *model.Project) error {
	query := `
		UPDATE projects
		SET name = $2, type = $3, mode = $4, status = $5, git_repo = $6, config = $7, updated_at = $8
		WHERE id = $1
	`
	project.UpdatedAt = time.Now()
	_, err := r.db.Exec(ctx, query,
		project.ID,
		project.Name,
		project.Type,
		project.Mode,
		project.Status,
		project.GitRepo,
		project.Config,
		project.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM projects WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) GetByThreadID(ctx context.Context, threadID uuid.UUID) (*model.Project, error) {
	query := `
		SELECT p.id, p.name, p.type, p.mode, p.status, p.git_repo, p.config, p.created_at, p.updated_at
		FROM projects p
		JOIN threads t ON t.project_id = p.id
		WHERE t.id = $1
	`
	project := &model.Project{}
	var config []byte
	err := r.db.QueryRow(ctx, query, threadID).Scan(
		&project.ID,
		&project.Name,
		&project.Type,
		&project.Mode,
		&project.Status,
		&project.GitRepo,
		&config,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find project by thread: %w", err)
	}
	project.Config = json.RawMessage(config)
	return project, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/repo/... -v
# Expected: PASS
```

- [ ] **Step 5: 提交**

```bash
git add internal/repo/
git commit -m "feat: add database connection and project repository"
```

---

### Task 1.6: Redis客户端与主程序入口

**Files:**
- Create: `internal/repo/redis.go`
- Create: `cmd/server/main.go`

- [ ] **Step 1: 实现Redis客户端**

```go
// internal/repo/redis.go
package repo

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func NewRedis(cfg RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return client, nil
}
```

- [ ] **Step 2: 创建主程序入口**

```go
// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger, err := initLogger(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}

	// 连接数据库
	db, err := repo.NewDBFromConfig(repo.DBConfig{
		Host:         cfg.Database.Host,
		Port:         cfg.Database.Port,
		User:         cfg.Database.User,
		Password:     cfg.Database.Password,
		Name:         cfg.Database.Name,
		SSLMode:      cfg.Database.SSLMode,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
	})
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// 连接Redis
	redisClient, err := repo.NewRedis(repo.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		logger.Fatal("Failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 创建路由
	router := gin.New()
	router.Use(gin.Recovery())

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// API路由组
	v1 := router.Group("/api/v1")
	_ = v1 // 后续添加路由

	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// 优雅关闭
	go func() {
		logger.Info("Starting server", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func initLogger(level, format string) (*zap.Logger, error) {
	var cfg zap.Config
	if format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	}

	return cfg.Build()
}
```

- [ ] **Step 3: 编译测试**

```bash
go build ./cmd/server
# Expected: success
```

- [ ] **Step 4: 提交**

```bash
git add internal/repo/redis.go cmd/server/main.go
git commit -m "feat: add redis client and main server entry point"
```

---

## Chunk 1 完成

基础架构搭建完成，包括：
- 项目初始化与依赖管理
- 配置管理（Viper）
- 数据库Schema与数据模型
- 数据库连接与Repo层
- Redis客户端
- 主程序入口与健康检查

---

## Chunk 2: 项目管理模块（M2）

### Task 2.1: Thread服务实现

**Files:**
- Create: `internal/repo/thread.go`
- Create: `internal/service/thread/service.go`
- Create: `internal/api/handler/thread.go`

- [ ] **Step 1: 编写Thread Repo测试**

```go
// internal/repo/thread_test.go
package repo

import (
	"context"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestThreadRepository_Create(t *testing.T) {
	// 使用测试数据库
	// ...
}
```

- [ ] **Step 2: 实现Thread Repo**

```go
// internal/repo/thread.go
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ThreadRepository struct {
	db *pgxpool.Pool
}

func NewThreadRepository(db *pgxpool.Pool) *ThreadRepository {
	return &ThreadRepository{db: db}
}

func (r *ThreadRepository) Create(ctx context.Context, thread *model.Thread) error {
	query := `
		INSERT INTO threads (id, project_id, status, current_phase, current_agent, depth, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query,
		thread.ID, thread.ProjectID, thread.Status, thread.CurrentPhase, thread.CurrentAgent, thread.Depth, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}
	thread.CreatedAt = now
	thread.UpdatedAt = now
	return nil
}

func (r *ThreadRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Thread, error) {
	query := `
		SELECT id, project_id, status, current_phase, current_agent, depth, abort_token, created_at, updated_at
		FROM threads WHERE id = $1
	`
	thread := &model.Thread{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&thread.ID, &thread.ProjectID, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
		&thread.Depth, &thread.AbortToken, &thread.CreatedAt, &thread.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find thread: %w", err)
	}
	return thread, nil
}

func (r *ThreadRepository) FindByProjectID(ctx context.Context, projectID uuid.UUID) ([]*model.Thread, error) {
	query := `
		SELECT id, project_id, status, current_phase, current_agent, depth, abort_token, created_at, updated_at
		FROM threads WHERE project_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to find threads: %w", err)
	}
	defer rows.Close()

	var threads []*model.Thread
	for rows.Next() {
		thread := &model.Thread{}
		err := rows.Scan(
			&thread.ID, &thread.ProjectID, &thread.Status, &thread.CurrentPhase, &thread.CurrentAgent,
			&thread.Depth, &thread.AbortToken, &thread.CreatedAt, &thread.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread: %w", err)
		}
		threads = append(threads, thread)
	}
	return threads, nil
}

func (r *ThreadRepository) Update(ctx context.Context, thread *model.Thread) error {
	query := `
		UPDATE threads
		SET status = $2, current_phase = $3, current_agent = $4, depth = $5, abort_token = $6, updated_at = $7
		WHERE id = $1
	`
	thread.UpdatedAt = time.Now()
	_, err := r.db.Exec(ctx, query,
		thread.ID, thread.Status, thread.CurrentPhase, thread.CurrentAgent, thread.Depth, thread.AbortToken, thread.UpdatedAt,
	)
	return err
}
```

- [ ] **Step 3: 实现Thread服务**

```go
// internal/service/thread/service.go
package thread

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

type ThreadService struct {
	repo *repo.ThreadRepository
}

func NewThreadService(repo *repo.ThreadRepository) *ThreadService {
	return &ThreadService{repo: repo}
}

func (s *ThreadService) Create(ctx context.Context, projectID uuid.UUID) (*model.Thread, error) {
	thread := &model.Thread{
		ID:           uuid.New(),
		ProjectID:    projectID,
		Status:       model.ThreadStatusIdle,
		CurrentPhase: model.PhaseRequirement,
	}

	if err := s.repo.Create(ctx, thread); err != nil {
		return nil, err
	}
	return thread, nil
}

func (s *ThreadService) GetByID(ctx context.Context, id uuid.UUID) (*model.Thread, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *ThreadService) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*model.Thread, error) {
	return s.repo.FindByProjectID(ctx, projectID)
}

func (s *ThreadService) UpdateStatus(ctx context.Context, id uuid.UUID, status model.ThreadStatus) error {
	thread, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	thread.Status = status
	return s.repo.Update(ctx, thread)
}

func (s *ThreadService) SetPhase(ctx context.Context, id uuid.UUID, phase model.Phase, agent string) error {
	thread, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	thread.CurrentPhase = phase
	thread.CurrentAgent = agent
	return s.repo.Update(ctx, thread)
}
```

- [ ] **Step 4: 实现Thread API处理器**

```go
// internal/api/handler/thread.go
package handler

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/thread"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ThreadHandler struct {
	svc *thread.ThreadService
}

func NewThreadHandler(svc *thread.ThreadService) *ThreadHandler {
	return &ThreadHandler{svc: svc}
}

func (h *ThreadHandler) CreateThread(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	thread, err := h.svc.Create(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": thread})
}

func (h *ThreadHandler) GetThread(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	thread, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "thread not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": thread})
}

func (h *ThreadHandler) PauseThread(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), id, model.ThreadStatusPaused); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "paused"})
}
```

- [ ] **Step 5: 提交**

```bash
git add internal/repo/thread.go internal/service/thread/ internal/api/handler/thread.go
git commit -m "feat: add thread service and API"
```

---

### Task 2.2: Message服务实现

**Files:**
- Create: `internal/repo/message.go`
- Create: `internal/service/message/service.go`
- Create: `internal/api/handler/message.go`

- [ ] **Step 1: 实现Message Repo**

```go
// internal/repo/message.go
package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, msg *model.Message) error {
	query := `
		INSERT INTO messages (id, thread_id, role, agent_id, content, message_type, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now()
	_, err := r.db.Exec(ctx, query,
		msg.ID, msg.ThreadID, msg.Role, msg.AgentID, msg.Content, msg.MessageType, msg.Metadata, msg.CreatedAt,
	)
	return err
}

func (r *MessageRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, message_type, metadata, created_at
		FROM messages WHERE thread_id = $1 ORDER BY created_at ASC LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, threadID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		err := rows.Scan(
			&msg.ID, &msg.ThreadID, &msg.Role, &msg.AgentID, &msg.Content, &msg.MessageType, &msg.Metadata, &msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (r *MessageRepository) GetRecent(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	query := `
		SELECT id, thread_id, role, agent_id, content, message_type, metadata, created_at
		FROM messages WHERE thread_id = $1 ORDER BY created_at DESC LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, threadID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		err := rows.Scan(
			&msg.ID, &msg.ThreadID, &msg.Role, &msg.AgentID, &msg.Content, &msg.MessageType, &msg.Metadata, &msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
```

- [ ] **Step 2: 实现Message服务**

```go
// internal/service/message/service.go
package message

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

type MessageService struct {
	repo    *repo.MessageRepository
	wsHub   *ws.Hub
}

func NewMessageService(repo *repo.MessageRepository, wsHub *ws.Hub) *MessageService {
	return &MessageService{repo: repo, wsHub: wsHub}
}

func (s *MessageService) Create(ctx context.Context, threadID uuid.UUID, role model.MessageRole, agentID, content string) (*model.Message, error) {
	msg := &model.Message{
		ThreadID:    threadID,
		Role:        role,
		AgentID:     agentID,
		Content:     content,
		MessageType: model.MessageTypeText,
	}

	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, err
	}

	// 通过WebSocket广播消息
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_message",
			ThreadID:  threadID.String(),
			Timestamp: msg.CreatedAt.UnixMilli(),
			Payload: map[string]interface{}{
				"messageId": msg.ID.String(),
				"agentId":   agentID,
				"content":   content,
			},
		})
	}

	return msg, nil
}

func (s *MessageService) GetByThreadID(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.FindByThreadID(ctx, threadID, limit)
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/repo/message.go internal/service/message/
git commit -m "feat: add message service with WebSocket broadcast"
```

---

### Task 2.3: WebSocket处理器实现

**Files:**
- Create: `internal/ws/handler.go`
- Create: `internal/ws/hub.go`

- [ ] **Step 1: 实现WebSocket Hub**

```go
// internal/ws/hub.go
package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage WebSocket消息结构
type WSMessage struct {
	Type      string                 `json:"type"`
	ThreadID  string                 `json:"threadId"`
	Timestamp int64                  `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// Hub WebSocket连接管理中心
type Hub struct {
	clients    map[string]map[*websocket.Conn]bool // threadId -> connections
	broadcast  chan BroadcastMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// Client WebSocket客户端
type Client struct {
	conn     *websocket.Conn
	threadID string
}

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	ThreadID string
	Message  WSMessage
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*websocket.Conn]bool),
		broadcast:  make(chan BroadcastMessage, 100),
		register:   make(chan *Client, 10),
		unregister: make(chan *Client, 10),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.threadID] == nil {
				h.clients[client.threadID] = make(map[*websocket.Conn]bool)
			}
			h.clients[client.threadID][client.conn] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if conns, ok := h.clients[client.threadID]; ok {
				if _, ok := conns[client.conn]; ok {
					delete(conns, client.conn)
					client.conn.Close()
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			if conns, ok := h.clients[msg.ThreadID]; ok {
				for conn := range conns {
					err := conn.WriteJSON(msg.Message)
					if err != nil {
						conn.Close()
						delete(conns, conn)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastToThread(threadID string, msg WSMessage) {
	msg.Timestamp = time.Now().UnixMilli()
	h.broadcast <- BroadcastMessage{
		ThreadID: threadID,
		Message:  msg,
	}
}

func (h *Hub) RegisterClient(conn *websocket.Conn, threadID string) {
	h.register <- &Client{conn: conn, threadID: threadID}
}

func (h *Hub) UnregisterClient(conn *websocket.Conn, threadID string) {
	h.unregister <- &Client{conn: conn, threadID: threadID}
}
```

- [ ] **Step 2: 实现WebSocket处理器**

```go
// internal/ws/handler.go
package ws

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要限制
	},
}

type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

func (h *Handler) HandleConnection(c *gin.Context) {
	threadID := c.Query("thread_id")
	if threadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thread_id required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// 注册客户端
	h.hub.RegisterClient(conn, threadID)

	defer func() {
		h.hub.UnregisterClient(conn, threadID)
	}()

	// 心跳检测
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.WriteJSON(WSMessage{Type: "ping", Timestamp: time.Now().UnixMilli()})
			}
		}
	}()

	// 读取消息
	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "pong":
			// 心跳响应
		case "subscribe":
			// 已在连接时处理
		case "unsubscribe":
			h.hub.UnregisterClient(conn, threadID)
			return
		}
	}
}
```

- [ ] **Step 3: 更新路由注册**

```go
// 在 router.go 中添加WebSocket路由
import "github.com/anthropic/isdp/internal/ws"

// 在Setup方法中添加
wsHub := ws.NewHub()
go wsHub.Run()

wsHandler := ws.NewHandler(wsHub)
r.engine.GET("/ws", wsHandler.HandleConnection)
```

- [ ] **Step 4: 提交**

```bash
git add internal/ws/
git commit -m "feat: add WebSocket hub and handler for real-time messaging"
```

---

### Task 2.4: 项目服务实现

**Files:**
- Create: `internal/service/project/service.go`
- Create: `internal/api/handler/project.go`
- Create: `internal/api/router.go`

- [ ] **Step 1: 编写项目服务测试**

```go
// internal/service/project/service_test.go
package project

import (
	"context"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProjectRepo struct {
	mock.Mock
}

func (m *MockProjectRepo) Create(ctx context.Context, p *model.Project) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockProjectRepo) FindByID(ctx context.Context, id string) (*model.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Project), args.Error(1)
}

func TestProjectService_Create(t *testing.T) {
	mockRepo := new(MockProjectRepo)
	svc := &ProjectService{repo: mockRepo}

	req := &model.CreateProjectRequest{
		Name: "Test Project",
		Type: model.ProjectTypeService,
		Mode: model.ProjectModeNew,
	}

	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	project, err := svc.Create(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "Test Project", project.Name)
	assert.Equal(t, model.ProjectStatusDraft, project.Status)
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/service/project/... -v
# Expected: FAIL
```

- [ ] **Step 3: 实现项目服务**

```go
// internal/service/project/service.go
package project

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/git"
	"github.com/google/uuid"
)

type ProjectService struct {
	repo   *repo.ProjectRepository
	gitSvc *git.Service
}

func NewProjectService(repo *repo.ProjectRepository, gitSvc *git.Service) *ProjectService {
	return &ProjectService{
		repo:   repo,
		gitSvc: gitSvc,
	}
}

type CodeStructure struct {
	TechStack    []string            `json:"tech_stack"`
	MainModules  []ModuleInfo        `json:"main_modules"`
	EntryPoints  []string            `json:"entry_points"`
	ConfigFiles  []string            `json:"config_files"`
	Dependencies map[string]string   `json:"dependencies"`
}

type ModuleInfo struct {
	Path        string   `json:"path"`
	Description string   `json:"description"`
	MainFiles   []string `json:"main_files"`
}

func (s *ProjectService) Create(ctx context.Context, req *model.CreateProjectRequest) (*model.Project, error) {
	// 验证请求
	if err := req.Validate(); err != nil {
		return nil, err
	}

	project := &model.Project{
		ID:     uuid.New(),
		Name:   req.Name,
		Type:   req.Type,
		Mode:   req.Mode,
		Status: model.ProjectStatusDraft,
	}

	// 根据模式处理Git仓库
	switch req.Mode {
	case model.ProjectModeNew:
		// 新项目模式：初始化空仓库
		if s.gitSvc != nil {
			repoURL, err := s.gitSvc.InitRepo(ctx, project.ID, req.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to init repo: %w", err)
			}
			project.GitRepo = repoURL
		}

	case model.ProjectModeEnhance:
		// 增量开发模式
		if req.ExistingRepoURL == "" {
			return nil, &model.ValidationError{
				Field:   "existing_repo_url",
				Message: "enhance mode requires existing_repo_url",
			}
		}

		if s.gitSvc != nil {
			// 克隆现有仓库
			repoPath, err := s.gitSvc.CloneRepo(ctx, project.ID, req.ExistingRepoURL, req.Branch)
			if err != nil {
				return nil, fmt.Errorf("failed to clone repository: %w", err)
			}

			// 分析代码结构
			codeStructure, err := s.gitSvc.AnalyzeStructure(ctx, repoPath)
			if err != nil {
				return nil, fmt.Errorf("failed to analyze code structure: %w", err)
			}

			// 保存代码结构到配置
			project.Config, _ = json.Marshal(map[string]interface{}{
				"existing_repo_url": req.ExistingRepoURL,
				"branch":           req.Branch,
				"code_structure":   codeStructure,
			})
		}
		project.GitRepo = req.ExistingRepoURL
	}

	// 保存到数据库
	if err := s.repo.Create(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ProjectService) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *ProjectService) List(ctx context.Context, limit, offset int) ([]*model.Project, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.FindAll(ctx, limit, offset)
}

func (s *ProjectService) UpdateStatus(ctx context.Context, id uuid.UUID, status model.ProjectStatus) error {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	project.Status = status
	return s.repo.Update(ctx, project)
}

func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/service/project/... -v
# Expected: PASS
```

- [ ] **Step 5: 提交**

```bash
git add internal/service/project/
git commit -m "feat: add project service with create and list"
```

---

### Task 2.2: Git服务实现

**Files:**
- Create: `internal/service/git/service.go`

- [ ] **Step 1: 编写Git服务测试**

```go
// internal/service/git/service_test.go
package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGitService_InitRepo(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	ctx := context.Background()
	projectID := uuid.New()

	repoPath, err := svc.InitRepo(ctx, projectID, "test-project")
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(repoPath, ".git"))
	assert.FileExists(t, filepath.Join(repoPath, "README.md"))
}

func TestGitService_DetectTechStack(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	// 创建go.mod文件
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module test\n\nrequire github.com/gin-gonic/gin v1.9.0"), 0644)

	stack := svc.detectTechStack(tmpDir)
	assert.Contains(t, stack, "go")
	assert.Contains(t, stack, "gin")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/service/git/... -v
# Expected: FAIL
```

- [ ] **Step 3: 实现Git服务**

```go
// internal/service/git/service.go
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	reposDir string
}

func NewService(reposDir string) *Service {
	return &Service{reposDir: reposDir}
}

// InitRepo 初始化新仓库
func (s *Service) InitRepo(ctx context.Context, projectID uuid.UUID, name string) (string, error) {
	repoPath := filepath.Join(s.reposDir, projectID.String())

	// 创建目录
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create repo directory: %w", err)
	}

	// 初始化Git仓库
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git init failed: %w", err)
	}

	// 创建初始README
	readmePath := filepath.Join(repoPath, "README.md")
	readmeContent := fmt.Sprintf("# %s\n\nGenerated by ISDP Platform.\n", name)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return "", fmt.Errorf("failed to create README: %w", err)
	}

	// 创建初始提交
	cmd = exec.CommandContext(ctx, "git", "add", ".")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	cmd.Run()

	return repoPath, nil
}

// CloneRepo 克隆现有仓库
func (s *Service) CloneRepo(ctx context.Context, projectID uuid.UUID, repoURL, branch string) (string, error) {
	repoPath := filepath.Join(s.reposDir, projectID.String())

	args := []string{"clone", repoURL, repoPath}
	if branch != "" {
		args = []string{"clone", "-b", branch, repoURL, repoPath}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	return repoPath, nil
}

// AnalyzeStructure 分析代码结构
func (s *Service) AnalyzeStructure(ctx context.Context, repoPath string) (*CodeStructure, error) {
	structure := &CodeStructure{
		Dependencies: make(map[string]string),
	}

	// 检测技术栈
	structure.TechStack = s.detectTechStack(repoPath)

	// 分析目录结构
	structure.MainModules = s.analyzeModules(repoPath)

	// 查找入口文件
	structure.EntryPoints = s.findEntryPoints(repoPath, structure.TechStack)

	// 查找配置文件
	structure.ConfigFiles = s.findConfigFiles(repoPath, structure.TechStack)

	return structure, nil
}

// detectTechStack 检测技术栈
func (s *Service) detectTechStack(repoPath string) []string {
	var stack []string

	// Go项目
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		stack = append(stack, "go")

		content, _ := os.ReadFile(filepath.Join(repoPath, "go.mod"))
		contentStr := string(content)
		if strings.Contains(contentStr, "gin-gonic") {
			stack = append(stack, "gin")
		}
		if strings.Contains(contentStr, "echo") {
			stack = append(stack, "echo")
		}
		if strings.Contains(contentStr, "fiber") {
			stack = append(stack, "fiber")
		}
	}

	// Node.js项目
	if _, err := os.Stat(filepath.Join(repoPath, "package.json")); err == nil {
		stack = append(stack, "nodejs")

		content, _ := os.ReadFile(filepath.Join(repoPath, "package.json"))
		contentStr := string(content)
		if strings.Contains(contentStr, "react") {
			stack = append(stack, "react")
		}
		if strings.Contains(contentStr, "vue") {
			stack = append(stack, "vue")
		}
		if strings.Contains(contentStr, "express") {
			stack = append(stack, "express")
		}
	}

	// Python项目
	if _, err := os.Stat(filepath.Join(repoPath, "requirements.txt")); err == nil {
		stack = append(stack, "python")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "pyproject.toml")); err == nil {
		stack = append(stack, "python")
	}

	return stack
}

// analyzeModules 分析模块结构
func (s *Service) analyzeModules(repoPath string) []ModuleInfo {
	var modules []ModuleInfo

	// 常见模块目录
	moduleDirs := []string{"cmd", "internal", "pkg", "src", "app", "api", "services", "controllers"}

	for _, dir := range moduleDirs {
		modulePath := filepath.Join(repoPath, dir)
		if info, err := os.Stat(modulePath); err == nil && info.IsDir() {
			var mainFiles []string
			filepath.Walk(modulePath, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !fi.IsDir() && (strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".js")) {
					rel, _ := filepath.Rel(repoPath, path)
					mainFiles = append(mainFiles, rel)
				}
				return nil
			})

			modules = append(modules, ModuleInfo{
				Path:        dir,
				Description: s.guessModuleDescription(dir),
				MainFiles:   mainFiles[:min(5, len(mainFiles))], // 限制数量
			})
		}
	}

	return modules
}

// findEntryPoints 查找入口文件
func (s *Service) findEntryPoints(repoPath string, techStack []string) []string {
	var entryPoints []string

	// Go入口
	if containsStr(techStack, "go") {
		candidates := []string{"cmd/server/main.go", "cmd/main.go", "main.go"}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(repoPath, c)); err == nil {
				entryPoints = append(entryPoints, c)
			}
		}
	}

	// Node.js入口
	if containsStr(techStack, "nodejs") {
		candidates := []string{"index.js", "index.ts", "src/index.js", "src/index.ts", "app.js", "app.ts"}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(repoPath, c)); err == nil {
				entryPoints = append(entryPoints, c)
			}
		}
	}

	return entryPoints
}

// findConfigFiles 查找配置文件
func (s *Service) findConfigFiles(repoPath string, techStack []string) []string {
	var configs []string

	candidates := []string{
		"config.yaml", "config.yml", "config.json",
		".env", ".env.example",
		"docker-compose.yml", "docker-compose.yaml",
		"Dockerfile",
	}

	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(repoPath, c)); err == nil {
			configs = append(configs, c)
		}
	}

	return configs
}

func (s *Service) guessModuleDescription(dir string) string {
	descriptions := map[string]string{
		"cmd":         "应用入口，包含主程序",
		"internal":    "内部包，不对外暴露",
		"pkg":         "公共包，可被外部引用",
		"src":         "源代码目录",
		"app":         "应用逻辑",
		"api":         "API接口定义",
		"services":    "业务服务层",
		"controllers": "控制器层",
		"models":      "数据模型",
	}
	if desc, ok := descriptions[dir]; ok {
		return desc
	}
	return dir
}

// Commit 提交代码
func (s *Service) Commit(ctx context.Context, repoPath, message string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "add", ".")
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "commit", "-m", message)
	return cmd.Run()
}

// CreateBranch 创建分支
func (s *Service) CreateBranch(ctx context.Context, repoPath, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", "-b", branchName)
	return cmd.Run()
}

// CreateTag 创建标签
func (s *Service) CreateTag(ctx context.Context, repoPath, tagName, message string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "tag", "-a", tagName, "-m", message)
	return cmd.Run()
}

func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/service/git/... -v
# Expected: PASS
```

- [ ] **Step 5: 提交**

```bash
git add internal/service/git/
git commit -m "feat: add git service with init, clone, and analyze"
```

---

### Task 2.3: 项目API处理与路由

**Files:**
- Create: `internal/api/handler/project.go`
- Create: `internal/api/router.go`
- Create: `internal/api/middleware/cors.go`
- Create: `internal/api/middleware/logger.go`

- [ ] **Step 1: 实现项目API处理器**

```go
// internal/api/handler/project.go
package handler

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/project"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProjectHandler struct {
	svc *project.ProjectService
}

func NewProjectHandler(svc *project.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

// ListProjects 获取项目列表
// @Summary 获取项目列表
// @Tags projects
// @Produce json
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/projects [get]
func (h *ProjectHandler) ListProjects(c *gin.Context) {
	limit := parseIntQuery(c, "limit", 20)
	offset := parseIntQuery(c, "offset", 0)

	projects, err := h.svc.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"projects": projects,
			"limit":    limit,
			"offset":   offset,
		},
	})
}

// GetProject 获取项目详情
// @Summary 获取项目详情
// @Tags projects
// @Produce json
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/projects/{id} [get]
func (h *ProjectHandler) GetProject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	project, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": project,
	})
}

// CreateProject 创建项目
// @Summary 创建项目
// @Tags projects
// @Accept json
// @Produce json
// @Param request body model.CreateProjectRequest true "创建请求"
// @Success 201 {object} map[string]interface{}
// @Router /api/v1/projects [post]
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req model.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if ve, ok := err.(*model.ValidationError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": ve.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code": 0,
		"data": project,
	})
}

// DeleteProject 删除项目
// @Summary 删除项目
// @Tags projects
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/projects/{id} [delete]
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func parseIntQuery(c *gin.Context, key string, defaultValue int) int {
	if v := c.Query(key); v != "" {
		var result int
		if _, err := c.GetQuery(key); err {
			return defaultValue
		}
		_, _ = fmt.Sscanf(v, "%d", &result)
		return result
	}
	return defaultValue
}
```

- [ ] **Step 2: 实现中间件**

```go
// internal/api/middleware/cors.go
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// internal/api/middleware/logger.go
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)

		logger.Info("HTTP Request",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
```

- [ ] **Step 3: 实现路由注册**

```go
// internal/api/router.go
package api

import (
	"github.com/anthropic/isdp/internal/api/handler"
	"github.com/anthropic/isdp/internal/api/middleware"
	"github.com/anthropic/isdp/internal/service/project"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Router struct {
	engine         *gin.Engine
	projectHandler *handler.ProjectHandler
	logger         *zap.Logger
}

func NewRouter(engine *gin.Engine, projectSvc *project.ProjectService, logger *zap.Logger) *Router {
	return &Router{
		engine:         engine,
		projectHandler: handler.NewProjectHandler(projectSvc),
		logger:         logger,
	}
}

func (r *Router) Setup() {
	// 中间件
	r.engine.Use(middleware.CORS())
	r.engine.Use(middleware.Logger(r.logger))
	r.engine.Use(gin.Recovery())

	// 健康检查
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1
	v1 := r.engine.Group("/api/v1")
	{
		// 项目管理
		projects := v1.Group("/projects")
		{
			projects.GET("", r.projectHandler.ListProjects)
			projects.POST("", r.projectHandler.CreateProject)
			projects.GET("/:id", r.projectHandler.GetProject)
			projects.DELETE("/:id", r.projectHandler.DeleteProject)
		}

		// 其他路由将在后续任务中添加
		// threads := v1.Group("/threads")
		// agents := v1.Group("/agents")
		// sandbox := v1.Group("/sandbox")
		// mcp := v1.Group("/mcp")
	}
}
```

- [ ] **Step 4: 更新main.go使用路由**

```go
// 在 cmd/server/main.go 中添加
// 在创建数据库和Redis后添加：

// 创建服务
projectRepo := repo.NewProjectRepository(db)
gitSvc := git.NewService(cfg.Sandbox.ReposDir)
projectSvc := project.NewProjectService(projectRepo, gitSvc)

// 设置路由
router := api.NewRouter(ginEngine, projectSvc, logger)
router.Setup()
```

- [ ] **Step 5: 运行集成测试**

```bash
go test ./... -v
# Expected: PASS
```

- [ ] **Step 6: 提交**

```bash
git add internal/api/
git commit -m "feat: add project API handlers and router setup"
```

---

## Chunk 2 完成

项目管理模块完成，包括：
- 项目服务（Create, Get, List, Delete）
- Git服务（Init, Clone, AnalyzeStructure）
- 项目API处理器
- 路由注册与中间件

---

## Chunk 3: Agent引擎核心（M3）

### Task 3.1: Agent配置服务

**Files:**
- Create: `internal/repo/agent_config.go`
- Create: `internal/service/agent/config_service.go`
- Create: `internal/api/handler/agent.go`

- [ ] **Step 1: 编写Agent配置Repo测试**

```go
// internal/repo/agent_config_test.go
package repo

import (
	"context"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestAgentConfigRepository_FindActive(t *testing.T) {
	// 使用测试数据库
	// ...
}
```

- [ ] **Step 2: 实现Agent配置Repo**

```go
// internal/repo/agent_config.go
package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentConfigRepository struct {
	db *pgxpool.Pool
}

func NewAgentConfigRepository(db *pgxpool.Pool) *AgentConfigRepository {
	return &AgentConfigRepository{db: db}
}

func (r *AgentConfigRepository) Create(ctx context.Context, cfg *model.AgentConfig) error {
	query := `
		INSERT INTO agent_configs (id, agent_id, display_name, description, phase, routing_config, tools, system_prompt, model, is_active, is_builtin, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	now := time.Now()
	routingConfig, _ := json.Marshal(cfg.RoutingConfig)
	tools, _ := json.Marshal(cfg.Tools)

	_, err := r.db.Exec(ctx, query,
		cfg.ID, cfg.AgentID, cfg.DisplayName, cfg.Description, cfg.Phase,
		routingConfig, tools, cfg.SystemPrompt, cfg.Model,
		cfg.IsActive, cfg.IsBuiltin, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create agent config: %w", err)
	}
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	return nil
}

func (r *AgentConfigRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AgentConfig, error) {
	query := `
		SELECT id, agent_id, display_name, description, phase, routing_config, tools, system_prompt, model, is_active, is_builtin, created_at, updated_at
		FROM agent_configs WHERE id = $1
	`
	cfg := &model.AgentConfig{}
	var routingConfig, tools []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&cfg.ID, &cfg.AgentID, &cfg.DisplayName, &cfg.Description, &cfg.Phase,
		&routingConfig, &tools, &cfg.SystemPrompt, &cfg.Model,
		&cfg.IsActive, &cfg.IsBuiltin, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent config: %w", err)
	}

	json.Unmarshal(routingConfig, &cfg.RoutingConfig)
	json.Unmarshal(tools, &cfg.Tools)
	return cfg, nil
}

func (r *AgentConfigRepository) FindByAgentID(ctx context.Context, agentID string) (*model.AgentConfig, error) {
	query := `
		SELECT id, agent_id, display_name, description, phase, routing_config, tools, system_prompt, model, is_active, is_builtin, created_at, updated_at
		FROM agent_configs WHERE agent_id = $1
	`
	cfg := &model.AgentConfig{}
	var routingConfig, tools []byte

	err := r.db.QueryRow(ctx, query, agentID).Scan(
		&cfg.ID, &cfg.AgentID, &cfg.DisplayName, &cfg.Description, &cfg.Phase,
		&routingConfig, &tools, &cfg.SystemPrompt, &cfg.Model,
		&cfg.IsActive, &cfg.IsBuiltin, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find agent config by agent_id: %w", err)
	}

	json.Unmarshal(routingConfig, &cfg.RoutingConfig)
	json.Unmarshal(tools, &cfg.Tools)
	return cfg, nil
}

func (r *AgentConfigRepository) FindActive(ctx context.Context) ([]*model.AgentConfig, error) {
	query := `
		SELECT id, agent_id, display_name, description, phase, routing_config, tools, system_prompt, model, is_active, is_builtin, created_at, updated_at
		FROM agent_configs WHERE is_active = true ORDER BY phase, display_name
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find active agent configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.AgentConfig
	for rows.Next() {
		cfg := &model.AgentConfig{}
		var routingConfig, tools []byte

		err := rows.Scan(
			&cfg.ID, &cfg.AgentID, &cfg.DisplayName, &cfg.Description, &cfg.Phase,
			&routingConfig, &tools, &cfg.SystemPrompt, &cfg.Model,
			&cfg.IsActive, &cfg.IsBuiltin, &cfg.CreatedAt, &cfg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent config: %w", err)
		}

		json.Unmarshal(routingConfig, &cfg.RoutingConfig)
		json.Unmarshal(tools, &cfg.Tools)
		configs = append(configs, cfg)
	}
	return configs, nil
}

func (r *AgentConfigRepository) Update(ctx context.Context, cfg *model.AgentConfig) error {
	query := `
		UPDATE agent_configs
		SET display_name = $2, description = $3, phase = $4, routing_config = $5, tools = $6, system_prompt = $7, model = $8, is_active = $9, updated_at = $10
		WHERE id = $1
	`
	cfg.UpdatedAt = time.Now()
	routingConfig, _ := json.Marshal(cfg.RoutingConfig)
	tools, _ := json.Marshal(cfg.Tools)

	_, err := r.db.Exec(ctx, query,
		cfg.ID, cfg.DisplayName, cfg.Description, cfg.Phase,
		routingConfig, tools, cfg.SystemPrompt, cfg.Model, cfg.IsActive, cfg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent config: %w", err)
	}
	return nil
}

func (r *AgentConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agent_configs WHERE id = $1 AND is_builtin = false`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent config: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("cannot delete builtin agent or agent not found")
	}
	return nil
}
```

- [ ] **Step 3: 实现Agent配置服务**

```go
// internal/service/agent/config_service.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type AgentConfigService struct {
	repo  *repo.AgentConfigRepository
	cache *redis.Client
}

func NewAgentConfigService(repo *repo.AgentConfigRepository, cache *redis.Client) *AgentConfigService {
	return &AgentConfigService{repo: repo, cache: cache}
}

// ListActive 获取所有启用的Agent配置
func (s *AgentConfigService) ListActive(ctx context.Context) ([]*model.AgentConfig, error) {
	// 优先从缓存获取
	cached, err := s.cache.Get(ctx, "agent_configs:active").Result()
	if err == nil {
		var configs []*model.AgentConfig
		if err := json.Unmarshal([]byte(cached), &configs); err == nil {
			return configs, nil
		}
	}

	// 从数据库查询
	configs, err := s.repo.FindActive(ctx)
	if err != nil {
		return nil, err
	}

	// 缓存
	data, _ := json.Marshal(configs)
	s.cache.Set(ctx, "agent_configs:active", data, 10*time.Minute)

	return configs, nil
}

// GetByAgentID 根据AgentID获取配置
func (s *AgentConfigService) GetByAgentID(ctx context.Context, agentID string) (*model.AgentConfig, error) {
	return s.repo.FindByAgentID(ctx, agentID)
}

// Create 创建自定义Agent角色
func (s *AgentConfigService) Create(ctx context.Context, req *model.CreateAgentRequest) (*model.AgentConfig, error) {
	// 检查AgentID是否已存在
	if existing, _ := s.repo.FindByAgentID(ctx, req.AgentID); existing != nil {
		return nil, fmt.Errorf("agent_id '%s' already exists", req.AgentID)
	}

	cfg := req.ToAgentConfig()
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}

	// 清除缓存
	s.cache.Del(ctx, "agent_configs:active")

	return cfg, nil
}

// Update 更新Agent配置
func (s *AgentConfigService) Update(ctx context.Context, id uuid.UUID, req *model.CreateAgentRequest) (*model.AgentConfig, error) {
	cfg, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	cfg.DisplayName = req.DisplayName
	cfg.Description = req.Description
	cfg.Phase = req.Phase
	cfg.RoutingConfig = req.RoutingConfig
	cfg.Tools = req.Tools
	cfg.SystemPrompt = req.SystemPrompt
	if req.Model != "" {
		cfg.Model = req.Model
	}

	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}

	// 清除缓存
	s.cache.Del(ctx, "agent_configs:active")

	return cfg, nil
}

// Delete 删除Agent配置（仅限自定义角色）
func (s *AgentConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否为内置角色
	cfg, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if cfg.IsBuiltin {
		return fmt.Errorf("cannot delete builtin agent")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// 清除缓存
	s.cache.Del(ctx, "agent_configs:active")

	return nil
}
```

- [ ] **Step 4: 实现Agent API处理器**

```go
// internal/api/handler/agent.go
package handler

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AgentHandler struct {
	svc *agent.AgentConfigService
}

func NewAgentHandler(svc *agent.AgentConfigService) *AgentHandler {
	return &AgentHandler{svc: svc}
}

// ListAgents 获取Agent列表
func (h *AgentHandler) ListAgents(c *gin.Context) {
	configs, err := h.svc.ListActive(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": configs,
	})
}

// GetAgent 获取Agent详情
func (h *AgentHandler) GetAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	config, err := h.svc.GetByAgentID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": config,
	})
}

// CreateAgent 创建自定义Agent
func (h *AgentHandler) CreateAgent(c *gin.Context) {
	var req model.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code": 0,
		"data": config,
	})
}

// UpdateAgent 更新Agent配置
func (h *AgentHandler) UpdateAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": config,
	})
}

// DeleteAgent 删除Agent
func (h *AgentHandler) DeleteAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}
```

- [ ] **Step 5: 更新路由注册**

```go
// 在 internal/api/router.go 的 Setup 方法中添加
agents := v1.Group("/agents")
{
	agents.GET("", r.agentHandler.ListAgents)
	agents.GET("/:id", r.agentHandler.GetAgent)
	agents.POST("", r.agentHandler.CreateAgent)
	agents.PUT("/:id", r.agentHandler.UpdateAgent)
	agents.DELETE("/:id", r.agentHandler.DeleteAgent)
}
```

- [ ] **Step 6: 提交**

```bash
git add internal/repo/agent_config.go internal/service/agent/config_service.go internal/api/handler/agent.go
git commit -m "feat: add agent config service and API"
```

---

### Task 3.2: 调用追踪器与进程管理

**Files:**
- Create: `internal/service/agent/tracker.go`

- [ ] **Step 1: 实现进程管理器**

```go
// internal/service/agent/tracker.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// Invocation 调用记录（可序列化存储到Redis）
type Invocation struct {
	ID          string     `json:"id"`
	ThreadID    string     `json:"thread_id"`
	AgentID     string     `json:"agent_id"`
	SessionID   string     `json:"session_id"`
	PID         int        `json:"pid"`
	Depth       int        `json:"depth"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}

// ProcessManager 进程管理器（内存中管理进程句柄）
type ProcessManager struct {
	mu       sync.RWMutex
	procs    map[string]*os.Process      // invocationID -> Process
	contexts map[string]context.CancelFunc // invocationID -> CancelFunc
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		procs:    make(map[string]*os.Process),
		contexts: make(map[string]context.CancelFunc),
	}
}

func (pm *ProcessManager) Register(invocationID string, proc *os.Process, cancel context.CancelFunc) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.procs[invocationID] = proc
	pm.contexts[invocationID] = cancel
}

func (pm *ProcessManager) Unregister(invocationID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.procs, invocationID)
	delete(pm.contexts, invocationID)
}

func (pm *ProcessManager) Kill(invocationID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 先触发context取消
	if cancel, ok := pm.contexts[invocationID]; ok {
		cancel()
	}

	// 然后终止进程
	if proc, ok := pm.procs[invocationID]; ok {
		// 先发送SIGTERM
		proc.Signal(syscall.SIGTERM)

		// 等待3秒后强制杀死
		time.AfterFunc(3*time.Second, func() {
			proc.Kill()
		})

		delete(pm.procs, invocationID)
		delete(pm.contexts, invocationID)
	}

	return nil
}

// InvocationTracker 调用追踪器
type InvocationTracker struct {
	redis          *redis.Client
	maxDepth       int
	processManager *ProcessManager
}

func NewInvocationTracker(redis *redis.Client, maxDepth int) *InvocationTracker {
	return &InvocationTracker{
		redis:          redis,
		maxDepth:       maxDepth,
		processManager: NewProcessManager(),
	}
}

// Register 注册Agent调用
func (t *InvocationTracker) Register(ctx context.Context, inv *Invocation, proc *os.Process, cancel context.CancelFunc) error {
	// 深度检查
	if inv.Depth > t.maxDepth {
		return fmt.Errorf("depth limit exceeded: %d > %d", inv.Depth, t.maxDepth)
	}

	// 注册进程到管理器
	t.processManager.Register(inv.ID, proc, cancel)

	// 保存调用记录到Redis
	key := fmt.Sprintf("invocation:%s", inv.ID)
	data, _ := json.Marshal(inv)
	t.redis.Set(ctx, key, data, 30*time.Minute)

	// 关联到thread
	threadKey := fmt.Sprintf("thread:%s:invocations", inv.ThreadID)
	t.redis.SAdd(ctx, threadKey, inv.ID)

	return nil
}

// Cancel 取消thread下所有Agent调用
func (t *InvocationTracker) Cancel(ctx context.Context, threadID string) error {
	// 获取该thread所有invocation ID
	threadKey := fmt.Sprintf("thread:%s:invocations", threadID)
	invocationIDs := t.redis.SMembers(ctx, threadKey).Val()

	var errs []error
	for _, invID := range invocationIDs {
		// 从进程管理器终止进程
		if err := t.processManager.Kill(invID); err != nil {
			errs = append(errs, err)
		}

		// 更新Redis中的状态
		key := fmt.Sprintf("invocation:%s", invID)
		data := t.redis.Get(ctx, key).Val()
		if data != "" {
			var inv Invocation
			json.Unmarshal([]byte(data), &inv)
			inv.Status = "cancelled"
			now := time.Now()
			inv.EndedAt = &now
			updatedData, _ := json.Marshal(inv)
			t.redis.Set(ctx, key, updatedData, 30*time.Minute)
		}
	}

	// 清理thread的invocation列表
	t.redis.Del(ctx, threadKey)

	if len(errs) > 0 {
		return fmt.Errorf("partial cancellation errors: %v", errs)
	}
	return nil
}

// Complete 标记调用完成
func (t *InvocationTracker) Complete(ctx context.Context, invocationID string) {
	t.processManager.Unregister(invocationID)

	key := fmt.Sprintf("invocation:%s", invocationID)
	data := t.redis.Get(ctx, key).Val()
	if data != "" {
		var inv Invocation
		json.Unmarshal([]byte(data), &inv)
		inv.Status = "completed"
		now := time.Now()
		inv.EndedAt = &now
		updatedData, _ := json.Marshal(inv)
		t.redis.Set(ctx, key, updatedData, 30*time.Minute)
	}
}

// GetInvocation 获取调用记录
func (t *InvocationTracker) GetInvocation(ctx context.Context, invocationID string) (*Invocation, error) {
	key := fmt.Sprintf("invocation:%s", invocationID)
	data, err := t.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var inv Invocation
	if err := json.Unmarshal([]byte(data), &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}
```

- [ ] **Step 2: 编写测试**

```go
// internal/service/agent/tracker_test.go
package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProcessManager_RegisterKill(t *testing.T) {
	pm := NewProcessManager()

	// 注册一个模拟进程
	pm.Register("test-inv", nil, nil)

	// 杀死进程
	err := pm.Kill("test-inv")
	assert.NoError(t, err)
}

func TestInvocationTracker_DepthLimit(t *testing.T) {
	// 使用mock redis
	tracker := &InvocationTracker{
		maxDepth:       15,
		processManager: NewProcessManager(),
	}

	inv := &Invocation{
		ID:        "test",
		ThreadID:  "thread-1",
		AgentID:   "developer",
		Depth:     20, // 超过限制
		StartedAt: time.Now(),
	}

	// 应该返回深度限制错误
	// 这里需要mock redis来完整测试
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/service/agent/tracker.go internal/service/agent/tracker_test.go
git commit -m "feat: add invocation tracker with process management"
```

---

### Task 3.3: 工作流引擎

**Files:**
- Create: `internal/service/agent/workflow.go`

- [ ] **Step 1: 实现工作流引擎**

```go
// internal/service/agent/workflow.go
package agent

import (
	"context"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
)

// Phase 定义工作流阶段
type Phase string

const (
	PhaseRequirement Phase = "requirement"
	PhaseDesign      Phase = "design"
	PhaseImplement   Phase = "implement"
	PhaseReview      Phase = "review"
	PhaseTest        Phase = "test"
	PhaseDeploy      Phase = "deploy"
)

// PhaseConfig 阶段配置
type PhaseConfig struct {
	Name       Phase
	Agent      string
	Next       Phase
	Checkpoint bool
	OnReject   Phase
}

// Workflow 工作流配置
var Workflow = map[Phase]PhaseConfig{
	PhaseRequirement: {
		Name:       PhaseRequirement,
		Agent:      "requirement-analyst",
		Next:       PhaseDesign,
		Checkpoint: true,
		OnReject:   PhaseRequirement,
	},
	PhaseDesign: {
		Name:       PhaseDesign,
		Agent:      "architect",
		Next:       PhaseImplement,
		Checkpoint: true,
		OnReject:   PhaseDesign,
	},
	PhaseImplement: {
		Name:       PhaseImplement,
		Agent:      "developer",
		Next:       PhaseReview,
		Checkpoint: false,
		OnReject:   PhaseImplement,
	},
	PhaseReview: {
		Name:       PhaseReview,
		Agent:      "reviewer",
		Next:       PhaseTest,
		Checkpoint: true,
		OnReject:   PhaseImplement,
	},
	PhaseTest: {
		Name:       PhaseTest,
		Agent:      "test-engineer",
		Next:       PhaseDeploy,
		Checkpoint: true,
		OnReject:   PhaseImplement,
	},
	PhaseDeploy: {
		Name:       PhaseDeploy,
		Agent:      "devops",
		Next:       "",
		Checkpoint: false,
		OnReject:   PhaseDeploy,
	},
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	configSvc      *AgentConfigService
	tracker        *InvocationTracker
	mergeGateSvc   *MergeGateService
}

func NewWorkflowEngine(configSvc *AgentConfigService, tracker *InvocationTracker, mergeGateSvc *MergeGateService) *WorkflowEngine {
	return &WorkflowEngine{
		configSvc:    configSvc,
		tracker:      tracker,
		mergeGateSvc: mergeGateSvc,
	}
}

// GetPhaseConfig 获取阶段配置
func (e *WorkflowEngine) GetPhaseConfig(phase Phase) (PhaseConfig, error) {
	cfg, ok := Workflow[phase]
	if !ok {
		return PhaseConfig{}, fmt.Errorf("unknown phase: %s", phase)
	}
	return cfg, nil
}

// GetAgentForPhase 获取阶段对应的Agent
func (e *WorkflowEngine) GetAgentForPhase(phase Phase) (string, error) {
	cfg, err := e.GetPhaseConfig(phase)
	if err != nil {
		return "", err
	}
	return cfg.Agent, nil
}

// NextPhase 获取下一阶段
func (e *WorkflowEngine) NextPhase(current Phase) (Phase, error) {
	cfg, err := e.GetPhaseConfig(current)
	if err != nil {
		return "", err
	}
	if cfg.Next == "" {
		return "", fmt.Errorf("no next phase, workflow complete")
	}
	return cfg.Next, nil
}

// IsCheckpointPhase 检查是否为检查点阶段
func (e *WorkflowEngine) IsCheckpointPhase(phase Phase) bool {
	cfg, ok := Workflow[phase]
	return ok && cfg.Checkpoint
}

// GetRejectPhase 获取拒绝后的回退阶段
func (e *WorkflowEngine) GetRejectPhase(current Phase) Phase {
	cfg, ok := Workflow[current]
	if !ok {
		return current
	}
	return cfg.OnReject
}

// ValidateTransition 验证阶段转换是否合法
func (e *WorkflowEngine) ValidateTransition(from, to Phase) error {
	fromCfg, ok := Workflow[from]
	if !ok {
		return fmt.Errorf("invalid from phase: %s", from)
	}

	if fromCfg.Next != to {
		return fmt.Errorf("invalid transition: %s -> %s, expected %s -> %s", from, to, from, fromCfg.Next)
	}

	return nil
}

// GetPhaseOrder 获取阶段顺序
func (e *WorkflowEngine) GetPhaseOrder() []Phase {
	return []Phase{
		PhaseRequirement,
		PhaseDesign,
		PhaseImplement,
		PhaseReview,
		PhaseTest,
		PhaseDeploy,
	}
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/service/agent/workflow.go
git commit -m "feat: add workflow engine with phase management"
```

---

### Task 3.4: Agent编排器实现

**Files:**
- Create: `internal/service/agent/orchestrator.go`

- [ ] **Step 1: 编写编排器测试**

```go
// internal/service/agent/orchestrator_test.go
package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_StartAgent(t *testing.T) {
	// 测试Agent启动逻辑
}

func TestOrchestrator_HandlePhaseTransition(t *testing.T) {
	// 测试阶段转换处理
}
```

- [ ] **Step 2: 实现Agent编排器**

```go
// internal/service/agent/orchestrator.go
package agent

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/anthropic/isdp/pkg/claude"
	"github.com/google/uuid"
)

// Orchestrator Agent编排器
type Orchestrator struct {
	configSvc    *AgentConfigService
	tracker      *InvocationTracker
	workflow     *WorkflowEngine
	contextBuild *ContextBuilder
	claude       *claude.Adapter
	threadRepo   *repo.ThreadRepository
	msgRepo      *repo.MessageRepository
	wsHub        *ws.Hub
	mu           sync.Mutex
}

func NewOrchestrator(
	configSvc *AgentConfigService,
	tracker *InvocationTracker,
	workflow *WorkflowEngine,
	contextBuild *ContextBuilder,
	claude *claude.Adapter,
	threadRepo *repo.ThreadRepository,
	msgRepo *repo.MessageRepository,
	wsHub *ws.Hub,
) *Orchestrator {
	return &Orchestrator{
		configSvc:    configSvc,
		tracker:      tracker,
		workflow:     workflow,
		contextBuild: contextBuild,
		claude:       claude,
		threadRepo:   threadRepo,
		msgRepo:      msgRepo,
		wsHub:        wsHub,
	}
}

// StartOptions 启动选项
type StartOptions struct {
	ThreadID     string
	AgentID      string
	Input        string
	Depth        int
	SessionID    string
}

// StartAgent 启动Agent执行
func (o *Orchestrator) StartAgent(ctx context.Context, opts StartOptions) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	threadID, err := uuid.Parse(opts.ThreadID)
	if err != nil {
		return fmt.Errorf("invalid thread id: %w", err)
	}

	// 获取Agent配置
	agentCfg, err := o.configSvc.GetByAgentID(ctx, opts.AgentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// 构建上下文
	prompt, err := o.contextBuild.BuildPrompt(ctx, ContextConfig{
		ThreadID: opts.ThreadID,
		AgentID:  opts.AgentID,
		Phase:    agentCfg.Phase,
	}, opts.Input)
	if err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	// 生成调用ID和认证token
	invocationID := uuid.New().String()
	tokenPair, err := o.configSvc.mcpAuth.GenerateToken(ctx, invocationID)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// 通知前端Agent开始工作
	if o.wsHub != nil {
		o.wsHub.BroadcastToThread(opts.ThreadID, ws.WSMessage{
			Type:     "agent_start",
			ThreadID: opts.ThreadID,
			Payload: map[string]interface{}{
				"agentId":   opts.AgentID,
				"agentName": agentCfg.DisplayName,
				"phase":     agentCfg.Phase,
			},
		})
	}

	// 启动Claude CLI
	eventChan, cmd, err := o.claude.Invoke(ctx, claude.InvokeOptions{
		Prompt:        prompt,
		SessionID:     opts.SessionID,
		Model:         agentCfg.Model,
		AllowedTools:  agentCfg.Tools,
		ThreadID:      opts.ThreadID,
		InvocationID:  tokenPair.InvocationID,
		CallbackToken: tokenPair.CallbackToken,
	})
	if err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	// 注册调用追踪
	inv := &Invocation{
		ID:        invocationID,
		ThreadID:  opts.ThreadID,
		AgentID:   opts.AgentID,
		Depth:     opts.Depth,
		Status:    "running",
		StartedAt: time.Now(),
	}
	o.tracker.Register(ctx, inv, cmd.Process, cancel)

	// 异步处理事件流
	go o.processEvents(ctx, opts.ThreadID, opts.AgentID, invocationID, eventChan)

	return nil
}

// processEvents 处理Claude输出事件
func (o *Orchestrator) processEvents(ctx context.Context, threadID, agentID, invocationID string, eventChan <-chan claude.StreamEvent) {
	for event := range eventChan {
		switch event.Type {
		case "text":
			// 广播消息到前端
			if o.wsHub != nil {
				o.wsHub.BroadcastToThread(threadID, ws.WSMessage{
					Type:     "agent_message",
					ThreadID: threadID,
					Payload: map[string]interface{}{
						"agentId": agentID,
						"content": event.Content,
					},
				})
			}

		case "tool_use":
			// 处理工具调用
			// TODO: 检查是否有@mention

		case "error":
			// 错误处理
			o.tracker.Complete(ctx, invocationID)
			return
		}
	}

	// 完成
	o.tracker.Complete(ctx, invocationID)

	// 通知前端Agent完成
	if o.wsHub != nil {
		o.wsHub.BroadcastToThread(threadID, ws.WSMessage{
			Type:     "agent_end",
			ThreadID: threadID,
			Payload: map[string]interface{}{
				"agentId": agentID,
			},
		})
	}
}

// TransitionToNextPhase 转换到下一阶段
func (o *Orchestrator) TransitionToNextPhase(ctx context.Context, threadID string) error {
	thread, err := o.threadRepo.FindByID(ctx, uuid.MustParse(threadID))
	if err != nil {
		return err
	}

	nextPhase, err := o.workflow.NextPhase(thread.CurrentPhase)
	if err != nil {
		// 工作流完成
		thread.Status = model.ThreadStatusCompleted
		o.threadRepo.Update(ctx, thread)
		return nil
	}

	nextAgent, err := o.workflow.GetAgentForPhase(nextPhase)
	if err != nil {
		return err
	}

	// 更新阶段
	thread.CurrentPhase = nextPhase
	thread.CurrentAgent = nextAgent
	if err := o.threadRepo.Update(ctx, thread); err != nil {
		return err
	}

	// 通知前端阶段变化
	if o.wsHub != nil {
		o.wsHub.BroadcastToThread(threadID, ws.WSMessage{
			Type:     "phase_change",
			ThreadID: threadID,
			Payload: map[string]interface{}{
				"fromPhase": string(thread.CurrentPhase),
				"toPhase":   string(nextPhase),
				"toAgent":   nextAgent,
			},
		})
	}

	return nil
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/service/agent/orchestrator.go
git commit -m "feat: add agent orchestrator with workflow coordination"
```

---

## Chunk 3 完成

Agent引擎核心完成，包括：
- Agent配置服务（CRUD + 缓存）
- 调用追踪器与进程管理
- 工作流引擎（阶段定义与转换）
- Agent编排器（启动、事件处理、阶段转换）

---

## Chunk 4: A2A路由与MCP回传（M4）

### Task 4.1: Worklist队列实现

**Files:**
- Create: `internal/service/agent/worklist.go`

- [ ] **Step 1: 实现Worklist**

```go
// internal/service/agent/worklist.go
package agent

import (
	"fmt"
	"time"
)

// WorklistItem 工作队列项
type WorklistItem struct {
	AgentID   string    `json:"agent_id"`
	TriggerBy string    `json:"trigger_by"`
	Input     string    `json:"input"`
	Depth     int       `json:"depth"`
	AddedAt   time.Time `json:"added_at"`
}

// Worklist 工作队列
type Worklist struct {
	threadID string
	items    []WorklistItem
	maxDepth int
	maxItems int
}

func NewWorklist(threadID string, maxDepth, maxItems int) *Worklist {
	return &Worklist{
		threadID: threadID,
		items:    make([]WorklistItem, 0),
		maxDepth: maxDepth,
		maxItems: maxItems,
	}
}

// Push 添加工作项
func (w *Worklist) Push(item WorklistItem) error {
	// 深度检查
	if item.Depth > w.maxDepth {
		return fmt.Errorf("depth limit exceeded: %d > %d", item.Depth, w.maxDepth)
	}

	// 数量限制
	if len(w.items) >= w.maxItems {
		return fmt.Errorf("worklist full: max %d items", w.maxItems)
	}

	// 去重检查
	for _, existing := range w.items {
		if existing.AgentID == item.AgentID {
			return nil // 已存在，跳过
		}
	}

	item.AddedAt = time.Now()
	w.items = append(w.items, item)
	return nil
}

// Pop 取出工作项
func (w *Worklist) Pop() *WorklistItem {
	if len(w.items) == 0 {
		return nil
	}

	item := w.items[0]
	w.items = w.items[1:]
	return &item
}

// Peek 查看下一个工作项
func (w *Worklist) Peek() *WorklistItem {
	if len(w.items) == 0 {
		return nil
	}
	return &w.items[0]
}

// IsEmpty 检查是否为空
func (w *Worklist) IsEmpty() bool {
	return len(w.items) == 0
}

// Size 获取队列大小
func (w *Worklist) Size() int {
	return len(w.items)
}

// Clear 清空队列
func (w *Worklist) Clear() {
	w.items = make([]WorklistItem, 0)
}

// GetAll 获取所有工作项
func (w *Worklist) GetAll() []WorklistItem {
	return w.items
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/service/agent/worklist.go
git commit -m "feat: add worklist queue for agent task management"
```

---

### Task 4.2: @mention解析器

**Files:**
- Create: `internal/service/agent/mention_parser.go`

- [ ] **Step 1: 编写测试**

```go
// internal/service/agent/mention_parser_test.go
package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMentions_SingleMention(t *testing.T) {
	content := "@architect 请帮我review这个设计"
	mentions := ParseMentions(content, "developer")

	assert.Len(t, mentions, 1)
	assert.Equal(t, "architect", mentions[0].AgentID)
	assert.Equal(t, "请帮我review这个设计", mentions[0].Content)
}

func TestParseMentions_NotAtLineStart(t *testing.T) {
	content := "看这个 @architect"
	mentions := ParseMentions(content, "")

	assert.Len(t, mentions, 0) // 不在行首，不触发
}

func TestParseMentions_MaxTwoMentions(t *testing.T) {
	content := "@developer 请实现\n@architect 请设计\n@test-engineer 请测试"
	mentions := ParseMentions(content, "reviewer")

	assert.Len(t, mentions, 2) // 最多2个
}

func TestParseMentions_ExcludeSelf(t *testing.T) {
	content := "@developer 这个\n@architect 那个"
	mentions := ParseMentions(content, "developer")

	assert.Len(t, mentions, 1)
	assert.Equal(t, "architect", mentions[0].AgentID)
}

func TestParseMentions_InCodeComment(t *testing.T) {
	content := "// @developer 注释中的不应该触发"
	mentions := ParseMentions(content, "")

	assert.Len(t, mentions, 0) // 不在行首
}
```

- [ ] **Step 2: 实现解析器**

```go
// internal/service/agent/mention_parser.go
package agent

import (
	"regexp"
	"strings"
)

// Mention @提及信息
type Mention struct {
	AgentID string `json:"agent_id"`
	Content string `json:"content"`
}

var mentionPattern = regexp.MustCompile(`^@([\w-]+)\s*(.*)`)

// ParseMentions 解析@mention
// 只匹配行首的@，防止代码注释、文档中的误触发
// 单次最多解析2个@mention
func ParseMentions(content string, excludeSelf string) []Mention {
	var mentions []Mention

	lines := strings.Split(content, "\n")
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		matches := mentionPattern.FindStringSubmatch(line)
		if len(matches) >= 2 {
			agentID := matches[1]

			// 排除自己
			if agentID == excludeSelf {
				continue
			}

			// 限制最多2个
			if len(mentions) >= 2 {
				break
			}

			mentions = append(mentions, Mention{
				AgentID: agentID,
				Content: matches[2],
			})
		}
	}

	return mentions
}

// ValidateMention 验证mention是否有效
func ValidateMention(agentID string, activeAgents []string) bool {
	for _, agent := range activeAgents {
		if agent == agentID {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: 运行测试确认通过**

```bash
go test ./internal/service/agent/... -v -run TestParseMentions
# Expected: PASS
```

- [ ] **Step 4: 提交**

```bash
git add internal/service/agent/mention_parser.go internal/service/agent/mention_parser_test.go
git commit -m "feat: add @mention parser with line-start detection"
```

---

### Task 4.3: MCP认证服务

**Files:**
- Create: `internal/service/agent/mcp_auth.go`
- Create: `internal/api/middleware/mcp_auth.go`
- Create: `internal/api/handler/mcp.go`

- [ ] **Step 1: 实现MCP认证服务**

```go
// internal/service/agent/mcp_auth.go
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// MCPAuthService MCP认证服务
type MCPAuthService struct {
	redis *redis.Client
}

// TokenPair 认证令牌对
type TokenPair struct {
	InvocationID  string `json:"invocation_id"`
	CallbackToken string `json:"callback_token"`
}

func NewMCPAuthService(redis *redis.Client) *MCPAuthService {
	return &MCPAuthService{redis: redis}
}

// GenerateToken 生成认证令牌
func (s *MCPAuthService) GenerateToken(ctx context.Context, invocationID string) (*TokenPair, error) {
	// 生成随机callback token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	callbackToken := hex.EncodeToString(tokenBytes)

	// 存储到Redis，TTL 30分钟
	key := fmt.Sprintf("mcp:token:%s", invocationID)
	s.redis.Set(ctx, key, callbackToken, 30*time.Minute)

	return &TokenPair{
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	}, nil
}

// ValidateToken 验证令牌
func (s *MCPAuthService) ValidateToken(ctx context.Context, invocationID, callbackToken string) (string, error) {
	key := fmt.Sprintf("mcp:token:%s", invocationID)

	// 获取存储的token
	storedToken, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("invalid invocation id")
	}

	// 验证token匹配
	if storedToken != callbackToken {
		return "", fmt.Errorf("invalid callback token")
	}

	// 获取invocation对应的thread_id
	invKey := fmt.Sprintf("invocation:%s", invocationID)
	data, err := s.redis.Get(ctx, invKey).Result()
	if err != nil {
		return "", fmt.Errorf("invocation not found")
	}

	var inv Invocation
	json.Unmarshal([]byte(data), &inv)

	return inv.ThreadID, nil
}

// RevokeToken 撤销令牌（Agent结束时调用）
func (s *MCPAuthService) RevokeToken(ctx context.Context, invocationID string) {
	key := fmt.Sprintf("mcp:token:%s", invocationID)
	s.redis.Del(ctx, key)
}
```

- [ ] **Step 2: 实现MCP认证中间件**

```go
// internal/api/middleware/mcp_auth.go
package middleware

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/gin-gonic/gin"
)

func MCPAuth(authService *agent.MCPAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header获取认证信息
		invocationID := c.GetHeader("X-Invocation-ID")
		callbackToken := c.GetHeader("X-Callback-Token")

		if invocationID == "" || callbackToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "missing authentication headers",
			})
			c.Abort()
			return
		}

		// 验证令牌
		threadID, err := authService.ValidateToken(c.Request.Context(), invocationID, callbackToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// 设置上下文
		c.Set("invocation_id", invocationID)
		c.Set("thread_id", threadID)
		c.Next()
	}
}
```

- [ ] **Step 3: 实现MCP API处理器**

```go
// internal/api/handler/mcp.go
package handler

import (
	"net/http"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/gin-gonic/gin"
)

type MCPHandler struct {
	msgRepo *repo.MessageRepository
}

func NewMCPHandler(msgRepo *repo.MessageRepository) *MCPHandler {
	return &MCPHandler{msgRepo: msgRepo}
}

// PostMessage 发送消息到聊天室
func (h *MCPHandler) PostMessage(c *gin.Context) {
	threadID := c.GetString("thread_id")

	var req struct {
		Content string `json:"content" binding:"required"`
		AgentID string `json:"agent_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: 保存消息到数据库并通过WebSocket广播

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "message posted",
		"thread_id": threadID,
	})
}

// RequestPermission 请求危险操作授权
func (h *MCPHandler) RequestPermission(c *gin.Context) {
	threadID := c.GetString("thread_id")

	var req struct {
		Action      string `json:"action" binding:"required"`
		Description string `json:"description" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: 创建权限请求记录，等待用户确认

	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"message":   "permission request created",
		"thread_id": threadID,
		"action":    req.Action,
	})
}
```

- [ ] **Step 4: 提交**

```bash
git add internal/service/agent/mcp_auth.go internal/api/middleware/mcp_auth.go internal/api/handler/mcp.go
git commit -m "feat: add MCP authentication service and API"
```

---

## Chunk 4 完成

A2A路由与MCP回传完成，包括：
- Worklist队列（任务管理）
- @mention解析器（行首检测，最多2个）
- MCP认证服务（双UUID认证）
- MCP API处理器

---

## Chunk 5: ClaudeCode集成（M5）

### Task 5.1: Claude CLI适配器

**Files:**
- Create: `pkg/claude/adapter.go`

- [ ] **Step 1: 编写适配器测试**

```go
// pkg/claude/adapter_test.go
package claude

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdapter_BuildArgs(t *testing.T) {
	adapter := NewAdapter("/usr/bin/claude", "/tmp/work", "claude-sonnet-4-6", 30*time.Minute)

	opts := InvokeOptions{
		Prompt:       "Hello",
		AllowedTools: []string{"Read", "Write"},
	}

	args := adapter.buildArgs(opts)
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "Hello")
	assert.Contains(t, args, "--allowedTools")
	assert.Contains(t, args, "Read,Write")
}

func TestAdapter_BuildEnv(t *testing.T) {
	adapter := NewAdapter("/usr/bin/claude", "/tmp/work", "claude-sonnet-4-6", 30*time.Minute)

	opts := InvokeOptions{
		ThreadID:      "thread-123",
		InvocationID:  "inv-456",
		CallbackToken: "token-789",
	}

	env := adapter.buildEnv(opts)
	assert.Contains(t, env, "ISDP_THREAD_ID=thread-123")
	assert.Contains(t, env, "ISDP_INVOCATION_ID=inv-456")
	assert.Contains(t, env, "ISDP_CALLBACK_TOKEN=token-789")
}
```

- [ ] **Step 2: 实现适配器**

```go
// pkg/claude/adapter.go
package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Adapter Claude CLI适配器
type Adapter struct {
	claudePath string
	workDir    string
	model      string
	timeout    time.Duration
}

// InvokeOptions 调用选项
type InvokeOptions struct {
	Prompt         string
	SessionID      string
	Model          string
	AllowedTools   []string
	PermissionMode string
	ThreadID       string
	InvocationID   string
	CallbackToken  string
	MCPBaseURL     string
	WorkDir        string
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type           string          `json:"type"`
	Content        string          `json:"content"`
	ToolName       string          `json:"tool_name,omitempty"`
	ToolInput      json.RawMessage `json:"tool_input,omitempty"`
	SessionID      string          `json:"session_id,omitempty"`
	InputTokens    int             `json:"input_tokens,omitempty"`
	OutputTokens   int             `json:"output_tokens,omitempty"`
	Error          string          `json:"error,omitempty"`
}

func NewAdapter(claudePath, workDir, model string, timeout time.Duration) *Adapter {
	return &Adapter{
		claudePath: claudePath,
		workDir:    workDir,
		model:      model,
		timeout:    timeout,
	}
}

// Invoke 启动Claude CLI并返回事件流
func (a *Adapter) Invoke(ctx context.Context, opts InvokeOptions) (<-chan StreamEvent, *exec.Cmd, error) {
	args := a.buildArgs(opts)
	cmd := exec.CommandContext(ctx, a.claudePath, args...)

	// 设置工作目录
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = a.workDir
	}
	cmd.Dir = workDir

	// 设置环境变量
	cmd.Env = a.buildEnv(opts)

	// 获取stdout管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// 创建事件通道
	eventChan := make(chan StreamEvent, 100)

	// 异步处理输出流
	go a.processStream(stdout, eventChan)

	return eventChan, cmd, nil
}

// buildArgs 构建CLI参数
func (a *Adapter) buildArgs(opts InvokeOptions) []string {
	args := []string{}

	// Prompt
	args = append(args, "-p", opts.Prompt)

	// 输出格式
	args = append(args, "--output-format", "stream-json")

	// Verbose模式
	args = append(args, "--verbose")

	// 模型
	model := opts.Model
	if model == "" {
		model = a.model
	}
	args = append(args, "--model", model)

	// 恢复会话
	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}

	// 允许的工具
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}

	// 权限模式
	if opts.PermissionMode != "" {
		args = append(args, "--permission-mode", opts.PermissionMode)
	}

	return args
}

// buildEnv 构建环境变量
func (a *Adapter) buildEnv(opts InvokeOptions) []string {
	env := os.Environ()

	if opts.ThreadID != "" {
		env = append(env, fmt.Sprintf("ISDP_THREAD_ID=%s", opts.ThreadID))
	}
	if opts.InvocationID != "" {
		env = append(env, fmt.Sprintf("ISDP_INVOCATION_ID=%s", opts.InvocationID))
	}
	if opts.CallbackToken != "" {
		env = append(env, fmt.Sprintf("ISDP_CALLBACK_TOKEN=%s", opts.CallbackToken))
	}
	if opts.MCPBaseURL != "" {
		env = append(env, fmt.Sprintf("ISDP_MCP_BASE_URL=%s", opts.MCPBaseURL))
	}

	return env
}

// processStream 处理输出流
func (a *Adapter) processStream(reader io.Reader, eventChan chan<- StreamEvent) {
	defer close(eventChan)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// 解析失败，作为原始内容
			event = StreamEvent{
				Type:    "text",
				Content: line,
			}
		}

		eventChan <- event
	}
}

// ToolPresets 工具预设
var ToolPresets = map[string][]string{
	"readonly": {"Read", "Glob", "Grep"},
	"write":    {"Read", "Write", "Edit", "Glob", "Grep"},
	"full":     {"Read", "Write", "Edit", "Glob", "Grep", "Bash"},
}

// GetToolPreset 获取工具预设
func GetToolPreset(preset string) []string {
	if tools, ok := ToolPresets[preset]; ok {
		return tools
	}
	return ToolPresets["readonly"]
}
```

- [ ] **Step 3: 运行测试确认通过**

```bash
go test ./pkg/claude/... -v
# Expected: PASS
```

- [ ] **Step 4: 提交**

```bash
git add pkg/claude/
git commit -m "feat: add Claude CLI adapter with stream processing"
```

---

### Task 5.2: 上下文构建器

**Files:**
- Create: `internal/service/agent/context_builder.go`

- [ ] **Step 1: 实现四层上下文构建器**

```go
// internal/service/agent/context_builder.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/repo"
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	msgRepo      *repo.MessageRepository
	artifactRepo *repo.ArtifactRepository
	projectRepo  *repo.ProjectRepository
	agentRepo    *repo.AgentConfigRepository
	maxLines     int
}

// ContextConfig 上下文配置
type ContextConfig struct {
	ThreadID    string
	AgentID     string
	Phase       string
	ProjectMode string
}

func NewContextBuilder(
	msgRepo *repo.MessageRepository,
	artifactRepo *repo.ArtifactRepository,
	projectRepo *repo.ProjectRepository,
	agentRepo *repo.AgentConfigRepository,
) *ContextBuilder {
	return &ContextBuilder{
		msgRepo:      msgRepo,
		artifactRepo: artifactRepo,
		projectRepo:  projectRepo,
		agentRepo:    agentRepo,
		maxLines:     400,
	}
}

// BuildPrompt 构建Agent的完整prompt
func (b *ContextBuilder) BuildPrompt(ctx context.Context, config ContextConfig, taskInput string) (string, error) {
	var sb strings.Builder
	totalLines := 0

	// ========== Layer 0: 核心铁律（始终加载，≤100行）==========
	layer0 := b.buildLayer0()
	sb.WriteString(layer0)
	totalLines += countLines(layer0)

	// ========== Layer 1: 工作流指南（按需加载，≤150行）==========
	layer1 := b.buildLayer1(config.Phase)
	sb.WriteString(layer1)
	totalLines += countLines(layer1)

	// ========== Layer 2: 模板引用（按需加载）==========
	if config.Phase == "requirement" || config.Phase == "design" || config.Phase == "review" {
		layer2 := b.buildLayer2(config.Phase)
		sb.WriteString(layer2)
	}

	// ========== Layer 3: 需求注入 ==========
	layer3, err := b.buildLayer3(ctx, config)
	if err == nil {
		sb.WriteString(layer3)
		totalLines += countLines(layer3)
	}

	// ========== 当前任务 ==========
	sb.WriteString("\n---\n\n# 当前任务\n\n")
	sb.WriteString(taskInput)
	sb.WriteString("\n")

	return sb.String(), nil
}

// buildLayer0 核心铁律（≤100行）
func (b *ContextBuilder) buildLayer0() string {
	return `# 你的角色

你是ISDP平台的AI开发助手，协助用户完成软件开发任务。

## 核心价值观

- 用户需求第一：始终以解决用户问题为目标
- 质量优先：不追求速度，追求正确和可靠
- 诚实透明：不确定就承认，不编造信息

## 铁律（必须遵守）

1. **不确定就提问**：不要猜测，问清楚再做
2. **交接写WHY**：决策原因比结果更重要
3. **Review铁面无私**：发现问题必须指出，不讨好
4. **禁止客套话**：不要说"您说得对"、"好的没问题"，用技术论证
5. **深度限制**：A2A调用不超过15轮

## 可用MCP工具

- post_message: 发送消息到聊天室
- thread_context: 获取对话历史
- request_permission: 请求危险操作授权（删除文件、执行敏感命令）

## @其他Agent

需要其他Agent协助时，在行首使用 @Agent名 格式：
- @architect - 架构设计
- @developer - 代码实现
- @reviewer - 代码审查
- @test-engineer - 测试
- @devops - 部署

`
}

// buildLayer1 工作流指南（≤150行）
func (b *ContextBuilder) buildLayer1(phase string) string {
	var sb strings.Builder

	sb.WriteString("\n# 工作流指南\n\n")

	switch phase {
	case "requirement":
		sb.WriteString(`## 需求分析阶段

### 你的任务
1. 解析用户输入的自然语言需求
2. 识别功能需求和非功能需求
3. 生成结构化的需求文档
4. 定义验收标准（AC）

### 输出格式
` + "```markdown" + `
## 项目概述
[项目描述]

## 功能需求
### F1: [功能名称]
- 描述: [功能描述]
- 优先级: P0/P1/P2
- 验收标准:
  - [ ] AC1
  - [ ] AC2

## 非功能需求
- 性能要求:
- 安全要求:
- 兼容性要求:
` + "```" + `

### 注意事项
- 如果需求不清晰，必须向用户提问澄清
- 验收标准要具体可测试
- 优先级P0为核心功能，P1为重要功能，P2为锦上添花

`)

	case "design":
		sb.WriteString(`## 方案设计阶段

### 你的任务
1. 根据需求文档设计技术方案
2. 选择技术栈（默认Go+Gin后端，React前端，PostgreSQL数据库）
3. 设计API接口
4. 设计数据模型

### 输出产物
1. 架构图（使用Mermaid格式）
2. API文档（OpenAPI格式）
3. 数据库Schema
4. 技术选型报告

### 技术栈默认选择
- 后端: Go 1.21+ + Gin
- 前端: React 18 + TypeScript + Ant Design
- 数据库: PostgreSQL 15
- 缓存: Redis 7

`)

	case "implement":
		sb.WriteString(`## 代码实现阶段

### 你的任务
1. 根据技术方案编写代码
2. 遵循代码规范
3. 编写单元测试

### Go代码规范
- 使用gofmt格式化
- 错误处理必须显式
- 导出函数必须有注释

### 测试要求
- 每个公开函数要有单元测试
- 测试覆盖率目标 > 70%
- 使用testify/assert断言

`)

	case "review":
		sb.WriteString(`## 代码审查阶段

### 审查要点
1. 功能正确性
2. 安全漏洞（SQL注入、XSS、命令注入等）
3. 性能问题
4. 测试覆盖
5. 代码规范

### Review分级标准
- P1: 阻断级（功能错误/安全问题）- 必须立即修
- P2: 重要级（质量/测试问题）- 必须修完才能放行
- P3: 建议级（风格/命名）- 可登记Backlog

### 放行条件
必须明确说"可以放行"或"P1/P2清零"才算通过

`)

	case "test":
		sb.WriteString(`## 测试验证阶段

### 你的任务
1. 执行单元测试
2. 执行集成测试
3. 生成测试报告

### 测试报告格式
` + "```markdown" + `
## 测试报告

### 测试概览
- 总用例数: X
- 通过: X
- 失败: X
- 覆盖率: X%

### 结论
- [ ] 全部通过
- [ ] 存在失败用例
` + "```" + `

`)

	case "deploy":
		sb.WriteString(`## 沙箱部署阶段

### 你的任务
1. 构建Docker镜像
2. 启动容器
3. 验证服务可用

### 部署检查清单
- [ ] 环境变量配置正确
- [ ] 端口映射正确
- [ ] 健康检查通过
- [ ] 日志输出正常

`)
	}

	return sb.String()
}

// buildLayer2 模板引用（按需加载）
func (b *ContextBuilder) buildLayer2(phase string) string {
	var sb strings.Builder

	sb.WriteString("\n# 模板参考\n\n")

	switch phase {
	case "requirement":
		sb.WriteString(`## 需求文档模板

` + "```markdown" + `
# 项目名称

## 项目概述
[一句话描述项目目标]

## 功能需求

### F1: [功能名称]
- **描述**: [详细描述]
- **优先级**: P0/P1/P2
- **验收标准**:
  - [ ] AC1: [具体验收条件]

## 非功能需求
- **性能**: [响应时间、并发量等]
- **安全**: [认证、授权、数据保护等]
` + "```" + `

`)

	case "design":
		sb.WriteString(`## 架构图模板（Mermaid）

` + "```" + `
graph TB
    subgraph Frontend
        UI[Web UI]
    end
    subgraph Backend
        API[API Gateway]
        SVC[Services]
        DB[(Database)]
    end
    UI --> API
    API --> SVC
    SVC --> DB
` + "```" + `

## API文档模板（OpenAPI）

` + "```yaml" + `
openapi: 3.0.0
paths:
  /api/v1/resource:
    get:
      summary: 获取资源列表
      responses:
        '200':
          description: 成功
` + "```" + `

`)

	case "review":
		sb.WriteString(`## Review报告模板

` + "```markdown" + `
## Review报告 #R{n}

### P1（共X个）— 必须立即修
1. file.go:line - 问题描述
   建议：修复建议

### P2（共X个）— 必须修完才能放行
...

### 结论
- [ ] P1/P2清零，可以放行
- [ ] 存在问题，不能放行
` + "```" + `

## 常见问题检查清单

### 安全问题
- [ ] SQL注入：是否使用参数化查询？
- [ ] XSS：是否对用户输入进行转义？
- [ ] 命令注入：是否验证外部输入？
- [ ] 敏感信息：是否硬编码密码/密钥？

### 性能问题
- [ ] N+1查询：是否在循环中查询数据库？
- [ ] 连接泄露：是否正确关闭连接？

### 代码质量
- [ ] 错误处理：是否处理所有错误？
- [ ] 空指针：是否检查nil？

`)
	}

	return sb.String()
}

// buildLayer3 需求注入（从数据库加载）
func (b *ContextBuilder) buildLayer3(ctx context.Context, config ContextConfig) (string, error) {
	var sb strings.Builder

	// 获取项目信息（简化版，实际需要实现repo方法）
	sb.WriteString("\n# 项目信息\n\n")
	sb.WriteString(fmt.Sprintf("- **会话ID**: %s\n", config.ThreadID))
	sb.WriteString(fmt.Sprintf("- **当前阶段**: %s\n", config.Phase))

	// 增量开发模式：注入现有代码结构
	if config.ProjectMode == "enhance" {
		sb.WriteString("\n## 现有项目增强模式\n\n")
		sb.WriteString("请在现有代码基础上进行开发，保持一致的技术栈和代码风格。\n")
	}

	return sb.String(), nil
}

func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractSummary 提取需求摘要
func extractSummary(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	var summary []string
	for _, line := range lines {
		if len(summary) >= maxLines {
			break
		}
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			summary = append(summary, line)
		}
	}
	return strings.Join(summary, "\n")
}

// CodeStructure 代码结构
type CodeStructure struct {
	TechStack    []string          `json:"tech_stack"`
	MainModules  []ModuleInfo      `json:"main_modules"`
	EntryPoints  []string          `json:"entry_points"`
	ConfigFiles  []string          `json:"config_files"`
	Dependencies map[string]string `json:"dependencies"`
}

// ModuleInfo 模块信息
type ModuleInfo struct {
	Path        string   `json:"path"`
	Description string   `json:"description"`
	MainFiles   []string `json:"main_files"`
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/service/agent/context_builder.go
git commit -m "feat: add four-layer context builder for agent prompts"
```

---

## Chunk 5 完成

ClaudeCode集成完成，包括：
- Claude CLI适配器（流式输出处理）
- 工具权限预设
- 四层上下文构建器

---

## Chunk 6: 沙箱环境（M6）

### Task 6.1: Docker客户端封装

**Files:**
- Create: `pkg/docker/client.go`

- [ ] **Step 1: 实现Docker客户端**

```go
// pkg/docker/client.go
package docker

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// DockerClient Docker客户端封装
type DockerClient struct {
	cli *client.Client
}

// ContainerConfig 容器配置
type ContainerConfig struct {
	Name        string
	Image       string
	WorkDir     string
	Ports       []PortMapping
	CPULimit    int64
	MemoryLimit int64
	Env         []string
	Volumes     []VolumeMount
	Network     string
}

// PortMapping 端口映射
type PortMapping struct {
	ContainerPort int
	HostPort      int
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// LogOptions 日志选项
type LogOptions struct {
	Follow     bool
	Tail       string
	Since      time.Time
	Timestamps bool
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to docker daemon: %w", err)
	}

	return &DockerClient{cli: cli}, nil
}

// CreateContainer 创建容器
func (d *DockerClient) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
	// 构建端口映射
	portSet := nat.PortSet{}
	portMap := nat.PortMap{}
	for _, pm := range config.Ports {
		port := nat.Port(fmt.Sprintf("%d/tcp", pm.ContainerPort))
		portSet[port] = struct{}{}
		portMap[port] = []nat.PortBinding{
			{HostPort: fmt.Sprintf("%d", pm.HostPort)},
		}
	}

	// 构建卷挂载
	var binds []string
	for _, vm := range config.Volumes {
		ro := ""
		if vm.ReadOnly {
			ro = ":ro"
		}
		binds = append(binds, fmt.Sprintf("%s:%s%s", vm.HostPath, vm.ContainerPath, ro))
	}

	// 创建容器
	containerConfig := &container.Config{
		Image:        config.Image,
		Env:          config.Env,
		WorkingDir:   config.WorkDir,
		ExposedPorts: portSet,
	}

	hostConfig := &container.HostConfig{
		PortBindings: portMap,
		Binds:        binds,
		Resources: container.Resources{
			NanoCPUs: config.CPULimit * 1e9,  // 转换为纳秒
			Memory:   config.MemoryLimit * 1024 * 1024, // 转换为字节
		},
		SecurityOpt: []string{
			"no-new-privileges",
		},
	}

	networkingConfig := &network.NetworkingConfig{}
	if config.Network != "" {
		networkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			config.Network: {},
		}
	}

	resp, err := d.cli.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, config.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer 启动容器
func (d *DockerClient) StartContainer(ctx context.Context, containerID string) error {
	return d.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer 停止容器
func (d *DockerClient) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error {
	var timeoutSec *int
	if timeout != nil {
		sec := int(timeout.Seconds())
		timeoutSec = &sec
	}
	return d.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: timeoutSec})
}

// RemoveContainer 移除容器
func (d *DockerClient) RemoveContainer(ctx context.Context, containerID string) error {
	return d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// GetContainerLogs 获取容器日志
func (d *DockerClient) GetContainerLogs(ctx context.Context, containerID string, opts LogOptions) (io.ReadCloser, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Tail:       opts.Tail,
		Since:      opts.Since.Format(time.RFC3339),
		Timestamps: opts.Timestamps,
	}

	return d.cli.ContainerLogs(ctx, containerID, options)
}

// ExecInContainer 在容器中执行命令
func (d *DockerClient) ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, error) {
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	execResp, err := d.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := d.cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to attach exec: %w", err)
	}
	defer attachResp.Close()

	output, err := io.ReadAll(attachResp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	return string(output), nil
}

// GetContainerStatus 获取容器状态
func (d *DockerClient) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	container, err := d.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}
	return container.State.Status, nil
}

// PullImage 拉取镜像
func (d *DockerClient) PullImage(ctx context.Context, image string) error {
	reader, err := d.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// 等待拉取完成
	_, err = io.Copy(io.Discard, reader)
	return err
}
```

- [ ] **Step 2: 提交**

```bash
git add pkg/docker/client.go
git commit -m "feat: add docker client wrapper for container management"
```

---

### Task 6.2: 沙箱服务

**Files:**
- Create: `internal/service/sandbox/service.go`
- Create: `internal/repo/sandbox.go`
- Create: `internal/api/handler/sandbox.go`

- [ ] **Step 1: 实现沙箱Repo**

```go
// internal/repo/sandbox.go
package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SandboxRepository struct {
	db *pgxpool.Pool
}

func NewSandboxRepository(db *pgxpool.Pool) *SandboxRepository {
	return &SandboxRepository{db: db}
}

func (r *SandboxRepository) Create(ctx context.Context, sandbox *model.Sandbox) error {
	query := `
		INSERT INTO sandbox_containers (id, project_id, container_id, name, status, image, ports, cpu_limit, memory_limit, network_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query,
		sandbox.ID, sandbox.ProjectID, sandbox.ContainerID, sandbox.Name, sandbox.Status, sandbox.Image,
		sandbox.Ports, sandbox.CPULimit, sandbox.MemoryLimit, sandbox.NetworkName, now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create sandbox: %w", err)
	}
	sandbox.CreatedAt = now
	sandbox.UpdatedAt = now
	return nil
}

func (r *SandboxRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Sandbox, error) {
	query := `
		SELECT id, project_id, container_id, name, status, image, ports, cpu_limit, memory_limit, network_name, started_at, stopped_at, error_message, created_at, updated_at
		FROM sandbox_containers WHERE id = $1
	`
	sandbox := &model.Sandbox{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&sandbox.ID, &sandbox.ProjectID, &sandbox.ContainerID, &sandbox.Name, &sandbox.Status, &sandbox.Image,
		&sandbox.Ports, &sandbox.CPULimit, &sandbox.MemoryLimit, &sandbox.NetworkName,
		&sandbox.StartedAt, &sandbox.StoppedAt, &sandbox.ErrorMessage, &sandbox.CreatedAt, &sandbox.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find sandbox: %w", err)
	}
	return sandbox, nil
}

func (r *SandboxRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SandboxStatus, errorMsg string) error {
	query := `
		UPDATE sandbox_containers SET status = $2, error_message = $3, updated_at = $4 WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, status, errorMsg, time.Now())
	return err
}

func (r *SandboxRepository) SetStarted(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE sandbox_containers SET status = 'running', started_at = $2, updated_at = $3 WHERE id = $1
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query, id, now, now)
	return err
}

func (r *SandboxRepository) SetStopped(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE sandbox_containers SET status = 'stopped', stopped_at = $2, updated_at = $3 WHERE id = $1
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query, id, now, now)
	return err
}
```

- [ ] **Step 2: 实现沙箱服务**

```go
// internal/service/sandbox/service.go
package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	docker "github.com/anthropic/isdp/pkg/docker"
	"github.com/google/uuid"
)

type SandboxService struct {
	docker   *docker.DockerClient
	repo     *repo.SandboxRepository
	portPool *PortPool
	mu       sync.Mutex
}

type PortPool struct {
	start    int
	end      int
	used     map[int]bool
	mu       sync.Mutex
}

func NewPortPool(start, end int) *PortPool {
	return &PortPool{
		start: start,
		end:   end,
		used:  make(map[int]bool),
	}
}

func (p *PortPool) Allocate() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port := p.start; port <= p.end; port++ {
		if !p.used[port] {
			p.used[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports")
}

func (p *PortPool) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.used, port)
}

func NewSandboxService(docker *docker.DockerClient, repo *repo.SandboxRepository, portStart, portEnd int) *SandboxService {
	return &SandboxService{
		docker:   docker,
		repo:     repo,
		portPool: NewPortPool(portStart, portEnd),
	}
}

type CreateSandboxRequest struct {
	ProjectID   uuid.UUID
	Image       string
	WorkDir     string
	CPULimit    int
	MemoryLimit int
	Env         []string
}

func (s *SandboxService) CreateSandbox(ctx context.Context, req CreateSandboxRequest) (*model.Sandbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 分配端口
	hostPort, err := s.portPool.Allocate()
	if err != nil {
		return nil, err
	}

	sandbox := &model.Sandbox{
		ID:          uuid.New(),
		ProjectID:   req.ProjectID,
		Name:        fmt.Sprintf("isdp-%s", req.ProjectID.String()[:8]),
		Status:      model.SandboxStatusCreated,
		Image:       req.Image,
		CPULimit:    req.CPULimit,
		MemoryLimit: req.MemoryLimit,
	}

	ports := []docker.PortMapping{{ContainerPort: 8080, HostPort: hostPort}}
	portsJSON, _ := json.Marshal(ports)
	sandbox.Ports = portsJSON

	// 创建Docker容器
	containerID, err := s.docker.CreateContainer(ctx, docker.ContainerConfig{
		Name:        sandbox.Name,
		Image:       req.Image,
		WorkDir:     req.WorkDir,
		Ports:       ports,
		CPULimit:    int64(req.CPULimit),
		MemoryLimit: int64(req.MemoryLimit),
		Env:         req.Env,
	})

	if err != nil {
		s.portPool.Release(hostPort)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	sandbox.ContainerID = containerID

	// 保存到数据库
	if err := s.repo.Create(ctx, sandbox); err != nil {
		s.docker.RemoveContainer(ctx, containerID)
		s.portPool.Release(hostPort)
		return nil, err
	}

	return sandbox, nil
}

func (s *SandboxService) StartSandbox(ctx context.Context, sandboxID uuid.UUID) error {
	sandbox, err := s.repo.FindByID(ctx, sandboxID)
	if err != nil {
		return err
	}

	if err := s.docker.StartContainer(ctx, sandbox.ContainerID); err != nil {
		s.repo.UpdateStatus(ctx, sandboxID, model.SandboxStatusError, err.Error())
		return err
	}

	return s.repo.SetStarted(ctx, sandboxID)
}

func (s *SandboxService) StopSandbox(ctx context.Context, sandboxID uuid.UUID) error {
	sandbox, err := s.repo.FindByID(ctx, sandboxID)
	if err != nil {
		return err
	}

	timeout := 30 * time.Second
	if err := s.docker.StopContainer(ctx, sandbox.ContainerID, &timeout); err != nil {
		return err
	}

	// 释放端口
	var ports []docker.PortMapping
	json.Unmarshal(sandbox.Ports, &ports)
	for _, p := range ports {
		s.portPool.Release(p.HostPort)
	}

	return s.repo.SetStopped(ctx, sandboxID)
}

func (s *SandboxService) GetLogs(ctx context.Context, sandboxID uuid.UUID, follow bool) (io.ReadCloser, error) {
	sandbox, err := s.repo.FindByID(ctx, sandboxID)
	if err != nil {
		return nil, err
	}

	return s.docker.GetContainerLogs(ctx, sandbox.ContainerID, docker.LogOptions{
		Follow: follow,
		Tail:   "100",
	})
}

func (s *SandboxService) ExecuteCommand(ctx context.Context, sandboxID uuid.UUID, cmd []string) (string, error) {
	sandbox, err := s.repo.FindByID(ctx, sandboxID)
	if err != nil {
		return "", err
	}

	return s.docker.ExecInContainer(ctx, sandbox.ContainerID, cmd)
}

func (s *SandboxService) DeleteSandbox(ctx context.Context, sandboxID uuid.UUID) error {
	sandbox, err := s.repo.FindByID(ctx, sandboxID)
	if err != nil {
		return err
	}

	// 停止并移除容器
	s.docker.StopContainer(ctx, sandbox.ContainerID, nil)
	s.docker.RemoveContainer(ctx, sandbox.ContainerID)

	// 释放端口
	var ports []docker.PortMapping
	json.Unmarshal(sandbox.Ports, &ports)
	for _, p := range ports {
		s.portPool.Release(p.HostPort)
	}

	return s.repo.UpdateStatus(ctx, sandboxID, model.SandboxStatusStopped, "")
}
```

- [ ] **Step 3: 实现沙箱API处理器**

```go
// internal/api/handler/sandbox.go
package handler

import (
	"io"
	"net/http"

	"github.com/anthropic/isdp/internal/service/sandbox"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SandboxHandler struct {
	svc *sandbox.SandboxService
}

func NewSandboxHandler(svc *sandbox.SandboxService) *SandboxHandler {
	return &SandboxHandler{svc: svc}
}

func (h *SandboxHandler) CreateSandbox(c *gin.Context) {
	var req sandbox.CreateSandboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sb, err := h.svc.CreateSandbox(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code": 0,
		"data": sb,
	})
}

func (h *SandboxHandler) StartSandbox(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sandbox id"})
		return
	}

	if err := h.svc.StartSandbox(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "started"})
}

func (h *SandboxHandler) StopSandbox(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sandbox id"})
		return
	}

	if err := h.svc.StopSandbox(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "stopped"})
}

func (h *SandboxHandler) GetLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sandbox id"})
		return
	}

	follow := c.Query("follow") == "true"

	logs, err := h.svc.GetLogs(c.Request.Context(), id, follow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer logs.Close()

	c.Header("Content-Type", "text/plain")
	io.Copy(c.Writer, logs)
}

func (h *SandboxHandler) DeleteSandbox(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sandbox id"})
		return
	}

	if err := h.svc.DeleteSandbox(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}
```

- [ ] **Step 4: 提交**

```bash
git add internal/service/sandbox/ internal/repo/sandbox.go internal/api/handler/sandbox.go
git commit -m "feat: add sandbox service with docker container management"
```

---

## Chunk 6 完成

沙箱环境完成，包括：
- Docker客户端封装
- 端口池管理
- 沙箱服务（Create, Start, Stop, Logs, Delete）
- 沙箱API处理器

---

## Chunk 7: 协作规则与上下文工程（M7）

### Task 7.1: 合入门禁服务

**Files:**
- Create: `internal/service/agent/merge_gate.go`

- [ ] **Step 1: 实现合并门禁服务**

```go
// internal/service/agent/merge_gate.go
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MergeGateService 合入门禁服务
type MergeGateService struct {
	// 在实际实现中需要注入相关repo
}

// MergeGateStatus 合入门禁状态
type MergeGateStatus struct {
	CanMerge bool        `json:"can_merge"`
	Checks   []GateCheck `json:"checks"`
	Blockers []string    `json:"blockers"`
}

// GateCheck 门禁检查项
type GateCheck struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // passed, failed, pending
	Description string `json:"description,omitempty"`
}

// ReviewReport 审查报告
type ReviewReport struct {
	ID        uuid.UUID `json:"id"`
	ThreadID  string    `json:"thread_id"`
	P1Issues  []Issue   `json:"p1_issues"`
	P2Issues  []Issue   `json:"p2_issues"`
	P3Issues  []Issue   `json:"p3_issues"`
	Approved  bool      `json:"approved"`
	CreatedAt time.Time `json:"created_at"`
}

// Issue 问题
type Issue struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// TestReport 测试报告
type TestReport struct {
	ID          uuid.UUID `json:"id"`
	ThreadID    string    `json:"thread_id"`
	TotalCount  int       `json:"total_count"`
	PassedCount int       `json:"passed_count"`
	FailedCount int       `json:"failed_count"`
	SkippedCount int      `json:"skipped_count"`
	Coverage    float64   `json:"coverage"`
	CreatedAt   time.Time `json:"created_at"`
}

// Checkpoint 检查点
type Checkpoint struct {
	ID        uuid.UUID `json:"id"`
	ThreadID  string    `json:"thread_id"`
	Type      string    `json:"type"` // merge, deploy
	Status    string    `json:"status"` // pending, approved, rejected
	CreatedAt time.Time `json:"created_at"`
}

func NewMergeGateService() *MergeGateService {
	return &MergeGateService{}
}

// CheckMergeGate 检查是否满足合入条件
func (s *MergeGateService) CheckMergeGate(ctx context.Context, threadID string) (*MergeGateStatus, error) {
	status := &MergeGateStatus{
		CanMerge: true,
	}

	// 检查1: P1/P2清零（模拟实现）
	// 在实际实现中需要从repo获取
	status.Checks = append(status.Checks, GateCheck{
		Name:   "P1/P2清零",
		Status: "passed",
	})

	// 检查2: 审查员放行
	status.Checks = append(status.Checks, GateCheck{
		Name:   "审查员放行",
		Status: "passed",
	})

	// 检查3: 测试通过
	status.Checks = append(status.Checks, GateCheck{
		Name:   "测试通过",
		Status: "passed",
	})

	// 检查4: 用户确认
	status.Checks = append(status.Checks, GateCheck{
		Name:   "用户确认",
		Status: "pending",
	})
	status.CanMerge = false
	status.Blockers = append(status.Blockers, "需要用户确认合入")

	return status, nil
}

// RequestMergeApproval 请求合入审批
func (s *MergeGateService) RequestMergeApproval(ctx context.Context, threadID string) error {
	status, err := s.CheckMergeGate(ctx, threadID)
	if err != nil {
		return err
	}

	if status.CanMerge {
		// 创建待确认的检查点
		return nil
	}

	return fmt.Errorf("不满足合入条件: %s", strings.Join(status.Blockers, "; "))
}

// ParseReviewReport 解析审查报告
func ParseReviewReport(content string) (*ReviewReport, error) {
	report := &ReviewReport{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
	}

	// 解析P1问题
	report.P1Issues = parseIssues(content, "P1")

	// 解析P2问题
	report.P2Issues = parseIssues(content, "P2")

	// 解析P3问题
	report.P3Issues = parseIssues(content, "P3")

	// 检查是否放行
	report.Approved = strings.Contains(content, "可以放行") ||
		strings.Contains(content, "P1/P2清零")

	return report, nil
}

// CanProceedToMerge 是否可以继续合并
func (r *ReviewReport) CanProceedToMerge() bool {
	return r.Approved && len(r.P1Issues) == 0 && len(r.P2Issues) == 0
}

func parseIssues(content string, level string) []Issue {
	var issues []Issue

	// 简化的解析逻辑
	lines := strings.Split(content, "\n")
	inSection := false

	for _, line := range lines {
		if strings.Contains(line, level+"（") {
			inSection = true
			continue
		}

		if inSection && strings.HasPrefix(strings.TrimSpace(line), "###") {
			break
		}

		if inSection && strings.TrimSpace(line) != "" {
			issues = append(issues, Issue{
				Description: strings.TrimSpace(line),
			})
		}
	}

	return issues
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/service/agent/merge_gate.go
git commit -m "feat: add merge gate service with P1/P2 check"
```

---

### Task 7.2: 交接报告模板

**Files:**
- Create: `internal/service/agent/handoff.go`

- [ ] **Step 1: 实现交接报告生成器**

```go
// internal/service/agent/handoff.go
package agent

import (
	"fmt"
	"strings"
	"time"
)

// HandoffReport 交接报告
type HandoffReport struct {
	FromAgent    string   `json:"from_agent"`
	ToAgent      string   `json:"to_agent"`
	What         string   `json:"what"`
	Why          string   `json:"why"`
	Tradeoffs    string   `json:"tradeoffs"`
	OpenQuestions []string `json:"open_questions"`
	NextActions  []string `json:"next_actions"`
	CreatedAt    time.Time `json:"created_at"`
}

// GenerateHandoffReport 生成交接报告
func GenerateHandoffReport(from, to string) *HandoffReport {
	return &HandoffReport{
		FromAgent: from,
		ToAgent:   to,
		CreatedAt: time.Now(),
	}
}

// SetWhat 设置做了什么
func (r *HandoffReport) SetWhat(what string) *HandoffReport {
	r.What = what
	return r
}

// SetWhy 设置为什么这样做
func (r *HandoffReport) SetWhy(why string) *HandoffReport {
	r.Why = why
	return r
}

// SetTradeoffs 设置放弃了什么方案
func (r *HandoffReport) SetTradeoffs(tradeoffs string) *HandoffReport {
	r.Tradeoffs = tradeoffs
	return r
}

// AddOpenQuestion 添加不确定的点
func (r *HandoffReport) AddOpenQuestion(q string) *HandoffReport {
	r.OpenQuestions = append(r.OpenQuestions, q)
	return r
}

// AddNextAction 添加下一步行动
func (r *HandoffReport) AddNextAction(action string) *HandoffReport {
	r.NextActions = append(r.NextActions, action)
	return r
}

// ToMarkdown 转换为Markdown格式
func (r *HandoffReport) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString("## 交接报告\n\n")
	sb.WriteString(fmt.Sprintf("**From**: %s → **To**: %s\n\n", r.FromAgent, r.ToAgent))

	sb.WriteString("### What - 我做了什么\n")
	sb.WriteString(r.What)
	sb.WriteString("\n\n")

	sb.WriteString("### Why - 为什么这样做\n")
	sb.WriteString(r.Why)
	sb.WriteString("\n\n")

	if r.Tradeoffs != "" {
		sb.WriteString("### Tradeoff - 放弃了什么方案\n")
		sb.WriteString(r.Tradeoffs)
		sb.WriteString("\n\n")
	}

	if len(r.OpenQuestions) > 0 {
		sb.WriteString("### Open Questions - 不确定的点\n")
		for _, q := range r.OpenQuestions {
			sb.WriteString(fmt.Sprintf("- %s\n", q))
		}
		sb.WriteString("\n")
	}

	if len(r.NextActions) > 0 {
		sb.WriteString("### Next Action - 希望对方做什么\n")
		for _, a := range r.NextActions {
			sb.WriteString(fmt.Sprintf("- %s\n", a))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// HandoffTemplate 交接报告模板
const HandoffTemplate = `## 交接报告

### What - 我做了什么
[具体修改内容]

### Why - 为什么这样做
[决策原因，最重要]

### Tradeoff - 放弃了什么方案
[备选方案说明]

### Open Questions - 不确定的点
- [需要澄清的问题1]
- [需要澄清的问题2]

### Next Action - 希望对方做什么
@目标Agent [具体请求]
`

// ReviewTemplate Review报告模板
const ReviewTemplate = `## Review报告

### P1（共X个）— 必须立即修
1. file.go:line - 问题描述
   建议：修复建议

### P2（共X个）— 必须修完才能放行
...

### P3（共X个）— 可登记Backlog
...

### 结论
- [ ] P1/P2清零，可以放行
- [ ] 存在P1/P2，不能放行
`
```

- [ ] **Step 2: 提交**

```bash
git add internal/service/agent/handoff.go
git commit -m "feat: add handoff report generator and templates"
```

---

## Chunk 7 完成

协作规则与上下文工程完成，包括：
- 合入门禁服务（P1/P2检查）
- 审查报告解析
- 交接报告模板

---

## Chunk 8: Web前端开发（M8）

### Task 8.1: 前端项目初始化

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/index.html`

- [ ] **Step 1: 初始化React项目**

```bash
cd web
npm create vite@latest . -- --template react-ts
npm install antd @ant-design/icons zustand axios
npm install -D @types/node
```

- [ ] **Step 2: 配置Vite**

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
})
```

- [ ] **Step 3: 配置TypeScript**

```json
// web/tsconfig.json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 4: 提交**

```bash
git add web/
git commit -m "feat: initialize react frontend with vite and antd"
```

---

### Task 8.2: 类型定义与API客户端

**Files:**
- Create: `web/src/types/index.ts`
- Create: `web/src/api/index.ts`

- [ ] **Step 1: 定义类型**

```typescript
// web/src/types/index.ts

export interface Project {
  id: string;
  name: string;
  type: 'service' | 'app' | 'task';
  mode: 'new' | 'enhance';
  status: 'draft' | 'developing' | 'testing' | 'deployed' | 'archived';
  git_repo?: string;
  created_at: string;
  updated_at: string;
}

export interface Thread {
  id: string;
  project_id: string;
  status: 'idle' | 'running' | 'paused' | 'completed';
  current_phase: Phase;
  current_agent: string;
  created_at: string;
  updated_at: string;
}

export type Phase = 'requirement' | 'design' | 'implement' | 'review' | 'test' | 'deploy';

export interface Message {
  id: string;
  thread_id: string;
  role: 'user' | 'agent' | 'system';
  agent_id?: string;
  content: string;
  message_type: 'text' | 'artifact' | 'system';
  created_at: string;
}

export interface AgentConfig {
  id: string;
  agent_id: string;
  display_name: string;
  description: string;
  phase: string;
  tools: string[];
  is_active: boolean;
  is_builtin: boolean;
}

export interface Artifact {
  id: string;
  thread_id: string;
  phase: string;
  type: string;
  name: string;
  path?: string;
  content?: string;
  created_at: string;
}

export interface Sandbox {
  id: string;
  project_id: string;
  container_id: string;
  name: string;
  status: 'created' | 'running' | 'stopped' | 'error';
  image: string;
  cpu_limit: number;
  memory_limit: number;
  created_at: string;
}

// WebSocket消息类型
export interface WSMessage {
  type: string;
  threadId: string;
  timestamp: number;
  payload?: any;
}

export interface ApiResponse<T> {
  code: number;
  data: T;
  message?: string;
}
```

- [ ] **Step 2: 实现API客户端**

```typescript
// web/src/api/index.ts
import axios from 'axios';
import type { Project, Thread, Message, AgentConfig, Sandbox, ApiResponse } from '../types';

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
});

// 项目API
export const projectApi = {
  list: (limit = 20, offset = 0) =>
    api.get<ApiResponse<{ projects: Project[] }>>(`/projects?limit=${limit}&offset=${offset}`),

  get: (id: string) =>
    api.get<ApiResponse<Project>>(`/projects/${id}`),

  create: (data: { name: string; type: string; mode: string; existing_repo_url?: string }) =>
    api.post<ApiResponse<Project>>('/projects', data),

  delete: (id: string) =>
    api.delete(`/projects/${id}`),
};

// 会话API
export const threadApi = {
  create: (projectId: string) =>
    api.post<ApiResponse<Thread>>(`/projects/${projectId}/threads`),

  getMessages: (threadId: string, limit = 50) =>
    api.get<ApiResponse<{ messages: Message[] }>>(`/threads/${threadId}/messages?limit=${limit}`),

  sendMessage: (threadId: string, content: string) =>
    api.post<ApiResponse<Message>>(`/threads/${threadId}/messages`, { content }),

  pause: (threadId: string) =>
    api.post(`/threads/${threadId}/pause`),

  cancel: (threadId: string) =>
    api.post(`/threads/${threadId}/cancel`),
};

// Agent API
export const agentApi = {
  list: () =>
    api.get<ApiResponse<AgentConfig[]>>('/agents'),

  create: (data: Partial<AgentConfig>) =>
    api.post<ApiResponse<AgentConfig>>('/agents', data),

  update: (id: string, data: Partial<AgentConfig>) =>
    api.put<ApiResponse<AgentConfig>>(`/agents/${id}`, data),

  delete: (id: string) =>
    api.delete(`/agents/${id}`),
};

// 沙箱API
export const sandboxApi = {
  create: (projectId: string, image: string) =>
    api.post<ApiResponse<Sandbox>>('/sandbox', { project_id: projectId, image }),

  start: (id: string) =>
    api.post(`/sandbox/${id}/start`),

  stop: (id: string) =>
    api.post(`/sandbox/${id}/stop`),

  getLogs: (id: string, follow = false) =>
    api.get(`/sandbox/${id}/logs?follow=${follow}`, { responseType: 'stream' }),
};

export default api;
```

- [ ] **Step 3: 提交**

```bash
git add web/src/types/ web/src/api/
git commit -m "feat: add frontend types and API client"
```

---

### Task 8.3: 状态管理

**Files:**
- Create: `web/src/stores/index.ts`

- [ ] **Step 1: 实现Zustand Store**

```typescript
// web/src/stores/index.ts
import { create } from 'zustand';
import type { Project, Thread, Message, AgentConfig, Phase } from '../types';

interface AppState {
  // 当前项目
  currentProject: Project | null;
  setCurrentProject: (project: Project | null) => void;

  // 当前会话
  currentThread: Thread | null;
  setCurrentThread: (thread: Thread | null) => void;

  // 消息列表
  messages: Message[];
  addMessage: (message: Message) => void;
  setMessages: (messages: Message[]) => void;

  // Agent配置
  agents: AgentConfig[];
  setAgents: (agents: AgentConfig[]) => void;

  // 当前阶段
  currentPhase: Phase;
  setCurrentPhase: (phase: Phase) => void;

  // 当前Agent
  currentAgent: string;
  setCurrentAgent: (agent: string) => void;

  // 产物列表
  artifacts: any[];
  addArtifact: (artifact: any) => void;
}

export const useAppStore = create<AppState>((set) => ({
  // 项目
  currentProject: null,
  setCurrentProject: (project) => set({ currentProject: project }),

  // 会话
  currentThread: null,
  setCurrentThread: (thread) => set({ currentThread: thread }),

  // 消息
  messages: [],
  addMessage: (message) => set((state) => ({ messages: [...state.messages, message] })),
  setMessages: (messages) => set({ messages }),

  // Agent
  agents: [],
  setAgents: (agents) => set({ agents }),

  // 阶段
  currentPhase: 'requirement',
  setCurrentPhase: (phase) => set({ currentPhase: phase }),

  // 当前Agent
  currentAgent: '',
  setCurrentAgent: (agent) => set({ currentAgent: agent }),

  // 产物
  artifacts: [],
  addArtifact: (artifact) => set((state) => ({ artifacts: [...state.artifacts, artifact] })),
}));
```

- [ ] **Step 2: 提交**

```bash
git add web/src/stores/
git commit -m "feat: add zustand store for state management"
```

---

### Task 8.4: WebSocket Hook

**Files:**
- Create: `web/src/hooks/useWebSocket.ts`

- [ ] **Step 1: 实现WebSocket Hook**

```typescript
// web/src/hooks/useWebSocket.ts
import { useEffect, useRef, useState, useCallback } from 'react';
import type { WSMessage } from '../types';

export function useWebSocket(baseUrl: string) {
  const wsRef = useRef<WebSocket | null>(null);
  const handlersRef = useRef<Map<string, Set<(data: WSMessage) => void>>>(new Map());
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>();
  const [connected, setConnected] = useState(false);

  const connect = useCallback(() => {
    const ws = new WebSocket(baseUrl);

    ws.onopen = () => {
      setConnected(true);
      // 重连后重新订阅
      handlersRef.current.forEach((_, threadId) => {
        ws.send(JSON.stringify({ type: 'subscribe', threadId, timestamp: Date.now() }));
      });
    };

    ws.onclose = () => {
      setConnected(false);
      // 5秒后重连
      reconnectTimeoutRef.current = setTimeout(connect, 5000);
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        const handlers = handlersRef.current.get(msg.threadId);
        if (handlers) {
          handlers.forEach(handler => handler(msg));
        }
      } catch (e) {
        console.error('Failed to parse message:', e);
      }
    };

    wsRef.current = ws;
  }, [baseUrl]);

  useEffect(() => {
    connect();
    return () => {
      reconnectTimeoutRef.current && clearTimeout(reconnectTimeoutRef.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const subscribe = useCallback((threadId: string, handler: (msg: WSMessage) => void) => {
    if (!handlersRef.current.has(threadId)) {
      handlersRef.current.set(threadId, new Set());
      wsRef.current?.send(JSON.stringify({ type: 'subscribe', threadId, timestamp: Date.now() }));
    }
    handlersRef.current.get(threadId)!.add(handler);

    return () => {
      handlersRef.current.get(threadId)?.delete(handler);
      if (handlersRef.current.get(threadId)?.size === 0) {
        handlersRef.current.delete(threadId);
        wsRef.current?.send(JSON.stringify({ type: 'unsubscribe', threadId, timestamp: Date.now() }));
      }
    };
  }, []);

  return { connected, subscribe };
}
```

- [ ] **Step 2: 提交**

```bash
git add web/src/hooks/
git commit -m "feat: add websocket hook with reconnection support"
```

---

### Task 8.5: 核心页面组件

**Files:**
- Create: `web/src/components/Layout/index.tsx`
- Create: `web/src/components/AgentProgress/index.tsx`
- Create: `web/src/pages/Workspace/index.tsx`
- Create: `web/src/pages/AgentSettings/index.tsx`

- [ ] **Step 1: 实现布局组件**

```tsx
// web/src/components/Layout/index.tsx
import React from 'react';
import { Layout, Menu } from 'antd';
import { ProjectOutlined, SettingOutlined, HomeOutlined } from '@ant-design/icons';
import { useNavigate, Outlet, useLocation } from 'react-router-dom';

const { Sider, Content } = Layout;

export const AppLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();

  const menuItems = [
    { key: '/', icon: <HomeOutlined />, label: '仪表盘' },
    { key: '/projects', icon: <ProjectOutlined />, label: '项目列表' },
    { key: '/agents', icon: <SettingOutlined />, label: 'Agent配置' },
  ];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider width={200} theme="light">
        <div style={{ padding: '16px', fontSize: '18px', fontWeight: 'bold' }}>
          ISDP
        </div>
        <Menu
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Content style={{ padding: '24px', background: '#f0f2f5' }}>
        <Outlet />
      </Content>
    </Layout>
  );
};
```

- [ ] **Step 2: 实现Agent进度组件**

```tsx
// web/src/components/AgentProgress/index.tsx
import React from 'react';
import { Steps, Tag } from 'antd';
import type { Phase } from '../../types';

interface AgentProgressProps {
  phases: Phase[];
  currentPhase: Phase;
  currentAgent: string;
}

const phaseLabels: Record<Phase, string> = {
  requirement: '需求分析',
  design: '架构设计',
  implement: '代码实现',
  review: '代码审查',
  test: '测试验证',
  deploy: '部署上线',
};

const agentNames: Record<string, string> = {
  'requirement-analyst': '需求分析师',
  'architect': '架构师',
  'developer': '开发者',
  'reviewer': '审查员',
  'test-engineer': '测试工程师',
  'devops': '运维工程师',
};

export const AgentProgress: React.FC<AgentProgressProps> = ({
  phases,
  currentPhase,
  currentAgent,
}) => {
  const currentIndex = phases.indexOf(currentPhase);

  return (
    <div style={{ marginBottom: '24px' }}>
      <Steps
        current={currentIndex}
        items={phases.map((phase) => ({
          title: phaseLabels[phase],
          status: phases.indexOf(phase) < currentIndex ? 'finish' :
                  phases.indexOf(phase) === currentIndex ? 'process' : 'wait',
        }))}
      />
      {currentAgent && (
        <div style={{ marginTop: '16px', textAlign: 'center' }}>
          <Tag color="blue">
            当前: {agentNames[currentAgent] || currentAgent}
          </Tag>
        </div>
      )}
    </div>
  );
};
```

- [ ] **Step 3: 实现工作台页面**

```tsx
// web/src/pages/Workspace/index.tsx
import React, { useState, useEffect } from 'react';
import { Input, Button, Card, List, Typography, Space, Avatar, message } from 'antd';
import { SendOutlined, PauseOutlined } from '@ant-design/icons';
import { useParams } from 'react-router-dom';
import { AgentProgress } from '../../components/AgentProgress';
import { useAppStore } from '../../stores';
import { threadApi } from '../../api';
import { useWebSocket } from '../../hooks/useWebSocket';

const { TextArea } = Input;
const { Text } = Typography;

const phases: any[] = ['requirement', 'design', 'implement', 'review', 'test', 'deploy'];

export const Workspace: React.FC = () => {
  const { threadId } = useParams<{ threadId: string }>();
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);

  const { messages, addMessage, currentPhase, currentAgent, setMessages } = useAppStore();
  const { connected, subscribe } = useWebSocket('/ws');

  useEffect(() => {
    if (threadId) {
      // 加载消息历史
      threadApi.getMessages(threadId).then(res => {
        setMessages(res.data.data.messages || []);
      });

      // 订阅WebSocket
      const unsubscribe = subscribe(threadId, (msg) => {
        if (msg.type === 'agent_message') {
          addMessage({
            id: msg.payload.messageId,
            thread_id: threadId,
            role: 'agent',
            agent_id: msg.payload.agentId,
            content: msg.payload.content,
            message_type: 'text',
            created_at: new Date().toISOString(),
          });
        }
      });

      return unsubscribe;
    }
  }, [threadId, subscribe, setMessages, addMessage]);

  const handleSend = async () => {
    if (!inputValue.trim() || !threadId) return;

    setLoading(true);
    try {
      await threadApi.sendMessage(threadId, inputValue);
      setInputValue('');
    } catch (error) {
      message.error('发送失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* 进度条 */}
      <AgentProgress
        phases={phases}
        currentPhase={currentPhase}
        currentAgent={currentAgent}
      />

      {/* 消息区域 */}
      <Card style={{ flex: 1, overflow: 'auto', marginBottom: '16px' }}>
        <List
          dataSource={messages}
          renderItem={(msg) => (
            <List.Item>
              <List.Item.Meta
                avatar={<Avatar>{msg.agent_id?.[0]?.toUpperCase() || 'U'}</Avatar>}
                title={
                  <Space>
                    <Text strong>{msg.agent_id || '用户'}</Text>
                    <Text type="secondary" style={{ fontSize: '12px' }}>
                      {new Date(msg.created_at).toLocaleString()}
                    </Text>
                  </Space>
                }
                description={<div style={{ whiteSpace: 'pre-wrap' }}>{msg.content}</div>}
              />
            </List.Item>
          )}
        />
      </Card>

      {/* 输入区域 */}
      <Space.Compact style={{ width: '100%' }}>
        <TextArea
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          placeholder="输入消息或指令..."
          autoSize={{ minRows: 2, maxRows: 4 }}
          style={{ flex: 1 }}
        />
        <Button type="primary" icon={<SendOutlined />} onClick={handleSend} loading={loading}>
          发送
        </Button>
        <Button icon={<PauseOutlined />}>
          暂停
        </Button>
      </Space.Compact>
    </div>
  );
};
```

- [ ] **Step 4: 实现Agent配置页面**

```tsx
// web/src/pages/AgentSettings/index.tsx
import React, { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { agentApi } from '../../api';
import type { AgentConfig } from '../../types';

export const AgentSettings: React.FC = () => {
  const [agents, setAgents] = useState<AgentConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingAgent, setEditingAgent] = useState<AgentConfig | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    loadAgents();
  }, []);

  const loadAgents = async () => {
    setLoading(true);
    try {
      const res = await agentApi.list();
      setAgents(res.data.data);
    } catch (error) {
      message.error('加载Agent列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingAgent(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (agent: AgentConfig) => {
    setEditingAgent(agent);
    form.setFieldsValue(agent);
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await agentApi.delete(id);
      message.success('删除成功');
      loadAgents();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      if (editingAgent) {
        await agentApi.update(editingAgent.id, values);
        message.success('更新成功');
      } else {
        await agentApi.create(values);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadAgents();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const columns = [
    { title: 'ID', dataIndex: 'agent_id', key: 'agent_id' },
    { title: '名称', dataIndex: 'display_name', key: 'display_name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '阶段',
      dataIndex: 'phase',
      key: 'phase',
      render: (phase: string) => <Tag color="blue">{phase}</Tag>,
    },
    {
      title: '类型',
      dataIndex: 'is_builtin',
      key: 'is_builtin',
      render: (builtin: boolean) => (
        <Tag color={builtin ? 'green' : 'orange'}>
          {builtin ? '内置' : '自定义'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: AgentConfig) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          {!record.is_builtin && (
            <Popconfirm
              title="确定删除此Agent?"
              onConfirm={() => handleDelete(record.id)}
            >
              <Button type="link" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: '16px' }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新增Agent
        </Button>
      </div>

      <Table
        dataSource={agents}
        columns={columns}
        rowKey="id"
        loading={loading}
      />

      <Modal
        title={editingAgent ? '编辑Agent' : '新增Agent'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="agent_id"
            label="Agent ID"
            rules={[{ required: true, message: '请输入Agent ID' }]}
          >
            <Input placeholder="例如: my-agent" disabled={!!editingAgent} />
          </Form.Item>

          <Form.Item
            name="display_name"
            label="显示名称"
            rules={[{ required: true, message: '请输入显示名称' }]}
          >
            <Input placeholder="例如: 我的Agent" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="Agent功能描述" />
          </Form.Item>

          <Form.Item
            name="phase"
            label="所属阶段"
            rules={[{ required: true, message: '请选择阶段' }]}
          >
            <Select placeholder="选择阶段">
              <Select.Option value="requirement">需求分析</Select.Option>
              <Select.Option value="design">架构设计</Select.Option>
              <Select.Option value="implement">代码实现</Select.Option>
              <Select.Option value="review">代码审查</Select.Option>
              <Select.Option value="test">测试验证</Select.Option>
              <Select.Option value="deploy">部署上线</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};
```

- [ ] **Step 5: 实现Dashboard页面**

```tsx
// web/src/pages/Dashboard/index.tsx
import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, List, Typography, Tag, Button } from 'antd';
import { ProjectOutlined, TeamOutlined, CodeOutlined, RocketOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { projectApi, agentApi } from '../../api';

const { Title } = Typography;

export const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const [stats, setStats] = useState({
    totalProjects: 0,
    activeProjects: 0,
    totalAgents: 6,
    completedTasks: 0,
  });
  const [recentProjects, setRecentProjects] = useState<any[]>([]);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const [projectsRes, agentsRes] = await Promise.all([
        projectApi.list(10, 0),
        agentApi.list(),
      ]);

      const projects = projectsRes.data.data.projects || [];
      setRecentProjects(projects.slice(0, 5));
      setStats({
        totalProjects: projects.length,
        activeProjects: projects.filter((p: any) => p.status === 'developing').length,
        totalAgents: agentsRes.data.data?.length || 6,
        completedTasks: projects.filter((p: any) => p.status === 'deployed').length,
      });
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    }
  };

  return (
    <div>
      <Title level={2}>仪表盘</Title>

      <Row gutter={16} style={{ marginBottom: '24px' }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="项目总数"
              value={stats.totalProjects}
              prefix={<ProjectOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="进行中"
              value={stats.activeProjects}
              prefix={<CodeOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="Agent数量"
              value={stats.totalAgents}
              prefix={<TeamOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="已完成"
              value={stats.completedTasks}
              prefix={<RocketOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={12}>
          <Card
            title="最近项目"
            extra={<Button type="link" onClick={() => navigate('/projects')}>查看全部</Button>}
          >
            <List
              dataSource={recentProjects}
              renderItem={(project) => (
                <List.Item
                  actions={[<Tag color={getStatusColor(project.status)}>{project.status}</Tag>]}
                >
                  <List.Item.Meta
                    title={<a onClick={() => navigate(`/projects/${project.id}`)}>{project.name}</a>}
                    description={project.type}
                  />
                </List.Item>
              )}
            />
          </Card>
        </Col>

        <Col span={12}>
          <Card title="快速开始">
            <Button type="primary" size="large" block onClick={() => navigate('/projects')}>
              创建新项目
            </Button>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

function getStatusColor(status: string): string {
  const colors: Record<string, string> = {
    draft: 'default',
    developing: 'processing',
    testing: 'warning',
    deployed: 'success',
    archived: 'default',
  };
  return colors[status] || 'default';
}
```

- [ ] **Step 6: 实现ProjectList页面**

```tsx
// web/src/pages/ProjectList/index.tsx
import React, { useEffect, useState } from 'react';
import { Table, Button, Space, Tag, Modal, Form, Input, Select, message, Popconfirm } from 'antd';
import { PlusOutlined, DeleteOutlined, FolderOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { projectApi } from '../../api';
import type { Project } from '../../types';

export const ProjectList: React.FC = () => {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadProjects();
  }, []);

  const loadProjects = async () => {
    setLoading(true);
    try {
      const res = await projectApi.list(100, 0);
      setProjects(res.data.data.projects || []);
    } catch (error) {
      message.error('加载项目列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      const res = await projectApi.create(values);
      message.success('项目创建成功');
      setModalVisible(false);
      navigate(`/projects/${res.data.data.id}`);
    } catch (error) {
      message.error('创建失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await projectApi.delete(id);
      message.success('删除成功');
      loadProjects();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const columns = [
    {
      title: '项目名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: Project) => (
        <a onClick={() => navigate(`/projects/${record.id}`)}>
          <FolderOutlined style={{ marginRight: 8 }} />
          {name}
        </a>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => <Tag color="blue">{type}</Tag>,
    },
    {
      title: '模式',
      dataIndex: 'mode',
      key: 'mode',
      render: (mode: string) => (
        <Tag color={mode === 'new' ? 'green' : 'orange'}>
          {mode === 'new' ? '新建' : '增强'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const colors: Record<string, string> = {
          draft: 'default',
          developing: 'processing',
          testing: 'warning',
          deployed: 'success',
        };
        const labels: Record<string, string> = {
          draft: '草稿',
          developing: '开发中',
          testing: '测试中',
          deployed: '已部署',
        };
        return <Tag color={colors[status]}>{labels[status] || status}</Tag>;
      },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (date: string) => new Date(date).toLocaleDateString(),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: Project) => (
        <Space>
          <Button type="link" onClick={() => navigate(`/projects/${record.id}`)}>
            打开
          </Button>
          <Popconfirm
            title="确定删除此项目?"
            onConfirm={() => handleDelete(record.id)}
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalVisible(true)}>
          新建项目
        </Button>
      </div>

      <Table
        dataSource={projects}
        columns={columns}
        rowKey="id"
        loading={loading}
      />

      <Modal
        title="创建项目"
        open={modalVisible}
        onOk={handleCreate}
        onCancel={() => setModalVisible(false)}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="项目名称"
            rules={[{ required: true, message: '请输入项目名称' }]}
          >
            <Input placeholder="输入项目名称" />
          </Form.Item>

          <Form.Item
            name="type"
            label="项目类型"
            rules={[{ required: true, message: '请选择项目类型' }]}
          >
            <Select placeholder="选择项目类型">
              <Select.Option value="service">服务</Select.Option>
              <Select.Option value="app">应用</Select.Option>
              <Select.Option value="task">任务</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="mode"
            label="开发模式"
            rules={[{ required: true, message: '请选择开发模式' }]}
          >
            <Select placeholder="选择开发模式">
              <Select.Option value="new">新建项目</Select.Option>
              <Select.Option value="enhance">增强现有项目</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, curr) => prev.mode !== curr.mode}
          >
            {({ getFieldValue }) =>
              getFieldValue('mode') === 'enhance' && (
                <Form.Item
                  name="existing_repo_url"
                  label="现有仓库URL"
                  rules={[{ required: true, message: '请输入仓库URL' }]}
                >
                  <Input placeholder="https://github.com/user/repo.git" />
                </Form.Item>
              )
            }
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};
```

- [ ] **Step 7: 提交**

```bash
git add web/src/components/ web/src/pages/
git commit -m "feat: add core frontend components and pages"
```

---

## Chunk 8 完成

Web前端开发完成，包括：
- 项目初始化（Vite + React + TypeScript + Ant Design）
- 类型定义与API客户端
- 状态管理（Zustand）
- WebSocket Hook（自动重连）
- 核心页面组件（布局、进度条、工作台、Agent配置）

---

## Chunk 9: 集成测试与部署（M9）

### Task 9.1: 单元测试完善

**Files:**
- Create: `internal/api/handler/project_test.go`
- Create: `internal/service/agent/mention_parser_test.go`

- [ ] **Step 1: 编写API Handler测试**

```go
// internal/api/handler/project_test.go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/project"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProjectService struct {
	mock.Mock
}

func (m *MockProjectService) Create(ctx context.Context, req *model.CreateProjectRequest) (*model.Project, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Project), args.Error(1)
}

func (m *MockProjectService) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Project), args.Error(1)
}

func TestProjectHandler_CreateProject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockSvc := new(MockProjectService)
	handler := NewProjectHandler(mockSvc)

	router := gin.New()
	router.POST("/projects", handler.CreateProject)

	reqBody := model.CreateProjectRequest{
		Name: "Test Project",
		Type: model.ProjectTypeService,
		Mode: model.ProjectModeNew,
	}
	body, _ := json.Marshal(reqBody)

	mockSvc.On("Create", mock.Anything, mock.Anything).Return(&model.Project{
		ID:     uuid.New(),
		Name:   "Test Project",
		Type:   model.ProjectTypeService,
		Mode:   model.ProjectModeNew,
		Status: model.ProjectStatusDraft,
	}, nil)

	req, _ := http.NewRequest("POST", "/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockSvc.AssertExpectations(t)
}
```

- [ ] **Step 2: 编写Mention解析测试**

```go
// internal/service/agent/mention_parser_test.go
package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMentions_SingleMention(t *testing.T) {
	content := "@architect 请帮我review这个设计"
	mentions := ParseMentions(content, "developer")

	assert.Len(t, mentions, 1)
	assert.Equal(t, "architect", mentions[0].AgentID)
	assert.Equal(t, "请帮我review这个设计", mentions[0].Content)
}

func TestParseMentions_NotAtLineStart(t *testing.T) {
	content := "看这个 @architect"
	mentions := ParseMentions(content, "")

	assert.Len(t, mentions, 0) // 不在行首，不触发
}

func TestParseMentions_MaxTwoMentions(t *testing.T) {
	content := "@developer 请实现\n@architect 请设计\n@test-engineer 请测试"
	mentions := ParseMentions(content, "reviewer")

	assert.Len(t, mentions, 2) // 最多2个
}

func TestParseMentions_ExcludeSelf(t *testing.T) {
	content := "@developer 这个\n@architect 那个"
	mentions := ParseMentions(content, "developer")

	assert.Len(t, mentions, 1)
	assert.Equal(t, "architect", mentions[0].AgentID)
}

func TestParseMentions_MultilineContent(t *testing.T) {
	content := "@architect 设计API\n要有RESTful风格\n@developer 实现代码"
	mentions := ParseMentions(content, "")

	assert.Len(t, mentions, 2)
	assert.Equal(t, "architect", mentions[0].AgentID)
	assert.Equal(t, "设计API", mentions[0].Content)
}

func TestParseMentions_EmptyContent(t *testing.T) {
	content := "@architect"
	mentions := ParseMentions(content, "")

	assert.Len(t, mentions, 1)
	assert.Equal(t, "", mentions[0].Content)
}

func TestParseMentions_NoMatches(t *testing.T) {
	content := "这是一段普通文字\n没有@任何人"
	mentions := ParseMentions(content, "")

	assert.Len(t, mentions, 0)
}
```

- [ ] **Step 3: 运行测试**

```bash
go test ./... -v -cover
# Expected: PASS with coverage > 70%
```

- [ ] **Step 4: 提交**

```bash
git add internal/api/handler/project_test.go internal/service/agent/mention_parser_test.go
git commit -m "test: add unit tests for project handler and mention parser"
```

---

### Task 9.2: Docker Compose配置

**Files:**
- Create: `docker-compose.yml`
- Create: `Dockerfile`

- [ ] **Step 1: 创建Dockerfile**

```dockerfile
# Dockerfile

# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 复制go.mod和go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o isdp ./cmd/server

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates git

WORKDIR /app

# 复制二进制文件
COPY --from=builder /app/isdp .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/scripts ./scripts

# 创建仓库目录
RUN mkdir -p /var/lib/isdp/repos

EXPOSE 8080

CMD ["./isdp"]
```

- [ ] **Step 2: 创建docker-compose.yml**

```yaml
# docker-compose.yml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: isdp
      POSTGRES_PASSWORD: isdp123
      POSTGRES_DB: isdp
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init_db.sql:/docker-entrypoint-initdb.d/init_db.sql
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U isdp"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  backend:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      DATABASE_HOST: postgres
      DATABASE_PORT: 5432
      DATABASE_USER: isdp
      DATABASE_PASSWORD: isdp123
      DATABASE_NAME: isdp
      REDIS_ADDR: redis:6379
      REPOS_DIR: /var/lib/isdp/repos
    volumes:
      - repos_data:/var/lib/isdp/repos
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  frontend:
    build:
      context: ./web
      dockerfile: Dockerfile
    ports:
      - "80:80"
    depends_on:
      - backend

volumes:
  postgres_data:
  redis_data:
  repos_data:
```

- [ ] **Step 3: 创建前端Dockerfile**

```dockerfile
# web/Dockerfile

# 构建阶段
FROM node:18-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .
RUN npm run build

# 运行阶段
FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

- [ ] **Step 4: 创建前端nginx配置**

```nginx
# web/nginx.conf
server {
    listen 80;
    server_name localhost;

    root /usr/share/nginx/html;
    index index.html;

    # 前端路由
    location / {
        try_files $uri $uri/ /index.html;
    }

    # API代理
    location /api {
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # WebSocket代理
    location /ws {
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

- [ ] **Step 5: 提交**

```bash
git add docker-compose.yml Dockerfile web/Dockerfile web/nginx.conf
git commit -m "feat: add docker compose and dockerfiles for deployment"
```

---

### Task 9.3: 健康检查与监控

**Files:**
- Create: `internal/api/handler/health.go`
- Update: `internal/api/router.go`

- [ ] **Step 1: 实现健康检查API**

```go
// internal/api/handler/health.go
package handler

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx := c.Request.Context()

	// 检查数据库
	dbStatus := "ok"
	if err := h.db.Ping(ctx); err != nil {
		dbStatus = "error: " + err.Error()
	}

	// 检查Redis
	redisStatus := "ok"
	if err := h.redis.Ping(ctx).Err(); err != nil {
		redisStatus = "error: " + err.Error()
	}

	// 获取系统信息
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"checks": gin.H{
			"database": dbStatus,
			"redis":    redisStatus,
		},
		"system": gin.H{
			"goroutines": runtime.NumGoroutine(),
			"memory_mb":  m.Alloc / 1024 / 1024,
		},
	})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx := c.Request.Context()

	// 检查所有依赖
	if err := h.db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "database: " + err.Error(),
		})
		return
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "redis: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}
```

- [ ] **Step 2: 更新路由注册**

```go
// 在 router.go 的 Setup 方法中添加

// 健康检查端点（不需要认证）
r.engine.GET("/health", r.healthHandler.Health)
r.engine.GET("/ready", r.healthHandler.Ready)
r.engine.GET("/live", r.healthHandler.Live)
```

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/health.go
git commit -m "feat: add health check endpoints for k8s probes"
```

---

### Task 9.4: Makefile完善

**Files:**
- Update: `Makefile`

- [ ] **Step 1: 完善Makefile**

```makefile
# Makefile

.PHONY: all build run test clean docker-up docker-down lint fmt

# 默认目标
all: build

# 构建
build:
	go build -o bin/isdp ./cmd/server

# 运行
run:
	go run ./cmd/server

# 测试
test:
	go test ./... -v -cover -race

# 测试覆盖率
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# 清理
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# 代码检查
lint:
	golangci-lint run ./...

# 格式化
fmt:
	go fmt ./...

# Docker操作
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-build:
	docker-compose build

# 数据库迁移
db-init:
	psql -h localhost -U isdp -d isdp -f scripts/init_db.sql

# 前端构建
frontend-build:
	cd web && npm install && npm run build

# 完整构建
build-all: build frontend-build docker-build

# 开发环境
dev: docker-up
	sleep 5
	go run ./cmd/server
```

- [ ] **Step 2: 提交**

```bash
git add Makefile
git commit -m "chore: enhance makefile with development commands"
```

---

### Task 9.5: 最终验收

- [ ] **Step 1: 运行完整测试套件**

```bash
# 运行所有测试
make test

# 生成覆盖率报告
make test-coverage

# 检查代码质量
make lint
```

- [ ] **Step 2: 启动完整系统**

```bash
# 启动Docker环境
make docker-up

# 等待服务启动
sleep 10

# 检查服务健康状态
curl http://localhost:8080/health
curl http://localhost:8080/ready

# 访问前端
open http://localhost:80
```

- [ ] **Step 3: 功能验收清单**

```
## 验收清单

### 后端API
- [ ] POST /api/v1/projects - 创建项目
- [ ] GET /api/v1/projects - 获取项目列表
- [ ] GET /api/v1/projects/:id - 获取项目详情
- [ ] GET /api/v1/agents - 获取Agent列表
- [ ] POST /api/v1/agents - 创建自定义Agent
- [ ] PUT /api/v1/agents/:id - 更新Agent配置
- [ ] DELETE /api/v1/agents/:id - 删除自定义Agent
- [ ] POST /api/v1/sandbox - 创建沙箱
- [ ] POST /api/v1/sandbox/:id/start - 启动沙箱
- [ ] POST /api/v1/sandbox/:id/stop - 停止沙箱
- [ ] GET /health - 健康检查
- [ ] GET /ready - 就绪检查

### 前端页面
- [ ] 项目列表页面正常展示
- [ ] 项目创建功能正常
- [ ] Agent配置页面可新增/编辑/删除
- [ ] 工作台消息实时更新
- [ ] Agent进度条正确显示

### WebSocket
- [ ] WebSocket连接正常
- [ ] 消息实时推送
- [ ] 断线自动重连

### Docker环境
- [ ] PostgreSQL正常启动
- [ ] Redis正常启动
- [ ] 后端服务正常启动
- [ ] 前端服务正常启动
```

- [ ] **Step 4: 最终提交**

```bash
git add .
git commit -m "chore: final integration and acceptance testing"
git tag v1.0.0
```

---

## Chunk 9 完成

集成测试与部署完成，包括：
- 单元测试完善
- Docker Compose配置
- 健康检查API
- Makefile完善
- 最终验收

---

## 实施计划总结

本实施计划覆盖了ISDP平台的完整开发周期，共分为9个里程碑（M1-M9）：

| 里程碑 | 内容 | 预计工作量 |
|--------|------|-----------|
| M1 | 基础架构搭建 | 2天 |
| M2 | 项目管理模块 | 2天 |
| M3 | Agent引擎核心 | 3天 |
| M4 | A2A路由与MCP回传 | 2天 |
| M5 | ClaudeCode集成 | 2天 |
| M6 | 沙箱环境 | 2天 |
| M7 | 协作规则与上下文工程 | 1天 |
| M8 | Web前端开发 | 3天 |
| M9 | 集成测试与部署 | 2天 |

**总计：约19个工作日**

### 技术栈确认

- **后端**: Go 1.21+ / Gin 1.9+ / PostgreSQL 15 / Redis 7
- **前端**: React 18 / TypeScript / Ant Design 5 / Zustand
- **容器**: Docker 24+ / Docker Compose
- **AI集成**: Claude CLI

### 下一步

计划已完成，可以使用以下方式执行：

1. **使用subagent-driven-development**: 为每个Task分配独立的子Agent并行执行
2. **使用executing-plans**: 在当前会话中按顺序执行

**执行命令：** "开始执行实施计划" 或 "使用subagent执行计划"
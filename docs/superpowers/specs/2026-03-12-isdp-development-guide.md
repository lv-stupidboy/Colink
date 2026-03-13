# ISDP平台开发指导文档

> 智能软件开发平台（ISDP）AI开发完整指南
>
> 版本：1.2 | 日期：2026-03-12 | 状态：草案

---

## 目录

1. [项目概述与技术架构](#第1章项目概述与技术架构)
2. [基础架构搭建](#第2章基础架构搭建m1)
3. [项目管理模块](#第3章项目管理模块m2)
4. [Agent引擎核心](#第4章agent引擎核心m3)
5. [A2A路由与MCP回传](#第5章a2a路由与mcp回传m4)
6. [ClaudeCode集成](#第6章claudecode集成m5)
7. [沙箱环境](#第7章沙箱环境m6)
8. [协作规则与上下文工程](#第8章协作规则与上下文工程m7)
9. [Web前端开发](#第9章web前端开发m8)
10. [集成测试与部署](#第10章集成测试与部署m9)
11. [附录](#附录)

---

## 第1章：项目概述与技术架构

### 1.1 项目背景与目标

**一句话定位**：构建一个项目级多Agent协同开发系统，让用户从想法快速获得可运行产品。

**一期MVP目标**：
- 6个Agent角色（需求分析师、架构师、开发者、审查员、测试工程师、运维工程师）
- 串行工作流 + A2A路由
- 本地Docker沙箱
- 基础Web界面
- 支持服务类型项目开发

### 1.2 系统架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                     Web前端 (React)                          │
└─────────────────────────────────────────────────────────────┘
                              ↓ REST/WebSocket
┌─────────────────────────────────────────────────────────────┐
│                     后端服务 (Go + Gin)                       │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │项目管理 │ │Agent引擎│ │沙箱服务 │ │Git服务  │          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
└─────────────────────────────────────────────────────────────┘
                              ↓ CLI Spawn
┌─────────────────────────────────────────────────────────────┐
│                     Agent实例层                              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │Claude   │ │Claude   │ │Claude   │ │Claude   │          │
│  │(需求)   │ │(架构)   │ │(开发)   │ │(审查)   │          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                     沙箱环境 (Docker)                        │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  PostgreSQL    │    Redis    │    Git仓库    │    文件存储  │
└─────────────────────────────────────────────────────────────┘
```

### 1.3 技术选型

| 层级 | 技术 | 版本 |
|------|------|------|
| 后端语言 | Go | 1.21+ |
| 后端框架 | Gin | 1.9+ |
| 数据库 | PostgreSQL | 15+ |
| 缓存 | Redis | 7+ |
| 前端框架 | React | 18+ |
| UI组件库 | Ant Design | 5+ |
| 容器 | Docker | 24+ |

### 1.4 推荐目录结构

```
isdp/
├── cmd/
│   └── server/          # 主程序入口
├── internal/
│   ├── api/             # HTTP API处理
│   ├── service/         # 业务逻辑
│   │   ├── project/     # 项目管理
│   │   ├── agent/       # Agent引擎
│   │   ├── sandbox/     # 沙箱管理
│   │   └── git/         # Git操作
│   ├── model/           # 数据模型
│   ├── repo/            # 数据访问
│   └── ws/              # WebSocket处理
├── pkg/
│   ├── claude/          # Claude CLI适配器
│   ├── docker/          # Docker操作封装
│   └── utils/           # 工具函数
├── web/                 # 前端项目
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── stores/
│   │   └── services/
│   └── package.json
├── configs/             # 配置文件
├── scripts/             # 构建脚本
└── docs/                # 文档
```

---

## 第2章：基础架构搭建（M1）

### 2.1 后端项目初始化

**目标**：搭建Go后端项目骨架，包含基础框架和配置管理。

**技术要点**：
- 使用Gin框架搭建RESTful API
- 使用Viper管理配置
- 使用Zap进行日志记录
- 实现优雅关闭

**核心代码结构**：
```
cmd/server/main.go          # 入口
internal/api/router.go      # 路由注册
internal/api/middleware/    # 中间件（CORS、认证、日志）
configs/config.yaml         # 配置文件
```

**API路由规划**：
```
/api/v1
├── /projects          # 项目管理
│   ├── GET    /       # 列表
│   ├── POST   /       # 创建
│   ├── GET    /:id    # 详情
│   └── PUT    /:id    # 更新
├── /threads           # 开发会话
│   ├── POST   /       # 创建会话
│   ├── GET    /:id/messages  # 消息列表
│   └── POST   /:id/messages  # 发送消息
├── /agents            # Agent管理
│   └── GET    /       # Agent列表
├── /sandbox           # 沙箱管理
│   ├── POST   /:projectId/start   # 启动
│   └── GET    /:projectId/logs    # 日志
└── /ws                # WebSocket连接
```

### 2.2 数据库设计

**核心数据表**：

| 表名 | 说明 | 关键字段 |
|------|------|----------|
| projects | 项目表 | id, name, type, status, git_repo |
| threads | 开发会话表 | id, project_id, status, current_phase |
| messages | 消息表 | id, thread_id, role, content, agent_id |
| agent_configs | Agent配置表 | id, agent_id, display_name, phase, tools |
| agent_invocations | Agent调用记录 | id, thread_id, agent_id, status, depth |
| artifacts | 产物表 | id, thread_id, type, path, content |
| sandbox_containers | 沙箱容器表 | id, project_id, container_id, status |

**数据库Schema**：

```sql
-- 项目表
CREATE TABLE projects (
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
CREATE TABLE threads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id),
    status VARCHAR(50) DEFAULT 'idle',  -- idle, running, paused, completed
    current_phase VARCHAR(50),  -- requirement, design, implement, review, test, deploy
    current_agent VARCHAR(50),
    depth INTEGER DEFAULT 0,
    abort_token VARCHAR(100),  -- 取消信号token
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 消息表
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id),
    role VARCHAR(50) NOT NULL,  -- user, agent, system
    agent_id VARCHAR(50),  -- 需求分析师, 架构师, 开发者...
    content TEXT,
    message_type VARCHAR(50) DEFAULT 'text',  -- text, artifact, system
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Agent配置表
CREATE TABLE agent_configs (
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
CREATE TABLE agent_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id),
    agent_id VARCHAR(50) NOT NULL,
    session_id VARCHAR(100),  -- Claude session ID
    status VARCHAR(50) DEFAULT 'running',  -- running, completed, failed, cancelled
    depth INTEGER DEFAULT 0,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP
);

-- 产物表
CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id),
    phase VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- requirement_doc, architecture, code, test_report
    name VARCHAR(255),
    path VARCHAR(500),
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 沙箱容器表
CREATE TABLE sandbox_containers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id),
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
 '["Read", "Write", "Edit", "Bash"]');
```

### 2.3 Redis设计

**Key设计**：

| Key模式 | 说明 | TTL |
|---------|------|-----|
| `thread:{threadId}:worklist` | Agent执行队列 | 会话结束清除 |
| `thread:{threadId}:context` | 会话上下文缓存 | 24h |
| `invocation:{invocationId}:token` | MCP回调token | 30min |
| `agent:{agentId}:status` | Agent状态 | 5min |
| `agent_configs:active` | Agent配置缓存 | 10min |

### 2.4 开发环境搭建

**必需软件**：
- Go 1.21+
- Node.js 18+
- PostgreSQL 15+
- Redis 7+
- Docker 24+

**初始化命令**：

```bash
# 后端
go mod init github.com/your-org/isdp
go get github.com/gin-gonic/gin
go get github.com/spf13/viper
go get github.com/jackc/pgx/v5
go get github.com/redis/go-redis/v9

# 前端
cd web && npm create vite@latest . -- --template react-ts
npm install antd @ant-design/icons zustand axios
```

---

## 第3章：项目管理模块（M2）

### 3.1 功能概述

**目标**：实现项目的CRUD操作，作为用户开发需求的入口。

**核心功能**：
- 项目创建与初始化
- 项目状态跟踪
- 项目列表与详情查询
- 支持新建项目模式和增量开发模式

### 3.2 数据模型

```go
// internal/model/project.go

type ProjectType string
const (
    ProjectTypeService ProjectType = "service"
    ProjectTypeApp     ProjectType = "app"
    ProjectTypeTask    ProjectType = "task"
)

type ProjectMode string
const (
    ProjectModeNew      ProjectMode = "new"
    ProjectModeEnhance  ProjectMode = "enhance"
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
    ID        uuid.UUID      `json:"id"`
    Name      string         `json:"name"`
    Type      ProjectType    `json:"type"`
    Mode      ProjectMode    `json:"mode"`
    Status    ProjectStatus  `json:"status"`
    GitRepo   string         `json:"git_repo,omitempty"`
    Config    json.RawMessage `json:"config,omitempty"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
}
```

### 3.3 API实现

**创建项目流程**：
```
用户提交项目信息
    ↓
验证项目类型和模式
    ↓
创建Git仓库（如需）
    ↓
生成项目配置
    ↓
保存到数据库
    ↓
返回项目ID
```

**核心Service代码结构**：

```go
// internal/service/project/service.go

type ProjectService struct {
    repo   repo.ProjectRepository
    gitSvc *git.Service
}

func (s *ProjectService) Create(ctx context.Context, req *CreateProjectRequest) (*Project, error) {
    // 1. 验证
    if err := req.Validate(); err != nil {
        return nil, err
    }

    // 2. 创建项目记录
    project := &Project{
        ID:     uuid.New(),
        Name:   req.Name,
        Type:   req.Type,
        Mode:   req.Mode,
        Status: ProjectStatusDraft,
    }

    // 3. 根据模式处理Git仓库
    switch req.Mode {
    case ProjectModeNew:
        // 新项目模式：初始化空仓库
        repoURL, err := s.gitSvc.InitRepo(ctx, project.ID, req.Name)
        if err != nil {
            return nil, err
        }
        project.GitRepo = repoURL

    case ProjectModeEnhance:
        // 增量开发模式：克隆已有仓库
        if req.ExistingRepoURL == "" {
            return nil, errors.New("enhance mode requires existing_repo_url")
        }

        // 克隆现有仓库到项目目录
        repoPath, err := s.gitSvc.CloneRepo(ctx, project.ID, req.ExistingRepoURL, req.Branch)
        if err != nil {
            return nil, fmt.Errorf("failed to clone repository: %w", err)
        }

        // 分析现有代码结构
        codeStructure, err := s.gitSvc.AnalyzeStructure(ctx, repoPath)
        if err != nil {
            return nil, fmt.Errorf("failed to analyze code structure: %w", err)
        }

        // 保存代码结构到项目配置
        project.Config, _ = json.Marshal(map[string]interface{}{
            "existing_repo_url": req.ExistingRepoURL,
            "branch":           req.Branch,
            "code_structure":   codeStructure,
        })
        project.GitRepo = req.ExistingRepoURL
    }

    // 4. 保存
    if err := s.repo.Create(ctx, project); err != nil {
        return nil, err
    }

    return project, nil
}
```

### 3.3.1 增量开发模式流程

```
用户提供现有仓库URL
    ↓
验证仓库访问权限
    ↓
克隆仓库到项目目录
    ↓
分析代码结构（语言、框架、目录结构）
    ↓
生成代码结构报告
    ↓
保存项目配置
    ↓
用户可开始增量开发
```

**增量开发上下文注入**：

```go
// 为增量开发项目构建额外上下文
func (s *ProjectService) BuildEnhanceContext(ctx context.Context, projectID uuid.UUID) (*EnhanceContext, error) {
    project, err := s.repo.FindByID(ctx, projectID)
    if err != nil {
        return nil, err
    }

    var config struct {
        ExistingRepoURL string            `json:"existing_repo_url"`
        Branch          string            `json:"branch"`
        CodeStructure   *CodeStructure    `json:"code_structure"`
    }
    json.Unmarshal(project.Config, &config)

    return &EnhanceContext{
        ExistingRepoURL: config.ExistingRepoURL,
        Branch:         config.Branch,
        TechStack:      config.CodeStructure.TechStack,
        MainModules:    config.CodeStructure.MainModules,
        EntryPoints:    config.CodeStructure.EntryPoints,
        ConfigFiles:    config.CodeStructure.ConfigFiles,
    }, nil
}

type CodeStructure struct {
    TechStack    []string          // ["go", "gin", "postgresql"]
    MainModules  []ModuleInfo      // 主要模块
    EntryPoints  []string          // 入口文件
    ConfigFiles  []string          // 配置文件
    Dependencies map[string]string // 依赖版本
}

type ModuleInfo struct {
    Path        string
    Description string
    MainFiles   []string
}
```

### 3.4 项目工作台集成点

项目管理模块需要为开发工作台提供：

| 接口 | 说明 |
|------|------|
| `GET /projects/:id` | 获取项目详情，包含当前活跃的thread |
| `POST /projects/:id/threads` | 为项目创建新的开发会话 |
| `GET /projects/:id/threads` | 获取项目的历史会话列表 |

### 3.5 验收标准

- [ ] 可创建service类型项目
- [ ] 项目状态正确流转（draft → developing → testing → deployed）
- [ ] 项目列表支持分页和状态筛选
- [ ] 项目详情包含关联的会话信息
- [ ] 增量开发模式可正确克隆和分析现有仓库
- [ ] 代码结构分析结果正确保存

### 3.6 Git服务实现

```go
// internal/service/git/service.go

package git

import (
    "context"
    "os/exec"
    "path/filepath"
    "strings"
)

type Service struct {
    reposDir string  // 仓库存储目录
}

func NewService(reposDir string) *Service {
    return &Service{reposDir: reposDir}
}

// InitRepo 初始化新仓库
func (s *Service) InitRepo(ctx context.Context, projectID uuid.UUID, name string) (string, error) {
    repoPath := filepath.Join(s.reposDir, projectID.String())

    // 初始化Git仓库
    cmd := exec.CommandContext(ctx, "git", "init", repoPath)
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("git init failed: %w", err)
    }

    // 创建初始提交
    readmePath := filepath.Join(repoPath, "README.md")
    os.WriteFile(readmePath, []byte(fmt.Sprintf("# %s\n", name)), 0644)

    cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "add", ".")
    cmd.Run()

    cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "commit", "-m", "Initial commit")
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
    structure := &CodeStructure{}

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

        // 检测框架
        content, _ := os.ReadFile(filepath.Join(repoPath, "go.mod"))
        if strings.Contains(string(content), "gin-gonic") {
            stack = append(stack, "gin")
        }
        if strings.Contains(string(content), "echo") {
            stack = append(stack, "echo")
        }
    }

    // Node.js项目
    if _, err := os.Stat(filepath.Join(repoPath, "package.json")); err == nil {
        stack = append(stack, "nodejs")

        content, _ := os.ReadFile(filepath.Join(repoPath, "package.json"))
        if strings.Contains(string(content), "react") {
            stack = append(stack, "react")
        }
        if strings.Contains(string(content), "vue") {
            stack = append(stack, "vue")
        }
        if strings.Contains(string(content), "express") {
            stack = append(stack, "express")
        }
    }

    return stack
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
```

---

## 第4章：Agent引擎核心（M3）

### 4.1 功能概述

**目标**：构建Agent编排引擎，支持动态角色配置和生命周期管理。

**核心职责**：
- Agent角色的动态配置管理（数据库存储）
- 通过CLI子进程启动Agent实例
- 管理Agent生命周期
- 调度Agent按工作流执行
- 追踪Agent状态和调用深度

### 4.2 核心架构

```
┌─────────────────────────────────────────────────────────────┐
│                     AgentOrchestrator                        │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    WorkflowEngine                    │   │
│  │   Phase: 需求 → 设计 → 实现 → 审查 → 测试 → 部署     │   │
│  └─────────────────────────────────────────────────────┘   │
│                            ↓                                │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   InvocationTracker                  │   │
│  │   - 注册所有Agent调用                                 │   │
│  │   - 管理AbortController                              │   │
│  │   - 深度限制检查（≤15）                               │   │
│  └─────────────────────────────────────────────────────┘   │
│                            ↓                                │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                    ClaudeAdapter                     │   │
│  │   - spawn CLI子进程                                   │   │
│  │   - 流式输出解析                                      │   │
│  │   - 事件转换                                          │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 4.3 Go数据模型

```go
// internal/model/agent_config.go

type AgentConfig struct {
    ID           uuid.UUID       `json:"id"`
    AgentID      string          `json:"agent_id"`
    DisplayName  string          `json:"display_name"`
    Description  string          `json:"description"`
    Phase        string          `json:"phase"`
    RoutingConfig RoutingConfig  `json:"routing_config"`
    Tools        []string        `json:"tools"`
    SystemPrompt string          `json:"system_prompt"`
    Model        string          `json:"model"`
    IsActive     bool            `json:"is_active"`
    IsBuiltin    bool            `json:"is_builtin"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
}

type RoutingConfig struct {
    UseWhen []string `json:"use_when"`
    NotFor  []string `json:"not_for"`
    Output  []string `json:"output"`
}
```

### 4.4 Agent配置服务

```go
// internal/service/agent/config_service.go

type AgentConfigService struct {
    repo   repo.AgentConfigRepository
    cache  *redis.Client
}

// 获取所有启用的Agent配置
func (s *AgentConfigService) ListActive(ctx context.Context) ([]*AgentConfig, error) {
    // 优先从缓存获取
    cached, err := s.cache.Get(ctx, "agent_configs:active").Result()
    if err == nil {
        var configs []*AgentConfig
        json.Unmarshal([]byte(cached), &configs)
        return configs, nil
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

// 创建自定义Agent角色
func (s *AgentConfigService) Create(ctx context.Context, req *CreateAgentRequest) (*AgentConfig, error) {
    config := &AgentConfig{
        ID:           uuid.New(),
        AgentID:      req.AgentID,
        DisplayName:  req.DisplayName,
        Description:  req.Description,
        Phase:        req.Phase,
        RoutingConfig: req.RoutingConfig,
        Tools:        req.Tools,
        SystemPrompt: req.SystemPrompt,
        Model:        req.Model,
        IsActive:     true,
        IsBuiltin:    false,
    }

    if err := s.repo.Create(ctx, config); err != nil {
        return nil, err
    }

    // 清除缓存
    s.cache.Del(ctx, "agent_configs:active")

    return config, nil
}
```

### 4.5 Agent配置API

```
/api/v1/agents
├── GET    /              # 获取Agent列表
├── POST   /              # 创建自定义Agent
├── GET    /:id           # 获取Agent详情
├── PUT    /:id           # 更新Agent配置
└── DELETE /:id           # 删除Agent（仅自定义）
```

### 4.6 调用追踪器

**设计说明**：由于CancelFunc无法序列化到Redis，采用进程管理器模式，通过PID管理进程生命周期。

```go
// internal/service/agent/tracker.go

import (
    "context"
    "os"
    "syscall"
    "sync"
)

type InvocationTracker struct {
    redis          *redis.Client
    maxDepth       int           // 默认15
    processManager *ProcessManager
}

// Invocation 调用记录（可序列化存储到Redis）
type Invocation struct {
    ID          string    `json:"id"`
    ThreadID    string    `json:"thread_id"`
    AgentID     string    `json:"agent_id"`
    SessionID   string    `json:"session_id"`
    PID         int       `json:"pid"`           // 进程ID
    Depth       int       `json:"depth"`
    Status      string    `json:"status"`        // running, completed, cancelled
    StartedAt   time.Time `json:"started_at"`
    EndedAt     *time.Time `json:"ended_at,omitempty"`
}

// ProcessManager 进程管理器（内存中管理进程句柄）
type ProcessManager struct {
    mu       sync.RWMutex
    procs    map[string]*os.Process    // invocationID -> Process
    contexts map[string]context.CancelFunc  // invocationID -> CancelFunc
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

        // 等待3秒，如果不退出则SIGKILL
        time.Sleep(3 * time.Second)

        // 强制杀死
        proc.Kill()

        delete(pm.procs, invocationID)
        delete(pm.contexts, invocationID)
    }

    return nil
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
```

### 4.7 工作流引擎

```go
// internal/service/agent/workflow.go

type Phase string

const (
    PhaseRequirement Phase = "requirement"
    PhaseDesign      Phase = "design"
    PhaseImplement   Phase = "implement"
    PhaseReview      Phase = "review"
    PhaseTest        Phase = "test"
    PhaseDeploy      Phase = "deploy"
)

type PhaseConfig struct {
    Name       Phase
    Agent      AgentRole
    Next       Phase
    Checkpoint bool
    OnReject   Phase
}

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
```

### 4.8 验收标准

- [ ] 可通过API管理Agent配置（CRUD）
- [ ] 内置6个默认Agent角色
- [ ] 支持新增自定义Agent角色
- [ ] 自定义角色可编辑和删除，内置角色不可删除
- [ ] Agent配置变更后缓存自动刷新
- [ ] 工作流引擎动态加载Agent配置

---

## 第5章：A2A路由与MCP回传（M4）

### 5.1 功能概述

**A2A路由**：支持Agent之间通过`@Agent名`互相触发协作。

**MCP回传**：让Agent在执行过程中主动发言到聊天室，而非等任务结束。

### 5.2 A2A路由机制

#### 核心设计

| 设计项 | 规则 |
|--------|------|
| 触发方式 | 行首`@Agent名`（防止代码注释、文档中的误触发） |
| 执行模式 | Worklist串行执行 |
| 深度限制 | ≤15轮（防止无限递归） |
| 取消机制 | 共享AbortController，一键终止全链 |
| 多目标限制 | 单次最多@2个Agent（防止扇形爆炸） |

#### Worklist数据结构

```go
// internal/service/agent/worklist.go

type WorklistItem struct {
    AgentID   string
    TriggerBy string
    Input     string
    Depth     int
    AddedAt   time.Time
}

type Worklist struct {
    threadID string
    items    []WorklistItem
    maxDepth int
}

func (w *Worklist) Push(item WorklistItem) error {
    if item.Depth > w.maxDepth {
        return fmt.Errorf("depth limit exceeded")
    }

    // 去重检查
    for _, existing := range w.items {
        if existing.AgentID == item.AgentID {
            return nil
        }
    }

    w.items = append(w.items, item)
    return nil
}
```

#### @mention解析器

```go
// internal/service/agent/mention_parser.go

var mentionPattern = regexp.MustCompile(`^@([\w-]+)\s*(.*)`)

func ParseMentions(content string, excludeSelf string) []Mention {
    var mentions []Mention

    lines := strings.Split(content, "\n")
    for _, line := range lines {
        matches := mentionPattern.FindStringSubmatch(strings.TrimSpace(line))
        if len(matches) >= 2 {
            agentID := matches[1]
            if agentID == excludeSelf {
                continue
            }
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
```

### 5.3 MCP回传机制

#### MCP工具定义

| 工具 | 功能 | 使用场景 |
|------|------|----------|
| post_message | 发送消息到项目聊天室 | 汇报进度、@其他Agent |
| thread_context | 获取对话上下文 | 理解历史讨论 |
| update_task | 更新任务状态 | 标记完成、失败 |
| request_permission | 请求危险操作授权 | 删除文件、执行敏感命令 |

#### MCP API路由

```
/api/v1/mcp
├── POST /post_message        # 发送消息到聊天室
├── GET  /thread_context      # 获取对话上下文
├── POST /update_task         # 更新任务状态
└── POST /request_permission  # 请求危险操作授权
```

### 5.4 MCP认证机制

**双UUID认证**：使用invocationId + callbackToken确保只有正在运行的Agent才能调用MCP接口。

```go
// internal/service/agent/mcp_auth.go

import (
    "crypto/rand"
    "encoding/hex"
)

type MCPAuthService struct {
    redis *redis.Client
}

// TokenPair 认证令牌对
type TokenPair struct {
    InvocationID  string
    CallbackToken string
}

// GenerateToken 生成认证令牌
func (s *MCPAuthService) GenerateToken(ctx context.Context, invocationID string) (*TokenPair, error) {
    // 生成随机callback token
    tokenBytes := make([]byte, 32)
    if _, err := rand.Read(tokenBytes); err != nil {
        return nil, err
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

**认证中间件**：

```go
// internal/api/middleware/mcp_auth.go

func MCPAuthMiddleware(authService *MCPAuthService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 从Header获取认证信息
        invocationID := c.GetHeader("X-Invocation-ID")
        callbackToken := c.GetHeader("X-Callback-Token")

        if invocationID == "" || callbackToken == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authentication headers"})
            c.Abort()
            return
        }

        // 验证令牌
        threadID, err := authService.ValidateToken(c.Request.Context(), invocationID, callbackToken)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
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

**Agent进程环境变量注入**：

```go
// 在启动Agent时注入认证信息
func (r *AgentRouter) startAgent(ctx context.Context, opts AgentStartOptions) (*os.Process, error) {
    // 生成认证令牌
    tokenPair, err := r.mcpAuthService.GenerateToken(ctx, opts.InvocationID)
    if err != nil {
        return nil, err
    }

    // 构建环境变量
    env := append(os.Environ(),
        fmt.Sprintf("ISDP_THREAD_ID=%s", opts.ThreadID),
        fmt.Sprintf("ISDP_INVOCATION_ID=%s", tokenPair.InvocationID),
        fmt.Sprintf("ISDP_CALLBACK_TOKEN=%s", tokenPair.CallbackToken),
        fmt.Sprintf("ISDP_MCP_BASE_URL=%s", r.config.MCPBaseURL),
    )

    cmd := exec.CommandContext(ctx, r.claudePath, args...)
    cmd.Env = env

    // ... 启动进程
}
```

### 5.5 验收标准

- [ ] @mention只匹配行首，不误触发
- [ ] A2A深度限制正确生效（≤15轮）
- [ ] 单次最多@2个Agent
- [ ] 取消信号可传播到所有Agent进程
- [ ] Agent可通过MCP主动发言

---

## 第6章：ClaudeCode集成（M5）

### 6.1 功能概述

**目标**：将Claude Code CLI作为核心代码生成工具集成到ISDP平台。

### 6.2 Claude Code CLI基础

```bash
# 安装
npm install -g @anthropic-ai/claude-code

# 基础调用
claude -p "帮我写一个API" --output-format stream-json --verbose

# 恢复会话
claude -p "继续" --resume session-xxx
```

### 6.3 适配器核心实现

```go
// pkg/claude/adapter.go

type Adapter struct {
    claudePath string
    workDir    string
    model      string
    timeout    time.Duration
}

type InvokeOptions struct {
    Prompt        string
    SessionID     string
    Model         string
    AllowedTools  []string
    PermissionMode string
    ThreadID      string
    InvocationID  string
    CallbackToken string
    WorkDir       string
}

type StreamEvent struct {
    Type        string
    Content     string
    ToolName    string
    ToolInput   json.RawMessage
    SessionID   string
    InputTokens  int
    OutputTokens int
    Error       string
}

func (a *Adapter) Invoke(ctx context.Context, opts InvokeOptions) (<-chan StreamEvent, error) {
    args := a.buildArgs(opts)
    cmd := exec.CommandContext(ctx, a.claudePath, args...)
    cmd.Env = a.buildEnv(opts)

    stdout, _ := cmd.StdoutPipe()
    cmd.Start()

    eventChan := make(chan StreamEvent, 100)
    go a.processStream(stdout, eventChan)

    return eventChan, nil
}
```

### 6.4 工具权限配置

```go
var ToolPresets = map[string][]string{
    "readonly": {"Read", "Glob", "Grep"},
    "write":    {"Read", "Write", "Edit", "Glob", "Grep"},
    "full":     {"Read", "Write", "Edit", "Glob", "Grep", "Bash"},
}
```

### 6.5 上下文注入

```go
// internal/service/agent/context_builder.go

import (
    "context"
    "strings"
)

type ContextBuilder struct {
    msgRepo      repo.MessageRepository
    artifactRepo repo.ArtifactRepository
    projectRepo  repo.ProjectRepository
    agentRepo    repo.AgentConfigRepository
    maxLines     int  // 默认400行
}

type ContextConfig struct {
    ThreadID   string
    AgentID    string
    Phase      string
    ProjectMode string  // new 或 enhance
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

## 技术约束
[技术约束说明]
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

### API设计规范
- RESTful风格
- URL使用kebab-case
- 响应格式: {"code": 0, "data": {}, "message": ""}
- 错误码定义

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
- 使用goimports管理import

### 目录结构
` + "```" + `
cmd/
  └── server/      # 入口
internal/
  ├── api/         # HTTP处理
  ├── service/     # 业务逻辑
  ├── model/       # 数据模型
  └── repo/        # 数据访问
pkg/               # 公共包
` + "```" + `

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

### 输出格式
` + "```markdown" + `
## Review报告

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
` + "```" + `

### 放行条件
必须明确说"可以放行"或"P1/P2清零"才算通过

`)

    case "test":
        sb.WriteString(`## 测试验证阶段

### 你的任务
1. 执行单元测试
2. 执行集成测试
3. 生成测试报告

### 测试类型
- 单元测试：测试单个函数/方法
- 集成测试：测试模块间交互
- 端到端测试：测试完整流程

### 测试报告格式
` + "```markdown" + `
## 测试报告

### 测试概览
- 总用例数: X
- 通过: X
- 失败: X
- 跳过: X
- 覆盖率: X%

### 失败用例
1. TestXXX - 失败原因

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

### Docker命令
` + "```bash" + `
# 构建镜像
docker build -t isdp-project .

# 运行容器
docker run -d -p 8080:8080 isdp-project

# 查看日志
docker logs <container_id>
` + "```" + `

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
  - [ ] AC2: [具体验收条件]

## 非功能需求
- **性能**: [响应时间、并发量等]
- **安全**: [认证、授权、数据保护等]
- **兼容性**: [浏览器、操作系统等]

## 技术约束
- [技术栈限制]
- [集成要求]
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

### P3（共X个）— 可登记Backlog
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
- [ ] 大对象：是否频繁创建大对象？
- [ ] 连接泄露：是否正确关闭连接？

### 代码质量
- [ ] 错误处理：是否处理所有错误？
- [ ] 边界检查：是否检查数组越界？
- [ ] 空指针：是否检查nil？

`)
    }

    return sb.String()
}

// buildLayer3 需求注入（从数据库加载）
func (b *ContextBuilder) buildLayer3(ctx context.Context, config ContextConfig) (string, error) {
    var sb strings.Builder

    // 获取项目信息
    project, err := b.projectRepo.GetByThreadID(ctx, config.ThreadID)
    if err == nil {
        sb.WriteString("\n# 项目信息\n\n")
        sb.WriteString(fmt.Sprintf("- **项目名称**: %s\n", project.Name))
        sb.WriteString(fmt.Sprintf("- **项目类型**: %s\n", project.Type))
        sb.WriteString(fmt.Sprintf("- **开发模式**: %s\n", project.Mode))

        // 增量开发模式：注入现有代码结构
        if project.Mode == "enhance" {
            var config struct {
                CodeStructure *CodeStructure `json:"code_structure"`
            }
            json.Unmarshal(project.Config, &config)
            if config.CodeStructure != nil {
                sb.WriteString("\n## 现有代码结构\n\n")
                sb.WriteString(fmt.Sprintf("- **技术栈**: %s\n", strings.Join(config.CodeStructure.TechStack, ", ")))
                if len(config.CodeStructure.MainModules) > 0 {
                    sb.WriteString("- **主要模块**:\n")
                    for _, mod := range config.CodeStructure.MainModules {
                        sb.WriteString(fmt.Sprintf("  - %s: %s\n", mod.Path, mod.Description))
                    }
                }
                if len(config.CodeStructure.EntryPoints) > 0 {
                    sb.WriteString(fmt.Sprintf("- **入口文件**: %s\n", strings.Join(config.CodeStructure.EntryPoints, ", ")))
                }
            }
        }
    }

    // 获取需求文档
    requirement, err := b.artifactRepo.GetByType(ctx, config.ThreadID, "requirement_doc")
    if err == nil {
        sb.WriteString("\n# 原始需求\n\n")
        // 提取≤5行关键摘要
        summary := extractSummary(requirement.Content, 5)
        sb.WriteString(summary)
        sb.WriteString("\n")
    }

    // 获取最近的消息历史
    messages, err := b.msgRepo.GetRecent(ctx, config.ThreadID, 10)
    if err == nil && len(messages) > 0 {
        sb.WriteString("\n# 最近对话\n\n")
        for _, msg := range messages {
            if msg.Role == "user" {
                sb.WriteString(fmt.Sprintf("**用户**: %s\n\n", truncate(msg.Content, 200)))
            } else if msg.Role == "agent" {
                sb.WriteString(fmt.Sprintf("**%s**: %s\n\n", msg.AgentID, truncate(msg.Content, 200)))
            }
        }
    }

    return sb.String(), nil
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

func countLines(s string) int {
    return strings.Count(s, "\n") + 1
}

func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}
```

### 6.6 验收标准

- [ ] Claude CLI可正确启动和执行
- [ ] 流式输出可正确解析为事件
- [ ] 会话可恢复（--resume）
- [ ] 工具权限正确限制
- [ ] 错误可正确处理和重试

---

## 第7章：沙箱环境（M6）

### 7.1 功能概述

**目标**：提供隔离的Docker容器环境，用于安全运行和验证生成的代码。

### 7.2 Docker客户端封装

```go
// pkg/docker/client.go

type DockerClient struct {
    cli *client.Client
}

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

func (d *DockerClient) CreateContainer(ctx context.Context, config ContainerConfig) (string, error)
func (d *DockerClient) StartContainer(ctx context.Context, containerID string) error
func (d *DockerClient) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error
func (d *DockerClient) GetContainerLogs(ctx context.Context, containerID string, opts LogOptions) (io.ReadCloser, error)
func (d *DockerClient) ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, error)
```

### 7.3 沙箱服务实现

```go
// internal/service/sandbox/service.go

type SandboxService struct {
    docker     *docker.DockerClient
    repo       repo.SandboxRepository
    portPool   *PortPool
}

func (s *SandboxService) CreateSandbox(ctx context.Context, projectID uuid.UUID, config SandboxConfig) (*Sandbox, error)
func (s *SandboxService) StartSandbox(ctx context.Context, sandboxID uuid.UUID) error
func (s *SandboxService) StopSandbox(ctx context.Context, sandboxID uuid.UUID) error
func (s *SandboxService) GetLogs(ctx context.Context, sandboxID uuid.UUID, follow bool) (io.ReadCloser, error)
func (s *SandboxService) ExecuteCommand(ctx context.Context, sandboxID uuid.UUID, cmd []string) (string, error)
```

### 7.4 安全配置

```go
const (
    DefaultCPULimit    = 2
    DefaultMemoryLimit = 4096  // MB
    DefaultPidsLimit   = 256
    PortRangeStart     = 30000
    PortRangeEnd       = 40000
)

func DefaultSecurityConfig() SecurityConfig {
    return SecurityConfig{
        RunAsNonRoot:           true,
        CapDrop:               []string{"ALL"},
        NoNewPrivileges:       true,
        PidsLimit:             DefaultPidsLimit,
        DisableInterContainer: true,
    }
}
```

### 7.5 API设计

```
/api/v1/sandbox
├── POST /                        # 创建沙箱
├── GET  /:id                     # 获取沙箱详情
├── POST /:id/start               # 启动沙箱
├── POST /:id/stop                # 停止沙箱
├── GET  /:id/logs                # 获取日志
├── POST /:id/exec                # 执行命令
└── DELETE /:id                   # 删除沙箱
```

### 7.6 验收标准

- [ ] 可创建和启动Docker容器
- [ ] 容器资源限制正确生效
- [ ] 端口自动分配不冲突
- [ ] 可获取容器日志
- [ ] 安全限制正确应用

---

## 第8章：协作规则与上下文工程（M7）

### 8.1 协作规则定义

#### 交接五件套

```markdown
## 交接报告

### What - 我做了什么
[具体修改内容]

### Why - 为什么这样做
[决策原因，最重要]

### Tradeoff - 放弃了什么方案
[备选方案说明]

### Open Questions - 不确定的点
[需要澄清的问题]

### Next Action - 希望对方做什么
[@目标Agent] [具体请求]
```

#### Review分级标准

| 级别 | 定义 | 处理方式 |
|------|------|----------|
| P1 | 阻断级：功能错误/安全问题 | 必须立即修 |
| P2 | 重要级：质量/测试问题 | 必须修完才能放行 |
| P3 | 建议级：风格/命名 | 可登记Backlog |

### 8.2 Review报告解析

```go
func ParseReviewReport(content string) (*ReviewReport, error) {
    report := &ReviewReport{}
    report.P1Issues = parseIssues(content, "P1")
    report.P2Issues = parseIssues(content, "P2")
    report.P3Issues = parseIssues(content, "P3")
    report.Approved = isApproved(content)
    return report, nil
}

func (r *ReviewReport) CanProceedToMerge() bool {
    return r.Approved && len(r.P1Issues) == 0 && len(r.P2Issues) == 0
}
```

### 8.2.1 合入门禁实现

```go
// internal/service/agent/merge_gate.go

package agent

import (
    "context"
    "fmt"
)

type MergeGateService struct {
    reviewRepo     repo.ReviewReportRepository
    testRepo       repo.TestReportRepository
    checkpointRepo repo.CheckpointRepository
}

type MergeGateStatus struct {
    CanMerge      bool
    Checks        []GateCheck
    Blockers      []string
}

type GateCheck struct {
    Name        string
    Status      string  // passed, failed, pending
    Description string
}

// CheckMergeGate 检查是否满足合入条件
func (s *MergeGateService) CheckMergeGate(ctx context.Context, threadID string) (*MergeGateStatus, error) {
    status := &MergeGateStatus{
        CanMerge: true,
    }

    // 检查1: P1/P2清零
    review, err := s.reviewRepo.GetLatest(ctx, threadID)
    if err != nil {
        status.CanMerge = false
        status.Blockers = append(status.Blockers, "未找到审查报告")
        status.Checks = append(status.Checks, GateCheck{
            Name:   "审查报告",
            Status: "failed",
        })
    } else {
        p1Count := len(review.P1Issues)
        p2Count := len(review.P2Issues)

        if p1Count > 0 || p2Count > 0 {
            status.CanMerge = false
            status.Blockers = append(status.Blockers, fmt.Sprintf("P1/P2未清零: P1=%d, P2=%d", p1Count, p2Count))
            status.Checks = append(status.Checks, GateCheck{
                Name:        "P1/P2清零",
                Status:      "failed",
                Description: fmt.Sprintf("P1: %d, P2: %d", p1Count, p2Count),
            })
        } else {
            status.Checks = append(status.Checks, GateCheck{
                Name:   "P1/P2清零",
                Status: "passed",
            })
        }

        // 检查2: 审查员放行
        if !review.Approved {
            status.CanMerge = false
            status.Blockers = append(status.Blockers, "审查员未放行")
            status.Checks = append(status.Checks, GateCheck{
                Name:   "审查员放行",
                Status: "failed",
            })
        } else {
            status.Checks = append(status.Checks, GateCheck{
                Name:   "审查员放行",
                Status: "passed",
            })
        }
    }

    // 检查3: 测试全部通过
    testReport, err := s.testRepo.GetLatest(ctx, threadID)
    if err != nil {
        status.Checks = append(status.Checks, GateCheck{
            Name:   "测试通过",
            Status: "pending",
        })
    } else {
        if testReport.FailedCount > 0 {
            status.CanMerge = false
            status.Blockers = append(status.Blockers, fmt.Sprintf("存在%d个失败测试", testReport.FailedCount))
            status.Checks = append(status.Checks, GateCheck{
                Name:        "测试通过",
                Status:      "failed",
                Description: fmt.Sprintf("失败: %d", testReport.FailedCount),
            })
        } else {
            status.Checks = append(status.Checks, GateCheck{
                Name:   "测试通过",
                Status: "passed",
            })
        }
    }

    // 检查4: 用户确认（如需要）
    checkpoint, err := s.checkpointRepo.GetByType(ctx, threadID, "merge")
    if err == nil {
        if checkpoint.Status != "approved" {
            status.CanMerge = false
            status.Blockers = append(status.Blockers, "需要用户确认合入")
            status.Checks = append(status.Checks, GateCheck{
                Name:   "用户确认",
                Status: "pending",
            })
        } else {
            status.Checks = append(status.Checks, GateCheck{
                Name:   "用户确认",
                Status: "passed",
            })
        }
    }

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
        checkpoint := &Checkpoint{
            ID:        uuid.New(),
            ThreadID:  threadID,
            Type:      "merge",
            Status:    "pending",
            CreatedAt: time.Now(),
        }
        return s.checkpointRepo.Create(ctx, checkpoint)
    }

    return fmt.Errorf("不满足合入条件: %s", strings.Join(status.Blockers, "; "))
}
```

**集成到工作流引擎**：

```go
// 在PhaseReview阶段完成后检查
func (e *WorkflowEngine) afterReview(ctx context.Context, threadID string) error {
    // 检查合入门禁
    status, err := e.mergeGateSvc.CheckMergeGate(ctx, threadID)
    if err != nil {
        return err
    }

    if !status.CanMerge {
        // 返回消息给用户，说明为什么不能继续
        return fmt.Errorf("合入检查未通过: %s", strings.Join(status.Blockers, "; "))
    }

    // 请求用户确认
    return e.mergeGateSvc.RequestMergeApproval(ctx, threadID)
}
```

### 8.3 上下文工程

#### 四层信息架构

| 层级 | 内容 | 加载时机 | 行数限制 |
|------|------|----------|----------|
| Layer 0 | 身份+价值观+铁律 | 始终 | ≤100行 |
| Layer 1 | 工作流+参考 | 按需 | ≤150行 |
| Layer 2 | 模板+清单 | 引用时 | 不限 |
| Layer 3 | 需求注入 | 任务创建时 | 自动 |

### 8.4 冷启动验证器

```go
type ColdStartVerifier struct {
    adapter *claude.Adapter
}

func (v *ColdStartVerifier) Verify(ctx context.Context, req VerificationRequest) (*VerificationResult, error) {
    // 启动全新Agent session，只给需求+交付物
    // 让它独立判断是否符合
}
```

### 8.5 验收标准

- [ ] 交接报告模板正确生成
- [ ] Review报告可正确解析P1/P2/P3
- [ ] 可判断Review是否放行
- [ ] 上下文按四层架构正确构建
- [ ] 冷启动验证器可独立运行

---

## 第9章：Web前端开发（M8）

### 9.1 技术栈

- React 18 + TypeScript
- Ant Design 5
- Zustand（状态管理）
- Axios + WebSocket

### 9.2 项目结构

```
web/src/
├── api/                    # API接口
├── components/             # 通用组件
│   ├── Layout/
│   ├── MessageList/
│   ├── AgentProgress/
│   └── ArtifactCard/
├── pages/                  # 页面
│   ├── Dashboard/
│   ├── ProjectList/
│   ├── ProjectDetail/
│   ├── Workspace/          # 开发工作台
│   └── Settings/
├── stores/                 # 状态管理
├── hooks/                  # 自定义Hooks
└── types/                  # 类型定义
```

### 9.3 核心页面

#### 开发工作台

```typescript
// src/pages/Workspace/index.tsx

export function Workspace() {
  const { messages, currentPhase, currentAgent, sendMessage, pauseThread } = useThreadStore();

  return (
    <div className="workspace">
      {/* 顶部进度条 */}
      <AgentProgress
        phases={['requirement', 'design', 'implement', 'review', 'test', 'deploy']}
        currentPhase={currentPhase}
        currentAgent={currentAgent}
      />

      {/* 消息区域 */}
      <div className="message-area">
        <MessageList messages={messages} />
      </div>

      {/* 底部输入区 */}
      <div className="input-area">
        <TextArea placeholder="输入消息或指令..." />
        <Button type="primary" onClick={handleSend}>发送</Button>
      </div>

      {/* 侧边产物面板 */}
      <ArtifactPanel artifacts={artifacts} />
    </div>
  );
}
```

### 9.4 WebSocket协议规范

#### 消息格式

所有WebSocket消息使用JSON格式，包含统一的消息信封：

```typescript
interface WSMessage {
  type: string;           // 消息类型
  threadId: string;       // 会话ID
  timestamp: number;      // 时间戳
  payload: any;           // 消息内容
}
```

#### 消息类型定义

| 类型 | 方向 | 说明 |
|------|------|------|
| subscribe | C→S | 订阅会话消息 |
| unsubscribe | C→S | 取消订阅 |
| ping | C→S | 心跳请求 |
| pong | S→C | 心跳响应 |
| agent_message | S→C | Agent消息 |
| agent_start | S→C | Agent开始工作 |
| agent_end | S→C | Agent工作结束 |
| phase_change | S→C | 阶段变化 |
| artifact_created | S→C | 新产物生成 |
| error | S→C | 错误消息 |

#### 客户端消息

```typescript
// 订阅会话
{
  "type": "subscribe",
  "threadId": "thread-uuid",
  "timestamp": 1710123456789
}

// 取消订阅
{
  "type": "unsubscribe",
  "threadId": "thread-uuid",
  "timestamp": 1710123456789
}

// 心跳
{
  "type": "ping",
  "timestamp": 1710123456789
}
```

#### 服务端消息

```typescript
// Agent消息
{
  "type": "agent_message",
  "threadId": "thread-uuid",
  "timestamp": 1710123456789,
  "payload": {
    "messageId": "msg-uuid",
    "agentId": "developer",
    "agentName": "开发者",
    "content": "正在编写代码..."
  }
}

// Agent开始工作
{
  "type": "agent_start",
  "threadId": "thread-uuid",
  "timestamp": 1710123456789,
  "payload": {
    "agentId": "developer",
    "agentName": "开发者",
    "phase": "implement"
  }
}

// 阶段变化
{
  "type": "phase_change",
  "threadId": "thread-uuid",
  "timestamp": 1710123456789,
  "payload": {
    "fromPhase": "design",
    "toPhase": "implement",
    "fromAgent": "architect",
    "toAgent": "developer"
  }
}

// 新产物
{
  "type": "artifact_created",
  "threadId": "thread-uuid",
  "timestamp": 1710123456789,
  "payload": {
    "artifactId": "artifact-uuid",
    "type": "requirement_doc",
    "name": "需求文档.md",
    "preview": "前100字符预览..."
  }
}

// 心跳响应
{
  "type": "pong",
  "timestamp": 1710123456789
}

// 错误消息
{
  "type": "error",
  "threadId": "thread-uuid",
  "timestamp": 1710123456789,
  "payload": {
    "code": 30002,
    "message": "Agent深度超限"
  }
}
```

#### 后端WebSocket处理

```go
// internal/ws/handler.go

type WSHandler struct {
    clients    map[string]map[*websocket.Conn]bool  // threadId -> connections
    broadcast  chan BroadcastMessage
    register   chan *Client
    unregister chan *Client
}

type Client struct {
    conn     *websocket.Conn
    threadId string
}

type BroadcastMessage struct {
    ThreadId string
    Message  WSMessage
}

func (h *WSHandler) Run() {
    for {
        select {
        case client := <-h.register:
            if h.clients[client.threadId] == nil {
                h.clients[client.threadId] = make(map[*websocket.Conn]bool)
            }
            h.clients[client.threadId][client.conn] = true

        case client := <-h.unregister:
            if conns, ok := h.clients[client.threadId]; ok {
                if _, ok := conns[client.conn]; ok {
                    delete(conns, client.conn)
                    client.conn.Close()
                }
            }

        case msg := <-h.broadcast:
            if conns, ok := h.clients[msg.ThreadId]; ok {
                for conn := range conns {
                    err := conn.WriteJSON(msg.Message)
                    if err != nil {
                        conn.Close()
                        delete(conns, conn)
                    }
                }
            }
        }
    }
}

func (h *WSHandler) HandleConnection(c *gin.Context) {
    threadId := c.Query("thread_id")
    if threadId == "" {
        c.JSON(400, gin.H{"error": "thread_id required"})
        return
    }

    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }

    client := &Client{conn: conn, threadId: threadId}
    h.register <- client

    defer func() {
        h.unregister <- client
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
            // 心跳响应，忽略
        case "subscribe":
            // 已在连接时处理
        case "unsubscribe":
            h.unregister <- client
            return
        }
    }
}

// BroadcastToThread 向特定会话广播消息
func (h *WSHandler) BroadcastToThread(threadId string, msg WSMessage) {
    h.broadcast <- BroadcastMessage{
        ThreadId: threadId,
        Message:  msg,
    }
}
```

#### 前端实现

```typescript
// src/hooks/useWebSocket.ts

interface WSMessage {
  type: string;
  threadId: string;
  timestamp: number;
  payload?: any;
}

export function useWebSocket(baseUrl: string) {
  const wsRef = useRef<WebSocket | null>(null);
  const handlersRef = useRef<Map<string, Set<(data: any) => void>>>(new Map());
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>();
  const [connected, setConnected] = useState(false);

  const connect = useCallback(() => {
    const ws = new WebSocket(baseUrl);

    ws.onopen = () => {
      setConnected(true);
      // 重连后重新订阅
      handlersRef.current.forEach((_, threadId) => {
        ws.send(JSON.stringify({ type: 'subscribe', threadId }));
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
      wsRef.current?.send(JSON.stringify({ type: 'subscribe', threadId }));
    }
    handlersRef.current.get(threadId)!.add(handler);

    return () => {
      handlersRef.current.get(threadId)?.delete(handler);
      if (handlersRef.current.get(threadId)?.size === 0) {
        handlersRef.current.delete(threadId);
        wsRef.current?.send(JSON.stringify({ type: 'unsubscribe', threadId }));
      }
    };
  }, []);

  return { connected, subscribe };
}
```

### 9.5 验收标准

- [ ] 项目列表页面正确展示
- [ ] 开发工作台消息实时更新
- [ ] Agent进度条正确显示当前阶段
- [ ] WebSocket连接稳定，可重连
- [ ] Agent配置页面可新增/编辑/删除

---

## 第10章：集成测试与部署（M9）

### 10.1 测试策略

```
                    ┌─────────┐
                    │ E2E测试  │  少量，验证关键流程
                  ┌─┴─────────┴─┐
                  │  集成测试    │  中量，验证模块交互
                ┌─┴─────────────┴─┐
                │     单元测试      │  大量，验证函数逻辑
              └─┴───────────────────┴─┘
```

### 10.2 单元测试示例

```go
func TestParseMentions_Success(t *testing.T) {
    tests := []struct {
        name      string
        content   string
        exclude   string
        expected  []Mention
    }{
        {
            name:     "single mention",
            content:  "@architect 请帮我review",
            exclude:  "developer",
            expected: []Mention{{AgentID: "architect", Content: "请帮我review"}},
        },
        {
            name:     "not at line start",
            content:  "看这个 @architect",
            exclude:  "",
            expected: []Mention{},  // 不在行首，不触发
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ParseMentions(tt.content, tt.exclude)
            assert.Equal(t, len(tt.expected), len(result))
        })
    }
}
```

### 10.3 Docker Compose部署

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: isdp
      POSTGRES_PASSWORD: isdp123
      POSTGRES_DB: isdp
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7
    volumes:
      - redis_data:/data

  backend:
    build: ./backend
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis

  frontend:
    build: ./web
    ports:
      - "80:80"
    depends_on:
      - backend

volumes:
  postgres_data:
  redis_data:
```

### 10.4 监控指标

```go
var (
    HttpRequestsTotal = promauto.NewCounterVec(...)
    AgentInvocationsTotal = promauto.NewCounterVec(...)
    SandboxContainersTotal = promauto.NewGauge(...)
    WebSocketConnections = promauto.NewGauge(...)
)
```

### 10.5 验收标准

- [ ] 单元测试覆盖率 > 70%
- [ ] 集成测试通过
- [ ] Docker镜像构建成功
- [ ] 健康检查端点正常
- [ ] Prometheus指标可访问

---

## 附录

### A：配置文件规范

```yaml
# configs/config.yaml

server:
  port: 8080
  mode: release  # debug, release

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
  default_memory_limit: 4096  # MB
  network: isdp-network
  repos_dir: ${REPOS_DIR:/var/lib/isdp/repos}

agent:
  max_depth: 15
  max_retries: 3
  context_max_lines: 400

logging:
  level: info  # debug, info, warn, error
  format: json  # json, text

mcp:
  base_url: ${MCP_BASE_URL:http://localhost:8080/api/v1/mcp}
  token_ttl: 30m
```

### B：API完整清单

```
/api/v1
├── /projects
│   ├── GET    /                    # 项目列表
│   ├── POST   /                    # 创建项目
│   ├── GET    /:id                 # 项目详情
│   └── POST   /:id/threads         # 创建会话
│
├── /threads
│   ├── GET    /:id/messages        # 消息列表
│   ├── POST   /:id/messages        # 发送消息
│   ├── POST   /:id/pause           # 暂停会话
│   └── POST   /:id/cancel          # 取消会话
│
├── /agents
│   ├── GET    /                    # Agent列表
│   ├── POST   /                    # 创建Agent
│   └── DELETE /:id                 # 删除Agent
│
├── /sandbox
│   ├── POST   /                    # 创建沙箱
│   ├── POST   /:id/start           # 启动
│   ├── POST   /:id/stop            # 停止
│   └── GET    /:id/logs            # 日志
│
├── /mcp
│   ├── POST   /post_message        # 发送消息
│   └── POST   /request_permission  # 请求授权
│
└── /ws                           # WebSocket
```

### C：错误码定义

| 错误码 | 说明 |
|--------|------|
| 10001 | 项目不存在 |
| 10002 | 项目创建失败 |
| 20001 | 会话不存在 |
| 30001 | Agent不存在 |
| 30002 | Agent深度超限 |
| 40001 | 沙箱不存在 |
| 50001 | Claude调用失败 |

---

**文档版本历史**

| 版本 | 日期 | 修改内容 |
|------|------|----------|
| 1.0 | 2026-03-12 | 初始版本，基于PRD v0.3生成完整开发指导 |
| 1.1 | 2026-03-12 | 修复审查问题：添加Enhance模式实现、Git服务、MCP认证、WebSocket协议、完整上下文构建器、合并门禁、取消流程修复 |
| 1.2 | 2026-03-12 | 添加Layer 2模板引用实现，完善四层上下文架构 |
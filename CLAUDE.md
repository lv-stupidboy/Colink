# CLAUDE.md

Colink - 多智能体协作平台，多 Agent 协同开发系统。

## 常用命令

```bash
# 后端
go run ./cmd/server       # 启动后端服务
make build                # 构建 bin/isdp-server.exe

# 前端
cd web && npm run dev     # 启动开发服务器
cd web && npm run build   # 构建生产版本

# 完整构建（发布）
cd installer && ./build.ps1    # Windows
cd installer && ./build.sh     # Unix/Linux/macOS
```

## 启动原则

### 端口配置（灵活查找）
- **后端端口**：查看 `data/configs/config.yaml` 中 `server.port`（默认 26305）
- **前端端口**：查看 `data/configs/config.yaml` 中 `web.port`（默认 26306）
- **前端代理目标**：查看 `data/configs/config.yaml` 中 `web.api_url`

### 启动顺序
**先启动后端，再启动前端**（前端代理依赖后端）

```bash
# 1. 启动后端（项目根目录）
go run ./cmd/server

# 2. 启动前端
cd web && npm run dev
```

### 配置查找顺序（后端）
命令行 `-config` → 环境变量 `ISDP_CONFIG` → `data/configs/config.yaml` → `configs/config.yaml`

### 前端代理规则
- `/api/*` 代理到后端地址（查看 `configs/config.yaml.example` 中 `web.api_url`）
- WebSocket 代理已启用（`ws: true`）

### 注意事项
- 前端 `strictPort: true`：端口冲突时报错，不会自动切换端口
- 后端构建会自动生成插件注册（`make genplugins`）
- 前端构建会先执行 TypeScript 检查（`tsc && vite build`）

## 架构

```
Web前端 (React + Ant Design + Zustand)
        ↓ REST/WebSocket
后端服务 (Go + Gin)
  - internal/api/       HTTP 处理器
  - internal/service/   业务逻辑（agent/a2a/sandbox/workflow）
  - internal/repo/      数据访问层
  - internal/model/     数据模型
        ↓ CLI Spawn
Agent 实例 (Claude CLI / OpenCode)
```

### 基础Agent插件化架构

支持多种CLI工具（Claude Code、OpenCode等），通过插件机制扩展：

```
internal/service/agent/plugins/
├── all/all.go              # 自动导入所有插件（genplugins生成）
├── claude_code/            # Claude CLI 适配器
│   ├── plugin.go           # init()注册插件
│   └── adapter.go          # 实现AgentAdapter接口
└── open_code/              # OpenCode CLI 适配器
    ├── plugin.go
    └── adapter.go
```

**新增插件步骤：**
1. 在 `plugins/` 下创建新目录
2. 实现 `AgentAdapter` 接口（Execute、ParseOutput等）
3. 在 `plugin.go` 中调用 `RegisterPlugin()` 注册
4. 运行 `make genplugins` 更新 `all/all.go`

## 关键约束

### API 字段命名
- **JSON 字段统一使用 camelCase**（与前端保持一致）
- 后端 Go 结构体 JSON tag：`json:"baseAgentId"` 而非 `json:"base_agent_id"`

### 前端深色模式适配
- **使用 CSS 变量**：颜色值使用 `var(--color-primary)` 等变量，禁止硬编码颜色
- **变量定义**：`web/src/themes/themeVariables.css`
- **主题切换**：通过 `data-theme` 属性切换，深色主题为 `dark`
- **新增组件时**：确保在 `[data-theme='dark']` 下样式正常

```css
/* ✅ 正确：使用 CSS 变量 */
background: var(--bg-container);
color: var(--text-primary);
border: 1px solid var(--border-color);

/* ❌ 错误：硬编码颜色 */
background: #ffffff;
color: #333333;
```

### 配置文件
- `configs/config.yaml.example` - 配置模板（提交）
- `configs/config.yaml` - 本地配置，含敏感信息（不提交）

### 数据库变更

迁移脚本位于 `sql-change/` 目录，按版本号组织：

```
sql-change/
├── v1.1.0/sqlite/00001_init.sql    # goose版本: 1
├── v1.2.0/sqlite/00002_xxx.sql     # goose版本: 2
└── v1.2.0/mysql/00002_xxx.sql
```

**Goose版本号规则（重要）：**
- 文件序号 `{序号}_xxx.sql` 是**全局递增**的，不按版本目录隔离
- goose 根据文件名中的序号判断迁移顺序
- 不同版本目录下的文件序号不能重复（否则会跳过执行）

**管理规则：**
- 新版本发布时创建 `v{版本号}` 目录
- 目录下按数据库类型分 `mysql/` 和 `sqlite/`
- 文件序号必须全局递增（00001, 00002, 00003...）
- 必须包含 `-- +goose Up` 和 `-- +goose Down` 注释

**执行方式：**
- 安装器统一调用 `migrate up` 命令
- goose 自动处理首次安装和版本升级

**migrate 工具使用：**

```bash
# 构建工具
go build -o bin/migrate.exe ./cmd/migrate

# 查看当前版本
bin/migrate.exe version --db ./data/sqlite/colink.db

# 查看迁移状态
bin/migrate.exe status --db ./data/sqlite/colink.db --version 1.1.0

# 执行版本迁移（自动执行未执行的脚本，记录版本）
bin/migrate.exe up --db ./data/sqlite/colink.db --version 1.1.0

# 执行单个 SQL 文件（不记录版本，用于开发协作）
bin/migrate.exe run --db ./data/sqlite/colink.db --file sql-change/v1.1.0/sqlite/xxx.sql

# 预览模式（不执行）
bin/migrate.exe up --db ./data/sqlite/colink.db --version 1.1.0 --dry-run
bin/migrate.exe run --db ./data/sqlite/colink.db --file xxx.sql --dry-run

# 执行前备份
bin/migrate.exe up --db ./data/sqlite/colink.db --version 1.1.0 --backup
bin/migrate.exe run --db ./data/sqlite/colink.db --file xxx.sql --backup

# JSON 输出（用于脚本集成）
bin/migrate.exe up --db ./data/sqlite/colink.db --version 1.1.0 --json
```

**命令对比：**
| 命令 | 执行范围 | 版本记录 | 适用场景 |
|------|----------|----------|----------|
| `up` | 版本目录下所有未执行的脚本 | 记录 goose 版本 | 正式迁移、安装器自动执行 |
| `run` | 单个指定文件 | 不记录版本 | 开发协作、手动执行其他人的变更 |

**开发协作流程：**
1. 拉取代码后，检查 `sql-change/` 目录是否有新增 SQL 文件
2. 使用 `migrate run --file xxx.sql` 执行其他开发人员的变更
3. 或使用 `migrate up --version xxx` 执行整个版本目录的变更

**新增变更工作流：**
1. 在 `sql-change/v{版本}/{db_type}/` 下创建新 SQL 文件
2. 文件名遵循命名规范，内容包含变更说明注释
3. 同步更新 `init/init-sqlite.sql`（合并变更内容）
4. 测试验证后提交代码

**团队数据库命名：**
- 正式数据库：`product`（团队共享，谨慎操作）
- 开发数据库：`dev_<姓名拼音>`（如 dev_zhangsan）
- 账号密码通过团队内部渠道获取，严禁提交到代码仓库

**主要表结构：**
| 表名 | 说明 |
|------|------|
| base_agents | 基础 Agent 配置（Claude、OpenAI 等） |
| workflow_templates | 工作流模板 |
| projects | 项目信息 |
| threads | 开发会话 |
| messages | 对话消息 |
| agent_configs | Agent 角色配置 |
| agent_invocations | Agent 调用记录 |
| artifacts | 开发产物 |
| sandboxes | 沙箱容器 |

### 数据库类型
- 默认使用 SQLite（`modernc.org/sqlite` 纯 Go 驱动，无需 CGO）
- 过渡期保留 MySQL 支持（通过 `database.type` 配置切换）

### 服务端口（灵活查找）
- 后端端口：查看 `configs/config.yaml` 中 `server.port`
- 前端开发端口：查看 `configs/config.yaml` 中 `web.port`

## Data 目录

`data/` 存放用户数据，升级时保留：
- `configs/config.yaml` - 主配置
- `logs/` - 日志
- `agent-assets/` - Agent 资产
- `repos/` - 代码仓库

**配置查找顺序**：命令行参数 `-config` → 环境变量 `ISDP_CONFIG` → `data/configs/config.yaml` → `configs/config.yaml`

## 自动测试体系

所有测试统一放置在 `auto-test/` 目录，采用混合分层结构：

```
auto-test/
├── e2e/                    # Playwright E2E 测试
│   ├── fixtures/           # 测试 fixtures
│   ├── agent-dialog/       # AD-01 ~ AD-04
│   ├── websocket/          # WS-01 ~ WS-02
│   ├── thread-workflow/    # TW-01 ~ TW-03
│   ├── team-package/       # TP-01
│   └── api/                # API-02 ~ API-04
│
├── internal/               # Go 单元测试（可导入 internal 包）
│   ├── api/                # API Handler 测试
│   ├── service/            # Service 层测试
│   ├── repo/               # Repo 层测试
│   └── testdata/           # 测试数据
│
├── vitest/                 # Vitest 组件测试
│   ├── components/         # UI 组件测试
│   └── setup.ts            # Mock 配置
│
└── feature-map.yaml        # 功能 ID 映射
```

### 测试优先级

| 优先级 | 覆盖率要求 | 说明 |
|--------|-----------|------|
| **P0** | 必须 100% | 阻塞功能、核心流程、安全相关 |
| **P1** | ≥ 95% | 重要功能、主要路径 |
| **P2** | ≥ 80% | 一般功能、边界场景 |
| **P3** | 可选 | 体验优化、次要功能 |

### 测试命令

```bash
# 运行所有测试
make test-all

# 分类测试
make test-frontend      # E2E + Vitest
make test-backend       # Go 单元测试

# 功能测试（按 Feature ID）
python scripts/run-feature-tests.py --feature F001
python scripts/run-feature-tests.py --priority P0

# E2E 测试
cd web && npx playwright test
npx playwright test --grep "AD-01"    # 按测试 ID

# Go 测试
go test ./auto-test/internal/... -v
go test ./auto-test/internal/api/... -run API-01
```

### 测试 ID 格式

测试用例 ID 格式：`{模块}-{分类}-{序号}`

- `AD-01-02`：Agent Dialog，消息输入分类，第 2 个用例
- `WS-01-03`：WebSocket，连接管理分类，第 3 个用例
- `API-01-05`：API Handler，Agent CRUD 分类，第 5 个用例

每个测试文件头部需标注 Feature ID 和优先级：

```go
// @feature F001 - Agent 对话核心
// @priority P0
// @id API-01-01
func TestAgentHandler_List(t *testing.T) { ... }
```
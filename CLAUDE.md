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
├── history/              # 历史归档（不再使用）
├── v1.0.0/               # 版本目录
│   └── mysql/            # 按数据库类型分目录
│       └── *.sql
└── v1.1.0/
    ├── mysql/
    └── sqlite/
```

**管理规则：**
- 新版本发布时创建 `v{版本号}` 目录
- 目录下按数据库类型分 `mysql/` 和 `sqlite/`
- 文件命名：`{序号}_description.sql`
- 必须包含 `-- +goose Up` 和 `-- +goose Down` 注释
- MySQL 禁止 `DROP COLUMN IF EXISTS`（5.7 不支持）
- SQLite 表结构变更需重建表

**执行方式：**
- 安装器统一调用 `migrate up` 命令
- goose 自动处理首次安装和版本升级

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

### 服务端口
- 后端: 8080
- 前端开发: 3000

## Data 目录

`data/` 存放用户数据，升级时保留：
- `configs/config.yaml` - 主配置
- `logs/` - 日志
- `agent-assets/` - Agent 资产
- `repos/` - 代码仓库

**配置查找顺序**：命令行参数 `-config` → 环境变量 `ISDP_CONFIG` → `data/configs/config.yaml` → `configs/config.yaml`
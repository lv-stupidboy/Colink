# CLAUDE.md

ISDP (Intelligent Software Development Platform) - 智能软件开发平台，多 Agent 协同开发系统。

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

迁移脚本位于 `sql-change/` 目录：

```
sql-change/
├── init.sql         # 当前版本初始化 SQL
├── history/         # 历史归档（1.0.0 之前）
└── migrations/      # 按版本归档的增量变更
    ├── v1.0.1/
    │   └── 202604100001_add_xxx.sql
    └── v1.0.2/
```

**归档规则：**
- 新版本发布时创建 `v{版本号}` 目录，如 `v1.0.1`
- 文件命名：`YYYYMMDDNN_description.sql`（日期+序号+描述）
- **禁止使用 `DROP COLUMN IF EXISTS`**（MySQL 5.7 不支持）

**执行流程：**
- 新环境：执行 `init.sql`
- 版本升级：执行 `migrations/v{版本}/` 下对应 SQL

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
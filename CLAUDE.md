# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

ISDP (Intelligent Software Development Platform) - 智能软件开发平台，通过多 Agent 协同开发系统实现从想法到产品的快速落地。

## 常用命令

### 后端 (Go)
```bash
make build        # 构建到 bin/isdp
make run          # 运行服务 (go run ./cmd/server)
make test         # 运行测试 (覆盖率报告)
go test ./internal/service/agent -v  # 运行单个包测试
```

### 前端 (React + Vite)
```bash
cd web && npm run dev       # 启动开发服务器
cd web && npm run build     # 构建生产版本
cd web && npm run lint      # ESLint 检查
cd web && npm run test      # 运行测试运行器
cd web && npm run test:e2e  # Playwright E2E 测试
```

## 架构

```
┌─────────────────────────────────────────────┐
│  Web前端 (React + Ant Design + Zustand)     │
└─────────────────────────────────────────────┘
                    ↓ REST/WebSocket
┌─────────────────────────────────────────────┐
│  后端服务 (Go + Gin)                         │
│  internal/                                   │
│    api/       → HTTP 处理器                  │
│    service/   → 业务逻辑层                   │
│      agent/   → Agent 引擎核心               │
│      sandbox/ → Docker 沙箱管理              │
│    repo/      → 数据访问层                   │
│    model/     → 数据模型                     │
└─────────────────────────────────────────────┘
                    ↓ CLI Spawn
┌─────────────────────────────────────────────┐
│  Agent 实例 (Claude CLI / OpenCode 适配器)  │
└─────────────────────────────────────────────┘
```

## 关键模块

- **Agent 引擎** (`internal/service/agent/`): Orchestrator 编排器、ExecutionService 执行服务、ContextBuilder 上下文构建器
- **A2A 路由** (`internal/service/a2a/`): `@Agent名` 触发协作，MCP 工具回传
- **沙箱服务** (`internal/service/sandbox/`): Docker 容器管理，端口自动分配 (30000-40000)
- **工作流引擎** (`internal/service/workflow/`): 阶段流转 (需求→设计→实现→审查→测试→部署)

## 配置

- 配置模板: `configs/config.yaml.example`
- 复制为 `configs/config.yaml` 后修改
- 敏感信息（数据库密码）不提交，使用占位符

## 数据库变更

- 增量脚本: `sql-change/migrations/`
- 命名格式: `YYYYMMDD_description.sql`
- 完整初始化: `sql-change/init_db_mysql.sql`
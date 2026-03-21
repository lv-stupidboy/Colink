# 开发范式融入 ISDP 平台 - 第二阶段：配置生成

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现项目配置生成功能，允许用户将绑定的 Skill 同步到项目的 .claude/ 或 .opencode/ 目录。

**Architecture:** 新增 ConfigGenerator 服务，通过 HTTP API 触发，根据项目的 AgentRole 和绑定的 Skill 生成对应智能体的配置文件。

**Tech Stack:** Go 1.21+ / Gin / HTTP Client / 文件系统操作

---

## 文件结构

**新增文件：**

```
isdp/
├── internal/
│   ├── service/
│   │   └── configgen/
│   │       ├── service.go           # 配置生成服务
│   │       └── downloader.go        # Skill 文件下载器
│   └── api/
│       └── configgen_handler.go     # 配置生成 API
└── web/src/
    └── pages/ProjectDetail/
        └── SkillSyncButton.tsx      # 项目详情页的同步按钮
```

**修改文件：**

```
isdp/
├── cmd/server/main.go               # 注册新服务
└── web/src/pages/ProjectDetail/
    └── index.tsx                    # 集成同步按钮
```

---

## Task 1: Skill 下载服务

**Files:**
- Create: `isdp/internal/service/configgen/downloader.go`

**Requirements:**
1. 下载器从 `install_source` 字段获取下载 URL
2. 支持不同智能体类型的下载地址
3. 下载文件保存到指定目录
4. 错误处理和重试机制

**Instructions:**
1. 创建文件
2. 验证编译
3. 提交: `feat(configgen): add Skill file downloader`

---

## Task 2: 配置生成服务

**Files:**
- Create: `isdp/internal/service/configgen/service.go`

**Requirements:**
1. `GenerateConfig(projectID, baseAgentType)` 方法
2. 获取项目的所有 AgentRole
3. 获取每个 AgentRole 绑定的 Skill
4. 调用下载器下载 Skill 文件
5. 生成目录结构：
   - ClaudeCode: `.claude/skills/*.md`, `.claude/rules/*.md`
   - OpenCode: `.opencode/tool/*.md`, `.opencode/agent/*.md`

**Instructions:**
1. 创建文件
2. 验证编译
3. 提交: `feat(configgen): add ConfigGenerator service`

---

## Task 3: 配置生成 API

**Files:**
- Create: `isdp/internal/api/configgen_handler.go`

**Requirements:**
1. POST `/projects/:id/config/sync` 端点
2. 请求体: `{ "baseAgentType": "claude_code" }`
3. 返回生成结果

**Instructions:**
1. 创建文件
2. 验证编译
3. 提交: `feat(api): add config generation API endpoint`

---

## Task 4: 集成到主程序

**Files:**
- Modify: `isdp/cmd/server/main.go`

**Instructions:**
1. 添加依赖注入
2. 注册路由
3. 验证编译和启动
4. 提交: `feat: integrate ConfigGenerator into main server`

---

## Task 5: 前端同步按钮

**Files:**
- Create: `isdp/web/src/pages/ProjectDetail/SkillSyncButton.tsx`
- Modify: `isdp/web/src/pages/ProjectDetail/index.tsx`

**Requirements:**
1. 项目设置页显示 "同步配置" 按钮
2. 选择目标智能体类型
3. 显示同步进度和结果

**Instructions:**
1. 创建组件
2. 集成到项目详情页
3. 验证编译
4. 提交: `feat(web): add Skill sync button to project detail`

---

## 执行选择

**计划完成并保存。两种执行方式：**

**1. Subagent-Driven (推荐)** - 我为每个任务派遣一个新的子代理

**2. Inline Execution** - 在此会话中批量执行

**选择哪种方式？**
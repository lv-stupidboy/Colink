# 开发变更记录

本文件记录项目的开发变更历史，用于后期复盘和追溯。

---

## 2026-04-01 资产包轻量化重构与团队包功能

### 背景

原有的资产包设计引入了版本概念和元数据表管理，带来了额外的管理负担。实际使用中，资产包只需要作为导入导出工具，不需要存储版本信息。同时，团队协作场景需要能够完整导出工作流及其关联的角色和资产。

### 目标

1. 移除资产包的版本概念，简化为纯粹的导入导出工具
2. 删除 asset_packages 元数据表
3. 新增团队包功能，支持导出工作流+角色+资产的完整配置
4. 菜单结构调整：管理工具作为 Agent团队的二级菜单

### 核心变更

#### 后端改动

**模型简化：**
- `isdp/internal/model/asset_package.go` - 移除 AssetPackage 实体，保留请求/响应结构和 Manifest
- `isdp/internal/model/team_package.go` - 新增团队包模型（TeamPackageManifest, TeamPackagePreview, TeamPackageImportConfirm）

**服务层：**
- `isdp/internal/service/assetpackage/service.go` - 简化为仅 Import/Export 方法，移除元数据管理
- `isdp/internal/service/teampackage/service.go` - 新增团队包服务（Export, ImportPreview, ImportConfirm）

**API Handler：**
- `isdp/internal/api/asset_package_handler.go` - 仅保留 Import 和 Export 端点
- `isdp/internal/api/team_package_handler.go` - 新增团队包端点（Import, ImportConfirm, Export）

**数据库：**
- `isdp/sql-change/migrations/202604010001_drop_asset_packages_table.sql` - 删除 asset_packages 表

#### 前端改动

**菜单结构：**
- `isdp/web/src/layouts/MainLayout.tsx` - 管理工具作为 Agent团队的二级菜单，团队包和资产包为三级

**页面组件：**
- `isdp/web/src/pages/AssetPackage/index.tsx` - 重写为双卡片布局（导入+导出）
- `isdp/web/src/pages/TeamPackage/index.tsx` - 新增团队包页面（导入预览+导出）

**路由配置：**
- `isdp/web/src/App.tsx` - 路由路径改为 `/agents/team-packages` 和 `/agents/asset-packages`

**API 客户端：**
- `isdp/web/src/api/client.ts` - 简化 assetPackages，新增 teamPackages API

### 新增/修改文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/sql-change/migrations/202604010001_drop_asset_packages_table.sql` | 新建 | 数据库迁移脚本 |
| `isdp/internal/model/asset_package.go` | 修改 | 简化模型，移除版本 |
| `isdp/internal/model/team_package.go` | 新建 | 团队包模型定义 |
| `isdp/internal/service/assetpackage/service.go` | 修改 | 简化服务，仅保留导入导出 |
| `isdp/internal/service/teampackage/service.go` | 新建 | 团队包服务实现 |
| `isdp/internal/api/asset_package_handler.go` | 修改 | 简化端点 |
| `isdp/internal/api/team_package_handler.go` | 新建 | 团队包 Handler |
| `isdp/cmd/server/main.go` | 修改 | 更新服务注册 |
| `isdp/web/src/layouts/MainLayout.tsx` | 修改 | 管理工具改为二级菜单 |
| `isdp/web/src/pages/AssetPackage/index.tsx` | 重写 | 双卡片布局 |
| `isdp/web/src/pages/TeamPackage/index.tsx` | 新建 | 团队包页面 |
| `isdp/web/src/api/client.ts` | 修改 | 更新 API 客户端 |
| `isdp/web/src/App.tsx` | 修改 | 更新路由路径 |

### 验证方法

1. 启动后端服务：`cd isdp && go run ./cmd/server`
2. 启动前端：`cd web && npm run dev`
3. 访问 Agent团队 → 管理工具 菜单，验证团队包和资产包页面
4. 测试资产包导入导出功能
5. 测试团队包导入（包含冲突预览）导出功能

### 影响范围

- 资产包功能完全重构，原有版本管理功能移除
- 新增团队包功能模块
- 菜单结构变更：管理工具从一级菜单改为 Agent团队的二级菜单

---

## 2026-03-27 安装器完整实现与优化

### 背景

ISDP 需要一个完整的 Windows 安装器，支持一键安装、服务管理、升级卸载等功能。原项目使用简单的打包方式，存在安装流程不完整、依赖检测缺失、服务管理不便等问题。

### 目标

1. 实现完整的 Windows 安装器（Setup.exe）和启动器
2. 支持依赖检测和自动安装
3. 支持数据库配置和服务启动
4. 支持 ZIP 包分发，便于离线安装

### 核心变更

#### 安装器架构重构

**Setup 与 Launcher 分离：**
- **ISDP Setup.exe**：负责安装、升级、卸载流程
- **ISDP.exe (Launcher)**：负责服务启停、日志查看、配置管理

**新的打包结构：**
```
release/
├── ISDP-1.0.0.zip          # 发布包
├── ISDP/                   # 安装文件
│   ├── ISDP.exe            # Launcher
│   ├── ISDP-Setup.exe      # 安装程序
│   ├── isdp-server.exe     # 后端服务
│   └── web/                # 前端静态文件
└── launcher/               # Launcher 独立包
```

#### 后端改动

**新增文件：**
- `installer/src/main/index.ts` - Setup 入口（安装/升级/卸载）
- `installer/src/main/launcher-entry.ts` - Launcher 入口（服务管理）
- `installer/src/main/service-manager.ts` - 服务进程管理
- `installer/src/main/installer.ts` - 安装逻辑
- `installer/src/main/shared/window-utils.ts` - 窗口工具函数

**打包配置优化：**
- `electron-builder.yml` - Setup 打包配置，精确控制打包内容
- `electron-builder.launcher.yml` - Launcher 独立打包配置
- `build.ps1` / `build.sh` - 一键构建脚本

#### 前端改动

**新增页面：**
- `pages/Welcome.tsx` - 欢迎页
- `pages/DirectorySelect.tsx` - 安装目录选择
- `pages/DependencyCheck.tsx` - 依赖检测（Node.js、Git、Claude CLI）
- `pages/ModeSelect.tsx` - 安装模式选择
- `pages/SystemConfig.tsx` - 数据库配置
- `pages/Installing.tsx` - 安装进度
- `pages/Complete.tsx` - 安装完成
- `pages/Dashboard.tsx` - Setup 仪表盘
- `pages/LauncherDashboard.tsx` - Launcher 仪表盘

**组件和工具：**
- `components/Layout.tsx` - 页面布局组件
- `services/dependencyChecker.ts` - 依赖检测服务
- `services/configGenerator.ts` - 配置文件生成

### 功能特性

| 功能 | 说明 |
|------|------|
| 依赖检测 | 自动检测 Node.js、Git、Claude CLI，缺失时提供下载链接 |
| 数据库配置 | 支持 MySQL 和 SQLite，自动生成配置文件 |
| 桌面快捷方式 | 自动创建桌面和开始菜单快捷方式 |
| 服务管理 | Launcher 支持启动/停止后端服务 |
| 日志查看 | 内置日志查看功能 |
| 升级安装 | 检测已安装版本，支持升级和卸载 |
| ZIP 分发 | 打包为 ZIP，支持离线分发 |

### 新增文件列表

| 文件 | 说明 |
|------|------|
| `installer/src/main/index.ts` | Setup 主入口 |
| `installer/src/main/launcher-entry.ts` | Launcher 主入口 |
| `installer/src/main/service-manager.ts` | 服务管理器 |
| `installer/src/main/installer.ts` | 安装逻辑 |
| `installer/src/main/shared/window-utils.ts` | 窗口工具 |
| `installer/src/preload/index.ts` | Electron preload |
| `installer/src/renderer/*` | React 前端界面 |
| `installer/build.ps1` | Windows 构建脚本 |
| `installer/build.sh` | Unix 构建脚本 |
| `installer/electron-builder.yml` | Setup 打包配置 |
| `installer/electron-builder.launcher.yml` | Launcher 打包配置 |

### 修改文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `installer/package.json` | 修改 | 添加依赖和脚本 |
| `.gitignore` | 修改 | 忽略 release 目录 |
| `CLAUDE.md` | 修改 | 更新安装器文档 |
| `isdp/web/src/config/version.ts` | 修改 | 版本号更新 |

### 提交记录（60 个提交）

按功能分组：

**安装器功能实现：**
```
ed66b5c feat(installer): initialize project structure
c1f5028 feat(installer): add vite and electron-builder config
abf79f9 feat(installer): add main process entry
b1b4db8 feat(installer): add preload script
606b14b feat(installer): add type definitions
975f924 feat(installer): add global styles
75b4661 feat(installer): add React entry and App component
0886a4f feat(installer): add Layout component
9f0b898 feat(installer): add Welcome page
d29728b feat(installer): add DirectorySelect page
a8012bd feat(installer): add DependencyCheck page and service
b3375da feat(installer): add ModeSelect page
1d88ebb feat(installer): add ConfigSection component
aaef2cc feat(installer): add SystemConfig page and database connector
3b62cb5 feat(installer): add Installing page and config generator
0e73df8 feat(installer): add Complete page
7500741 feat(installer): add main process installer logic and IPC handlers
14261e6 feat(installer): implement file copy logic
a3b2ace feat(installer): implement directory selection dialog
f814205 feat(installer): implement installation flow
92f0c38 feat(installer): implement launcher service manager and tray
d48dc8f feat(installer): implement Node.js and Git installation
11f3540 feat(installer): add launch service functionality
a24a7f9 feat(installer): add launcher standalone build config
50af906 feat(installer): implement desktop shortcut creation
bb4d4f3 feat(installer): add icon and complete package build
d1f290d feat(installer): add build scripts for Windows and Unix
```

**安装器修复：**
```
7fb5e6e fix(installer): include mysql2 in bundle instead of externalizing
38b308e fix(installer): update file paths for new package structure
56ad230 fix(installer): use original-fs to handle asar file copying
b7f1233 fix(installer): improve UI, fix launcher path, exclude debug files
```

**安装器重构：**
```
748e859 refactor(installer): remove tray and launcher-entry files
86a1b85 refactor(installer): remove tray functions from index.ts
b67dfbb refactor(installer): change window close behavior with confirmation dialog
d18035d refactor(installer): update dashboard close hint text
7bd0a10 refactor(installer): update launcher build config with file exclusions
c7e6559 refactor(installer): update installer build config with runtime and launcher
c32261c refactor(installer): add package:launcher script
4cbe060 refactor(installer): update build scripts for new workflow
37b6d2a refactor(installer): update zip script for new structure
e273a7c refactor(installer): remove launcher-entry from vite config
```

**文档更新：**
```
7caf3c7 docs: update changelog for installer refactor
79764d9 docs: add installer refactor implementation plan
95bf3b9 docs: add installer refactor design v2
c8fd2d7 docs: add Windows installer implementation plan
f6a05a1 docs: improve Windows installer design spec
323093f docs: add Windows installer design specification
d78cc85 docs: add tasks 25-28 for complete installer functionality
715fcb7 docs: add prerequisites and clarify launcher architecture
feba230 docs: add missing dependencies and APIs to installer plan
b3f9533 docs: fix and enhance Windows installer plan
cd36c6a docs: fix installer plan issues
d1a9f44 docs: fix build script for proper launcher packaging order
96645d9 docs: fix duplicate content in NSIS script and step numbering
02436be docs: fix plan documentation issues
6cc57ba docs: integrate Node.js/Git installation and add disk space detection
24280f0 docs: fix plan documentation issues and fix missing imports
665ec3e fix(installer-plan): add Windows PowerShell build script to Task 28
```

**清理工作：**
```
9d597a2 feat(installer): separate Setup and Launcher functionality
617fac9 chore(installer): remove release directory from tracking and improve code
```

### 验证方法

1. 进入 installer 目录执行 `.\build.ps1`
2. 检查 `release/ISDP-1.0.0.zip` 生成成功
3. 解压并运行 `ISDP-Setup.exe` 测试安装流程
4. 运行 `ISDP.exe` 测试服务启动

### 影响范围

- installer 模块完整重构
- 新增 Windows 安装器功能
- 支持 ZIP 包分发

---

## 2026-03-26 Agent资产绑定与Skill使用统计

### 背景

修复调试功能问题，完善Agent复制功能，实现Skill使用次数统计定时任务，清理项目代码。

### 改动内容

**后端改动：**
- 新增UseCountUpdater定时任务，统计技能被项目使用次数
- Agent复制时同步复制绑定的Skill/Subagent/Command/Rule
- WorkflowService添加GetAgentIDs方法支持统计
- 统一使用zap日志框架

**前端改动：**
- 修复调试模式WebSocket连接问题（第二个角色不响应）
- 消息显示优化，正确展示Agent名称而非UUID

**其他：**
- 更新.gitignore，排除openspec/和playwright-report/
- 删除废弃的subagent模板文件

---

## 2026-03-25 Solo模式、消息渲染优化、工作流页面优化

### 背景

团队协作开发的功能合并，来自同事的提交。

### 改动内容

**前端改动：**
- 菜单结构调整：重命名"Agent 角色"为"Agent 资产"，修复路由问题
- 消息渲染优化：支持Markdown格式、代码高亮、Mermaid架构图
- 工作流页面优化：应用沙箱沉浸式风格

---

## 2026-03-24 Agent 资产管理功能实现

### 背景

将团队基于 Claude Code 的实践范式沉淀到 ISDP 平台，包括 Command、Rule 等资产管理能力，将 Agent 角色升级为一级菜单。

### 目标

1. 将 Agent 角色菜单改为一级菜单，下设多个二级菜单
2. 新增 Command（命令）、Rule（规约）资产管理能力
3. 建立清晰的资产绑定关系：Agent→Command/Subagent/Rule，Command/Subagent→Skill
4. 配置生成时支持复制所有相关资产文件

### 核心变更

#### 后端改动

##### 数据模型
- 新增 `internal/model/command.go` - Command 模型
- 新增 `internal/model/rule.go` - Rule 模型

##### Repository 层
- 新增 `internal/repo/command.go` - Command Repository
- 新增 `internal/repo/rule.go` - Rule Repository
- 新增 `internal/repo/agent_command_binding.go` - Agent-Command 绑定
- 新增 `internal/repo/agent_rule_binding.go` - Agent-Rule 绑定
- 新增 `internal/repo/command_skill_binding.go` - Command-Skill 绑定
- 新增 `internal/repo/subagent_skill_binding.go` - Subagent-Skill 绑定

##### Service 层
- 新增 `internal/service/command/service.go` - Command Service
- 新增 `internal/service/rule/service.go` - Rule Service
- 扩展 `internal/service/configgen/service.go` - 支持 Command 和 Rule

##### API 层
- 新增 `internal/api/command_handler.go` - Command Handler
- 新增 `internal/api/rule_handler.go` - Rule Handler
- 扩展 Agent 绑定 API（Command/Rule）
- 新增 Subagent 技能绑定 API

#### 前端改动

##### 菜单结构重构
- 重构 `MainLayout.tsx`，Agent 角色改为一级菜单
- 新增二级菜单：Agent 角色、Subagent、Command、Rule

##### 新增页面
- 新增 `web/src/pages/CommandList.tsx` - 命令集管理页面
- 新增 `web/src/pages/RuleList.tsx` - 规约管理页面
- 新增 `web/src/pages/PlaceholderPage.tsx` - 占位页面组件

##### 页面扩展
- 扩展 `AgentRoleList.tsx` - 支持 Command/Rule 绑定
- 扩展 `SubagentList.tsx` - 支持技能绑定

#### 数据库变更

```sql
-- 新增 commands 表
CREATE TABLE commands (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    content TEXT NOT NULL,
    tags JSON,
    use_count INT DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- 新增 rules 表
CREATE TABLE rules (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    content TEXT NOT NULL,
    scope VARCHAR(50) NOT NULL,  -- public/instance
    tags JSON,
    use_count INT DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- 新增绑定关系表
CREATE TABLE agent_command_bindings (...);
CREATE TABLE agent_rule_bindings (...);
CREATE TABLE command_skill_bindings (...);
CREATE TABLE subagent_skill_bindings (...);
```

### 新增文件列表

| 文件 | 说明 |
|------|------|
| `internal/model/command.go` | Command 模型 |
| `internal/model/rule.go` | Rule 模型 |
| `internal/repo/command.go` | Command Repository |
| `internal/repo/rule.go` | Rule Repository |
| `internal/repo/agent_command_binding.go` | Agent-Command 绑定 |
| `internal/repo/agent_rule_binding.go` | Agent-Rule 绑定 |
| `internal/repo/command_skill_binding.go` | Command-Skill 绑定 |
| `internal/repo/subagent_skill_binding.go` | Subagent-Skill 绑定 |
| `internal/service/command/service.go` | Command Service |
| `internal/service/rule/service.go` | Rule Service |
| `internal/api/command_handler.go` | Command Handler |
| `internal/api/rule_handler.go` | Rule Handler |
| `web/src/pages/CommandList.tsx` | 命令集管理页面 |
| `web/src/pages/RuleList.tsx` | 规约管理页面 |
| `web/src/pages/PlaceholderPage.tsx` | 占位页面组件 |

### API 端点

| 模块 | 方法 | 路径 | 说明 |
|------|------|------|------|
| Command | GET | `/api/v1/commands` | 列出命令 |
| Command | POST | `/api/v1/commands` | 创建命令 |
| Command | PUT | `/api/v1/commands/:id` | 更新命令 |
| Command | DELETE | `/api/v1/commands/:id` | 删除命令 |
| Rule | GET | `/api/v1/rules` | 列出规约 |
| Rule | POST | `/api/v1/rules` | 创建规约 |
| Rule | PUT | `/api/v1/rules/:id` | 更新规约 |
| Rule | DELETE | `/api/v1/rules/:id` | 删除规约 |
| Agent-Command | GET/POST | `/api/v1/agent-commands/:agentId` | Agent 命令绑定 |
| Agent-Rule | GET/POST | `/api/v1/agent-rules/:agentId` | Agent 规约绑定 |
| Command-Skill | GET/POST | `/api/v1/command-skills/:commandId` | 命令技能绑定 |
| Subagent-Skill | GET/POST | `/api/v1/subagent-skills/:subagentId` | Subagent 技能绑定 |

### 验证方法

1. 启动后端服务: `cd isdp && go run ./cmd/server`
2. 启动前端服务: `cd web && npm run dev`
3. 访问 Agent 角色菜单，测试各二级菜单功能
4. 测试 Command 创建、编辑、删除
5. 测试 Rule 创建、编辑、删除
6. 测试 Agent 绑定 Command/Rule
7. 测试 Subagent 绑定 Skill

### 影响范围

- Agent 角色管理模块全面重构
- 配置生成服务功能扩展
- 前端菜单结构调整
- 新增 6 张数据库表

### 设计文档

- `docs/Agent配置隔离与Subagent融合设计文档.md`
- `docs/Agent资产管理设计文档.md`
- `docs/openspec/plans/2026-03-24-agent-assets-management.md`

### 单元测试

- Command Service 名称验证测试（18个测试用例）
- Rule Service 名称验证测试（18个测试用例）
- RuleScope 类型测试

---

## 2026-03-23 Solo 模式任务抽屉修复

### 背景

Solo 模式左侧任务列表存在布局异常、切换功能缺失、新建任务跳转页面等问题，影响用户的主要开发交互体验。

### 目标

1. 修复 Solo 模式布局，实现任务抽屉与消息区的水平排列
2. 完善任务切换逻辑，确保选择历史任务时能正确加载消息
3. 新建任务时不跳转页面，直接开启新对话
4. 添加抽屉展开/收起控制，提升用户体验

### 核心变更

#### 前端改动

##### CSS 样式
- `pages/ThreadView.css` - 添加 `.solo-mode-body` 水平布局容器、`.solo-task-drawer` 抽屉样式

##### 状态管理
- `store/index.ts` - 新增 `clearThreadMessages`、`setCurrentThread` actions

##### 组件逻辑
- `pages/ThreadView.tsx` - 添加 `taskDrawerOpen` 状态、修改 JSX 结构、修复切换和新建逻辑

##### 关键修复
1. **布局修复**：添加 `.solo-mode-body` 容器实现水平布局
2. **消息清空**：工作流模式下新建对话时清空 `workflowMessages`
3. **线程设置**：创建/切换任务时设置 `currentThread`，确保 `sendMessage` 正常工作
4. **任务切换**：加载历史消息到对应的 store（调试模式/工作流模式）

### 新增/修改文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| isdp/web/src/pages/ThreadView.css | 修改 | 添加布局和抽屉样式 |
| isdp/web/src/pages/ThreadView.tsx | 修改 | 状态、JSX 结构、事件处理 |
| isdp/web/src/store/index.ts | 修改 | 新增 clearThreadMessages、setCurrentThread actions |

### 验证方法

1. 进入项目 → 切换 Solo 模式
2. 点击"新建对话"，确认对话框清空显示欢迎界面
3. 输入消息发送，确认任务创建成功且消息正常显示
4. 切换历史任务，确认消息正确加载
5. 在历史任务中发送消息，确认响应正常

### 影响范围

- Solo 模式（所有 Agent 调试模式可用）
- 全栈工程师 Agent：自动进入 Solo 模式
- 其他工作流：可通过顶部 Solo 按钮手动切换进入

---

## 2026-03-23 工作流模板页面沙箱风格优化

### 背景

工作流模板页面使用传统 Ant Design 样式，与沙箱预览页面的现代沉浸式风格不一致。需要按照沙箱空间风格进行调整，统一视觉体验。

### 目标

1. 应用沙箱沉浸式背景和浮动卡片设计
2. 统一圆角、阴影和过渡动画
3. 优化模板卡片的选中状态和悬停效果
4. 支持深色主题
5. 响应式设计适配

### 核心变更

#### 前端改动

##### 新增样式文件
- `pages/Workflow/Workflow.css` - 工作流页面沙箱风格样式

##### 修改组件
- `pages/Workflow/index.tsx` - 应用新的 CSS 类名和结构

##### 设计特点
- **沉浸式背景**：使用 `--gradient-bg` CSS 变量实现渐变背景
- **浮动卡片**：圆角 16px，柔和阴影，悬停时微妙上浮
- **模板卡片**：选中状态高亮，悬停时边框变色 + 阴影
- **Agent 卡片**：圆角头像，渐变背景，悬停动效
- **编排规则**：图标化标签，流程箭头可视化
- **深色主题**：完整支持 `data-theme='dark'`

### 新增/修改文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| isdp/web/src/pages/Workflow/Workflow.css | 新增 | 沙箱风格样式 |
| isdp/web/src/pages/Workflow/index.tsx | 修改 | 应用新样式 |

### 验证方法

1. 访问工作流编排页面
2. 切换不同主题（翡翠绿、深海蓝、深邃黑等）
3. 测试模板卡片选中/悬停效果
4. 验证深色主题显示

### 影响范围

- 工作流编排页面视觉体验
- 与沙箱预览风格统一

---

## 2026-03-23 消息内容展示优化（视觉内容 + 代码 Diff）

### 背景

消息内容展示优化，将视觉内容（架构图、图表、图片）与代码内容分离展示：
- 视觉内容直接在气泡内展示，一目了然
- 代码内容在右侧面板展示，支持 Split Diff 对比和深入操作
- 右侧面板可收起，不影响对话流畅性

### 目标

1. 实现视觉内容卡片组件，支持 Mermaid 架构图渲染
2. 实现代码预览入口按钮，点击打开右侧面板
3. 实现右侧代码面板，包含文件列表和 Split Diff 视图
4. 支持面板收起/展开，文件列表纵向排列

### 核心变更

#### 前端改动

##### 新增依赖
- `mermaid` - 架构图渲染
- `diff-match-patch` - 代码差异计算
- `@types/diff-match-patch` - TypeScript 类型定义

##### 新增类型定义
- `types/content.ts` - 内容类型定义（ContentType, ContentBlock, FileChange, CodePanelState）

##### 新增工具函数
- `utils/contentDetector.ts` - 内容类型检测
  - `detectContentType()` - 检测代码块内容类型
  - `parseContentBlocks()` - 解析 Markdown 为内容块列表
  - `parseCodeFiles()` - 解析代码块为文件变更列表

##### 新增组件
- `components/thread/ContentCard.tsx` - 视觉内容卡片（架构图、错误日志）
- `components/thread/CodePreviewButton.tsx` - 代码预览入口按钮
- `components/thread/CodePanel/index.tsx` - 代码面板容器
- `components/thread/CodePanel/FileList.tsx` - 文件列表
- `components/thread/CodePanel/FileItem.tsx` - 文件项
- `components/thread/CodePanel/SplitDiff.tsx` - Split Diff 视图

##### 组件功能
- **ContentCard**：Mermaid 架构图渲染、错误日志终端样式、放大/下载功能
- **CodePreviewButton**：显示代码文件名、变更统计、点击打开面板
- **CodePanel**：文件列表纵向排列、支持收起/展开、全部应用/复制
- **SplitDiff**：左右对比视图、新增/删除行高亮、同步滚动

##### Store 改动
- `store/debugThread.ts` - 添加代码面板状态和方法
  - `codePanelOpen`, `codePanelCollapsed`, `expandedFiles`, `codeFiles`
  - `openCodePanel`, `closeCodePanel`, `toggleCodePanelCollapse`, `toggleFileExpand`

##### ThreadView.tsx 改动
- 根据内容类型选择展示方式
- 视觉内容在气泡内卡片展示
- 代码内容通过入口按钮打开右侧面板
- 集成 CodePanel 到 Solo 模式布局

### 新增文件

| 文件 | 说明 |
|------|------|
| `components/thread/ContentCard.tsx` | 视觉内容卡片组件 |
| `components/thread/ContentCard.css` | 卡片样式 |
| `components/thread/CodePreviewButton.tsx` | 代码预览入口 |
| `components/thread/CodePreviewButton.css` | 入口样式 |
| `components/thread/CodePanel/index.tsx` | 代码面板容器 |
| `components/thread/CodePanel/FileList.tsx` | 文件列表 |
| `components/thread/CodePanel/FileItem.tsx` | 文件项 |
| `components/thread/CodePanel/SplitDiff.tsx` | Diff 视图 |
| `components/thread/CodePanel/CodePanel.css` | 面板样式 |
| `types/content.ts` | 内容类型定义 |
| `utils/contentDetector.ts` | 内容检测工具 |

### 验证方法

1. 发送消息请求生成架构图，验证 ContentCard 正确渲染 Mermaid
2. 发送消息请求生成代码，验证 CodePreviewButton 正确显示
3. 点击代码入口，验证 CodePanel 打开
4. 测试面板收起/展开功能
5. 测试文件展开/收起功能
6. 验证 Split Diff 左右对比正确

### 影响范围

- 前端消息渲染逻辑
- 调试模式状态管理
- 新增多个 UI 组件

---

## 2026-03-23 消息内容 Markdown 渲染功能

### 背景

对话框中 AI 输出的内容（代码、架构图、图片等）一直以纯文本形式展示，用户体验不佳。需要实现类似 Trae 的富文本渲染效果，支持 Markdown 格式、代码高亮、图片展示等。

### 目标

1. 实现 Markdown 内容渲染（标题、列表、引用、表格等）
2. 实现代码块语法高亮和复制功能
3. 支持图片渲染和预览
4. 保持流式消息的实时渲染

### 核心变更

#### 前端改动

##### 新增依赖
- `react-markdown` - Markdown 解析渲染
- `remark-gfm` - GitHub 风格 Markdown（表格、删除线、任务列表）
- `rehype-highlight` - 代码语法高亮
- `highlight.js` - 高亮样式（github-dark 主题）

##### 新增组件
- `components/thread/MessageContent.tsx` - 消息内容渲染组件
- `components/thread/MessageContent.css` - 组件样式

##### 组件功能
- 代码块渲染：带语言标识、复制按钮、语法高亮
- 行内代码：背景高亮、等宽字体
- 图片渲染：懒加载、居中展示、悬停放大效果
- 表格渲染：响应式滚动、斑马纹样式
- 引用块：左侧边框、背景色区分
- 链接：新窗口打开、悬停下划线
- 深色主题适配

##### ThreadView.tsx 改动
- 导入 MessageContent 组件
- 已完成消息使用 MessageContent 渲染 Markdown
- 流式消息保持纯文本渲染（避免不完整 Markdown 导致渲染问题）

### 新增文件

| 文件 | 说明 |
|------|------|
| `components/thread/MessageContent.tsx` | 消息内容 Markdown 渲染组件 |
| `components/thread/MessageContent.css` | 组件样式文件 |

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `package.json` | 新增依赖 | react-markdown, remark-gfm, rehype-highlight, highlight.js |
| `components/thread/index.ts` | 修改 | 导出 MessageContent |
| `pages/ThreadView.tsx` | 修改 | 使用 MessageContent 渲染消息内容 |

### 渲染策略

| 消息类型 | 渲染方式 | 原因 |
|----------|----------|------|
| 已完成消息 | Markdown 渲染 | 内容完整，可正确解析 |
| 流式消息 | 纯文本渲染 | 内容可能不完整，避免解析错误 |

### 验证方法

1. 启动前端服务：`cd web && npm run dev`
2. 打开 Agent 调试页面，发送消息
3. 验证 AI 回复中的代码块有语法高亮和复制按钮
4. 验证 Markdown 元素（列表、表格、引用）正确渲染
5. 验证深色主题下样式正常

### 影响范围

- 前端：消息渲染组件、新增依赖
- 后端：无改动
- 数据：不影响现有数据

---

## 2026-03-23 Solo 模式与全栈工程师角色

### 背景

为了提供更纯净的用户体验，参考 Trae 的 Solo 模式，在开发对话框中增加沉浸式工作模式。

### 目标

1. 实现 Solo 模式：只保留消息区和输入框，隐藏其他界面元素
2. 新增全栈工程师系统预置角色
3. 全栈工程师角色自动触发 Solo 模式
4. 简化界面，移除不必要的控制元素
5. 美化 Solo 模式界面，提升用户体验

### 核心变更

#### 前端改动

##### ThreadView.tsx
- 新增 `soloMode` 状态控制界面显示模式
- 新增 Solo 模式顶部切换栏（工作流模式 / Solo 模式标签）
- Solo 模式下隐藏：左侧文件树、顶部干预控制栏、右侧产物侧边栏
- Solo 模式下保留：沙箱按钮（右上角）、消息区、输入框
- 全栈工程师角色进入调试时自动开启 Solo 模式
- 移除干预控制按钮（暂停/跳过/终止/重做）- 无实际使用场景
- 移除当前阶段状态显示（需求/设计/开发等阶段标签）
- Solo 模式顶部布局：左侧为模式切换标签，右侧为沙箱按钮
- 优化空状态提示样式

##### ThreadView.css
- 新增 `.solo-mode` 类样式，全屏垂直布局
- 新增 `.solo-mode-header` 顶部切换栏样式，浮动阴影效果
- 新增 `.solo-mode-tabs` / `.solo-mode-tab` 标签样式，胶囊式设计
- 新增 `.solo-mode-actions` / `.solo-mode-action-btn` 操作按钮样式
- Solo 模式下消息区居中显示，最大宽度 900px
- 消息气泡优化：更大圆角(16px)、微妙阴影、悬停动效
- 用户消息渐变背景，Agent 消息保持容器背景
- 输入框优化：更大圆角、聚焦发光效果、居中布局
- 新增 `.solo-mode-welcome` 欢迎提示样式

#### 后端改动

##### model/agent_config.go
- 新增 `AgentRoleFullstackEngineer` 角色常量

##### service/agent/config_service.go
- 新增 `InitSystemAgents()` 方法初始化系统预置角色
- 新增全栈工程师系统提示词（涵盖需求分析、架构设计、前后端开发、测试运维）

##### sql-change/migrations/202603230001_insert_fullstack_engineer.sql
- 新增全栈工程师角色的数据库迁移脚本

### 使用说明

1. **手动切换**：在 ThreadView 页面点击顶部 "Solo" 按钮进入 Solo 模式
2. **自动触发**：选择"全栈工程师"角色进入调试时自动进入 Solo 模式
3. **退出 Solo 模式**：点击顶部"退出"按钮或"工作流模式"标签
4. **沙箱操作**：在 Solo 模式下点击右上角"沙箱"按钮打开/关闭沙箱面板

---

## 2026-03-21 技能库完整功能实现

### 背景

项目需要实现完整的技能库管理功能，包括技能的CRUD、与Agent的绑定关系、技能包上传与管理、联邦技能源同步等核心能力。

### 目标

1. 实现技能数据的完整CRUD功能
2. 实现Agent与Skill的多对多绑定关系
3. 支持技能包的上传、仓库导入、联邦同步三种来源
4. 实现配置生成服务，支持技能安装到项目
5. 实现联邦技能源管理和知识库管理功能

### 核心变更

#### 后端改动

##### 数据库迁移
- 新增 `sql-change/migrations/202603210001_add_skill_tables.sql`
  - `skills` 表：技能基础信息、来源类型、支持的Agent、使用次数等
  - `agent_skill_bindings` 表：Agent与Skill的多对多绑定关系
- 新增 `sql-change/migrations/202603210002_add_skill_registries.sql`
  - `skill_registries` 表：联邦技能源配置
- 新增 `sql-change/migrations/202603210003_add_knowledge_bases.sql`
  - `knowledge_bases` 表：知识库配置

##### Skill 核心功能
- 新增 `internal/model/skill.go` - Skill、AgentSkillBinding、SkillRegistry、KnowledgeBase 模型
- 新增 `internal/repo/skill.go` - SkillRepository 数据访问层
- 新增 `internal/repo/agent_skill_binding.go` - AgentSkillBindingRepository
- 新增 `internal/service/skill/service.go` - Skill 业务逻辑层
- 新增 `internal/service/skill/usecount_updater.go` - 技能使用次数定时更新器
- 新增 `internal/api/skill_handler.go` - Skill API 处理器
  - 支持上传技能包（zip格式，解析skill.md）
  - 支持从Git仓库导入（GitHub/Gitee）
  - 支持从联邦源导入

##### 配置生成服务
- 新增 `internal/service/configgen/downloader.go` - 技能包下载器
- 新增 `internal/service/configgen/service.go` - 配置生成服务
- 新增 `internal/api/configgen_handler.go` - 配置生成API
- 支持将技能安装到项目的 `.claude/` 目录

##### 联邦技能源管理
- 新增 `internal/service/skill/registry_service.go` - Registry 业务逻辑
- 新增 `internal/api/registry_handler.go` - Registry API

##### 知识库管理
- 新增 `internal/repo/knowledge.go` - KnowledgeBase 数据访问层
- 新增 `internal/service/knowledge/service.go` - Knowledge 业务逻辑
- 新增 `internal/api/knowledge_handler.go` - Knowledge API

##### 配置扩展
- `pkg/config/config.go` 新增 SkillConfig 结构体
  - `use_count_update_interval` - 使用次数更新间隔
  - `upload_max_size` - 上传文件大小限制
  - `storage_path` - 技能包存储路径

#### 前端改动

##### 技能库页面 (`SkillLibrary/index.tsx`)
- 技能卡片列表展示，支持分页
- 新增/编辑技能弹窗，集成技能包上传
- 支持按标签、来源类型、Agent类型过滤
- 技能卡片显示：名称、描述、标签、支持的Agent、来源、使用次数
- 删除技能功能

##### 联邦技能源页面 (`RegistryManagement/index.tsx`)
- 注册表列表管理
- 新增/编辑/删除注册表
- 同步功能（单个/全部）

##### 知识库页面 (`KnowledgeManagement/index.tsx`)
- 知识库列表管理
- 新增/编辑/删除知识库

##### 布局调整
- `MainLayout.tsx` - 沙箱环境菜单移至设置子菜单
- `AgentConfig.tsx` - Agent编辑页面集成技能绑定
- 各页面间距优化

##### API 客户端
- `api/client.ts` 新增 skills、registries、knowledge API方法

##### 类型定义
- `types/index.ts` 新增 Skill、SkillRegistry、KnowledgeBase 类型

### 数据库表结构

#### skills 表
```sql
CREATE TABLE skills (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    tags JSON,
    source_type VARCHAR(50) NOT NULL,      -- platform/personal/federated
    source_registry_id VARCHAR(64),
    supported_agents JSON,
    version VARCHAR(50) DEFAULT '1.0.0',
    use_count INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active',
    is_public TINYINT DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

#### agent_skill_bindings 表
```sql
CREATE TABLE agent_skill_bindings (
    id VARCHAR(64) PRIMARY KEY,
    agent_role_id VARCHAR(64) NOT NULL,
    skill_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP,
    UNIQUE KEY (agent_role_id, skill_id),
    FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);
```

### API 端点

| 模块 | 方法 | 路径 | 说明 |
|------|------|------|------|
| Skills | GET | `/api/v1/skills` | 列出技能（支持分页、过滤） |
| Skills | POST | `/api/v1/skills` | 创建技能 |
| Skills | POST | `/api/v1/skills/upload` | 上传技能包 |
| Skills | POST | `/api/v1/skills/import/repo` | 从仓库导入 |
| Skills | POST | `/api/v1/skills/import/federated` | 从联邦源导入 |
| Skills | PUT | `/api/v1/skills/:id` | 更新技能 |
| Skills | DELETE | `/api/v1/skills/:id` | 删除技能（同时删除文件） |
| Agent-Skills | GET | `/api/v1/agent-skills/:agentId` | 获取Agent绑定的技能 |
| Agent-Skills | POST | `/api/v1/agent-skills/:agentId` | 绑定技能到Agent |
| Registries | GET/POST/PUT/DELETE | `/api/v1/registries` | 联邦技能源CRUD |
| Knowledge | GET/POST/PUT/DELETE | `/api/v1/knowledge` | 知识库CRUD |
| Config | POST | `/api/v1/projects/:id/config/sync` | 同步技能配置到项目 |

### 技能文件存储

- 存储路径：`{storage_path}/{skill_name}.zip`
- 删除技能时同步删除对应文件
- 支持平台、个人、联邦三种来源类型

### 修改文件统计

| 类型 | 数量 | 说明 |
|------|------|------|
| 新增文件 | 15 | 模型、Repository、Service、Handler |
| 修改文件 | 15 | 配置、前端页面、类型定义 |

### 提交记录

```
c3c4884 chore: 移除暂时不需要的 skill_favorites 表
5faddd1 chore: 更新项目配置文件
9955a61 feat: 技能库功能增强和UI优化
900afdc chore: 固定前后端端口并更新规范
5d4f7a1 feat: 添加联邦技能源和知识库功能
... (共27个提交)
```

### 验证方法

1. 启动后端服务：`cd isdp && go run ./cmd/server`
2. 启动前端服务：`cd web && npm run dev`
3. 访问技能库页面：http://localhost:3000/skills
4. 测试技能上传、编辑、删除功能
5. 测试Agent绑定技能功能
6. 测试联邦技能源管理功能

### 影响范围

- 后端：新增技能库、联邦源、知识库完整功能
- 前端：新增三个管理页面，Agent编辑页集成技能绑定
- 数据库：新增三张表

---

## 2026-03-20 邀请码认证系统与沙箱代理功能

### 背景

项目需要增加访问控制机制，防止未授权访问。同时沙箱服务需要通过后端代理访问，解决前端跨域问题。

### 目标

1. 实现邀请码认证中间件，支持配置化启用
2. 新增沙箱代理路由，解决前端跨域访问问题
3. 前端版本管理自动化
4. 清理无用文件，优化项目配置

### 核心变更

#### 后端改动

##### 邀请码认证中间件
- 新增 `internal/middleware/invite.go` - 邀请码认证中间件
- 功能特性：
  - 配置化启用：`auth.invite_code` 为空则放行
  - 会话管理：24小时有效期，自动清理过期会话
  - 公开路径放行：健康检查、静态资源、认证接口
  - 精美登录页面：内置 HTML/CSS/JS 单页面

##### 配置结构扩展
- `pkg/config/config.go` 新增 `AuthConfig` 结构体
- 支持通过 `config.yaml` 配置 `auth.invite_code`

##### 沙箱代理功能
- `internal/api/sandbox_handler.go` 新增代理路由
- 新增 `ProxySandbox` 方法：反向代理沙箱服务
- API 返回结构调整：
  - `url`: 代理URL（前端使用）
  - `localUrl`: 本地URL（调试使用）

#### 前端改动

##### BaseAgentSettings 页面优化
- OpenCode 类型显示特殊的 API URL 配置提示
- OpenCode 类型隐藏 API Token 字段
- 提示用户使用 `opencode providers login` 命令管理凭证

##### 版本管理自动化
- 新增 `web/scripts/generate-version.js` - 版本号自动生成脚本
- 新增 `web/src/config/version.ts` - 版本配置文件
- 构建时自动注入版本信息

#### 配置文件优化

##### config.yaml.example
- 新增 `auth` 配置段
- 数据库名默认改为 `dev_xx`，防止误用生产库
- 密码使用 `<密码>` 占位符

##### .gitignore
- 添加 `isdp-server` 编译产物忽略规则

#### 删除的文件

| 文件 | 说明 |
|------|------|
| `isdp/Dockerfile` | Docker 构建文件 |
| `isdp/docker-compose.yml` | Docker 编排文件 |
| `isdp/web/Dockerfile.dev` | 前端开发 Docker 文件 |
| `isdp/sql-change/migrations/.gitkeep` | 空文件 |

### 新增文件

| 文件 | 说明 |
|------|------|
| `isdp/internal/middleware/invite.go` | 邀请码认证中间件 |
| `isdp/web/scripts/generate-version.js` | 版本生成脚本 |
| `isdp/web/src/config/version.ts` | 版本配置 |
| `isdp/CLAUDE.md` | Claude Code 项目指南 |

### 修改文件统计

| 类型 | 数量 | 说明 |
|------|------|------|
| 新增文件 | 4 | 中间件、脚本、配置、文档 |
| 修改文件 | 10 | 后端API、配置、前端页面 |
| 删除文件 | 4 | Docker 相关、空文件 |

### 详细文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/internal/middleware/invite.go` | 新增 | 邀请码认证中间件 |
| `isdp/pkg/config/config.go` | 修改 | 新增 AuthConfig |
| `isdp/cmd/server/main.go` | 修改 | 集成邀请码中间件 |
| `isdp/internal/api/sandbox_handler.go` | 修改 | 新增代理路由 |
| `isdp/web/src/pages/Settings/BaseAgentSettings.tsx` | 修改 | OpenCode 特殊处理 |
| `isdp/web/scripts/generate-version.js` | 新增 | 版本生成脚本 |
| `isdp/web/src/config/version.ts` | 新增 | 版本配置 |
| `isdp/configs/config.yaml.example` | 修改 | 新增 auth 配置 |
| `isdp/.gitignore` | 修改 | 添加 isdp-server |
| `isdp/CLAUDE.md` | 新增 | Claude Code 指南 |
| `isdp/Dockerfile` | 删除 | 不再使用 Docker |
| `isdp/docker-compose.yml` | 删除 | 不再使用 Docker |
| `isdp/web/Dockerfile.dev` | 删除 | 不再使用 Docker |
| `isdp/sql-change/migrations/.gitkeep` | 删除 | 空文件 |

### 验证方法

1. 启动后端服务：`go run ./cmd/server`
2. 配置 `config.yaml` 中 `auth.invite_code` 为非空值
3. 访问任意页面，验证跳转到邀请码输入页面
4. 输入正确邀请码，验证进入系统
5. 测试沙箱代理：运行项目后通过代理 URL 访问沙箱服务

### 影响范围

- 后端：认证中间件、沙箱代理、配置结构
- 前端：BaseAgentSettings 页面、版本管理
- 配置：新增 auth 配置段
- 删除：Docker 相关文件

---

## 2026-03-19 项目清理与前端布局优化

### 背景

项目完成了从 SQLite 到 MySQL 的数据库迁移，需要清理迁移相关的临时文件。同时前端布局存在高度溢出问题，页面级滚动导致交互体验不佳。

### 目标

1. 清理数据库迁移相关的临时文件和脚本
2. 重构前端布局，解决 100vh 溢出问题
3. 优化前端样式，提取内联样式到 CSS 类
4. 修复潜在的空值引用错误

### 核心变更

#### 后端清理

##### 删除迁移相关文件
- 移除 `cmd/migrate/main.go` - SQLite 到 MySQL 数据迁移工具
- 移除 `isdp/DB_MIGRATION_GUIDE.md` 和 `isdp/docs/DB_MIGRATION_GUIDE.md` - 数据库迁移指南
- 删除 `scripts/` 目录下所有 SQL 脚本：
  - `init_db.sql`, `init_db_mysql.sql`, `init_db_sqlite.sql`
  - `initial_schema.sql`
  - `migrate.sh`, `schema.sh`
  - `202403160001_remove_model_name_field.sql`
  - `202403170003_remove_is_active_field.sql`
- 删除 `test-results/.last-run.json` - 测试结果缓存

##### 配置文件调整
- 删除旧配置 `configs/config.yaml`
- 新增 `configs/config.yaml` - 新版配置文件
- 新增 `configs/config.yaml.example` - 配置示例

##### SQL 变更目录
- 新增 `sql-change/` 目录，包含：
  - `README.md` - SQL 变更说明
  - `init_db_mysql.sql` - MySQL 初始化脚本
  - `migrations/` - 迁移脚本目录

#### 后端依赖升级

##### go.mod 更新
- 升级 SQLite 驱动：`modernc.org/sqlite` 1.29.10 → 1.47.0
- 新增 `github.com/mattn/go-sqlite3` 依赖
- 升级相关依赖版本（`golang.org/x/exp`, `golang.org/x/sys`, `modernc.org/*`）

##### MySQL 字符集修复
- 在 `db_mysql.go` 中添加 `SET NAMES utf8mb4`，确保中文字符正确存储和读取

#### 前端布局重构

##### 全局高度修复 (index.css)
```css
/* 修改前 */
#root {
  min-height: 100vh;
}

/* 修改后 */
html, body {
  height: 100%;
  margin: 0;
  padding: 0;
  overflow: hidden;
}
#root {
  height: 100%;
}
```

##### 主布局 Flex 改造 (MainLayout.tsx)
```tsx
// 修改前：整体页面滚动，内容区域有 margin
<Layout style={{ minHeight: '100vh' }}>
  <Content style={{ margin: 16, borderRadius: 8, padding: 24 }}>

// 修改后：分区独立滚动，Header 固定，Content 自适应
<Layout style={{ height: '100vh', overflow: 'hidden' }}>
  <Sider style={{ height: '100vh', overflow: 'auto' }} />
  <Layout style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
    <Header style={{ flexShrink: 0 }} />
    <Content style={{ flex: 1, margin: 0, padding: 16, overflow: 'auto' }} />
  </Layout>
</Layout>
```

#### 前端样式优化

##### SandboxPanel 组件重构
- 将内联样式提取为 CSS 类：
  - `.sandbox-control-bar` - 控制栏样式
  - `.sandbox-preview-bar` - 预览栏样式
  - `.sandbox-iframe-container` - iframe 容器样式
  - `.sandbox-empty` - 空状态样式
- 简化 `ThreadView.tsx` 中 @mention 列表项的交互，移除内联事件处理

##### CSS 文件更新
- `SandboxPanel.css` - 新增沙箱面板样式
- `ThreadView.css` - 新增 `.mention-list-item` 悬停样式
- `FileTree.css` - 文件树样式优化
- `MessageInput.css` - 消息输入组件样式优化

#### 前端 Bug 修复

##### Workflow 页面空值保护
```tsx
// 修改前：可能因 undefined 导致渲染错误
const templateAgents = agents.filter(a => template.agentIds?.includes(a.id));

// 修改后：添加空值默认值
const templateAgents = (agents || []).filter(a => template.agentIds?.includes(a.id));
```

涉及数组：`agents`, `workflowTemplates`，所有 `.map()` 和 `.filter()` 调用都添加了 `|| []` 保护。

### 修改文件统计

| 类型 | 数量 | 说明 |
|------|------|------|
| 删除文件 | 13 | 迁移工具、SQL脚本、旧配置 |
| 新增目录 | 2 | `configs/`, `sql-change/` |
| 修改文件 | 10 | 依赖升级、布局优化、样式重构 |

### 详细文件列表

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/cmd/migrate/main.go` | 删除 | 迁移工具 |
| `isdp/DB_MIGRATION_GUIDE.md` | 删除 | 迁移指南 |
| `isdp/docs/DB_MIGRATION_GUIDE.md` | 删除 | 迁移指南（重复） |
| `isdp/configs/config.yaml` | 删除 | 旧配置文件 |
| `isdp/scripts/*` | 删除 | 所有 SQL 脚本和 shell 脚本 |
| `isdp/test-results/.last-run.json` | 删除 | 测试缓存 |
| `isdp/go.mod` / `go.sum` | 修改 | 依赖升级 |
| `isdp/internal/repo/db_mysql.go` | 修改 | 字符集设置 |
| `isdp/web/src/index.css` | 修改 | 全局高度修复 |
| `isdp/web/src/layouts/MainLayout.tsx` | 修改 | Flex 布局改造 |
| `isdp/web/src/pages/ThreadView.tsx` | 修改 | 移除内联事件 |
| `isdp/web/src/pages/Workflow/index.tsx` | 修改 | 空值保护 |
| `isdp/web/src/components/thread/SandboxPanel.tsx` | 修改 | 样式类提取 |
| `isdp/web/src/components/thread/SandboxPanel.css` | 修改 | 新增样式 |
| `isdp/web/src/pages/ThreadView.css` | 修改 | mention 样式 |
| `isdp/web/src/components/FileTree/FileTree.css` | 修改 | 样式优化 |
| `isdp/web/src/components/thread/MessageInput.css` | 修改 | 样式优化 |

### 验证方法

1. 启动后端服务：`go run ./cmd/server`
2. 启动前端服务：`cd web && npm run dev`
3. 验证页面布局：
   - 整体页面不滚动
   - 侧边栏和内容区域独立滚动
   - Header 固定高度
4. 验证中文存储：创建/编辑包含中文的内容，刷新页面检查显示
5. 验证 Workflow 页面：无报错，正常显示 Agent 列表和模板列表

### 影响范围

- 后端：依赖版本、MySQL 字符集
- 前端：整体布局、样式结构
- 数据：不影响现有数据
- 配置：清理旧配置，新增示例配置

---

## 2026-03-18 Agent调试功能重构与状态管理优化

### 背景

Agent调试功能需要进行重构，将调试模式与工作流模式分离，并优化状态管理和线程安全。同时清理了冗余代码，统一了日志管理。

### 目标

1. 实现调试模式与工作流模式的独立状态管理
2. 增强调试线程管理的线程安全性
3. 重构WebSocket连接，简化接口并添加自动重连
4. 清理冗余代码，统一日志工具

### 核心变更

#### 后端改动

##### Orchestrator 调试功能增强
- 新增 `SpawnDebugAgent` 方法 - 调试模式启动Agent
- 新增 `ContinueDebugAgent` 方法 - 继续调试会话
- 新增 `SetDebugThreadManager` 方法 - 注入调试线程管理器

##### DebugThreadManager 线程安全增强
- 新增状态常量：`DebugThreadStatusIdle`, `DebugThreadStatusRunning`, `DebugThreadStatusCompleted`, `DebugThreadStatusError`
- 新增 `CompareAndSwapStatus` 方法 - 原子状态比较交换
- 新增 `TryStartExecution` 方法 - 原子启动执行
- 新增 `GetProjectPath` 方法 - 获取工作目录
- 新增 `ProjectPath` 字段 - 存储工作目录路径
- 使用 `sync.Once` 保护 `Stop()` 方法，防止多次调用 panic
- `GetMessages` 返回副本，避免外部修改影响内部状态

##### 日志工具统一
- 新增 `internal/service/agent/logger.go` - 统一的日志辅助函数
- 提供 `logInfo`, `logError`, `logDebug`, `logWarn` 等函数

#### 前端改动

##### 新增调试状态管理
- 新增 `web/src/store/debugThread.ts` - 调试模式专用 Zustand store
- 独立管理：threadId, status, messages, streamingContent, sandboxServer 等
- 与工作流模式状态完全隔离

##### WebSocket Hook 重构
- 简化接口签名：`useWebSocket(threadId, options)`
- 添加自动重连机制（默认 3 秒间隔）
- 添加 `onConnect`, `onDisconnect` 回调
- 新增 `disconnect` 方法用于主动断开

##### ThreadView 页面重构
- 支持调试模式和工作流模式分离
- 根据 URL 中的 `agentId` 参数判断模式
- 调试模式使用本地 WebSocket 状态
- 工作流模式使用全局 store 状态
- 新增沙箱侧边栏支持
- 新增 `ThreadView.css` 样式文件

##### 类型定义扩展
- 新增 `WSMessage`, `WSMessageDebug`, `WSMessageType` 类型
- 新增 `AgentOutputChunk`, `AgentMessage`, `SystemMessage` 类型
- 新增 `SandboxServer`, `SandboxReady` 类型

#### 删除的文件

| 文件 | 说明 |
|------|------|
| `internal/service/agent/session_manager.go` | 会话管理器，功能已整合到其他模块 |
| `internal/service/agent/interactive_session.go` | 交互式会话，功能已整合到其他模块 |
| `internal/service/agent/execution_context.go` | 部分代码删除，剩余整合到 execution_service |
| `cmd/test/test_opencode.go` | 测试文件移除 |
| `isdp/docs/*` | 设计文档移动到项目根目录 `docs/` |

### 修改文件统计

| 类型 | 数量 |
|------|------|
| 新增文件 | 4 |
| 修改文件 | 30+ |
| 删除文件 | 10+ |

### 提交记录

```
aa33c50 chore: allow debugThread.ts in gitignore
dabcce1 feat(web): add debug thread Zustand store
fb11f5d feat(web): add useWebSocket hook for real-time updates
6ef13df feat(web): add TypeScript types for debug functionality
ef5e40b feat(server): initialize DebugThreadManager in main
1f9bb10 refactor(api): inject DebugThreadManager into AgentHandler
90d8d5b fix: improve debug thread safety and preserve ProjectPath
5abbf26 feat(agent): add SpawnDebugAgent and ContinueDebugAgent to Orchestrator
6dfb6da fix: improve thread safety in GetMessages and add status constants
2fae2d2 fix: add sync.Once protection to Stop() to prevent panic on multiple calls
```

### 数据流

```
调试模式启动:
  前端 → api.agents.debug() → 后端 AgentHandler → DebugThreadManager.CreateThread()
      → Orchestrator.SpawnDebugAgent() → adapter.ExecuteWithStream()
      → WebSocket 广播流式输出 → 前端 debugThread store 更新

调试模式继续:
  前端 → api.agents.continueDebug() → Orchestrator.ContinueDebugAgent()
      → 获取最后 Agent 配置 → SpawnDebugAgent()
```

### 验证方法

1. 启动后端服务：`go run ./cmd/server`
2. 启动前端服务：`cd web && npm run dev`
3. 打开 Agent 调试页面
4. 选择一个 Agent 配置进行调试
5. 验证流式输出正常显示
6. 继续对话，验证上下文保持正确

### 影响范围

- 后端：Orchestrator, DebugThreadManager, AgentHandler
- 前端：ThreadView, useWebSocket, debugThread store
- 删除：session_manager, interactive_session 等冗余模块

---

## 2026-03-17 Agent执行功能重构与团队协作增强

### 背景

项目在团队协作开发中遇到了数据库变更同步困难的问题，需要建立一套完善的数据库版本控制机制。同时，对Agent执行与调试功能进行了重构，并对日志管理和无用文件进行了清理和优化。

### 目标

1. 实施数据库版本控制机制，确保变更同步
2. 重构Agent执行与调试的底层功能
3. 清理无用文件，优化项目结构
4. 建立日志管理规范

### 核心变更

#### Agent执行功能重构

##### 新增ExecutionService
- 创建 `internal/service/agent/execution_service.go` - 统一执行服务
- 整合 Orchestrator 和 InteractiveSession 的功能
- 实现 ExecutionContext（执行上下文）概念

##### Session管理增强
- 创建 `internal/service/agent/session_manager.go` - 会话管理器
- 创建 `internal/service/agent/execution_context.go` - 执行上下文定义

#### 数据库协作机制
- 创建 `scripts/migrate.sh` - 数据库迁移管理脚本
- 创建 `DB_MIGRATION_GUIDE.md` - 数据库迁移指南文档
- 实现数据库版本控制与团队协作方案

#### 日志管理改进
- 创建 `internal/config/logging.go` - 集中式日志配置
- 在 `cmd/server/main.go` 中集成日志管理功能
- 添加自动日志维护机制，定期清理过期日志

#### A2A交互协议完善
- 在 Orchestrator 中完善 Agent 路由验证逻辑
- 优化消息驱动的Agent交互流程
- 添加项目路径绑定功能，确保Agent在正确的工作目录下执行

#### 文件清理
删除了以下无用文件：
- `add_debug_project.go` - 调试项目添加脚本
- `add_debug_project.sql` - 调试项目SQL脚本
- `check_data.go` - 数据检查脚本
- `check_data2.go` - 数据检查脚本2
- `check_schema.go` - 模式检查脚本
- `check_current_schema.go` - 当前模式检查脚本
- `server.log` - 服务器日志文件
- `main.exe` - 旧可执行文件
- `server.exe` - 旧可执行文件

---

## 2026-03-17 修复项目路径绑定问题

### 背景

进行新项目开发时，Agent 的工作目录应该是绑定的项目路径，而不是当前工程路径。

### 问题

1. **SpawnRequest.ProjectPath 未设置**：`invocation_handler.go` 中的 Spawn 函数没有传递项目路径
2. **A2A 路由缺少项目路径**：`checkRouting` 和 `checkSignalRouting` 触发新 Agent 时没有传递项目路径
3. **用户消息触发缺少项目路径**：`SpawnAgentForUserMessage` 触发 Agent 时没有传递项目路径

### 目标

确保所有 Agent 触发场景都正确传递绑定项目的 `LocalPath` 作为工作目录。

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/api/invocation_handler.go` | 修改 | 添加 projectRepo 依赖，Spawn 时获取项目路径 |
| `internal/service/agent/orchestrator.go` | 修改 | 添加 projectRepo 依赖，多处 SpawnAgent 调用添加 ProjectPath |
| `cmd/server/main.go` | 修改 | 传递 projectRepo 给 InvocationHandler 和 Orchestrator |

### 详细改动

#### 1. InvocationHandler 添加项目路径获取

```go
// NewInvocationHandler 创建处理器
func NewInvocationHandler(orchestrator *agent.Orchestrator, mcpAuth *a2a.MCPAuthService, projectRepo *repo.ProjectRepository) *InvocationHandler

// Spawn 启动Agent
func (h *InvocationHandler) Spawn(c *gin.Context) {
    // 获取绑定的项目路径
    var projectPath string
    if h.projectRepo != nil {
        project, err := h.projectRepo.GetByThreadID(c.Request.Context(), threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    spawnReq := &agent.SpawnRequest{
        // ...
        ProjectPath: projectPath,
    }
}
```

#### 2. Orchestrator 添加 projectRepo 依赖

```go
type Orchestrator struct {
    // ...
    projectRepo *repo.ProjectRepository // 新增：项目仓库
}

func NewOrchestrator(..., projectRepo *repo.ProjectRepository, ...) *Orchestrator
```

#### 3. checkRouting 添加项目路径

```go
func (o *Orchestrator) checkRouting(...) {
    // 获取项目路径
    var projectPath string
    if o.projectRepo != nil {
        project, err := o.projectRepo.GetByThreadID(ctx, threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    o.SpawnAgent(ctx, &SpawnRequest{
        // ...
        ProjectPath: projectPath,
    })
}
```

#### 4. checkSignalRouting 添加项目路径

```go
func (o *Orchestrator) checkSignalRouting(...) {
    // 获取项目路径
    var projectPath string
    if o.projectRepo != nil {
        project, err := o.projectRepo.GetByThreadID(ctx, threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    o.SpawnAgent(ctx, &SpawnRequest{
        // ...
        ProjectPath: projectPath,
    })
}
```

#### 5. SpawnAgentForUserMessage 添加项目路径

```go
func (o *Orchestrator) SpawnAgentForUserMessage(...) error {
    // 获取项目路径
    var projectPath string
    if o.projectRepo != nil {
        project, err := o.projectRepo.GetByThreadID(ctx, threadID)
        if err == nil && project != nil {
            projectPath = project.LocalPath
        }
    }
    // 两处 SpawnAgent 调用都添加 ProjectPath
}
```

### 数据流

```
Thread → ProjectID → Project → LocalPath → SpawnRequest.ProjectPath
```

### 验证方法

1. 创建一个项目，设置 `local_path` 为目标开发目录
2. 创建 Thread 绑定该项目
3. 触发 Agent 执行
4. 验证 Agent 的工作目录为绑定的项目路径

### 影响范围

- 后端：Agent 触发逻辑
- 数据：不影响现有数据

---

## 2026-03-16 A2A @mention 路由验证功能

### 背景

当前 A2A @mention 路由存在以下问题：

1. **路由验证逻辑未生效**：`ValidateRouting` 和 `CanRouteTo` 配置存在但未被调用
2. **路由范围不受控**：Agent 可以 @mention 任意角色，不受工作流模板限制
3. **重复代码**：`getAllowedRoutes` 和 `getDefaultRouting` 定义了相同的路由规则

### 目标

修复 @mention 路由验证逻辑，使 Agent 只能路由到工作流模板中已配置的 Agent 实例。支持两种格式：
- `@role`（角色别名，如 `@developer`）
- `@agent-name`（实例名称，如 `@前端开发`）

### 设计文档

`isdp/docs/superpowers/specs/2026-03-16-a2a-mention-routing-validation-design.md`

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/service/agent/orchestrator.go` | 修改 | 修改 `parseMentions`、`checkRouting`，新增辅助函数 |
| `internal/service/a2a/mention_parser.go` | 删除 | 整个文件可删除，代码未被使用 |

### 详细改动

#### 1. 新增数据结构

**File:** `internal/service/agent/orchestrator.go`

```go
// ParsedMention @mention 解析结果
type ParsedMention struct {
    Role      model.AgentRole // 角色类型（可能为空）
    AgentName string          // Agent 实例名称（可能为空）
    Raw       string          // 原始 mention 文本
}
```

#### 2. 修改 parseMentions 函数

**改动:** 返回类型从 `[]model.AgentRole` 改为 `[]ParsedMention`

```go
// 修改前
func parseMentions(content string) []model.AgentRole

// 修改后
func (o *Orchestrator) parseMentions(content string) []ParsedMention
```

#### 3. 修改 checkRouting 函数

**核心逻辑变更:**

```go
// 修改前：直接按 role 触发，无验证
for _, role := range mentions {
    if role != "" {
        o.SpawnAgent(ctx, &SpawnRequest{
            ThreadID: threadID,
            Role:     role,
            Input:    output,
        })
    }
}

// 修改后：验证目标是否在工作流模板中
allowedAgents := o.getAllowedAgentsFromWorkflow(ctx, threadID)
for _, mention := range mentions {
    var targetConfig *model.AgentRoleConfig
    if mention.Role != "" {
        targetConfig = o.findAgentByRole(allowedAgents, mention.Role)
    } else {
        targetConfig = o.findAgentByName(allowedAgents, mention.AgentName)
    }
    if targetConfig == nil {
        logInfo("路由被拒绝：目标不在工作流模板中", ...)
        continue
    }
    o.SpawnAgent(ctx, &SpawnRequest{
        ThreadID: threadID,
        ConfigID: targetConfig.ID,
        Role:     targetConfig.Role,
        Input:    output,
    })
}
```

#### 4. 新增辅助函数

```go
// getAllowedAgentsFromWorkflow 从工作流模板获取允许路由的 Agent 列表
// 数据流: Thread → WorkflowTemplate → AgentIDs → AgentConfigs
func (o *Orchestrator) getAllowedAgentsFromWorkflow(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig

// findAgentByRole 在 Agent 列表中按角色查找
func (o *Orchestrator) findAgentByRole(agents []*model.AgentRoleConfig, role model.AgentRole) *model.AgentRoleConfig

// findAgentByName 在 Agent 列表中按名称查找
func (o *Orchestrator) findAgentByName(agents []*model.AgentRoleConfig, name string) *model.AgentRoleConfig

// checkSignalRouting 检查信号路由（原有逻辑提取）
func (o *Orchestrator) checkSignalRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string)
```

#### 5. 删除未使用代码

**File:** `internal/service/a2a/mention_parser.go` - 整个文件删除

删除内容：
- `MentionParser` 结构体和 `NewMentionParser` 函数
- `ParsedMention` 结构体（旧版，字段为 `{Role, Content}`）
- `ParseMentions` 方法
- `ParseAgentRole` 函数（与 `orchestrator.go` 中的重复）
- `ExtractRouting` 方法和 `RoutingInfo` 结构体
- `ValidateRouting` 方法
- `getAllowedRoutes` 函数（与 `config_service.go` 中的 `getDefaultRouting` 重复）
- `FormatMention` 函数和 `roleToString` 函数

### 数据流

```
Agent 执行完成 → checkRouting() → parseMentions(output)
    ↓
getAllowedAgentsFromWorkflow(threadID)
    → threadRepo.FindByID(threadID)
    → workflowRepo.FindByID(templateID)
    → configSvc.GetByID() for each agent ID
    ↓
匹配 @mention 与 allowedAgents
    → findAgentByRole() 或 findAgentByName()
    ↓
SpawnAgent(ConfigID: targetConfig.ID)
```

### 边界情况处理

| 场景 | 处理方式 |
|------|----------|
| Thread 未绑定工作流模板 | 返回 nil，所有 @mention 被记录为"路由被拒绝"并跳过 |
| @mention 角色不在模板中 | 记录日志"路由被拒绝：目标不在工作流模板中"，跳过 |
| @mention 名称不在模板中 | 记录日志"路由被拒绝：目标不在工作流模板中"，跳过 |
| Agent 配置被删除 | `GetByID` 失败，跳过该 Agent |
| 工作流模板 AgentIDs 为空 | 返回 nil，所有 @mention 被记录为"路由被拒绝"并跳过 |

### 回退方法

如需回退此功能：

1. 恢复 `orchestrator.go` 中的 `parseMentions` 函数为返回 `[]model.AgentRole`
2. 恢复 `checkRouting` 函数为原来的直接触发逻辑
3. 删除新增的辅助函数：`getAllowedAgentsFromWorkflow`、`findAgentByRole`、`findAgentByName`、`checkSignalRouting`
4. 删除新增的 `ParsedMention` 结构体
5. 恢复 `a2a/mention_parser.go` 文件（如需）

### 验证方法

1. 启动服务，创建一个绑定了工作流模板的 Thread
2. 让 Agent 输出 `@前端开发 请实现登录页面`
3. 验证：
   - 如果"前端开发"在模板中 → 触发该 Agent
   - 如果"前端开发"不在模板中 → 日志记录"路由被拒绝"
4. 测试 `@developer` 等角色别名格式

### 影响范围

- 后端：`orchestrator.go` 路由逻辑
- 删除：`a2a/mention_parser.go` 未使用代码
- 数据：不影响现有数据

---

## 2026-03-15 工作流阶段配置改为Agent实例选择

### 背景

工作流页面的"阶段配置"原来选择的是阶段名称（需求分析、架构设计等），但实际应该配置的是具体的 Agent 实例。

### 目标

将"阶段配置"改为选择 Agent 实例，Agent 实例从后端 API 动态获取。

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `isdp/web/src/pages/Workflow/index.tsx` | 修改 | 主要修改文件 |

### 详细改动

#### 1. 新增导入

```tsx
import React, { useState, useEffect } from 'react';
// 新增 Spin 组件
import { ..., Spin } from 'antd';
// 新增 API 客户端
import { api } from '@/api/client';
// 新增类型导入
import type { AgentConfig } from '@/types';
import { AgentRoleLabels } from '@/types';
```

#### 2. 接口定义修改

**WorkflowTemplate 接口**：

```typescript
// 修改前
interface WorkflowTemplate {
  phases: string[];  // 阶段名称列表
  ...
}

// 修改后
interface WorkflowTemplate {
  agentIds: string[];  // Agent 实例 ID 列表
  ...
}
```

#### 3. 新增状态和 API 调用

```tsx
const [agents, setAgents] = useState<AgentConfig[]>([]);
const [loadingAgents, setLoadingAgents] = useState(false);

useEffect(() => {
  setLoadingAgents(true);
  api.agents.list()
    .then(setAgents)
    .catch((error) => {
      console.error('Failed to fetch agents:', error);
      message.error('获取Agent列表失败');
    })
    .finally(() => setLoadingAgents(false));
}, []);
```

#### 4. 删除硬编码数据

删除了静态的 `agentRoles` 数组，改为从 API 动态获取：

```tsx
// 已删除
const agentRoles = [
  { id: 'requirement', name: '需求分析师', ... },
  { id: 'architect', name: '架构师', ... },
  ...
];
```

#### 5. 模板数据结构调整

```tsx
// 修改前
const workflowTemplates = [
  {
    id: 'standard',
    phases: ['需求分析', '架构设计', '代码实现', ...],
    ...
  }
];

// 修改后
const workflowTemplates = [
  {
    id: 'standard',
    agentIds: [], // 将根据角色动态匹配
    ...
  }
];
```

#### 6. 表单字段修改

```tsx
// 修改前
<Form.Item name="phases" label="阶段配置">
  <Select mode="multiple">
    <Option value="requirement">需求分析</Option>
    ...
  </Select>
</Form.Item>

// 修改后
<Form.Item name="agentIds" label="Agent配置">
  <Select mode="multiple" loading={loadingAgents}>
    {agents.map((agent) => (
      <Option key={agent.id} value={agent.id}>
        {agent.name} ({AgentRoleLabels[agent.role]})
      </Option>
    ))}
  </Select>
</Form.Item>
```

#### 7. UI 显示更新

- 模板卡片：从显示"阶段流程"改为显示"Agent配置"
- 右侧卡片：标题从"Agent 角色"改为"Agent 实例"
- Agent 列表：从静态数据改为动态数据，增加了加载状态和空状态处理

### 数据流变化

```
修改前:
  硬编码 agentRoles → 渲染 UI

修改后:
  API /agents → agents state → 渲染 UI
```

### 验证方法

1. 启动前后端服务
2. 打开工作流页面 http://localhost:3004/workflow
3. 点击"自定义工作流"按钮
4. 验证"Agent配置"下拉列表显示从后端获取的 Agent 实例
5. 选择多个 Agent 实例并提交表单

### 影响范围

- 仅影响前端 Workflow 页面
- 不涉及后端 API 修改
- 不影响其他页面功能

### 备注

- 预设工作流模板的 `agentIds` 目前为空数组，后续可根据实际业务需求进行配置
- Agent 实例数据来自 `api.agents.list()` 接口，返回 `AgentConfig[]` 类型数据

---

## 2026-03-15 工作流模板持久化功能实现

### 背景

工作流创建后未正常保存，`handleCreateWorkflow` 函数只是打印日志，没有实际调用后端 API 保存数据。需要实现完整的工作流模板持久化功能。

### 目标

实现工作流模板的完整 CRUD 功能：
- 后端：创建模型、Repository、Service、Handler
- 前端：API 客户端、页面交互

### 新增文件

| 文件 | 说明 |
|------|------|
| `internal/model/workflow_template.go` | 工作流模板数据模型 |
| `internal/repo/workflow_template.go` | 工作流模板数据访问层 |
| `internal/service/workflow/service.go` | 工作流模板业务逻辑层 |
| `internal/api/workflow_handler.go` | 工作流模板 API 处理器 |

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `cmd/server/main.go` | 修改 | 添加 workflow 相关初始化和路由注册 |
| `web/src/api/client.ts` | 修改 | 添加 workflows API 方法 |
| `web/src/api/transform.ts` | 修改 | 添加 workflow 数据转换函数 |
| `web/src/types/index.ts` | 修改 | 添加 WorkflowTemplate 类型定义 |
| `web/src/pages/Workflow/index.tsx` | 修改 | 使用 API 实现模板的增删改查 |

### 详细改动

#### 1. 后端模型定义 (workflow_template.go)

```go
type WorkflowTemplate struct {
    ID            uuid.UUID       `json:"id"`
    Name          string          `json:"name"`
    Description   string          `json:"description"`
    AgentIDs      json.RawMessage `json:"agent_ids"`      // Agent实例ID列表
    Checkpoints   json.RawMessage `json:"checkpoints"`    // 人工检查点列表
    EstimatedTime string          `json:"estimated_time"`
    IsSystem      bool            `json:"is_system"`      // 是否系统预设
    CreatedAt     time.Time       `json:"created_at"`
    UpdatedAt     time.Time       `json:"updated_at"`
}
```

#### 2. 数据库表结构 (main.go)

```sql
CREATE TABLE IF NOT EXISTS workflow_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids TEXT DEFAULT '[]',
    checkpoints TEXT DEFAULT '[]',
    estimated_time TEXT,
    is_system INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### 3. API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/workflows` | 获取所有工作流模板 |
| POST | `/api/v1/workflows` | 创建工作流模板 |
| GET | `/api/v1/workflows/:id` | 获取单个工作流模板 |
| PUT | `/api/v1/workflows/:id` | 更新工作流模板 |
| DELETE | `/api/v1/workflows/:id` | 删除工作流模板（仅非系统模板） |

#### 4. 前端 API 客户端 (client.ts)

```typescript
workflows = {
  list: (): Promise<WorkflowTemplate[]> => this.request('/workflows', 'GET'),
  get: (id: string): Promise<WorkflowTemplate> => this.request(`/workflows/${id}`, 'GET'),
  create: (data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
    this.request('/workflows', 'POST', data),
  update: (id: string, data: Partial<WorkflowTemplate>): Promise<WorkflowTemplate> =>
    this.request(`/workflows/${id}`, 'PUT', data),
  delete: (id: string): Promise<void> => this.request(`/workflows/${id}`, 'DELETE'),
};
```

#### 5. 前端类型定义 (types/index.ts)

```typescript
export interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  agentIds: string[];
  checkpoints: string[];
  estimatedTime: string;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
}
```

#### 6. 页面交互改进

- 从 API 获取工作流模板列表，替代硬编码数据
- 创建工作流时调用 `api.workflows.create()` 保存到后端
- 添加删除功能，支持删除非系统预设模板
- 添加加载状态和提交状态显示
- 系统预设模板显示"系统预设"标签，不可删除

### 系统预设模板

服务启动时自动初始化 4 个系统预设模板：

1. **标准开发流程** - 完整的软件开发流程，从需求到部署
2. **快速原型流程** - 快速构建原型，验证想法
3. **代码重构流程** - 优化现有代码结构和质量
4. **问题修复流程** - 快速定位和修复问题

### 数据流

```
创建工作流:
  前端表单 → api.workflows.create() → 后端 Handler → Service → Repository → SQLite

获取工作流列表:
  页面加载 → api.workflows.list() → 后端 Handler → Service → Repository → 返回数据

删除工作流:
  点击删除 → Popconfirm确认 → api.workflows.delete() → 后端 Handler → Service → Repository
```

### 验证方法

1. 启动后端服务：`go run ./cmd/server`
2. 启动前端服务：`cd web && npm run dev`
3. 打开工作流页面 http://localhost:3004/workflow
4. 验证页面显示系统预设的 4 个工作流模板
5. 点击"自定义工作流"，填写表单并提交
6. 验证新创建的工作流模板出现在列表中
7. 刷新页面，验证数据持久化成功
8. 测试删除功能，验证非系统模板可删除

### 影响范围

- 后端：新增工作流模板相关的完整 CRUD 功能
- 前端：Workflow 页面实现完整的增删改查交互
- 数据库：新增 `workflow_templates` 表

### 备注

- 系统预设模板（`is_system = true`）不可删除
- 删除操作有二次确认（Popconfirm）
- 表单提交有防重复提交保护（`submitting` 状态）

---

## 2026-03-15 工作流模板功能Bug修复

### 背景

工作流编排页面打开报错，创建自定义工作流也报错。经排查发现以下问题：

1. **JSON字段存储问题**：`json.RawMessage` 类型未正确存储到 SQLite
2. **空值处理问题**：前端 transform 函数未正确处理 `null` 值
3. **布尔值转换问题**：SQLite 存储 `is_system` 为 INTEGER (0/1)，但前端期望布尔值
4. **系统模板重复初始化**：服务重启时可能重复创建系统预设模板

### 目标

修复以上问题，确保工作流模板功能正常运行。

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/repo/workflow_template.go` | 修复 | JSON字段存储转换为 `[]byte` |
| `internal/service/workflow/service.go` | 修复 | 添加系统模板存在性检查 |
| `web/src/api/transform.ts` | 修复 | 增强空值处理和布尔值转换 |

### 详细改动

#### 1. Repository JSON字段存储修复

**问题**：`json.RawMessage` 直接传递给 SQL Exec 时存储失败

**修复**：转换为 `[]byte` 后存储

```go
// 修改前
_, err := r.db.ExecContext(ctx, query,
    template.AgentIDs,    // json.RawMessage
    template.Checkpoints, // json.RawMessage
    ...
)

// 修改后
_, err := r.db.ExecContext(ctx, query,
    []byte(template.AgentIDs),      // 转换为 []byte
    []byte(template.Checkpoints),   // 转换为 []byte
    ...
)
```

#### 2. Service 系统模板初始化修复

**问题**：服务重启时重复创建系统预设模板

**修复**：初始化前检查是否已存在系统模板

```go
func (s *Service) InitSystemTemplates(ctx context.Context) error {
    // 先检查是否已有系统模板
    existingTemplates, err := s.repo.FindAll(ctx)
    if err != nil {
        return err
    }

    // 如果已有系统模板，跳过初始化
    for _, t := range existingTemplates {
        if t.IsSystem {
            return nil
        }
    }

    // 创建系统模板...
}
```

#### 3. 前端 Transform 函数修复

**问题**：
- `agentIds` 和 `checkpoints` 可能为 `null`，导致前端解析失败
- `isSystem` 从后端返回为数字 `0/1`，前端期望布尔值

**修复**：增强空值处理和类型转换

```typescript
export function transformWorkflowTemplate(data: any): any {
  if (!data) return data;
  const result = snakeToCamel(data);

  // 确保 agentIds 是数组
  if (result.agentIds == null) {
    result.agentIds = [];
  } else if (typeof result.agentIds === 'string') {
    try {
      result.agentIds = JSON.parse(result.agentIds);
    } catch {
      result.agentIds = [];
    }
  }

  // 确保 checkpoints 是数组
  if (result.checkpoints == null) {
    result.checkpoints = [];
  } else if (typeof result.checkpoints === 'string') {
    try {
      result.checkpoints = JSON.parse(result.checkpoints);
    } catch {
      result.checkpoints = [];
    }
  }

  // 确保 isSystem 是布尔值
  if (typeof result.isSystem === 'number') {
    result.isSystem = result.isSystem === 1;
  }

  return result;
}
```

### 修复前后对比

| 问题 | 修复前 | 修复后 |
|------|--------|--------|
| 创建工作流 | 报错，数据未保存 | 正常创建并保存 |
| 页面加载 | 报错，无法显示模板 | 正常显示所有模板 |
| 服务重启 | 可能产生重复模板 | 跳过已存在的模板 |
| 系统模板标识 | 显示为数字 | 正确显示为布尔值 |

### 验证方法

1. 重新编译后端：`go build -o bin/server.exe ./cmd/server`
2. 重启后端服务
3. 重启前端服务：`cd web && npm run dev`
4. 打开工作流页面，验证无报错
5. 创建自定义工作流，验证保存成功
6. 刷新页面，验证数据持久化

### 影响范围

- 后端：JSON字段存储逻辑
- 前端：数据转换逻辑
- 数据：现有数据不受影响

---
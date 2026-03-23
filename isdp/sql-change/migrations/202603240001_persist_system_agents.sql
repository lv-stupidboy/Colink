-- 固化系统预置 Agent 角色配置
-- 执行时间: 2026-03-24
-- 说明: 将所有内置 Agent 角色标记为系统预置，初始化基础 Agent 配置及工作流模板

SET NAMES utf8mb4;

-- ============================================
-- 1. 更新所有内置 Agent 角色的 is_system 标志
-- ============================================

-- 更新所有现有内置角色的 is_system 标志
UPDATE agent_configs SET is_system = 1 WHERE role IN (
    'requirement_analyst', 'architect', 'frontend_developer',
    'backend_developer', 'test_engineer', 'sre_engineer',
    'code_reviewer', 'project_manager', 'ui_designer',
    'database_designer', 'security_engineer', 'tech_writer',
    'fullstack_engineer'
);

-- ============================================
-- 2. 初始化基础 Agent 配置 (Claude CLI)
-- ============================================

-- 清理可能存在的旧数据
DELETE FROM base_agents WHERE id = 'claude-cli-default';

-- 插入默认 Claude CLI 配置
INSERT INTO base_agents (id, name, type, default_model, cli_path, max_tokens, timeout_minutes, created_at, updated_at)
VALUES (
    'claude-cli-default',
    'Claude CLI (默认)',
    'claude',
    'claude-sonnet-4-6',
    'claude',
    4096,
    30,
    NOW(),
    NOW()
);

-- ============================================
-- 3. 确保 fullstack_engineer 存在（如果之前未插入）
-- ============================================

-- 使用 INSERT IGNORE 避免重复插入
INSERT IGNORE INTO agent_configs (id, name, role, description, system_prompt, model_name, max_tokens, temperature, is_default, is_system, created_at, updated_at)
SELECT UUID(), '全栈工程师', 'fullstack_engineer', '全栈开发工程师，能独立完成从需求分析、架构设计、前后端开发到测试部署的完整项目开发流程',
'你是一位资深的全栈工程师，具备完整的项目开发能力，能够独立完成从需求到上线的全部工作。

## 核心能力

### 1. 需求分析
- 理解业务需求，转化为技术方案
- 识别核心功能和边界条件
- 定义功能优先级和验收标准

### 2. 架构设计
- 系统架构设计：整体架构、模块划分、接口设计
- 技术选型：框架、中间件、数据库选择
- 数据建模：ER图设计、表结构设计

### 3. 前端开发
- React/Vue/Next.js 等现代前端框架开发
- 响应式布局、组件封装
- 状态管理、API 对接
- Tailwind CSS、Ant Design 等 UI 库使用

### 4. 后端开发
- Go/Node.js/Python 等后端语言开发
- RESTful/GraphQL API 设计与实现
- 数据库操作、缓存策略
- 消息队列、定时任务

### 5. 测试与质量
- 单元测试、集成测试编写
- 测试用例设计
- 代码审查、性能优化

### 6. 部署运维
- Docker 容器化
- CI/CD 流程配置
- 日志监控、故障排查

## 工作流程

### 接收任务时
1. 分析需求，确认理解无误
2. 制定开发计划，拆分任务
3. 设计技术方案

### 开发过程中
1. 按照最佳实践编写代码
2. 保持代码整洁、注释清晰
3. 编写必要的测试用例
4. 及时提交进度更新

### 完成任务后
1. 自测功能是否正常
2. 检查代码质量
3. 编写简要的完成说明

## 代码规范

### 通用规范
- 使用有意义的变量和函数命名
- 保持函数单一职责
- 添加必要的注释
- 遵循项目既有的代码风格

### 前端规范
- 组件化开发，保持组件可复用
- 合理使用状态管理
- 注意性能优化（懒加载、缓存等）

### 后端规范
- RESTful API 设计规范
- 统一的错误处理
- 合理的日志记录
- SQL 注入、XSS 等安全防护

## 输出标准

### 需求分析阶段
- 需求文档：功能描述、用户故事、验收标准
- 技术方案：架构图、技术选型、风险评估

### 开发阶段
- 代码实现：结构清晰、注释完整
- API 文档：接口说明、请求响应示例
- 数据库脚本：建表语句、索引设计

### 测试阶段
- 测试用例：覆盖主要场景
- 测试报告：通过情况、遗留问题

### 部署阶段
- 部署文档：部署步骤、配置说明
- 运维手册：监控配置、故障处理

## 完成标志
完成任务后，在输出末尾明确标注：【开发完成】',
'claude-sonnet-4-6', 4096, 0.7, 0, 1, NOW(), NOW();

-- ============================================
-- 4. 初始化系统工作流模板
-- ============================================

-- 清理可能存在的旧系统模板
DELETE FROM workflow_templates WHERE is_system = 1;

-- 插入系统工作流模板
-- 标准开发流程
INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at)
SELECT UUID(), '标准开发流程', '完整的软件开发流程，从需求到部署',
'[]', '{}', '["需求确认", "方案确认", "代码合入", "部署确认"]',
'2-4小时', 1, 0, NOW(), NOW();

-- 快速原型流程
INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at)
SELECT UUID(), '快速原型流程', '快速构建原型，验证想法',
'[]', '{}', '["需求确认"]',
'30分钟-1小时', 1, 0, NOW(), NOW();

-- 代码重构流程
INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at)
SELECT UUID(), '代码重构流程', '优化现有代码结构和质量',
'[]', '{}', '["方案确认", "代码合入"]',
'1-3小时', 1, 0, NOW(), NOW();

-- 问题修复流程
INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, created_at, updated_at)
SELECT UUID(), '问题修复流程', '快速定位和修复问题',
'[]', '{}', '["修复确认"]',
'30分钟-2小时', 1, 0, NOW(), NOW();

-- ============================================
-- 回滚语句（如需回滚执行以下语句）
-- ============================================
-- UPDATE agent_configs SET is_system = 0 WHERE role IN (
--     'requirement_analyst', 'architect', 'frontend_developer',
--     'backend_developer', 'test_engineer', 'sre_engineer',
--     'code_reviewer', 'project_manager', 'ui_designer',
--     'database_designer', 'security_engineer', 'tech_writer',
--     'fullstack_engineer'
-- );
-- DELETE FROM base_agents WHERE id = 'claude-cli-default';
-- DELETE FROM agent_configs WHERE role = 'fullstack_engineer';
-- DELETE FROM workflow_templates WHERE is_system = 1;
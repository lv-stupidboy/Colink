-- auto-test/internal/testdata/init-sqlite.sql
-- 测试数据库初始化脚本
-- 包含最小化的测试数据集

-- 清空现有数据（测试环境）
DELETE FROM base_agents;
DELETE FROM projects;
DELETE FROM threads;
DELETE FROM agent_configs;
DELETE FROM workflow_templates;

-- 插入测试基础 Agent
INSERT INTO base_agents (id, name, type, description, created_at, updated_at) VALUES
('test-base-001', 'Claude Code', 'claude_code', 'Claude CLI 适配器', datetime('now'), datetime('now')),
('test-base-002', 'Backend Developer', 'claude_code', '后端开发 Agent', datetime('now'), datetime('now')),
('test-base-003', 'Architect', 'claude_code', '架构师 Agent', datetime('now'), datetime('now'));

-- 插入测试项目
INSERT INTO projects (id, name, description, created_at, updated_at) VALUES
('test-proj-001', '测试项目', '用于 E2E 测试的项目', datetime('now'), datetime('now'));

-- 插入测试线程
INSERT INTO threads (id, project_id, title, status, created_at, updated_at) VALUES
('test-thread-001', 'test-proj-001', '测试线程', 'active', datetime('now'), datetime('now'));

-- 插入测试 Agent 配置
INSERT INTO agent_configs (id, project_id, name, base_agent_id, description, created_at, updated_at) VALUES
('test-agent-001', 'test-proj-001', 'Backend Developer', 'test-base-002', '后端开发测试配置', datetime('now'), datetime('now')),
('test-agent-002', 'test-proj-001', 'Architect', 'test-base-003', '架构师测试配置', datetime('now'), datetime('now'));
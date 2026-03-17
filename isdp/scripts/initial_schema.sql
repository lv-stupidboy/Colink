-- ISDP Database Schema
-- Initial schema for database version control

PRAGMA foreign_keys = ON;

-- Table: base_agents
-- 存储基础Agent配置（如Claude Code、Open Code等）
CREATE TABLE base_agents (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    api_url TEXT,
    api_token TEXT,
    default_model TEXT,
    cli_path TEXT,
    git_bash_path TEXT,
    max_tokens INTEGER,
    timeout_minutes INTEGER NOT NULL DEFAULT 30,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Table: projects
-- 项目表，存储用户创建的各个开发项目
CREATE TABLE projects (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT NOT NULL,
    local_path TEXT NOT NULL,
    git_repo TEXT,
    config BLOB,
    workflow_template_id TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (workflow_template_id) REFERENCES workflow_templates(id) ON DELETE SET NULL
);

-- Table: threads
-- 会话表，表示开发过程中的单次会话
CREATE TABLE threads (
    id TEXT PRIMARY KEY NOT NULL,
    project_id TEXT NOT NULL,
    status TEXT NOT NULL,
    current_phase TEXT NOT NULL,
    current_agent TEXT,
    depth INTEGER NOT NULL DEFAULT 0,
    abort_token TEXT,
    workflow_template_id TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (workflow_template_id) REFERENCES workflow_templates(id) ON DELETE SET NULL
);

-- Table: messages
-- 消息表，存储用户和AI代理之间的对话
CREATE TABLE messages (
    id TEXT PRIMARY KEY NOT NULL,
    thread_id TEXT NOT NULL,
    role TEXT NOT NULL,
    agent_id TEXT,
    content TEXT NOT NULL,
    message_type TEXT NOT NULL,
    metadata BLOB,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

-- Table: agent_configs
-- Agent配置表，定义不同类型的AI Agent及其行为
CREATE TABLE agent_configs (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    base_agent_id TEXT,
    description TEXT,
    system_prompt TEXT NOT NULL,
    model_name TEXT,
    max_tokens INTEGER,
    temperature REAL NOT NULL DEFAULT 0.7,
    routing_config BLOB,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (base_agent_id) REFERENCES base_agents(id) ON DELETE SET NULL
);

-- Table: agent_invocations
-- Agent调用记录表，记录每次Agent的调用情况
CREATE TABLE agent_invocations (
    id TEXT PRIMARY KEY NOT NULL,
    thread_id TEXT NOT NULL,
    agent_config_id TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    input TEXT NOT NULL,
    output TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_config_id) REFERENCES agent_configs(id) ON DELETE CASCADE
);

-- Table: workflow_templates
-- 工作流模板表，定义不同的开发工作流模式
CREATE TABLE workflow_templates (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids BLOB NOT NULL,
    checkpoints BLOB,
    estimated_time TEXT,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Table: artifacts
-- 工作产物表，存储开发过程中产生的文件和内容
CREATE TABLE artifacts (
    id TEXT PRIMARY KEY NOT NULL,
    thread_id TEXT NOT NULL,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    file_path TEXT,
    artifact_type TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

-- Table: sandboxes
-- 沙箱环境表，用于隔离开发环境
CREATE TABLE sandboxes (
    id TEXT PRIMARY KEY NOT NULL,
    thread_id TEXT NOT NULL,
    config BLOB,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX idx_projects_created_at ON projects(created_at);
CREATE INDEX idx_threads_project_id ON threads(project_id);
CREATE INDEX idx_threads_created_at ON threads(created_at);
CREATE INDEX idx_messages_thread_id ON messages(thread_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX idx_agent_invocations_created_at ON agent_invocations(created_at);
CREATE INDEX idx_artifacts_thread_id ON artifacts(thread_id);
CREATE INDEX idx_sandboxes_thread_id ON sandboxes(thread_id);

-- Insert default workflow templates
INSERT INTO workflow_templates (id, name, description, agent_ids, checkpoints, estimated_time, is_system, is_default, created_at, updated_at)
VALUES
('00000000-0000-0000-0000-000000000001', '标准软件开发流程', '包含需求分析、架构设计、开发、测试、评审的标准流程', '[]', '[]', '2-4小时', 1, 1, datetime('now'), datetime('now'));

-- Insert default agent configurations
INSERT INTO agent_configs (id, name, role, description, system_prompt, is_default, created_at, updated_at)
VALUES
('11111111-1111-1111-1111-111111111111', '需求分析师', 'requirement', '负责分析和澄清用户需求', '你是专业的软件需求分析师...', 1, datetime('now'), datetime('now')),
('22222222-2222-2222-2222-222222222222', '系统架构师', 'architect', '负责设计系统架构', '你是经验丰富的系统架构师...', 1, datetime('now'), datetime('now')),
('33333333-3333-3333-3333-333333333333', '开发者', 'developer', '负责编写代码实现', '你是专业开发者...', 1, datetime('now'), datetime('now'));
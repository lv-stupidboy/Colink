-- ISDP SQLite 数据库初始化脚本
-- 版本: 1.0.1
-- 生成日期: 2026-04-11
-- 说明: 新环境首次安装时执行此脚本创建所有表结构
-- 数据库文件: data/sqlite/colink.db
-- 驱动: modernc.org/sqlite (纯 Go，无需 CGO)

-- ==================== 主表 ====================

-- 表: agent_configs
CREATE TABLE IF NOT EXISTS agent_configs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    base_agent_id TEXT DEFAULT NULL,
    description TEXT,
    system_prompt TEXT,
    max_tokens INTEGER DEFAULT 4096,
    temperature REAL DEFAULT 0.7,
    is_default INTEGER DEFAULT 0,
    is_system INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    config_generated_at TEXT DEFAULT NULL,
    config_path TEXT DEFAULT NULL,
    mention_patterns TEXT DEFAULT NULL
);

-- 表: agent_invocations
CREATE TABLE IF NOT EXISTS agent_invocations (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    agent_config_id TEXT DEFAULT NULL,
    role TEXT NOT NULL,
    agent_name TEXT DEFAULT NULL,
    status TEXT DEFAULT 'running',
    input TEXT,
    output TEXT,
    started_at TEXT DEFAULT NULL,
    completed_at TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    process_id TEXT DEFAULT NULL,
    full_prompt TEXT,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0.0,
    duration_ms INTEGER DEFAULT 0,
    duration_api_ms INTEGER DEFAULT 0
);

-- 表: artifacts
CREATE TABLE IF NOT EXISTS artifacts (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    name TEXT DEFAULT NULL,
    content TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    type TEXT DEFAULT NULL,
    path TEXT DEFAULT NULL,
    metadata TEXT DEFAULT NULL
);

-- 表: base_agents
CREATE TABLE IF NOT EXISTS base_agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    api_url TEXT DEFAULT NULL,
    api_token TEXT,
    default_model TEXT DEFAULT NULL,
    cli_path TEXT DEFAULT 'claude',
    git_bash_path TEXT DEFAULT NULL,
    max_tokens INTEGER DEFAULT NULL,
    timeout_minutes INTEGER DEFAULT 30,
    is_default INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: commands
CREATE TABLE IF NOT EXISTS commands (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: invocation_content_blocks
CREATE TABLE IF NOT EXISTS invocation_content_blocks (
    id TEXT PRIMARY KEY,
    invocation_id TEXT NOT NULL,
    type TEXT NOT NULL,
    content TEXT,
    tool_name TEXT DEFAULT NULL,
    tool_id TEXT DEFAULT NULL,
    input TEXT DEFAULT NULL,
    output TEXT,
    is_error INTEGER DEFAULT 0,
    status TEXT DEFAULT NULL,
    timestamp INTEGER NOT NULL,
    started_at INTEGER DEFAULT NULL,
    completed_at INTEGER DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: knowledge_bases
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT DEFAULT NULL,
    description TEXT,
    type TEXT NOT NULL,
    config TEXT DEFAULT NULL,
    query_endpoint TEXT DEFAULT NULL,
    status TEXT DEFAULT 'active',
    last_query_at TEXT DEFAULT NULL,
    query_count INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: messages
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    role TEXT NOT NULL,
    agent_id TEXT DEFAULT NULL,
    content TEXT,
    content_blocks TEXT DEFAULT NULL,
    message_type TEXT DEFAULT 'text',
    metadata TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    mentions TEXT DEFAULT NULL,
    origin TEXT DEFAULT NULL,
    reply_to TEXT DEFAULT NULL
);

-- 表: projects
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT DEFAULT 'draft',
    local_path TEXT NOT NULL,
    git_repo TEXT DEFAULT NULL,
    config TEXT DEFAULT NULL,
    workflow_template_id TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: rules
CREATE TABLE IF NOT EXISTS rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: sandboxes (v1.0.1 修复后的结构)
CREATE TABLE IF NOT EXISTS sandboxes (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    name TEXT DEFAULT NULL,
    image TEXT DEFAULT NULL,
    status TEXT DEFAULT 'created',
    container_id TEXT DEFAULT NULL,
    port INTEGER DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at TEXT DEFAULT NULL
);

-- 表: settings
CREATE TABLE IF NOT EXISTS settings (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    directory_path TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: skill_registries
CREATE TABLE IF NOT EXISTS skill_registries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT DEFAULT NULL,
    type TEXT NOT NULL,
    url TEXT NOT NULL,
    auth_config TEXT DEFAULT NULL,
    sync_interval INTEGER DEFAULT 3600,
    last_sync_at TEXT DEFAULT NULL,
    sync_status TEXT DEFAULT 'pending',
    skill_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: skills
CREATE TABLE IF NOT EXISTS skills (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    tags TEXT DEFAULT NULL,
    source_type TEXT NOT NULL,
    source_registry_id TEXT DEFAULT NULL,
    author_id TEXT DEFAULT NULL,
    project_id TEXT DEFAULT NULL,
    supported_agents TEXT DEFAULT NULL,
    use_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    is_public INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: subagents
CREATE TABLE IF NOT EXISTS subagents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 表: threads
CREATE TABLE IF NOT EXISTS threads (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    status TEXT DEFAULT 'idle',
    current_phase TEXT DEFAULT NULL,
    current_agent TEXT DEFAULT NULL,
    depth INTEGER DEFAULT 0,
    abort_token TEXT,
    workflow_template_id TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    name TEXT NOT NULL DEFAULT ''
);

-- 表: workflow_templates
CREATE TABLE IF NOT EXISTS workflow_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids TEXT DEFAULT NULL,
    checkpoints TEXT DEFAULT NULL,
    estimated_time TEXT DEFAULT NULL,
    is_system INTEGER DEFAULT 0,
    is_default INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    transitions TEXT DEFAULT NULL
);

-- ==================== 绑定表（有外键约束，放在最后） ====================

-- 表: agent_command_bindings
CREATE TABLE IF NOT EXISTS agent_command_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    command_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, command_id),
    FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    FOREIGN KEY (command_id) REFERENCES commands(id) ON DELETE CASCADE
);

-- 表: agent_rule_bindings
CREATE TABLE IF NOT EXISTS agent_rule_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, rule_id),
    FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
);

-- 表: agent_settings_bindings
CREATE TABLE IF NOT EXISTS agent_settings_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    settings_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, settings_id),
    FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    FOREIGN KEY (settings_id) REFERENCES settings(id) ON DELETE CASCADE
);

-- 表: agent_skill_bindings
CREATE TABLE IF NOT EXISTS agent_skill_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, skill_id),
    FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

-- 表: agent_subagent_bindings
CREATE TABLE IF NOT EXISTS agent_subagent_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    subagent_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, subagent_id),
    FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    FOREIGN KEY (subagent_id) REFERENCES subagents(id) ON DELETE CASCADE
);

-- 表: command_skill_bindings
CREATE TABLE IF NOT EXISTS command_skill_bindings (
    id TEXT PRIMARY KEY,
    command_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(command_id, skill_id),
    FOREIGN KEY (command_id) REFERENCES commands(id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

-- 表: subagent_skill_bindings
CREATE TABLE IF NOT EXISTS subagent_skill_bindings (
    id TEXT PRIMARY KEY,
    subagent_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subagent_id, skill_id),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
    FOREIGN KEY (subagent_id) REFERENCES subagents(id) ON DELETE CASCADE
);

-- ==================== 索引 ====================

-- agent_configs 索引
CREATE INDEX IF NOT EXISTS idx_agent_configs_base_agent_id ON agent_configs(base_agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_configs_is_system ON agent_configs(is_system);

-- agent_invocations 索引
CREATE INDEX IF NOT EXISTS idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_created_at ON agent_invocations(created_at);

-- artifacts 索引
CREATE INDEX IF NOT EXISTS idx_artifacts_thread_id ON artifacts(thread_id);

-- base_agents 索引
CREATE INDEX IF NOT EXISTS idx_base_agents_type ON base_agents(type);
CREATE INDEX IF NOT EXISTS idx_base_agents_is_default ON base_agents(is_default);

-- invocation_content_blocks 索引
CREATE INDEX IF NOT EXISTS idx_invocation_content_blocks_invocation_id ON invocation_content_blocks(invocation_id);
CREATE INDEX IF NOT EXISTS idx_invocation_content_blocks_timestamp ON invocation_content_blocks(timestamp);

-- knowledge_bases 索引
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_type ON knowledge_bases(type);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_status ON knowledge_bases(status);

-- messages 索引
CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_origin ON messages(origin);
CREATE INDEX IF NOT EXISTS idx_messages_reply_to ON messages(reply_to);

-- projects 索引
CREATE INDEX IF NOT EXISTS idx_projects_workflow_template_id ON projects(workflow_template_id);
CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects(created_at);

-- sandboxes 索引
CREATE INDEX IF NOT EXISTS idx_sandboxes_thread_id ON sandboxes(thread_id);

-- skill_registries 索引
CREATE INDEX IF NOT EXISTS idx_skill_registries_type ON skill_registries(type);
CREATE INDEX IF NOT EXISTS idx_skill_registries_status ON skill_registries(status);

-- skills 索引
CREATE INDEX IF NOT EXISTS idx_skills_source_type ON skills(source_type);
CREATE INDEX IF NOT EXISTS idx_skills_project_id ON skills(project_id);

-- threads 索引
CREATE INDEX IF NOT EXISTS idx_threads_project_id ON threads(project_id);
CREATE INDEX IF NOT EXISTS idx_threads_workflow_template_id ON threads(workflow_template_id);
CREATE INDEX IF NOT EXISTS idx_threads_created_at ON threads(created_at);

-- 绑定表索引
CREATE INDEX IF NOT EXISTS idx_agent_command_bindings_agent_role_id ON agent_command_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_command_bindings_command_id ON agent_command_bindings(command_id);
CREATE INDEX IF NOT EXISTS idx_agent_rule_bindings_agent_role_id ON agent_rule_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_rule_bindings_rule_id ON agent_rule_bindings(rule_id);
CREATE INDEX IF NOT EXISTS idx_agent_settings_bindings_agent_role_id ON agent_settings_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_settings_bindings_settings_id ON agent_settings_bindings(settings_id);
CREATE INDEX IF NOT EXISTS idx_agent_skill_bindings_agent_role_id ON agent_skill_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_skill_bindings_skill_id ON agent_skill_bindings(skill_id);
CREATE INDEX IF NOT EXISTS idx_agent_subagent_bindings_agent_role_id ON agent_subagent_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_subagent_bindings_subagent_id ON agent_subagent_bindings(subagent_id);
CREATE INDEX IF NOT EXISTS idx_command_skill_bindings_command_id ON command_skill_bindings(command_id);
CREATE INDEX IF NOT EXISTS idx_command_skill_bindings_skill_id ON command_skill_bindings(skill_id);
CREATE INDEX IF NOT EXISTS idx_subagent_skill_bindings_subagent_id ON subagent_skill_bindings(subagent_id);
CREATE INDEX IF NOT EXISTS idx_subagent_skill_bindings_skill_id ON subagent_skill_bindings(skill_id);
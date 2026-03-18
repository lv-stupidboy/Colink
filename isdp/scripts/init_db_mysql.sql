-- ISDP Database Initialization Script for MySQL
-- Version: 2.0

-- 设置字符集
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 项目表
CREATE TABLE IF NOT EXISTS projects (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(64) NOT NULL,
    mode VARCHAR(64) NOT NULL,
    status VARCHAR(32) DEFAULT 'draft',
    git_repo VARCHAR(512),
    config TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 开发会话表
CREATE TABLE IF NOT EXISTS threads (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64),
    status VARCHAR(32) DEFAULT 'idle',
    current_phase VARCHAR(64),
    current_agent VARCHAR(64),
    depth INT DEFAULT 0,
    abort_token TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 消息表
CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    role VARCHAR(32) NOT NULL,
    agent_id VARCHAR(64),
    content LONGTEXT,
    message_type VARCHAR(32) DEFAULT 'text',
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 基础Agent配置表
CREATE TABLE IF NOT EXISTS base_agents (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(64) NOT NULL,
    api_url VARCHAR(512),
    api_token TEXT,
    default_model VARCHAR(128),
    cli_path VARCHAR(512) DEFAULT 'claude',
    git_bash_path VARCHAR(512),
    max_tokens INT DEFAULT 4096,
    timeout_minutes INT DEFAULT 30,
    is_active TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Agent配置表（Agent角色）
CREATE TABLE IF NOT EXISTS agent_configs (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(64) NOT NULL,
    base_agent_id VARCHAR(64),
    description TEXT,
    system_prompt TEXT,
    model_name VARCHAR(128) DEFAULT 'claude-sonnet-4-6',
    max_tokens INT DEFAULT 4096,
    temperature DECIMAL(3,2) DEFAULT 0.7,
    routing_config JSON,
    is_default TINYINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (base_agent_id) REFERENCES base_agents(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Agent调用记录表
CREATE TABLE IF NOT EXISTS agent_invocations (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    agent_config_id VARCHAR(64),
    role VARCHAR(64) NOT NULL,
    status VARCHAR(32) DEFAULT 'running',
    input LONGTEXT,
    output LONGTEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 产物表
CREATE TABLE IF NOT EXISTS artifacts (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    type VARCHAR(64) NOT NULL,
    name VARCHAR(255),
    path VARCHAR(512),
    content LONGTEXT,
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 沙箱容器表
CREATE TABLE IF NOT EXISTS sandboxes (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    name VARCHAR(255),
    image VARCHAR(255),
    status VARCHAR(32) DEFAULT 'created',
    container_id VARCHAR(128),
    port INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 工作流模板表
CREATE TABLE IF NOT EXISTS workflow_templates (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    phases JSON,
    is_active TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 创建索引
CREATE INDEX idx_threads_project_id ON threads(project_id);
CREATE INDEX idx_messages_thread_id ON messages(thread_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_agent_configs_base_agent_id ON agent_configs(base_agent_id);
CREATE INDEX idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX idx_artifacts_thread_id ON artifacts(thread_id);
CREATE INDEX idx_sandboxes_thread_id ON sandboxes(thread_id);
CREATE INDEX idx_base_agents_type ON base_agents(type);

SET FOREIGN_KEY_CHECKS = 1;
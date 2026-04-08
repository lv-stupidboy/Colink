-- ISDP Database Initialization Script for MySQL
-- Version: 2.4
-- Description: ISDP智能软件开发平台数据库初始化脚本
-- 更新说明:
--   - v2.4: agent_configs 表增加 is_system 字段和索引
--   - v2.3: 初始版本

-- 设置字符集
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- 基础Agent配置表
-- ----------------------------
DROP TABLE IF EXISTS base_agents;
CREATE TABLE base_agents (
    id VARCHAR(64) NOT NULL COMMENT '基础Agent唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT 'Agent名称',
    type VARCHAR(64) NOT NULL COMMENT 'Agent类型(claude/openai/custom)',
    api_url VARCHAR(512) COMMENT 'API服务地址(自定义Agent用)',
    api_token TEXT COMMENT 'API认证令牌(加密存储)',
    default_model VARCHAR(128) COMMENT '默认模型名称',
    cli_path VARCHAR(512) DEFAULT 'claude' COMMENT 'CLI工具路径',
    git_bash_path VARCHAR(512) COMMENT 'Git Bash路径(Windows环境)',
    max_tokens INT COMMENT '最大token数',
    timeout_minutes INT DEFAULT 30 COMMENT '超时时间(分钟)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_base_agents_type (type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='基础Agent配置表';

-- ----------------------------
-- 工作流模板表
-- ----------------------------
DROP TABLE IF EXISTS workflow_templates;
CREATE TABLE workflow_templates (
    id VARCHAR(64) NOT NULL COMMENT '模板唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '模板名称',
    description TEXT COMMENT '模板描述',
    agent_ids JSON COMMENT 'Agent ID列表(JSON数组)',
    transitions JSON COMMENT 'A2A路由转换规则',
    checkpoints JSON COMMENT '检查点配置(JSON格式)',
    estimated_time VARCHAR(64) COMMENT '预计完成时间',
    is_system TINYINT DEFAULT 0 COMMENT '是否系统模板(0-否,1-是)',
    is_default TINYINT DEFAULT 0 COMMENT '是否默认模板(0-否,1-是)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='工作流模板表';

-- ----------------------------
-- 项目表
-- ----------------------------
DROP TABLE IF EXISTS projects;
CREATE TABLE projects (
    id VARCHAR(64) NOT NULL COMMENT '项目唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '项目名称',
    type VARCHAR(64) NOT NULL COMMENT '项目类型(frontend/backend/fullstack等)',
    mode VARCHAR(64) NOT NULL COMMENT '开发模式(standard/advanced等)',
    status VARCHAR(32) DEFAULT 'draft' COMMENT '项目状态(draft/active/archived/deleted)',
    local_path VARCHAR(512) NOT NULL COMMENT '项目本地路径',
    git_repo VARCHAR(512) COMMENT 'Git仓库地址',
    config JSON COMMENT '项目配置(JSON格式)',
    workflow_template_id VARCHAR(64) COMMENT '工作流模板ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_projects_workflow_template_id (workflow_template_id),
    CONSTRAINT fk_projects_workflow_template FOREIGN KEY (workflow_template_id) REFERENCES workflow_templates(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目信息表';

-- ----------------------------
-- 开发会话表
-- ----------------------------
DROP TABLE IF EXISTS threads;
CREATE TABLE threads (
    id VARCHAR(64) NOT NULL COMMENT '会话唯一标识符',
    project_id VARCHAR(64) NOT NULL COMMENT '关联项目ID',
    name VARCHAR(255) NOT NULL DEFAULT '' COMMENT '会话名称',
    status VARCHAR(32) DEFAULT 'idle' COMMENT '会话状态(idle/running/completed/error)',
    current_phase VARCHAR(64) COMMENT '当前开发阶段(planning/coding/testing等)',
    current_agent VARCHAR(64) COMMENT '当前执行Agent ID',
    depth INT DEFAULT 0 COMMENT '会话嵌套深度',
    abort_token TEXT COMMENT '中止令牌(用于取消操作)',
    workflow_template_id VARCHAR(64) COMMENT '工作流模板ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_threads_project_id (project_id),
    KEY idx_threads_workflow_template_id (workflow_template_id),
    CONSTRAINT fk_threads_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_threads_workflow_template FOREIGN KEY (workflow_template_id) REFERENCES workflow_templates(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='开发会话表';

-- ----------------------------
-- 消息表
-- ----------------------------
DROP TABLE IF EXISTS messages;
CREATE TABLE messages (
    id VARCHAR(64) NOT NULL COMMENT '消息唯一标识符',
    thread_id VARCHAR(64) NOT NULL COMMENT '关联会话ID',
    role VARCHAR(32) NOT NULL COMMENT '消息角色(user/assistant/system/tool)',
    agent_id VARCHAR(64) COMMENT '产生消息的Agent ID',
    content LONGTEXT COMMENT '消息内容',
    message_type VARCHAR(32) DEFAULT 'text' COMMENT '消息类型(text/code/tool_use/tool_result)',
    metadata JSON COMMENT '消息元数据(如token使用量、模型信息等)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_messages_thread_id (thread_id),
    KEY idx_messages_created_at (created_at),
    CONSTRAINT fk_messages_thread FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='对话消息表';

-- ----------------------------
-- Agent配置表（Agent角色）
-- ----------------------------
DROP TABLE IF EXISTS agent_configs;
CREATE TABLE agent_configs (
    id VARCHAR(64) NOT NULL COMMENT 'Agent配置唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '配置名称',
    role VARCHAR(64) NOT NULL COMMENT 'Agent角色(developer/reviewer/tester等)',
    base_agent_id VARCHAR(64) COMMENT '关联基础Agent ID',
    description TEXT COMMENT '角色描述',
    system_prompt TEXT COMMENT '系统提示词',
    model_name VARCHAR(128) DEFAULT 'claude-sonnet-4-6' COMMENT '使用的模型名称',
    max_tokens INT DEFAULT 4096 COMMENT '最大生成token数',
    temperature DECIMAL(3,2) DEFAULT 0.7 COMMENT '温度参数(0-1,越高越随机)',
    routing_config JSON COMMENT '路由配置(定义Agent调用规则)',
    capabilities JSON COMMENT 'Agent能力声明',
    dependencies JSON COMMENT 'Agent依赖配置',
    outputs JSON COMMENT 'Agent输出配置',
    is_default TINYINT DEFAULT 0 COMMENT '是否默认配置(0-否,1-是)',
    is_system TINYINT DEFAULT 0 COMMENT '是否系统预置(0-否,1-是)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_agent_configs_base_agent_id (base_agent_id),
    KEY idx_agent_configs_is_system (is_system),
    CONSTRAINT fk_agent_configs_base_agent FOREIGN KEY (base_agent_id) REFERENCES base_agents(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent角色配置表';

-- ----------------------------
-- Agent调用记录表
-- ----------------------------
DROP TABLE IF EXISTS agent_invocations;
CREATE TABLE agent_invocations (
    id VARCHAR(64) NOT NULL COMMENT '调用记录唯一标识符',
    thread_id VARCHAR(64) NOT NULL COMMENT '关联会话ID',
    agent_config_id VARCHAR(64) COMMENT '关联Agent配置ID',
    role VARCHAR(64) NOT NULL COMMENT '执行角色',
    status VARCHAR(32) DEFAULT 'running' COMMENT '调用状态(running/completed/failed/cancelled)',
    input LONGTEXT COMMENT '输入内容',
    output LONGTEXT COMMENT '输出结果',
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
    completed_at TIMESTAMP NULL COMMENT '完成时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_agent_invocations_thread_id (thread_id),
    CONSTRAINT fk_agent_invocations_thread FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent调用记录表';

-- ----------------------------
-- 产物表
-- ----------------------------
DROP TABLE IF EXISTS artifacts;
CREATE TABLE artifacts (
    id VARCHAR(64) NOT NULL COMMENT '产物唯一标识符',
    thread_id VARCHAR(64) NOT NULL COMMENT '关联会话ID',
    type VARCHAR(64) COMMENT '产物类型(file/code/document/review等)',
    name VARCHAR(255) COMMENT '产物名称',
    path VARCHAR(512) COMMENT '文件路径',
    content LONGTEXT COMMENT '产物内容',
    metadata JSON COMMENT '元数据(JSON格式)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_artifacts_thread_id (thread_id),
    CONSTRAINT fk_artifacts_thread FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='开发产物表';

-- ----------------------------
-- 沙箱容器表
-- ----------------------------
DROP TABLE IF EXISTS sandboxes;
CREATE TABLE sandboxes (
    id VARCHAR(64) NOT NULL COMMENT '沙箱唯一标识符',
    thread_id VARCHAR(64) NOT NULL COMMENT '关联会话ID',
    config JSON COMMENT '沙箱配置(JSON格式)',
    status VARCHAR(32) COMMENT '容器状态(created/running/stopped/error)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_sandboxes_thread_id (thread_id),
    CONSTRAINT fk_sandboxes_thread FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='沙箱容器表';

-- ----------------------------
-- 创建索引
-- ----------------------------
CREATE INDEX idx_projects_created_at ON projects(created_at);
CREATE INDEX idx_threads_created_at ON threads(created_at);
CREATE INDEX idx_agent_invocations_created_at ON agent_invocations(created_at);

SET FOREIGN_KEY_CHECKS = 1;
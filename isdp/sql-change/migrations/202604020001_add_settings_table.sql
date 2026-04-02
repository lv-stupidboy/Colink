-- isdp/sql-change/migrations/202604020001_add_settings_table.sql
-- 变更说明：新增 Settings（配置目录）表及 Agent-Settings 绑定表
-- 作者：ISDP Team
-- 日期：2026-04-02

SET NAMES utf8mb4;

START TRANSACTION;

-- ----------------------------
-- Settings 配置目录表
-- ----------------------------
CREATE TABLE IF NOT EXISTS settings (
    id VARCHAR(64) NOT NULL COMMENT 'Settings唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT 'Settings名称(唯一标识)',
    description TEXT COMMENT '描述',
    directory_path VARCHAR(512) COMMENT '存储路径',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_settings_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='配置目录表';

-- ----------------------------
-- Agent-Settings 绑定表
-- ----------------------------
CREATE TABLE IF NOT EXISTS agent_settings_bindings (
    id VARCHAR(64) NOT NULL COMMENT '绑定唯一标识符',
    agent_role_id VARCHAR(64) NOT NULL COMMENT 'AgentRole ID',
    settings_id VARCHAR(64) NOT NULL COMMENT 'Settings ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_settings (agent_role_id, settings_id),
    KEY idx_agent_settings_bindings_agent_role_id (agent_role_id),
    KEY idx_agent_settings_bindings_settings_id (settings_id),
    CONSTRAINT fk_agent_settings_agent FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    CONSTRAINT fk_agent_settings_settings FOREIGN KEY (settings_id) REFERENCES settings(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent与Settings关联表';

COMMIT;

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS agent_settings_bindings;
-- DROP TABLE IF EXISTS settings;
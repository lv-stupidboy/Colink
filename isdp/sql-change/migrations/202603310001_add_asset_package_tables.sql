-- isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql
-- 变更说明：新增资产包系统相关表
-- 作者：ISDP Team
-- 日期：2026-03-31

SET NAMES utf8mb4;

START TRANSACTION;

-- ----------------------------
-- 资产包表
-- ----------------------------
CREATE TABLE IF NOT EXISTS asset_packages (
    id VARCHAR(64) NOT NULL COMMENT '资产包唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '资产包名称',
    version VARCHAR(50) NOT NULL COMMENT '资产包版本号',
    description TEXT COMMENT '资产包描述',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    KEY idx_asset_packages_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='资产包表';

-- ----------------------------
-- Settings 表
-- ----------------------------
CREATE TABLE IF NOT EXISTS settings (
    id VARCHAR(64) NOT NULL COMMENT 'Settings唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT 'Settings名称(唯一标识)',
    description TEXT COMMENT 'Settings描述',
    directory_path VARCHAR(500) COMMENT 'Settings目录路径',
    version VARCHAR(20) COMMENT 'Settings版本号',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_settings_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Settings配置表';

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
-- DROP TABLE IF EXISTS asset_packages;
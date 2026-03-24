-- isdp/sql-change/migrations/202603220001_add_subagent_tables.sql
-- 变更说明：添加子代理(Subagent)相关表结构
-- 作者：ISDP Team
-- 日期：2026-03-22

-- 设置字符集
SET NAMES utf8mb4;

-- ----------------------------
-- Subagent 表
-- ----------------------------
DROP TABLE IF EXISTS agent_subagent_bindings;
DROP TABLE IF EXISTS subagents;

CREATE TABLE subagents (
    id VARCHAR(64) NOT NULL COMMENT 'Subagent唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT 'Subagent名称(唯一标识)',
    description TEXT COMMENT '描述',
    content LONGTEXT NOT NULL COMMENT 'Subagent配置内容(CLAUDE.md格式)',

    -- 关联信息
    skill_id VARCHAR(64) COMMENT '关联技能包ID(可选)',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_subagents_name (name),
    KEY idx_subagents_skill_id (skill_id),
    CONSTRAINT fk_subagents_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='子代理配置表';

-- ----------------------------
-- AgentRole 与 Subagent 关联表
-- ----------------------------
CREATE TABLE agent_subagent_bindings (
    id VARCHAR(64) NOT NULL COMMENT '关联唯一标识符',
    agent_role_id VARCHAR(64) NOT NULL COMMENT 'AgentRole ID',
    subagent_id VARCHAR(64) NOT NULL COMMENT 'Subagent ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_subagent_binding (agent_role_id, subagent_id),
    KEY idx_agent_subagent_bindings_agent_role_id (agent_role_id),
    KEY idx_agent_subagent_bindings_subagent_id (subagent_id),
    CONSTRAINT fk_agent_subagent_agent FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    CONSTRAINT fk_agent_subagent_subagent FOREIGN KEY (subagent_id) REFERENCES subagents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent与Subagent关联表';

-- ----------------------------
-- 为 agent_configs 表添加配置生成相关字段
-- ----------------------------
ALTER TABLE agent_configs
    ADD COLUMN config_generated_at TIMESTAMP NULL COMMENT '配置文件生成时间',
    ADD COLUMN config_path VARCHAR(512) NULL COMMENT '配置文件存储路径';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_configs DROP COLUMN config_generated_at;
-- ALTER TABLE agent_configs DROP COLUMN config_path;
-- DROP TABLE IF EXISTS agent_subagent_bindings;
-- DROP TABLE IF EXISTS subagents;
-- isdp/sql-change/migrations/202603240001_add_command_rule_tables.sql
-- 变更说明：新增 Command、Rule 表及相关绑定表
-- 作者：ISDP Team
-- 日期：2026-03-24

SET NAMES utf8mb4;

START TRANSACTION;

-- ----------------------------
-- 命令表
-- ----------------------------
CREATE TABLE IF NOT EXISTS commands (
    id VARCHAR(64) NOT NULL COMMENT '命令唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '命令名称(唯一标识)',
    description TEXT COMMENT '命令描述',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_commands_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='命令表';

-- ----------------------------
-- 规约表
-- ----------------------------
CREATE TABLE IF NOT EXISTS rules (
    id VARCHAR(64) NOT NULL COMMENT '规约唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '规约名称(唯一标识)',
    description TEXT COMMENT '规约描述',
    scope VARCHAR(20) NOT NULL DEFAULT 'instance' COMMENT '作用域: public / instance',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_rules_name (name),
    KEY idx_rules_scope (scope)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='规约表';

-- ----------------------------
-- Agent-Command 绑定表
-- ----------------------------
CREATE TABLE IF NOT EXISTS agent_command_bindings (
    id VARCHAR(64) NOT NULL COMMENT '绑定唯一标识符',
    agent_role_id VARCHAR(64) NOT NULL COMMENT 'AgentRole ID',
    command_id VARCHAR(64) NOT NULL COMMENT 'Command ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_command (agent_role_id, command_id),
    KEY idx_agent_command_bindings_agent_role_id (agent_role_id),
    KEY idx_agent_command_bindings_command_id (command_id),
    CONSTRAINT fk_agent_command_agent FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    CONSTRAINT fk_agent_command_command FOREIGN KEY (command_id) REFERENCES commands(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent与Command关联表';

-- ----------------------------
-- Agent-Rule 绑定表
-- ----------------------------
CREATE TABLE IF NOT EXISTS agent_rule_bindings (
    id VARCHAR(64) NOT NULL COMMENT '绑定唯一标识符',
    agent_role_id VARCHAR(64) NOT NULL COMMENT 'AgentRole ID',
    rule_id VARCHAR(64) NOT NULL COMMENT 'Rule ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_rule (agent_role_id, rule_id),
    KEY idx_agent_rule_bindings_agent_role_id (agent_role_id),
    KEY idx_agent_rule_bindings_rule_id (rule_id),
    CONSTRAINT fk_agent_rule_agent FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    CONSTRAINT fk_agent_rule_rule FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent与Rule关联表';

-- ----------------------------
-- Command-Skill 绑定表
-- ----------------------------
CREATE TABLE IF NOT EXISTS command_skill_bindings (
    id VARCHAR(64) NOT NULL COMMENT '绑定唯一标识符',
    command_id VARCHAR(64) NOT NULL COMMENT 'Command ID',
    skill_id VARCHAR(64) NOT NULL COMMENT 'Skill ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_command_skill (command_id, skill_id),
    KEY idx_command_skill_bindings_command_id (command_id),
    KEY idx_command_skill_bindings_skill_id (skill_id),
    CONSTRAINT fk_command_skill_command FOREIGN KEY (command_id) REFERENCES commands(id) ON DELETE CASCADE,
    CONSTRAINT fk_command_skill_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Command与Skill关联表';

-- ----------------------------
-- Subagent-Skill 绑定表
-- ----------------------------
CREATE TABLE IF NOT EXISTS subagent_skill_bindings (
    id VARCHAR(64) NOT NULL COMMENT '绑定唯一标识符',
    subagent_id VARCHAR(64) NOT NULL COMMENT 'Subagent ID',
    skill_id VARCHAR(64) NOT NULL COMMENT 'Skill ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_subagent_skill (subagent_id, skill_id),
    KEY idx_subagent_skill_bindings_subagent_id (subagent_id),
    KEY idx_subagent_skill_bindings_skill_id (skill_id),
    CONSTRAINT fk_subagent_skill_subagent FOREIGN KEY (subagent_id) REFERENCES subagents(id) ON DELETE CASCADE,
    CONSTRAINT fk_subagent_skill_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Subagent与Skill关联表';

-- ----------------------------
-- 迁移现有 subagents.skill_id 数据到绑定表
-- ----------------------------
INSERT INTO subagent_skill_bindings (id, subagent_id, skill_id, created_at)
SELECT
    UUID() as id,
    id as subagent_id,
    skill_id,
    NOW() as created_at
FROM subagents
WHERE skill_id IS NOT NULL AND skill_id != '';

-- ----------------------------
-- 为 subagents 表移除 skill_id 字段（可选，保留向后兼容）
-- ----------------------------
-- ALTER TABLE subagents DROP COLUMN skill_id;

COMMIT;

-- 回滚语句（如需回滚执行以下语句）
-- 回滚数据迁移说明：
-- 由于数据已从 subagents.skill_id 迁移到 subagent_skill_bindings 表，
-- 回滚时需要先确认 subagents 表仍保留 skill_id 字段。
-- 如果已删除 skill_id 字段，需要先恢复该字段再迁移数据回来。
-- DROP TABLE IF EXISTS subagent_skill_bindings;
-- DROP TABLE IF EXISTS command_skill_bindings;
-- DROP TABLE IF EXISTS agent_rule_bindings;
-- DROP TABLE IF EXISTS agent_command_bindings;
-- DROP TABLE IF EXISTS rules;
-- DROP TABLE IF EXISTS commands;
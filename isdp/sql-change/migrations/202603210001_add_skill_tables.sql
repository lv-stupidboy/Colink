-- isdp/sql-change/migrations/202603210001_add_skill_tables.sql
-- 变更说明：添加技能库相关表
-- 作者：ISDP Team
-- 日期：2026-03-21

-- 设置字符集
SET NAMES utf8mb4;

-- ----------------------------
-- Skill 表
-- ----------------------------
DROP TABLE IF EXISTS skills;
CREATE TABLE skills (
    id VARCHAR(64) NOT NULL COMMENT 'Skill唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT 'Skill名称(唯一标识)',
    display_name VARCHAR(255) COMMENT '显示名称',
    description TEXT COMMENT '描述',
    type VARCHAR(50) DEFAULT 'skill' COMMENT '类型(skill/rule)',
    category VARCHAR(100) COMMENT '分类',

    -- 来源信息
    source_type VARCHAR(50) NOT NULL COMMENT '来源类型(built_in/uploaded/federated)',
    source_registry_id VARCHAR(64) COMMENT '联邦来源ID',
    author_id VARCHAR(64) COMMENT '创建者ID',
    project_id VARCHAR(64) COMMENT '所属项目ID',

    -- 安装信息
    install_source JSON COMMENT '不同智能体的安装地址',

    -- 兼容性
    supported_agents JSON COMMENT '支持的智能体列表',

    -- 版本
    version VARCHAR(50) DEFAULT '1.0.0' COMMENT '版本号',

    -- 统计数据
    use_count INT DEFAULT 0 COMMENT '使用次数',
    star_count INT DEFAULT 0 COMMENT '点赞数',
    favorite_count INT DEFAULT 0 COMMENT '收藏数',

    -- 状态
    status VARCHAR(50) DEFAULT 'active' COMMENT '状态(active/deprecated)',
    is_public TINYINT DEFAULT 0 COMMENT '是否公开(0-否,1-是)',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_skills_name (name),
    KEY idx_skills_type (type),
    KEY idx_skills_source_type (source_type),
    KEY idx_skills_category (category),
    KEY idx_skills_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='技能表';

-- ----------------------------
-- AgentRole 与 Skill 关联表
-- ----------------------------
DROP TABLE IF EXISTS agent_skill_bindings;
CREATE TABLE agent_skill_bindings (
    id VARCHAR(64) NOT NULL COMMENT '关联唯一标识符',
    agent_role_id VARCHAR(64) NOT NULL COMMENT 'AgentRole ID',
    skill_id VARCHAR(64) NOT NULL COMMENT 'Skill ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_binding (agent_role_id, skill_id),
    KEY idx_agent_skill_bindings_agent_role_id (agent_role_id),
    KEY idx_agent_skill_bindings_skill_id (skill_id),
    CONSTRAINT fk_binding_agent FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    CONSTRAINT fk_binding_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent与Skill关联表';

-- ----------------------------
-- Skill 收藏记录表
-- ----------------------------
DROP TABLE IF EXISTS skill_favorites;
CREATE TABLE skill_favorites (
    id VARCHAR(64) NOT NULL COMMENT '收藏记录唯一标识符',
    skill_id VARCHAR(64) NOT NULL COMMENT 'Skill ID',
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_favorite (skill_id, user_id),
    KEY idx_skill_favorites_skill_id (skill_id),
    KEY idx_skill_favorites_user_id (user_id),
    CONSTRAINT fk_favorite_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Skill收藏记录表';

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS skill_favorites;
-- DROP TABLE IF EXISTS agent_skill_bindings;
-- DROP TABLE IF EXISTS skills;
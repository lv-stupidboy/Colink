-- isdp/sql-change/migrations/202603210002_add_skill_registries.sql
-- 变更说明：添加联邦技能源配置表
-- 作者：ISDP Team
-- 日期：2026-03-21

-- 设置字符集
SET NAMES utf8mb4;

-- ----------------------------
-- 联邦技能源配置表
-- ----------------------------
DROP TABLE IF EXISTS skill_registries;
CREATE TABLE skill_registries (
    id VARCHAR(64) NOT NULL COMMENT '注册表唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '注册表名称(唯一标识)',
    display_name VARCHAR(255) COMMENT '显示名称',
    type VARCHAR(50) NOT NULL COMMENT '类型(github/gitlab/api/custom)',
    url VARCHAR(500) NOT NULL COMMENT '注册表URL',
    auth_config JSON COMMENT '认证配置(加密存储)',
    sync_interval INT DEFAULT 3600 COMMENT '同步间隔(秒)',
    last_sync_at TIMESTAMP NULL COMMENT '最后同步时间',
    sync_status VARCHAR(50) DEFAULT 'pending' COMMENT '同步状态(pending/success/failed)',
    skill_count INT DEFAULT 0 COMMENT '技能数量',
    status VARCHAR(50) DEFAULT 'active' COMMENT '状态(active/inactive)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_skill_registries_name (name),
    KEY idx_skill_registries_type (type),
    KEY idx_skill_registries_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='联邦技能源配置表';

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS skill_registries;
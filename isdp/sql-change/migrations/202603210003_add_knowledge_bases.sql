-- isdp/sql-change/migrations/202603210003_add_knowledge_bases.sql
-- 变更说明：添加知识库配置表
-- 作者：ISDP Team
-- 日期：2026-03-21

-- 设置字符集
SET NAMES utf8mb4;

-- ----------------------------
-- 知识库配置表
-- ----------------------------
DROP TABLE IF EXISTS knowledge_bases;
CREATE TABLE knowledge_bases (
    id VARCHAR(64) NOT NULL COMMENT '知识库唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT '知识库名称(唯一标识)',
    display_name VARCHAR(255) COMMENT '显示名称',
    description TEXT COMMENT '描述',
    type VARCHAR(50) NOT NULL COMMENT '类型(git/mcp/api)',
    config JSON COMMENT '配置信息(加密存储)',
    query_endpoint VARCHAR(500) COMMENT '查询端点URL',
    status VARCHAR(50) DEFAULT 'active' COMMENT '状态(active/inactive)',
    last_query_at TIMESTAMP NULL COMMENT '最后查询时间',
    query_count INT DEFAULT 0 COMMENT '查询次数',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_knowledge_bases_name (name),
    KEY idx_knowledge_bases_type (type),
    KEY idx_knowledge_bases_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识库配置表';

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS knowledge_bases;
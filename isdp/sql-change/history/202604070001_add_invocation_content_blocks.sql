-- 文件路径: isdp/sql-change/migrations/202604070001_add_invocation_content_blocks.sql
-- 变更说明：Agent 后台执行支持 - 增量持久化内容块
-- 作者：Claude
-- 日期：2026-04-07

SET NAMES utf8mb4;

-- 1. 添加 process_id 字段用于进程追踪
ALTER TABLE agent_invocations ADD COLUMN process_id VARCHAR(36) DEFAULT NULL;

-- 2. 创建内容块表
CREATE TABLE invocation_content_blocks (
    id VARCHAR(36) PRIMARY KEY,
    invocation_id VARCHAR(36) NOT NULL,
    type VARCHAR(20) NOT NULL COMMENT 'thinking, text, tool_use, tool_result',
    content TEXT,
    tool_name VARCHAR(100),
    tool_id VARCHAR(36),
    input JSON,
    output TEXT,
    is_error BOOLEAN DEFAULT FALSE,
    status VARCHAR(20) COMMENT 'streaming, completed',
    timestamp BIGINT NOT NULL,
    started_at BIGINT,
    completed_at BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_invocation_id (invocation_id),
    INDEX idx_timestamp (timestamp)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


ALTER TABLE invocation_content_blocks MODIFY COLUMN id VARCHAR(128) NOT NULL;
ALTER TABLE invocation_content_blocks MODIFY COLUMN tool_id VARCHAR(128);
-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS invocation_content_blocks;
-- ALTER TABLE agent_invocations DROP COLUMN process_id;
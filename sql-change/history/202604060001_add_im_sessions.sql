-- 文件路径: isdp/sql-change/migrations/202604060001_add_im_sessions.sql
-- 变更说明：添加 IM 会话映射表，支持飞书等 IM 平台与 ISDP Thread 的映射
-- 作者：ISDP Team
-- 日期：2026-04-06

SET NAMES utf8mb4;

-- 正向变更
CREATE TABLE IF NOT EXISTS im_sessions (
    id              VARCHAR(36) PRIMARY KEY,
    platform        VARCHAR(20) NOT NULL DEFAULT 'feishu',
    chat_id         VARCHAR(128) NOT NULL,
    chat_type       VARCHAR(20) NOT NULL DEFAULT 'p2p',
    thread_id       VARCHAR(36) NOT NULL,
    project_id      VARCHAR(36) NOT NULL,
    user_id         VARCHAR(128) DEFAULT '',
    user_name       VARCHAR(128) DEFAULT '',
    last_message_at TIMESTAMP NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_platform_chat (platform, chat_id),
    INDEX idx_thread_id (thread_id),
    INDEX idx_platform_active (platform, is_active)
);

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS im_sessions;

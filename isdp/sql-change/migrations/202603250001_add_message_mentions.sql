-- 文件路径: isdp/sql-change/migrations/202603250001_add_message_mentions.sql
-- 变更说明: 添加消息 mentions 相关字段，支持 A2A 协作
-- 作者: Claude
-- 日期: 2026-03-25

SET NAMES utf8mb4;

-- 添加 mentions 字段（JSON 数组存储被 @mention 的 Agent IDs）
ALTER TABLE messages ADD COLUMN mentions JSON DEFAULT NULL COMMENT '被 @mention 的 Agent IDs';

-- 添加 origin 字段（消息来源）
ALTER TABLE messages ADD COLUMN origin VARCHAR(20) DEFAULT NULL COMMENT '消息来源: user, callback, stream';

-- 添加 reply_to 字段（回复的消息 ID）
ALTER TABLE messages ADD COLUMN reply_to VARCHAR(36) DEFAULT NULL COMMENT '回复的消息 ID';

-- 添加索引加速查询
CREATE INDEX idx_messages_mentions ON messages ((CAST(mentions AS CHAR(500))));
CREATE INDEX idx_messages_origin ON messages (origin);
CREATE INDEX idx_messages_reply_to ON messages (reply_to);

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE messages DROP COLUMN mentions;
-- ALTER TABLE messages DROP COLUMN origin;
-- ALTER TABLE messages DROP COLUMN reply_to;
-- DROP INDEX idx_messages_mentions ON messages;
-- DROP INDEX idx_messages_origin ON messages;
-- DROP INDEX idx_messages_reply_to ON messages;
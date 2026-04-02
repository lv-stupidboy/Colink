-- 文件路径: isdp/sql-change/migrations/202604020001_add_invocation_usage.sql
-- 变更说明：为 agent_invocations 表添加 Token 使用统计字段
-- 作者：ISDP Team
-- 日期：2026-04-02

SET NAMES utf8mb4;

-- 添加 Token 使用统计字段
ALTER TABLE agent_invocations
ADD COLUMN input_tokens BIGINT DEFAULT 0 COMMENT '输入 Token 数量',
ADD COLUMN output_tokens BIGINT DEFAULT 0 COMMENT '输出 Token 数量',
ADD COLUMN cache_read_tokens BIGINT DEFAULT 0 COMMENT '缓存读取 Token 数量',
ADD COLUMN cache_creation_tokens BIGINT DEFAULT 0 COMMENT '缓存创建 Token 数量',
ADD COLUMN cost_usd DECIMAL(10,6) DEFAULT 0 COMMENT 'API 成本（美元）',
ADD COLUMN duration_ms BIGINT DEFAULT 0 COMMENT '总执行时长（毫秒）',
ADD COLUMN duration_api_ms BIGINT DEFAULT 0 COMMENT 'API 响应时长（毫秒）';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_invocations
-- DROP COLUMN input_tokens,
-- DROP COLUMN output_tokens,
-- DROP COLUMN cache_read_tokens,
-- DROP COLUMN cache_creation_tokens,
-- DROP COLUMN cost_usd,
-- DROP COLUMN duration_ms,
-- DROP COLUMN duration_api_ms;
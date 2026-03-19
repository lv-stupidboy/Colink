-- 变更说明：为 threads 表添加 name 字段
-- 作者：系统迁移
-- 日期：2026-03-20
-- 来源：scripts/202403200001_add_thread_name.sql

-- 正向变更
ALTER TABLE threads ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT '' COMMENT '会话名称';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE threads DROP COLUMN name;
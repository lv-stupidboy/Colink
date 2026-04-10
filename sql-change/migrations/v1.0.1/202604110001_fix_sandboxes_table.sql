-- 文件路径: sql-change/migrations/v1.0.1/202604110001_fix_sandboxes_table.sql
-- 版本号: 202604110001
-- 变更说明：重构 sandboxes 表结构，删除废弃 config 字段，添加实际使用字段
-- 作者：axiang
-- 日期：2026-04-11
-- 影响范围：sandboxes 表
-- 回滚风险：中（需要恢复原结构）

-- MySQL 正向变更
-- 1. 删除废弃字段
ALTER TABLE sandboxes DROP COLUMN IF EXISTS config;

-- 2. 添加新字段
ALTER TABLE sandboxes ADD COLUMN IF NOT EXISTS name VARCHAR(255) DEFAULT NULL COMMENT '沙箱名称';
ALTER TABLE sandboxes ADD COLUMN IF NOT EXISTS image VARCHAR(255) DEFAULT NULL COMMENT 'Docker镜像';
ALTER TABLE sandboxes ADD COLUMN IF NOT EXISTS container_id VARCHAR(128) DEFAULT NULL COMMENT '容器ID';
ALTER TABLE sandboxes ADD COLUMN IF NOT EXISTS port INT DEFAULT NULL COMMENT '服务端口';
ALTER TABLE sandboxes ADD COLUMN IF NOT EXISTS ended_at TIMESTAMP NULL DEFAULT NULL COMMENT '结束时间';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE sandboxes ADD COLUMN config JSON DEFAULT NULL COMMENT '沙箱配置';
-- ALTER TABLE sandboxes DROP COLUMN name;
-- ALTER TABLE sandboxes DROP COLUMN image;
-- ALTER TABLE sandboxes DROP COLUMN container_id;
-- ALTER TABLE sandboxes DROP COLUMN port;
-- ALTER TABLE sandboxes DROP COLUMN ended_at;
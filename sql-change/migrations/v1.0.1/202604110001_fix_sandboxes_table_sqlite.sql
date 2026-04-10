-- 文件路径: sql-change/migrations/v1.0.1/202604110001_fix_sandboxes_table_sqlite.sql
-- 版本号: 202604110001
-- 变更说明：重构 sandboxes 表结构（SQLite 版本）
-- 作者：axiang
-- 日期：2026-04-11
-- 影响范围：sandboxes 表
-- 回滚风险：高（SQLite 需要重建表）
-- 注意：SQLite 不支持 DROP COLUMN（3.35+ 支持），需要重建表

-- SQLite 正向变更（重建表方式）
-- 由于 SQLite 对 ALTER TABLE 的限制，需要采用重建表方式

-- 1. 创建新表结构
CREATE TABLE IF NOT EXISTS sandboxes_new (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    name TEXT DEFAULT NULL,
    image TEXT DEFAULT NULL,
    status TEXT DEFAULT NULL,
    container_id TEXT DEFAULT NULL,
    port INTEGER DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at TEXT DEFAULT NULL
);

-- 2. 复制现有数据（仅复制有效字段）
INSERT INTO sandboxes_new (id, thread_id, status, created_at)
SELECT id, thread_id, status, created_at FROM sandboxes;

-- 3. 删除旧表
DROP TABLE sandboxes;

-- 4. 重命名新表
ALTER TABLE sandboxes_new RENAME TO sandboxes;

-- 5. 创建索引
CREATE INDEX IF NOT EXISTS idx_sandboxes_thread_id ON sandboxes(thread_id);

-- 回滚语句（需要反向重建）
-- CREATE TABLE sandboxes_old (...);
-- INSERT INTO sandboxes_old SELECT ...;
-- DROP TABLE sandboxes;
-- ALTER TABLE sandboxes_old RENAME TO sandboxes;
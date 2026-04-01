-- 文件路径: isdp/sql-change/migrations/202604010001_drop_asset_packages_table.sql
-- 变更说明：删除 asset_packages 表，资产包不再存储元数据
-- 作者：AI Assistant
-- 日期：2026-04-01

SET NAMES utf8mb4;

-- 删除 asset_packages 表
DROP TABLE IF EXISTS asset_packages;

-- 回滚语句（如需回滚执行以下语句）
-- CREATE TABLE asset_packages (
--   id VARCHAR(36) PRIMARY KEY,
--   name VARCHAR(255) NOT NULL,
--   version VARCHAR(100),
--   description TEXT,
--   created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
--   updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
-- );
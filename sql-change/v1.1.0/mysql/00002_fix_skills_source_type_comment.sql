-- +goose Up
-- +goose StatementBegin

-- 变更说明：修正 skills.source_type 字段注释，使其与代码定义一致
-- 作者：axiang
-- 日期：2026-04-11
-- 影响范围：skills 表 source_type 字段注释

-- MySQL 正向变更（修改字段注释）
ALTER TABLE skills MODIFY COLUMN source_type VARCHAR(50) NOT NULL COMMENT '来源类型(platform/personal/federated)';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚语句
ALTER TABLE skills MODIFY COLUMN source_type VARCHAR(50) NOT NULL COMMENT '来源类型(built_in/uploaded/federated)';

-- +goose StatementEnd
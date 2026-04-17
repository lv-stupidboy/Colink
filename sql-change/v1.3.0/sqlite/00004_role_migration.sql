-- +goose Up
-- +goose StatementBegin

-- 将现有细分角色类型统一迁移为 'agent'
-- 现有值: requirement, architect, developer, reviewer, testengineer, devops, fullstack_engineer, custom
-- 新值: agent 或 human
UPDATE agent_configs SET role = 'agent'
WHERE role IN ('requirement', 'architect', 'developer', 'reviewer', 'testengineer', 'devops', 'fullstack_engineer', 'custom');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 无法精确回滚，保留现有值
-- +goose StatementEnd
-- +goose Up
-- P1优化：添加supports_multiplexing字段到base_agents表
ALTER TABLE base_agents ADD COLUMN supports_multiplexing INTEGER DEFAULT 0;

-- +goose Down
ALTER TABLE base_agents DROP COLUMN supports_multiplexing;
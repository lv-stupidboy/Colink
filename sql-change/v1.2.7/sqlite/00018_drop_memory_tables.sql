-- +goose Up
-- Colink memory now uses Markdown files:
--   project: <workspace>/.colink/memory/project.md
--   team:    ~/.colink/memory/team.md
-- Drop legacy SQLite memory tables if an older local database created them.
DROP TABLE IF EXISTS project_memories;
DROP TABLE IF EXISTS thread_memories;
DROP TABLE IF EXISTS team_memories;
DROP TABLE IF EXISTS memories;

-- +goose Down
-- Legacy SQLite memory tables are intentionally not recreated.

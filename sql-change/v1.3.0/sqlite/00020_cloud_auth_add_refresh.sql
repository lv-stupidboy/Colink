-- +goose Up
ALTER TABLE cloud_auth ADD COLUMN refresh_token TEXT NOT NULL DEFAULT '';
ALTER TABLE cloud_auth ADD COLUMN token_expires_at DATETIME;

-- +goose Down
ALTER TABLE cloud_auth DROP COLUMN refresh_token;
ALTER TABLE cloud_auth DROP COLUMN token_expires_at;

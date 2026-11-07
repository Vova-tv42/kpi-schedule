-- +goose Up
ALTER TABLE users ADD COLUMN schedule_locked BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE users DROP COLUMN schedule_locked;

-- +goose Up
CREATE TABLE bot_group_admins (
    group_id     TEXT NOT NULL,
    telegram_id  INTEGER NOT NULL,
    username     TEXT NOT NULL DEFAULT '',
    first_name   TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'invited',
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL,
    PRIMARY KEY (group_id, telegram_id)
);

CREATE INDEX idx_bot_group_admins_user ON bot_group_admins (telegram_id, status);

-- +goose Down
DROP INDEX IF EXISTS idx_bot_group_admins_user;
DROP TABLE IF EXISTS bot_group_admins;

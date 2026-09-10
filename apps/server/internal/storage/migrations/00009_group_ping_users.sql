-- +goose Up
CREATE TABLE bot_group_ping_users (
    group_id   TEXT NOT NULL REFERENCES bot_groups(id) ON DELETE CASCADE,
    username   TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (group_id, username)
);

CREATE INDEX idx_bot_group_ping_users_group ON bot_group_ping_users (group_id);

CREATE TABLE bot_group_ping_pending (
    id         TEXT PRIMARY KEY,
    group_id   TEXT NOT NULL REFERENCES bot_groups(id) ON DELETE CASCADE,
    chat_id    INTEGER NOT NULL,
    user_id    INTEGER NOT NULL,
    usernames  TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS bot_group_ping_pending;
DROP INDEX IF EXISTS idx_bot_group_ping_users_group;
DROP TABLE IF EXISTS bot_group_ping_users;

-- +goose Up
CREATE TABLE bot_groups (
    id                  TEXT PRIMARY KEY,
    creator_telegram_id INTEGER NOT NULL,
    academic_group_id   INTEGER NOT NULL,
    academic_group_name TEXT NOT NULL,
    faculty             TEXT NOT NULL DEFAULT '',
    telegram_chat_id    INTEGER,
    telegram_chat_title TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMP NOT NULL,
    updated_at          TIMESTAMP NOT NULL
);

CREATE INDEX idx_bot_groups_creator ON bot_groups (creator_telegram_id);
CREATE UNIQUE INDEX idx_bot_groups_chat_id ON bot_groups (telegram_chat_id) WHERE telegram_chat_id IS NOT NULL;

CREATE TABLE user_group_prompts (
    telegram_id       INTEGER PRIMARY KEY,
    prompt_message_id INTEGER NOT NULL,
    action            TEXT NOT NULL,
    group_id          TEXT,
    bind_chat_id      INTEGER,
    bind_chat_title   TEXT NOT NULL DEFAULT '',
    updated_at        TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE user_group_prompts;
DROP INDEX IF EXISTS idx_bot_groups_chat_id;
DROP INDEX IF EXISTS idx_bot_groups_creator;
DROP TABLE bot_groups;

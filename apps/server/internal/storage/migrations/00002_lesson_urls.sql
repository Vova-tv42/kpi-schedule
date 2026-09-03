-- +goose Up
CREATE TABLE user_lesson_urls (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject_norm TEXT NOT NULL,
    tag          TEXT NOT NULL DEFAULT '',
    url          TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL,
    UNIQUE (user_id, subject_norm, tag)
);

CREATE INDEX idx_user_lesson_urls_user ON user_lesson_urls (user_id);

CREATE TABLE user_url_prompts (
    user_id           TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    telegram_id       INTEGER NOT NULL UNIQUE,
    prompt_message_id INTEGER NOT NULL,
    subject_norm      TEXT NOT NULL,
    tag               TEXT NOT NULL DEFAULT '',
    subject_name      TEXT NOT NULL,
    updated_at        TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE user_url_prompts;
DROP TABLE user_lesson_urls;

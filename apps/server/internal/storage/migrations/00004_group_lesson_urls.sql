-- +goose Up
CREATE TABLE bot_group_lesson_urls (
    id           TEXT PRIMARY KEY,
    group_id     TEXT NOT NULL REFERENCES bot_groups(id) ON DELETE CASCADE,
    subject_norm TEXT NOT NULL,
    tag          TEXT NOT NULL DEFAULT '',
    url          TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL,
    UNIQUE (group_id, subject_norm, tag)
);

CREATE INDEX idx_bot_group_lesson_urls_group ON bot_group_lesson_urls (group_id);

ALTER TABLE user_group_prompts ADD COLUMN subject_norm TEXT NOT NULL DEFAULT '';
ALTER TABLE user_group_prompts ADD COLUMN tag TEXT NOT NULL DEFAULT '';
ALTER TABLE user_group_prompts ADD COLUMN subject_name TEXT NOT NULL DEFAULT '';

-- +goose Down
DROP TABLE bot_group_lesson_urls;

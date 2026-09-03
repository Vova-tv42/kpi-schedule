-- +goose Up
ALTER TABLE users ADD COLUMN notifications_enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE bot_groups ADD COLUMN notifications_enabled BOOLEAN NOT NULL DEFAULT 1;

CREATE TABLE sent_lesson_alerts (
    id             TEXT PRIMARY KEY,
    recipient_type TEXT NOT NULL CHECK (recipient_type IN ('user', 'group')),
    recipient_id   TEXT NOT NULL,
    lesson_date    TEXT NOT NULL,
    lesson_time    TEXT NOT NULL,
    alert_type     TEXT NOT NULL CHECK (alert_type IN ('before_10m', 'at_start')),
    sent_at        TIMESTAMP NOT NULL,
    UNIQUE (recipient_type, recipient_id, lesson_date, lesson_time, alert_type)
);

CREATE INDEX idx_sent_alerts_lookup ON sent_lesson_alerts (recipient_type, recipient_id, lesson_date, alert_type);

-- +goose Down
DROP INDEX IF EXISTS idx_sent_alerts_lookup;
DROP TABLE IF EXISTS sent_lesson_alerts;

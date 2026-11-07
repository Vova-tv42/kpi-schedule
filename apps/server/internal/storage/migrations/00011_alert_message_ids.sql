-- Remembers which Telegram message carried each lesson alert so the next cron
-- run can delete it (see docs/architecture/notifications-and-cron.md §5).
-- message_id is NULL when the send failed, and is cleared once the message has
-- been deleted, so "message_id IS NOT NULL" means "still visible in the chat".

-- +goose Up
ALTER TABLE sent_lesson_alerts ADD COLUMN chat_id INTEGER;
ALTER TABLE sent_lesson_alerts ADD COLUMN message_id INTEGER;

CREATE INDEX idx_sent_alerts_live_messages ON sent_lesson_alerts (message_id)
    WHERE message_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_sent_alerts_live_messages;
ALTER TABLE sent_lesson_alerts DROP COLUMN message_id;
ALTER TABLE sent_lesson_alerts DROP COLUMN chat_id;

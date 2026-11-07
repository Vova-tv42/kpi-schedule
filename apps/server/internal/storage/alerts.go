package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

// HasAlertBeenSent checks if a notification for a lesson has already been dispatched.
func (db *DB) HasAlertBeenSent(ctx context.Context, recipientType, recipientID, lessonDate, lessonTime string, alertType model.AlertType) (bool, error) {
	var exists int
	err := db.SQL.QueryRowContext(ctx, `
		SELECT 1 FROM sent_lesson_alerts
		WHERE recipient_type = ? AND recipient_id = ? AND lesson_date = ? AND lesson_time = ? AND alert_type = ?
	`, recipientType, recipientID, lessonDate, lessonTime, string(alertType)).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("checking sent alert: %w", err)
	}
	return true, nil
}

// RecordAlertSent records that a notification has been dispatched. messageID is
// the Telegram message that carried it (0 when the send failed); it is kept so
// a later cron run can delete the message.
func (db *DB) RecordAlertSent(ctx context.Context, recipientType, recipientID, lessonDate, lessonTime string, alertType model.AlertType, chatID, messageID int64) error {
	now := time.Now().UTC()
	id := uuid.New().String()
	var chatArg, msgArg any
	if messageID != 0 {
		chatArg, msgArg = chatID, messageID
	}
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO sent_lesson_alerts (id, recipient_type, recipient_id, lesson_date, lesson_time, alert_type, sent_at, chat_id, message_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (recipient_type, recipient_id, lesson_date, lesson_time, alert_type) DO NOTHING
	`, id, recipientType, recipientID, lessonDate, lessonTime, string(alertType), now, chatArg, msgArg)
	if err != nil {
		return fmt.Errorf("recording sent alert: %w", err)
	}
	return nil
}

// AlertMessage is a lesson alert whose Telegram message is still in the chat.
type AlertMessage struct {
	ID            string
	RecipientType string
	RecipientID   string
	AlertType     model.AlertType
	LessonDate    string // YYYY-MM-DD, Europe/Kyiv calendar day
	LessonTime    string // HH:MM[:SS], Europe/Kyiv wall clock
	ChatID        int64
	MessageID     int64
}

// ListLiveAlertMessages returns every recorded alert whose Telegram message has
// not been deleted yet, oldest first.
func (db *DB) ListLiveAlertMessages(ctx context.Context) ([]AlertMessage, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT id, recipient_type, recipient_id, alert_type, lesson_date, lesson_time, chat_id, message_id
		FROM sent_lesson_alerts
		WHERE message_id IS NOT NULL AND chat_id IS NOT NULL
		ORDER BY sent_at
	`)
	if err != nil {
		return nil, fmt.Errorf("listing live alert messages: %w", err)
	}
	defer rows.Close()

	var out []AlertMessage
	for rows.Next() {
		var m AlertMessage
		var alertType string
		if err := rows.Scan(&m.ID, &m.RecipientType, &m.RecipientID, &alertType, &m.LessonDate, &m.LessonTime, &m.ChatID, &m.MessageID); err != nil {
			return nil, fmt.Errorf("scanning live alert message: %w", err)
		}
		m.AlertType = model.AlertType(alertType)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating live alert messages: %w", err)
	}
	return out, nil
}

// ClearAlertMessage forgets the Telegram message of an alert once it has been
// deleted. The alert row itself stays, so deduplication is unaffected.
func (db *DB) ClearAlertMessage(ctx context.Context, id string) error {
	_, err := db.SQL.ExecContext(ctx, `
		UPDATE sent_lesson_alerts SET chat_id = NULL, message_id = NULL WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("clearing alert message: %w", err)
	}
	return nil
}

// CleanOldAlerts deletes alerts older than olderThan to prevent table unbounded growth.
func (db *DB) CleanOldAlerts(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().UTC().Add(-olderThan)
	_, err := db.SQL.ExecContext(ctx, `DELETE FROM sent_lesson_alerts WHERE sent_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("cleaning old alerts: %w", err)
	}
	return nil
}

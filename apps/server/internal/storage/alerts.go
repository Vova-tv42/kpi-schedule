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

// RecordAlertSent records that a notification has been successfully dispatched.
func (db *DB) RecordAlertSent(ctx context.Context, recipientType, recipientID, lessonDate, lessonTime string, alertType model.AlertType) error {
	now := time.Now().UTC()
	id := uuid.New().String()
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO sent_lesson_alerts (id, recipient_type, recipient_id, lesson_date, lesson_time, alert_type, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (recipient_type, recipient_id, lesson_date, lesson_time, alert_type) DO NOTHING
	`, id, recipientType, recipientID, lessonDate, lessonTime, string(alertType), now)
	if err != nil {
		return fmt.Errorf("recording sent alert: %w", err)
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

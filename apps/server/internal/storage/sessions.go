package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kpi-schedule-bot/server/internal/model"
)

// UpsertSession stores or replaces a user's encrypted cookie blob, marking status active.
func (db *DB) UpsertSession(ctx context.Context, userID uuid.UUID, ciphertext []byte, userAgent string) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO user_sessions (user_id, ciphertext, user_agent, status, synced_at, last_checked_at, last_error)
		VALUES ($1, $2, $3, 'active', now(), now(), NULL)
		ON CONFLICT (user_id) DO UPDATE
		SET ciphertext = EXCLUDED.ciphertext,
		    user_agent = EXCLUDED.user_agent,
		    status = 'active',
		    synced_at = now(),
		    last_checked_at = now(),
		    last_error = NULL
	`, userID, ciphertext, userAgent)
	if err != nil {
		return fmt.Errorf("upserting session: %w", err)
	}
	return nil
}

func (db *DB) GetSession(ctx context.Context, userID uuid.UUID) (model.UserSession, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT user_id, ciphertext, user_agent, status, synced_at, last_checked_at, last_error
		FROM user_sessions WHERE user_id = $1
	`, userID)

	var s model.UserSession
	var status string
	if err := row.Scan(&s.UserID, &s.Ciphertext, &s.UserAgent, &status, &s.SyncedAt, &s.LastCheckedAt, &s.LastError); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.UserSession{}, ErrNotFound
		}
		return model.UserSession{}, fmt.Errorf("fetching session: %w", err)
	}
	s.Status = model.SessionStatus(status)
	return s, nil
}

// MarkSessionStatus records the outcome of the last my.kpi.ua probe without
// touching the stored ciphertext, so an expired cookie is never silently discarded.
func (db *DB) MarkSessionStatus(ctx context.Context, userID uuid.UUID, status model.SessionStatus, lastError *string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE user_sessions
		SET status = $2, last_checked_at = now(), last_error = $3
		WHERE user_id = $1
	`, userID, string(status), lastError)
	if err != nil {
		return fmt.Errorf("marking session status: %w", err)
	}
	return nil
}

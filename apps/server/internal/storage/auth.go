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

var ErrInvalidOrExpiredCode = errors.New("invalid or expired pairing code")
var ErrInvalidToken = errors.New("invalid user token")
var ErrCodeCollision = errors.New("pairing code collision")

// CreatePairingCode stores a 6-digit pairing code with an expiration timestamp.
func (db *DB) CreatePairingCode(ctx context.Context, code string, telegramID int64, expiresAt time.Time) error {
	now := time.Now().UTC()
	// Purge expired codes
	_, _ = db.SQL.ExecContext(ctx, `DELETE FROM pairing_codes WHERE expires_at < ?`, now)

	// Clean any old active code for this specific telegramID
	_, _ = db.SQL.ExecContext(ctx, `DELETE FROM pairing_codes WHERE telegram_id = ?`, telegramID)

	// Insert new code; fail if collision on another user's active code
	res, err := db.SQL.ExecContext(ctx, `
		INSERT INTO pairing_codes (code, telegram_id, expires_at)
		VALUES (?, ?, ?)
		ON CONFLICT (code) DO NOTHING
	`, code, telegramID, expiresAt)
	if err != nil {
		return fmt.Errorf("creating pairing code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking inserted rows: %w", err)
	}
	if n == 0 {
		return ErrCodeCollision
	}
	return nil
}

// VerifyAndConsumePairingCode validates a pairing code, checks expiration, and deletes it (single use).
func (db *DB) VerifyAndConsumePairingCode(ctx context.Context, code string) (int64, error) {
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT telegram_id, expires_at
		FROM pairing_codes
		WHERE code = ?
	`, code)

	var telegramID int64
	var expiresAt time.Time
	if err := row.Scan(&telegramID, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidOrExpiredCode
		}
		return 0, fmt.Errorf("scanning pairing code: %w", err)
	}

	if time.Now().UTC().After(expiresAt) {
		_, _ = tx.ExecContext(ctx, `DELETE FROM pairing_codes WHERE code = ?`, code)
		_ = tx.Commit()
		return 0, ErrInvalidOrExpiredCode
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM pairing_codes WHERE code = ?`, code); err != nil {
		return 0, fmt.Errorf("deleting consumed pairing code: %w", err)
	}

	return telegramID, tx.Commit()
}

// CreateUserToken saves an authenticated client token for a user.
func (db *DB) CreateUserToken(ctx context.Context, userID uuid.UUID, token string) error {
	now := time.Now().UTC()
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO user_tokens (token, user_id, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT (token) DO UPDATE
		SET user_id = excluded.user_id, created_at = excluded.created_at
	`, token, userID, now)
	if err != nil {
		return fmt.Errorf("creating user token: %w", err)
	}
	return nil
}

// GetUserByToken resolves a user from their extension auth token.
func (db *DB) GetUserByToken(ctx context.Context, token string) (model.User, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT u.id, u.telegram_id, u.group_id, u.group_name, u.created_at, u.updated_at
		FROM users u
		JOIN user_tokens ut ON ut.user_id = u.id
		WHERE ut.token = ?
	`, token)

	var u model.User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.GroupID, &u.GroupName, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrInvalidToken
		}
		return model.User{}, fmt.Errorf("fetching user by token: %w", err)
	}
	return u, nil
}

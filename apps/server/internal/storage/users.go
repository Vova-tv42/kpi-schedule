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

var ErrNotFound = errors.New("not found")

// UpsertUser creates the user row if absent, or updates group_id/group_name
// if provided and the row already exists. Returns the resulting user.
func (db *DB) UpsertUser(ctx context.Context, telegramID int64, groupID *int, groupName *string) (model.User, error) {
	now := time.Now().UTC()
	row := db.SQL.QueryRowContext(ctx, `
		INSERT INTO users (id, telegram_id, group_id, group_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (telegram_id) DO UPDATE
		SET group_id = COALESCE(excluded.group_id, users.group_id),
		    group_name = COALESCE(excluded.group_name, users.group_name),
		    updated_at = excluded.updated_at
		RETURNING id, telegram_id, group_id, group_name, created_at, updated_at
	`, uuid.New(), telegramID, groupID, groupName, now, now)

	var u model.User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.GroupID, &u.GroupName, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return model.User{}, fmt.Errorf("upserting user: %w", err)
	}
	return u, nil
}

func (db *DB) GetUserByTelegramID(ctx context.Context, telegramID int64) (model.User, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT id, telegram_id, group_id, group_name, created_at, updated_at
		FROM users WHERE telegram_id = ?
	`, telegramID)

	var u model.User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.GroupID, &u.GroupName, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("fetching user: %w", err)
	}
	return u, nil
}

// DeleteUser removes the user and, via ON DELETE CASCADE, their schedule state and lessons.
func (db *DB) DeleteUser(ctx context.Context, id uuid.UUID) error {
	res, err := db.SQL.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kpi-schedule-bot/server/internal/model"
)

var ErrNotFound = errors.New("not found")

// UpsertUser creates the user row if absent, or updates group_id/group_name
// if provided and the row already exists. Returns the resulting user.
func (db *DB) UpsertUser(ctx context.Context, telegramID int64, groupID *int, groupName *string) (model.User, error) {
	row := db.Pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, group_id, group_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO UPDATE
		SET group_id = COALESCE(EXCLUDED.group_id, users.group_id),
		    group_name = COALESCE(EXCLUDED.group_name, users.group_name),
		    updated_at = now()
		RETURNING id, telegram_id, group_id, group_name, created_at, updated_at
	`, telegramID, groupID, groupName)

	var u model.User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.GroupID, &u.GroupName, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return model.User{}, fmt.Errorf("upserting user: %w", err)
	}
	return u, nil
}

func (db *DB) GetUserByTelegramID(ctx context.Context, telegramID int64) (model.User, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, telegram_id, group_id, group_name, created_at, updated_at
		FROM users WHERE telegram_id = $1
	`, telegramID)

	var u model.User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.GroupID, &u.GroupName, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("fetching user: %w", err)
	}
	return u, nil
}

// DeleteUser removes the user and, via ON DELETE CASCADE, their session and lessons.
func (db *DB) DeleteUser(ctx context.Context, id uuid.UUID) error {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

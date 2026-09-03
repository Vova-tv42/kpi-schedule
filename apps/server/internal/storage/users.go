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

const userColumns = `id, telegram_id, group_id, group_name, notifications_enabled, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.TelegramID, &u.GroupID, &u.GroupName, &u.NotificationsEnabled, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("scanning user: %w", err)
	}
	return u, nil
}

// UpsertUser creates the user row if absent, or updates group_id/group_name
// if provided and the row already exists. Returns the resulting user.
func (db *DB) UpsertUser(ctx context.Context, telegramID int64, groupID *int, groupName *string) (model.User, error) {
	now := time.Now().UTC()
	row := db.SQL.QueryRowContext(ctx, `
		INSERT INTO users (id, telegram_id, group_id, group_name, notifications_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (telegram_id) DO UPDATE
		SET group_id = COALESCE(excluded.group_id, users.group_id),
		    group_name = COALESCE(excluded.group_name, users.group_name),
		    updated_at = excluded.updated_at
		RETURNING `+userColumns+`
	`, uuid.New(), telegramID, groupID, groupName, now, now)

	return scanUser(row)
}

func (db *DB) GetUserByTelegramID(ctx context.Context, telegramID int64) (model.User, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT `+userColumns+`
		FROM users WHERE telegram_id = ?
	`, telegramID)

	return scanUser(row)
}

// SetUserNotifications toggles the notifications_enabled flag for a user.
func (db *DB) SetUserNotifications(ctx context.Context, telegramID int64, enabled bool) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE users
		SET notifications_enabled = ?, updated_at = ?
		WHERE telegram_id = ?
	`, enabled, now, telegramID)
	if err != nil {
		return fmt.Errorf("updating user notifications: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetUsersWithNotifications returns all users who have lesson notifications enabled.
func (db *DB) GetUsersWithNotifications(ctx context.Context) ([]model.User, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE notifications_enabled = 1
	`)
	if err != nil {
		return nil, fmt.Errorf("querying users with notifications: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
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

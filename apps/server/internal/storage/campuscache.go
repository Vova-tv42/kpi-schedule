package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CacheGet looks up key in the disk-backed Campus API cache and, if present
// and fetched within maxAge, JSON-decodes its value into out and returns
// true. A miss (absent or stale) returns false with no error — the caller
// should just fetch fresh and call CacheSet.
func (db *DB) CacheGet(ctx context.Context, key string, maxAge time.Duration, out any) (bool, error) {
	row := db.SQL.QueryRowContext(ctx, `SELECT value, fetched_at FROM campus_cache WHERE key = ?`, key)

	var value string
	var fetchedAt time.Time
	if err := row.Scan(&value, &fetchedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("reading cache entry %q: %w", key, err)
	}

	if time.Since(fetchedAt) > maxAge {
		return false, nil
	}
	if err := json.Unmarshal([]byte(value), out); err != nil {
		return false, fmt.Errorf("decoding cache entry %q: %w", key, err)
	}
	return true, nil
}

// CacheSet stores value under key, JSON-encoded, stamped with the current
// time as fetched_at.
func (db *DB) CacheSet(ctx context.Context, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding cache entry %q: %w", key, err)
	}

	_, err = db.SQL.ExecContext(ctx, `
		INSERT INTO campus_cache (key, value, fetched_at)
		VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, fetched_at = excluded.fetched_at
	`, key, string(data), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("writing cache entry %q: %w", key, err)
	}
	return nil
}

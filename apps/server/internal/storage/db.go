// Package storage wraps the SQLite connection and embedded migrations.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// dsn builds the SQLite connection string for path, with the pragmas this
// package relies on: WAL for concurrent-safe reads during a write, foreign
// keys (off by default in SQLite otherwise, needed for ON DELETE CASCADE),
// a busy-timeout fallback, and sqlite time format/UTC so time.Time values
// round-trip through TIMESTAMP columns without manual parsing. _timezone
// must be "UTC" (uppercase) — modernc.org/sqlite's DSN parser is case-sensitive
// and "utc" fails with "unknown time zone utc".
func dsn(path string) string {
	return fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_journal=WAL&_time_format=sqlite&_timezone=UTC",
		path,
	)
}

type DB struct {
	SQL *sql.DB
}

// Open opens the SQLite database at path, creating its parent directory if
// needed (SQLite creates the file itself, but not the directory).
func Open(ctx context.Context, path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	// A single connection serializes all access through SQLite's one writer,
	// avoiding SQLITE_BUSY entirely rather than relying on retries — the
	// simplest correct choice at this project's scale.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("pinging sqlite: %w", err)
	}
	return &DB{SQL: sqlDB}, nil
}

func (db *DB) Close() {
	db.SQL.Close()
}

func Migrate(path string) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating db directory: %w", err)
		}
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	sqlDB, err := goose.OpenDBWithDriver("sqlite3", dsn(path))
	if err != nil {
		return fmt.Errorf("opening db for migrations: %w", err)
	}
	defer sqlDB.Close()

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func setupAdminTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "admin_test.db")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create test tables
	_, err = db.SQL.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			telegram_id INTEGER NOT NULL,
			group_name TEXT,
			created_at TIMESTAMP NOT NULL
		);
		INSERT INTO users (id, telegram_id, group_name, created_at) VALUES 
		('u1', 111, 'IA-01', datetime('now')),
		('u2', 222, 'IA-02', datetime('now'));
	`)
	if err != nil {
		t.Fatalf("seeding test db: %v", err)
	}
	return db
}

func TestGetTables(t *testing.T) {
	db := setupAdminTestDB(t)
	defer db.Close()

	tables, err := db.GetTables(context.Background())
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if tables[0].Name != "users" {
		t.Errorf("expected table 'users', got %q", tables[0].Name)
	}
	if tables[0].RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", tables[0].RowCount)
	}
}

func TestGetTableRows(t *testing.T) {
	db := setupAdminTestDB(t)
	defer db.Close()

	ctx := context.Background()
	data, err := db.GetTableRows(ctx, "users", 10, 0, "telegram_id", "DESC")
	if err != nil {
		t.Fatalf("GetTableRows failed: %v", err)
	}
	if data.Total != 2 {
		t.Errorf("expected total 2, got %d", data.Total)
	}
	if len(data.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(data.Rows))
	}
	// Sorted DESC by telegram_id, u2 should be first
	firstID, ok := data.Rows[0]["id"].(string)
	if !ok || firstID != "u2" {
		t.Errorf("expected first row to be u2, got %v", data.Rows[0]["id"])
	}

	// Non-existent table
	_, err = db.GetTableRows(ctx, "non_existent", 10, 0, "", "")
	if err == nil {
		t.Error("expected error for non_existent table, got nil")
	}
}

func TestUpdateTableRow(t *testing.T) {
	db := setupAdminTestDB(t)
	defer db.Close()

	ctx := context.Background()
	err := db.UpdateTableRow(ctx, "users", "id", "u1", map[string]any{
		"group_name": "IA-99",
	})
	if err != nil {
		t.Fatalf("UpdateTableRow failed: %v", err)
	}

	var updatedName string
	err = db.SQL.QueryRow("SELECT group_name FROM users WHERE id = 'u1'").Scan(&updatedName)
	if err != nil {
		t.Fatalf("scanning updated row: %v", err)
	}
	if updatedName != "IA-99" {
		t.Errorf("expected 'IA-99', got %q", updatedName)
	}

	// Updating non-existent table
	err = db.UpdateTableRow(ctx, "non_existent", "id", "u1", map[string]any{
		"group_name": "IA-99",
	})
	if err == nil {
		t.Error("expected error for non_existent table, got nil")
	}
}

func TestExecuteAdminQuery(t *testing.T) {
	db := setupAdminTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Select query
	selRes, err := db.ExecuteAdminQuery(ctx, "SELECT id, telegram_id FROM users ORDER BY id ASC")
	if err != nil {
		t.Fatalf("ExecuteAdminQuery SELECT failed: %v", err)
	}
	if len(selRes.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(selRes.Rows))
	}
	if len(selRes.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(selRes.Columns))
	}

	// Query with leading line comment
	commentRes, err := db.ExecuteAdminQuery(ctx, "-- query comments\nSELECT id FROM users")
	if err != nil {
		t.Fatalf("ExecuteAdminQuery with comment failed: %v", err)
	}
	if len(commentRes.Rows) != 2 {
		t.Errorf("expected 2 rows from commented query, got %d", len(commentRes.Rows))
	}

	// Query with WITH CTE
	cteRes, err := db.ExecuteAdminQuery(ctx, "WITH active AS (SELECT id FROM users) SELECT id FROM active")
	if err != nil {
		t.Fatalf("ExecuteAdminQuery with WITH CTE failed: %v", err)
	}
	if len(cteRes.Rows) != 2 {
		t.Errorf("expected 2 rows from CTE query, got %d", len(cteRes.Rows))
	}

	// Update query
	updRes, err := db.ExecuteAdminQuery(ctx, "UPDATE users SET group_name = 'IA-XX' WHERE id = 'u2'")
	if err != nil {
		t.Fatalf("ExecuteAdminQuery UPDATE failed: %v", err)
	}
	if updRes.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", updRes.RowsAffected)
	}

	// Truncation test: seed > 1000 rows
	_, err = db.SQL.Exec(`
		CREATE TABLE bulk_test (n INTEGER);
		WITH RECURSIVE cnt(x) AS (
			SELECT 1 UNION ALL SELECT x+1 FROM cnt WHERE x < 1050
		) INSERT INTO bulk_test SELECT x FROM cnt;
	`)
	if err != nil {
		t.Fatalf("seeding bulk_test: %v", err)
	}

	bulkRes, err := db.ExecuteAdminQuery(ctx, "SELECT n FROM bulk_test")
	if err != nil {
		t.Fatalf("ExecuteAdminQuery bulk_test failed: %v", err)
	}
	if len(bulkRes.Rows) != 1000 {
		t.Errorf("expected 1000 rows, got %d", len(bulkRes.Rows))
	}
	if !bulkRes.Truncated {
		t.Errorf("expected bulkRes.Truncated to be true, got false")
	}
}

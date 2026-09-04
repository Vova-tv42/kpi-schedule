package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/storage"
)

func setupTestRouter(t *testing.T, adminSecret string) (http.Handler, *storage.DB) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "admin_api_test.db")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.SQL.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			telegram_id INTEGER NOT NULL,
			group_name TEXT,
			created_at TIMESTAMP NOT NULL
		);
		INSERT INTO users (id, telegram_id, group_name, created_at) VALUES 
		('u1', 111, 'IA-01', datetime('now'));
	`)
	if err != nil {
		t.Fatalf("seeding db: %v", err)
	}

	campusClient := campus.NewClient(db)
	svc := NewService(db, campusClient)
	router := NewRouterWithOpts(svc, "internal-token", RouterOpts{
		AdminSecret: adminSecret,
	})

	return router, db
}

func TestAdminAuth(t *testing.T) {
	router, _ := setupTestRouter(t, "super-secret")

	// 1. Missing secret
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tables", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing header, got %d", rec.Code)
	}

	// 2. Wrong secret
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/tables", nil)
	req.Header.Set("X-Admin-Secret", "wrong-secret")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong header, got %d", rec.Code)
	}

	// 3. Valid secret
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/tables", nil)
	req.Header.Set("X-Admin-Secret", "super-secret")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with valid secret, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetTablesAndRows(t *testing.T) {
	router, _ := setupTestRouter(t, "super-secret")

	// 1. Get tables
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tables", nil)
	req.Header.Set("X-Admin-Secret", "super-secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var tablesResp struct {
		Tables []storage.TableInfo `json:"tables"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tablesResp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(tablesResp.Tables) != 1 || tablesResp.Tables[0].Name != "users" {
		t.Errorf("unexpected tables: %+v", tablesResp)
	}

	// 2. Get rows
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/tables/users", nil)
	req.Header.Set("X-Admin-Secret", "super-secret")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for table rows, got %d: %s", rec.Code, rec.Body.String())
	}

	var rowsResp storage.TableData
	if err := json.Unmarshal(rec.Body.Bytes(), &rowsResp); err != nil {
		t.Fatalf("decoding rows response: %v", err)
	}
	if rowsResp.Total != 1 || len(rowsResp.Rows) != 1 {
		t.Errorf("unexpected rows data: %+v", rowsResp)
	}
}

func TestAdminRolePermissions(t *testing.T) {
	router, _ := setupTestRouter(t, "super-secret")

	// 1. Read-only user attempts to update row -> 403 Forbidden
	updatePayload := []byte(`{"primary_key_column":"id","primary_key_value":"u1","updates":{"group_name":"IA-02"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/tables/users/row", bytes.NewReader(updatePayload))
	req.Header.Set("X-Admin-Secret", "super-secret")
	req.Header.Set("X-Admin-Role", "read-only")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for read-only row update, got %d", rec.Code)
	}

	// 2. Read-only user attempts custom query -> 403 Forbidden
	queryPayload := []byte(`{"query":"SELECT * FROM users"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/query", bytes.NewReader(queryPayload))
	req.Header.Set("X-Admin-Secret", "super-secret")
	req.Header.Set("X-Admin-Role", "read-only")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for read-only custom query, got %d", rec.Code)
	}

	// 3. Read-write user updates row -> 200 OK
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/tables/users/row", bytes.NewReader(updatePayload))
	req.Header.Set("X-Admin-Secret", "super-secret")
	req.Header.Set("X-Admin-Role", "read-write")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for read-write row update, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Read-write user executes query -> 200 OK
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/query", bytes.NewReader(queryPayload))
	req.Header.Set("X-Admin-Secret", "super-secret")
	req.Header.Set("X-Admin-Role", "read-write")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for read-write query, got %d: %s", rec.Code, rec.Body.String())
	}
}

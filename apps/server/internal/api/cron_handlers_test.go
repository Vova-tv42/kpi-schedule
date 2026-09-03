package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"kpi-schedule-bot/server/internal/alerts"
	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/storage"
)

func TestCronHandler(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_cron.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	d := alerts.NewDispatcher(db, nil, nil)
	cronSecret := "test-secret-123"
	handler := api.NewCronHandler(d, cronSecret)

	router := api.NewRouterWithOpts(nil, "internal-tok", api.RouterOpts{
		CronHandler: handler,
	})

	// 1. Unauthorized request (no token)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cron/lesson-alerts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. Unauthorized request (wrong token)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cron/lesson-alerts", nil)
	req.Header.Set("Authorization", "Bearer wrong-secret")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 3. Authorized request via Authorization header
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cron/lesson-alerts", nil)
	req.Header.Set("Authorization", "Bearer test-secret-123")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// 4. Authorized request via GET with ?secret= query param
	req = httptest.NewRequest(http.MethodGet, "/api/v1/cron/lesson-alerts?secret=test-secret-123", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/storage"
)

func TestTelegramWebhookRoute(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrating db: %v", err)
	}

	db, err := storage.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	svc := NewService(db, campus.NewClient(db))
	token := "test-internal-token"

	// 1. Without webhook handler: route returns 404
	routerWithoutWebhook := NewRouter(svc, token)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	routerWithoutWebhook.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when webhook handler is nil, got %d", w.Code)
	}

	// 2. With webhook handler: route delegates to handler
	called := false
	dummyHandler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	routerWithWebhook := NewRouter(svc, token, dummyHandler)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", strings.NewReader(`{}`))
	w = httptest.NewRecorder()
	routerWithWebhook.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from webhook handler, got %d", w.Code)
	}
	if !called {
		t.Fatal("expected dummyHandler to be called")
	}

	// 3. Webhook route must NOT be subject to IP rate limiting (20 req/min limit)
	for i := 0; i < 25; i++ {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", strings.NewReader(`{}`))
		req.RemoteAddr = "192.0.2.1:12345"
		w = httptest.NewRecorder()
		routerWithWebhook.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("webhook route was rate limited on request %d", i+1)
		}
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 on request %d, got %d", i+1, w.Code)
		}
	}
}

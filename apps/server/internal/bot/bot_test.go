package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/storage"
)

func TestBotWebhookAuthentication(t *testing.T) {
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

	svc := api.NewService(db, campus.NewClient(db))

	// Create bot with DisableTokenCheck so it doesn't query Telegram API during test
	b, err := New("123456789:AAFakeTokenForTestingWebhookHandler", svc, db, &gotgbot.BotOpts{
		DisableTokenCheck: true,
	})
	if err != nil {
		t.Fatalf("creating bot: %v", err)
	}
	defer b.Stop()

	secret := "test-secret-token-xyz"
	if err := b.AddWebhook(secret); err != nil {
		t.Fatalf("adding webhook: %v", err)
	}

	handler := b.WebhookHandler()

	// 1. Missing secret header -> 401 Unauthorized
	req := httptest.NewRequest(http.MethodPost, WebhookPath, strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing secret header, got %d", w.Code)
	}

	// 2. Wrong secret header -> 401 Unauthorized
	req = httptest.NewRequest(http.MethodPost, WebhookPath, strings.NewReader(`{}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid secret header, got %d", w.Code)
	}

	// 3. Valid secret header -> 200 OK
	req = httptest.NewRequest(http.MethodPost, WebhookPath, strings.NewReader(`{}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid secret header, got %d", w.Code)
	}

	// Give the async dispatcher a brief moment to process the raw update before teardown
	time.Sleep(20 * time.Millisecond)
}

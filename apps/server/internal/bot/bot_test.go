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

func TestInstallAndOnboardingScreens(t *testing.T) {
	// 1. Check formatInstallScreen content
	installText := formatInstallScreen()
	if !strings.Contains(installText, "Встановлення розширення") {
		t.Errorf("formatInstallScreen missing title: %s", installText)
	}
	if !strings.Contains(installText, "chrome://extensions") {
		t.Errorf("formatInstallScreen missing chrome://extensions: %s", installText)
	}
	if !strings.Contains(installText, "Режим розробника") {
		t.Errorf("formatInstallScreen missing developer mode note: %s", installText)
	}

	// 2. Check installKeyboard with URL
	kbWithURL := installKeyboard("https://example.com/download/extension.zip")
	if len(kbWithURL.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows in installKeyboard with URL, got %d", len(kbWithURL.InlineKeyboard))
	}
	if kbWithURL.InlineKeyboard[0][0].Url != "https://example.com/download/extension.zip" {
		t.Errorf("expected URL on first button, got %s", kbWithURL.InlineKeyboard[0][0].Url)
	}
	if kbWithURL.InlineKeyboard[1][0].CallbackData != menuCallbackData("link") {
		t.Errorf("expected link callback on second button, got %s", kbWithURL.InlineKeyboard[1][0].CallbackData)
	}
	if kbWithURL.InlineKeyboard[2][0].CallbackData != menuCallbackData("back") {
		t.Errorf("expected back callback on third button, got %s", kbWithURL.InlineKeyboard[2][0].CallbackData)
	}

	// 3. Check installKeyboard without URL
	kbNoURL := installKeyboard("")
	if len(kbNoURL.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in installKeyboard without URL, got %d", len(kbNoURL.InlineKeyboard))
	}

	// 4. Check startKeyboard buttons
	startKbNone := startKeyboard(linkStateNone)
	if len(startKbNone.InlineKeyboard) != 1 || len(startKbNone.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 1 row with 2 buttons in startKeyboard(linkStateNone), got %+v", startKbNone.InlineKeyboard)
	}
	if startKbNone.InlineKeyboard[0][0].CallbackData != menuCallbackData("install") {
		t.Errorf("expected install button first, got %s", startKbNone.InlineKeyboard[0][0].CallbackData)
	}
	if startKbNone.InlineKeyboard[0][1].CallbackData != menuCallbackData("link") {
		t.Errorf("expected link button second, got %s", startKbNone.InlineKeyboard[0][1].CallbackData)
	}

	// 5. Check startKeyboard fresh state has schedule button
	startKbFresh := startKeyboard(linkStateFresh)
	if len(startKbFresh.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in startKeyboard(linkStateFresh), got %d", len(startKbFresh.InlineKeyboard))
	}

	// 6. Check linkKeyboard has install button
	linkKb := linkKeyboard()
	foundInstall := false
	for _, row := range linkKb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == menuCallbackData("install") {
				foundInstall = true
			}
		}
	}
	if !foundInstall {
		t.Errorf("linkKeyboard missing install button")
	}

	// 7. Check formatStartScreen text variations
	textNone := formatStartScreen(linkStateNone)
	textFresh := formatStartScreen(linkStateFresh)
	textStale := formatStartScreen(linkStateStale)

	if !strings.Contains(textNone, "персональний розклад КПІ") {
		t.Errorf("textNone missing header: %s", textNone)
	}
	if !strings.Contains(textFresh, "Твій розклад уже синхронізовано") {
		t.Errorf("textFresh missing synced note: %s", textFresh)
	}
	if !strings.Contains(textStale, "Розклад застарів") {
		t.Errorf("textStale missing stale note: %s", textStale)
	}
}

func TestBotExtensionDownloadURL(t *testing.T) {
	b := &Bot{}
	if b.ExtensionDownloadURL() != "" {
		t.Errorf("expected empty initial download URL, got %s", b.ExtensionDownloadURL())
	}
	b.SetExtensionDownloadURL("https://cdn.example.com/ext.zip")
	if b.ExtensionDownloadURL() != "https://cdn.example.com/ext.zip" {
		t.Errorf("expected set download URL, got %s", b.ExtensionDownloadURL())
	}
}

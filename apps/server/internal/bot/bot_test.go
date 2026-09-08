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
	// 1. Check formatInstallScreen content (method choice)
	installText := formatInstallScreen()
	if !strings.Contains(installText, "Як підключити розклад") {
		t.Errorf("formatInstallScreen missing title: %s", installText)
	}
	if !strings.Contains(installText, "Браузерне розширення") {
		t.Errorf("formatInstallScreen missing extension option: %s", installText)
	}
	if !strings.Contains(installText, "Скрипт для консолі") {
		t.Errorf("formatInstallScreen missing console script option: %s", installText)
	}

	// 2. Check installKeyboard (2 choices + back)
	installKb := installKeyboard()
	if len(installKb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in installKeyboard, got %d", len(installKb.InlineKeyboard))
	}
	if len(installKb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 2 buttons in first row of installKeyboard, got %d", len(installKb.InlineKeyboard[0]))
	}
	if installKb.InlineKeyboard[0][0].CallbackData != menuCallbackData("install_ext") {
		t.Errorf("expected install_ext on first button, got %s", installKb.InlineKeyboard[0][0].CallbackData)
	}
	if installKb.InlineKeyboard[0][1].CallbackData != menuCallbackData("install_script") {
		t.Errorf("expected install_script on second button, got %s", installKb.InlineKeyboard[0][1].CallbackData)
	}
	if installKb.InlineKeyboard[1][0].CallbackData != menuCallbackData("back") {
		t.Errorf("expected back on second row, got %s", installKb.InlineKeyboard[1][0].CallbackData)
	}

	// 3. Check extensionKeyboard with and without URL
	kbWithURL := extensionKeyboard("https://example.com/download/extension.zip")
	if len(kbWithURL.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows in extensionKeyboard with URL, got %d", len(kbWithURL.InlineKeyboard))
	}
	if kbWithURL.InlineKeyboard[0][0].Url != "https://example.com/download/extension.zip" {
		t.Errorf("expected URL on first button, got %s", kbWithURL.InlineKeyboard[0][0].Url)
	}
	if kbWithURL.InlineKeyboard[1][0].CallbackData != menuCallbackData("link") {
		t.Errorf("expected link callback on second button, got %s", kbWithURL.InlineKeyboard[1][0].CallbackData)
	}
	if kbWithURL.InlineKeyboard[2][0].CallbackData != menuCallbackData("install") {
		t.Errorf("expected install callback on third button, got %s", kbWithURL.InlineKeyboard[2][0].CallbackData)
	}

	kbNoURL := extensionKeyboard("")
	if len(kbNoURL.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in extensionKeyboard without URL, got %d", len(kbNoURL.InlineKeyboard))
	}

	// 4. Check formatExtensionInstructions
	extText := formatExtensionInstructions()
	if !strings.Contains(extText, "chrome://extensions") {
		t.Errorf("formatExtensionInstructions missing chrome://extensions: %s", extText)
	}
	if !strings.Contains(extText, "Режим розробника") {
		t.Errorf("formatExtensionInstructions missing developer mode note: %s", extText)
	}

	// 5. Check console script building and rendering
	script := buildConsoleScript("https://test.fly.dev", "123456")
	if !strings.Contains(script, "https://test.fly.dev/api/v1/schedule/raw-sync") {
		t.Errorf("buildConsoleScript missing endpoint URL: %s", script)
	}
	if !strings.Contains(script, "123456") {
		t.Errorf("buildConsoleScript missing pair code: %s", script)
	}

	scriptText := formatConsoleScriptScreen(script, 600)
	if !strings.Contains(scriptText, "my.kpi.ua") {
		t.Errorf("formatConsoleScriptScreen missing my.kpi.ua: %s", scriptText)
	}
	if !strings.Contains(scriptText, "<pre><code class=\"language-javascript\">") {
		t.Errorf("formatConsoleScriptScreen missing code block: %s", scriptText)
	}

	scriptKb := consoleScriptKeyboard()
	if len(scriptKb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in consoleScriptKeyboard, got %d", len(scriptKb.InlineKeyboard))
	}
	if scriptKb.InlineKeyboard[0][0].CallbackData != menuCallbackData("install_script") {
		t.Errorf("expected refresh on first button, got %s", scriptKb.InlineKeyboard[0][0].CallbackData)
	}

	// 6. Check startKeyboard buttons
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
	b.SetExtensionInstallURL("https://drive.google.com/test-install")
	if b.ExtensionInstallURL() != "https://drive.google.com/test-install" {
		t.Errorf("expected set install URL, got %s", b.ExtensionInstallURL())
	}
}

func TestDayKeyboard(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	kb := dayKeyboard(now)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 3 {
		t.Fatalf("unexpected keyboard shape: %+v", kb)
	}
	if kb.InlineKeyboard[0][0].Text != "◀️" {
		t.Errorf("expected prev button text '◀️', got %q", kb.InlineKeyboard[0][0].Text)
	}
	if kb.InlineKeyboard[0][1].Text != "📅 Сьогодні" {
		t.Errorf("expected today button text '📅 Сьогодні', got %q", kb.InlineKeyboard[0][1].Text)
	}
	if kb.InlineKeyboard[0][2].Text != "▶️" {
		t.Errorf("expected next button text '▶️', got %q", kb.InlineKeyboard[0][2].Text)
	}
}

func TestTomorrowDayKeyboard(t *testing.T) {
	tomorrow := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	kb := dayKeyboard(tomorrow)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 3 {
		t.Fatalf("unexpected keyboard shape: %+v", kb)
	}
	if kb.InlineKeyboard[0][0].Text != "◀️" {
		t.Errorf("expected prev button text '◀️', got %q", kb.InlineKeyboard[0][0].Text)
	}
	if kb.InlineKeyboard[0][0].CallbackData != "nav:prev:2026-09-03" {
		t.Errorf("expected prev callback data 'nav:prev:2026-09-03', got %q", kb.InlineKeyboard[0][0].CallbackData)
	}
	if kb.InlineKeyboard[0][1].Text != "📅 Сьогодні" {
		t.Errorf("expected today button text '📅 Сьогодні', got %q", kb.InlineKeyboard[0][1].Text)
	}
	if kb.InlineKeyboard[0][1].CallbackData != "nav:today:2026-09-03" {
		t.Errorf("expected today callback data 'nav:today:2026-09-03', got %q", kb.InlineKeyboard[0][1].CallbackData)
	}
	if kb.InlineKeyboard[0][2].Text != "▶️" {
		t.Errorf("expected next button text '▶️', got %q", kb.InlineKeyboard[0][2].Text)
	}
	if kb.InlineKeyboard[0][2].CallbackData != "nav:next:2026-09-03" {
		t.Errorf("expected next callback data 'nav:next:2026-09-03', got %q", kb.InlineKeyboard[0][2].CallbackData)
	}
}

func TestTomorrowGroupDayKeyboard(t *testing.T) {
	tomorrow := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	kb := groupDayKeyboard(tomorrow, 4402)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 3 {
		t.Fatalf("unexpected keyboard shape: %+v", kb)
	}
	if kb.InlineKeyboard[0][0].Text != "◀️" {
		t.Errorf("expected prev button text '◀️', got %q", kb.InlineKeyboard[0][0].Text)
	}
	if kb.InlineKeyboard[0][0].CallbackData != "gnav:prev:2026-09-03:4402" {
		t.Errorf("expected prev callback data 'gnav:prev:2026-09-03:4402', got %q", kb.InlineKeyboard[0][0].CallbackData)
	}
	if kb.InlineKeyboard[0][1].Text != "📅 Сьогодні" {
		t.Errorf("expected today button text '📅 Сьогодні', got %q", kb.InlineKeyboard[0][1].Text)
	}
	if kb.InlineKeyboard[0][1].CallbackData != "gnav:today:2026-09-03:4402" {
		t.Errorf("expected today callback data 'gnav:today:2026-09-03:4402', got %q", kb.InlineKeyboard[0][1].CallbackData)
	}
	if kb.InlineKeyboard[0][2].Text != "▶️" {
		t.Errorf("expected next button text '▶️', got %q", kb.InlineKeyboard[0][2].Text)
	}
	if kb.InlineKeyboard[0][2].CallbackData != "gnav:next:2026-09-03:4402" {
		t.Errorf("expected next callback data 'gnav:next:2026-09-03:4402', got %q", kb.InlineKeyboard[0][2].CallbackData)
	}
}

func TestSetupCommandsIncludesTomorrow(t *testing.T) {
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
	b, err := New("123456789:AAFakeTokenForTestingWebhookHandler", svc, db, &gotgbot.BotOpts{
		DisableTokenCheck: true,
	})
	if err != nil {
		t.Fatalf("creating bot: %v", err)
	}
	defer b.Stop()

	// SetupCommands won't crash even if network calls fail (it logs warnings)
	if err := b.SetupCommands(); err != nil {
		t.Errorf("SetupCommands returned error: %v", err)
	}
}

func TestCommandScopesAndDescriptions(t *testing.T) {
	privCmds := PrivateCommands()
	privMap := make(map[string]string)
	for _, c := range privCmds {
		privMap[c.Command] = c.Description
	}

	if privMap["today"] != "Показати розклад на сьогодні" {
		t.Errorf("unexpected private today desc: %q", privMap["today"])
	}
	if privMap["tomorrow"] != "Показати розклад на завтра" {
		t.Errorf("unexpected private tomorrow desc: %q", privMap["tomorrow"])
	}
	if privMap["week"] != "Показати розклад на тиждень" {
		t.Errorf("unexpected private week desc: %q", privMap["week"])
	}
	if privMap["install"] != "Інструкція та підключення розкладу" {
		t.Errorf("unexpected private install desc: %q", privMap["install"])
	}
	if _, ok := privMap["me_today"]; ok {
		t.Errorf("private commands should not list me_today")
	}

	grpCmds := GroupCommands()
	grpMap := make(map[string]string)
	for _, c := range grpCmds {
		grpMap[c.Command] = c.Description
	}

	expectedGrp := map[string]string{
		"today":          "Показати розклад групи на сьогодні",
		"tomorrow":       "Показати розклад групи на завтра",
		"week":           "Показати розклад групи на тиждень",
		"me_today":       "Показати персональний розклад на сьогодні",
		"me_tomorrow":    "Показати персональний розклад на завтра",
		"me_week":        "Показати персональний розклад на тиждень",
		"group_url_sync": "Синхронізувати посилання з розкладу групи",
	}

	if len(grpMap) != len(expectedGrp) {
		t.Fatalf("expected %d group commands, got %d: %+v", len(expectedGrp), len(grpMap), grpMap)
	}

	for cmd, desc := range expectedGrp {
		if grpMap[cmd] != desc {
			t.Errorf("expected command %s to have description %q, got %q", cmd, desc, grpMap[cmd])
		}
	}

	if _, ok := grpMap["group_today"]; ok {
		t.Errorf("group commands should not list deprecated /group_today")
	}
	if _, ok := grpMap["group_week"]; ok {
		t.Errorf("group commands should not list deprecated /group_week")
	}
	if _, ok := grpMap["me_group"]; ok {
		t.Errorf("group commands should not list me_group")
	}

	adminCmds := AdminCommands()
	adminMap := make(map[string]string)
	for _, c := range adminCmds {
		adminMap[c.Command] = c.Description
	}

	if adminMap["group"] != "Керування академічною групою" {
		t.Errorf("expected admin command 'group' to be present, got %q", adminMap["group"])
	}
	for cmd, desc := range expectedGrp {
		if adminMap[cmd] != desc {
			t.Errorf("expected admin command %s to have description %q, got %q", cmd, desc, adminMap[cmd])
		}
	}
}


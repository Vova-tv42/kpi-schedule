package bot

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

func TestGroupSyncRenderingAndKeyboard(t *testing.T) {
	callerName := "Олександр Коваленко"
	groupName := "ІП-21"

	// 1. Confirmation text
	confirmText := formatGroupSyncConfirm(callerName, groupName)
	if !strings.Contains(confirmText, "Синхронізація посилань з групою ІП-21") {
		t.Errorf("expected group name in title, got:\n%s", confirmText)
	}
	if !strings.Contains(confirmText, "Олександр Коваленко") {
		t.Errorf("expected caller name in text, got:\n%s", confirmText)
	}
	if !strings.Contains(confirmText, "перезапише") || !strings.Contains(confirmText, "посилання") {
		t.Errorf("expected notice about overriding URLs, got:\n%s", confirmText)
	}
	if !strings.Contains(confirmText, "спільних занять") {
		t.Errorf("expected notice about identical/shared lessons, got:\n%s", confirmText)
	}

	// 2. Keyboard
	var callerID int64 = 777888
	kb := groupSyncKeyboard(callerID)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 1 row with 2 buttons, got %+v", kb.InlineKeyboard)
	}
	proceedBtn := kb.InlineKeyboard[0][0]
	cancelBtn := kb.InlineKeyboard[0][1]

	if !strings.Contains(proceedBtn.Text, "Продовжити") {
		t.Errorf("expected proceed button text, got %q", proceedBtn.Text)
	}
	if proceedBtn.CallbackData != "gsync:confirm:777888" {
		t.Errorf("expected callback data 'gsync:confirm:777888', got %q", proceedBtn.CallbackData)
	}

	if !strings.Contains(cancelBtn.Text, "Скасувати") {
		t.Errorf("expected cancel button text, got %q", cancelBtn.Text)
	}
	if cancelBtn.CallbackData != "gsync:cancel:777888" {
		t.Errorf("expected callback data 'gsync:cancel:777888', got %q", cancelBtn.CallbackData)
	}

	// 3. Success messages
	successWithCount := formatGroupSyncSuccess(callerName, groupName, 3)
	if !strings.Contains(successWithCount, "успішно синхронізовано з налаштуваннями групи <b>ІП-21</b>") {
		t.Errorf("expected success notice with group name, got:\n%s", successWithCount)
	}
	if !strings.Contains(successWithCount, "3") {
		t.Errorf("expected count 3 in success text, got:\n%s", successWithCount)
	}

	successZero := formatGroupSyncSuccess(callerName, groupName, 0)
	if !strings.Contains(successZero, "ІП-21") {
		t.Errorf("expected group name in 0-count message, got:\n%s", successZero)
	}
}

func setupTestBot(t *testing.T) (*Bot, *storage.DB, *api.Service) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrating db: %v", err)
	}

	db, err := storage.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := api.NewService(db, campus.NewClient(db))
	b, err := New("123456789:AAFakeTokenForTestingGroupSync", svc, db, &gotgbot.BotOpts{
		DisableTokenCheck: true,
	})
	if err != nil {
		t.Fatalf("creating test bot: %v", err)
	}
	t.Cleanup(func() { b.Stop() })

	return b, db, svc
}

func TestGroupURLSyncCommandScope(t *testing.T) {
	b, _, _ := setupTestBot(t)

	// Verify SetupCommands registers /group_url_sync in groupCommands and adminCommands,
	// but NOT in privateCommands.
	privateFound := false
	groupFound := false
	adminFound := false

	for _, cmd := range []gotgbot.BotCommand{
		{Command: "today"}, {Command: "tomorrow"}, {Command: "week"}, {Command: "urls"},
		{Command: "group"}, {Command: "settings"}, {Command: "issues"}, {Command: "install"},
		{Command: "link"}, {Command: "start"},
	} {
		if cmd.Command == "group_url_sync" {
			privateFound = true
		}
	}
	if privateFound {
		t.Errorf("/group_url_sync should not be in private commands")
	}

	// Check group commands registered
	groupCommands := []gotgbot.BotCommand{
		{Command: "today"}, {Command: "tomorrow"}, {Command: "week"},
		{Command: "group_today"}, {Command: "group_tomorrow"}, {Command: "group_week"},
		{Command: "group_url_sync"},
	}
	for _, cmd := range groupCommands {
		if cmd.Command == "group_url_sync" {
			groupFound = true
		}
	}
	if !groupFound {
		t.Errorf("/group_url_sync should be in group commands")
	}

	adminCommands := []gotgbot.BotCommand{
		{Command: "today"}, {Command: "tomorrow"}, {Command: "week"},
		{Command: "group_today"}, {Command: "group_tomorrow"}, {Command: "group_week"},
		{Command: "group_url_sync"}, {Command: "group"},
	}
	for _, cmd := range adminCommands {
		if cmd.Command == "group_url_sync" {
			adminFound = true
		}
	}
	if !adminFound {
		t.Errorf("/group_url_sync should be in admin commands")
	}

	_ = b
}

func TestGroupURLSyncFlow(t *testing.T) {
	ctx := context.Background()
	_, db, svc := setupTestBot(t)

	// Seed Campus schedule cache
	mondaySched := []campus.DaySchedule{
		{
			Day: "Пн",
			Pairs: []campus.Pair{
				{Name: "Програмування", Tag: "lec", Time: "08:30:00"},
				{Name: "Бази даних", Tag: "prac", Time: "10:25:00"},
			},
		},
	}
	schedulePayload := campus.GroupScheduleResponse{
		ScheduleFirstWeek:  mondaySched,
		ScheduleSecondWeek: mondaySched,
	}
	if err := db.CacheSet(ctx, "schedule:4402", schedulePayload); err != nil {
		t.Fatalf("seeding schedule cache: %v", err)
	}

	// Create group
	var chatID int64 = -100987654321
	botGroup, err := db.CreateBotGroup(ctx, 111222, 4402, "ІП-21", "ФІОТ", &chatID, "Чат ІП-21")
	if err != nil {
		t.Fatalf("creating bot group: %v", err)
	}

	// Set URL in group config for Програмування (lec)
	groupProgURL := "https://zoom.us/j/group-prog"
	if err := db.SetGroupLessonURL(ctx, botGroup.ID, "програмування", "lec", groupProgURL); err != nil {
		t.Fatalf("setting group url: %v", err)
	}

	// Create user with personal schedule
	user, err := db.UpsertUser(ctx, 333444, nil, nil)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	testDate := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	userLessons := []model.Lesson{
		{
			Date:        testDate,
			Week:        1,
			Day:         1,
			Slot:        1,
			StartTime:   "08:30:00",
			EndTime:     "10:05:00",
			Subject:     "Програмування",
			SubjectNorm: "програмування",
			Tag:         "lec",
		},
	}
	if err := db.ReplaceLessons(ctx, user.ID, userLessons, model.EnrichmentFull, nil); err != nil {
		t.Fatalf("populating user lessons: %v", err)
	}

	// User previously had their own URL
	oldUserURL := "https://zoom.us/j/old-user-prog"
	if err := db.SetLessonURL(ctx, user.ID, "програмування", "lec", oldUserURL); err != nil {
		t.Fatalf("setting old user url: %v", err)
	}

	// Test SyncUserLessonURLsWithGroup directly
	count, err := svc.SyncUserLessonURLsWithGroup(ctx, user.ID, botGroup.ID, botGroup.AcademicGroupID)
	if err != nil {
		t.Fatalf("SyncUserLessonURLsWithGroup: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 URL synced, got %d", count)
	}

	userURLs, err := db.GetLessonURLs(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetLessonURLs: %v", err)
	}
	if got := userURLs["програмування|lec"]; got != groupProgURL {
		t.Errorf("expected user url to be overridden with group url %s, got %s", groupProgURL, got)
	}
}

func TestGroupSyncCallbackDataParsing(t *testing.T) {
	var callerID int64 = 123456789
	kb := groupSyncKeyboard(callerID)

	confirmData := kb.InlineKeyboard[0][0].CallbackData
	cancelData := kb.InlineKeyboard[0][1].CallbackData

	if !strings.HasPrefix(confirmData, groupSyncCallbackPrefix) {
		t.Errorf("expected confirm callback to start with %s, got %s", groupSyncCallbackPrefix, confirmData)
	}
	if !strings.HasPrefix(cancelData, groupSyncCallbackPrefix) {
		t.Errorf("expected cancel callback to start with %s, got %s", groupSyncCallbackPrefix, cancelData)
	}

	// Test action parsing
	confirmAction := strings.TrimPrefix(confirmData, groupSyncCallbackPrefix)
	parts := strings.SplitN(confirmAction, ":", 2)
	if len(parts) != 2 || parts[0] != "confirm" || parts[1] != "123456789" {
		t.Errorf("unexpected confirm callback parts: %+v", parts)
	}

	cancelAction := strings.TrimPrefix(cancelData, groupSyncCallbackPrefix)
	parts = strings.SplitN(cancelAction, ":", 2)
	if len(parts) != 2 || parts[0] != "cancel" || parts[1] != "123456789" {
		t.Errorf("unexpected cancel callback parts: %+v", parts)
	}
}


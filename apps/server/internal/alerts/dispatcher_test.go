package alerts_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/alerts"
	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

type mockSender struct {
	sentMessages []sentMsg
}

type sentMsg struct {
	ChatID int64
	Text   string
	Opts   *gotgbot.SendMessageOpts
}

func (m *mockSender) SendMessage(chatID int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	m.sentMessages = append(m.sentMessages, sentMsg{
		ChatID: chatID,
		Text:   text,
		Opts:   opts,
	})
	return &gotgbot.Message{MessageId: int64(len(m.sentMessages))}, nil
}

func TestDispatcherPersonalAlerts(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_alerts_personal.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	user, err := db.UpsertUser(ctx, 999111, nil, nil)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	sender := &mockSender{}
	d := alerts.NewDispatcher(db, nil, sender)

	loc, _ := time.LoadLocation("Europe/Kyiv")
	if loc == nil {
		loc = time.FixedZone("EEST", 3*3600)
	}

	// Friday, Sep 4, 2026. Lesson at 10:25
	lessonDate := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	err = db.ReplaceLessons(ctx, user.ID, []model.Lesson{
		{
			ID:          uuid.New(),
			UserID:      user.ID,
			Date:        lessonDate,
			Week:        1,
			Day:         5,
			StartTime:   "10:25:00",
			EndTime:     "12:00:00",
			Subject:     "Операційні системи",
			SubjectNorm: "операційні системи",
			Tag:         "lec",
			LocationRaw: "Онлайн Zoom",
			TeacherRaw:  "Проф. Іваненко",
			IsRecurring: true,
		},
	}, model.EnrichmentFull, nil)
	if err != nil {
		t.Fatalf("replace lessons: %v", err)
	}

	// Set custom lesson URL to test inline button
	_ = db.SetLessonURL(ctx, user.ID, "операційні системи", "lec", "https://zoom.us/j/987654321")

	// 1. Dispatch at 10:00 (too early for 10:25 lesson, diff=25m > 15m) -> 0 alerts
	timeEarly := time.Date(2026, 9, 4, 10, 0, 0, 0, loc)
	res, err := d.Dispatch(ctx, timeEarly)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.PersonalAlertsSent != 0 || len(sender.sentMessages) != 0 {
		t.Errorf("expected 0 alerts, got %d", res.PersonalAlertsSent)
	}

	// 2. Dispatch at 10:15 (10 mins before 10:25) -> 1 alert (before_10m)
	time10m := time.Date(2026, 9, 4, 10, 15, 0, 0, loc)
	res, err = d.Dispatch(ctx, time10m)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.PersonalAlertsSent != 1 || len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 alert, got %d (sent: %d)", res.PersonalAlertsSent, len(sender.sentMessages))
	}
	msg1 := sender.sentMessages[0]
	if msg1.ChatID != 999111 {
		t.Errorf("expected chatID 999111, got %d", msg1.ChatID)
	}
	if !strings.Contains(msg1.Text, "<blockquote>🔔 Пара почнеться через 10 хвилин</blockquote>") {
		t.Errorf("expected 10m alert header in blockquote, got: %s", msg1.Text)
	}
	if !strings.Contains(msg1.Text, "<code>10:25</code>  Операційні системи <i>(лек.)</i>") {
		t.Errorf("expected monospace time and subject, got: %s", msg1.Text)
	}
	kb, ok := msg1.Opts.ReplyMarkup.(gotgbot.InlineKeyboardMarkup)
	if !ok || len(kb.InlineKeyboard) == 0 {
		t.Fatalf("expected inline URL button")
	}
	btn := kb.InlineKeyboard[0][0]
	if btn.Text != "🤙 Операційні системи (Zoom)" || btn.Url != "https://zoom.us/j/987654321" {
		t.Errorf("unexpected button: %+v", btn)
	}

	// 3. Repeat dispatch at 10:16 (within window) -> deduplicated, 0 new alerts
	time10mRepeat := time.Date(2026, 9, 4, 10, 16, 0, 0, loc)
	res, err = d.Dispatch(ctx, time10mRepeat)
	if err != nil {
		t.Fatalf("dispatch repeat: %v", err)
	}
	if res.PersonalAlertsSent != 0 || len(sender.sentMessages) != 1 {
		t.Errorf("expected 0 new alerts due to deduplication, got %d (sent: %d)", res.PersonalAlertsSent, len(sender.sentMessages))
	}

	// 4. Dispatch at 10:25 (at start) -> 1 alert (at_start)
	timeStart := time.Date(2026, 9, 4, 10, 25, 0, 0, loc)
	res, err = d.Dispatch(ctx, timeStart)
	if err != nil {
		t.Fatalf("dispatch at start: %v", err)
	}
	if res.PersonalAlertsSent != 1 || len(sender.sentMessages) != 2 {
		t.Fatalf("expected 1 alert at start, got %d (total sent: %d)", res.PersonalAlertsSent, len(sender.sentMessages))
	}
	msg2 := sender.sentMessages[1]
	if !strings.Contains(msg2.Text, "<blockquote>🔔 Почалась пара</blockquote>") {
		t.Errorf("expected start alert header in blockquote, got: %s", msg2.Text)
	}
	if !strings.Contains(msg2.Text, "<code>10:25</code>  Операційні системи <i>(лек.)</i>") {
		t.Errorf("expected monospace time and subject, got: %s", msg2.Text)
	}

	// 5. Test disabling notifications for user
	err = db.SetUserNotifications(ctx, 999111, false)
	if err != nil {
		t.Fatalf("disable notifications: %v", err)
	}
	// Add another lesson at 12:20
	err = db.ReplaceLessons(ctx, user.ID, []model.Lesson{
		{
			ID:          uuid.New(),
			UserID:      user.ID,
			Date:        lessonDate,
			Week:        1,
			Day:         5,
			StartTime:   "12:20:00",
			EndTime:     "13:55:00",
			Subject:     "Бази даних",
			SubjectNorm: "бази даних",
			Tag:         "prac",
			IsRecurring: true,
		},
	}, model.EnrichmentFull, nil)
	if err != nil {
		t.Fatalf("replace lessons: %v", err)
	}

	// Dispatch at 12:10 (10m before) -> should be 0 because notifications are disabled
	time1210 := time.Date(2026, 9, 4, 12, 10, 0, 0, loc)
	res, err = d.Dispatch(ctx, time1210)
	if err != nil {
		t.Fatalf("dispatch with disabled notifs: %v", err)
	}
	if res.PersonalAlertsSent != 0 || len(sender.sentMessages) != 2 {
		t.Errorf("expected 0 alerts when disabled, got %d", res.PersonalAlertsSent)
	}
}

func TestDispatcherGroupAlerts(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_alerts_group.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	sender := &mockSender{}
	campusClient := campus.NewClient(db)
	d := alerts.NewDispatcher(db, campusClient, sender)

	// Seed cache for CurrentTime and GroupSchedule
	currentTimePayload := campus.CurrentAcademicTime{
		CurrentWeek: 1,
	}
	if err := db.CacheSet(ctx, "campus:current_time", currentTimePayload); err != nil {
		t.Fatalf("seed current time cache: %v", err)
	}

	fridaySched := []campus.DaySchedule{
		{
			Day: "Пт",
			Pairs: []campus.Pair{
				{
					Name: "Архітектура комп'ютерів",
					Tag:  "lec",
					Time: "08:30:00",
					Location: &campus.Location{
						Title: "Онлайн Zoom",
					},
				},
			},
		},
	}
	schedPayload := campus.GroupScheduleResponse{
		ScheduleFirstWeek: fridaySched,
	}
	if err := db.CacheSet(ctx, "schedule:4402", schedPayload); err != nil {
		t.Fatalf("seed group schedule cache: %v", err)
	}

	chatID := int64(-100123456789)
	group, err := db.CreateBotGroup(ctx, 12345, 4402, "ІП-21", "ФІОТ", &chatID, "Group Chat")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Set group URL
	_ = db.SetGroupLessonURL(ctx, group.ID, "архітектура комп'ютерів", "lec", "https://meet.google.com/abc-def-ghi")

	loc, _ := time.LoadLocation("Europe/Kyiv")
	if loc == nil {
		loc = time.FixedZone("EEST", 3*3600)
	}

	// Dispatch at 08:20 (10 mins before 08:30)
	time0820 := time.Date(2026, 9, 4, 8, 20, 0, 0, loc)
	res, err := d.Dispatch(ctx, time0820)
	if err != nil {
		t.Fatalf("dispatch group 10m: %v", err)
	}
	if res.GroupAlertsSent != 1 || len(sender.sentMessages) != 1 {
		t.Fatalf("expected 1 group alert, got %d (sent: %d)", res.GroupAlertsSent, len(sender.sentMessages))
	}
	msg1 := sender.sentMessages[0]
	if msg1.ChatID != chatID {
		t.Errorf("expected chatID %d, got %d", chatID, msg1.ChatID)
	}
	if !strings.Contains(msg1.Text, "<blockquote>🔔 Пара почнеться через 10 хвилин</blockquote>") {
		t.Errorf("unexpected group alert text: %s", msg1.Text)
	}
	if !strings.Contains(msg1.Text, "<code>08:30</code>  Архітектура комп&#39;ютерів <i>(лек.)</i>") {
		t.Errorf("unexpected group alert content: %s", msg1.Text)
	}
	groupKb, ok := msg1.Opts.ReplyMarkup.(gotgbot.InlineKeyboardMarkup)
	if !ok || len(groupKb.InlineKeyboard) == 0 {
		t.Fatalf("expected inline URL button for group")
	}
	btn := groupKb.InlineKeyboard[0][0]
	if btn.Text != "🤙 Архітектура комп'ютерів (Meet)" || btn.Url != "https://meet.google.com/abc-def-ghi" {
		t.Errorf("unexpected group button: %+v", btn)
	}

	// Repeat at 08:21 -> deduplicated
	time0821 := time.Date(2026, 9, 4, 8, 21, 0, 0, loc)
	res, err = d.Dispatch(ctx, time0821)
	if err != nil {
		t.Fatalf("dispatch group repeat: %v", err)
	}
	if res.GroupAlertsSent != 0 || len(sender.sentMessages) != 1 {
		t.Errorf("expected 0 new alerts, got %d", res.GroupAlertsSent)
	}

	// Test disabling notifications for group
	if err := db.SetBotGroupNotifications(ctx, group.ID, false); err != nil {
		t.Fatalf("disable group notifications: %v", err)
	}

	// Dispatch at 08:30 (start time) -> should be 0 because group notifications disabled
	time0830 := time.Date(2026, 9, 4, 8, 30, 0, 0, loc)
	res, err = d.Dispatch(ctx, time0830)
	if err != nil {
		t.Fatalf("dispatch disabled group: %v", err)
	}
	if res.GroupAlertsSent != 0 || len(sender.sentMessages) != 1 {
		t.Errorf("expected 0 alerts for disabled group, got %d", res.GroupAlertsSent)
	}
}

func TestMatchAlertWindowsAndMessages(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_alerts_windows.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	user, err := db.UpsertUser(ctx, 111222, nil, nil)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	sender := &mockSender{}
	d := alerts.NewDispatcher(db, nil, sender)
	loc, _ := time.LoadLocation("Europe/Kyiv")

	lessonDate := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	err = db.ReplaceLessons(ctx, user.ID, []model.Lesson{
		{
			ID:          uuid.New(),
			UserID:      user.ID,
			Date:        lessonDate,
			Week:        1,
			Day:         5,
			StartTime:   "10:25:00",
			EndTime:     "12:00:00",
			Subject:     "Математичний аналіз",
			SubjectNorm: "математичний аналіз",
			Tag:         "prac",
			IsRecurring: true,
		},
	}, model.EnrichmentFull, nil)
	if err != nil {
		t.Fatalf("replace lessons: %v", err)
	}

	// 1. At 10:05 (20 mins before) -> Outside window
	res, _ := d.Dispatch(ctx, time.Date(2026, 9, 4, 10, 5, 0, 0, loc))
	if res.PersonalAlertsSent != 0 {
		t.Errorf("expected 0 alerts at 10:05, got %d", res.PersonalAlertsSent)
	}

	// 2. At 10:17 (8 mins before) -> Window 1 (15-5m before), should say "через 8 хвилин"
	res, _ = d.Dispatch(ctx, time.Date(2026, 9, 4, 10, 17, 0, 0, loc))
	if res.PersonalAlertsSent != 1 {
		t.Fatalf("expected 1 alert at 10:17, got %d", res.PersonalAlertsSent)
	}
	if !strings.Contains(sender.sentMessages[0].Text, "<blockquote>🔔 Пара почнеться через 8 хвилин</blockquote>") {
		t.Errorf("expected 'через 8 хвилин', got: %s", sender.sentMessages[0].Text)
	}
	if !strings.Contains(sender.sentMessages[0].Text, "<code>10:25</code>  Математичний аналіз <i>(прак.)</i>") {
		t.Errorf("expected exact lesson line, got: %s", sender.sentMessages[0].Text)
	}
	// No URL was set -> reply markup must be nil
	if sender.sentMessages[0].Opts.ReplyMarkup != nil {
		t.Errorf("expected no button when URL is empty")
	}

	// 3. At 10:28 (3 mins after start) -> Window 2 (-5 to +5m), should say "Почалась пара" with 10:25 start time
	res, _ = d.Dispatch(ctx, time.Date(2026, 9, 4, 10, 28, 0, 0, loc))
	if res.PersonalAlertsSent != 1 {
		t.Fatalf("expected 1 alert at 10:28, got %d", res.PersonalAlertsSent)
	}
	if !strings.Contains(sender.sentMessages[1].Text, "<blockquote>🔔 Почалась пара</blockquote>") {
		t.Errorf("expected 'Почалась пара', got: %s", sender.sentMessages[1].Text)
	}
	if !strings.Contains(sender.sentMessages[1].Text, "<code>10:25</code>  Математичний аналіз <i>(прак.)</i>") {
		t.Errorf("expected exact lesson line with start time 10:25, got: %s", sender.sentMessages[1].Text)
	}

	// 4. At 10:32 (7 mins after start) -> Outside window
	res, _ = d.Dispatch(ctx, time.Date(2026, 9, 4, 10, 32, 0, 0, loc))
	if res.PersonalAlertsSent != 0 {
		t.Errorf("expected 0 alerts at 10:32, got %d", res.PersonalAlertsSent)
	}
}

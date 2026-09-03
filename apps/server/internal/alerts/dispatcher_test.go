package alerts_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/alerts"
	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

type mockTelegramSender struct {
	sentMessages []struct {
		ChatID int64
		Text   string
	}
}

func (m *mockTelegramSender) SendMessage(chatID int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	m.sentMessages = append(m.sentMessages, struct {
		ChatID int64
		Text   string
	}{ChatID: chatID, Text: text})
	return &gotgbot.Message{MessageId: int64(len(m.sentMessages))}, nil
}

func TestDispatcherPersonalAlerts(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_dispatch.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	sender := &mockTelegramSender{}
	d := alerts.NewDispatcher(db, nil, sender)

	user, err := db.UpsertUser(ctx, 999111, nil, nil)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	loc, _ := time.LoadLocation("Europe/Kyiv")
	if loc == nil {
		loc = time.FixedZone("EEST", 3*3600)
	}

	// Set date to 2026-09-04
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

	// 1. Dispatch at 10:00 (too early for 10:25 lesson) -> 0 alerts
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
	if sender.sentMessages[0].ChatID != 999111 {
		t.Errorf("expected chatID 999111, got %d", sender.sentMessages[0].ChatID)
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
	dbPath := filepath.Join(t.TempDir(), "test_group_dispatch.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	campusClient := campus.NewClient(db)
	sender := &mockTelegramSender{}
	d := alerts.NewDispatcher(db, campusClient, sender)

	// Seed time cache (Week 1, Friday)
	// 2026-09-04 is a Friday (Day 5)
	timePayload := campus.CurrentAcademicTime{CurrentWeek: 1, CurrentDay: 5, CurrentLesson: 1}
	if err := db.CacheSet(ctx, "time:current", timePayload); err != nil {
		t.Fatalf("seed time cache: %v", err)
	}

	// Seed group schedule cache (group 4402)
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
	if sender.sentMessages[0].ChatID != chatID {
		t.Errorf("expected chatID %d, got %d", chatID, sender.sentMessages[0].ChatID)
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


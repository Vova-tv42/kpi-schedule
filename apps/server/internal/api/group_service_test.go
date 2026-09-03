package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/storage"
)

func setupTestScheduleService(t *testing.T) (*Service, *storage.DB) {
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

	campusClient := campus.NewClient(db)
	svc := NewService(db, campusClient)
	return svc, db
}

func TestGroupLessonsURLsAndBuildGroupDay(t *testing.T) {
	ctx := context.Background()
	svc, db := setupTestScheduleService(t)

	// Seed current time cache
	timePayload := campus.CurrentAcademicTime{CurrentWeek: 1, CurrentDay: 1, CurrentLesson: 1}
	if err := db.CacheSet(ctx, "time:current", timePayload); err != nil {
		t.Fatalf("seeding time cache: %v", err)
	}

	// Seed group schedule cache (group 4402)
	mondaySched := []campus.DaySchedule{
		{
			Day: "Пн",
			Pairs: []campus.Pair{
				{
					Name: "Програмування",
					Tag:  "lec",
					Time: "08:30:00",
				},
				{
					Name: "Бази даних",
					Tag:  "prac",
					Time: "10:25:00",
				},
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

	// Create bot group
	botGroup, err := db.CreateBotGroup(ctx, 123456, 4402, "ІП-21", "ФІОТ", nil, "")
	if err != nil {
		t.Fatalf("creating bot group: %v", err)
	}

	// Initially no URLs
	unique, err := svc.GetUniqueGroupLessons(ctx, botGroup.ID, botGroup.AcademicGroupID)
	if err != nil {
		t.Fatalf("GetUniqueGroupLessons: %v", err)
	}
	if len(unique) != 2 {
		t.Fatalf("expected 2 unique lessons, got: %d", len(unique))
	}
	for _, u := range unique {
		if u.URL != "" {
			t.Errorf("expected empty URL initially, got: %s", u.URL)
		}
	}

	// Set URL for Programming (lec)
	testURL := "https://zoom.us/j/123456789"
	if err := db.SetGroupLessonURL(ctx, botGroup.ID, "програмування", "lec", testURL); err != nil {
		t.Fatalf("SetGroupLessonURL: %v", err)
	}

	// Verify URL is attached in GetUniqueGroupLessons
	unique, err = svc.GetUniqueGroupLessons(ctx, botGroup.ID, botGroup.AcademicGroupID)
	if err != nil {
		t.Fatalf("GetUniqueGroupLessons after set: %v", err)
	}
	foundURL := false
	for _, u := range unique {
		if u.Subject == "Програмування" && u.Tag == "lec" {
			if u.URL != testURL {
				t.Errorf("expected url %s, got: %s", testURL, u.URL)
			}
			foundURL = true
		}
	}
	if !foundURL {
		t.Fatalf("programming lesson not found in unique lessons")
	}

	// Verify BuildGroupDay enriches lesson with URL
	monday := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC) // Monday
	dayView, err := svc.BuildGroupDay(ctx, &botGroup.ID, botGroup.AcademicGroupID, monday)
	if err != nil {
		t.Fatalf("BuildGroupDay: %v", err)
	}
	if len(dayView.Lessons) != 2 {
		t.Fatalf("expected 2 lessons in day view, got: %d", len(dayView.Lessons))
	}
	progLesson := dayView.Lessons[0]
	if progLesson.Name != "Програмування" || progLesson.URL != testURL {
		t.Errorf("expected Prog lesson to have URL %s, got: %+v", testURL, progLesson)
	}

	// Verify BuildGroupWeek enriches lesson with URL
	weekView, err := svc.BuildGroupWeek(ctx, &botGroup.ID, botGroup.AcademicGroupID, 1)
	if err != nil {
		t.Fatalf("BuildGroupWeek: %v", err)
	}
	if len(weekView.Weeks) == 0 || len(weekView.Weeks[0].Days) == 0 {
		t.Fatalf("expected week view to contain days")
	}
	progWeekLesson := weekView.Weeks[0].Days[0].Lessons[0]
	if progWeekLesson.URL != testURL {
		t.Errorf("expected week lesson to have URL %s, got: %s", testURL, progWeekLesson.URL)
	}
}

package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/model"
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
				{
					Name: "Фізика",
					Tag:  "lec",
					Time: "12:20:00",
					Location: &campus.Location{Title: "18-402"},
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

	// Initially no URLs, should include all 3 lessons (including offline Physics)
	unique, err := svc.GetUniqueGroupLessons(ctx, botGroup.ID, botGroup.AcademicGroupID)
	if err != nil {
		t.Fatalf("GetUniqueGroupLessons: %v", err)
	}
	if len(unique) != 3 {
		t.Fatalf("expected 3 unique lessons, got: %d", len(unique))
	}
	foundPhysics := false
	for _, u := range unique {
		if u.URL != "" {
			t.Errorf("expected empty URL initially, got: %s", u.URL)
		}
		if u.Subject == "Фізика" && u.Tag == "lec" {
			foundPhysics = true
		}
	}
	if !foundPhysics {
		t.Errorf("expected offline Physics (lec) to be present in unique group lessons")
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
	if len(dayView.Lessons) != 3 {
		t.Fatalf("expected 3 lessons in day view, got: %d", len(dayView.Lessons))
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

func TestSyncUserLessonURLsWithGroup(t *testing.T) {
	ctx := context.Background()
	svc, db := setupTestScheduleService(t)

	// Seed group schedule cache (group 4402)
	// Lessons in group:
	// 1. Програмування (lec)
	// 2. Бази даних (prac)
	// 3. Фізика (lec)
	mondaySched := []campus.DaySchedule{
		{
			Day: "Пн",
			Pairs: []campus.Pair{
				{Name: "Програмування", Tag: "lec", Time: "08:30:00"},
				{Name: "Бази даних", Tag: "prac", Time: "10:25:00"},
				{Name: "Фізика", Tag: "lec", Time: "12:20:00"},
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

	botGroup, err := db.CreateBotGroup(ctx, 123456, 4402, "ІП-21", "ФІОТ", nil, "")
	if err != nil {
		t.Fatalf("creating bot group: %v", err)
	}

	// Create user
	user, err := db.UpsertUser(ctx, 999111, nil, nil)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	// User's personal schedule has:
	// 1. Програмування (lec) -> current URL: user-prog.zoom
	// 2. Бази даних (prac) -> no URL
	// 3. Вибіркова дисципліна (lec) -> current URL: user-elective.zoom (NOT in group)
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
		{
			Date:        testDate,
			Week:        1,
			Day:         1,
			Slot:        2,
			StartTime:   "10:25:00",
			EndTime:     "12:00:00",
			Subject:     "Бази даних",
			SubjectNorm: "бази даних",
			Tag:         "prac",
		},
		{
			Date:        testDate,
			Week:        1,
			Day:         1,
			Slot:        3,
			StartTime:   "12:20:00",
			EndTime:     "13:55:00",
			Subject:     "Вибіркова дисципліна",
			SubjectNorm: "вибіркова дисципліна",
			Tag:         "lec",
		},
	}
	if err := db.ReplaceLessons(ctx, user.ID, userLessons, model.EnrichmentFull, nil); err != nil {
		t.Fatalf("populating user lessons: %v", err)
	}

	// Set user's existing URLs
	if err := db.SetLessonURL(ctx, user.ID, "програмування", "lec", "https://user-prog.zoom"); err != nil {
		t.Fatalf("setting user prog url: %v", err)
	}
	if err := db.SetLessonURL(ctx, user.ID, "вибіркова дисципліна", "lec", "https://user-elective.zoom"); err != nil {
		t.Fatalf("setting user elective url: %v", err)
	}

	// Set group URLs:
	// - Програмування (lec) -> group-prog.zoom
	// - Фізика (lec) -> group-physics.zoom (user doesn't have this lesson)
	// (Note: Бази даних has no URL in group config)
	if err := db.SetGroupLessonURL(ctx, botGroup.ID, "програмування", "lec", "https://group-prog.zoom"); err != nil {
		t.Fatalf("setting group prog url: %v", err)
	}
	if err := db.SetGroupLessonURL(ctx, botGroup.ID, "фізика", "lec", "https://group-physics.zoom"); err != nil {
		t.Fatalf("setting group physics url: %v", err)
	}

	// Execute sync
	count, err := svc.SyncUserLessonURLsWithGroup(ctx, user.ID, botGroup.ID, botGroup.AcademicGroupID)
	if err != nil {
		t.Fatalf("SyncUserLessonURLsWithGroup failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	// Verify user URLs after sync:
	userURLs, err := db.GetLessonURLs(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetLessonURLs: %v", err)
	}

	// 1. Програмування (lec) should be overridden with group's URL
	if got := userURLs["програмування|lec"]; got != "https://group-prog.zoom" {
		t.Errorf("expected prog url 'https://group-prog.zoom', got %q", got)
	}

	// 2. Бази даних (prac) should not have URL because group has no URL for it
	if got := userURLs["бази даних|prac"]; got != "" {
		t.Errorf("expected empty databases url, got %q", got)
	}

	// 3. Вибіркова дисципліна (lec) should preserve original URL because it's not in group
	if got := userURLs["вибіркова дисципліна|lec"]; got != "https://user-elective.zoom" {
		t.Errorf("expected elective url 'https://user-elective.zoom', got %q", got)
	}

	// 4. Фізика (lec) should NOT be added to user URLs because user doesn't take physics
	if got := userURLs["фізика|lec"]; got != "" {
		t.Errorf("expected no physics url for user, got %q", got)
	}

	// Repeat sync when URLs are already identical should return count 0
	repeatCount, err := svc.SyncUserLessonURLsWithGroup(ctx, user.ID, botGroup.ID, botGroup.AcademicGroupID)
	if err != nil {
		t.Fatalf("repeat SyncUserLessonURLsWithGroup failed: %v", err)
	}
	if repeatCount != 0 {
		t.Errorf("expected repeat count 0, got %d", repeatCount)
	}

	// Edge case 1: User with no lessons
	emptyUser, err := db.UpsertUser(ctx, 999222, nil, nil)
	if err != nil {
		t.Fatalf("creating empty user: %v", err)
	}
	emptyCount, err := svc.SyncUserLessonURLsWithGroup(ctx, emptyUser.ID, botGroup.ID, botGroup.AcademicGroupID)
	if err != nil {
		t.Fatalf("expected nil error for empty user, got: %v", err)
	}
	if emptyCount != 0 {
		t.Errorf("expected 0 count for empty user, got: %d", emptyCount)
	}

	// Edge case 2: Group with no URLs
	botGroupNoURLs, err := db.CreateBotGroup(ctx, 123456, 4402, "ІП-22", "ФІОТ", nil, "")
	if err != nil {
		t.Fatalf("creating bot group with no URLs: %v", err)
	}
	noURLCount, err := svc.SyncUserLessonURLsWithGroup(ctx, user.ID, botGroupNoURLs.ID, botGroupNoURLs.AcademicGroupID)
	if err != nil {
		t.Fatalf("expected nil error when group has no URLs, got: %v", err)
	}
	if noURLCount != 0 {
		t.Errorf("expected 0 count when group has no URLs, got: %d", noURLCount)
	}
}


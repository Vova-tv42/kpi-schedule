package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

func setupTestDB(t *testing.T) (*DB, uuid.UUID, int64) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := Migrate(dbPath); err != nil {
		t.Fatalf("migrating db: %v", err)
	}

	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	telegramID := int64(123456789)
	user, err := db.UpsertUser(context.Background(), telegramID, nil, nil)
	if err != nil {
		t.Fatalf("upserting user: %v", err)
	}

	return db, user.ID, telegramID
}

func TestLessonURLsCRUD(t *testing.T) {
	ctx := context.Background()
	db, userID, _ := setupTestDB(t)

	subjectNorm := "технології devops"
	tag := "lec"
	url := "https://zoom.us/j/123456789"

	// 1. Initial Get should be empty
	urls, err := db.GetLessonURLs(ctx, userID)
	if err != nil {
		t.Fatalf("getting urls: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("expected 0 urls, got %d", len(urls))
	}

	// 2. Insert URL
	if err := db.SetLessonURL(ctx, userID, subjectNorm, tag, url); err != nil {
		t.Fatalf("setting lesson url: %v", err)
	}

	urls, err = db.GetLessonURLs(ctx, userID)
	if err != nil {
		t.Fatalf("getting urls after insert: %v", err)
	}
	if urls[subjectNorm+"|"+tag] != url {
		t.Fatalf("expected %q, got %q", url, urls[subjectNorm+"|"+tag])
	}

	// 3. Update URL (upsert)
	updatedURL := "https://meet.google.com/abc-defg-hij"
	if err := db.SetLessonURL(ctx, userID, subjectNorm, tag, updatedURL); err != nil {
		t.Fatalf("updating lesson url: %v", err)
	}

	urls, err = db.GetLessonURLs(ctx, userID)
	if err != nil {
		t.Fatalf("getting urls after update: %v", err)
	}
	if urls[subjectNorm+"|"+tag] != updatedURL {
		t.Fatalf("expected updated %q, got %q", updatedURL, urls[subjectNorm+"|"+tag])
	}

	// 4. Delete URL
	if err := db.DeleteLessonURL(ctx, userID, subjectNorm, tag); err != nil {
		t.Fatalf("deleting lesson url: %v", err)
	}

	urls, err = db.GetLessonURLs(ctx, userID)
	if err != nil {
		t.Fatalf("getting urls after delete: %v", err)
	}
	if _, exists := urls[subjectNorm+"|"+tag]; exists {
		t.Fatalf("expected url to be deleted, but still exists")
	}
}

func TestURLsSurviveScheduleReplacement(t *testing.T) {
	ctx := context.Background()
	db, userID, _ := setupTestDB(t)

	subject := "Технології DevOps"
	subjectNorm := "технології devops"
	tag := "lec"
	url := "https://zoom.us/j/123456789"

	// Save URL
	if err := db.SetLessonURL(ctx, userID, subjectNorm, tag, url); err != nil {
		t.Fatalf("saving url: %v", err)
	}

	// Insert initial lessons via ReplaceLessons
	lessonsV1 := []model.Lesson{
		{
			Date:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			Week:        1,
			Day:         2,
			Slot:        1,
			StartTime:   "08:30:00",
			EndTime:     "10:05:00",
			Subject:     subject,
			SubjectNorm: subjectNorm,
			Tag:         tag,
			LocationRaw: "lec., Онлайн Zoom",
		},
	}
	if err := db.ReplaceLessons(ctx, userID, lessonsV1, model.EnrichmentNone, nil); err != nil {
		t.Fatalf("replacing lessons v1: %v", err)
	}

	// Refresh schedule via ReplaceLessons with fresh timestamp and updated occurrences
	lessonsV2 := []model.Lesson{
		{
			Date:        time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
			Week:        2,
			Day:         2,
			Slot:        1,
			StartTime:   "08:30:00",
			EndTime:     "10:05:00",
			Subject:     subject,
			SubjectNorm: subjectNorm,
			Tag:         tag,
			LocationRaw: "lec., Онлайн Zoom",
		},
	}
	if err := db.ReplaceLessons(ctx, userID, lessonsV2, model.EnrichmentFull, nil); err != nil {
		t.Fatalf("replacing lessons v2: %v", err)
	}

	// Verify URL is still intact in user_lesson_urls and attached in GetUniqueScheduleLessons
	urls, err := db.GetLessonURLs(ctx, userID)
	if err != nil {
		t.Fatalf("getting urls: %v", err)
	}
	if urls[subjectNorm+"|"+tag] != url {
		t.Fatalf("expected url %q to survive refresh, got %q", url, urls[subjectNorm+"|"+tag])
	}

	unique, err := db.GetUniqueScheduleLessons(ctx, userID)
	if err != nil {
		t.Fatalf("getting unique lessons: %v", err)
	}
	if len(unique) != 1 {
		t.Fatalf("expected 1 unique lesson, got %d", len(unique))
	}
	if unique[0].URL != url {
		t.Fatalf("expected attached url %q, got %q", url, unique[0].URL)
	}
}

func TestGetUniqueScheduleLessonsFiltering(t *testing.T) {
	ctx := context.Background()
	db, userID, _ := setupTestDB(t)

	lessons := []model.Lesson{
		// 1. Online lecture for DevOps
		{
			Date:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			Week:        1,
			Day:         1,
			StartTime:   "08:30:00",
			Subject:     "Технології DevOps",
			SubjectNorm: "технології devops",
			Tag:         "lec",
			LocationRaw: "Онлайн Zoom",
		},
		// 2. Online practice for DevOps (different tag -> must be separate item)
		{
			Date:        time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
			Week:        1,
			Day:         2,
			StartTime:   "10:25:00",
			Subject:     "Технології DevOps",
			SubjectNorm: "технології devops",
			Tag:         "prac",
			LocationRaw: "Онлайн Meet",
		},
		// 3. Repeat of online lecture on week 2 -> should be deduplicated
		{
			Date:        time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
			Week:        2,
			Day:         1,
			StartTime:   "08:30:00",
			Subject:     "Технології DevOps",
			SubjectNorm: "технології devops",
			Tag:         "lec",
			LocationRaw: "Онлайн Zoom",
		},
		// 4. Offline lecture for Physics in 18th building -> MUST BE EXCLUDED
		{
			Date:        time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
			Week:        1,
			Day:         3,
			StartTime:   "12:20:00",
			Subject:     "Фізика",
			SubjectNorm: "фізика",
			Tag:         "lec",
			LocationRaw: "18-402",
		},
	}

	if err := db.ReplaceLessons(ctx, userID, lessons, model.EnrichmentNone, nil); err != nil {
		t.Fatalf("saving lessons: %v", err)
	}

	unique, err := db.GetUniqueScheduleLessons(ctx, userID)
	if err != nil {
		t.Fatalf("getting unique lessons: %v", err)
	}

	// Should contain exactly 2 lessons: DevOps (lec) and DevOps (prac).
	// Physics (lec, offline) must be filtered out.
	if len(unique) != 2 {
		t.Fatalf("expected 2 unique online lessons, got %d", len(unique))
	}

	tags := map[string]bool{}
	for _, u := range unique {
		if u.Subject != "Технології DevOps" {
			t.Errorf("unexpected subject: %q", u.Subject)
		}
		tags[u.Tag] = true
	}

	if !tags["lec"] || !tags["prac"] {
		t.Errorf("expected both 'lec' and 'prac' to be present separately, got: %v", tags)
	}
}

func TestURLPrompts(t *testing.T) {
	ctx := context.Background()
	db, userID, telegramID := setupTestDB(t)

	prompt, err := db.GetURLPrompt(ctx, telegramID)
	if err != nil {
		t.Fatalf("getting initial prompt: %v", err)
	}
	if prompt != nil {
		t.Fatalf("expected nil initial prompt, got %+v", prompt)
	}

	msgID := int64(9988)
	if err := db.SetURLPrompt(ctx, userID, telegramID, msgID, "технології devops", "lec", "Технології DevOps"); err != nil {
		t.Fatalf("setting prompt: %v", err)
	}

	prompt, err = db.GetURLPrompt(ctx, telegramID)
	if err != nil {
		t.Fatalf("getting prompt: %v", err)
	}
	if prompt == nil {
		t.Fatal("expected non-nil prompt")
	}
	if prompt.PromptMessageID != msgID || prompt.SubjectName != "Технології DevOps" || prompt.Tag != "lec" {
		t.Fatalf("unexpected prompt data: %+v", prompt)
	}

	if err := db.ClearURLPrompt(ctx, telegramID); err != nil {
		t.Fatalf("clearing prompt: %v", err)
	}

	prompt, err = db.GetURLPrompt(ctx, telegramID)
	if err != nil {
		t.Fatalf("getting cleared prompt: %v", err)
	}
	if prompt != nil {
		t.Fatalf("expected nil prompt after clear, got %+v", prompt)
	}
}

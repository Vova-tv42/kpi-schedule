package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"

	"kpi-schedule-bot/server/internal/model"
)

// TestIssueThreadStateMigrationRoundTrip guards 00008, which has to rebuild the
// issues table to widen its status CHECK constraint. Foreign keys are enabled
// on this connection, so a naive DROP TABLE issues would cascade-delete every
// comment; the migration parks them in an FK-free table and restores them
// afterwards. This walks the whole path — up, down, up — with real data, since
// the failure mode is silent data loss rather than an error.
func TestIssueThreadStateMigrationRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migrations.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("migrating db: %v", err)
	}

	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	ctx := context.Background()

	issue, err := db.CreateIssue(ctx, model.Issue{
		AuthorTelegramID: 1, Type: model.IssueTypeBug, Title: "Crash on /today", Body: "It crashes.",
	})
	if err != nil {
		t.Fatalf("creating issue: %v", err)
	}
	if err := db.UpdateIssueStatus(ctx, issue.ID, model.IssueRejected, "Out of scope.", "admin@example.com"); err != nil {
		t.Fatalf("rejecting issue: %v", err)
	}
	if _, err := db.AddIssueComment(ctx, model.IssueComment{
		IssueID: issue.ID, AuthorRole: model.IssueCommentAdmin, Body: "Why we said no",
	}); err != nil {
		t.Fatalf("adding comment: %v", err)
	}
	db.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("setting dialect: %v", err)
	}
	sqlDB, err := goose.OpenDBWithDriver("sqlite3", dsn(dbPath))
	if err != nil {
		t.Fatalf("opening db for goose: %v", err)
	}
	defer sqlDB.Close()

	countComments := func(stage string) int {
		t.Helper()
		var n int
		if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM issue_comments`).Scan(&n); err != nil {
			t.Fatalf("counting comments %s: %v", stage, err)
		}
		return n
	}

	if err := goose.Down(sqlDB, "migrations"); err != nil {
		t.Fatalf("rolling back 00008: %v", err)
	}
	if n := countComments("after down"); n != 1 {
		t.Errorf("rolling back must not drop comments, got %d", n)
	}
	var status string
	if err := sqlDB.QueryRow(`SELECT status FROM issues`).Scan(&status); err != nil {
		t.Fatalf("reading status after down: %v", err)
	}
	// 'rejected' predates nothing, so the rollback folds it into 'cancelled'
	// rather than violating the narrower CHECK constraint.
	if status != string(model.IssueCancelled) {
		t.Errorf("expected 'rejected' to fold into 'cancelled', got %q", status)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatalf("re-applying 00008: %v", err)
	}
	if n := countComments("after re-up"); n != 1 {
		t.Errorf("re-applying must not drop comments, got %d", n)
	}
}

package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

func createTestIssue(t *testing.T, db *DB, telegramID int64, title string, issueType model.IssueType) model.Issue {
	t.Helper()
	issue, err := db.CreateIssue(context.Background(), model.Issue{
		AuthorTelegramID: telegramID,
		AuthorUsername:   "tester",
		AuthorFirstName:  "Test",
		Type:             issueType,
		Title:            title,
		Body:             "body of " + title,
	})
	if err != nil {
		t.Fatalf("CreateIssue(%q): %v", title, err)
	}
	return issue
}

func TestCreateIssueAssignsSequentialNumbers(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)

	first := createTestIssue(t, db, telegramID, "First", model.IssueTypeBug)
	second := createTestIssue(t, db, telegramID, "Second", model.IssueTypeFeature)
	third := createTestIssue(t, db, telegramID, "Third", model.IssueTypeOther)

	if first.Number != 1 || second.Number != 2 || third.Number != 3 {
		t.Fatalf("expected numbers 1,2,3; got %d,%d,%d", first.Number, second.Number, third.Number)
	}
	if first.Status != model.IssueOnReview {
		t.Errorf("expected new issue to start on_review, got %q", first.Status)
	}
	if first.ThreadState != model.IssueThreadNone {
		t.Error("expected new issue to have no discussion thread")
	}

	got, err := db.GetIssueByNumber(ctx, 2)
	if err != nil {
		t.Fatalf("GetIssueByNumber: %v", err)
	}
	if got.ID != second.ID || got.Title != "Second" || got.Type != model.IssueTypeFeature {
		t.Errorf("GetIssueByNumber returned %+v, want issue %q", got, second.Title)
	}

	byID, err := db.GetIssueByID(ctx, third.ID)
	if err != nil {
		t.Fatalf("GetIssueByID: %v", err)
	}
	if byID.Number != 3 {
		t.Errorf("GetIssueByID number = %d, want 3", byID.Number)
	}

	if _, err := db.GetIssueByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetIssueByID(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestListIssuesByAuthorIsScopedAndPaged(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)
	const otherUser = int64(99)

	createTestIssue(t, db, telegramID, "Mine 1", model.IssueTypeBug)
	createTestIssue(t, db, otherUser, "Theirs", model.IssueTypeBug)
	createTestIssue(t, db, telegramID, "Mine 2", model.IssueTypeBug)

	total, err := db.CountIssuesByAuthor(ctx, telegramID)
	if err != nil {
		t.Fatalf("CountIssuesByAuthor: %v", err)
	}
	if total != 2 {
		t.Fatalf("CountIssuesByAuthor = %d, want 2", total)
	}

	page, err := db.ListIssuesByAuthor(ctx, telegramID, 1, 0)
	if err != nil {
		t.Fatalf("ListIssuesByAuthor: %v", err)
	}
	if len(page) != 1 || page[0].Title != "Mine 2" {
		t.Fatalf("expected newest-first paging to yield [Mine 2], got %+v", page)
	}

	page, err = db.ListIssuesByAuthor(ctx, telegramID, 1, 1)
	if err != nil {
		t.Fatalf("ListIssuesByAuthor page 2: %v", err)
	}
	if len(page) != 1 || page[0].Title != "Mine 1" {
		t.Fatalf("expected second page [Mine 1], got %+v", page)
	}
}

func TestUpdateIssueStatusAndFilters(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)

	bug := createTestIssue(t, db, telegramID, "Week view crashes", model.IssueTypeBug)
	createTestIssue(t, db, telegramID, "Calendar export", model.IssueTypeFeature)

	if err := db.UpdateIssueStatus(ctx, bug.ID, model.IssueInDevelopment, "", "admin@example.com"); err != nil {
		t.Fatalf("UpdateIssueStatus: %v", err)
	}
	updated, err := db.GetIssueByID(ctx, bug.ID)
	if err != nil {
		t.Fatalf("GetIssueByID: %v", err)
	}
	if updated.Status != model.IssueInDevelopment {
		t.Errorf("status = %q, want in_development", updated.Status)
	}
	if updated.StatusBy != "admin@example.com" {
		t.Errorf("status_by = %q, want admin@example.com", updated.StatusBy)
	}

	if err := db.UpdateIssueStatus(ctx, uuid.New(), model.IssueReady, "", "admin@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateIssueStatus(unknown) error = %v, want ErrNotFound", err)
	}

	inDev, err := db.ListIssues(ctx, IssueFilter{Status: string(model.IssueInDevelopment)})
	if err != nil {
		t.Fatalf("ListIssues(status): %v", err)
	}
	if len(inDev) != 1 || inDev[0].ID != bug.ID {
		t.Fatalf("status filter returned %+v, want only the bug", inDev)
	}

	features, err := db.CountIssues(ctx, IssueFilter{Type: string(model.IssueTypeFeature)})
	if err != nil {
		t.Fatalf("CountIssues(type): %v", err)
	}
	if features != 1 {
		t.Errorf("CountIssues(type=feature) = %d, want 1", features)
	}

	matches, err := db.ListIssues(ctx, IssueFilter{Query: "crash"})
	if err != nil {
		t.Fatalf("ListIssues(query): %v", err)
	}
	if len(matches) != 1 || matches[0].ID != bug.ID {
		t.Fatalf("search returned %+v, want only the bug", matches)
	}

	counts, err := db.CountIssuesByStatus(ctx)
	if err != nil {
		t.Fatalf("CountIssuesByStatus: %v", err)
	}
	if counts[string(model.IssueOnReview)] != 1 || counts[string(model.IssueInDevelopment)] != 1 {
		t.Errorf("status counts = %v, want one on_review and one in_development", counts)
	}
}

func TestIssueCommentsAndThreadFlag(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)
	issue := createTestIssue(t, db, telegramID, "Calendar export", model.IssueTypeFeature)

	if _, err := db.AddIssueComment(ctx, model.IssueComment{
		IssueID:     issue.ID,
		AuthorRole:  model.IssueCommentAdmin,
		AuthorLabel: "admin@example.com",
		Body:        "Which calendar app do you use?",
	}); err != nil {
		t.Fatalf("AddIssueComment(admin): %v", err)
	}
	if _, err := db.AddIssueComment(ctx, model.IssueComment{
		IssueID:     issue.ID,
		AuthorRole:  model.IssueCommentUser,
		AuthorLabel: "@tester",
		Body:        "Google Calendar.",
	}); err != nil {
		t.Fatalf("AddIssueComment(user): %v", err)
	}

	comments, err := db.ListIssueComments(ctx, issue.ID)
	if err != nil {
		t.Fatalf("ListIssueComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("ListIssueComments returned %d comments, want 2", len(comments))
	}
	if comments[0].AuthorRole != model.IssueCommentAdmin || comments[1].AuthorRole != model.IssueCommentUser {
		t.Errorf("expected oldest-first ordering, got %q then %q", comments[0].AuthorRole, comments[1].AuthorRole)
	}
	if comments[0].IssueID != issue.ID {
		t.Errorf("comment issue id = %v, want %v", comments[0].IssueID, issue.ID)
	}

	n, err := db.CountIssueComments(ctx, issue.ID)
	if err != nil {
		t.Fatalf("CountIssueComments: %v", err)
	}
	if n != 2 {
		t.Errorf("CountIssueComments = %d, want 2", n)
	}

	if err := db.SetIssueThreadState(ctx, issue.ID, model.IssueThreadOpen); err != nil {
		t.Fatalf("SetIssueThreadOpen: %v", err)
	}
	reloaded, err := db.GetIssueByID(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetIssueByID: %v", err)
	}
	if reloaded.ThreadState != model.IssueThreadOpen {
		t.Error("expected thread_open to be true after SetIssueThreadOpen")
	}
	if !reloaded.UpdatedAt.After(issue.UpdatedAt) && !reloaded.UpdatedAt.Equal(issue.UpdatedAt) {
		t.Error("expected updated_at to move forward with thread activity")
	}
}

func TestIssueDraftLifecycle(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)

	draft, err := db.GetIssueDraft(ctx, telegramID)
	if err != nil || draft != nil {
		t.Fatalf("GetIssueDraft on empty db = (%+v, %v), want (nil, nil)", draft, err)
	}

	if err := db.SetIssueDraft(ctx, model.IssueDraft{
		TelegramID:      telegramID,
		ChatID:          telegramID,
		PromptMessageID: 555,
		Step:            model.IssueStepTitle,
		IssueType:       model.IssueTypeBug,
	}); err != nil {
		t.Fatalf("SetIssueDraft: %v", err)
	}

	draft, err = db.GetIssueDraft(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetIssueDraft: %v", err)
	}
	if draft == nil || draft.Step != model.IssueStepTitle || draft.IssueType != model.IssueTypeBug || draft.PromptMessageID != 555 {
		t.Fatalf("GetIssueDraft returned %+v, want the saved title-step draft", draft)
	}

	// Advancing the wizard replaces the same row rather than adding another.
	issueID := uuid.New()
	if err := db.SetIssueDraft(ctx, model.IssueDraft{
		TelegramID:      telegramID,
		ChatID:          telegramID,
		PromptMessageID: 555,
		Step:            model.IssueStepReply,
		Title:           "Week view crashes",
		IssueID:         &issueID,
	}); err != nil {
		t.Fatalf("SetIssueDraft(update): %v", err)
	}
	draft, err = db.GetIssueDraft(ctx, telegramID)
	if err != nil || draft == nil {
		t.Fatalf("GetIssueDraft after update = (%+v, %v)", draft, err)
	}
	if draft.Step != model.IssueStepReply || draft.Title != "Week view crashes" {
		t.Errorf("draft not updated in place: %+v", draft)
	}
	if draft.IssueID == nil || *draft.IssueID != issueID {
		t.Errorf("draft issue id = %v, want %v", draft.IssueID, issueID)
	}

	if err := db.ClearIssueDraft(ctx, telegramID); err != nil {
		t.Fatalf("ClearIssueDraft: %v", err)
	}
	draft, err = db.GetIssueDraft(ctx, telegramID)
	if err != nil || draft != nil {
		t.Fatalf("GetIssueDraft after clear = (%+v, %v), want (nil, nil)", draft, err)
	}
}

func TestIssueDraftExpiry(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)

	if err := db.SetIssueDraft(ctx, model.IssueDraft{
		TelegramID:      telegramID,
		ChatID:          telegramID,
		PromptMessageID: 777,
		Step:            model.IssueStepBody,
		IssueType:       model.IssueTypeOther,
		Title:           "Stale draft",
	}); err != nil {
		t.Fatalf("SetIssueDraft: %v", err)
	}

	past := time.Now().UTC().Add(-time.Minute)
	if _, err := db.SQL.ExecContext(ctx, `UPDATE user_issue_drafts SET expires_at = ? WHERE telegram_id = ?`, past, telegramID); err != nil {
		t.Fatalf("backdating draft: %v", err)
	}

	expired, err := db.ListExpiredIssueDrafts(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListExpiredIssueDrafts: %v", err)
	}
	if len(expired) != 1 || expired[0].PromptMessageID != 777 {
		t.Fatalf("ListExpiredIssueDrafts returned %+v, want the stale draft", expired)
	}

	draft, err := db.GetIssueDraft(ctx, telegramID)
	if !errors.Is(err, ErrIssueDraftExpired) {
		t.Fatalf("GetIssueDraft on expired draft error = %v, want ErrIssueDraftExpired", err)
	}
	// The expired draft comes back with the error so the caller can still
	// delete the wizard message it left behind.
	if draft == nil || draft.PromptMessageID != 777 {
		t.Fatalf("expected the expired draft alongside the error, got %+v", draft)
	}

	// The expiry is reported exactly once: the row is consumed on read.
	draft, err = db.GetIssueDraft(ctx, telegramID)
	if err != nil || draft != nil {
		t.Fatalf("GetIssueDraft after expiry = (%+v, %v), want (nil, nil)", draft, err)
	}

	remaining, err := db.ListExpiredIssueDrafts(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListExpiredIssueDrafts after consume: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected the expired row to be gone, got %+v", remaining)
	}
}

func TestDeleteIssueRemovesCommentsAndDrafts(t *testing.T) {
	db, _, telegramID := setupTestDB(t)
	ctx := context.Background()

	issue, err := db.CreateIssue(ctx, model.Issue{
		AuthorTelegramID: telegramID,
		Type:             model.IssueTypeBug,
		Title:            "Crash on /today",
		Body:             "It crashes.",
	})
	if err != nil {
		t.Fatalf("creating issue: %v", err)
	}
	keep, err := db.CreateIssue(ctx, model.Issue{
		AuthorTelegramID: telegramID,
		Type:             model.IssueTypeOther,
		Title:            "Untouched",
		Body:             "Stays.",
	})
	if err != nil {
		t.Fatalf("creating second issue: %v", err)
	}

	if _, err := db.AddIssueComment(ctx, model.IssueComment{
		IssueID: issue.ID, AuthorRole: model.IssueCommentAdmin, Body: "A question",
	}); err != nil {
		t.Fatalf("adding comment: %v", err)
	}
	if _, err := db.AddIssueComment(ctx, model.IssueComment{
		IssueID: keep.ID, AuthorRole: model.IssueCommentAdmin, Body: "Unrelated",
	}); err != nil {
		t.Fatalf("adding comment to the other issue: %v", err)
	}

	// A reply draft pointing at the doomed issue must not survive it.
	if err := db.SetIssueDraft(ctx, model.IssueDraft{
		TelegramID: telegramID, ChatID: 1, PromptMessageID: 2,
		Step: model.IssueStepReply, IssueID: &issue.ID,
	}); err != nil {
		t.Fatalf("setting draft: %v", err)
	}

	if err := db.DeleteIssue(ctx, issue.ID); err != nil {
		t.Fatalf("deleting issue: %v", err)
	}

	if _, err := db.GetIssueByID(ctx, issue.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected the issue to be gone, got %v", err)
	}
	if n, err := db.CountIssueComments(ctx, issue.ID); err != nil || n != 0 {
		t.Errorf("expected its comments to cascade away, got %d (%v)", n, err)
	}
	draft, err := db.GetIssueDraft(ctx, telegramID)
	if err != nil {
		t.Fatalf("reading draft: %v", err)
	}
	if draft != nil {
		t.Error("expected the reply draft to be cleared with the issue")
	}

	// Deleting one issue must not touch its neighbours.
	if _, err := db.GetIssueByID(ctx, keep.ID); err != nil {
		t.Errorf("expected the other issue to survive, got %v", err)
	}
	if n, err := db.CountIssueComments(ctx, keep.ID); err != nil || n != 1 {
		t.Errorf("expected the other issue to keep its comment, got %d (%v)", n, err)
	}

	if err := db.DeleteIssue(ctx, issue.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound deleting twice, got %v", err)
	}
}

func TestIssueStatusNoteAndNewStatuses(t *testing.T) {
	db, _, telegramID := setupTestDB(t)
	ctx := context.Background()

	issue, err := db.CreateIssue(ctx, model.Issue{
		AuthorTelegramID: telegramID,
		Type:             model.IssueTypeFeature,
		Title:            "Add calendar export",
		Body:             "Please.",
	})
	if err != nil {
		t.Fatalf("creating issue: %v", err)
	}
	if issue.StatusNote != "" {
		t.Errorf("expected no note on a fresh issue, got %q", issue.StatusNote)
	}

	// 'rejected' and 'duplicate' are new in 00008; the CHECK constraint has to
	// accept them.
	if err := db.UpdateIssueStatus(ctx, issue.ID, model.IssueRejected, "Out of scope for now.", "admin@example.com"); err != nil {
		t.Fatalf("rejecting issue: %v", err)
	}
	reloaded, err := db.GetIssueByID(ctx, issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if reloaded.Status != model.IssueRejected || reloaded.StatusNote != "Out of scope for now." {
		t.Errorf("expected rejected + note, got %q / %q", reloaded.Status, reloaded.StatusNote)
	}

	// A later change without a note must not leave the old explanation behind.
	if err := db.UpdateIssueStatus(ctx, issue.ID, model.IssueDuplicate, "", "admin@example.com"); err != nil {
		t.Fatalf("marking duplicate: %v", err)
	}
	reloaded, err = db.GetIssueByID(ctx, issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if reloaded.Status != model.IssueDuplicate {
		t.Errorf("expected duplicate, got %q", reloaded.Status)
	}
	if reloaded.StatusNote != "" {
		t.Errorf("expected the stale note to be cleared, got %q", reloaded.StatusNote)
	}
}

func TestSetIssueThreadStateTransitions(t *testing.T) {
	db, _, telegramID := setupTestDB(t)
	ctx := context.Background()

	issue, err := db.CreateIssue(ctx, model.Issue{
		AuthorTelegramID: telegramID,
		Type:             model.IssueTypeBug,
		Title:            "Crash on /today",
		Body:             "It crashes.",
	})
	if err != nil {
		t.Fatalf("creating issue: %v", err)
	}
	if issue.ThreadState.Started() {
		t.Error("a new issue must not look like it has a discussion")
	}

	for _, state := range []model.IssueThreadState{model.IssueThreadOpen, model.IssueThreadClosed, model.IssueThreadOpen} {
		if err := db.SetIssueThreadState(ctx, issue.ID, state); err != nil {
			t.Fatalf("setting thread state %q: %v", state, err)
		}
		reloaded, err := db.GetIssueByID(ctx, issue.ID)
		if err != nil {
			t.Fatalf("reloading issue: %v", err)
		}
		if reloaded.ThreadState != state {
			t.Errorf("expected thread state %q, got %q", state, reloaded.ThreadState)
		}
		if !reloaded.ThreadState.Started() {
			t.Errorf("state %q should count as started", state)
		}
	}
}

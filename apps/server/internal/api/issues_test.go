package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// stubIssueNotifier records what the handlers would have DM'd to the reporter,
// standing in for the Telegram bot (which internal/api must not import).
type stubIssueNotifier struct {
	comments     []model.IssueComment
	statuses     []model.IssueStatus
	threadStates []model.IssueThreadState
}

func (s *stubIssueNotifier) NotifyIssueComment(_ context.Context, _ model.Issue, c model.IssueComment) error {
	s.comments = append(s.comments, c)
	return nil
}

func (s *stubIssueNotifier) NotifyIssueStatus(_ context.Context, issue model.Issue, _ model.IssueStatus) error {
	s.statuses = append(s.statuses, issue.Status)
	return nil
}

func (s *stubIssueNotifier) NotifyIssueThreadState(_ context.Context, issue model.Issue, _ model.IssueThreadState) error {
	s.threadStates = append(s.threadStates, issue.ThreadState)
	return nil
}

// setupIssuesRouter builds a fully migrated database (the issue tables come
// from 00007_issues.sql) behind the real admin router, unlike setupTestRouter
// which hand-rolls a minimal users table.
func setupIssuesRouter(t *testing.T) (http.Handler, *storage.DB, *stubIssueNotifier) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "issues_api_test.db")

	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrating db: %v", err)
	}
	db, err := storage.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	notifier := &stubIssueNotifier{}
	svc := NewService(db, campus.NewClient(db))
	svc.SetIssueNotifier(notifier)

	router := NewRouterWithOpts(svc, "internal-token", RouterOpts{AdminSecret: "admin-secret"})
	return router, db, notifier
}

func seedIssue(t *testing.T, db *storage.DB, title string, issueType model.IssueType) model.Issue {
	t.Helper()
	issue, err := db.CreateIssue(context.Background(), model.Issue{
		AuthorTelegramID: 111,
		AuthorUsername:   "reporter",
		AuthorFirstName:  "Reporter",
		Type:             issueType,
		Title:            title,
		Body:             "body of " + title,
	})
	if err != nil {
		t.Fatalf("creating issue: %v", err)
	}
	return issue
}

// adminRequest issues an authenticated admin call. Passing role "read-only"
// exercises adminWritePermissionMiddleware.
func adminRequest(t *testing.T, router http.Handler, method, path, role string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshalling body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("X-Admin-Secret", "admin-secret")
	req.Header.Set("X-Admin-Email", "admin@example.com")
	if role != "" {
		req.Header.Set("X-Admin-Role", role)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding response %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestGetAdminIssuesListsAndFilters(t *testing.T) {
	router, db, _ := setupIssuesRouter(t)

	bug := seedIssue(t, db, "Crash on /today", model.IssueTypeBug)
	seedIssue(t, db, "Add calendar export", model.IssueTypeFeature)

	if err := db.UpdateIssueStatus(context.Background(), bug.ID, model.IssueInDevelopment, "", "admin@example.com"); err != nil {
		t.Fatalf("updating status: %v", err)
	}

	// Unfiltered: both issues plus the per-status tally the dashboard tabs use.
	rec := adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if total, _ := body["total"].(float64); total != 2 {
		t.Errorf("expected total 2, got %v", body["total"])
	}
	counts, _ := body["status_counts"].(map[string]any)
	if counts[string(model.IssueInDevelopment)] != float64(1) {
		t.Errorf("expected one in_development issue, got %v", counts)
	}

	// Filtered by status.
	rec = adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues?status=in_development", "", nil)
	body = decodeBody(t, rec)
	issues, _ := body["issues"].([]any)
	if len(issues) != 1 {
		t.Fatalf("expected 1 filtered issue, got %d", len(issues))
	}
	if first, _ := issues[0].(map[string]any); first["title"] != "Crash on /today" {
		t.Errorf("unexpected filtered issue: %v", first)
	}

	// Filtered by free-text search.
	rec = adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues?q=calendar", "", nil)
	body = decodeBody(t, rec)
	if issues, _ := body["issues"].([]any); len(issues) != 1 {
		t.Errorf("expected 1 search hit, got %d", len(issues))
	}

	// An unknown status is a client error, not an empty list.
	rec = adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues?status=nonsense", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown status filter, got %d", rec.Code)
	}
}

func TestGetAdminIssueNotFound(t *testing.T) {
	router, _, _ := setupIssuesRouter(t)

	rec := adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues/not-a-uuid", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed id, got %d", rec.Code)
	}

	rec = adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues/"+uuid.New().String(), "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown id, got %d", rec.Code)
	}
	if code, _ := decodeBody(t, rec)["error_code"].(string); code != model.ErrIssueNotFound {
		t.Errorf("expected %s, got %v", model.ErrIssueNotFound, code)
	}
}

func TestPatchAdminIssueStatus(t *testing.T) {
	router, db, notifier := setupIssuesRouter(t)
	issue := seedIssue(t, db, "Crash on /today", model.IssueTypeBug)
	path := "/api/v1/admin/issues/" + issue.ID.String() + "/status"

	// Invalid status.
	rec := adminRequest(t, router, http.MethodPatch, path, "", map[string]string{"status": "shipped"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown status, got %d", rec.Code)
	}

	// Valid change: persisted, attributed, and pushed to the reporter.
	rec = adminRequest(t, router, http.MethodPatch, path, "", map[string]string{"status": "ready"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if changed, _ := decodeBody(t, rec)["changed"].(bool); !changed {
		t.Error("expected changed=true")
	}

	stored, err := db.GetIssueByID(context.Background(), issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if stored.Status != model.IssueReady {
		t.Errorf("expected status ready, got %q", stored.Status)
	}
	if stored.StatusBy != "admin@example.com" {
		t.Errorf("expected status_by to record the acting admin, got %q", stored.StatusBy)
	}
	if len(notifier.statuses) != 1 {
		t.Fatalf("expected 1 status notification, got %d", len(notifier.statuses))
	}

	// Re-sending the same status must not spam the reporter.
	rec = adminRequest(t, router, http.MethodPatch, path, "", map[string]string{"status": "ready"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if changed, _ := decodeBody(t, rec)["changed"].(bool); changed {
		t.Error("expected changed=false for a no-op status write")
	}
	if len(notifier.statuses) != 1 {
		t.Errorf("expected no extra notification, got %d", len(notifier.statuses))
	}
}

func TestPostAdminIssueCommentOpensThread(t *testing.T) {
	router, db, notifier := setupIssuesRouter(t)
	issue := seedIssue(t, db, "Add calendar export", model.IssueTypeFeature)
	path := "/api/v1/admin/issues/" + issue.ID.String() + "/comments"

	// An empty body is rejected before anything is written.
	rec := adminRequest(t, router, http.MethodPost, path, "", map[string]string{"body": "   "})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a blank comment, got %d", rec.Code)
	}

	rec = adminRequest(t, router, http.MethodPost, path, "", map[string]string{"body": "Which calendar app do you use?"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	comment, _ := decodeBody(t, rec)["comment"].(map[string]any)
	if comment["author_role"] != string(model.IssueCommentAdmin) {
		t.Errorf("expected an admin-authored comment, got %v", comment["author_role"])
	}
	if comment["author_label"] != "admin@example.com" {
		t.Errorf("expected the admin email as the label, got %v", comment["author_label"])
	}

	// The first admin comment is what unlocks the discussion screen in the bot.
	stored, err := db.GetIssueByID(context.Background(), issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if stored.ThreadState != model.IssueThreadOpen {
		t.Error("expected the first admin comment to open the thread")
	}
	if len(notifier.comments) != 1 {
		t.Fatalf("expected 1 comment notification, got %d", len(notifier.comments))
	}

	// The detail endpoint returns the transcript.
	rec = adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues/"+issue.ID.String(), "", nil)
	if comments, _ := decodeBody(t, rec)["comments"].([]any); len(comments) != 1 {
		t.Errorf("expected 1 comment in the detail response, got %d", len(comments))
	}
}

func TestIssueWritesRejectReadOnlyAdmins(t *testing.T) {
	router, db, notifier := setupIssuesRouter(t)
	issue := seedIssue(t, db, "Crash on /today", model.IssueTypeBug)

	// Reading stays open to every role.
	if rec := adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues", "read-only", nil); rec.Code != http.StatusOK {
		t.Errorf("expected read-only admins to list issues, got %d", rec.Code)
	}
	if rec := adminRequest(t, router, http.MethodGet, "/api/v1/admin/issues/"+issue.ID.String(), "read-only", nil); rec.Code != http.StatusOK {
		t.Errorf("expected read-only admins to open an issue, got %d", rec.Code)
	}

	rec := adminRequest(t, router, http.MethodPatch, "/api/v1/admin/issues/"+issue.ID.String()+"/status", "read-only",
		map[string]string{"status": "ready"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 on status write, got %d", rec.Code)
	}

	rec = adminRequest(t, router, http.MethodPost, "/api/v1/admin/issues/"+issue.ID.String()+"/comments", "read-only",
		map[string]string{"body": "hello"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 on comment write, got %d", rec.Code)
	}

	stored, err := db.GetIssueByID(context.Background(), issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if stored.Status != model.IssueOnReview || stored.ThreadState != model.IssueThreadNone {
		t.Error("expected the rejected writes to leave the issue untouched")
	}
	if len(notifier.comments)+len(notifier.statuses) != 0 {
		t.Error("expected no notifications for rejected writes")
	}
}

func TestPatchAdminIssueStatusNote(t *testing.T) {
	router, db, notifier := setupIssuesRouter(t)
	issue := seedIssue(t, db, "Add calendar export", model.IssueTypeFeature)
	path := "/api/v1/admin/issues/" + issue.ID.String() + "/status"

	// The whole point of the note: explain a rejection without opening a thread.
	rec := adminRequest(t, router, http.MethodPatch, path, "",
		map[string]string{"status": "rejected", "note": "Out of scope for this term."})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	view, _ := decodeBody(t, rec)["issue"].(map[string]any)
	if view["status_note"] != "Out of scope for this term." {
		t.Errorf("expected the note on the response, got %v", view["status_note"])
	}

	stored, err := db.GetIssueByID(context.Background(), issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if stored.Status != model.IssueRejected || stored.StatusNote != "Out of scope for this term." {
		t.Errorf("expected rejected + note, got %q / %q", stored.Status, stored.StatusNote)
	}
	// A note must not open a discussion — that is the difference from a comment.
	if stored.ThreadState.Started() {
		t.Error("a status note must not start a discussion thread")
	}
	if len(notifier.statuses) != 1 {
		t.Errorf("expected the reporter to be notified once, got %d", len(notifier.statuses))
	}

	// A note with no status change would be silently dropped, so it is refused.
	rec = adminRequest(t, router, http.MethodPatch, path, "",
		map[string]string{"status": "rejected", "note": "Another thought."})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a note without a status change, got %d", rec.Code)
	}

	// Over-long notes are rejected before anything is written.
	rec = adminRequest(t, router, http.MethodPatch, path, "",
		map[string]string{"status": "duplicate", "note": strings.Repeat("x", model.IssueCommentMaxLen+1)})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an over-long note, got %d", rec.Code)
	}
}

func TestPatchAdminIssueThreadCloseAndReopen(t *testing.T) {
	router, db, notifier := setupIssuesRouter(t)
	issue := seedIssue(t, db, "Add calendar export", model.IssueTypeFeature)
	threadPath := "/api/v1/admin/issues/" + issue.ID.String() + "/thread"
	commentPath := "/api/v1/admin/issues/" + issue.ID.String() + "/comments"

	// Nothing to close before a discussion exists.
	rec := adminRequest(t, router, http.MethodPatch, threadPath, "", map[string]string{"state": "closed"})
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 closing a thread that was never started, got %d", rec.Code)
	}

	rec = adminRequest(t, router, http.MethodPatch, threadPath, "", map[string]string{"state": "none"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an unknown thread state, got %d", rec.Code)
	}

	if rec = adminRequest(t, router, http.MethodPost, commentPath, "", map[string]string{"body": "Which app?"}); rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 posting the first comment, got %d", rec.Code)
	}

	rec = adminRequest(t, router, http.MethodPatch, threadPath, "", map[string]string{"state": "closed"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 closing the thread, got %d: %s", rec.Code, rec.Body.String())
	}
	stored, err := db.GetIssueByID(context.Background(), issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if stored.ThreadState != model.IssueThreadClosed {
		t.Errorf("expected the thread to be closed, got %q", stored.ThreadState)
	}
	if len(notifier.threadStates) != 1 {
		t.Errorf("expected the reporter to be told once, got %d", len(notifier.threadStates))
	}

	// Replying into a closed thread would resurrect it behind the reporter's back.
	rec = adminRequest(t, router, http.MethodPost, commentPath, "", map[string]string{"body": "One more thing"})
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 replying to a closed thread, got %d", rec.Code)
	}

	// Closing an already-closed thread is a no-op, not a second DM.
	rec = adminRequest(t, router, http.MethodPatch, threadPath, "", map[string]string{"state": "closed"})
	if changed, _ := decodeBody(t, rec)["changed"].(bool); changed {
		t.Error("expected changed=false re-closing a closed thread")
	}
	if len(notifier.threadStates) != 1 {
		t.Errorf("expected no extra notification, got %d", len(notifier.threadStates))
	}

	// Reopening restores replies on both sides.
	if rec = adminRequest(t, router, http.MethodPatch, threadPath, "", map[string]string{"state": "open"}); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 reopening, got %d", rec.Code)
	}
	if rec = adminRequest(t, router, http.MethodPost, commentPath, "", map[string]string{"body": "One more thing"}); rec.Code != http.StatusCreated {
		t.Errorf("expected 201 replying after reopening, got %d", rec.Code)
	}
}

func TestDeleteAdminIssue(t *testing.T) {
	router, db, _ := setupIssuesRouter(t)
	issue := seedIssue(t, db, "Spam", model.IssueTypeOther)
	path := "/api/v1/admin/issues/" + issue.ID.String()

	if rec := adminRequest(t, router, http.MethodDelete, path, "read-only", nil); rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for a read-only admin, got %d", rec.Code)
	}
	if _, err := db.GetIssueByID(context.Background(), issue.ID); err != nil {
		t.Fatalf("the rejected delete must leave the issue in place: %v", err)
	}

	if rec := adminRequest(t, router, http.MethodDelete, path, "", nil); rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if _, err := db.GetIssueByID(context.Background(), issue.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected the issue to be gone, got %v", err)
	}

	if rec := adminRequest(t, router, http.MethodDelete, path, "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 deleting it twice, got %d", rec.Code)
	}
}

func TestIssueThreadWriteRejectsReadOnlyAdmins(t *testing.T) {
	router, db, notifier := setupIssuesRouter(t)
	issue := seedIssue(t, db, "Add calendar export", model.IssueTypeFeature)

	if err := db.SetIssueThreadState(context.Background(), issue.ID, model.IssueThreadOpen); err != nil {
		t.Fatalf("opening thread: %v", err)
	}

	rec := adminRequest(t, router, http.MethodPatch, "/api/v1/admin/issues/"+issue.ID.String()+"/thread",
		"read-only", map[string]string{"state": "closed"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 closing a thread as read-only, got %d", rec.Code)
	}

	stored, err := db.GetIssueByID(context.Background(), issue.ID)
	if err != nil {
		t.Fatalf("reloading issue: %v", err)
	}
	if stored.ThreadState != model.IssueThreadOpen {
		t.Errorf("expected the thread to stay open, got %q", stored.ThreadState)
	}
	if len(notifier.threadStates) != 0 {
		t.Error("expected no notification for a rejected write")
	}
}

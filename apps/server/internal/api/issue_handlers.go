package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// issueView is the JSON shape the admin dashboard consumes. It is deliberately
// separate from model.Issue so renaming a column never silently changes the
// dashboard's contract.
type issueView struct {
	ID               string    `json:"id"`
	Number           int       `json:"number"`
	AuthorTelegramID int64     `json:"author_telegram_id"`
	AuthorUsername   string    `json:"author_username"`
	AuthorFirstName  string    `json:"author_first_name"`
	Type             string    `json:"type"`
	Title            string    `json:"title"`
	Body             string    `json:"body"`
	Status           string    `json:"status"`
	StatusBy         string    `json:"status_by"`
	StatusNote       string    `json:"status_note"`
	ThreadState      string    `json:"thread_state"`
	CommentCount     int       `json:"comment_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type issueCommentView struct {
	ID          string    `json:"id"`
	AuthorRole  string    `json:"author_role"`
	AuthorLabel string    `json:"author_label"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

func toIssueView(issue model.Issue, commentCount int) issueView {
	return issueView{
		ID:               issue.ID.String(),
		Number:           issue.Number,
		AuthorTelegramID: issue.AuthorTelegramID,
		AuthorUsername:   issue.AuthorUsername,
		AuthorFirstName:  issue.AuthorFirstName,
		Type:             string(issue.Type),
		Title:            issue.Title,
		Body:             issue.Body,
		Status:           string(issue.Status),
		StatusBy:         issue.StatusBy,
		StatusNote:       issue.StatusNote,
		ThreadState:      string(issue.ThreadState),
		CommentCount:     commentCount,
		CreatedAt:        issue.CreatedAt,
		UpdatedAt:        issue.UpdatedAt,
	}
}

func toIssueCommentView(c model.IssueComment) issueCommentView {
	return issueCommentView{
		ID:          c.ID.String(),
		AuthorRole:  string(c.AuthorRole),
		AuthorLabel: c.AuthorLabel,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
}

type updateIssueStatusRequest struct {
	Status string `json:"status"`
	// Note is the optional explanation delivered to the reporter alongside the
	// status DM — a way to say "rejected because…" without opening a thread.
	Note string `json:"note"`
}

type updateIssueThreadRequest struct {
	State string `json:"state"`
}

type createIssueCommentRequest struct {
	Body string `json:"body"`
}

// adminEmail identifies the acting admin. The dashboard forwards it on every
// proxied call (see apps/admin/src/lib/server/main-server.ts); the shared
// telemetry pipeline strips identifiers, so this header is the only audit
// trail the issue queue gets.
func adminEmail(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Admin-Email"))
}

func (h *handlers) getAdminIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := storage.IssueFilter{
		Status: q.Get("status"),
		Type:   q.Get("type"),
		Query:  q.Get("q"),
		Limit:  50,
	}
	if filter.Status != "" && !model.ValidIssueStatus(filter.Status) {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "unknown status filter: "+filter.Status)
		return
	}
	if filter.Type != "" && !model.ValidIssueType(filter.Type) {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "unknown type filter: "+filter.Type)
		return
	}
	if raw := q.Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			filter.Limit = parsed
		}
	}
	if raw := q.Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			filter.Offset = parsed
		}
	}

	issues, err := h.svc.DB().ListIssues(r.Context(), filter)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}
	total, err := h.svc.DB().CountIssues(r.Context(), filter)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}
	statusCounts, err := h.svc.DB().CountIssuesByStatus(r.Context(), filter)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	ids := make([]uuid.UUID, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}
	counts, err := h.svc.DB().CountIssueCommentsByIssue(r.Context(), ids)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	views := make([]issueView, 0, len(issues))
	for _, issue := range issues {
		views = append(views, toIssueView(issue, counts[issue.ID]))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"issues":        views,
		"total":         total,
		"limit":         filter.Limit,
		"offset":        filter.Offset,
		"status_counts": statusCounts,
	})
}

// issueFromPath resolves the {id} path parameter, writing the error response
// itself and returning ok=false when it cannot.
func (h *handlers) issueFromPath(w http.ResponseWriter, r *http.Request) (model.Issue, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid issue id")
		return model.Issue{}, false
	}

	issue, err := h.svc.DB().GetIssueByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			model.WriteError(w, http.StatusNotFound, model.ErrIssueNotFound, "issue not found")
			return model.Issue{}, false
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return model.Issue{}, false
	}
	return issue, true
}

func (h *handlers) getAdminIssue(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.issueFromPath(w, r)
	if !ok {
		return
	}

	comments, err := h.svc.DB().ListIssueComments(r.Context(), issue.ID)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	commentViews := make([]issueCommentView, 0, len(comments))
	for _, c := range comments {
		commentViews = append(commentViews, toIssueCommentView(c))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"issue":    toIssueView(issue, len(comments)),
		"comments": commentViews,
	})
}

// issueCommentCount is a best-effort tally for a response body: a count the
// caller only displays is not worth failing an already-committed write over.
func (h *handlers) issueCommentCount(r *http.Request, issue model.Issue) int {
	n, err := h.svc.DB().CountIssueComments(r.Context(), issue.ID)
	if err != nil {
		slog.Warn("counting issue comments", "error", err, "issue_id", issue.ID)
		return 0
	}
	return n
}

func (h *handlers) patchAdminIssueStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	issue, ok := h.issueFromPath(w, r)
	if !ok {
		return
	}

	var req updateIssueStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid JSON payload")
		return
	}
	if !model.ValidIssueStatus(req.Status) {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "unknown status: "+req.Status)
		return
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > model.IssueCommentMaxLen {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest,
			"note is too long, keep it under "+strconv.Itoa(model.IssueCommentMaxLen)+" characters")
		return
	}

	next := model.IssueStatus(req.Status)
	if next == issue.Status {
		// A note only ever travels with a status change. Silently dropping one
		// the admin typed would be worse than refusing it, so say what to do
		// instead rather than pretending the request succeeded.
		if note != "" {
			model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest,
				"status is unchanged; use the discussion thread to message the reporter without changing the status")
			return
		}
		// Nothing changed; don't notify the reporter about a no-op.
		writeJSON(w, http.StatusOK, map[string]any{"issue": toIssueView(issue, h.issueCommentCount(r, issue)), "changed": false})
		return
	}

	if err := h.svc.DB().UpdateIssueStatus(r.Context(), issue.ID, next, note, adminEmail(r)); err != nil {
		if h.telemetry != nil {
			h.telemetry.ReportAction("admin_action", "issue_status:"+req.Status, http.StatusInternalServerError, time.Since(start).Milliseconds(), nil)
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	previous := issue.Status
	issue.Status = next
	issue.StatusBy = adminEmail(r)
	issue.StatusNote = note
	h.svc.NotifyIssueStatus(r.Context(), issue, previous)

	if h.telemetry != nil {
		h.telemetry.ReportAction("admin_action", "issue_status:"+req.Status, http.StatusOK, time.Since(start).Milliseconds(), map[string]any{
			"from": string(previous),
			"to":   req.Status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"issue": toIssueView(issue, h.issueCommentCount(r, issue)), "changed": true})
}

func (h *handlers) postAdminIssueComment(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	issue, ok := h.issueFromPath(w, r)
	if !ok {
		return
	}

	var req createIssueCommentRequest
	if err := decodeJSON(r, &req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid JSON payload")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "body is required")
		return
	}
	if len([]rune(body)) > model.IssueCommentMaxLen {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest,
			"body is too long, keep it under "+strconv.Itoa(model.IssueCommentMaxLen)+" characters")
		return
	}

	// Posting into a closed thread would silently resurrect it for the
	// reporter, who was told the discussion had ended. Reopening is a separate,
	// deliberate action.
	if issue.ThreadState == model.IssueThreadClosed {
		model.WriteError(w, http.StatusConflict, model.ErrInvalidRequest,
			"the discussion is closed; reopen it before replying")
		return
	}

	comment, err := h.svc.DB().AddIssueComment(r.Context(), model.IssueComment{
		IssueID:     issue.ID,
		AuthorRole:  model.IssueCommentAdmin,
		AuthorLabel: adminEmail(r),
		Body:        body,
	})
	if err != nil {
		if h.telemetry != nil {
			h.telemetry.ReportAction("admin_action", "issue_comment", http.StatusInternalServerError, time.Since(start).Milliseconds(), nil)
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	// The first admin comment is what opens the thread for the reporter, and
	// AddIssueComment already did it in the same transaction — so the DM below
	// can never hand out a button leading to a thread the bot thinks is
	// unstarted.
	if !issue.ThreadState.Started() {
		issue.ThreadState = model.IssueThreadOpen
	}

	h.svc.NotifyIssueComment(r.Context(), issue, comment)

	if h.telemetry != nil {
		h.telemetry.ReportAction("admin_action", "issue_comment", http.StatusOK, time.Since(start).Milliseconds(), nil)
	}

	writeJSON(w, http.StatusCreated, map[string]any{"comment": toIssueCommentView(comment)})
}

// patchAdminIssueThread opens, closes or reopens a discussion. Closing leaves
// the transcript readable to the reporter but stops them replying; the thread
// cannot be pushed back to "none", since a thread with history is not one that
// never existed.
func (h *handlers) patchAdminIssueThread(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	issue, ok := h.issueFromPath(w, r)
	if !ok {
		return
	}

	var req updateIssueThreadRequest
	if err := decodeJSON(r, &req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid JSON payload")
		return
	}

	next := model.IssueThreadState(req.State)
	if next != model.IssueThreadOpen && next != model.IssueThreadClosed {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest,
			`state must be "open" or "closed"`)
		return
	}
	// Threads are admin-initiated by the first comment, so neither state can be
	// reached from "none": there is nothing to close, and "reopening" an empty
	// thread would hand the reporter a Reply button over a blank transcript.
	if !issue.ThreadState.Started() {
		model.WriteError(w, http.StatusConflict, model.ErrInvalidRequest,
			"there is no discussion yet; post a comment to start one")
		return
	}

	if next == issue.ThreadState {
		writeJSON(w, http.StatusOK, map[string]any{"issue": toIssueView(issue, h.issueCommentCount(r, issue)), "changed": false})
		return
	}

	if err := h.svc.DB().SetIssueThreadState(r.Context(), issue.ID, next); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	previous := issue.ThreadState
	issue.ThreadState = next
	h.svc.NotifyIssueThreadState(r.Context(), issue, previous)

	if h.telemetry != nil {
		h.telemetry.ReportAction("admin_action", "issue_thread:"+req.State, http.StatusOK, time.Since(start).Milliseconds(), nil)
	}

	writeJSON(w, http.StatusOK, map[string]any{"issue": toIssueView(issue, h.issueCommentCount(r, issue)), "changed": true})
}

// deleteAdminIssue removes an issue and its whole discussion permanently. The
// reporter is not notified: an issue they can no longer see is not something a
// DM can usefully explain, and a deletion is usually spam cleanup.
func (h *handlers) deleteAdminIssue(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	issue, ok := h.issueFromPath(w, r)
	if !ok {
		return
	}

	if err := h.svc.DB().DeleteIssue(r.Context(), issue.ID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			model.WriteError(w, http.StatusNotFound, model.ErrIssueNotFound, "issue not found")
			return
		}
		if h.telemetry != nil {
			h.telemetry.ReportAction("admin_action", "issue_delete", http.StatusInternalServerError, time.Since(start).Milliseconds(), nil)
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	if h.telemetry != nil {
		h.telemetry.ReportAction("admin_action", "issue_delete", http.StatusOK, time.Since(start).Milliseconds(), nil)
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

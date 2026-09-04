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
	ThreadOpen       bool      `json:"thread_open"`
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
		ThreadOpen:       issue.ThreadOpen,
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
	statusCounts, err := h.svc.DB().CountIssuesByStatus(r.Context())
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	views := make([]issueView, 0, len(issues))
	for _, issue := range issues {
		count, err := h.svc.DB().CountIssueComments(r.Context(), issue.ID)
		if err != nil {
			// A missing count is not worth failing the whole queue over.
			slog.Warn("counting issue comments", "error", err, "issue_id", issue.ID)
		}
		views = append(views, toIssueView(issue, count))
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

	next := model.IssueStatus(req.Status)
	if next == issue.Status {
		// Nothing changed; don't notify the reporter about a no-op.
		writeJSON(w, http.StatusOK, map[string]any{"issue": toIssueView(issue, 0), "changed": false})
		return
	}

	if err := h.svc.DB().UpdateIssueStatus(r.Context(), issue.ID, next, adminEmail(r)); err != nil {
		if h.telemetry != nil {
			h.telemetry.ReportAction("admin_action", "issue_status:"+req.Status, http.StatusInternalServerError, time.Since(start).Milliseconds(), nil)
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	previous := issue.Status
	issue.Status = next
	issue.StatusBy = adminEmail(r)
	h.svc.NotifyIssueStatus(r.Context(), issue, previous)

	if h.telemetry != nil {
		h.telemetry.ReportAction("admin_action", "issue_status:"+req.Status, http.StatusOK, time.Since(start).Milliseconds(), map[string]any{
			"from": string(previous),
			"to":   req.Status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"issue": toIssueView(issue, 0), "changed": true})
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

	// The first admin comment is what opens the thread for the reporter.
	if !issue.ThreadOpen {
		if err := h.svc.DB().SetIssueThreadOpen(r.Context(), issue.ID, true); err != nil {
			slog.Error("opening issue thread", "error", err, "issue_id", issue.ID)
		} else {
			issue.ThreadOpen = true
		}
	}

	h.svc.NotifyIssueComment(r.Context(), issue, comment)

	if h.telemetry != nil {
		h.telemetry.ReportAction("admin_action", "issue_comment", http.StatusOK, time.Since(start).Milliseconds(), nil)
	}

	writeJSON(w, http.StatusCreated, map[string]any{"comment": toIssueCommentView(comment)})
}

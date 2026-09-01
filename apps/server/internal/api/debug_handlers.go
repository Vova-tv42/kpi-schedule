package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/mykpi"
)

type debugDumpRequest struct {
	Cookies   model.Cookies `json:"cookies"`
	UserAgent string        `json:"user_agent"`
}

// POST /api/v1/debug/mykpi/dump — only mounted when DEBUG_ROUTES=true.
// Captures both the calendar shell page and the FullCalendar events JSON it
// references, so the scraper's parser can be written against real markup
// and a real event payload instead of guessed selectors.
// See docs/schedules/main/data-extraction.md.
func (h *handlers) postDebugMyKPIDump(w http.ResponseWriter, r *http.Request) {
	var req debugDumpRequest
	if err := decodeJSON(r, &req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid JSON body")
		return
	}
	if req.Cookies.PHPSESSID == "" {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "cookies.PHPSESSID is required")
		return
	}

	page, err := h.svc.mykpi.FetchCalendarPage(r.Context(), req.Cookies, req.UserAgent)
	if err != nil {
		if errors.Is(err, mykpi.ErrSessionExpired) {
			model.WriteError(w, http.StatusUnauthorized, model.ErrInvalidSessionCookies, "my.kpi.ua rejected these cookies")
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	pagePath := filepath.Join("internal", "mykpi", "testdata", fmt.Sprintf("calendar-%s.html", stamp))
	if err := os.WriteFile(pagePath, page, 0o644); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, fmt.Sprintf("writing calendar fixture: %v", err))
		return
	}

	result := map[string]any{
		"success":        true,
		"calendar_path":  pagePath,
		"calendar_bytes": len(page),
	}

	eventsURL, err := mykpi.ExtractEventsURL(page)
	if err != nil {
		result["events_error"] = err.Error()
		writeJSON(w, http.StatusOK, result)
		return
	}
	result["events_url"] = eventsURL

	now := time.Now().UTC()
	eventsJSON, err := h.svc.mykpi.FetchEventsJSONRange(r.Context(), eventsURL, req.Cookies, req.UserAgent, now.Add(-fetchWindowPast), now.Add(fetchWindowFuture))
	if err != nil {
		result["events_error"] = err.Error()
		writeJSON(w, http.StatusOK, result)
		return
	}

	eventsPath := filepath.Join("internal", "mykpi", "testdata", fmt.Sprintf("events-%s.json", stamp))
	if err := os.WriteFile(eventsPath, eventsJSON, 0o644); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, fmt.Sprintf("writing events fixture: %v", err))
		return
	}
	result["events_path"] = eventsPath
	result["events_bytes"] = len(eventsJSON)

	writeJSON(w, http.StatusOK, result)
}

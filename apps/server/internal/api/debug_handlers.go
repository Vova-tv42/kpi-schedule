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
// Captures a raw HTML fixture from my.kpi.ua so the scraper's parser can be
// written and tested against real markup instead of guessed selectors.
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

	html, err := h.svc.mykpi.FetchCalendarHTML(r.Context(), req.Cookies, req.UserAgent)
	if err != nil {
		if errors.Is(err, mykpi.ErrSessionExpired) {
			model.WriteError(w, http.StatusUnauthorized, model.ErrInvalidSessionCookies, "my.kpi.ua rejected these cookies")
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	filename := fmt.Sprintf("calendar-%s.html", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join("internal", "mykpi", "testdata", filename)
	if err := os.WriteFile(path, html, 0o644); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, fmt.Sprintf("writing fixture: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"path":       path,
		"size_bytes": len(html),
	})
}
